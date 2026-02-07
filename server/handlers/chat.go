package handlers

import (
	"net/http"

	"github.com/ensoul-labs/ensoul-server/middleware"
	"github.com/ensoul-labs/ensoul-server/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ChatCreateSession handles POST /api/chat/:handle/session
// Creates a new chat session. If user is logged in, session is linked to wallet.
func ChatCreateSession(c *gin.Context) {
	handle := c.Param("handle")
	walletAddr := middleware.GetSessionWallet(c)

	session, err := services.CreateChatSession(handle, walletAddr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"session_id": session.ID,
		"tier":       session.Tier,
	})
}

// ChatListSessions handles GET /api/chat/sessions
// Returns logged-in user's chat sessions.
func ChatListSessions(c *gin.Context) {
	walletAddr := middleware.GetSessionWallet(c)
	if walletAddr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "login required"})
		return
	}

	handle := c.Query("handle")
	sessions, err := services.ListChatSessions(walletAddr, handle)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"sessions": sessions})
}

// ChatGetSession handles GET /api/chat/sessions/:id
// Returns a chat session with its messages.
func ChatGetSession(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid session ID"})
		return
	}

	session, err := services.GetChatSession(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	// Only allow session owner or guest sessions to be accessed
	walletAddr := middleware.GetSessionWallet(c)
	if session.WalletAddr != "" && session.WalletAddr != walletAddr {
		c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
		return
	}

	c.JSON(http.StatusOK, session)
}

// ChatDeleteSession handles DELETE /api/chat/sessions/:id
// Deletes a chat session (only by owner).
func ChatDeleteSession(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid session ID"})
		return
	}

	walletAddr := middleware.GetSessionWallet(c)
	if walletAddr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "login required"})
		return
	}

	if err := services.DeleteChatSession(id, walletAddr); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

// ChatSendMessage handles POST /api/chat/sessions/:id/message
// Sends a message in a chat session and streams the response.
func ChatSendMessage(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid session ID"})
		return
	}

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

	if err := services.ChatWithSoul(c, id, req.Message); err != nil {
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
