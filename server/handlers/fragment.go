package handlers

import (
	"net/http"

	"github.com/ensoul-labs/ensoul-server/middleware"
	"github.com/ensoul-labs/ensoul-server/services"
	"github.com/gin-gonic/gin"
)

// FragmentSubmit handles POST /api/fragment/submit (DEPRECATED)
// Returns 410 Gone and directs callers to use the batch endpoint.
func FragmentSubmit(c *gin.Context) {
	c.JSON(http.StatusGone, gin.H{
		"error":   "This endpoint is deprecated. Use POST /api/fragment/batch instead.",
		"message": "Submit all dimensions for a soul in a single batch request. See documentation for the new format.",
		"migrate": "POST /api/fragment/batch with {handle, fragments: [{dimension, content}, ...]}",
	})
}

// FragmentBatchItem is a single fragment in a batch submission.
type FragmentBatchItem struct {
	Dimension string `json:"dimension" binding:"required"`
	Content   string `json:"content" binding:"required"`
}

// FragmentBatch handles POST /api/fragment/batch
// Allows a claimed Claw to submit multiple dimension fragments for a single soul at once.
func FragmentBatch(c *gin.Context) {
	claw := middleware.GetClaw(c)
	if claw == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	var req struct {
		Handle    string              `json:"handle" binding:"required"`
		Fragments []FragmentBatchItem `json:"fragments" binding:"required,min=3,max=6"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request. Required: handle + fragments array (3-6 items)",
			"example": map[string]interface{}{
				"handle": "cz_binance",
				"fragments": []map[string]string{
					{"dimension": "personality", "content": "..."},
					{"dimension": "stance", "content": "..."},
					{"dimension": "style", "content": "..."},
				},
			},
		})
		return
	}

	// Sanitize and validate handle
	cleanHandle, err := services.ValidateHandle(req.Handle)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.Handle = cleanHandle

	// Validate dimensions: each must be valid and no duplicates
	validDims := map[string]bool{
		"personality": true, "knowledge": true, "stance": true,
		"style": true, "relationship": true, "timeline": true,
	}
	seenDims := make(map[string]bool)
	for i, f := range req.Fragments {
		if !validDims[f.Dimension] {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":            "Invalid dimension in fragment " + string(rune('1'+i)),
				"valid_dimensions": []string{"personality", "knowledge", "stance", "style", "relationship", "timeline"},
			})
			return
		}
		if seenDims[f.Dimension] {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Duplicate dimension: " + f.Dimension + ". Each dimension can only appear once per batch.",
			})
			return
		}
		seenDims[f.Dimension] = true

		if len(f.Content) > 5000 {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Content too long for dimension " + f.Dimension + " (max 5000 characters)",
			})
			return
		}
		if len(f.Content) < 50 {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Content too short for dimension " + f.Dimension + " (min 50 characters)",
			})
			return
		}
	}

	// Convert to service layer input
	items := make([]services.BatchFragmentItem, len(req.Fragments))
	for i, f := range req.Fragments {
		items[i] = services.BatchFragmentItem{
			Dimension: f.Dimension,
			Content:   f.Content,
		}
	}

	results, err := services.SubmitFragmentBatch(claw, req.Handle, items)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to submit batch: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"handle":    req.Handle,
		"submitted": len(results),
		"fragments": results,
	})
}

// FragmentList handles GET /api/fragment/list
// Returns fragments filtered by shell, claw, or status.
func FragmentList(c *gin.Context) {
	shellHandle := services.SanitizeHandle(c.Query("handle"))
	status := c.Query("status")
	dimension := c.Query("dimension")
	page := c.DefaultQuery("page", "1")
	limit := c.DefaultQuery("limit", "20")

	result, err := services.ListFragments(shellHandle, status, dimension, page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// FragmentGetByID handles GET /api/fragment/:id
// Returns details of a specific fragment.
func FragmentGetByID(c *gin.Context) {
	id := c.Param("id")

	fragment, err := services.GetFragmentByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Fragment not found"})
		return
	}

	c.JSON(http.StatusOK, fragment)
}
