package handlers

import (
	"net/http"
	"strings"

	"github.com/ensoul-labs/ensoul-server/database"
	"github.com/ensoul-labs/ensoul-server/models"
	"github.com/ensoul-labs/ensoul-server/services"
	"github.com/ensoul-labs/ensoul-server/services/methodology"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ═══════════════════════════════════════════════════════════════════════
// Admin: Mentor Methodology CRUD
// ═══════════════════════════════════════════════════════════════════════
//
// Source-attribution rules enforced by these endpoints:
//   - Create: always uses source = "internal-ensoul" (admin custom records).
//     To replace bundled records (x-mentor-skill@v2.0), edit md and run
//     `cmd/seed_methodology --force`.
//   - Update/Delete on records with source != "internal-ensoul" require
//     ?force=true to prevent accidentally clobbering imported sources.
//   - Delete is always soft (Enabled=false). Hard delete via ?hard=true.

// AdminListMethodology handles GET /api/admin/methodology
// Query params:
//
//	?category=  (reference|mental_model|heuristic|routing)
//	?source=    (filter by source tag, default all)
//	?locale=    (default all)
//	?enabled=   (true|false, default all)
//	?q=         (substring match in title/summary/tags)
func AdminListMethodology(c *gin.Context) {
	q := database.DB.Model(&models.MentorMethodology{})

	if v := c.Query("category"); v != "" {
		q = q.Where("category = ?", v)
	}
	if v := c.Query("source"); v != "" {
		q = q.Where("source = ?", v)
	}
	if v := c.Query("locale"); v != "" {
		q = q.Where("locale = ?", v)
	}
	if v := c.Query("enabled"); v != "" {
		q = q.Where("enabled = ?", v == "true")
	}
	if v := strings.TrimSpace(c.Query("q")); v != "" {
		like := "%" + v + "%"
		q = q.Where("title ILIKE ? OR summary ILIKE ? OR tags ILIKE ?", like, like, like)
	}

	var rows []models.MentorMethodology
	if err := q.Order("category ASC, priority DESC, slug ASC").
		Limit(500).Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Stats for the management UI
	type stat struct {
		Category string `json:"category"`
		Source   string `json:"source"`
		N        int64  `json:"n"`
	}
	var stats []stat
	database.DB.Model(&models.MentorMethodology{}).
		Select("category, source, count(*) as n").
		Group("category, source").Scan(&stats)

	c.JSON(http.StatusOK, gin.H{
		"records": rows,
		"total":   len(rows),
		"stats":   stats,
	})
}

// AdminGetMethodology handles GET /api/admin/methodology/:id
func AdminGetMethodology(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var row models.MentorMethodology
	if err := database.DB.First(&row, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusOK, row)
}

type methodologyWriteReq struct {
	Category string `json:"category" binding:"required"`
	Slug     string `json:"slug" binding:"required"`
	Locale   string `json:"locale"`
	Title    string `json:"title" binding:"required"`
	Summary  string `json:"summary"`
	BodyMD   string `json:"body_md" binding:"required"`
	Tags     string `json:"tags"`
	Priority int    `json:"priority"`
	Enabled  *bool  `json:"enabled"`
}

var allowedCategories = map[string]bool{
	models.MentorCategoryReference:   true,
	models.MentorCategoryMentalModel: true,
	models.MentorCategoryHeuristic:   true,
	models.MentorCategoryRouting:     true,
}

// AdminCreateMethodology handles POST /api/admin/methodology
// Always creates with source = "internal-ensoul" (admin custom records).
func AdminCreateMethodology(c *gin.Context) {
	admin := getAdminFromContext(c)
	var req methodologyWriteReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !allowedCategories[req.Category] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid category"})
		return
	}
	locale := req.Locale
	if locale == "" {
		locale = "zh"
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	priority := req.Priority
	if priority == 0 {
		priority = 50
	}

	row := models.MentorMethodology{
		Category: req.Category,
		Slug:     strings.TrimSpace(req.Slug),
		Locale:   locale,
		Title:    req.Title,
		Summary:  req.Summary,
		BodyMD:   req.BodyMD,
		Tags:     req.Tags,
		Source:   "internal-ensoul",
		Version:  "1.0",
		Enabled:  enabled,
		Priority: priority,
	}
	if err := database.DB.Create(&row).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "create failed (slug may collide): " + err.Error()})
		return
	}

	services.WriteAuditLog(admin, "methodology.create", "methodology", row.ID.String(),
		map[string]interface{}{"slug": row.Slug, "category": row.Category}, c.ClientIP())

	c.JSON(http.StatusOK, row)
}

