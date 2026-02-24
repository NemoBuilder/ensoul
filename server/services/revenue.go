package services

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/ensoul-labs/ensoul-server/chain"
	"github.com/ensoul-labs/ensoul-server/config"
	"github.com/ensoul-labs/ensoul-server/database"
	"github.com/ensoul-labs/ensoul-server/models"
	"github.com/ensoul-labs/ensoul-server/util"
	"github.com/google/uuid"
)

// Stage multipliers for revenue weight calculation
var stageMultiplier = map[string]float64{
	models.StageEmbryo:   0.5,
	models.StageGrowing:  1.0,
	models.StageMature:   1.5,
	models.StageEvolving: 2.0,
}

// RecordUsage increments the usage count for a Soul in the current period.
// Called every time a Soul is used (chat, sniper reply, etc.)
func RecordUsage(shellID uuid.UUID) {
	period := currentPeriod()

	var usage models.SoulUsage
	result := database.DB.Where("shell_id = ? AND period = ?", shellID, period).First(&usage)

	if result.Error != nil {
		// Create new record
		usage = models.SoulUsage{
			ShellID:    shellID,
			Period:     period,
			UsageCount: 1,
		}
		database.DB.Create(&usage)
	} else {
		database.DB.Model(&usage).UpdateColumn("usage_count", usage.UsageCount+1)
	}
}

// currentPeriod returns the current month in "2026-02" format.
func currentPeriod() string {
	return time.Now().UTC().Format("2006-01")
}

// previousPeriod returns the previous month in "2026-01" format.
func previousPeriod() string {
	now := time.Now().UTC()
	prev := now.AddDate(0, -1, 0)
	return prev.Format("2006-01")
}

// CalculateMonthlyRevenue computes revenue shares for all Soul holders for a given period.
// Uses logarithmic curve: weight = ln(usage + 1) × stageMultiplier
func CalculateMonthlyRevenue(period string) error {
	// Check if already distributed
	var pool models.RevenuePool
	if err := database.DB.Where("period = ?", period).First(&pool).Error; err == nil {
		if pool.Distributed {
			return fmt.Errorf("revenue for period %s already distributed", period)
		}
	} else {
		return fmt.Errorf("no revenue pool found for period %s", period)
	}

	if pool.PoolAmount <= 0 {
		util.Log.Info("[revenue] No revenue to distribute for period %s", period)
		return nil
	}

	// Get all soul usage for this period
	var usages []models.SoulUsage
	database.DB.Where("period = ? AND usage_count > 0", period).Find(&usages)

	if len(usages) == 0 {
		util.Log.Info("[revenue] No soul usage recorded for period %s", period)
		return nil
	}

	// Calculate weights
	type weightEntry struct {
		ShellID    uuid.UUID
		WalletAddr string
		Usage      int
		Weight     float64
	}

	var holders []weightEntry
	var totalWeight float64

	for _, u := range usages {
		// Get the shell owner
		var shell models.Shell
		if err := database.DB.Where("id = ?", u.ShellID).First(&shell).Error; err != nil {
			continue
		}

		// Get stage multiplier
		mult, ok := stageMultiplier[shell.Stage]
		if !ok {
			mult = 1.0
		}

		// Check if KOL has claimed — adjust revenue split
		ownerAddr := shell.OwnerAddr
		kolSplit := getKOLRevenueSplit(shell.ID)

		// weight = ln(usage + 1) × stageMultiplier
		weight := math.Log(float64(u.UsageCount)+1) * mult

		// Holder gets (1 - kolSplit) of the weight
		hw := weight * (1 - kolSplit)

		holders = append(holders, weightEntry{
			ShellID:    shell.ID,
			WalletAddr: ownerAddr,
			Usage:      u.UsageCount,
			Weight:     hw,
		})
		totalWeight += hw

		// If KOL has claimed, they also get a share
		if kolSplit > 0 {
			var claim models.KOLClaim
			if err := database.DB.Where("shell_id = ? AND status = ?", shell.ID, models.ClaimStatusVerified).
				First(&claim).Error; err == nil {
				kw := weight * kolSplit
				holders = append(holders, weightEntry{
					ShellID:    shell.ID,
					WalletAddr: claim.KOLWalletAddr,
					Usage:      u.UsageCount,
					Weight:     kw,
				})
				totalWeight += kw
			}
		}
	}

	if totalWeight == 0 {
		util.Log.Info("[revenue] Total weight is 0 for period %s", period)
		return nil
	}

	// Distribute pool amount proportionally
	for _, h := range holders {
		amount := pool.PoolAmount * (h.Weight / totalWeight)
		if amount < 0.0001 {
			continue // Skip dust amounts
		}

		revenue := &models.HolderRevenue{
			ShellID:    h.ShellID,
			WalletAddr: h.WalletAddr,
			Period:     period,
			UsageCount: h.Usage,
			Weight:     h.Weight,
			Amount:     amount,
			Status:     models.HolderRevenueStatusPending,
		}
		database.DB.Create(revenue)
	}

	// Mark pool as distributed
	database.DB.Model(&pool).Update("distributed", true)

	util.Log.Info("[revenue] Distributed %.4f $Ensoul across %d holders for period %s",
		pool.PoolAmount, len(holders), period)
	return nil
}

