package middleware

import (
	"net/http"
	"strings"

	"github.com/ensoul-labs/ensoul-server/database"
	"github.com/ensoul-labs/ensoul-server/models"
	"github.com/ensoul-labs/ensoul-server/util"
	"github.com/gin-gonic/gin"
)

// AuthClaw extracts the API key from the Authorization header,
// hashes it with SHA-256, and looks up the Claw by hash.
func AuthClaw() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header is required"})
			c.Abort()
			return
		}

		// Expect "Bearer <api_key>"
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid authorization format, expected: Bearer <api_key>"})
			c.Abort()
			return
		}

		apiKey := parts[1]
		if apiKey == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "API key is empty"})
			c.Abort()
			return
		}

		// Hash the API key and look up by hash (keys are never stored in plaintext)
		keyHash := util.HashToken(apiKey)
		var claw models.Claw
		if err := database.DB.Where("api_key_hash = ?", keyHash).First(&claw).Error; err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid API key"})
			c.Abort()
			return
		}

		// Inject claw into context
		c.Set("claw", &claw)
		c.Next()
	}
}

// RequireClaimed ensures the authenticated Claw has completed the claim process.
func RequireClaimed() gin.HandlerFunc {
	return func(c *gin.Context) {
		clawVal, exists := c.Get("claw")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			c.Abort()
			return
		}

		claw := clawVal.(*models.Claw)
		if claw.Status != models.ClawStatusClaimed {
			c.JSON(http.StatusForbidden, gin.H{
				"error":     "Claw must complete the claim process before performing this action",
				"status":    claw.Status,
				"claim_url": "/claim/" + claw.ClaimCode,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// GetClaw retrieves the authenticated Claw from the Gin context.
func GetClaw(c *gin.Context) *models.Claw {
	clawVal, exists := c.Get("claw")
	if !exists {
		return nil
	}
	return clawVal.(*models.Claw)
}
