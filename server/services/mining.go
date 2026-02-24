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
		pool = models.MiningPool{
			Balance:     0,
			LastResetAt: time.Now().UTC(),
		}
		if err := database.DB.Create(&pool).Error; err != nil {
			return nil, fmt.Errorf("failed to create mining pool: %w", err)
		}
	}
	return &pool, nil
}

// GetPoolStatus returns the current mining pool status.
func GetPoolStatus() (map[string]interface{}, error) {
	pool, err := GetOrCreateMiningPool()
	if err != nil {
		return nil, err
	}

	dailyLimit := pool.Balance * DailyReleaseRate
	remaining := dailyLimit - pool.DailyReleased
	if remaining < 0 {
		remaining = 0
	}

	return map[string]interface{}{
		"balance":         pool.Balance,
		"total_deposited": pool.TotalDeposited,
		"total_released":  pool.TotalReleased,
		"daily_limit":     dailyLimit,
		"daily_released":  pool.DailyReleased,
		"daily_remaining": remaining,
		"paused":          pool.Balance < PoolPauseThreshold,
		"last_reset_at":   pool.LastResetAt,
	}, nil
}

// DepositToPool adds $Ensoul tokens to the mining pool.
// Called after buyback operations or manual team injection.
func DepositToPool(amount float64, source string) error {
	pool, err := GetOrCreateMiningPool()
	if err != nil {
		return err
	}

	pool.Balance += amount
	pool.TotalDeposited += amount

	if err := database.DB.Save(pool).Error; err != nil {
		return fmt.Errorf("failed to update mining pool: %w", err)
	}

	util.Log.Info("[mining] Deposited %.4f $Ensoul to pool (source: %s), new balance: %.4f",
		amount, source, pool.Balance)
	return nil
}

// CalculateDailyRelease returns how much $Ensoul can still be released today.
func CalculateDailyRelease() (float64, error) {
	pool, err := GetOrCreateMiningPool()
	if err != nil {
		return 0, err
	}

	dailyLimit := pool.Balance * DailyReleaseRate
	remaining := dailyLimit - pool.DailyReleased
	if remaining < 0 {
		remaining = 0
	}
	return remaining, nil
}

// ResetDailyRelease resets the daily release counter. Called by cron at 00:00 UTC.
func ResetDailyRelease() error {
	pool, err := GetOrCreateMiningPool()
	if err != nil {
		return err
	}

	pool.DailyReleased = 0
	pool.LastResetAt = time.Now().UTC()

	if err := database.DB.Save(pool).Error; err != nil {
		return fmt.Errorf("failed to reset daily release: %w", err)
	}

	util.Log.Info("[mining] Daily release reset. Pool balance: %.4f", pool.Balance)
	return nil
}

// ReleaseFromPool atomically deducts an amount from the pool for a reward.
// Uses a DB transaction to prevent concurrent over-release.
func ReleaseFromPool(amount float64) error {
	return database.DB.Transaction(func(tx *gorm.DB) error {
		var pool models.MiningPool
		// Lock the row to prevent concurrent reads
		if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&pool).Error; err != nil {
			return fmt.Errorf("failed to lock mining pool: %w", err)
		}

		dailyLimit := pool.Balance * DailyReleaseRate
		if pool.DailyReleased+amount > dailyLimit {
			return fmt.Errorf("daily release limit exceeded (limit: %.4f, used: %.4f, requested: %.4f)",
				dailyLimit, pool.DailyReleased, amount)
		}

		if amount > pool.Balance {
			return fmt.Errorf("insufficient pool balance (balance: %.4f, requested: %.4f)",
				pool.Balance, amount)
		}

		pool.Balance -= amount
		pool.DailyReleased += amount
		pool.TotalReleased += amount

		return tx.Save(&pool).Error
	})
}

// RefundToPool returns tokens to the pool when a chain transfer fails.
func RefundToPool(amount float64) {
	pool, err := GetOrCreateMiningPool()
	if err != nil {
		util.Log.Error("[mining] Failed to refund %.4f to pool: %v", amount, err)
		return
	}
	pool.Balance += amount
	pool.DailyReleased -= amount
	pool.TotalReleased -= amount
	if pool.DailyReleased < 0 {
		pool.DailyReleased = 0
	}
	if pool.TotalReleased < 0 {
		pool.TotalReleased = 0
	}
	if err := database.DB.Save(pool).Error; err != nil {
		util.Log.Error("[mining] Failed to save pool refund: %v", err)
	}
	util.Log.Info("[mining] Refunded %.4f $Ensoul to pool", amount)
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

// sendRewardOnChain transfers $Ensoul tokens to the Claw's wallet.
func sendRewardOnChain(reward *models.MiningReward) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Get Claw's wallet address
	var claw models.Claw
	if err := database.DB.Where("id = ?", reward.ClawID).First(&claw).Error; err != nil {
		util.Log.Error("[mining] Failed to find claw %s: %v", reward.ClawID, err)
		database.DB.Model(reward).Update("status", models.RewardStatusFailed)
		return
	}

	if claw.WalletAddr == "" {
		util.Log.Error("[mining] Claw %s has no wallet address", claw.Name)
		database.DB.Model(reward).Update("status", models.RewardStatusFailed)
		RefundToPool(reward.Amount)
		return
	}

	// Parse mining pool private key
	poolKey, err := chain.ParsePrivateKey(config.Cfg.MiningPoolPrivateKey)
	if err != nil {
		util.Log.Error("[mining] Failed to parse mining pool key: %v", err)
		database.DB.Model(reward).Update("status", models.RewardStatusFailed)
		RefundToPool(reward.Amount)
		return
	}

	// Convert amount to wei (18 decimals)
	amountWei := toWei(reward.Amount)

	txHash, err := chain.TransferToken(ctx, poolKey, claw.WalletAddr, amountWei)
	if err != nil {
		util.Log.Error("[mining] Token transfer failed for claw %s: %v", claw.Name, err)
		database.DB.Model(reward).Update("status", models.RewardStatusFailed)
		RefundToPool(reward.Amount)
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
		database.DB.Model(reward).Update("status", models.RewardStatusFailed)
		RefundToPool(reward.Amount)
		return
	}

	database.DB.Model(reward).Update("status", models.RewardStatusConfirmed)

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

// StartMiningDailyReset starts a background goroutine that resets daily release at 00:00 UTC.
func StartMiningDailyReset() {
	go func() {
		for {
			now := time.Now().UTC()
			next := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.UTC)
			time.Sleep(time.Until(next))

			if err := ResetDailyRelease(); err != nil {
				util.Log.Error("[mining] Failed to reset daily release: %v", err)
			}
		}
	}()
	util.Log.Info("[mining] Daily release reset scheduler started")
}
