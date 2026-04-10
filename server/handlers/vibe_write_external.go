package handlers

import (
	"net/http"
	"strings"

	"github.com/ensoul-labs/ensoul-server/models"
	"github.com/ensoul-labs/ensoul-server/services"
	"github.com/gin-gonic/gin"
)

// ══════════════════════════════════════════════════════════════════════════════
// Vibe Write External API Handlers
// ══════════════════════════════════════════════════════════════════════════════

// ──────────────────────────────────────────────────────────────────────────────
// Requirement ①: External Tweet Push
// ──────────────────────────────────────────────────────────────────────────────

// VibeWritePushTweets handles POST /api/vibe-write/feed/push
// Accepts tweets from external sources and injects them into the feed.
// Requires X-API-Key header (PUSH_API_KEY).
func VibeWritePushTweets(c *gin.Context) {
	var req struct {
		Tweets []services.ExternalTweetInput `json:"tweets" binding:"required,min=1"`
		Source string                        `json:"source"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tweets array is required (each needs id, text, author_handle, tag_ids)"})
		return
	}

	// Validate: max 100 tweets per push
	if len(req.Tweets) > 100 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "maximum 100 tweets per push"})
		return
	}

	source := req.Source
	if source == "" {
		source = "external"
	}

	injected, err := services.InjectExternalTweets(req.Tweets, source)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"injected": injected,
		"total":    len(req.Tweets),
		"source":   source,
	})
}

// ──────────────────────────────────────────────────────────────────────────────
// Requirement ③: External Snipe API
// ──────────────────────────────────────────────────────────────────────────────

// VibeWriteExternalSnipe handles POST /api/vibe-write/external/snipe
// Generates reply suggestions for a tweet via external API key auth.
// No wallet login or Pro subscription required.
func VibeWriteExternalSnipe(c *gin.Context) {
	var req struct {
		TweetID      string `json:"tweet_id" binding:"required"`
		TweetText    string `json:"tweet_text" binding:"required"`
		AuthorHandle string `json:"author_handle" binding:"required"`
		Language     string `json:"language"`
		TagID        string `json:"tag_id"`
		CallerID     string `json:"caller_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tweet_id, tweet_text, and author_handle are required"})
		return
	}

	// Prefer caller_id from middleware (X-Caller-ID header), fallback to body
	callerID, _ := c.Get("caller_id")
	callerStr, _ := callerID.(string)
	if callerStr == "" || callerStr == "anonymous" {
		if req.CallerID != "" {
			callerStr = req.CallerID
		} else {
			callerStr = "anonymous"
		}
	}

	reply, err := services.ExternalSnipe(callerStr, req.AuthorHandle, req.TweetID, req.TweetText, req.TagID, req.Language)
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, "limit reached") {
			c.JSON(http.StatusTooManyRequests, gin.H{"error": errMsg})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": errMsg})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"reply_id":  reply.ID,
		"tweet_id":  reply.TweetID,
		"tweet_url": reply.TweetURL,
		"variants":  reply.Replies["variants"],
		"used_soul": reply.UsedSoul,
		"caller_id": callerStr,
	})
}

// ══════════════════════════════════════════════════════════════════════════════
// Requirement ②: Multi-Dimensional Tag Handlers
// ══════════════════════════════════════════════════════════════════════════════

// VibeWriteGetDimensions handles GET /api/vibe-write/dimensions
// Returns all active dimensions with their values. Public endpoint.
func VibeWriteGetDimensions(c *gin.Context) {
	dims, err := services.GetAllDimensions()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"dimensions": dims})
}

// VibeWriteFilterByDimensions handles POST /api/vibe-write/dimensions/filter
// Returns tag IDs matching all the given dimension filters.
// Can be chained with /feed endpoint to get filtered tweets.
func VibeWriteFilterByDimensions(c *gin.Context) {
	var req struct {
		Filters map[string][]string `json:"filters" binding:"required"` // dimension_id → []value_id
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "filters object is required (dimension_id → [value_ids])"})
		return
	}

	tagIDs, err := services.GetTagIDsByDimensions(req.Filters)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"tag_ids": tagIDs,
		"count":   len(tagIDs),
		"filters": req.Filters,
	})
}

