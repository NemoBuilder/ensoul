package handlers

import (
	"net/http"
	"strconv"

	"github.com/ensoul-labs/ensoul-server/models"
	"github.com/ensoul-labs/ensoul-server/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ═══════════════════════════════════════════════════════════════════════
// Admin User Management Handlers
// ═══════════════════════════════════════════════════════════════════════

// parseUserID extracts and validates the :id route param as a UUID.
// On failure it writes a 400 response and returns ok=false.
func parseUserID(c *gin.Context) (uuid.UUID, bool) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return uuid.Nil, false
	}
	return id, true
}

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
		AuthType:     c.Query("auth_type"),
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

// AdminGetUser handles GET /api/admin/users/:id
func AdminGetUser(c *gin.Context) {
	id, ok := parseUserID(c)
	if !ok {
		return
	}

	detail, err := services.AdminGetUserDetail(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, detail)
}

// AdminBanUser handles POST /api/admin/users/:id/ban
func AdminBanUser(c *gin.Context) {
	admin := getAdminFromContext(c)
	if admin == nil || admin.Role != models.AdminRoleSuperAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "Super admin access required"})
		return
	}

	id, ok := parseUserID(c)
	if !ok {
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		req.Reason = ""
	}

	if err := services.AdminBanUser(id, req.Reason, admin); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "banned", "id": id})
}

// AdminUnbanUser handles POST /api/admin/users/:id/unban
func AdminUnbanUser(c *gin.Context) {
	admin := getAdminFromContext(c)
	if admin == nil || admin.Role != models.AdminRoleSuperAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "Super admin access required"})
		return
	}

	id, ok := parseUserID(c)
	if !ok {
		return
	}

	if err := services.AdminUnbanUser(id, admin); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "active", "id": id})
}

// AdminUpdateUserNote handles PUT /api/admin/users/:id/note
func AdminUpdateUserNote(c *gin.Context) {
	admin := getAdminFromContext(c)
	id, ok := parseUserID(c)
	if !ok {
		return
	}

	var req struct {
		Note string `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "note field is required"})
		return
	}

	if err := services.AdminUpdateUserNote(id, req.Note, admin); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "updated", "id": id})
}

// AdminGrantSubscription handles POST /api/admin/users/:id/subscription/grant
func AdminGrantSubscription(c *gin.Context) {
	admin := getAdminFromContext(c)
	id, ok := parseUserID(c)
	if !ok {
		return
	}

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

	if err := services.AdminGrantSubscription(id, req.Tier, req.Days, req.Reason, admin); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "granted", "id": id, "tier": req.Tier, "days": req.Days})
}

// AdminExtendSubscription handles POST /api/admin/users/:id/subscription/extend
func AdminExtendSubscription(c *gin.Context) {
	admin := getAdminFromContext(c)
	id, ok := parseUserID(c)
	if !ok {
		return
	}

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

	if err := services.AdminExtendSubscription(id, req.Days, req.Reason, admin); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "extended", "id": id, "days": req.Days})
}

// AdminRevokeSubscription handles POST /api/admin/users/:id/subscription/revoke
func AdminRevokeSubscription(c *gin.Context) {
	admin := getAdminFromContext(c)
	if admin == nil || admin.Role != models.AdminRoleSuperAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "Super admin access required"})
		return
	}

	id, ok := parseUserID(c)
	if !ok {
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		req.Reason = ""
	}

	if err := services.AdminRevokeSubscription(id, req.Reason, admin); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "revoked", "id": id})
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
