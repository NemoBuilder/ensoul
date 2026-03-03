package handlers

import (
	"net/http"
	"strconv"

	"github.com/ensoul-labs/ensoul-server/models"
	"github.com/ensoul-labs/ensoul-server/services"
	"github.com/gin-gonic/gin"
)

// ═══════════════════════════════════════════════════════════════════════
// Admin User Management Handlers
// ═══════════════════════════════════════════════════════════════════════

// AdminListUsers handles GET /api/admin/users
func AdminListUsers(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	params := services.AdminUserListParams{
		Page:         page,
		PageSize:     pageSize,
		Search:       c.Query("search"),
		Status:       c.Query("status"),
		Subscription: c.Query("subscription"),
		Sort:         c.Query("sort"),
		Order:        c.Query("order"),
	}

	items, total, err := services.AdminListUsers(params)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"items":     items,
		"total":     total,
		"page":      params.Page,
		"page_size": params.PageSize,
	})
}

// AdminGetUser handles GET /api/admin/users/:wallet
func AdminGetUser(c *gin.Context) {
	wallet := c.Param("wallet")
	if wallet == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "wallet address is required"})
		return
	}

	detail, err := services.AdminGetUserDetail(wallet)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, detail)
}

// AdminBanUser handles POST /api/admin/users/:wallet/ban
func AdminBanUser(c *gin.Context) {
	admin := getAdminFromContext(c)
	if admin == nil || admin.Role != models.AdminRoleSuperAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "Super admin access required"})
		return
	}

	wallet := c.Param("wallet")
	var req struct {
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		req.Reason = ""
	}

	if err := services.AdminBanUser(wallet, req.Reason, admin); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "banned", "wallet_addr": wallet})
}

// AdminUnbanUser handles POST /api/admin/users/:wallet/unban
func AdminUnbanUser(c *gin.Context) {
	admin := getAdminFromContext(c)
	if admin == nil || admin.Role != models.AdminRoleSuperAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "Super admin access required"})
		return
	}

	wallet := c.Param("wallet")

	if err := services.AdminUnbanUser(wallet, admin); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "active", "wallet_addr": wallet})
}

// AdminUpdateUserNote handles PUT /api/admin/users/:wallet/note
func AdminUpdateUserNote(c *gin.Context) {
	admin := getAdminFromContext(c)
	wallet := c.Param("wallet")

	var req struct {
		Note string `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "note field is required"})
		return
	}

	if err := services.AdminUpdateUserNote(wallet, req.Note, admin); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "updated", "wallet_addr": wallet})
}

// AdminGrantSubscription handles POST /api/admin/users/:wallet/subscription/grant
func AdminGrantSubscription(c *gin.Context) {
	admin := getAdminFromContext(c)
	wallet := c.Param("wallet")

	var req struct {
		Tier   string `json:"tier" binding:"required"`
		Days   int    `json:"days" binding:"required"`
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tier and days are required"})
		return
	}
	if req.Days < 1 || req.Days > 365 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "days must be between 1 and 365"})
		return
	}

	if err := services.AdminGrantSubscription(wallet, req.Tier, req.Days, req.Reason, admin); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "granted", "wallet_addr": wallet, "tier": req.Tier, "days": req.Days})
}

// AdminExtendSubscription handles POST /api/admin/users/:wallet/subscription/extend
func AdminExtendSubscription(c *gin.Context) {
	admin := getAdminFromContext(c)
	wallet := c.Param("wallet")

	var req struct {
		Days   int    `json:"days" binding:"required"`
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "days is required"})
		return
	}
	if req.Days < 1 || req.Days > 365 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "days must be between 1 and 365"})
		return
	}

	if err := services.AdminExtendSubscription(wallet, req.Days, req.Reason, admin); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "extended", "wallet_addr": wallet, "days": req.Days})
}

// AdminRevokeSubscription handles POST /api/admin/users/:wallet/subscription/revoke
func AdminRevokeSubscription(c *gin.Context) {
	admin := getAdminFromContext(c)
	if admin == nil || admin.Role != models.AdminRoleSuperAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "Super admin access required"})
		return
	}

	wallet := c.Param("wallet")
	var req struct {
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		req.Reason = ""
	}

	if err := services.AdminRevokeSubscription(wallet, req.Reason, admin); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "revoked", "wallet_addr": wallet})
}

// AdminUserStats handles GET /api/admin/users/stats
func AdminUserStats(c *gin.Context) {
	stats, err := services.AdminGetUserStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// AdminAuditLog handles GET /api/admin/audit-log
func AdminAuditLog(c *gin.Context) {
	admin := getAdminFromContext(c)
	if admin == nil || admin.Role != models.AdminRoleSuperAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "Super admin access required"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	params := services.AdminAuditLogParams{
		Page:     page,
		PageSize: pageSize,
		Action:   c.Query("action"),
		TargetID: c.Query("target_id"),
	}

	logs, total, err := services.AdminListAuditLogs(params)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"items": logs,
		"total": total,
		"page":  params.Page,
	})
}
