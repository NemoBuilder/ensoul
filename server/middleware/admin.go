package middleware

import (
	"crypto/subtle"
	"net/http"
	"strings"
	"time"

	"github.com/ensoul-labs/ensoul-server/config"
	"github.com/ensoul-labs/ensoul-server/database"
	"github.com/ensoul-labs/ensoul-server/models"
	"github.com/ensoul-labs/ensoul-server/util"
	"github.com/gin-gonic/gin"
)

const adminCookieName = "ensoul_admin"

// AuthAdmin verifies admin access via one of two methods (checked in order):
//  1. API key — Authorization: Bearer <ADMIN_API_KEY>  (for scripts / CI)
//  2. Admin session cookie — ensoul_admin (for browser-based admin UI)
//
// On success, sets "admin_user" in the Gin context (may be nil for API-key auth
// if no corresponding AdminUser record is linked).
//
// Usage:
//
//	admin := api.Group("/admin", middleware.AuthAdmin())
func AuthAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		// ── Method 1: API key ──────────────────────────────────────────
		if authHeader := c.GetHeader("Authorization"); authHeader != "" {
			if tryAPIKeyAuth(c, authHeader) {
				c.Next()
				return
			}
			// If header was present but invalid, abort immediately
			return
		}

		// ── Method 2: Admin session cookie ─────────────────────────────
		if tryAdminSessionAuth(c) {
			c.Next()
			return
		}

		// Neither method succeeded
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Admin authentication required (API key or login session)"})
		c.Abort()
	}
}

// tryAPIKeyAuth validates the Authorization: Bearer <key> header.
// Returns true if auth succeeded, false otherwise (also aborts on format errors).
func tryAPIKeyAuth(c *gin.Context, authHeader string) bool {
	if config.Cfg.AdminAPIKey == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Admin API key not configured"})
		c.Abort()
		return false
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid authorization format, expected: Bearer <key>"})
		c.Abort()
		return false
	}

	provided := parts[1]
	expected := config.Cfg.AdminAPIKey

	// Constant-time comparison to prevent timing attacks
	if subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) != 1 {
		c.JSON(http.StatusForbidden, gin.H{"error": "Invalid admin API key"})
		c.Abort()
		return false
	}

	// API key auth succeeded — no specific AdminUser linked
	// Set a synthetic admin context so handlers can detect auth method
	c.Set("admin_auth_method", "api_key")
	return true
}

// tryAdminSessionAuth validates the ensoul_admin session cookie.
// Returns true if a valid, non-expired admin session is found.
func tryAdminSessionAuth(c *gin.Context) bool {
	token, err := c.Cookie(adminCookieName)
	if err != nil || token == "" {
		return false
	}

	tokenHash := util.HashToken(token)
	var session models.AdminSession
	if err := database.DB.Where("token_hash = ? AND expires_at > ?", tokenHash, time.Now()).First(&session).Error; err != nil {
		return false
	}

	// Load the admin user
	var admin models.AdminUser
	if err := database.DB.First(&admin, "id = ?", session.AdminUserID).Error; err != nil {
		return false
	}

	c.Set("admin_user", &admin)
	c.Set("admin_auth_method", "session")
	return true
}

// GetAdminUser retrieves the authenticated admin user from context.
// Returns nil if authenticated via API key (no user record) or not authenticated.
func GetAdminUser(c *gin.Context) *models.AdminUser {
	if user, exists := c.Get("admin_user"); exists {
		if admin, ok := user.(*models.AdminUser); ok {
			return admin
		}
	}
	return nil
}
