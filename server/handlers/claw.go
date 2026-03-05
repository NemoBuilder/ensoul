package handlers

import (
	"net/http"

	"github.com/ensoul-labs/ensoul-server/database"
	"github.com/ensoul-labs/ensoul-server/middleware"
	"github.com/ensoul-labs/ensoul-server/models"
	"github.com/ensoul-labs/ensoul-server/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ClawRegister handles POST /api/claw/register
// Registers a new Claw (AI agent) and returns api_key + claim info.
func ClawRegister(c *gin.Context) {
	var req struct {
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}

	// Sanitize and validate Claw name to prevent Unicode homoglyph attacks
	cleanName, err := services.ValidateClawName(req.Name)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.Name = cleanName

	if len(req.Description) > 500 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "description too long (max 500 characters)"})
		return
	}

	result, err := services.RegisterClaw(req.Name, req.Description)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to register claw: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, result)
}

// ClawStatus handles GET /api/claw/status
// Returns the claim status of the authenticated Claw.
func ClawStatus(c *gin.Context) {
	claw := middleware.GetClaw(c)
	if claw == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":          claw.Status,
		"claimed":         claw.Status == "claimed",
		"mining_approved": claw.MiningApproved,
		"claim_url":       "/claim/" + claw.ClaimCode,
	})
}

// ClawClaimVerify handles POST /api/claw/claim/verify
// Claims a Claw via wallet session. No tweet verification required.
func ClawClaimVerify(c *gin.Context) {
	addr := middleware.GetSessionWallet(c)
	if addr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Wallet session required to claim a Claw"})
		return
	}

	var req struct {
		ClaimCode string `json:"claim_code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "claim_code is required"})
		return
	}

	result, err := services.ClaimClaw(req.ClaimCode, addr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// ClawMe handles GET /api/claw/me
// Returns information about the authenticated Claw.
func ClawMe(c *gin.Context) {
	claw := middleware.GetClaw(c)
	if claw == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":                claw.ID,
		"name":              claw.Name,
		"description":       claw.Description,
		"claim_code":        claw.ClaimCode,
		"verification_code": claw.VerificationCode,
		"status":            claw.Status,
		"mining_approved":   claw.MiningApproved,
		"twitter_handle":    claw.TwitterHandle,
		"wallet_addr":       claw.WalletAddr,
		"total_submitted":   claw.TotalSubmitted,
		"total_accepted":    claw.TotalAccepted,
		"earnings":          claw.Earnings,
		"created_at":        claw.CreatedAt,
	})
}

// ClawDashboard handles GET /api/claw/dashboard
// Returns dashboard data for the authenticated Claw.
func ClawDashboard(c *gin.Context) {
	claw := middleware.GetClaw(c)
	if claw == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	dashboard, err := services.GetClawDashboard(claw)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, dashboard)
}

// ClawClaimInfo handles GET /api/claw/claim/:code
// Returns public info (name + verification code) for the claim page. No sensitive data.
func ClawClaimInfo(c *gin.Context) {
	code := c.Param("code")
	claw, err := services.GetClawByClaimCode(code)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Claim code not found"})
		return
	}

	// Only expose name, verification code, and status — never claim_code or wallet info
	c.JSON(http.StatusOK, gin.H{
		"name":              claw.Name,
		"verification_code": claw.VerificationCode,
		"status":            claw.Status,
	})
}

// ClawContributions handles GET /api/claw/contributions
// Returns the contribution history of the authenticated Claw.
func ClawContributions(c *gin.Context) {
	claw := middleware.GetClaw(c)
	if claw == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	page := c.DefaultQuery("page", "1")
	limit := c.DefaultQuery("limit", "20")

	result, err := services.GetClawContributions(claw, page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// ClawPublicProfile handles GET /api/claw/profile/:id
// Returns public profile of a Claw including stats and contributions.
func ClawPublicProfile(c *gin.Context) {
	id := c.Param("id")
	result, err := services.GetClawPublicProfile(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

// ClawLeaderboard handles GET /api/claw/leaderboard
// Returns ranked list of Claws by accepted fragments.
func ClawLeaderboard(c *gin.Context) {
	page := c.DefaultQuery("page", "1")
	limit := c.DefaultQuery("limit", "20")
	result, err := services.GetClawLeaderboard(page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

// ShellContributors handles GET /api/shell/:handle/contributors
// Returns top contributors for a specific shell.
func ShellContributors(c *gin.Context) {
	handle := services.SanitizeHandle(c.Param("handle"))
	result, err := services.GetShellContributors(handle)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"contributors": result})
}

// ClawWithdrawCheck handles GET /api/claw/withdraw/check?claw_id=xxx
// Pre-flight check: gas balance, token balance, cooldown, etc.
func ClawWithdrawCheck(c *gin.Context) {
	addr := middleware.GetSessionWallet(c)
	if addr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not logged in"})
		return
	}

	clawIDStr := c.Query("claw_id")
	clawID, err := uuid.Parse(clawIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid claw_id"})
		return
	}

	// Verify this claw is bound to the user's wallet
	var binding models.ClawBinding
	if err := database.DB.Where("wallet_addr = ? AND claw_id = ?", addr, clawID).First(&binding).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Claw not bound to your wallet"})
		return
	}

	status, err := services.CheckWithdraw(clawID, addr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, status)
}

// ClawWithdraw handles POST /api/claw/withdraw
// Initiates a $Ensoul withdrawal from Claw wallet to user wallet.
func ClawWithdraw(c *gin.Context) {
	addr := middleware.GetSessionWallet(c)
	if addr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not logged in"})
		return
	}

	var req struct {
		ClawID string  `json:"claw_id" binding:"required"`
		Amount float64 `json:"amount" binding:"required,gt=0"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "claw_id and amount are required"})
		return
	}

	clawID, err := uuid.Parse(req.ClawID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid claw_id"})
		return
	}

	// Verify this claw is bound to the user's wallet
	var binding models.ClawBinding
	if err := database.DB.Where("wallet_addr = ? AND claw_id = ?", addr, clawID).First(&binding).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Claw not bound to your wallet"})
		return
	}

	record, err := services.ExecuteWithdraw(clawID, addr, req.Amount)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":     "Withdrawal initiated",
		"withdraw_id": record.ID,
		"amount":      record.Amount,
		"from":        record.FromAddr,
		"to":          record.ToAddr,
		"status":      record.Status,
	})
}

// ClawWithdrawHistory handles GET /api/claw/withdraw/history?claw_id=xxx
func ClawWithdrawHistory(c *gin.Context) {
	addr := middleware.GetSessionWallet(c)
	if addr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not logged in"})
		return
	}

	clawIDStr := c.Query("claw_id")
	clawID, err := uuid.Parse(clawIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid claw_id"})
		return
	}

	// Verify binding
	var binding models.ClawBinding
	if err := database.DB.Where("wallet_addr = ? AND claw_id = ?", addr, clawID).First(&binding).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Claw not bound to your wallet"})
		return
	}

	records, err := services.GetWithdrawHistory(clawID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get history"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"withdrawals": records})
}
