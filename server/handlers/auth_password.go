package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/ensoul-labs/ensoul-server/config"
	"github.com/ensoul-labs/ensoul-server/database"
	"github.com/ensoul-labs/ensoul-server/models"
	"github.com/ensoul-labs/ensoul-server/util"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// PasswordLogin handles POST /api/auth/email/password-login
// Logs in with email + password (for users who have set a password).
func PasswordLogin(c *gin.Context) {
	var req struct {
		Email    string `json:"email" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "email and password are required"})
		return
	}

	email := strings.ToLower(strings.TrimSpace(req.Email))
	if !emailRegexp.MatchString(email) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid email format"})
		return
	}

	if len(req.Password) < 8 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid credentials"})
		return
	}

	// Find user
	var user models.User
	if err := database.DB.Where("email = ?", email).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
		return
	}

	// Check if user is banned
	if user.Status == models.UserStatusBanned {
		c.JSON(http.StatusForbidden, gin.H{"error": "account has been banned", "reason": user.BanReason})
		return
	}

	// Check if user has a password set
	if user.PasswordHash == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no password set, please use verification code to login"})
		return
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	// Update login stats
	now := time.Now()
	database.DB.Model(&user).Updates(map[string]interface{}{
		"last_seen_at": now,
		"login_count":  gorm.Expr("login_count + 1"),
	})

	// Generate session token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate session"})
		return
	}
	token := hex.EncodeToString(tokenBytes)

	// Delete existing email sessions for this user
	database.DB.Where("user_id = ?", user.ID).Delete(&models.EmailSession{})

	// Create new session
	session := &models.EmailSession{
		TokenHash: util.HashToken(token),
		UserID:    user.ID,
		Email:     email,
		ExpiresAt: time.Now().Add(sessionDuration),
	}
	if err := database.DB.Create(session).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create session"})
		return
	}

	// Set HttpOnly cookie
	secureCookie := config.Cfg.IsProduction()
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(
		emailSessionCookieName,
		token,
		int(sessionDuration.Seconds()),
		"/",
		"",
		secureCookie,
		true,
	)

	c.JSON(http.StatusOK, gin.H{
		"email":   email,
		"user_id": user.ID,
		"message": "logged in successfully",
	})
}

// PasswordSet handles POST /api/auth/email/set-password
// Sets or updates the password for the currently logged-in user.
// Requires active email session.
func PasswordSet(c *gin.Context) {
	token, err := c.Cookie(emailSessionCookieName)
	if err != nil || token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not logged in"})
		return
	}

	tokenHash := util.HashToken(token)
	var session models.EmailSession
	if err := database.DB.Where("token_hash = ? AND expires_at > ?", tokenHash, time.Now()).
		First(&session).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "session expired"})
		return
	}

	var req struct {
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "password is required"})
		return
	}

	if len(req.Password) < 8 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "password must be at least 8 characters"})
		return
	}
	if len(req.Password) > 72 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "password too long"})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to hash password"})
		return
	}

	if err := database.DB.Model(&models.User{}).Where("id = ?", session.UserID).
		Update("password_hash", string(hash)).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to set password"})
		return
	}

	util.Log.Info("[auth] Password set for user: %s", session.Email)

	c.JSON(http.StatusOK, gin.H{"message": "password set successfully"})
}

// PasswordCheck handles GET /api/auth/email/has-password
// Checks if the given email has a password set (for UI to decide which login form to show).
func PasswordCheck(c *gin.Context) {
	email := strings.ToLower(strings.TrimSpace(c.Query("email")))
	if email == "" || !emailRegexp.MatchString(email) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "valid email is required"})
		return
	}

	var user models.User
	if err := database.DB.Where("email = ?", email).First(&user).Error; err != nil {
		// Don't reveal whether the email exists or not
		c.JSON(http.StatusOK, gin.H{"has_password": false})
		return
	}

	c.JSON(http.StatusOK, gin.H{"has_password": user.PasswordHash != ""})
}
