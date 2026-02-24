package services

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"time"

	"github.com/ensoul-labs/ensoul-server/chain"
	"github.com/ensoul-labs/ensoul-server/config"
	"github.com/ensoul-labs/ensoul-server/database"
	"github.com/ensoul-labs/ensoul-server/models"
	"github.com/ensoul-labs/ensoul-server/util"
)

const (
	// MintRevenueBuybackPct — percentage of mint BNB used for buyback (→ mining pool)
	MintRevenueBuybackPct = 60
	// MintRevenuePoolPct — percentage of mint BNB for holder revenue pool (→ $Ensoul)
	MintRevenuePoolPct = 10
	// MintRevenueRetainPct — percentage of mint BNB retained by Treasury (→ cold wallet)
	MintRevenueRetainPct = 30
	// SubscriptionRevenueBuybackPct — percentage of subscription revenue for buyback
	SubscriptionRevenueBuybackPct = 40
	// SubscriptionRevenuePoolPct — percentage of subscription revenue for holder revenue pool
	SubscriptionRevenuePoolPct = 10
	// SubscriptionRevenueRetainPct — percentage of subscription revenue retained by Treasury
	SubscriptionRevenueRetainPct = 50
	// SwapSlippageBps — default slippage tolerance for PancakeSwap (5%)
	SwapSlippageBps = 500
)

// ProcessMintRevenue handles BNB received from an NFT mint.
// After Plan C, ALL mint BNB arrives at the Buyback Wallet (contract treasury = buyback wallet).
//
// Flow:
//  1. Swap 70% BNB (60%+10%) → $Ensoul in a single swap for better price impact
//  2. Split $Ensoul output: 6/7 → Mining Pool wallet, 1/7 → Revenue Pool wallet
//  3. Transfer 30% BNB → Treasury cold wallet
func ProcessMintRevenue(bnbAmountWei *big.Int) error {
	if bnbAmountWei == nil || bnbAmountWei.Sign() <= 0 {
		return fmt.Errorf("invalid BNB amount")
	}

	swapPct := int64(MintRevenueBuybackPct + MintRevenuePoolPct) // 70
	return processRevenueSplit(bnbAmountWei, swapPct,
		MintRevenueBuybackPct, MintRevenuePoolPct, MintRevenueRetainPct,
		"mint_revenue")
}

// ProcessBNBSubscriptionRevenue handles BNB-denominated subscription payments.
// Same 3-way split: swap (buyback+pool)%, transfer retain% to Treasury.
func ProcessBNBSubscriptionRevenue(bnbAmountWei *big.Int) error {
	if bnbAmountWei == nil || bnbAmountWei.Sign() <= 0 {
		return fmt.Errorf("invalid BNB amount")
	}

	swapPct := int64(SubscriptionRevenueBuybackPct + SubscriptionRevenuePoolPct) // 50
	return processRevenueSplit(bnbAmountWei, swapPct,
		SubscriptionRevenueBuybackPct, SubscriptionRevenuePoolPct, SubscriptionRevenueRetainPct,
		"subscription_revenue_bnb")
}

// ProcessSubscriptionRevenue handles USDT subscription payments.
// Step 1: Swap (buyback+pool)% USDT → BNB → $Ensoul, split to mining + revenue pool.
// Step 2: retain% USDT stays in Buyback wallet (operator transfers to Treasury periodically).
func ProcessSubscriptionRevenue(usdAmountWei *big.Int) error {
	if usdAmountWei == nil || usdAmountWei.Sign() <= 0 {
		return fmt.Errorf("invalid USD amount")
	}

	// Calculate the portion to swap: (buyback + pool)% of total USDT
	swapPct := int64(SubscriptionRevenueBuybackPct + SubscriptionRevenuePoolPct) // 50
	usdtSwapAmount := new(big.Int).Mul(usdAmountWei, big.NewInt(swapPct))
	usdtSwapAmount.Div(usdtSwapAmount, big.NewInt(100))

	if usdtSwapAmount.Sign() <= 0 {
		util.Log.Info("[buyback] Subscription revenue too small, skipping")
		return nil
	}

	util.Log.Info("[buyback] Processing subscription revenue: %s USDT total, %s USDT (%d%%) for swap",
		usdAmountWei.String(), usdtSwapAmount.String(), swapPct)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	buybackKey, err := chain.ParsePrivateKey(config.Cfg.BuybackPrivateKey)
	if err != nil {
		return fmt.Errorf("failed to parse buyback wallet key: %w", err)
	}

	// Step 1: Swap USDT → BNB
	swapTxHash, bnbMinOut, err := chain.SwapUSDTForBNB(ctx, buybackKey, usdtSwapAmount, SwapSlippageBps)
	if err != nil {
		return fmt.Errorf("USDT→BNB swap failed: %w", err)
	}

	success, err := chain.WaitForTokenTx(ctx, swapTxHash)
	if err != nil || !success {
		util.Log.Error("[buyback] USDT→BNB swap tx failed: %s, err: %v", swapTxHash, err)
		return fmt.Errorf("USDT→BNB swap tx failed: %s", swapTxHash)
	}

	util.Log.Info("[buyback] USDT→BNB confirmed: %s USDT → ~%s BNB (tx: %s)",
		usdtSwapAmount.String(), bnbMinOut.String(), swapTxHash)

	// Step 2: Swap ALL the resulting BNB → $Ensoul and split between mining + revenue pool.
	// The BNB from step 1 already represents (buyback+pool)%, so swap 100% of it.
	return swapAndDistribute(ctx, buybackKey, bnbMinOut,
		SubscriptionRevenueBuybackPct, SubscriptionRevenuePoolPct,
		"subscription_revenue")
}

