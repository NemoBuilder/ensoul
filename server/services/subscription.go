package services

import (
	"fmt"
	"time"

	"github.com/ensoul-labs/ensoul-server/database"
	"github.com/ensoul-labs/ensoul-server/models"
	"github.com/ensoul-labs/ensoul-server/util"
	"github.com/google/uuid"
)

// CreateSubscription creates or renews a subscription after payment verification.
func CreateSubscription(walletAddr, tier, paymentTxHash, paymentToken string, paymentAmount float64) (*models.Subscription, error) {
	tierCfg, ok := models.SubscriptionTiers[tier]
	if !ok {
		return nil, fmt.Errorf("invalid subscription tier: %s", tier)
	}

	// Check for existing active subscription — extend if same tier, reject if different
	var existing models.Subscription
	if err := database.DB.Where("wallet_addr = ? AND status = ?", walletAddr, models.SubStatusActive).
		First(&existing).Error; err == nil {
		if existing.Tier != tier {
			return nil, fmt.Errorf("you already have an active %s subscription; cancel it first or wait for expiry", existing.Tier)
		}
		// Extend existing subscription by 30 days
		existing.ExpiresAt = existing.ExpiresAt.Add(30 * 24 * time.Hour)
		existing.PaymentTxHash = paymentTxHash
		existing.PaymentAmount += paymentAmount
		database.DB.Save(&existing)
		util.Log.Info("[subscription] Extended %s subscription for %s until %s", tier, walletAddr, existing.ExpiresAt)

		// Trigger economic flywheel: buyback + revenue pool
		triggerSubscriptionFlywheel(paymentToken, paymentAmount)

		return &existing, nil
	}

	sub := &models.Subscription{
		WalletAddr:    walletAddr,
		Tier:          tier,
		LLMModel:      tierCfg.DefaultModel,
		Status:        models.SubStatusActive,
		ExpiresAt:     time.Now().UTC().Add(30 * 24 * time.Hour),
		PaymentTxHash: paymentTxHash,
		PaymentToken:  paymentToken,
		PaymentAmount: paymentAmount,
	}

	if err := database.DB.Create(sub).Error; err != nil {
		return nil, fmt.Errorf("failed to create subscription: %w", err)
	}

	util.Log.Info("[subscription] Created %s subscription for %s (expires: %s)", tier, walletAddr, sub.ExpiresAt)

	// Trigger economic flywheel: buyback + revenue pool
	triggerSubscriptionFlywheel(paymentToken, paymentAmount)

	return sub, nil
}

// triggerSubscriptionFlywheel routes subscription payment into the economic flywheel.
// All subscription revenue follows the same split regardless of payment token:
//   40% → buyback $Ensoul → mining pool, 10% → holder revenue pool, 50% → treasury.
// For USDT: requires USDT→BNB conversion first.
// For BNB: skips the USDT→BNB step, directly uses BNB for buyback.
func triggerSubscriptionFlywheel(paymentToken string, paymentAmount float64) {
	if paymentAmount <= 0 {
		return
	}

	// Convert float amount to wei (18 decimals)
	amountWei := toWei(paymentAmount)

	switch paymentToken {
	case "USDT":
		// USDT path: 40% USDT→BNB→$Ensoul + 10% revenue pool (handled inside Async)
		ProcessSubscriptionRevenueAsync(amountWei)
	case "BNB":
		// BNB path: same 40/10/50 split, but skip USDT→BNB step
		ProcessBNBSubscriptionRevenueAsync(amountWei)
	default:
		// $ENSOUL or other tokens: only feed revenue pool (10% of amount)
		poolAmount := paymentAmount * float64(SubscriptionRevenuePoolPct) / 100.0
		AddToRevenuePool(poolAmount)
		util.Log.Info("[subscription] %s payment — added %.4f to revenue pool (no buyback for this token)", paymentToken, poolAmount)
	}
}

// GetActiveSubscription returns the user's active subscription, if any.
func GetActiveSubscription(walletAddr string) (*models.Subscription, error) {
	var sub models.Subscription
	if err := database.DB.Where("wallet_addr = ? AND status = ?", walletAddr, models.SubStatusActive).
		First(&sub).Error; err != nil {
		return nil, fmt.Errorf("no active subscription found")
	}

	// Check if expired
	if time.Now().UTC().After(sub.ExpiresAt) {
		sub.Status = models.SubStatusExpired
		database.DB.Save(&sub)
		return nil, fmt.Errorf("subscription expired")
	}

	return &sub, nil
}

