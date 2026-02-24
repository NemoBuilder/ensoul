package handlers

import (
	"net/http"
	"strings"

	"github.com/ensoul-labs/ensoul-server/database"
	"github.com/ensoul-labs/ensoul-server/models"
	"github.com/ensoul-labs/ensoul-server/services"
	"github.com/gin-gonic/gin"
)

// ═══════════════════════════════════════════════════════════════════════
// Admin: Mint Candidate Management
// ═══════════════════════════════════════════════════════════════════════

// AdminListCandidates handles GET /api/admin/candidates
// Query params: ?status=pending (optional filter)
func AdminListCandidates(c *gin.Context) {
	var candidates []models.MintCandidate
	query := database.DB.Order("priority DESC, created_at ASC")

	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}

	if err := query.Limit(200).Find(&candidates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list candidates"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"candidates": candidates,
		"total":      len(candidates),
	})
}

// AdminAddCandidate handles POST /api/admin/candidates
// Body: { "handle": "elonmusk", "priority": 10, "reason": "Top KOL" }
// Fetches Twitter profile to store follower count and mint price at add time.
func AdminAddCandidate(c *gin.Context) {
	var req struct {
		Handle   string `json:"handle" binding:"required"`
		Priority int    `json:"priority"`
		Reason   string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "handle is required"})
		return
	}

	handle := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(req.Handle), "@"))
	if handle == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "handle cannot be empty"})
		return
	}

	// Check if handle already exists as a Shell
	var existingShell models.Shell
	if err := database.DB.Where("LOWER(handle) = ?", handle).First(&existingShell).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "@" + handle + " is already minted as a Soul"})
		return
	}

	// Fetch Twitter profile for follower count and price
	priceWei, followers, tier, err := services.GetMintPriceForHandle(handle)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to fetch Twitter profile for @" + handle + ": " + err.Error()})
		return
	}

	// Check if already in candidates
	var existing models.MintCandidate
	if err := database.DB.Where("LOWER(handle) = ?", handle).First(&existing).Error; err == nil {
		if existing.Status == models.CandidateStatusPending {
			c.JSON(http.StatusConflict, gin.H{"error": "@" + handle + " is already in the candidate list"})
			return
		}
		// Re-activate if previously skipped/failed — update followers/price too
		existing.Status = models.CandidateStatusPending
		existing.Priority = req.Priority
		existing.Reason = req.Reason
		existing.Followers = followers
		existing.PriceWei = priceWei.String()
		existing.Tier = tier
		existing.ErrorMsg = ""
		database.DB.Save(&existing)
		c.JSON(http.StatusOK, existing)
		return
	}

	candidate := &models.MintCandidate{
		Handle:    handle,
		Followers: followers,
		PriceWei:  priceWei.String(),
		Tier:      tier,
		Priority:  req.Priority,
		Reason:    req.Reason,
		Status:    models.CandidateStatusPending,
	}
	if err := database.DB.Create(candidate).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add candidate: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, candidate)
}

