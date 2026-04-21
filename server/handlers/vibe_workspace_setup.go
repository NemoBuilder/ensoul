package handlers

import (
	"net/http"
	"strings"

	"github.com/ensoul-labs/ensoul-server/database"
	"github.com/ensoul-labs/ensoul-server/models"
	"github.com/ensoul-labs/ensoul-server/services"
	"github.com/ensoul-labs/ensoul-server/util"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// VibeWorkspaceSetup handles POST /api/vibe-write/workspaces/:id/setup
//
// Body: { "twitter_handle": "elonmusk", "auto_accept": false }
//
// Workflow:
//   1. Verify workspace ownership
//   2. Persist twitter_handle on workspace (and on User if empty)
//   3. Fetch profile + recent tweets via SocialData (or Twitter v2 fallback)
//   4. Run LLM distillation → 4–8 MemorySuggestion entries
//   5. Persist as pending memories (or accepted if auto_accept=true and explicitly Pro)
//   6. Return the persisted rows so the frontend can show review UI
//
// Idempotent: re-running with the same handle creates a new pending batch
// (does NOT delete previous memories) — user can compare and reject duplicates.
func VibeWorkspaceSetup(c *gin.Context) {
	userID, _, ok := getEmailSessionUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "login required"})
		return
	}

	wsID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid workspace ID"})
		return
	}

	var req struct {
		TwitterHandle string `json:"twitter_handle" binding:"required"`
		AutoAccept    bool   `json:"auto_accept"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "twitter_handle is required"})
		return
	}

	handle := strings.TrimPrefix(strings.TrimSpace(req.TwitterHandle), "@")
	if handle == "" || len(handle) > 30 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid twitter handle"})
		return
	}

	// Verify workspace ownership
	var ws models.VibeWorkspace
	if err := database.DB.Where("id = ? AND user_id = ?", wsID, userID).First(&ws).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "workspace not found", "code": "NOT_FOUND"})
		return
	}

	// Update workspace handle
	if ws.TwitterHandle != handle {
		database.DB.Model(&ws).Update("twitter_handle", handle)
		ws.TwitterHandle = handle
	}

	// Mirror to User.TwitterHandle when empty (first-time setup)
	var user models.User
	if err := database.DB.First(&user, "id = ?", userID).Error; err == nil && user.TwitterHandle == "" {
		database.DB.Model(&user).Update("twitter_handle", handle)
	}

	// Charge credits up-front (refunded on any failure below).
	creditCost := services.CreditCostSetup
	if err := services.DeductCredits(userID, creditCost); err != nil {
		c.JSON(http.StatusPaymentRequired, gin.H{
			"error": err.Error(),
			"code":  "INSUFFICIENT_CREDITS",
			"need":  creditCost,
		})
		return
	}

	// Fetch profile (SocialData → Twitter v2 → mock fallback)
	profile, err := services.FetchTwitterProfile(handle)
	if err != nil {
		_ = refundVibeWriteCredits(userID, creditCost)
		util.Log.Error("[vibe-setup] fetch profile failed handle=%s err=%v", handle, err)
		c.JSON(http.StatusBadGateway, gin.H{
			"error": "failed to fetch Twitter profile: " + err.Error(),
			"code":  "PROFILE_FETCH_FAILED",
		})
		return
	}

	// Distill via LLM
	suggestions, err := services.DistillTwitterProfile(profile)
	if err != nil {
		_ = refundVibeWriteCredits(userID, creditCost)
		util.Log.Error("[vibe-setup] distill failed handle=%s err=%v", handle, err)
		c.JSON(http.StatusBadGateway, gin.H{
			"error": "LLM distillation failed: " + err.Error(),
			"code":  "DISTILL_FAILED",
		})
		return
	}
	if len(suggestions) == 0 {
		// Refund — no value delivered.
		_ = refundVibeWriteCredits(userID, creditCost)
		c.JSON(http.StatusOK, gin.H{
			"workspace":        ws,
			"profile_source":   profile.DataSource,
			"tweets_analyzed":  len(profile.Tweets),
			"pending_memories": []models.VibeMemory{},
			"credits_used":     0,
			"message":          "no usable signals derived from this profile",
		})
		return
	}

	// Decide initial status. Default = pending (user reviews); auto_accept honored only if user is Pro.
	status := models.MemoryStatusPending
	if req.AutoAccept && user.IsPro() {
		status = models.MemoryStatusAccepted
	}

	persisted := make([]models.VibeMemory, 0, len(suggestions))
	for _, s := range suggestions {
		mem := models.VibeMemory{
			WorkspaceID: ws.ID,
			Category:    s.Category,
			Content:     s.Content,
			Reason:      "Setup: derived from @" + handle + ". " + s.Reason,
			Source:      "ai",
			Status:      status,
		}
		if err := database.DB.Create(&mem).Error; err != nil {
			util.Log.Warn("[vibe-setup] persist memory failed ws=%s cat=%s err=%v", ws.ID, s.Category, err)
			continue
		}
		persisted = append(persisted, mem)
	}

	util.Log.Info("[vibe-setup] ws=%s handle=%s source=%s tweets=%d persisted=%d status=%s credits=%d",
		ws.ID, handle, profile.DataSource, len(profile.Tweets), len(persisted), status, creditCost)

	c.JSON(http.StatusOK, gin.H{
		"workspace":        ws,
		"profile_source":   profile.DataSource,
		"tweets_analyzed":  len(profile.Tweets),
		"pending_memories": persisted,
		"status":           status,
		"credits_used":     creditCost,
	})
}
