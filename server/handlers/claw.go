package handlers

import (
	"net/http"

	"github.com/ensoul-labs/ensoul-server/middleware"
	"github.com/ensoul-labs/ensoul-server/services"
	"github.com/gin-gonic/gin"
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
		"status":    claw.Status,
		"claimed":   claw.Status == "claimed",
		"claim_url": "/claim/" + claw.ClaimCode,
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
