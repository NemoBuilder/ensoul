package services

import (
	"fmt"
	"math"
	"time"

	"github.com/ensoul-labs/ensoul-server/database"
	"github.com/ensoul-labs/ensoul-server/models"
	"github.com/ensoul-labs/ensoul-server/util"
	"github.com/google/uuid"
)

// Crab is the AI Agent responsible for fragment demand publishing,
// pricing, and enhanced quality review. It extends the existing
// ReviewFragmentBatch system with economic incentives.

// DimensionGap represents a gap in a Soul's dimension coverage.
type DimensionGap struct {
	ShellID   string `json:"shell_id"`
	Handle    string `json:"handle"`
	Dimension string `json:"dimension"`
	Score     int    `json:"score"`
	Followers int    `json:"followers"`
}

// ScanDimensionGaps scans all confirmed Souls and identifies dimensions
// with scores below 80 that need more fragments.
func ScanDimensionGaps() ([]DimensionGap, error) {
	var shells []models.Shell
	database.DB.Where("stage NOT IN ? AND mint_tx_hash != ''",
		[]string{"pending", models.StagePending}).Find(&shells)

	var gaps []DimensionGap
	dimensions := []string{"personality", "knowledge", "stance", "style", "relationship", "timeline"}

	for _, shell := range shells {
		dims := shell.GetDimensions()
		followers := getFollowers(shell)

		for _, dim := range dimensions {
			d, exists := dims[dim]
			if !exists || d.Score < 80 {
				gaps = append(gaps, DimensionGap{
					ShellID:   shell.ID.String(),
					Handle:    shell.Handle,
					Dimension: dim,
					Score:     d.Score,
					Followers: followers,
				})
			}
		}
	}

	return gaps, nil
}

// PriceDemand calculates the $Ensoul bounty for a fragment demand.
// Formula: bounty = (dailyReleasable × dimensionGapWeight) / totalGapsForSoul × followerCoefficient
// Follower coefficients: MEGA(1M+) 2x, LARGE(100K+) 1.5x, MEDIUM(10K+) 1x, SMALL(<10K) 0.7x
func PriceDemand(gap DimensionGap, dailyReleasable float64, totalGapsForSoul int) float64 {
	if totalGapsForSoul == 0 {
		return MinBounty
	}

	// Dimension gap weight: lower score = higher weight
	gapWeight := float64(100-gap.Score) / 100.0

	// Base bounty from pool
	baseBounty := (dailyReleasable * gapWeight) / float64(totalGapsForSoul)

	// Follower tier coefficient
	coeff := followerCoefficient(gap.Followers)
	bounty := baseBounty * coeff

	// Enforce minimum
	if bounty < MinBounty {
		bounty = MinBounty
	}

	// Round to 2 decimal places
	bounty = math.Round(bounty*100) / 100

	return bounty
}

// followerCoefficient returns the pricing multiplier based on follower count.
func followerCoefficient(followers int) float64 {
	switch {
	case followers >= 1000000:
		return 2.0 // MEGA
	case followers >= 100000:
		return 1.5 // LARGE
	case followers >= 10000:
		return 1.0 // MEDIUM
	default:
		return 0.7 // SMALL
	}
}

