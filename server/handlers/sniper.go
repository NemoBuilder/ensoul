package handlers

import (
	"net/http"

	"github.com/ensoul-labs/ensoul-server/middleware"
	"github.com/ensoul-labs/ensoul-server/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// SniperSubscribe handles POST /api/sniper/subscribe
// Creates or renews a Soul Sniper subscription.
func SniperSubscribe(c *gin.Context) {
	walletAddr := middleware.GetSessionWallet(c)
	if walletAddr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "login required"})
		return
	}

	var req struct {
		Tier          string  `json:"tier" binding:"required"`
		PaymentTxHash string  `json:"payment_tx_hash" binding:"required"`
		PaymentToken  string  `json:"payment_token"` // USDT/BNB/ENSOUL
		PaymentAmount float64 `json:"payment_amount"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tier and payment_tx_hash are required"})
		return
	}

	if req.PaymentToken == "" {
		req.PaymentToken = "USDT"
	}

	// TODO Phase 3: Verify payment on-chain before creating subscription
	// For now, trust the tx_hash (will add verification in production)

	sub, err := services.CreateSubscription(walletAddr, req.Tier, req.PaymentTxHash, req.PaymentToken, req.PaymentAmount)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, sub)
}

// SniperGetSubscription handles GET /api/sniper/subscription
// Returns the user's subscription status.
func SniperGetSubscription(c *gin.Context) {
	walletAddr := middleware.GetSessionWallet(c)
	if walletAddr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "login required"})
		return
	}

	status, err := services.GetSubscriptionStatus(walletAddr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, status)
}

// SniperAddKOL handles POST /api/sniper/kols
// Adds a KOL to the user's tracking list.
func SniperAddKOL(c *gin.Context) {
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

	kol, err := services.AddSniperKOL(walletAddr, cleanHandle)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, kol)
}

// SniperRemoveKOL handles DELETE /api/sniper/kols/:id
// Removes a KOL from the user's tracking list.
func SniperRemoveKOL(c *gin.Context) {
	walletAddr := middleware.GetSessionWallet(c)
	if walletAddr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "login required"})
		return
	}

	kolID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid KOL ID"})
		return
	}

	if err := services.RemoveSniperKOL(walletAddr, kolID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "removed"})
}

// SniperListKOLs handles GET /api/sniper/kols
// Returns the user's tracked KOLs.
func SniperListKOLs(c *gin.Context) {
	walletAddr := middleware.GetSessionWallet(c)
	if walletAddr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "login required"})
		return
	}

	kols, err := services.ListSniperKOLs(walletAddr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"kols": kols})
}

// SniperGenerateReply handles POST /api/sniper/reply
// Generates reply suggestions for a specific tweet.
func SniperGenerateReply(c *gin.Context) {
	walletAddr := middleware.GetSessionWallet(c)
	if walletAddr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "login required"})
		return
	}

	var req struct {
		Handle    string `json:"handle" binding:"required"`
		TweetID   string `json:"tweet_id" binding:"required"`
		TweetText string `json:"tweet_text" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "handle, tweet_id, and tweet_text are required"})
		return
	}

	cleanHandle, err := services.ValidateHandle(req.Handle)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	reply, err := services.GenerateReplies(walletAddr, cleanHandle, req.TweetID, req.TweetText)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, reply)
}

// SniperGetReplies handles GET /api/sniper/replies
// Returns the user's recent generated replies.
func SniperGetReplies(c *gin.Context) {
	walletAddr := middleware.GetSessionWallet(c)
	if walletAddr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "login required"})
		return
	}

	replies, err := services.GetUserReplies(walletAddr, 20)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"replies": replies})
}

// SniperSetPersona handles POST /api/sniper/persona
// Creates or updates the user's persona for reply generation.
func SniperSetPersona(c *gin.Context) {
	walletAddr := middleware.GetSessionWallet(c)
	if walletAddr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "login required"})
		return
	}

	var req struct {
		Bio       string `json:"bio"`
		Style     string `json:"style"`
		Materials string `json:"materials"`
		Language  string `json:"language"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	persona, err := services.SetUserPersona(walletAddr, req.Bio, req.Style, req.Materials, req.Language)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, persona)
}

// SniperGetPersona handles GET /api/sniper/persona
// Returns the user's persona.
func SniperGetPersona(c *gin.Context) {
	walletAddr := middleware.GetSessionWallet(c)
	if walletAddr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "login required"})
		return
	}

	persona, err := services.GetUserPersona(walletAddr)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"configured": false})
		return
	}

	c.JSON(http.StatusOK, gin.H{"configured": true, "persona": persona})
}
