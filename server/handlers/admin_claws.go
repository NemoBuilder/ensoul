package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/ensoul-labs/ensoul-server/database"
	"github.com/ensoul-labs/ensoul-server/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ═══════════════════════════════════════════════════════════════════════
// Admin: Claw Management (Mining Approval)
// ═══════════════════════════════════════════════════════════════════════

// AdminClawStats handles GET /api/admin/claws/stats
// Returns aggregate stats for the admin dashboard.
func AdminClawStats(c *gin.Context) {
	var totalClaws, claimedClaws, approvedClaws, pendingApproval int64

	database.DB.Model(&models.Claw{}).Count(&totalClaws)
	database.DB.Model(&models.Claw{}).Where("status = ?", models.ClawStatusClaimed).Count(&claimedClaws)
	database.DB.Model(&models.Claw{}).Where("mining_approved = ?", true).Count(&approvedClaws)
	database.DB.Model(&models.Claw{}).Where("status = ? AND mining_approved = ?", models.ClawStatusClaimed, false).Count(&pendingApproval)

	// Total submissions & accepted across all claws
	var totalSubmitted, totalAccepted int64
	database.DB.Model(&models.Claw{}).Select("COALESCE(SUM(total_submitted), 0)").Scan(&totalSubmitted)
	database.DB.Model(&models.Claw{}).Select("COALESCE(SUM(total_accepted), 0)").Scan(&totalAccepted)

	c.JSON(http.StatusOK, gin.H{
		"total_claws":      totalClaws,
		"claimed_claws":    claimedClaws,
		"approved_claws":   approvedClaws,
		"pending_approval": pendingApproval,
		"total_submitted":  totalSubmitted,
		"total_accepted":   totalAccepted,
	})
}

// AdminListClaws handles GET /api/admin/claws
// Query params: ?page=1&page_size=20&search=&status=&mining_approved=&sort=created_at&order=desc
func AdminListClaws(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	search := c.Query("search")
	statusFilter := c.Query("status")
	approvedFilter := c.Query("mining_approved")
	sortField := c.DefaultQuery("sort", "created_at")
	sortOrder := c.DefaultQuery("order", "desc")

	// Validate sort field
	allowedSorts := map[string]bool{
		"created_at":      true,
		"name":            true,
		"total_submitted": true,
		"total_accepted":  true,
		"earnings":        true,
	}
	if !allowedSorts[sortField] {
		sortField = "created_at"
	}
	if sortOrder != "asc" && sortOrder != "desc" {
		sortOrder = "desc"
	}

	query := database.DB.Model(&models.Claw{})

	if search != "" {
		query = query.Where("name ILIKE ? OR wallet_addr ILIKE ?", "%"+search+"%", "%"+search+"%")
	}
	if statusFilter != "" {
		query = query.Where("status = ?", statusFilter)
	}
	if approvedFilter == "true" {
		query = query.Where("mining_approved = ?", true)
	} else if approvedFilter == "false" {
		query = query.Where("mining_approved = ?", false)
	}

	var total int64
	query.Count(&total)

	var claws []models.Claw
	query.Order(sortField + " " + sortOrder).
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&claws)

	// Build response items
	type ClawItem struct {
		ID             uuid.UUID `json:"id"`
		Name           string    `json:"name"`
		Description    string    `json:"description"`
		Status         string    `json:"status"`
		MiningApproved bool      `json:"mining_approved"`
		WalletAddr     string    `json:"wallet_addr"`
		TwitterHandle  string    `json:"twitter_handle"`
		TotalSubmitted int       `json:"total_submitted"`
		TotalAccepted  int       `json:"total_accepted"`
		Earnings       float64   `json:"earnings"`
		Withdrawn      float64   `json:"withdrawn"`
		CreatedAt      time.Time `json:"created_at"`
	}

	items := make([]ClawItem, len(claws))
	for i, cl := range claws {
		items[i] = ClawItem{
			ID:             cl.ID,
			Name:           cl.Name,
			Description:    cl.Description,
			Status:         cl.Status,
			MiningApproved: cl.MiningApproved,
			WalletAddr:     cl.WalletAddr,
			TwitterHandle:  cl.TwitterHandle,
			TotalSubmitted: cl.TotalSubmitted,
			TotalAccepted:  cl.TotalAccepted,
			Earnings:       cl.Earnings,
			Withdrawn:      cl.Withdrawn,
			CreatedAt:      cl.CreatedAt,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"items":     items,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// AdminApproveClaw handles POST /api/admin/claws/:id/approve
// Sets mining_approved = true for the given Claw.
func AdminApproveClaw(c *gin.Context) {
	idStr := c.Param("id")
	clawID, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid claw ID"})
		return
	}

	var claw models.Claw
	if err := database.DB.First(&claw, "id = ?", clawID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Claw not found"})
		return
	}

	if claw.MiningApproved {
		c.JSON(http.StatusOK, gin.H{"status": "already_approved", "claw_id": claw.ID, "name": claw.Name})
		return
	}

	database.DB.Model(&claw).Update("mining_approved", true)

	c.JSON(http.StatusOK, gin.H{
		"status":  "approved",
		"claw_id": claw.ID,
		"name":    claw.Name,
	})
}

// AdminRejectClaw handles POST /api/admin/claws/:id/reject
// Sets mining_approved = false for the given Claw.
func AdminRejectClaw(c *gin.Context) {
	idStr := c.Param("id")
	clawID, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid claw ID"})
		return
	}

	var claw models.Claw
	if err := database.DB.First(&claw, "id = ?", clawID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Claw not found"})
		return
	}

	if !claw.MiningApproved {
		c.JSON(http.StatusOK, gin.H{"status": "already_rejected", "claw_id": claw.ID, "name": claw.Name})
		return
	}

	database.DB.Model(&claw).Update("mining_approved", false)

	c.JSON(http.StatusOK, gin.H{
		"status":  "rejected",
		"claw_id": claw.ID,
		"name":    claw.Name,
	})
}

// AdminBatchApproveClaws handles POST /api/admin/claws/batch-approve
// Body: { "claw_ids": ["uuid-1", "uuid-2"] }
func AdminBatchApproveClaws(c *gin.Context) {
	var req struct {
		ClawIDs []string `json:"claw_ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "claw_ids array is required"})
		return
	}

	approved := 0
	errors := []string{}

	for _, idStr := range req.ClawIDs {
		clawID, err := uuid.Parse(idStr)
		if err != nil {
			errors = append(errors, idStr+": invalid UUID")
			continue
		}

		result := database.DB.Model(&models.Claw{}).Where("id = ?", clawID).Update("mining_approved", true)
		if result.RowsAffected == 0 {
			errors = append(errors, idStr+": not found")
			continue
		}
		approved++
	}

	c.JSON(http.StatusOK, gin.H{
		"approved": approved,
		"errors":   errors,
	})
}
