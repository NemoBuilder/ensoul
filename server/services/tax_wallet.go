package services

import (
	"context"
	"crypto/ecdsa"
	"encoding/base64"
	"encoding/json"
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
	// TaxMintMaxPct — max percentage of tax wallet BNB balance to spend per cycle
	TaxMintMaxPct = 80
	// TaxMintBatchSize — max public souls to mint per cycle
	TaxMintBatchSize = 5
	// TaxMintInterval — how often the scheduler polls for candidates (30 seconds)
	TaxMintInterval = 30 * time.Second
)

// AutoMintPublicSouls evaluates and mints public Soul NFTs using the Tax Wallet.
// Called every 30 seconds. Uses pre-stored follower counts and prices from the
// MintCandidate table. Pre-checks balance vs price before attempting on-chain mint.
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

	if maxSpend.Sign() <= 0 {
		return // silent — don't spam logs every 30 seconds
	}

	// Find candidate handles with stored follower/price data
	candidates := findMintCandidatesWithPrice(ctx, TaxMintBatchSize)
	if len(candidates) == 0 {
		return // silent — no spam
	}

	util.Log.Info("[tax_wallet] Balance: %s wei, max spend: %s wei, %d candidates to evaluate",
		balance.String(), maxSpend.String(), len(candidates))

	totalSpent := new(big.Int)
	minted := 0

	for _, cand := range candidates {
		// Check budget
		remaining := new(big.Int).Sub(maxSpend, totalSpent)
		if remaining.Sign() <= 0 {
			util.Log.Info("[tax_wallet] Budget exhausted after %d mints", minted)
			break
		}

		// Use stored price from candidate record
		price := new(big.Int)
		if _, ok := price.SetString(cand.PriceWei, 10); !ok || price.Sign() <= 0 {
			// Price not stored or invalid — fetch fresh
			util.Log.Warn("[tax_wallet] Invalid stored price for @%s, fetching fresh", cand.Handle)
			freshPrice, followers, tier, fetchErr := GetMintPriceForHandle(cand.Handle)
			if fetchErr != nil {
				markCandidateStatus(cand.Handle, models.CandidateStatusSkipped, "fetch profile failed: "+fetchErr.Error())
				continue
			}
			price = freshPrice
			// Update stored data
			database.DB.Model(&models.MintCandidate{}).
				Where("id = ?", cand.ID).
				Updates(map[string]interface{}{
					"followers": followers,
					"price_wei": price.String(),
					"tier":      tier,
				})
		}

		// Pre-check: is the price within our remaining budget?
		if new(big.Int).Add(totalSpent, price).Cmp(maxSpend) > 0 {
			util.Log.Info("[tax_wallet] Skipping @%s (price %s exceeds remaining budget %s)",
				cand.Handle, price.String(), remaining.String())
			continue // don't mark — will retry next cycle
		}

		// Pre-check: does the actual balance cover this mint?
		actualRemaining := new(big.Int).Sub(balance, totalSpent)
		if price.Cmp(actualRemaining) > 0 {
			util.Log.Info("[tax_wallet] Insufficient balance for @%s (need %s, have %s)",
				cand.Handle, price.String(), actualRemaining.String())
			continue // don't mark — will retry when balance increases
		}

		// Generate seed preview (reuse existing flow)
		preview, err := GenerateSeedPreview(cand.Handle)
		if err != nil {
			util.Log.Warn("[tax_wallet] Failed to generate preview for @%s: %v", cand.Handle, err)
			markCandidateStatus(cand.Handle, models.CandidateStatusFailed, "preview failed: "+err.Error())
			continue
		}

		// Mint using the tax wallet as owner
		shell, err := MintShell(cand.Handle, taxAddr.Hex(), preview)
		if err != nil {
			util.Log.Warn("[tax_wallet] Failed to create shell for @%s: %v", cand.Handle, err)
			markCandidateStatus(cand.Handle, models.CandidateStatusFailed, "create shell failed: "+err.Error())
			continue
		}

		// Call EnsoulMinterV2 on-chain (with full EIP-8004 agentURI including avatar)
		txHash, err := mintOnChainViaTaxWallet(ctx, taxKey, cand.Handle, price, preview)
		if err != nil {
			util.Log.Error("[tax_wallet] On-chain mint failed for @%s: %v", cand.Handle, err)
			HardDeleteShell(shell.ID)
			markCandidateStatus(cand.Handle, models.CandidateStatusFailed, "on-chain mint failed: "+err.Error())
			continue
		}

		// Confirm the mint
		if err := ConfirmMint(cand.Handle, txHash, 0, taxAddr.Hex()); err != nil {
			util.Log.Error("[tax_wallet] Failed to confirm mint for @%s: %v", cand.Handle, err)
			markCandidateStatus(cand.Handle, models.CandidateStatusFailed, "confirm failed: "+err.Error())
			continue
		}

		// Record as public soul
		publicSoul := &models.PublicSoul{
			ShellID:  shell.ID,
			MintCost: weiToFloat(price),
			Status:   "minted",
		}
		database.DB.Create(publicSoul)

		// Mark candidate as minted
		markCandidateStatus(cand.Handle, models.CandidateStatusMinted, "")

		totalSpent.Add(totalSpent, price)
		minted++

		util.Log.Info("[tax_wallet] Auto-minted public Soul @%s (cost: %s wei, tx: %s)",
			cand.Handle, price.String(), txHash)
	}

	if minted > 0 {
		util.Log.Info("[tax_wallet] Cycle complete: %d souls minted, %s wei spent",
			minted, totalSpent.String())
	}
}

