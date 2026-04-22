package handlers

import (
	"io"
	"net/http"

	"github.com/ensoul-labs/ensoul-server/database"
	"github.com/ensoul-labs/ensoul-server/models"
	"github.com/ensoul-labs/ensoul-server/services"
	"github.com/ensoul-labs/ensoul-server/util"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// BillingCheckout handles POST /api/billing/checkout
// Creates a LemonSqueezy checkout session and returns the URL.
func BillingCheckout(c *gin.Context) {
	userID, email, ok := getEmailSessionUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "login required"})
		return
	}

	// Check if already Pro
	var user models.User
	if err := database.DB.First(&user, "id = ?", userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	// Pro users may purchase again to extend their subscription. The webhook
	// (HandleSubscriptionWebhook) already extends pro_expires_at idempotently.

	checkoutURL, err := services.CreateCheckoutURL(userID, email)
	if err != nil {
		util.Log.Error("[billing] Failed to create checkout: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create checkout session"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"url": checkoutURL})
}

// BillingWebhook handles POST /api/billing/webhook
// Receives LemonSqueezy webhook events.
func BillingWebhook(c *gin.Context) {
	payload, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read body"})
		return
	}

	// Verify signature
	signature := c.GetHeader("X-Signature")
	if !services.VerifyWebhookSignature(payload, signature) {
		util.Log.Warn("[billing] Invalid webhook signature")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid signature"})
		return
	}

	if err := services.HandleSubscriptionWebhook(payload); err != nil {
		util.Log.Error("[billing] Webhook processing error: %v", err)
		// Return 200 anyway to prevent retries for non-retryable errors
		c.JSON(http.StatusOK, gin.H{"received": true, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"received": true})
}

// BillingStatus handles GET /api/billing/status
// Returns current subscription and credits info.
func BillingStatus(c *gin.Context) {
	userID, _, ok := getEmailSessionUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "login required"})
		return
	}

	status, err := services.GetBillingStatus(userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, status)
}

// getEmailSessionUser extracts the user ID and email from the email session cookie.
func getEmailSessionUser(c *gin.Context) (uuid.UUID, string, bool) {
	token, err := c.Cookie(emailSessionCookieName)
	if err != nil || token == "" {
		return uuid.Nil, "", false
	}

	tokenHash := util.HashToken(token)
	var session models.EmailSession
	if err := database.DB.Where("token_hash = ? AND expires_at > NOW()", tokenHash).
		First(&session).Error; err != nil {
		return uuid.Nil, "", false
	}

	return session.UserID, session.Email, true
}
