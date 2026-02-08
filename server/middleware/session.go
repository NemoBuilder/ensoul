package middleware

import (
	"net/http"
	"time"

	"github.com/ensoul-labs/ensoul-server/database"
	"github.com/ensoul-labs/ensoul-server/models"
	"github.com/ensoul-labs/ensoul-server/util"
	"github.com/gin-gonic/gin"
)

const sessionCookieName = "ensoul_session"

// AuthSession validates the session cookie and injects the wallet address.
func AuthSession() gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := c.Cookie(sessionCookieName)
		if err != nil || token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Not logged in"})
			c.Abort()
			return
		}

		tokenHash := util.HashToken(token)
		var session models.WalletSession
		if err := database.DB.Where("token_hash = ? AND expires_at > ?", tokenHash, time.Now()).First(&session).Error; err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Session expired or invalid"})
			c.Abort()
			return
		}

		c.Set("session_wallet", session.WalletAddr)
		c.Next()
	}
}

// GetSessionWallet retrieves the wallet address from the session cookie
// without aborting the request. Returns "" if not logged in.
func GetSessionWallet(c *gin.Context) string {
	// Check if already set by middleware
	if addr, exists := c.Get("session_wallet"); exists {
		return addr.(string)
	}

	// Try reading the cookie directly (for handlers not behind AuthSession middleware)
	token, err := c.Cookie(sessionCookieName)
	if err != nil || token == "" {
		return ""
	}

	tokenHash := util.HashToken(token)
	var session models.WalletSession
	if err := database.DB.Where("token_hash = ? AND expires_at > ?", tokenHash, time.Now()).First(&session).Error; err != nil {
		return ""
	}

	return session.WalletAddr
}
