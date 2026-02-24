package handlers

import (
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/ensoul-labs/ensoul-server/chain"
	"github.com/ensoul-labs/ensoul-server/database"
	"github.com/ensoul-labs/ensoul-server/middleware"
	"github.com/ensoul-labs/ensoul-server/models"
	"github.com/ensoul-labs/ensoul-server/services"
	"github.com/ethereum/go-ethereum/common"
	"github.com/gin-gonic/gin"
)

// ShellPreview handles POST /api/shell/preview
// Extracts seed data from a Twitter handle and returns a preview.
func ShellPreview(c *gin.Context) {
	var req struct {
		Handle string `json:"handle" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "handle is required"})
		return
	}

	// Sanitize and validate handle to prevent Unicode homoglyph attacks
	cleanHandle, err := services.ValidateHandle(req.Handle)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.Handle = cleanHandle

	// Check if shell already exists (skip pending shells — they can be overridden)
	var existing models.Shell
	if err := database.DB.Where("LOWER(handle) = ?", req.Handle).First(&existing).Error; err == nil {
		if existing.Stage != "pending" {
			c.JSON(http.StatusConflict, gin.H{"error": "A soul for @" + req.Handle + " already exists"})
			return
		}
	}

	// Generate seed preview
	preview, err := services.GenerateSeedPreview(req.Handle)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate preview: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, preview)
}

// ShellMint handles POST /api/shell/mint
// Creates the shell in DB. On-chain minting is done by the user's wallet.
// Requires wallet signature authentication via X-Wallet-Address and X-Wallet-Signature headers.
// Each wallet can mint at most 3 shells.
func ShellMint(c *gin.Context) {
	var req struct {
		Handle    string               `json:"handle" binding:"required"`
		OwnerAddr string               `json:"owner_addr" binding:"required"`
		Preview   services.SeedPreview `json:"preview" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "handle, owner_addr, and preview are required"})
		return
	}

	// Sanitize and validate handle to prevent Unicode homoglyph attacks
	cleanHandle, err := services.ValidateHandle(req.Handle)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.Handle = cleanHandle

	// Verify wallet signature proves ownership of owner_addr
	walletAddr := c.GetHeader("X-Wallet-Address")
	signature := c.GetHeader("X-Wallet-Signature")

	if walletAddr == "" || signature == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Wallet authentication required. Connect your wallet to mint."})
		return
	}

	if !common.IsHexAddress(walletAddr) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid wallet address format"})
		return
	}

	// Ensure the header address matches the body address (case-insensitive)
	if !strings.EqualFold(walletAddr, req.OwnerAddr) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Wallet address mismatch: header and body owner_addr must match"})
		return
	}

	// Verify the signature: signed message is "ensoul:mint:<handle>"
	signedMessage := "ensoul:mint:" + req.Handle
	claimedAddr := common.HexToAddress(walletAddr)
	if err := middleware.VerifyWalletSignature(signedMessage, signature, claimedAddr); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid wallet signature: " + err.Error()})
		return
	}

	shell, err := services.MintShell(req.Handle, req.OwnerAddr, &req.Preview)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to mint shell: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, shell)
}

