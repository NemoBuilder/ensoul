package handlers

import (
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/ensoul-labs/ensoul-server/database"
	"github.com/ensoul-labs/ensoul-server/middleware"
	"github.com/ensoul-labs/ensoul-server/models"
	"github.com/ensoul-labs/ensoul-server/services"
	"github.com/ensoul-labs/ensoul-server/util"
	"github.com/ethereum/go-ethereum/common"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ─── helpers ─────────────────────────────────────────────────

var bindEmailRegexp = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

// currentEmailUser resolves the active email session to a User. Returns nil if not logged in.
func currentEmailUser(c *gin.Context) *models.User {
	token, err := c.Cookie(emailSessionCookieName)
	if err != nil || token == "" {
		return nil
	}
	var session models.EmailSession
	if err := database.DB.Where("token_hash = ? AND expires_at > ?", util.HashToken(token), time.Now()).
		First(&session).Error; err != nil {
		return nil
	}
	var user models.User
	if err := database.DB.First(&user, session.UserID).Error; err != nil {
		return nil
	}
	return &user
}

// currentWalletUser resolves the active wallet session to a User. Returns nil if not logged in.
func currentWalletUser(c *gin.Context) *models.User {
	addr := middleware.GetSessionWallet(c)
	if addr == "" {
		return nil
	}
	var user models.User
	if err := database.DB.Where("wallet_addr = ?", addr).First(&user).Error; err != nil {
		return nil
	}
	return &user
}

// ─── POST /api/auth/bind/wallet ──────────────────────────────
// Binds a wallet to the currently logged-in email user.
// Requires: email session cookie, body { address, signature, message }.

// AuthBindWallet handles POST /api/auth/bind/wallet
func AuthBindWallet(c *gin.Context) {
	user := currentEmailUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "email login required"})
		return
	}

	var req struct {
		Address   string `json:"address" binding:"required"`
		Signature string `json:"signature" binding:"required"`
		Message   string `json:"message" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "address, signature, message required"})
		return
	}

	if !common.IsHexAddress(req.Address) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid wallet address"})
		return
	}
	if !strings.HasPrefix(req.Message, "ensoul:bind:") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid message format (expected ensoul:bind:<timestamp>)"})
		return
	}
	claimed := common.HexToAddress(req.Address)
	if err := middleware.VerifyWalletSignature(req.Message, req.Signature, claimed); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "signature verification failed: " + err.Error()})
		return
	}

	addr := claimed.Hex()

	// Already bound to this user? idempotent success.
	if user.WalletAddr == addr {
		c.JSON(http.StatusOK, gin.H{"wallet_addr": addr, "already_bound": true})
		return
	}
	// User already bound to a different wallet?
	if user.WalletAddr != "" {
		c.JSON(http.StatusConflict, gin.H{"error": "this account already has a bound wallet", "wallet_addr": user.WalletAddr})
		return
	}
	// Wallet already owned by another user?
	var existing models.User
	if err := database.DB.Where("wallet_addr = ?", addr).First(&existing).Error; err == nil && existing.ID != user.ID {
		c.JSON(http.StatusConflict, gin.H{"error": "this wallet is already bound to another account"})
		return
	}

	if err := database.DB.Model(user).Update("wallet_addr", addr).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to bind wallet"})
		return
	}

	// Back-fill any existing wallet sessions / chat sessions with the user_id.
	database.DB.Model(&models.WalletSession{}).Where("wallet_addr = ?", addr).Update("user_id", user.ID)
	database.DB.Model(&models.ChatSession{}).Where("wallet_addr = ?", addr).Update("user_id", user.ID)

	util.Log.Info("[bind] user %s bound wallet %s", user.ID, addr)
	c.JSON(http.StatusOK, gin.H{"wallet_addr": addr, "bound": true})
}

// ─── POST /api/auth/bind/email/send ──────────────────────────
// Sends a verification code for binding an email to the wallet user.

// AuthBindEmailSend handles POST /api/auth/bind/email/send
func AuthBindEmailSend(c *gin.Context) {
	user := currentWalletUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "wallet login required"})
		return
	}

	var req struct {
		Email string `json:"email" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "email required"})
		return
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if !bindEmailRegexp.MatchString(email) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid email format"})
		return
	}

	// Pre-check: if email already belongs to another user, fail fast.
	var existing models.User
	if err := database.DB.Where("email = ?", email).First(&existing).Error; err == nil && existing.ID != user.ID {
		c.JSON(http.StatusConflict, gin.H{"error": "this email is already registered"})
		return
	}

	if err := services.SendEmailCode(email); err != nil {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "verification code sent"})
}

// ─── POST /api/auth/bind/email ───────────────────────────────
// Verifies the code and binds the email to the wallet user.

// AuthBindEmail handles POST /api/auth/bind/email
func AuthBindEmail(c *gin.Context) {
	user := currentWalletUser(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "wallet login required"})
		return
	}

	var req struct {
		Email string `json:"email" binding:"required"`
		Code  string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "email and code required"})
		return
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	code := strings.TrimSpace(req.Code)
	if !bindEmailRegexp.MatchString(email) || len(code) != 6 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid email or code format"})
		return
	}

	// Idempotent
	if user.Email == email {
		c.JSON(http.StatusOK, gin.H{"email": email, "already_bound": true})
		return
	}
	if user.Email != "" {
		c.JSON(http.StatusConflict, gin.H{"error": "this account already has a bound email", "email": user.Email})
		return
	}
	var existing models.User
	if err := database.DB.Where("email = ?", email).First(&existing).Error; err == nil && existing.ID != user.ID {
		c.JSON(http.StatusConflict, gin.H{"error": "this email is already registered"})
		return
	} else if err != nil && err != gorm.ErrRecordNotFound {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
		return
	}

	if !services.VerifyEmailCode(email, code) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired code"})
		return
	}

	if err := database.DB.Model(user).Updates(map[string]interface{}{
		"email":          email,
		"email_verified": true,
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to bind email"})
		return
	}

	// Burn the code only after the bind succeeded.
	services.ConsumeEmailCode(email, code)

	util.Log.Info("[bind] user %s bound email %s", user.ID, email)
	c.JSON(http.StatusOK, gin.H{"email": email, "bound": true})
}
