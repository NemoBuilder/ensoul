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
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	// DailyReleaseRate is the max percentage of pool balance released per day (5%)
	DailyReleaseRate = 0.05
	// MinBounty is the minimum $Ensoul bounty per fragment demand
	MinBounty = 10.0
	// PoolPauseThreshold — pause new demands when pool balance < this
	PoolPauseThreshold = 1000.0
)

// GetOrCreateMiningPool returns the singleton mining pool record.
func GetOrCreateMiningPool() (*models.MiningPool, error) {
	var pool models.MiningPool
	result := database.DB.First(&pool)
	if result.Error != nil {
		// Create initial pool
		now := time.Now().UTC()
		pool = models.MiningPool{
			Balance:           0,
			DailyStartBalance: 0,
			LastResetAt:       now,
		}
		if err := database.DB.Create(&pool).Error; err != nil {
			return nil, fmt.Errorf("failed to create mining pool: %w", err)
		}
	}
	return &pool, nil
}

// dailyLimit returns the stable daily release limit based on the start-of-day snapshot.
func dailyLimit(pool *models.MiningPool) float64 {
	return pool.DailyStartBalance * DailyReleaseRate
}

// GetPoolStatus returns the current mining pool status.
func GetPoolStatus() (map[string]interface{}, error) {
	pool, err := GetOrCreateMiningPool()
	if err != nil {
		return nil, err
	}

	limit := dailyLimit(pool)
	remaining := limit - pool.DailyReleased
	if remaining < 0 {
		remaining = 0
	}

	return map[string]interface{}{
		"balance":             pool.Balance,
		"total_deposited":     pool.TotalDeposited,
		"total_released":      pool.TotalReleased,
		"daily_limit":         limit,
		"daily_released":      pool.DailyReleased,
		"daily_remaining":     remaining,
		"daily_start_balance": pool.DailyStartBalance,
		"paused":              pool.Balance < PoolPauseThreshold,
		"last_reset_at":       pool.LastResetAt,
	}, nil
}

// DepositToPool adds $Ensoul tokens to the mining pool.
// Called after buyback operations or manual team injection.
func DepositToPool(amount float64, source string) error {
	return database.DB.Transaction(func(tx *gorm.DB) error {
		var pool models.MiningPool
		if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&pool).Error; err != nil {
			return fmt.Errorf("failed to lock mining pool: %w", err)
		}

		pool.Balance += amount
		pool.TotalDeposited += amount

		if err := tx.Save(&pool).Error; err != nil {
			return fmt.Errorf("failed to update mining pool: %w", err)
		}

		util.Log.Info("[mining] Deposited %.4f $Ensoul to pool (source: %s), new balance: %.4f",
			amount, source, pool.Balance)
		return nil
	})
}

// CalculateDailyRelease returns how much $Ensoul can still be released today.
func CalculateDailyRelease() (float64, error) {
	pool, err := GetOrCreateMiningPool()
	if err != nil {
		return 0, err
	}

	limit := dailyLimit(pool)
	remaining := limit - pool.DailyReleased
	if remaining < 0 {
		remaining = 0
	}
	return remaining, nil
}

// ResetDailyRelease resets the daily release counter. Called by cron at 00:00 UTC.
// Snapshots the current Balance into DailyStartBalance so that daily_limit stays
// stable for the entire day regardless of deposits, releases, or refunds.
func ResetDailyRelease() error {
	return database.DB.Transaction(func(tx *gorm.DB) error {
		var pool models.MiningPool
		if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&pool).Error; err != nil {
			return fmt.Errorf("failed to lock mining pool for reset: %w", err)
		}

		pool.DailyStartBalance = pool.Balance
		pool.DailyReleased = 0
		pool.LastResetAt = time.Now().UTC()

		if err := tx.Save(&pool).Error; err != nil {
			return fmt.Errorf("failed to reset daily release: %w", err)
		}

		util.Log.Info("[mining] Daily release reset. Pool balance: %.4f, DailyStartBalance: %.4f",
			pool.Balance, pool.DailyStartBalance)
		return nil
	})
}