// AdminUpdateMethodology handles PUT /api/admin/methodology/:id
// Records with source != "internal-ensoul" require ?force=true to update,
// to prevent accidentally clobbering imported pack content (which would be
// re-overwritten on next seed --force run anyway).
func AdminUpdateMethodology(c *gin.Context) {
	admin := getAdminFromContext(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var row models.MentorMethodology
	if err := database.DB.First(&row, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}

	if row.Source != "internal-ensoul" && c.Query("force") != "true" {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "this record was imported from " + row.Source +
				"; updating it will be reverted by next seed --force. Pass ?force=true to confirm.",
		})
		return
	}

	var req methodologyWriteReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !allowedCategories[req.Category] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid category"})
		return
	}

	row.Category = req.Category
	row.Slug = strings.TrimSpace(req.Slug)
	if req.Locale != "" {
		row.Locale = req.Locale
	}
	row.Title = req.Title
	row.Summary = req.Summary
	row.BodyMD = req.BodyMD
	row.Tags = req.Tags
	if req.Priority != 0 {
		row.Priority = req.Priority
	}
	if req.Enabled != nil {
		row.Enabled = *req.Enabled
	}
	if err := database.DB.Save(&row).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	services.WriteAuditLog(admin, "methodology.update", "methodology", row.ID.String(),
		map[string]interface{}{"slug": row.Slug, "source": row.Source}, c.ClientIP())

	c.JSON(http.StatusOK, row)
}

// AdminDeleteMethodology handles DELETE /api/admin/methodology/:id
//
// Default = soft delete (set enabled=false), preserving record so
// seed --force can re-enable it later. Pass ?hard=true to actually remove
// from DB (only allowed for source=internal-ensoul to prevent accidental
// data loss on imported packs).
func AdminDeleteMethodology(c *gin.Context) {
	admin := getAdminFromContext(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var row models.MentorMethodology
	if err := database.DB.First(&row, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}

	hard := c.Query("hard") == "true"
	if hard && row.Source != "internal-ensoul" {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "hard delete is only allowed for source=internal-ensoul records (this is " + row.Source + "). Use soft delete (default) instead.",
		})
		return
	}

	if hard {
		if err := database.DB.Unscoped().Delete(&row).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	} else {
		if err := database.DB.Model(&row).Update("enabled", false).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	action := "methodology.disable"
	if hard {
		action = "methodology.delete"
	}
	services.WriteAuditLog(admin, action, "methodology", row.ID.String(),
		map[string]interface{}{"slug": row.Slug, "source": row.Source, "hard": hard}, c.ClientIP())

	c.JSON(http.StatusOK, gin.H{"ok": true, "hard": hard, "id": row.ID})
}

// AdminPreviewMethodology handles POST /api/admin/methodology/preview
// Body: { "message": "<user message>" }
// Returns scenario classification + which records would be loaded for that
// message — useful for testing methodology routing without sending a real chat.
func AdminPreviewMethodology(c *gin.Context) {
	var req struct {
		Message string `json:"message" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "message is required"})
		return
	}
	res, err := methodology.Load(database.DB, req.Message, methodology.LoadOptions{MaxBodyChars: 12000})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	rendered := res.RenderPromptSection()
	c.JSON(http.StatusOK, gin.H{
		"scenario":     res.Scenario,
		"used_slugs":   res.UsedSlugs,
		"heuristics":   len(res.Heuristics),
		"references":   len(res.References),
		"mental_models": len(res.MentalModels),
		"prompt_chars": len(rendered),
		"prompt":       rendered,
	})
}

// AdminMethodologyFeedback handles GET /api/admin/methodology/feedback
// Returns feedback aggregated by scenario for the last N days (default 30).
func AdminMethodologyFeedback(c *gin.Context) {
	type row struct {
		Scenario string `json:"scenario"`
		Up       int64  `json:"up"`
		Down     int64  `json:"down"`
		Total    int64  `json:"total"`
	}
	var rows []row
	if err := database.DB.Raw(`
		SELECT
			COALESCE(NULLIF(scenario, ''), 'general') AS scenario,
			SUM(CASE WHEN feedback = 1 THEN 1 ELSE 0 END) AS up,
			SUM(CASE WHEN feedback = -1 THEN 1 ELSE 0 END) AS down,
			COUNT(*) AS total
		FROM vibe_chat_messages
		WHERE role = 'assistant'
		  AND created_at > NOW() - INTERVAL '30 days'
		GROUP BY 1
		ORDER BY total DESC
	`).Scan(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"window_days": 30, "rows": rows})
}
