package handlers

import (
	"context"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/ensoul-labs/ensoul-server/chain"
	"github.com/ensoul-labs/ensoul-server/config"
	"github.com/ensoul-labs/ensoul-server/middleware"
	"github.com/ensoul-labs/ensoul-server/models"
	"github.com/ensoul-labs/ensoul-server/services"
	"github.com/ensoul-labs/ensoul-server/util"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ─── BNB price cache ────────────────────────────────────────────────────────

var (
	bnbPriceMu       sync.RWMutex
	bnbPriceCached   float64
	bnbPriceCachedAt time.Time
	bnbPriceCacheTTL = 60 * time.Second
)

func getCachedBNBPrice(ctx context.Context) (float64, error) {
	bnbPriceMu.RLock()
	if time.Since(bnbPriceCachedAt) < bnbPriceCacheTTL && bnbPriceCached > 0 {
		p := bnbPriceCached
		bnbPriceMu.RUnlock()
		return p, nil
	}
	bnbPriceMu.RUnlock()

	// Fetch fresh price from PancakeSwap
	price, err := chain.GetBNBPriceInUSDT(ctx)
	if err != nil {
		return 0, err
	}

	bnbPriceMu.Lock()
	bnbPriceCached = price
	bnbPriceCachedAt = time.Now()
	bnbPriceMu.Unlock()

	return price, nil
}

// SniperSubscribePrice handles GET /api/sniper/subscribe-price
// Returns the Pro subscription price in both USDT and BNB.
func SniperSubscribePrice(c *gin.Context) {
	tier := c.DefaultQuery("tier", models.SubTierPro)
	tierCfg, ok := models.SubscriptionTiers[tier]
	if !ok || tierCfg.MonthlyPriceUSDT <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid or free tier"})
		return
	}

	priceUSDT := tierCfg.MonthlyPriceUSDT

	resp := gin.H{
		"tier":       tier,
		"price_usdt": priceUSDT,
		"treasury":   config.Cfg.TreasuryAddr,
	}

	// Try to fetch BNB price for conversion
	if chain.C != nil {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
		defer cancel()

		bnbPrice, err := getCachedBNBPrice(ctx)
		if err == nil && bnbPrice > 0 {
			// price_bnb = priceUSDT / bnbPrice, add 1% buffer for price movement
			priceBNB := priceUSDT / bnbPrice * 1.01
			resp["bnb_price"] = bnbPrice
			resp["price_bnb"] = fmt.Sprintf("%.6f", priceBNB)
		} else {
			util.Log.Warn("[sniper] Failed to get BNB price: %v", err)
			resp["bnb_price"] = 0
			resp["price_bnb"] = "0"
		}
	}

	c.JSON(http.StatusOK, resp)
}

// SniperSubscribe handles POST /api/sniper/subscribe
// Creates or renews a Soul Sniper subscription.
// Supports payment in USDT (ERC-20 transfer) or BNB (native transfer) to TREASURY_ADDR.
func SniperSubscribe(c *gin.Context) {
	walletAddr := middleware.GetSessionWallet(c)
	if walletAddr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "login required"})
		return
	}

	var req struct {
		Tier          string  `json:"tier" binding:"required"`
		PaymentTxHash string  `json:"payment_tx_hash" binding:"required"`
		PaymentToken  string  `json:"payment_token"` // "USDT" or "BNB"
		PaymentAmount float64 `json:"payment_amount"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tier and payment_tx_hash are required"})
		return
	}

	req.PaymentToken = strings.ToUpper(req.PaymentToken)
	if req.PaymentToken == "" {
		req.PaymentToken = "USDT"
	}
	if req.PaymentToken != "USDT" && req.PaymentToken != "BNB" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "payment_token must be USDT or BNB"})
		return
	}

	// Get tier config for minimum price
	tierCfg, ok := models.SubscriptionTiers[req.Tier]
	if !ok || tierCfg.MonthlyPriceUSDT <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tier"})
		return
	}

	// Check tx_hash hasn't been used before (prevent replay attacks)
	if err := services.CheckAndMarkPaymentTx(req.PaymentTxHash, walletAddr, "subscription"); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	// Verify payment on-chain
	verifiedAmount := req.PaymentAmount
	treasuryAddr := config.Cfg.TreasuryAddr
	if treasuryAddr == "" {
		services.UnmarkPaymentTx(req.PaymentTxHash)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "treasury address not configured"})
		return
	}

	if chain.C != nil {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
		defer cancel()

		switch req.PaymentToken {
		case "USDT":
			// Verify ERC-20 USDT transfer to treasury
			amount, err := chain.VerifyERC20PaymentTx(ctx, req.PaymentTxHash, walletAddr, treasuryAddr, config.Cfg.USDTAddr)
			if err != nil {
				services.UnmarkPaymentTx(req.PaymentTxHash)
				c.JSON(http.StatusBadRequest, gin.H{"error": "USDT payment verification failed: " + err.Error()})
				return
			}
			verifiedAmount = services.WeiToFloat(amount)
			// Verify amount >= required (allow 0.5% tolerance for rounding)
			if verifiedAmount < tierCfg.MonthlyPriceUSDT*0.995 {
				services.UnmarkPaymentTx(req.PaymentTxHash)
				c.JSON(http.StatusBadRequest, gin.H{
					"error": fmt.Sprintf("insufficient USDT payment: got %.2f, need %.2f", verifiedAmount, tierCfg.MonthlyPriceUSDT),
				})
				return
			}

		case "BNB":
			// Verify native BNB transfer to treasury
			value, to, err := chain.VerifyPaymentTx(ctx, req.PaymentTxHash, walletAddr)
			if err != nil {
				services.UnmarkPaymentTx(req.PaymentTxHash)
				c.JSON(http.StatusBadRequest, gin.H{"error": "BNB payment verification failed: " + err.Error()})
				return
			}
			// Check recipient is treasury
			if !strings.EqualFold(to, treasuryAddr) {
				services.UnmarkPaymentTx(req.PaymentTxHash)
				c.JSON(http.StatusBadRequest, gin.H{"error": "BNB payment recipient is not the treasury address"})
				return
			}
			bnbPaid := services.WeiToFloat(value)

			// Convert BNB amount to USDT value for verification
			bnbPrice, err := getCachedBNBPrice(ctx)
			if err != nil || bnbPrice <= 0 {
				services.UnmarkPaymentTx(req.PaymentTxHash)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch BNB price for verification"})
				return
			}
			usdtValue := bnbPaid * bnbPrice
			// Allow 3% tolerance (price can move between user's quote and tx confirmation)
			if usdtValue < tierCfg.MonthlyPriceUSDT*0.97 {
				services.UnmarkPaymentTx(req.PaymentTxHash)
				c.JSON(http.StatusBadRequest, gin.H{
					"error": fmt.Sprintf("insufficient BNB payment: %.6f BNB ≈ $%.2f, need $%.2f", bnbPaid, usdtValue, tierCfg.MonthlyPriceUSDT),
				})
				return
			}
			// Store the USDT equivalent for the economic flywheel
			verifiedAmount = usdtValue
			// Also store the raw BNB wei for reference
			_ = value
		}
	}

	sub, err := services.CreateSubscription(walletAddr, req.Tier, req.PaymentTxHash, req.PaymentToken, verifiedAmount)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, sub)
}

// toWei18 converts a float to *big.Int with 18 decimals — not used directly here
// but kept for reference.
func toWei18(amount float64) *big.Int {
	f := new(big.Float).SetFloat64(amount)
	exp := new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
	f.Mul(f, exp)
	result, _ := f.Int(nil)
	return result
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