// ReleaseFromPool atomically deducts an amount from the pool for a reward.
// Uses a DB transaction to prevent concurrent over-release.
// daily_limit is calculated from DailyStartBalance (snapshot), not live Balance.
func ReleaseFromPool(amount float64) error {
	return database.DB.Transaction(func(tx *gorm.DB) error {
		var pool models.MiningPool
		// Lock the row to prevent concurrent reads
		if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&pool).Error; err != nil {
			return fmt.Errorf("failed to lock mining pool: %w", err)
		}

		limit := pool.DailyStartBalance * DailyReleaseRate
		if pool.DailyReleased+amount > limit {
			return fmt.Errorf("daily release limit exceeded (limit: %.4f, used: %.4f, requested: %.4f)",
				limit, pool.DailyReleased, amount)
		}

		if amount > pool.Balance {
			return fmt.Errorf("insufficient pool balance (balance: %.4f, requested: %.4f)",
				pool.Balance, amount)
		}

		pool.Balance -= amount
		pool.DailyReleased += amount
		// NOTE: TotalReleased is NOT incremented here.
		// It is only incremented when the on-chain transfer is confirmed,
		// so that failed+refunded attempts don't inflate the counter.

		return tx.Save(&pool).Error
	})
}

// RefundToPool returns tokens to the pool when a chain transfer fails.
// Only Balance and TotalReleased are rolled back; DailyReleased is NOT rolled
// back so the "released today" number stays monotonically increasing and the
// frontend display remains stable.
func RefundToPool(amount float64) {
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		var pool models.MiningPool
		if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&pool).Error; err != nil {
			return fmt.Errorf("failed to lock mining pool for refund: %w", err)
		}

		pool.Balance += amount
		// NOTE: TotalReleased is NOT decremented here.
		// It was never incremented at release time — only on confirmed success.

		return tx.Save(&pool).Error
	})
	if err != nil {
		util.Log.Error("[mining] Failed to refund %.4f to pool: %v", amount, err)
		return
	}
	util.Log.Info("[mining] Refunded %.4f $Ensoul to pool (DailyReleased unchanged)", amount)
}

// reserveBalanceForRetry deducts Balance for a retry attempt.
// Unlike ReleaseFromPool, it does NOT check or increment DailyReleased,
// because the original attempt already counted toward the daily limit.
func reserveBalanceForRetry(amount float64) error {
	return database.DB.Transaction(func(tx *gorm.DB) error {
		var pool models.MiningPool
		if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&pool).Error; err != nil {
			return fmt.Errorf("failed to lock mining pool: %w", err)
		}

		if amount > pool.Balance {
			return fmt.Errorf("insufficient pool balance (balance: %.4f, requested: %.4f)",
				pool.Balance, amount)
		}

		pool.Balance -= amount

		return tx.Save(&pool).Error
	})
}

// confirmRelease increments TotalReleased after a reward is confirmed on-chain.
// This is the ONLY place TotalReleased is incremented, ensuring it only counts
// successfully delivered tokens (not failed/refunded attempts).
func confirmRelease(amount float64) {
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		var pool models.MiningPool
		if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&pool).Error; err != nil {
			return fmt.Errorf("failed to lock mining pool for confirm: %w", err)
		}

		pool.TotalReleased += amount

		return tx.Save(&pool).Error
	})
	if err != nil {
		util.Log.Error("[mining] Failed to confirm release of %.4f: %v", amount, err)
	}
}

// DistributeReward sends $Ensoul from the mining pool to a Claw's wallet.
func DistributeReward(clawID, fragmentID uuid.UUID, demandID *uuid.UUID, amount float64) error {
	// Release from pool first (atomic)
	if err := ReleaseFromPool(amount); err != nil {
		return fmt.Errorf("pool release failed: %w", err)
	}

	// Create reward record
	reward := &models.MiningReward{
		ClawID:     clawID,
		FragmentID: fragmentID,
		DemandID:   demandID,
		Amount:     amount,
		Status:     models.RewardStatusPending,
	}
	if err := database.DB.Create(reward).Error; err != nil {
		RefundToPool(amount)
		return fmt.Errorf("failed to create reward record: %w", err)
	}

	// Send tokens on-chain (async — will refund to pool on failure)
	go sendRewardOnChain(reward)

	return nil
}

// failReward marks a reward as failed, records the error, and refunds the pool.
func failReward(reward *models.MiningReward, reason string, refund bool) {
	now := time.Now()
	updates := map[string]interface{}{
		"status":          models.RewardStatusFailed,
		"last_error":      reason,
		"last_attempt_at": now,
	}
	database.DB.Model(reward).Updates(updates)
	if refund {
		RefundToPool(reward.Amount)
	}
}

