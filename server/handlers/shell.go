package handlers

import (
	"net/http"
	"strings"

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

	// Check if shell already exists
	var existing models.Shell
	if err := database.DB.Where("handle = ?", req.Handle).First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "A soul for @" + req.Handle + " already exists"})
		return
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

	// Enforce per-wallet mint limit (max 3 shells per address)
	var mintCount int64
	database.DB.Model(&models.Shell{}).Where("LOWER(owner_addr) = LOWER(?)", walletAddr).Count(&mintCount)
	if mintCount >= 3 {
		c.JSON(http.StatusForbidden, gin.H{"error": "Each wallet can mint at most 3 shells"})
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

	if err := services.ConfirmMint(req.Handle, req.TxHash, req.AgentID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to confirm mint: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "confirmed"})
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
	handle := c.Param("handle")

	shell, err := services.GetShellByHandle(handle)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Soul not found"})
		return
	}

	c.JSON(http.StatusOK, shell)
}

// ShellGetDimensions handles GET /api/shell/:handle/dimensions
// Returns the six-dimension data for a shell.
func ShellGetDimensions(c *gin.Context) {
	handle := c.Param("handle")

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
	handle := c.Param("handle")

	history, err := services.GetShellHistory(handle)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Soul not found"})
		return
	}

	c.JSON(http.StatusOK, history)
}
