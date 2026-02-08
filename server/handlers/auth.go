package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/ensoul-labs/ensoul-server/config"
	"github.com/ensoul-labs/ensoul-server/database"
	"github.com/ensoul-labs/ensoul-server/middleware"
	"github.com/ensoul-labs/ensoul-server/models"
	"github.com/ensoul-labs/ensoul-server/services"
	"github.com/ensoul-labs/ensoul-server/util"
	"github.com/ethereum/go-ethereum/common"
	"github.com/gin-gonic/gin"
)

const (
	sessionCookieName = "ensoul_session"
	sessionDuration   = 7 * 24 * time.Hour // 7 days
)

// AuthLogin handles POST /api/auth/login
// Verifies a wallet signature and creates a session (HttpOnly cookie).
func AuthLogin(c *gin.Context) {
	var req struct {
		Address   string `json:"address" binding:"required"`
		Signature string `json:"signature" binding:"required"`
		Message   string `json:"message" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "address, signature, and message are required"})
		return
	}

	// Validate address format
	if !common.IsHexAddress(req.Address) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid wallet address format"})
		return
	}

	// Validate message format: "ensoul:login:<timestamp>"
	if !strings.HasPrefix(req.Message, "ensoul:login:") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid message format"})
		return
	}

	// Verify signature
	claimed := common.HexToAddress(req.Address)
	if err := middleware.VerifyWalletSignature(req.Message, req.Signature, claimed); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Signature verification failed: " + err.Error()})
		return
	}

	// Generate session token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate session"})
		return
	}
	token := hex.EncodeToString(tokenBytes)

	// Delete any existing sessions for this wallet
	database.DB.Where("wallet_addr = ?", claimed.Hex()).Delete(&models.WalletSession{})

	// Create new session (store hash only, never the raw token)
	session := &models.WalletSession{
		TokenHash:  util.HashToken(token),
		WalletAddr: claimed.Hex(),
		ExpiresAt:  time.Now().Add(sessionDuration),
	}
	if err := database.DB.Create(session).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create session"})
		return
	}

	// Set HttpOnly cookie — Secure=true in production (HTTPS)
	secureCookie := config.Cfg.IsProduction()
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(
		sessionCookieName,
		token,
		int(sessionDuration.Seconds()),
		"/",
		"",           // domain — empty for current domain
		secureCookie, // secure — true in production with HTTPS
		true,         // httpOnly — JS cannot read this
	)

	c.JSON(http.StatusOK, gin.H{
		"address": claimed.Hex(),
		"message": "Logged in successfully",
	})
}

// AuthLogout handles POST /api/auth/logout
// Destroys the session and clears the cookie.
func AuthLogout(c *gin.Context) {
	token, err := c.Cookie(sessionCookieName)
	if err == nil && token != "" {
		tokenHash := util.HashToken(token)
		database.DB.Where("token_hash = ?", tokenHash).Delete(&models.WalletSession{})
	}

	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(sessionCookieName, "", -1, "/", "", config.Cfg.IsProduction(), true)

	c.JSON(http.StatusOK, gin.H{"message": "Logged out"})
}

// AuthSession handles GET /api/auth/session
// Returns the current session info (wallet address) if logged in.
func AuthSession(c *gin.Context) {
	addr := middleware.GetSessionWallet(c)
	if addr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not logged in"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"address": addr,
	})
}

// ClawBindKey handles POST /api/claw/keys
// Binds a Claw API key to the session wallet.
func ClawBindKey(c *gin.Context) {
	addr := middleware.GetSessionWallet(c)
	if addr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not logged in"})
		return
	}

	var req struct {
		APIKey string `json:"api_key" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "api_key is required"})
		return
	}

	// Verify the API key is valid (look up by hash)
	keyHash := util.HashToken(req.APIKey)
	var claw models.Claw
	if err := database.DB.Where("api_key_hash = ?", keyHash).First(&claw).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid API key"})
		return
	}

	// Check if already bound
	var existing models.ClawBinding
	if err := database.DB.Where("wallet_addr = ? AND claw_id = ?", addr, claw.ID).First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "This Claw is already bound to your wallet"})
		return
	}

	// Create binding
	binding := &models.ClawBinding{
		WalletAddr: addr,
		ClawID:     claw.ID,
		ClawName:   claw.Name,
	}
	if err := database.DB.Create(binding).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to bind Claw"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":   binding.ID,
		"name": claw.Name,
	})
}

// ClawListKeys handles GET /api/claw/keys
// Lists all Claws bound to the session wallet.
func ClawListKeys(c *gin.Context) {
	addr := middleware.GetSessionWallet(c)
	if addr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not logged in"})
		return
	}

	var bindings []models.ClawBinding
	database.DB.Where("wallet_addr = ?", addr).Order("created_at ASC").Find(&bindings)

	// Return binding info (without API keys!)
	type bindingInfo struct {
		ID       string `json:"id"`
		ClawID   string `json:"claw_id"`
		ClawName string `json:"claw_name"`
	}
	result := make([]bindingInfo, len(bindings))
	for i, b := range bindings {
		result[i] = bindingInfo{
			ID:       b.ID.String(),
			ClawID:   b.ClawID.String(),
			ClawName: b.ClawName,
		}
	}

	c.JSON(http.StatusOK, gin.H{"claws": result})
}

// ClawUnbindKey handles DELETE /api/claw/keys/:id
// Removes a Claw binding from the session wallet.
func ClawUnbindKey(c *gin.Context) {
	addr := middleware.GetSessionWallet(c)
	if addr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not logged in"})
		return
	}

	id := c.Param("id")
	result := database.DB.Where("id = ? AND wallet_addr = ?", id, addr).Delete(&models.ClawBinding{})
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Binding not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Claw unbound"})
}

// ClawBoundDashboard handles GET /api/claw/keys/:id/dashboard
// Returns the dashboard data for a specific bound Claw.
func ClawBoundDashboard(c *gin.Context) {
	addr := middleware.GetSessionWallet(c)
	if addr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not logged in"})
		return
	}

	id := c.Param("id")

	// Find the binding (must belong to this wallet)
	var binding models.ClawBinding
	if err := database.DB.Where("id = ? AND wallet_addr = ?", id, addr).First(&binding).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Binding not found"})
		return
	}

	// Load the Claw
	var claw models.Claw
	if err := database.DB.First(&claw, "id = ?", binding.ClawID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Claw not found"})
		return
	}

	// Reuse existing dashboard logic
	dashboard, err := services.GetClawDashboard(&claw)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, dashboard)
}
