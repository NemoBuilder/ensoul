package handlers

import (
	"net/http"

	"github.com/ensoul-labs/ensoul-server/middleware"
	"github.com/ensoul-labs/ensoul-server/services"
	"github.com/gin-gonic/gin"
)

// ClaimInitiate handles POST /api/claim/initiate
// Starts the KOL claim process for a Soul.
func ClaimInitiate(c *gin.Context) {
	walletAddr := middleware.GetSessionWallet(c)
	if walletAddr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "login required"})
		return
	}

	var req struct {
		Handle string `json:"handle" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "handle is required"})
		return
	}

	cleanHandle, err := services.ValidateHandle(req.Handle)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	claim, err := services.InitiateClaim(cleanHandle, walletAddr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"claim_id":    claim.ID,
		"verify_code": claim.VerifyCode,
		"instruction": "Tweet the verification code from your @" + cleanHandle + " account to complete the claim.",
	})
}

// ClaimVerify handles POST /api/claim/verify
// Verifies a KOL's claim by checking their tweet.
func ClaimVerify(c *gin.Context) {
	var req struct {
		Handle  string `json:"handle" binding:"required"`
		TweetID string `json:"tweet_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "handle and tweet_id are required"})
		return
	}

	cleanHandle, err := services.ValidateHandle(req.Handle)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := services.VerifyClaim(cleanHandle, req.TweetID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "verified"})
}

// ClaimStatus handles GET /api/claim/:handle
// Returns the claim status for a soul.
func ClaimStatus(c *gin.Context) {
	handle := services.SanitizeHandle(c.Param("handle"))

	status, err := services.GetClaimStatus(handle)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, status)
}

// HolderClaimRevenue handles POST /api/holder/claim
// Allows a holder to claim their pending revenue.
func HolderClaimRevenue(c *gin.Context) {
	walletAddr := middleware.GetSessionWallet(c)
	if walletAddr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "login required"})
		return
	}

	amount, txHash, err := services.ClaimHolderRevenue(walletAddr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"amount":  amount,
		"tx_hash": txHash,
		"status":  "claimed",
	})
}