// AdminAddCandidatesBatch handles POST /api/admin/candidates/batch
// Body: { "handles": ["elonmusk", "vitalikbuterin", "caboringdao"], "priority": 5, "reason": "Q1 target" }
// Fetches Twitter profile for each handle to store follower count and mint price.
func AdminAddCandidatesBatch(c *gin.Context) {
	var req struct {
		Handles  []string `json:"handles" binding:"required"`
		Priority int      `json:"priority"`
		Reason   string   `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "handles array is required"})
		return
	}

	added := 0
	skipped := 0
	errors := []string{}

	for _, h := range req.Handles {
		handle := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(h), "@"))
		if handle == "" {
			continue
		}

		// Skip if already a Shell
		var shell models.Shell
		if err := database.DB.Where("LOWER(handle) = ?", handle).First(&shell).Error; err == nil {
			skipped++
			continue
		}

		// Skip if already pending
		var existing models.MintCandidate
		if err := database.DB.Where("LOWER(handle) = ? AND status = ?", handle, models.CandidateStatusPending).First(&existing).Error; err == nil {
			skipped++
			continue
		}

		// Fetch Twitter profile for follower count and price
		priceWei, followers, tier, err := services.GetMintPriceForHandle(handle)
		if err != nil {
			errors = append(errors, handle+": fetch profile failed: "+err.Error())
			continue
		}

		candidate := &models.MintCandidate{
			Handle:    handle,
			Followers: followers,
			PriceWei:  priceWei.String(),
			Tier:      tier,
			Priority:  req.Priority,
			Reason:    req.Reason,
			Status:    models.CandidateStatusPending,
		}
		if err := database.DB.Create(candidate).Error; err != nil {
			errors = append(errors, handle+": "+err.Error())
			continue
		}
		added++
	}

	c.JSON(http.StatusOK, gin.H{
		"added":   added,
		"skipped": skipped,
		"errors":  errors,
	})
}

// AdminRemoveCandidate handles DELETE /api/admin/candidates/:handle
func AdminRemoveCandidate(c *gin.Context) {
	handle := strings.ToLower(strings.TrimPrefix(c.Param("handle"), "@"))

	result := database.DB.Where("LOWER(handle) = ?", handle).Delete(&models.MintCandidate{})
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "@" + handle + " not found in candidate list"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "removed", "handle": handle})
}

// AdminRefreshCandidate handles POST /api/admin/candidates/:handle/refresh
// Re-fetches Twitter profile to update follower count and mint price.
func AdminRefreshCandidate(c *gin.Context) {
	handle := strings.ToLower(strings.TrimPrefix(c.Param("handle"), "@"))
	if handle == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "handle is required"})
		return
	}

	var candidate models.MintCandidate
	if err := database.DB.Where("LOWER(handle) = ?", handle).First(&candidate).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "@" + handle + " not found in candidate list"})
		return
	}

	priceWei, followers, tier, err := services.GetMintPriceForHandle(handle)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to fetch Twitter profile: " + err.Error()})
		return
	}

	candidate.Followers = followers
	candidate.PriceWei = priceWei.String()
	candidate.Tier = tier
	database.DB.Save(&candidate)

	c.JSON(http.StatusOK, candidate)
}

// AdminRefreshAllCandidates handles POST /api/admin/candidates/refresh-all
// Re-fetches Twitter profiles for all pending candidates.
func AdminRefreshAllCandidates(c *gin.Context) {
	var candidates []models.MintCandidate
	database.DB.Where("status = ?", models.CandidateStatusPending).Find(&candidates)

	if len(candidates) == 0 {
		c.JSON(http.StatusOK, gin.H{"updated": 0, "errors": []string{}})
		return
	}

	updated := 0
	errors := []string{}

	for _, cand := range candidates {
		priceWei, followers, tier, err := services.GetMintPriceForHandle(cand.Handle)
		if err != nil {
			errors = append(errors, cand.Handle+": "+err.Error())
			continue
		}
		database.DB.Model(&models.MintCandidate{}).Where("id = ?", cand.ID).Updates(map[string]interface{}{
			"followers": followers,
			"price_wei": priceWei.String(),
			"tier":      tier,
		})
		updated++
	}

	c.JSON(http.StatusOK, gin.H{
		"updated": updated,
		"errors":  errors,
	})
}

// ═══════════════════════════════════════════════════════════════════════
// Admin: Tax Wallet Operations
// ═══════════════════════════════════════════════════════════════════════

// AdminTriggerMint handles POST /api/admin/tax-wallet/mint
// Manually triggers the auto-mint process for pending candidates.
func AdminTriggerMint(c *gin.Context) {
	go services.AutoMintPublicSouls()
	c.JSON(http.StatusOK, gin.H{
		"status":  "triggered",
		"message": "Auto-mint process started in background. Check server logs for progress.",
	})
}

// AdminTaxWalletStatus handles GET /api/admin/tax-wallet/status
func AdminTaxWalletStatus(c *gin.Context) {
	balance, err := services.GetTaxWalletBalance()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Count candidates by status
	var pendingCount, mintedCount, failedCount int64
	database.DB.Model(&models.MintCandidate{}).Where("status = ?", models.CandidateStatusPending).Count(&pendingCount)
	database.DB.Model(&models.MintCandidate{}).Where("status = ?", models.CandidateStatusMinted).Count(&mintedCount)
	database.DB.Model(&models.MintCandidate{}).Where("status = ?", models.CandidateStatusFailed).Count(&failedCount)

	c.JSON(http.StatusOK, gin.H{
		"balance_wei": balance.String(),
		"candidates": gin.H{
			"pending": pendingCount,
			"minted":  mintedCount,
			"failed":  failedCount,
		},
	})
}

// AdminMintSingle handles POST /api/admin/tax-wallet/mint/:handle
// Immediately mints a single specific handle using the tax wallet.
// Also resets failed/skipped candidates back to pending before retrying.
func AdminMintSingle(c *gin.Context) {
	handle := strings.ToLower(strings.TrimPrefix(c.Param("handle"), "@"))
	if handle == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "handle is required"})
		return
	}

	// Reset status to pending if previously failed/skipped, so the mint flow starts clean
	database.DB.Model(&models.MintCandidate{}).
		Where("LOWER(handle) = ? AND status IN ?", handle, []string{models.CandidateStatusFailed, models.CandidateStatusSkipped}).
		Updates(map[string]interface{}{"status": models.CandidateStatusPending, "error_msg": ""})

	go services.MintSinglePublicSoul(handle)
	c.JSON(http.StatusOK, gin.H{
		"status":  "triggered",
		"handle":  handle,
		"message": "Mint for @" + handle + " started in background. Check server logs for progress.",
	})
}