// GetSubscriptionStatus returns subscription info for display.
func GetSubscriptionStatus(walletAddr string) (map[string]interface{}, error) {
	sub, err := GetActiveSubscription(walletAddr)
	if err != nil {
		return map[string]interface{}{
			"active": false,
		}, nil
	}

	tierCfg := models.SubscriptionTiers[sub.Tier]

	// Count current KOLs
	var kolCount int64
	database.DB.Model(&models.SniperKOL{}).Where("subscription_id = ?", sub.ID).Count(&kolCount)

	// Count today's replies
	todayStart := time.Now().UTC().Truncate(24 * time.Hour)
	var replyCount int64
	database.DB.Model(&models.SniperReply{}).
		Where("wallet_addr = ? AND created_at >= ?", walletAddr, todayStart).
		Count(&replyCount)

	return map[string]interface{}{
		"active":        true,
		"tier":          sub.Tier,
		"llm_model":     sub.LLMModel,
		"expires_at":    sub.ExpiresAt,
		"kol_count":     kolCount,
		"kol_limit":     tierCfg.MaxKOLs,
		"daily_replies": replyCount,
		"daily_limit":   tierCfg.DailyReplies,
		"payment_token": sub.PaymentToken,
	}, nil
}

// AddSniperKOL adds a KOL to the user's tracking list.
func AddSniperKOL(walletAddr, handle string) (*models.SniperKOL, error) {
	sub, err := GetActiveSubscription(walletAddr)
	if err != nil {
		return nil, fmt.Errorf("active subscription required: %w", err)
	}

	tierCfg := models.SubscriptionTiers[sub.Tier]

	// Check KOL limit
	var kolCount int64
	database.DB.Model(&models.SniperKOL{}).Where("subscription_id = ?", sub.ID).Count(&kolCount)
	if int(kolCount) >= tierCfg.MaxKOLs {
		return nil, fmt.Errorf("KOL limit reached (%d/%d for %s tier)", kolCount, tierCfg.MaxKOLs, sub.Tier)
	}

	// Verify the shell exists
	shell, err := GetShellByHandle(handle)
	if err != nil {
		return nil, fmt.Errorf("soul @%s not found; it must be minted first", handle)
	}

	// Check for duplicate
	var existing models.SniperKOL
	if err := database.DB.Where("subscription_id = ? AND shell_id = ?", sub.ID, shell.ID).
		First(&existing).Error; err == nil {
		return nil, fmt.Errorf("@%s is already in your tracking list", handle)
	}

	kol := &models.SniperKOL{
		SubscriptionID: sub.ID,
		ShellID:        shell.ID,
		Handle:         handle,
	}

	if err := database.DB.Create(kol).Error; err != nil {
		return nil, fmt.Errorf("failed to add KOL: %w", err)
	}

	util.Log.Info("[sniper] Added @%s to tracking for %s", handle, walletAddr)
	return kol, nil
}

// RemoveSniperKOL removes a KOL from the user's tracking list.
func RemoveSniperKOL(walletAddr string, kolID uuid.UUID) error {
	sub, err := GetActiveSubscription(walletAddr)
	if err != nil {
		return fmt.Errorf("active subscription required: %w", err)
	}

	result := database.DB.Where("id = ? AND subscription_id = ?", kolID, sub.ID).Delete(&models.SniperKOL{})
	if result.RowsAffected == 0 {
		return fmt.Errorf("KOL not found in your tracking list")
	}

	return nil
}

// ListSniperKOLs returns the user's tracked KOLs.
func ListSniperKOLs(walletAddr string) ([]models.SniperKOL, error) {
	sub, err := GetActiveSubscription(walletAddr)
	if err != nil {
		return nil, err
	}

	var kols []models.SniperKOL
	if err := database.DB.Where("subscription_id = ?", sub.ID).
		Preload("Shell").Find(&kols).Error; err != nil {
		return nil, err
	}

	return kols, nil
}

// CheckDailyReplyLimit returns whether the user can still generate replies today.
func CheckDailyReplyLimit(walletAddr string, sub *models.Subscription) (bool, int64, error) {
	tierCfg := models.SubscriptionTiers[sub.Tier]
	if tierCfg.DailyReplies < 0 {
		return true, 0, nil // unlimited
	}

	todayStart := time.Now().UTC().Truncate(24 * time.Hour)
	var count int64
	database.DB.Model(&models.SniperReply{}).
		Where("wallet_addr = ? AND created_at >= ?", walletAddr, todayStart).
		Count(&count)

	return count < int64(tierCfg.DailyReplies), count, nil
}

// ExpireSubscriptions checks and expires overdue subscriptions.
func ExpireSubscriptions() {
	now := time.Now().UTC()
	result := database.DB.Model(&models.Subscription{}).
		Where("status = ? AND expires_at < ?", models.SubStatusActive, now).
		Update("status", models.SubStatusExpired)

	if result.RowsAffected > 0 {
		util.Log.Info("[subscription] Expired %d subscriptions", result.RowsAffected)
	}
}

// StartSubscriptionCleanup starts a background goroutine to expire subscriptions hourly.
func StartSubscriptionCleanup(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			ExpireSubscriptions()
		}
	}()
	util.Log.Info("[subscription] Expiry checker started (interval: %s)", interval)
}
