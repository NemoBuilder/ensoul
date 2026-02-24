package services

import (
	"context"
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
	// MintRevenueBuybackPct — percentage of mint BNB used for buyback
	MintRevenueBuybackPct = 60
	// MintRevenuePoolPct — percentage of mint BNB added to holder revenue pool
	MintRevenuePoolPct = 10
	// MintRevenueRetainPct — percentage of mint BNB retained by Treasury
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
// 60% swapped to $Ensoul → mining pool, 10% → holder revenue pool, 30% retained in Treasury.
func ProcessMintRevenue(bnbAmountWei *big.Int) error {
	if bnbAmountWei == nil || bnbAmountWei.Sign() <= 0 {
		return fmt.Errorf("invalid BNB amount")
	}

	// Calculate 60% for buyback
	buybackAmount := new(big.Int).Mul(bnbAmountWei, big.NewInt(MintRevenueBuybackPct))
	buybackAmount.Div(buybackAmount, big.NewInt(100))

	if buybackAmount.Sign() <= 0 {
		util.Log.Info("[buyback] Mint revenue too small for buyback, skipping")
		return nil
	}

	// Feed 10% into holder revenue pool
	totalFloat := weiToFloat(bnbAmountWei)
	poolAmount := totalFloat * float64(MintRevenuePoolPct) / 100.0
	AddToRevenuePool(poolAmount)
	util.Log.Info("[buyback] Mint revenue: added %.4f to revenue pool (10%%)", poolAmount)

	return executeBuyback(buybackAmount, "mint_revenue")
}

// ProcessSubscriptionRevenue handles USDT received from subscriptions.
// 40% converted: USDT → BNB via PancakeSwap, then BNB → $Ensoul, deposited to mining pool.
// 10% to Revenue Pool (holder revenue), 50% retained in Treasury.
func ProcessSubscriptionRevenue(usdAmountWei *big.Int) error {
	if usdAmountWei == nil || usdAmountWei.Sign() <= 0 {
		return fmt.Errorf("invalid USD amount")
	}

	// Calculate 40% for buyback
	buybackAmount := new(big.Int).Mul(usdAmountWei, big.NewInt(SubscriptionRevenueBuybackPct))
	buybackAmount.Div(buybackAmount, big.NewInt(100))

	if buybackAmount.Sign() <= 0 {
		util.Log.Info("[buyback] Subscription revenue too small for buyback, skipping")
		return nil
	}

	util.Log.Info("[buyback] Processing subscription revenue: %s USDT total, %s USDT (40%%) for buyback",
		usdAmountWei.String(), buybackAmount.String())

	// Step 1: Swap USDT → BNB
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	buybackKey, err := chain.ParsePrivateKey(config.Cfg.BuybackPrivateKey)
	if err != nil {
		return fmt.Errorf("failed to parse buyback wallet key: %w", err)
	}

	swapTxHash, bnbMinOut, err := chain.SwapUSDTForBNB(ctx, buybackKey, buybackAmount, SwapSlippageBps)
	if err != nil {
		return fmt.Errorf("USDT→BNB swap failed: %w", err)
	}

	// Wait for USDT→BNB confirmation
	success, err := chain.WaitForTokenTx(ctx, swapTxHash)
	if err != nil || !success {
		util.Log.Error("[buyback] USDT→BNB swap tx failed: %s, err: %v", swapTxHash, err)
		return fmt.Errorf("USDT→BNB swap tx failed: %s", swapTxHash)
	}

	util.Log.Info("[buyback] USDT→BNB swap confirmed: %s USDT → ~%s BNB (tx: %s)",
		buybackAmount.String(), bnbMinOut.String(), swapTxHash)

	// Step 2: Swap BNB → $Ensoul (reuse existing path)
	return executeBuyback(bnbMinOut, "subscription_revenue")
}

// executeBuyback performs the actual BNB → $Ensoul swap and deposits to mining pool.
func executeBuyback(bnbAmountWei *big.Int, source string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	buybackKey, err := chain.ParsePrivateKey(config.Cfg.BuybackPrivateKey)
	if err != nil {
		return fmt.Errorf("failed to parse buyback wallet key: %w", err)
	}

	// Get swap quote for logging
	quote, err := chain.GetSwapQuote(ctx, bnbAmountWei)
	if err != nil {
		util.Log.Warn("[buyback] Failed to get swap quote: %v", err)
		// Continue anyway — the swap will use on-chain slippage protection
	} else {
		util.Log.Info("[buyback] Swap quote: %s BNB → ~%s $Ensoul", bnbAmountWei.String(), quote.String())
	}

	// Execute swap
	txHash, minOut, err := chain.SwapBNBForToken(ctx, buybackKey, bnbAmountWei, SwapSlippageBps)
	if err != nil {
		return fmt.Errorf("buyback swap failed: %w", err)
	}

	// Wait for confirmation
	success, err := chain.WaitForTokenTx(ctx, txHash)
	if err != nil || !success {
		util.Log.Error("[buyback] Swap tx failed: %s, err: %v", txHash, err)
		// Record failed buyback
		record := &models.BuybackRecord{
			Source:     source,
			BNBAmount:  weiToFloat(bnbAmountWei),
			SwapTxHash: txHash,
		}
		database.DB.Create(record)
		return fmt.Errorf("buyback swap tx failed: %s", txHash)
	}

	tokenAmount := weiToFloat(minOut)

	// Record successful buyback
	record := &models.BuybackRecord{
		Source:      source,
		BNBAmount:   weiToFloat(bnbAmountWei),
		TokenAmount: tokenAmount,
		SwapTxHash:  txHash,
	}
	if err := database.DB.Create(record).Error; err != nil {
		util.Log.Error("[buyback] Failed to save buyback record: %v", err)
	}

	// Deposit swapped $Ensoul to mining pool
	if err := DepositToPool(tokenAmount, "buyback_"+source); err != nil {
		util.Log.Error("[buyback] Failed to deposit to mining pool: %v", err)
		return err
	}

	util.Log.Info("[buyback] Completed: %s BNB → %.4f $Ensoul (source: %s, tx: %s)",
		bnbAmountWei.String(), tokenAmount, source, txHash)
	return nil
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

// ProcessMintRevenueAsync triggers mint revenue buyback in a background goroutine.
// Called after a user or tax-wallet mint is confirmed on-chain.
// Safe to call from request handlers — won't block the response.
func ProcessMintRevenueAsync(bnbAmountWei *big.Int) {
	if config.Cfg.BuybackPrivateKey == "" {
		util.Log.Debug("[buyback] BUYBACK_PRIVATE_KEY not configured, skipping mint revenue buyback")
		return
	}
	if bnbAmountWei == nil || bnbAmountWei.Sign() <= 0 {
		return
	}
	// Copy to avoid race conditions on the big.Int pointer
	amount := new(big.Int).Set(bnbAmountWei)
	go func() {
		if err := ProcessMintRevenue(amount); err != nil {
			util.Log.Error("[buyback] Mint revenue buyback failed: %v", err)
		}
	}()
}

// ProcessSubscriptionRevenueAsync triggers subscription revenue buyback + revenue pool deposit
// in a background goroutine. Called after a subscription payment is verified.
func ProcessSubscriptionRevenueAsync(paymentAmountWei *big.Int) {
	if config.Cfg.BuybackPrivateKey == "" {
		util.Log.Debug("[buyback] BUYBACK_PRIVATE_KEY not configured, skipping subscription buyback")
		return
	}
	if paymentAmountWei == nil || paymentAmountWei.Sign() <= 0 {
		return
	}
	amount := new(big.Int).Set(paymentAmountWei)
	go func() {
		// 1. Feed 40% into buyback (USDT → BNB → $Ensoul → mining pool)
		if err := ProcessSubscriptionRevenue(amount); err != nil {
			util.Log.Error("[buyback] Subscription revenue buyback failed: %v", err)
		}

		// 2. Feed 10% into monthly holder revenue pool
		totalFloat := weiToFloat(amount)
		poolAmount := totalFloat * float64(SubscriptionRevenuePoolPct) / 100.0
		AddToRevenuePool(poolAmount)
		util.Log.Info("[buyback] Subscription revenue: added %.4f to revenue pool (10%%)", poolAmount)
	}()
}

// ProcessBNBSubscriptionRevenueAsync handles BNB-denominated subscription payments.
// Same 40/10/50 split as USDT subscriptions, but skips the USDT→BNB conversion step.
func ProcessBNBSubscriptionRevenueAsync(bnbAmountWei *big.Int) {
	if config.Cfg.BuybackPrivateKey == "" {
		util.Log.Debug("[buyback] BUYBACK_PRIVATE_KEY not configured, skipping subscription buyback")
		return
	}
	if bnbAmountWei == nil || bnbAmountWei.Sign() <= 0 {
		return
	}
	amount := new(big.Int).Set(bnbAmountWei)
	go func() {
		// 1. Take 40% BNB → directly swap to $Ensoul → mining pool
		buybackAmount := new(big.Int).Mul(amount, big.NewInt(SubscriptionRevenueBuybackPct))
		buybackAmount.Div(buybackAmount, big.NewInt(100))

		if buybackAmount.Sign() > 0 {
			if err := executeBuyback(buybackAmount, "subscription_revenue_bnb"); err != nil {
				util.Log.Error("[buyback] BNB subscription buyback failed: %v", err)
			}
		}

		// 2. Feed 10% into monthly holder revenue pool
		totalFloat := weiToFloat(amount)
		poolAmount := totalFloat * float64(SubscriptionRevenuePoolPct) / 100.0
		AddToRevenuePool(poolAmount)
		util.Log.Info("[buyback] BNB subscription revenue: added %.4f to revenue pool (10%%)", poolAmount)
	}()
}