// processRevenueSplit is the unified 3-way revenue split for BNB-denominated income.
// swapPct = buybackPct + poolPct (the portion to swap to $Ensoul).
// retainPct = the BNB portion to forward to Treasury cold wallet.
func processRevenueSplit(bnbTotal *big.Int, swapPct int64,
	buybackPct, poolPct, retainPct int, source string) error {

	// Calculate swap portion (buyback + pool combined for better swap price)
	swapAmount := new(big.Int).Mul(bnbTotal, big.NewInt(swapPct))
	swapAmount.Div(swapAmount, big.NewInt(100))

	// Calculate Treasury portion
	retainAmount := new(big.Int).Mul(bnbTotal, big.NewInt(int64(retainPct)))
	retainAmount.Div(retainAmount, big.NewInt(100))

	if swapAmount.Sign() <= 0 {
		util.Log.Info("[buyback] Revenue too small for swap, skipping (source: %s)", source)
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	buybackKey, err := chain.ParsePrivateKey(config.Cfg.BuybackPrivateKey)
	if err != nil {
		return fmt.Errorf("failed to parse buyback wallet key: %w", err)
	}

	// --- Part 1: Swap BNB → $Ensoul and distribute to Mining Pool + Revenue Pool ---
	if err := swapAndDistribute(ctx, buybackKey, swapAmount,
		buybackPct, poolPct, source); err != nil {
		return err
	}

	// --- Part 2: Transfer retain% BNB → Treasury cold wallet ---
	if retainAmount.Sign() > 0 && config.Cfg.TreasuryAddr != "" {
		if err := transferToTreasury(ctx, buybackKey, retainAmount, source); err != nil {
			// Log but don't fail — the swap already succeeded.
			// BNB stays in Buyback wallet and can be manually forwarded.
			util.Log.Error("[buyback] Treasury transfer failed (BNB stays in buyback wallet): %v", err)
		}
	}

	return nil
}

// swapAndDistribute swaps BNB → $Ensoul via PancakeSwap and splits the output
// between Mining Pool wallet (buybackPct share) and Revenue Pool wallet (poolPct share).
//
// Example for mint: buybackPct=60, poolPct=10 → mining gets 6/7, revenue gets 1/7 of $Ensoul.
func swapAndDistribute(ctx context.Context, buybackKey *ecdsa.PrivateKey,
	bnbAmountWei *big.Int, buybackPct, poolPct int, source string) error {

	// Get swap quote for logging
	quote, err := chain.GetSwapQuote(ctx, bnbAmountWei)
	if err != nil {
		util.Log.Warn("[buyback] Failed to get swap quote: %v", err)
	} else {
		util.Log.Info("[buyback] Swap quote: %s BNB → ~%s $Ensoul", bnbAmountWei.String(), quote.String())
	}

	// Record $Ensoul balance BEFORE swap, so we can compute actual received amount
	buybackAddr := chain.AddressFromKey(buybackKey).Hex()
	balanceBefore, err := chain.GetTokenBalance(ctx, buybackAddr)
	if err != nil {
		util.Log.Warn("[buyback] Failed to get pre-swap balance, will fall back to minOut: %v", err)
		balanceBefore = nil
	}

	// Execute BNB → $Ensoul swap. Output tokens land in Buyback wallet (fromAddr).
	txHash, minOut, err := chain.SwapBNBForToken(ctx, buybackKey, bnbAmountWei, SwapSlippageBps)
	if err != nil {
		return fmt.Errorf("[buyback] swap failed: %w", err)
	}

	// Wait for confirmation
	success, err := chain.WaitForTokenTx(ctx, txHash)
	if err != nil || !success {
		util.Log.Error("[buyback] Swap tx failed: %s, err: %v", txHash, err)
		record := &models.BuybackRecord{
			Source:     source,
			BNBAmount:  weiToFloat(bnbAmountWei),
			SwapTxHash: txHash,
		}
		database.DB.Create(record)
		return fmt.Errorf("buyback swap tx failed: %s", txHash)
	}

	// Determine actual $Ensoul received: prefer (balanceAfter - balanceBefore) over minOut
	totalTokenWei := minOut // fallback to slippage-adjusted minimum
	if balanceBefore != nil {
		balanceAfter, err := chain.GetTokenBalance(ctx, buybackAddr)
		if err == nil && balanceAfter != nil {
			actualReceived := new(big.Int).Sub(balanceAfter, balanceBefore)
			if actualReceived.Sign() > 0 {
				util.Log.Info("[buyback] Actual swap output: %s $Ensoul (minOut was %s, delta: +%s)",
					actualReceived.String(), minOut.String(),
					new(big.Int).Sub(actualReceived, minOut).String())
				totalTokenWei = actualReceived
			}
		}
	}

	// Calculate split: mining share and revenue pool share
	totalParts := int64(buybackPct + poolPct)
	miningTokenWei := new(big.Int).Mul(totalTokenWei, big.NewInt(int64(buybackPct)))
	miningTokenWei.Div(miningTokenWei, big.NewInt(totalParts))

	revenueTokenWei := new(big.Int).Sub(totalTokenWei, miningTokenWei) // remainder → revenue pool

	util.Log.Info("[buyback] Swap confirmed: %s BNB → %s $Ensoul (mining: %s, revenue: %s), tx=%s",
		bnbAmountWei.String(), totalTokenWei.String(),
		miningTokenWei.String(), revenueTokenWei.String(), txHash)

	// --- Transfer $Ensoul: Buyback Wallet → Mining Pool Wallet ---
	if miningTokenWei.Sign() > 0 && config.Cfg.MiningPoolPrivateKey != "" {
		miningPoolAddr := getWalletAddr(config.Cfg.MiningPoolPrivateKey)
		if miningPoolAddr != "" {
			mTxHash, err := chain.TransferToken(ctx, buybackKey, miningPoolAddr, miningTokenWei)
			if err != nil {
				util.Log.Error("[buyback] Transfer to Mining Pool failed: %v", err)
				// $Ensoul stays in Buyback wallet — can be manually transferred
			} else {
				mSuccess, mErr := chain.WaitForTokenTx(ctx, mTxHash)
				if mErr != nil || !mSuccess {
					util.Log.Error("[buyback] Mining Pool transfer tx failed: %s, err: %v", mTxHash, mErr)
				} else {
					util.Log.Info("[buyback] Transferred %s $Ensoul to Mining Pool, tx=%s",
						miningTokenWei.String(), mTxHash)
				}
			}
		}
	}

	// Record mining pool deposit in DB (for daily release calculation)
	miningFloat := weiToFloat(miningTokenWei)
	if err := DepositToPool(miningFloat, "buyback_"+source); err != nil {
		util.Log.Error("[buyback] Failed to record mining pool deposit: %v", err)
	}

	// --- Transfer $Ensoul: Buyback Wallet → Revenue Pool Wallet ---
	if revenueTokenWei.Sign() > 0 && config.Cfg.RevenuePoolPrivateKey != "" {
		revenuePoolAddr := getWalletAddr(config.Cfg.RevenuePoolPrivateKey)
		if revenuePoolAddr != "" {
			rTxHash, err := chain.TransferToken(ctx, buybackKey, revenuePoolAddr, revenueTokenWei)
			if err != nil {
				util.Log.Error("[buyback] Transfer to Revenue Pool failed: %v", err)
			} else {
				rSuccess, rErr := chain.WaitForTokenTx(ctx, rTxHash)
				if rErr != nil || !rSuccess {
					util.Log.Error("[buyback] Revenue Pool transfer tx failed: %s, err: %v", rTxHash, rErr)
				} else {
					util.Log.Info("[buyback] Transferred %s $Ensoul to Revenue Pool, tx=%s",
						revenueTokenWei.String(), rTxHash)
				}
			}
		}
	}

	// Record revenue pool deposit in DB (in $Ensoul terms, not BNB)
	revenueFloat := weiToFloat(revenueTokenWei)
	AddToRevenuePool(revenueFloat)
	util.Log.Info("[buyback] Revenue pool: recorded %.4f $Ensoul", revenueFloat)

	// Record buyback record
	record := &models.BuybackRecord{
		Source:      source,
		BNBAmount:   weiToFloat(bnbAmountWei),
		TokenAmount: weiToFloat(totalTokenWei),
		SwapTxHash:  txHash,
	}
	if err := database.DB.Create(record).Error; err != nil {
		util.Log.Error("[buyback] Failed to save buyback record: %v", err)
	}

	return nil
}

// transferToTreasury sends native BNB from Buyback wallet to the Treasury cold wallet.
func transferToTreasury(ctx context.Context, buybackKey *ecdsa.PrivateKey,
	bnbAmountWei *big.Int, source string) error {

	txHash, err := chain.TransferBNB(ctx, buybackKey, config.Cfg.TreasuryAddr, bnbAmountWei)
	if err != nil {
		return fmt.Errorf("BNB transfer to Treasury failed: %w", err)
	}

	success, err := chain.WaitForTokenTx(ctx, txHash)
	if err != nil || !success {
		return fmt.Errorf("Treasury transfer tx failed: %s, err: %v", txHash, err)
	}

	util.Log.Info("[buyback] Transferred %s BNB to Treasury %s (source: %s, tx: %s)",
		bnbAmountWei.String(), config.Cfg.TreasuryAddr, source, txHash)
	return nil
}

// getWalletAddr derives the wallet address from a hex-encoded private key.
func getWalletAddr(privateKeyHex string) string {
	key, err := chain.ParsePrivateKey(privateKeyHex)
	if err != nil {
		util.Log.Error("[buyback] Failed to parse private key for address derivation: %v", err)
		return ""
	}
	return chain.AddressFromKey(key).Hex()
}

// weiToFloat converts a *big.Int wei value to float64 token amount (18 decimals).
func weiToFloat(wei *big.Int) float64 {
	if wei == nil {
		return 0
	}
	f := new(big.Float).SetInt(wei)
	divisor := new(big.Float).SetFloat64(1e18)
	result, _ := new(big.Float).Quo(f, divisor).Float64()
	return result
}

// GetBuybackHistory returns recent buyback records.
func GetBuybackHistory(limit int) ([]models.BuybackRecord, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	var records []models.BuybackRecord
	if err := database.DB.Order("created_at DESC").Limit(limit).Find(&records).Error; err != nil {
		return nil, err
	}
	return records, nil
}

// ProcessMintRevenueAsync triggers mint revenue processing in a background goroutine.
// Called after a user or tax-wallet mint is confirmed on-chain.
// Safe to call from request handlers — won't block the response.
func ProcessMintRevenueAsync(bnbAmountWei *big.Int) {
	if config.Cfg.BuybackPrivateKey == "" {
		util.Log.Debug("[buyback] BUYBACK_PRIVATE_KEY not configured, skipping mint revenue")
		return
	}
	if bnbAmountWei == nil || bnbAmountWei.Sign() <= 0 {
		return
	}
	amount := new(big.Int).Set(bnbAmountWei)
	go func() {
		if err := ProcessMintRevenue(amount); err != nil {
			util.Log.Error("[buyback] Mint revenue processing failed: %v", err)
		}
	}()
}

// ProcessSubscriptionRevenueAsync triggers USDT subscription revenue processing
// in a background goroutine. Called after a subscription payment is verified.
func ProcessSubscriptionRevenueAsync(paymentAmountWei *big.Int) {
	if config.Cfg.BuybackPrivateKey == "" {
		util.Log.Debug("[buyback] BUYBACK_PRIVATE_KEY not configured, skipping subscription revenue")
		return
	}
	if paymentAmountWei == nil || paymentAmountWei.Sign() <= 0 {
		return
	}
	amount := new(big.Int).Set(paymentAmountWei)
	go func() {
		if err := ProcessSubscriptionRevenue(amount); err != nil {
			util.Log.Error("[buyback] Subscription revenue processing failed: %v", err)
		}
	}()
}

// ProcessBNBSubscriptionRevenueAsync handles BNB-denominated subscription payments.
// Same 40/10/50 split as USDT subscriptions, but skips the USDT→BNB conversion step.
func ProcessBNBSubscriptionRevenueAsync(bnbAmountWei *big.Int) {
	if config.Cfg.BuybackPrivateKey == "" {
		util.Log.Debug("[buyback] BUYBACK_PRIVATE_KEY not configured, skipping subscription revenue")
		return
	}
	if bnbAmountWei == nil || bnbAmountWei.Sign() <= 0 {
		return
	}
	amount := new(big.Int).Set(bnbAmountWei)
	go func() {
		if err := ProcessBNBSubscriptionRevenue(amount); err != nil {
			util.Log.Error("[buyback] BNB subscription revenue processing failed: %v", err)
		}
	}()
}
