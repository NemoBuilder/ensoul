package handlers

import (
	"net/http"

	"github.com/ensoul-labs/ensoul-server/middleware"
	"github.com/ensoul-labs/ensoul-server/services"
	"github.com/gin-gonic/gin"
)

// FragmentSubmit handles POST /api/fragment/submit
// Allows a claimed Claw to submit a fragment for a shell.
func FragmentSubmit(c *gin.Context) {
	claw := middleware.GetClaw(c)
	if claw == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	var req struct {
		Handle    string `json:"handle" binding:"required"`
		Dimension string `json:"dimension" binding:"required"`
		Content   string `json:"content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "handle, dimension, and content are required"})
		return
	}

	// Validate dimension
	validDims := map[string]bool{
		"personality": true, "knowledge": true, "stance": true,
		"style": true, "relationship": true, "timeline": true,
	}
	if !validDims[req.Dimension] {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":            "Invalid dimension",
			"valid_dimensions": []string{"personality", "knowledge", "stance", "style", "relationship", "timeline"},
		})
		return
	}

	fragment, err := services.SubmitFragment(claw, req.Handle, req.Dimension, req.Content)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to submit fragment: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, fragment)
}

// FragmentList handles GET /api/fragment/list
// Returns fragments filtered by shell, claw, or status.
func FragmentList(c *gin.Context) {
	shellHandle := c.Query("handle")
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