// getKOLRevenueSplit returns the KOL's share of revenue for a soul.
// During 3-month transition: 30% KOL / 70% holder
// After transition: 50% KOL / 50% holder
// No claim: 0% KOL / 100% holder
func getKOLRevenueSplit(shellID uuid.UUID) float64 {
	var claim models.KOLClaim
	if err := database.DB.Where("shell_id = ? AND status = ?", shellID, models.ClaimStatusVerified).
		First(&claim).Error; err != nil {
		return 0 // No claim
	}

	if claim.TransitionEnd != nil && time.Now().UTC().Before(*claim.TransitionEnd) {
		return 0.30 // Transition period: 30% to KOL
	}
	return 0.50 // Post-transition: 50% to KOL
}

// ClaimHolderRevenue allows a holder to claim their pending revenue.
// Transfers $Ensoul from the Revenue Pool wallet to the holder's wallet.
func ClaimHolderRevenue(walletAddr string) (float64, string, error) {
	// Find all pending revenue for this wallet
	var revenues []models.HolderRevenue
	database.DB.Where("wallet_addr = ? AND status = ?", walletAddr, models.HolderRevenueStatusPending).
		Find(&revenues)

	if len(revenues) == 0 {
		return 0, "", fmt.Errorf("no pending revenue to claim")
	}

	// Sum up total claimable
	var totalAmount float64
	var revenueIDs []uuid.UUID
	for _, r := range revenues {
		totalAmount += r.Amount
		revenueIDs = append(revenueIDs, r.ID)
	}

	if totalAmount < 0.01 {
		return 0, "", fmt.Errorf("claimable amount too small (%.8f $Ensoul)", totalAmount)
	}

	// Transfer from revenue pool wallet
	if config.Cfg.RevenuePoolPrivateKey == "" {
		return 0, "", fmt.Errorf("revenue pool wallet not configured")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	poolKey, err := chain.ParsePrivateKey(config.Cfg.RevenuePoolPrivateKey)
	if err != nil {
		return 0, "", fmt.Errorf("failed to parse revenue pool key: %w", err)
	}

	amountWei := toWei(totalAmount)
	txHash, err := chain.TransferToken(ctx, poolKey, walletAddr, amountWei)
	if err != nil {
		return 0, "", fmt.Errorf("transfer failed: %w", err)
	}

	// Update all claimed revenues
	database.DB.Model(&models.HolderRevenue{}).
		Where("id IN ?", revenueIDs).
		Updates(map[string]interface{}{
			"status":  models.HolderRevenueStatusClaimed,
			"tx_hash": txHash,
		})

	util.Log.Info("[revenue] Holder %s claimed %.4f $Ensoul, tx=%s", walletAddr, totalAmount, txHash)
	return totalAmount, txHash, nil
}

// GetHolderDashboard returns revenue data for a holder's dashboard.
func GetHolderDashboard(walletAddr string) (map[string]interface{}, error) {
	// Get owned shells
	var shells []models.Shell
	database.DB.Where("LOWER(owner_addr) = LOWER(?) AND stage != ?", walletAddr, models.StagePending).Find(&shells)

	// Get total earned and pending
	var totalEarned float64
	var totalPending float64
	database.DB.Model(&models.HolderRevenue{}).
		Where("wallet_addr = ? AND status = ?", walletAddr, models.HolderRevenueStatusClaimed).
		Select("COALESCE(SUM(amount), 0)").Scan(&totalEarned)
	database.DB.Model(&models.HolderRevenue{}).
		Where("wallet_addr = ? AND status = ?", walletAddr, models.HolderRevenueStatusPending).
		Select("COALESCE(SUM(amount), 0)").Scan(&totalPending)

	// Get recent revenue records
	var recentRevenue []models.HolderRevenue
	database.DB.Where("wallet_addr = ?", walletAddr).
		Preload("Shell").
		Order("created_at DESC").
		Limit(20).
		Find(&recentRevenue)

	// Build shell summaries
	var shellSummaries []map[string]interface{}
	for _, s := range shells {
		// Get current period usage
		var usage models.SoulUsage
		database.DB.Where("shell_id = ? AND period = ?", s.ID, currentPeriod()).First(&usage)

		shellSummaries = append(shellSummaries, map[string]interface{}{
			"handle":        s.Handle,
			"stage":         s.Stage,
			"avatar_url":    s.AvatarURL,
			"current_usage": usage.UsageCount,
		})
	}

	return map[string]interface{}{
		"total_earned":   totalEarned,
		"total_pending":  totalPending,
		"shells":         shellSummaries,
		"recent_revenue": recentRevenue,
	}, nil
}

// GetRevenueForPeriod returns detailed revenue for a specific period.
func GetRevenueForPeriod(walletAddr, period string) ([]models.HolderRevenue, error) {
	var revenues []models.HolderRevenue
	if err := database.DB.Where("wallet_addr = ? AND period = ?", walletAddr, period).
		Preload("Shell").
		Find(&revenues).Error; err != nil {
		return nil, err
	}
	return revenues, nil
}

// AddToRevenuePool adds a pre-calculated amount to the monthly holder revenue pool.
// The caller is responsible for computing the correct pool amount (e.g. 10% of revenue).
// TotalRevenue tracks the cumulative pool deposits for the period.
func AddToRevenuePool(poolAmount float64) {
	if poolAmount <= 0 {
		return
	}
	period := currentPeriod()

	var pool models.RevenuePool
	if err := database.DB.Where("period = ?", period).First(&pool).Error; err != nil {
		pool = models.RevenuePool{
			Period:       period,
			TotalRevenue: poolAmount,
			PoolAmount:   poolAmount,
		}
		database.DB.Create(&pool)
	} else {
		pool.TotalRevenue += poolAmount
		pool.PoolAmount += poolAmount
		database.DB.Save(&pool)
	}
}

// RunMonthlySettlement performs the monthly revenue settlement.
// Called on the 1st of each month for the previous month.
func RunMonthlySettlement() {
	period := previousPeriod()
	util.Log.Info("[revenue] Starting monthly settlement for period %s", period)

	if err := CalculateMonthlyRevenue(period); err != nil {
		util.Log.Error("[revenue] Monthly settlement failed: %v", err)
		return
	}

	util.Log.Info("[revenue] Monthly settlement completed for period %s", period)
}

// StartMonthlySettlement starts the monthly settlement scheduler.
// Runs on the 1st of each month at 00:00 UTC.
func StartMonthlySettlement() {
	go func() {
		for {
			now := time.Now().UTC()
			// Find next 1st of month at 00:00 UTC
			next := time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, time.UTC)
			time.Sleep(time.Until(next))

			RunMonthlySettlement()
		}
	}()
	util.Log.Info("[revenue] Monthly settlement scheduler started (runs 1st of each month)")
}