// mintOnChainViaTaxWallet calls EnsoulMinterV2.mint() with the tax wallet.
// This reuses the permit-based flow: backend signs a permit for the tax wallet address.
// preview is used to build the full EIP-8004 agentURI (with avatar, description, etc.).
func mintOnChainViaTaxWallet(ctx context.Context, taxKey *ecdsa.PrivateKey, handle string, priceWei *big.Int, preview *SeedPreview) (string, error) {
	taxAddr := chain.AddressFromKey(taxKey)

	// Generate a permit for the tax wallet
	deadline := time.Now().Unix() + 1800 // 30 minutes
	nonce := uint64(time.Now().UnixNano())

	permit, err := chain.SignMintPermit(handle, priceWei, taxAddr, deadline, nonce)
	if err != nil {
		return "", fmt.Errorf("failed to sign permit: %w", err)
	}

	// Build the full EIP-8004 agentURI (same format as the frontend)
	agentURI := buildAgentURI(handle, preview)

	// Call EnsoulMinterV2.mint() from the tax wallet
	// The tax wallet sends BNB (price) to the minter contract
	txHash, err := chain.CallMintWithPermit(ctx, taxKey, handle, priceWei, permit, agentURI)
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

// findMintCandidatesWithPrice returns pending candidates with stored follower/price data.
// Ordered by priority DESC, created_at ASC (highest priority first, FIFO within same priority).
func findMintCandidatesWithPrice(_ context.Context, limit int) []models.MintCandidate {
	var candidates []models.MintCandidate
	database.DB.Where("status = ?", models.CandidateStatusPending).
		Order("priority DESC, created_at ASC").
		Limit(limit).
		Find(&candidates)

	return candidates
}

// markCandidateStatus updates the status and error message for a candidate.
func markCandidateStatus(handle, status, errMsg string) {
	database.DB.Model(&models.MintCandidate{}).
		Where("LOWER(handle) = LOWER(?)", handle).
		Updates(map[string]interface{}{
			"status":    status,
			"error_msg": errMsg,
		})
}

// MintSinglePublicSoul mints a single handle using the tax wallet.
// Called by admin API for immediate mint of a specific handle.
// Pre-checks balance vs price before attempting on-chain mint.
func MintSinglePublicSoul(handle string) {
	handle = strings.ToLower(strings.TrimSpace(handle))
	util.Log.Info("[tax_wallet] Manual mint triggered for @%s", handle)

	if config.Cfg.TaxWalletPrivateKey == "" {
		util.Log.Error("[tax_wallet] TAX_WALLET_PRIVATE_KEY not configured")
		markCandidateStatus(handle, models.CandidateStatusFailed, "tax wallet not configured")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	taxKey, err := chain.ParsePrivateKey(config.Cfg.TaxWalletPrivateKey)
	if err != nil {
		util.Log.Error("[tax_wallet] Failed to parse tax wallet key: %v", err)
		markCandidateStatus(handle, models.CandidateStatusFailed, "parse key: "+err.Error())
		return
	}
	taxAddr := chain.AddressFromKey(taxKey)

	// Try to use stored price from candidate record first
	var candidate models.MintCandidate
	hasCandidate := database.DB.Where("LOWER(handle) = ?", handle).First(&candidate).Error == nil

	var price *big.Int
	var followers int

	if hasCandidate && candidate.PriceWei != "" && candidate.PriceWei != "0" {
		price = new(big.Int)
		if _, ok := price.SetString(candidate.PriceWei, 10); !ok {
			price = nil // fallback to fresh fetch
		} else {
			followers = candidate.Followers
		}
	}

	// Fallback: fetch fresh profile for pricing
	if price == nil || price.Sign() <= 0 {
		profile, err := FetchTwitterProfile(handle)
		if err != nil {
			util.Log.Error("[tax_wallet] Failed to fetch profile for @%s: %v", handle, err)
			markCandidateStatus(handle, models.CandidateStatusFailed, "fetch profile: "+err.Error())
			return
		}
		followers = profile.User.PublicMetrics.FollowersCount
		price = chain.GetMintPrice(followers)
	}

	// Pre-check balance
	balance, err := chain.GetBNBBalance(ctx, taxAddr.Hex())
	if err != nil {
		util.Log.Error("[tax_wallet] Failed to get balance: %v", err)
		markCandidateStatus(handle, models.CandidateStatusFailed, "get balance: "+err.Error())
		return
	}
	if price.Cmp(balance) > 0 {
		util.Log.Error("[tax_wallet] Insufficient balance for @%s (need %s, have %s)",
			handle, price.String(), balance.String())
		markCandidateStatus(handle, models.CandidateStatusFailed,
			fmt.Sprintf("insufficient balance: need %s, have %s", price.String(), balance.String()))
		return
	}

	// Generate seed preview
	preview, err := GenerateSeedPreview(handle)
	if err != nil {
		util.Log.Error("[tax_wallet] Failed to generate preview for @%s: %v", handle, err)
		markCandidateStatus(handle, models.CandidateStatusFailed, "preview: "+err.Error())
		return
	}

	// Create shell in DB
	shell, err := MintShell(handle, taxAddr.Hex(), preview)
	if err != nil {
		util.Log.Error("[tax_wallet] Failed to create shell for @%s: %v", handle, err)
		markCandidateStatus(handle, models.CandidateStatusFailed, "create shell: "+err.Error())
		return
	}

	// On-chain mint (with full EIP-8004 agentURI including avatar)
	txHash, err := mintOnChainViaTaxWallet(ctx, taxKey, handle, price, preview)
	if err != nil {
		util.Log.Error("[tax_wallet] On-chain mint failed for @%s: %v", handle, err)
		HardDeleteShell(shell.ID)
		markCandidateStatus(handle, models.CandidateStatusFailed, "on-chain: "+err.Error())
		return
	}

	// Confirm
	if err := ConfirmMint(handle, txHash, 0, taxAddr.Hex()); err != nil {
		util.Log.Error("[tax_wallet] Failed to confirm mint for @%s: %v", handle, err)
		markCandidateStatus(handle, models.CandidateStatusFailed, "confirm: "+err.Error())
		return
	}

	// Record public soul
	publicSoul := &models.PublicSoul{
		ShellID:  shell.ID,
		MintCost: weiToFloat(price),
		Status:   "minted",
	}
	database.DB.Create(publicSoul)

	markCandidateStatus(handle, models.CandidateStatusMinted, "")

	util.Log.Info("[tax_wallet] Manual mint completed: @%s (followers: %d, cost: %s wei, tx: %s)",
		handle, followers, price.String(), txHash)
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

// StartTaxWalletScheduler starts the auto-mint scheduler.
// Polls every 30 seconds for pending candidates and attempts mint if balance is sufficient.
func StartTaxWalletScheduler() {
	go func() {
		ticker := time.NewTicker(TaxMintInterval)
		defer ticker.Stop()

		// Run once at startup after a short delay (let chain client initialize)
		time.Sleep(5 * time.Second)
		AutoMintPublicSouls()

		for range ticker.C {
			AutoMintPublicSouls()
		}
	}()
	util.Log.Info("[tax_wallet] Auto-mint scheduler started (polls every %s)", TaxMintInterval)
}

// buildAgentURI constructs a full EIP-8004 registration file as a data URI,
// matching the format used by the frontend mint page (includes avatar, description, etc.).
func buildAgentURI(handle string, preview *SeedPreview) string {
	description := ""
	avatarURL := ""
	if preview != nil {
		description = preview.SeedSummary
		avatarURL = preview.AvatarURL
	}

	regFile := map[string]interface{}{
		"type":        "https://eips.ethereum.org/EIPS/eip-8004#registration-v1",
		"name":        fmt.Sprintf("@%s · Ensoul", handle),
		"description": description,
		"image":       avatarURL,
		"services": []map[string]string{
			{"name": "web", "endpoint": fmt.Sprintf("https://ensoul.ac/soul/%s", handle)},
			{"name": "chat", "endpoint": fmt.Sprintf("https://ensoul.ac/soul/%s/chat", handle)},
		},
		"active": true,
		"ensoul": map[string]interface{}{
			"handle":     handle,
			"stage":      "embryo",
			"dnaVersion": 1,
		},
	}

	jsonBytes, err := json.Marshal(regFile)
	if err != nil {
		// Fallback to simple URI if JSON marshaling fails
		util.Log.Warn("[tax_wallet] Failed to build agentURI JSON for @%s: %v, using fallback", handle, err)
		return "ensoul://soul/" + strings.ToLower(handle)
	}

	b64 := base64.StdEncoding.EncodeToString(jsonBytes)
	return "data:application/json;base64," + b64
}