// VibeWriteGetTagDimensions handles GET /api/vibe-write/tags/:id/dimensions
// Returns the dimension values for a specific tag.
func VibeWriteGetTagDimensions(c *gin.Context) {
	tagID := c.Param("id")
	if tagID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tag id is required"})
		return
	}

	values, err := services.GetTagDimensions(tagID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"tag_id":     tagID,
		"dimensions": values,
	})
}

// ══════════════════════════════════════════════════════════════════════════════
// Admin: Dimension Management
// ══════════════════════════════════════════════════════════════════════════════

// AdminVibeWriteListDimensions handles GET /api/admin/vibe-write/dimensions
func AdminVibeWriteListDimensions(c *gin.Context) {
	dims, err := services.AdminListDimensions()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"dimensions": dims})
}

// AdminVibeWriteCreateDimension handles POST /api/admin/vibe-write/dimensions
func AdminVibeWriteCreateDimension(c *gin.Context) {
	var req struct {
		ID        string `json:"id" binding:"required"`
		Name      string `json:"name" binding:"required"`
		NameEN    string `json:"name_en"`
		SortOrder int    `json:"sort_order"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id and name are required"})
		return
	}

	dim := &models.TagDimension{
		ID:        req.ID,
		Name:      req.Name,
		NameEN:    req.NameEN,
		SortOrder: req.SortOrder,
		Active:    true,
	}

	if err := services.AdminCreateDimension(dim); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"dimension": dim})
}

// AdminVibeWriteUpdateDimension handles PUT /api/admin/vibe-write/dimensions/:id
func AdminVibeWriteUpdateDimension(c *gin.Context) {
	dimID := c.Param("id")
	if dimID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "dimension id is required"})
		return
	}

	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	delete(req, "id")

	if err := services.AdminUpdateDimension(dimID, req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "updated", "dimension_id": dimID})
}

// AdminVibeWriteCreateDimensionValue handles POST /api/admin/vibe-write/dimensions/:id/values
func AdminVibeWriteCreateDimensionValue(c *gin.Context) {
	dimID := c.Param("id")
	if dimID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "dimension id is required"})
		return
	}

	var req struct {
		ID        string `json:"id" binding:"required"`
		Label     string `json:"label" binding:"required"`
		LabelEN   string `json:"label_en"`
		Icon      string `json:"icon"`
		SortOrder int    `json:"sort_order"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id and label are required"})
		return
	}

	val := &models.TagDimensionValue{
		ID:          req.ID,
		DimensionID: dimID,
		Label:       req.Label,
		LabelEN:     req.LabelEN,
		Icon:        req.Icon,
		SortOrder:   req.SortOrder,
		Active:      true,
	}

	if err := services.AdminCreateDimensionValue(val); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"value": val})
}

// AdminVibeWriteUpdateDimensionValue handles PUT /api/admin/vibe-write/dimensions/values/:id
func AdminVibeWriteUpdateDimensionValue(c *gin.Context) {
	valueID := c.Param("id")
	if valueID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "value id is required"})
		return
	}

	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	delete(req, "id")
	delete(req, "dimension_id")

	if err := services.AdminUpdateDimensionValue(valueID, req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "updated", "value_id": valueID})
}

// AdminVibeWriteSetTagDimensions handles PUT /api/admin/vibe-write/tags/:id/dimensions
// Sets dimension values for a tag (replaces all existing associations).
func AdminVibeWriteSetTagDimensions(c *gin.Context) {
	tagID := c.Param("id")
	if tagID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tag id is required"})
		return
	}

	var req struct {
		DimensionValueIDs []string `json:"dimension_value_ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "dimension_value_ids array is required"})
		return
	}

	if err := services.AdminSetTagDimensions(tagID, req.DimensionValueIDs); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":              "updated",
		"tag_id":              tagID,
		"dimension_value_ids": req.DimensionValueIDs,
	})
}