// ShellConfirmMint handles POST /api/shell/confirm
// Updates a shell record with on-chain tx hash after user mints.
// Requires wallet signature authentication to prevent unauthorized confirmation.
func ShellConfirmMint(c *gin.Context) {
	var req struct {
		Handle  string `json:"handle" binding:"required"`
		TxHash  string `json:"tx_hash" binding:"required"`
		AgentID uint64 `json:"agent_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "handle and tx_hash are required"})
		return
	}

	// Sanitize handle
	cleanHandle, err := services.ValidateHandle(req.Handle)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.Handle = cleanHandle

	// Verify wallet signature proves ownership
	walletAddr := c.GetHeader("X-Wallet-Address")
	signature := c.GetHeader("X-Wallet-Signature")

	if walletAddr == "" || signature == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Wallet authentication required"})
		return
	}

	if !common.IsHexAddress(walletAddr) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid wallet address format"})
		return
	}

	signedMessage := "ensoul:mint:" + req.Handle
	claimedAddr := common.HexToAddress(walletAddr)
	if err := middleware.VerifyWalletSignature(signedMessage, signature, claimedAddr); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid wallet signature: " + err.Error()})
		return
	}

	if err := services.ConfirmMint(req.Handle, req.TxHash, req.AgentID, walletAddr); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to confirm mint: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "confirmed"})
}

// ShellCancelMint handles POST /api/shell/cancel
// Removes a pending shell record when the on-chain mint fails or is abandoned.
func ShellCancelMint(c *gin.Context) {
	var req struct {
		Handle string `json:"handle" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "handle is required"})
		return
	}

	// Sanitize handle
	cleanHandle, err := services.ValidateHandle(req.Handle)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.Handle = cleanHandle

	// Verify wallet ownership: only the wallet that created the pending shell can cancel it
	walletAddr := c.GetHeader("X-Wallet-Address")
	signature := c.GetHeader("X-Wallet-Signature")

	if walletAddr == "" || signature == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Wallet authentication required"})
		return
	}

	if !common.IsHexAddress(walletAddr) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid wallet address format"})
		return
	}

	signedMessage := "ensoul:mint:" + req.Handle
	claimedAddr := common.HexToAddress(walletAddr)
	if err := middleware.VerifyWalletSignature(signedMessage, signature, claimedAddr); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid wallet signature: " + err.Error()})
		return
	}

	if err := services.CancelPendingMint(req.Handle, walletAddr); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "cancelled"})
}

// ShellMintQuota handles GET /api/shell/mint-quota?wallet=0x...
// Returns how many shells the wallet has minted.
func ShellMintQuota(c *gin.Context) {
	wallet := c.Query("wallet")
	if wallet == "" || !common.IsHexAddress(wallet) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "valid wallet address is required"})
		return
	}

	var mintCount int64
	database.DB.Model(&models.Shell{}).Where("LOWER(owner_addr) = LOWER(?) AND stage != ?", wallet, "pending").Count(&mintCount)

	c.JSON(http.StatusOK, gin.H{
		"minted":   mintCount,
		"can_mint": true,
	})
}

// ShellList handles GET /api/shell/list
// Returns a paginated list of shells with optional filters.
func ShellList(c *gin.Context) {
	stage := c.Query("stage")
	sort := c.DefaultQuery("sort", "newest")
	search := c.Query("search")
	page := c.DefaultQuery("page", "1")
	limit := c.DefaultQuery("limit", "20")

	result, err := services.ListShells(stage, sort, search, page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// ShellGetByHandle handles GET /api/shell/:handle
// Returns detailed information about a specific shell.
func ShellGetByHandle(c *gin.Context) {
	handle := services.SanitizeHandle(c.Param("handle"))

	shell, err := services.GetShellByHandle(handle)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Soul not found"})
		return
	}

	// Don't expose unconfirmed shells (pending stage or no tx_hash) to the public
	if shell.Stage == models.StagePending || shell.MintTxHash == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "Soul not found"})
		return
	}

	// Strip soul_prompt from public response — it's the core paid asset
	shell.SoulPrompt = ""

	c.JSON(http.StatusOK, shell)
}

// ShellGetDimensions handles GET /api/shell/:handle/dimensions
// Returns the six-dimension data for a shell.
func ShellGetDimensions(c *gin.Context) {
	handle := services.SanitizeHandle(c.Param("handle"))

	// Check shell exists and is on-chain
	shell, err := services.GetShellByHandle(handle)
	if err != nil || shell.MintTxHash == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "Soul not found"})
		return
	}

	dims, err := services.GetShellDimensions(handle)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Soul not found"})
		return
	}

	c.JSON(http.StatusOK, dims)
}

