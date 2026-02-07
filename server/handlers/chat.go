package handlers

import (
	"net/http"

	"github.com/ensoul-labs/ensoul-server/services"
	"github.com/gin-gonic/gin"
)

// ChatWithSoul handles POST /api/chat/:handle
// Streams a conversation response from the soul.
func ChatWithSoul(c *gin.Context) {
	handle := c.Param("handle")

	var req struct {
		Message string `json:"message" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "message is required"})
		return
	}

	// Set SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	err := services.ChatWithSoul(c, handle, req.Message)
	if err != nil {
		// If headers already sent, we can't change status code
		c.SSEvent("error", err.Error())
		return
	}
}

// GetStats handles GET /api/stats
// Returns global statistics for the landing page dashboard.
func GetStats(c *gin.Context) {
	stats, err := services.GetGlobalStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// GetTasks handles GET /api/tasks
// Returns the task board — dimensions that need more fragments.
func GetTasks(c *gin.Context) {
	tasks, err := services.GetTaskBoard()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, tasks)
}