// sendRewardOnChain transfers $Ensoul tokens to the Claw's wallet.
func sendRewardOnChain(reward *models.MiningReward) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Get Claw's wallet address
	var claw models.Claw
	if err := database.DB.Where("id = ?", reward.ClawID).First(&claw).Error; err != nil {
		util.Log.Error("[mining] Failed to find claw %s: %v", reward.ClawID, err)
		failReward(reward, fmt.Sprintf("claw not found: %v", err), true)
		return
	}

	if claw.WalletAddr == "" {
		util.Log.Error("[mining] Claw %s has no wallet address", claw.Name)
		failReward(reward, "claw has no wallet address", true)
		return
	}

	// Parse mining pool private key
	poolKey, err := chain.ParsePrivateKey(config.Cfg.MiningPoolPrivateKey)
	if err != nil {
		util.Log.Error("[mining] Failed to parse mining pool key: %v", err)
		failReward(reward, fmt.Sprintf("parse pool key: %v", err), true)
		return
	}

	// Convert amount to wei (18 decimals)
	amountWei := toWei(reward.Amount)

	txHash, err := chain.TransferToken(ctx, poolKey, claw.WalletAddr, amountWei)
	if err != nil {
		util.Log.Error("[mining] Token transfer failed for claw %s: %v", claw.Name, err)
		failReward(reward, fmt.Sprintf("transfer failed: %v", err), true)
		return
	}

	database.DB.Model(reward).Updates(map[string]interface{}{
		"tx_hash": txHash,
		"status":  models.RewardStatusSent,
	})

	// Wait for confirmation
	success, err := chain.WaitForTokenTx(ctx, txHash)
	if err != nil || !success {
		util.Log.Error("[mining] Reward tx failed: %s, err: %v", txHash, err)
		failReward(reward, fmt.Sprintf("tx receipt failed: %v", err), true)
		return
	}

	now := time.Now()
	database.DB.Model(reward).Updates(map[string]interface{}{
		"status":          models.RewardStatusConfirmed,
		"last_attempt_at": now,
	})

	// Now that the tx is confirmed, count it as truly released
	confirmRelease(reward.Amount)

	// Update Claw earnings
	database.DB.Model(&claw).Update("earnings", claw.Earnings+reward.Amount)

	util.Log.Info("[mining] Reward sent: %.4f $Ensoul to %s (claw: %s), tx=%s",
		reward.Amount, claw.WalletAddr, claw.Name, txHash)
}

// toWei converts a float64 token amount to *big.Int wei (18 decimals).
func toWei(amount float64) *big.Int {
	weiPerToken := new(big.Float).SetFloat64(1e18)
	amountFloat := new(big.Float).SetFloat64(amount)
	weiFloat := new(big.Float).Mul(amountFloat, weiPerToken)
	wei, _ := weiFloat.Int(nil)
	return wei
}

// repairTotalReleased recalculates TotalReleased from the sum of all confirmed
// mining rewards. This fixes the drift caused by the old logic where
// ReleaseFromPool incremented TotalReleased and RefundToPool decremented it.
func repairTotalReleased() {
	var confirmedSum float64
	err := database.DB.Model(&models.MiningReward{}).
		Where("status = ?", models.RewardStatusConfirmed).
		Select("COALESCE(SUM(amount), 0)").
		Row().Scan(&confirmedSum)
	if err != nil {
		util.Log.Error("[mining] Failed to calculate confirmed reward sum: %v", err)
		return
	}

	pool, err := GetOrCreateMiningPool()
	if err != nil {
		util.Log.Error("[mining] Failed to get pool for TotalReleased repair: %v", err)
		return
	}

	if pool.TotalReleased != confirmedSum {
		util.Log.Info("[mining] Repairing TotalReleased: %.4f → %.4f (from confirmed rewards)",
			pool.TotalReleased, confirmedSum)
		database.DB.Model(pool).Update("total_released", confirmedSum)
	}
}

// StartMiningDailyReset starts a background goroutine that resets daily release at 00:00 UTC.
// On startup, it checks LastResetAt — if the last reset was not today, it immediately
// performs a compensatory reset. This ensures restarts/deployments never cause a missed day.
// It also ensures DailyStartBalance is populated (handles migration from old schema).
func StartMiningDailyReset() {
	// Compensatory check: if last reset was before today, reset now.
	go func() {
		// Small delay to let DB connection settle after startup
		time.Sleep(5 * time.Second)

		// ── One-time migration: repair TotalReleased from confirmed rewards ──
		// Previously TotalReleased was incremented at release time and decremented
		// on refund, which caused it to drift lower than DailyReleased. Now it is
		// only incremented on confirmed success. Recalculate from ground truth.
		repairTotalReleased()

		pool, err := GetOrCreateMiningPool()
		if err != nil {
			util.Log.Error("[mining] Failed to check pool for compensatory reset: %v", err)
		} else {
			todayStart := time.Now().UTC().Truncate(24 * time.Hour)
			if pool.LastResetAt.Before(todayStart) {
				util.Log.Info("[mining] Compensatory reset: LastResetAt=%s is before today %s",
					pool.LastResetAt.Format(time.RFC3339), todayStart.Format(time.RFC3339))
				if err := ResetDailyRelease(); err != nil {
					util.Log.Error("[mining] Compensatory daily reset failed: %v", err)
				}
			} else if pool.DailyStartBalance == 0 && pool.Balance > 0 {
				// Migration: old rows without DailyStartBalance — backfill snapshot
				util.Log.Info("[mining] Backfilling DailyStartBalance from current Balance: %.4f", pool.Balance)
				pool.DailyStartBalance = pool.Balance
				database.DB.Save(pool)
			} else {
				util.Log.Debug("[mining] Daily reset already done today, no compensation needed")
			}
		}

		// Then loop: sleep until next 00:00 UTC and reset
		for {
			now := time.Now().UTC()
			next := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.UTC)
			time.Sleep(time.Until(next))

			if err := ResetDailyRelease(); err != nil {
				util.Log.Error("[mining] Failed to reset daily release: %v", err)
			}
		}
	}()
	util.Log.Info("[mining] Daily release reset scheduler started (with startup compensation)")
}

