package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/ensoul-labs/ensoul-server/config"
	"github.com/ensoul-labs/ensoul-server/database"
	"github.com/ensoul-labs/ensoul-server/models"
	"github.com/ensoul-labs/ensoul-server/services"
	"github.com/ensoul-labs/ensoul-server/util"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var emailRegexp = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

// EmailSendCode handles POST /api/auth/email/send-code
// Sends a 6-digit verification code to the given email.
func EmailSendCode(c *gin.Context) {
	var req struct {
		Email string `json:"email" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "email is required"})
		return
	}

	email := strings.ToLower(strings.TrimSpace(req.Email))
	if !emailRegexp.MatchString(email) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid email format"})
		return
	}

	// Check if email is banned
	var user models.User
	if err := database.DB.Where("email = ?", email).First(&user).Error; err == nil {
		if user.Status == models.UserStatusBanned {
			c.JSON(http.StatusForbidden, gin.H{"error": "account has been banned", "reason": user.BanReason})
			return
		}
	}

	if err := services.SendEmailCode(email); err != nil {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "verification code sent"})
}

// EmailVerify handles POST /api/auth/email/verify
// Verifies the code and creates a session (login or signup).
func EmailVerify(c *gin.Context) {
	var req struct {
		Email string `json:"email" binding:"required"`
		Code  string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "email and code are required"})
		return
	}

	email := strings.ToLower(strings.TrimSpace(req.Email))
	code := strings.TrimSpace(req.Code)

	if !emailRegexp.MatchString(email) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid email format"})
		return
	}

	if len(code) != 6 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid code format"})
		return
	}

	// Verify code
	if !services.VerifyEmailCode(email, code) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired code"})
		return
	}

	// Find or create user
	now := time.Now()
	var user models.User
	result := database.DB.Where("email = ?", email).First(&user)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			// New user — create
			user = models.User{
				Email:         email,
				EmailVerified: true,
				Status:        models.UserStatusActive,
				Credits:       50,
				CreditsReset:  now.Truncate(24*time.Hour).AddDate(0, 1, 0),
				FirstSeenAt:   now,
				LastSeenAt:    now,
				LoginCount:    1,
			}
			if err := database.DB.Create(&user).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create account"})
				return
			}
			util.Log.Info("[auth] New email user created: %s", email)
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}
	} else {
		// Existing user — update
		if user.Status == models.UserStatusBanned {
			c.JSON(http.StatusForbidden, gin.H{"error": "account has been banned", "reason": user.BanReason})
			return
		}
		database.DB.Model(&user).Updates(map[string]interface{}{
			"email_verified": true,
			"last_seen_at":   now,
			"login_count":    gorm.Expr("login_count + 1"),
		})
	}

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

	// Code is consumed only after the full verify→create-user→create-session
	// flow has succeeded, so a transient failure earlier doesn't burn the
	// user's only chance to use this code.
	services.ConsumeEmailCode(email, code)

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
		"is_new":  user.LoginCount <= 1,
		"message": "logged in successfully",
	})
}

// EmailLogout handles POST /api/auth/email/logout
func EmailLogout(c *gin.Context) {
	token, err := c.Cookie(emailSessionCookieName)
	if err == nil && token != "" {
		tokenHash := util.HashToken(token)
		database.DB.Where("token_hash = ?", tokenHash).Delete(&models.EmailSession{})
	}

	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(emailSessionCookieName, "", -1, "/", "", config.Cfg.IsProduction(), true)

	c.JSON(http.StatusOK, gin.H{"message": "logged out"})
}

// EmailSession handles GET /api/auth/email/session
// Returns the current email session info.
func EmailSessionInfo(c *gin.Context) {
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

	var user models.User
	if err := database.DB.First(&user, session.UserID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user_id":        user.ID,
		"email":          user.Email,
		"twitter_handle": user.TwitterHandle,
		"wallet_addr":    user.WalletAddr,
		"is_pro":         user.IsPro(),
		"pro_expires_at": user.ProExpiresAt,
		"credits":        user.Credits,
		"has_password":   user.PasswordHash != "",
	})
}

const emailSessionCookieName = "ensoul_email_session"
