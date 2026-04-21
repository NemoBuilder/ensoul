package handlers

import (
	"net/http"
	"strconv"

	"github.com/ensoul-labs/ensoul-server/services"
	"github.com/gin-gonic/gin"
)

// AdminGiftPro handles POST /api/admin/gift-pro
//
// Body: { "identifier": "<uuid|email|wallet>", "months": 1..24, "reason": "..." }
//
// Idempotent: sets user.pro_expires_at = max(now, current) + months * 30 days.
// Writes a gift_pro_logs row plus an admin_audit_log entry.
func AdminGiftPro(c *gin.Context) {
	admin := getAdminFromContext(c)

	var req struct {
		Identifier string `json:"identifier" binding:"required"`
		Months     int    `json:"months" binding:"required"`
		Reason     string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "identifier and months are required"})
		return
	}

	user, log, err := services.GiftProByIdentifier(req.Identifier, req.Months, req.Reason, admin)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":         "gifted",
		"user_id":        user.ID,
		"user_email":     user.Email,
		"user_wallet":    user.WalletAddr,
		"months":         req.Months,
		"pro_expires_at": user.ProExpiresAt,
		"log":            log,
	})
}

// AdminListGiftProLogs handles GET /api/admin/gift-pro/logs
//
// Query params:
//   - page (default 1)
//   - page_size (default 50, max 200)
//   - user (optional — UUID, email, or wallet to filter)
func AdminListGiftProLogs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))
	logs, total, err := services.ListGiftProLogs(services.GiftProLogParams{
		Page:     page,
		PageSize: pageSize,
		UserID:   c.Query("user"),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"items":     logs,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}
