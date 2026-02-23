package services

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ensoul-labs/ensoul-server/chain"
	"github.com/ensoul-labs/ensoul-server/config"
	"github.com/ensoul-labs/ensoul-server/database"
	"github.com/ensoul-labs/ensoul-server/models"
	"github.com/ensoul-labs/ensoul-server/util"
)

const (
	// TaxMintMaxPct — max percentage of tax wallet BNB balance to spend per week
	TaxMintMaxPct = 30
	// TaxMintMinFollowers — minimum followers for auto-mint candidates
	TaxMintMinFollowers = 10000
	// TaxMintMaxFollowers — maximum followers for auto-mint candidates (cost control)
	TaxMintMaxFollowers = 1000000
	// TaxMintBatchSize — max public souls to mint per weekly run
	TaxMintBatchSize = 5
)

// AutoMintPublicSouls evaluates and mints public Soul NFTs using the Tax Wallet.
// Called weekly. Prioritizes 10K-1M follower KOLs. Caps spending at 30% of balance.
func AutoMintPublicSouls() {
	if config.Cfg.TaxWalletPrivateKey == "" {
		util.Log.Warn("[tax_wallet] TAX_WALLET_PRIVATE_KEY not configured, skipping auto-mint")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// Get tax wallet BNB balance
	taxKey, err := chain.ParsePrivateKey(config.Cfg.TaxWalletPrivateKey)
	if err != nil {
		util.Log.Error("[tax_wallet] Failed to parse tax wallet key: %v", err)
		return
	}
	taxAddr := chain.AddressFromKey(taxKey)

	balance, err := chain.GetBNBBalance(ctx, taxAddr.Hex())
	if err != nil {
		util.Log.Error("[tax_wallet] Failed to get BNB balance: %v", err)
		return
	}

	// Calculate max spend (30% of balance)
	maxSpend := new(big.Int).Mul(balance, big.NewInt(TaxMintMaxPct))
	maxSpend.Div(maxSpend, big.NewInt(100))

	util.Log.Info("[tax_wallet] Balance: %s BNB, max spend this week: %s BNB",
		balance.String(), maxSpend.String())

	if maxSpend.Sign() <= 0 {
		util.Log.Info("[tax_wallet] Insufficient balance for auto-mint, skipping")
		return
	}

	// Find candidate handles to mint (not yet minted as shells)
	candidates := findMintCandidates(ctx, TaxMintBatchSize)
	if len(candidates) == 0 {
		util.Log.Info("[tax_wallet] No suitable candidates found for auto-mint")
		return
	}

	totalSpent := new(big.Int)
	minted := 0

	for _, handle := range candidates {
		// Check budget
		remaining := new(big.Int).Sub(maxSpend, totalSpent)
		if remaining.Sign() <= 0 {
			util.Log.Info("[tax_wallet] Weekly budget exhausted after %d mints", minted)
			break
		}

		// Get follower count and price
		profile, err := FetchTwitterProfile(handle)
		if err != nil {
			util.Log.Warn("[tax_wallet] Failed to fetch profile for @%s: %v", handle, err)
			continue
		}

		followers := profile.User.PublicMetrics.FollowersCount
		if followers < TaxMintMinFollowers || followers > TaxMintMaxFollowers {
			continue
		}

		price := chain.GetMintPrice(followers)
		if new(big.Int).Add(totalSpent, price).Cmp(maxSpend) > 0 {
			util.Log.Info("[tax_wallet] Skipping @%s (price %s exceeds remaining budget)", handle, price.String())
			continue
		}

		// Generate seed preview (reuse existing flow)
		preview, err := GenerateSeedPreview(handle)
		if err != nil {
			util.Log.Warn("[tax_wallet] Failed to generate preview for @%s: %v", handle, err)
			continue
		}

		// Mint using the tax wallet as owner (reuse MintShell)
		shell, err := MintShell(handle, taxAddr.Hex(), preview)
		if err != nil {
			util.Log.Warn("[tax_wallet] Failed to create shell for @%s: %v", handle, err)
			continue
		}

		// The on-chain mint would be done via EnsoulMinterV2 contract call.
		// For now, we create the DB record and mark it for on-chain confirmation.
		// The actual contract call requires the tax wallet to send BNB to the minter.
		txHash, err := mintOnChainViaTaxWallet(ctx, taxKey, handle, price)
		if err != nil {
			util.Log.Error("[tax_wallet] On-chain mint failed for @%s: %v", handle, err)
			// Clean up the pending shell
			HardDeleteShell(shell.ID)
			continue
		}

		// Confirm the mint
		if err := ConfirmMint(handle, txHash, 0, taxAddr.Hex()); err != nil {
			util.Log.Error("[tax_wallet] Failed to confirm mint for @%s: %v", handle, err)
			continue
		}

		// Record as public soul
		publicSoul := &models.PublicSoul{
			ShellID:  shell.ID,
			MintCost: weiToFloat(price),
			Status:   "minted",
		}
		database.DB.Create(publicSoul)

		totalSpent.Add(totalSpent, price)
		minted++

		util.Log.Info("[tax_wallet] Auto-minted public Soul @%s (cost: %s BNB, tx: %s)",
			handle, price.String(), txHash)
	}

	util.Log.Info("[tax_wallet] Weekly auto-mint complete: %d souls minted, %s BNB spent",
		minted, totalSpent.String())
}

// mintOnChainViaTaxWallet calls EnsoulMinterV2.mint() with the tax wallet.
// This reuses the permit-based flow: backend signs a permit for the tax wallet address.
func mintOnChainViaTaxWallet(ctx context.Context, taxKey *ecdsa.PrivateKey, handle string, priceWei *big.Int) (string, error) {
	taxAddr := chain.AddressFromKey(taxKey)

	// Generate a permit for the tax wallet
	deadline := time.Now().Unix() + 1800 // 30 minutes
	nonce := uint64(time.Now().UnixNano())

	permit, err := chain.SignMintPermit(handle, priceWei, taxAddr, deadline, nonce)
	if err != nil {
		return "", fmt.Errorf("failed to sign permit: %w", err)
	}

	// Call EnsoulMinterV2.mint() from the tax wallet
	// The tax wallet sends BNB (price) to the minter contract
	txHash, err := chain.CallMintWithPermit(ctx, taxKey, handle, priceWei, permit)
	if err != nil {
		return "", fmt.Errorf("mint contract call failed: %w", err)
	}

	// Wait for confirmation
	success, waitErr := chain.WaitForTokenTx(ctx, txHash)
	if waitErr != nil || !success {
		return txHash, fmt.Errorf("mint tx failed: %s, err: %v", txHash, waitErr)
	}

	return txHash, nil
}

// findMintCandidates returns a list of Twitter handles suitable for public soul minting.
// Looks for popular handles that don't have shells yet.
// In production, this could be driven by a curated list or trending analysis.
func findMintCandidates(ctx context.Context, limit int) []string {
	// Query existing shells to exclude already-minted handles
	var existingHandles []string
	database.DB.Model(&models.Shell{}).Pluck("handle", &existingHandles)

	existing := make(map[string]bool)
	for _, h := range existingHandles {
		existing[strings.ToLower(h)] = true
	}

	// TODO: In production, replace with a curated candidate list or trending KOL discovery.
	// For now, return empty — candidates should be configured or discovered externally.
	// The admin can manually trigger auto-mint for specific handles via API.
	util.Log.Debug("[tax_wallet] Candidate discovery not yet configured (need curated list)")
	return nil
}

// GetTaxWalletBalance returns the BNB balance of the tax wallet.
func GetTaxWalletBalance() (*big.Int, error) {
	if config.Cfg.TaxWalletPrivateKey == "" {
		return nil, fmt.Errorf("tax wallet not configured")
	}

	taxKey, err := chain.ParsePrivateKey(config.Cfg.TaxWalletPrivateKey)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return chain.GetBNBBalance(ctx, chain.AddressFromKey(taxKey).Hex())
}

// StartTaxWalletScheduler starts the weekly auto-mint scheduler.
// Runs every Monday at 00:00 UTC.
func StartTaxWalletScheduler() {
	go func() {
		for {
			now := time.Now().UTC()
			// Find next Monday 00:00 UTC
			daysUntilMonday := (8 - int(now.Weekday())) % 7
			if daysUntilMonday == 0 && now.Hour() > 0 {
				daysUntilMonday = 7
			}
			next := time.Date(now.Year(), now.Month(), now.Day()+daysUntilMonday, 0, 0, 0, 0, time.UTC)
			time.Sleep(time.Until(next))

			util.Log.Info("[tax_wallet] Starting weekly auto-mint evaluation")
			AutoMintPublicSouls()
		}
	}()
	util.Log.Info("[tax_wallet] Weekly auto-mint scheduler started (runs Monday 00:00 UTC)")
}