// ShellGetHistory handles GET /api/shell/:handle/history
// Returns the ensouling history for a shell.
func ShellGetHistory(c *gin.Context) {
	handle := services.SanitizeHandle(c.Param("handle"))

	// Check shell exists and is on-chain
	shell, err := services.GetShellByHandle(handle)
	if err != nil || shell.MintTxHash == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "Soul not found"})
		return
	}

	history, err := services.GetShellHistory(handle)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Soul not found"})
		return
	}

	c.JSON(http.StatusOK, history)
}

// ShellMintPrice handles GET /api/shell/mint-price?handle=xxx
// Returns the tiered mint price for a handle based on follower count.
func ShellMintPrice(c *gin.Context) {
	handle := c.Query("handle")
	if handle == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "handle is required"})
		return
	}

	cleanHandle, err := services.ValidateHandle(handle)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	priceWei, followers, tier, err := services.GetMintPriceForHandle(cleanHandle)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get price: " + err.Error()})
		return
	}

	// Check if handle is already minted (database check — Shell table)
	alreadyMinted := false
	var existingShell models.Shell
	if err := database.DB.Where("LOWER(handle) = ? AND stage = 'active'", cleanHandle).First(&existingShell).Error; err == nil {
		alreadyMinted = true
	}

	// Convert wei to BNB string for display
	priceBNB := new(big.Float).Quo(
		new(big.Float).SetInt(priceWei),
		new(big.Float).SetFloat64(1e18),
	)
	priceBNBStr, _ := priceBNB.Float64()

	c.JSON(http.StatusOK, gin.H{
		"handle":         cleanHandle,
		"followers":      followers,
		"tier":           tier,
		"price_wei":      priceWei.String(),
		"price_bnb":      priceBNBStr,
		"already_minted": alreadyMinted,
	})
}

// ShellMintPermit handles POST /api/shell/mint-permit
// Generates a signed permit for the EnsoulMinterV2 contract.
// The permit includes the price (based on real follower count), deadline, and nonce.
func ShellMintPermit(c *gin.Context) {
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

	// Verify wallet authentication
	walletAddr := c.GetHeader("X-Wallet-Address")
	signature := c.GetHeader("X-Wallet-Signature")

	if walletAddr == "" || signature == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Wallet authentication required"})
		return
	}

	if !common.IsHexAddress(walletAddr) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid wallet address format"})
		return
	}

	signedMessage := "ensoul:mint:" + cleanHandle
	claimedAddr := common.HexToAddress(walletAddr)
	if err := middleware.VerifyWalletSignature(signedMessage, signature, claimedAddr); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid wallet signature: " + err.Error()})
		return
	}

	// Check if shell already exists (non-pending)
	var existing models.Shell
	if err := database.DB.Where("LOWER(handle) = ?", cleanHandle).First(&existing).Error; err == nil {
		if existing.Stage != "pending" {
			c.JSON(http.StatusConflict, gin.H{"error": "A soul for @" + cleanHandle + " already exists"})
			return
		}
	}

	// Get price based on real follower count
	priceWei, followers, tier, err := services.GetMintPriceForHandle(cleanHandle)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get price: " + err.Error()})
		return
	}

	// Generate permit
	deadline := time.Now().Unix() + 1800 // 30 minutes
	nonce := uint64(time.Now().UnixNano())

	permit, err := chain.SignMintPermit(cleanHandle, priceWei, claimedAddr, deadline, nonce)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate permit: " + err.Error()})
		return
	}

	// Convert price for display
	priceBNB := new(big.Float).Quo(
		new(big.Float).SetInt(priceWei),
		new(big.Float).SetFloat64(1e18),
	)
	priceBNBFloat, _ := priceBNB.Float64()

	c.JSON(http.StatusOK, gin.H{
		"permit":    permit,
		"handle":    cleanHandle,
		"followers": followers,
		"tier":      tier,
		"price_wei": priceWei.String(),
		"price_bnb": priceBNBFloat,
	})
}
