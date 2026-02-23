package handlers

import (
	"net/http"

	"github.com/ensoul-labs/ensoul-server/database"
	"github.com/ensoul-labs/ensoul-server/models"
	"github.com/ensoul-labs/ensoul-server/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// MiningPoolStatus handles GET /api/mining/pool
// Returns the current mining pool status.
func MiningPoolStatus(c *gin.Context) {
	status, err := services.GetPoolStatus()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get pool status: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, status)
}

// MiningDemands handles GET /api/mining/demands
// Returns the current open fragment demands.
func MiningDemands(c *gin.Context) {
	var demands []models.FragmentDemand
	query := database.DB.Where("status = ?", models.DemandStatusOpen).
		Preload("Shell").
		Order("bounty DESC").
		Limit(100)

	// Optional filter by handle
	if handle := c.Query("handle"); handle != "" {
		var shell models.Shell
		if err := database.DB.Where("LOWER(handle) = LOWER(?)", handle).First(&shell).Error; err == nil {
			query = query.Where("shell_id = ?", shell.ID)
		}
	}

	// Optional filter by dimension
	if dim := c.Query("dimension"); dim != "" {
		query = query.Where("dimension = ?", dim)
	}

	if err := query.Find(&demands).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch demands"})
		return
	}

	// Build response with shell handle info
	type demandResponse struct {
		ID          string  `json:"id"`
		Handle      string  `json:"handle"`
		Dimension   string  `json:"dimension"`
		Description string  `json:"description"`
		Bounty      float64 `json:"bounty"`
		ExpiresAt   string  `json:"expires_at"`
	}

	var resp []demandResponse
	for _, d := range demands {
		resp = append(resp, demandResponse{
			ID:          d.ID.String(),
			Handle:      d.Shell.Handle,
			Dimension:   d.Dimension,
			Description: d.Description,
			Bounty:      d.Bounty,
			ExpiresAt:   d.ExpiresAt.Format("2006-01-02T15:04:05Z"),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"demands": resp,
		"total":   len(resp),
	})
}

// MiningRewards handles GET /api/mining/rewards/:claw_id
// Returns the reward history for a specific Claw.
func MiningRewards(c *gin.Context) {
	clawIDStr := c.Param("claw_id")
	clawID, err := uuid.Parse(clawIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid claw_id"})
		return
	}

	var rewards []models.MiningReward
	database.DB.Where("claw_id = ?", clawID).
		Order("created_at DESC").
		Limit(50).
		Find(&rewards)

	// Calculate totals
	var totalEarned float64
	var totalPending float64
	for _, r := range rewards {
		if r.Status == models.RewardStatusConfirmed {
			totalEarned += r.Amount
		} else if r.Status == models.RewardStatusPending || r.Status == models.RewardStatusSent {
			totalPending += r.Amount
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"rewards":       rewards,
		"total_earned":  totalEarned,
		"total_pending": totalPending,
	})
}

// MiningDeposit handles POST /api/mining/deposit (admin only)
// Manually deposits $Ensoul into the mining pool.
func MiningDeposit(c *gin.Context) {
	var req struct {
		Amount float64 `json:"amount" binding:"required,gt=0"`
		Source string  `json:"source" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "amount and source are required"})
		return
	}

	if err := services.DepositToPool(req.Amount, req.Source); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to deposit: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Deposited successfully",
		"amount":  req.Amount,
		"source":  req.Source,
	})
}
