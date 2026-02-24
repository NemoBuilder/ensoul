package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/ensoul-labs/ensoul-server/config"
	"github.com/ensoul-labs/ensoul-server/database"
	"github.com/ensoul-labs/ensoul-server/models"
	"github.com/ensoul-labs/ensoul-server/util"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

const (
	adminCookieName = "ensoul_admin"
	adminSessionDur = 24 * time.Hour // 24 hours — shorter than wallet sessions
)

// AdminLogin handles POST /api/admin/auth/login
// Authenticates an admin with username/password and sets an HttpOnly session cookie.
func AdminLogin(c *gin.Context) {
	var req struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username and password are required"})
		return
	}

	// Find admin user by username
	var admin models.AdminUser
	if err := database.DB.Where("username = ?", req.Username).First(&admin).Error; err != nil {
		// Use same error message for both invalid username and password (prevent enumeration)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid username or password"})
		return
	}

	// Verify password with bcrypt
	if err := bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid username or password"})
		return
	}

	// Generate session token (32 random bytes → 64 hex chars)
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate session"})
		return
	}
	token := hex.EncodeToString(tokenBytes)

	// Delete existing sessions for this admin
	database.DB.Where("admin_user_id = ?", admin.ID).Delete(&models.AdminSession{})

	// Create new session (store hash only)
	session := &models.AdminSession{
		TokenHash:   util.HashToken(token),
		AdminUserID: admin.ID,
		ExpiresAt:   time.Now().Add(adminSessionDur),
	}
	if err := database.DB.Create(session).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create session"})
		return
	}

	// Update last login time
	now := time.Now()
	database.DB.Model(&admin).Update("last_login_at", &now)

	// Set HttpOnly cookie
	secureCookie := config.Cfg.IsProduction()
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(
		adminCookieName,
		token,
		int(adminSessionDur.Seconds()),
		"/",
		"",
		secureCookie,
		true, // httpOnly
	)

	c.JSON(http.StatusOK, gin.H{
		"username": admin.Username,
		"role":     admin.Role,
		"message":  "Admin logged in successfully",
	})
}

// AdminLogout handles POST /api/admin/auth/logout
// Destroys the admin session and clears the cookie.
func AdminLogout(c *gin.Context) {
	token, err := c.Cookie(adminCookieName)
	if err == nil && token != "" {
		tokenHash := util.HashToken(token)
		database.DB.Where("token_hash = ?", tokenHash).Delete(&models.AdminSession{})
	}

	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(adminCookieName, "", -1, "/", "", config.Cfg.IsProduction(), true)

	c.JSON(http.StatusOK, gin.H{"message": "Admin logged out"})
}

// AdminMe handles GET /api/admin/auth/me
// Returns the current admin session info. Requires admin auth (API key or session).
func AdminMe(c *gin.Context) {
	adminUser := getAdminFromContext(c)
	if adminUser == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not authenticated"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":            adminUser.ID,
		"username":      adminUser.Username,
		"role":          adminUser.Role,
		"last_login_at": adminUser.LastLoginAt,
		"created_at":    adminUser.CreatedAt,
	})
}

// AdminChangePassword handles POST /api/admin/auth/password
// Allows the logged-in admin to change their password.
func AdminChangePassword(c *gin.Context) {
	adminUser := getAdminFromContext(c)
	if adminUser == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not authenticated"})
		return
	}

	var req struct {
		OldPassword string `json:"old_password" binding:"required"`
		NewPassword string `json:"new_password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "old_password and new_password are required"})
		return
	}

	if len(req.NewPassword) < 8 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "New password must be at least 8 characters"})
		return
	}

	// Verify old password
	if err := bcrypt.CompareHashAndPassword([]byte(adminUser.PasswordHash), []byte(req.OldPassword)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Old password is incorrect"})
		return
	}

	// Hash new password
	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	// Update password
	database.DB.Model(adminUser).Update("password_hash", string(hash))

	// Invalidate all sessions (force re-login)
	database.DB.Where("admin_user_id = ?", adminUser.ID).Delete(&models.AdminSession{})

	// Clear current cookie
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(adminCookieName, "", -1, "/", "", config.Cfg.IsProduction(), true)

	c.JSON(http.StatusOK, gin.H{"message": "Password changed. Please log in again."})
}

// getAdminFromContext retrieves the AdminUser set by the AuthAdmin middleware.
func getAdminFromContext(c *gin.Context) *models.AdminUser {
	if user, exists := c.Get("admin_user"); exists {
		if admin, ok := user.(*models.AdminUser); ok {
			return admin
		}
	}
	return nil
}