// ═══════════════════════════════════════════════════════════════
// Reward Retry System
// ═══════════════════════════════════════════════════════════════

const MaxRewardRetries = 5

// RetryFailedReward re-attempts a single failed reward.
// It re-deducts from the pool (since the original failure refunded it), then re-sends.
func RetryFailedReward(rewardID uuid.UUID) error {
	var reward models.MiningReward
	if err := database.DB.Where("id = ?", rewardID).First(&reward).Error; err != nil {
		return fmt.Errorf("reward not found: %w", err)
	}
	if reward.Status != models.RewardStatusFailed {
		return fmt.Errorf("reward status is %q, not failed", reward.Status)
	}
	if reward.RetryCount >= MaxRewardRetries {
		return fmt.Errorf("max retries (%d) exceeded", MaxRewardRetries)
	}

	// Re-deduct Balance only (skip daily limit check — the original attempt
	// already incremented DailyReleased and it was intentionally not rolled back).
	if err := reserveBalanceForRetry(reward.Amount); err != nil {
		return fmt.Errorf("pool re-deduction failed: %w", err)
	}

	// Reset status to pending, increment retry count
	database.DB.Model(&reward).Updates(map[string]interface{}{
		"status":      models.RewardStatusPending,
		"retry_count": reward.RetryCount + 1,
		"tx_hash":     "", // clear old failed tx
		"last_error":  "",
	})

	util.Log.Info("[mining] Retrying reward %s (attempt %d)", reward.ID, reward.RetryCount+1)

	// Re-send on-chain (async)
	go sendRewardOnChain(&reward)

	return nil
}

// RetryAllFailedRewards retries all eligible failed rewards.
// Returns the count of rewards queued for retry.
func RetryAllFailedRewards() (int, error) {
	var rewards []models.MiningReward
	database.DB.Where("status = ? AND retry_count < ?", models.RewardStatusFailed, MaxRewardRetries).
		Order("created_at ASC").
		Limit(50).
		Find(&rewards)

	retried := 0
	for _, r := range rewards {
		if err := RetryFailedReward(r.ID); err != nil {
			util.Log.Error("[mining] Skip retry for reward %s: %v", r.ID, err)
			continue
		}
		retried++
		// Small delay between retries to avoid nonce collisions
		time.Sleep(3 * time.Second)
	}
	return retried, nil
}

// GetFailedRewards returns all failed mining rewards for admin inspection.
func GetFailedRewards() ([]models.MiningReward, error) {
	var rewards []models.MiningReward
	err := database.DB.Where("status = ?", models.RewardStatusFailed).
		Order("created_at DESC").
		Preload("Claw").
		Find(&rewards).Error
	return rewards, err
}

// StartRewardRetryScheduler starts a background goroutine that retries
// failed rewards every 10 minutes, up to MaxRewardRetries attempts.
func StartRewardRetryScheduler() {
	go func() {
		// Wait for services to be ready
		time.Sleep(30 * time.Second)

		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			var count int64
			database.DB.Model(&models.MiningReward{}).
				Where("status = ? AND retry_count < ?", models.RewardStatusFailed, MaxRewardRetries).
				Count(&count)

			if count == 0 {
				continue
			}

			util.Log.Info("[mining] Auto-retry: found %d failed rewards, retrying...", count)
			retried, err := RetryAllFailedRewards()
			if err != nil {
				util.Log.Error("[mining] Auto-retry error: %v", err)
			} else {
				util.Log.Info("[mining] Auto-retry: queued %d rewards for retry", retried)
			}
		}
	}()
	util.Log.Info("[mining] Reward retry scheduler started (every 10 minutes, max %d retries)", MaxRewardRetries)
}
