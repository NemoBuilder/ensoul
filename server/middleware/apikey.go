package middleware

import (
	"crypto/subtle"
	"net/http"

	"github.com/ensoul-labs/ensoul-server/config"
	"github.com/gin-gonic/gin"
)

// AuthPushAPIKey validates the X-API-Key header against PUSH_API_KEY.
// Used for the external tweet push endpoint.
func AuthPushAPIKey() gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.GetHeader("X-API-Key")
		if key == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "X-API-Key header is required"})
			c.Abort()
			return
		}

		expected := config.Cfg.PushAPIKey
		if expected == "" {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Push API key not configured on server"})
			c.Abort()
			return
		}

		if subtle.ConstantTimeCompare([]byte(key), []byte(expected)) != 1 {
			c.JSON(http.StatusForbidden, gin.H{"error": "invalid API key"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// AuthVibeWriteAPIKey validates the X-API-Key header against VIBE_WRITE_API_KEY.
// Used for the external snipe endpoint.
func AuthVibeWriteAPIKey() gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.GetHeader("X-API-Key")
		if key == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "X-API-Key header is required"})
			c.Abort()
			return
		}

		expected := config.Cfg.VibeWriteAPIKey
		if expected == "" {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Vibe Write API key not configured on server"})
			c.Abort()
			return
		}

		if subtle.ConstantTimeCompare([]byte(key), []byte(expected)) != 1 {
			c.JSON(http.StatusForbidden, gin.H{"error": "invalid API key"})
			c.Abort()
			return
		}

		// Store caller_id from header or query param
		callerID := c.GetHeader("X-Caller-ID")
		if callerID == "" {
			callerID = "anonymous"
		}
		c.Set("caller_id", callerID)

		c.Next()
	}
}
