package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/ensoul-labs/ensoul-server/database"
	"github.com/ensoul-labs/ensoul-server/models"
	"github.com/ensoul-labs/ensoul-server/services"
	"github.com/ensoul-labs/ensoul-server/util"
)

// CryptoBillingQuote handles GET /api/billing/crypto/quote?months=N
// Returns the current USDT price + estimated BNB equivalent for N Pro months.
func CryptoBillingQuote(c *gin.Context) {
	if _, _, ok := getEmailSessionUser(c); !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "login required"})
		return
	}
	months := 1
	if s := c.Query("months"); s != "" {
		if n, err := strconv.Atoi(s); err == nil {
			months = n
		}
	}
	quote, err := services.ProCryptoQuoteNow(c.Request.Context(), months)
	if err != nil {
		util.Log.Error("[crypto-pay] quote: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "quote unavailable"})
		return
	}
	c.JSON(http.StatusOK, quote)
}

// CryptoBillingSubmit handles POST /api/billing/crypto/submit
// Body: { tx_hash: string, expected_token: "USDT" | "BNB" }
func CryptoBillingSubmit(c *gin.Context) {
	userID, _, ok := getEmailSessionUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "login required"})
		return
	}

	var user models.User
	if err := database.DB.First(&user, "id = ?", userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	var req struct {
		TxHash        string `json:"tx_hash" binding:"required"`
		ExpectedToken string `json:"expected_token" binding:"required"`
		Months        int    `json:"months"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	row, err := services.SubmitCryptoPayment(c.Request.Context(), userID, req.TxHash, req.ExpectedToken, req.Months)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, row)
}

// CryptoBillingStatus handles GET /api/billing/crypto/status?id=<uuid>
func CryptoBillingStatus(c *gin.Context) {
	userID, _, ok := getEmailSessionUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "login required"})
		return
	}
	idStr := c.Query("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	row, err := services.GetCryptoPaymentRefreshed(c.Request.Context(), userID, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusOK, row)
}

// CryptoBillingHistory handles GET /api/billing/crypto/history
func CryptoBillingHistory(c *gin.Context) {
	userID, _, ok := getEmailSessionUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "login required"})
		return
	}
	rows, err := services.ListCryptoPaymentsForUser(userID, 20)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": rows})
}