// PublishDemands scans for dimension gaps and publishes new fragment demands.
// Called periodically (every 6 hours).
func PublishDemands() error {
	pool, err := GetOrCreateMiningPool()
	if err != nil {
		return err
	}

	// Check if pool is paused
	if pool.Balance < PoolPauseThreshold {
		util.Log.Info("[crab] Mining pool below threshold (%.4f < %.4f), skipping demand publishing",
			pool.Balance, PoolPauseThreshold)
		return nil
	}

	// Calculate daily releasable
	dailyReleasable := pool.Balance * DailyReleaseRate
	remaining := dailyReleasable - pool.DailyReleased
	if remaining <= 0 {
		util.Log.Info("[crab] Daily release limit reached, skipping demand publishing")
		return nil
	}

	// Expire old open demands
	database.DB.Model(&models.FragmentDemand{}).
		Where("status = ? AND expires_at < ?", models.DemandStatusOpen, time.Now()).
		Update("status", models.DemandStatusExpired)

	// Scan gaps
	gaps, err := ScanDimensionGaps()
	if err != nil {
		return fmt.Errorf("failed to scan dimension gaps: %w", err)
	}

	if len(gaps) == 0 {
		util.Log.Info("[crab] No dimension gaps found")
		return nil
	}

	// Group gaps by shell to calculate totalGapsForSoul
	shellGaps := make(map[string][]DimensionGap)
	for _, g := range gaps {
		shellGaps[g.ShellID] = append(shellGaps[g.ShellID], g)
	}

	published := 0
	for shellID, sGaps := range shellGaps {
		// Look up the shell once per group
		var shell models.Shell
		if err := database.DB.Where("id = ?", shellID).First(&shell).Error; err != nil {
			util.Log.Error("[crab] Failed to find shell %s: %v", shellID, err)
			continue
		}

		for _, gap := range sGaps {
			// Skip if there's already an open demand for this shell+dimension
			var existing models.FragmentDemand
			if err := database.DB.Where("shell_id = ? AND dimension = ? AND status = ?",
				shell.ID, gap.Dimension, models.DemandStatusOpen).First(&existing).Error; err == nil {
				continue // already has an open demand
			}

			bounty := PriceDemand(gap, remaining, len(sGaps))

			demand := &models.FragmentDemand{
				ShellID:     shell.ID,
				Dimension:   gap.Dimension,
				Description: fmt.Sprintf("@%s needs more %s fragments (current score: %d)", gap.Handle, gap.Dimension, gap.Score),
				Bounty:      bounty,
				Status:      models.DemandStatusOpen,
				ExpiresAt:   time.Now().Add(48 * time.Hour), // 48h expiry
			}

			if err := database.DB.Create(demand).Error; err != nil {
				util.Log.Error("[crab] Failed to create demand for @%s/%s: %v", gap.Handle, gap.Dimension, err)
				continue
			}

			published++
		}
	}

	util.Log.Info("[crab] Published %d new fragment demands (from %d gaps across %d souls)",
		published, len(gaps), len(shellGaps))
	return nil
}

// ReviewAndReward is called after a fragment passes Crab review.
// It calculates and distributes the mining reward.
func ReviewAndReward(fragment *models.Fragment, confidence float64) error {
	// Find matching open demand
	var demand models.FragmentDemand
	hasDemand := false
	if err := database.DB.Where("shell_id = ? AND dimension = ? AND status = ?",
		fragment.ShellID, fragment.Dimension, models.DemandStatusOpen).
		First(&demand).Error; err == nil {
		hasDemand = true
	}

	// Calculate reward amount
	var rewardAmount float64
	var demandID *uuid.UUID

	if hasDemand {
		// Reward based on demand bounty × quality weight
		rewardAmount = demand.Bounty * confidence
		demandID = &demand.ID

		// Check if demand should be fulfilled (e.g., after enough fragments)
		var acceptedCount int64
		database.DB.Model(&models.Fragment{}).
			Where("shell_id = ? AND dimension = ? AND status = ?",
				fragment.ShellID, fragment.Dimension, models.FragStatusAccepted).
			Count(&acceptedCount)

		if acceptedCount >= 5 {
			database.DB.Model(&demand).Update("status", models.DemandStatusFulfilled)
		}
	} else {
		// Free submission (no matching demand): smaller base reward
		rewardAmount = MinBounty * confidence * 0.5
	}

	if rewardAmount < 1.0 {
		rewardAmount = 1.0 // minimum 1 $Ensoul per accepted fragment
	}

	// Distribute the reward
	if err := DistributeReward(fragment.ClawID, fragment.ID, demandID, rewardAmount); err != nil {
		util.Log.Error("[crab] Failed to distribute reward for fragment %s: %v", fragment.ID, err)
		return err
	}

	util.Log.Info("[crab] Reward %.4f $Ensoul for fragment %s (claw=%s, demand=%v)",
		rewardAmount, fragment.ID, fragment.ClawID, hasDemand)
	return nil
}

// StartCrabScheduler starts the periodic Crab tasks.
func StartCrabScheduler(interval time.Duration) {
	go func() {
		// Initial delay to let the system start up
		time.Sleep(30 * time.Second)

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		// Run once immediately
		if err := PublishDemands(); err != nil {
			util.Log.Error("[crab] Initial demand publishing failed: %v", err)
		}

		for range ticker.C {
			if err := PublishDemands(); err != nil {
				util.Log.Error("[crab] Demand publishing failed: %v", err)
			}
		}
	}()
	util.Log.Info("[crab] Crab scheduler started (interval: %s)", interval)
}
