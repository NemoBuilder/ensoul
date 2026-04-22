package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/ensoul-labs/ensoul-server/database"
	"github.com/ensoul-labs/ensoul-server/models"
	"github.com/ensoul-labs/ensoul-server/services"
	"github.com/ensoul-labs/ensoul-server/util"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Twitter-import-specific anti-abuse cap.
//
// Stricter than the free-text Smart Import limit (1000/day) because each
// import-twitter call hits the upstream Twitter / SocialData API, which has
// real per-key quotas and a non-trivial $ cost beyond LLM tokens.
const importTwitterDailyCap = 20

// MemorySourceTwitterImport identifies memories created by the
// "Import from Twitter" flow on the memory management page.
//
// Distinct from "import" (free-text Smart Import) and "ai"
// (workspace setup / self-portrait refresh) so that:
//   - the daily rate limiter can target this exact source,
//   - the UI can render a "🐦 from @handle" badge unambiguously.
const MemorySourceTwitterImport = "twitter_import"

// VibeMemoryImportTwitter handles
//
//	POST /api/vibe-write/workspaces/:id/memories/import-twitter
//
// Body: { "twitter_handle": "elonmusk", "auto_accept": false }
//
// Differs from VibeWorkspaceSetup in that it:
//   - does NOT modify workspace.twitter_handle (so the workspace's bound
//     identity is not overwritten when importing a third party's content);
//   - does NOT mirror to User.TwitterHandle;
//   - tags persisted memories with Source="twitter_import" and a Reason
//     prefix of "Imported from @<handle>. ..." for source attribution.
//
// Pricing & limits:
//   - costs services.CreditCostSetup (5 credits), refunded on failure;
//   - per-user daily cap of importTwitterDailyCap successful imports;
//   - workspace memory cap of importWorkspaceMemoryCap (shared with text import).
func VibeMemoryImportTwitter(c *gin.Context) {
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "twitter_handle is required", "code": "INVALID_HANDLE"})
		return
	}

	handle := strings.TrimPrefix(strings.TrimSpace(req.TwitterHandle), "@")
	if handle == "" || len(handle) > 30 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid twitter handle", "code": "INVALID_HANDLE"})
		return
	}

	// Verify workspace ownership.
	var ws models.VibeWorkspace
	if err := database.DB.Where("id = ? AND user_id = ?", wsID, userID).First(&ws).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "workspace not found", "code": "NOT_FOUND"})
		return
	}

	// Workspace memory cap.
	var wsCount int64
	database.DB.Model(&models.VibeMemory{}).Where("workspace_id = ?", wsID).Count(&wsCount)
	if wsCount >= importWorkspaceMemoryCap {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "this workspace's memory is full, please clean up first",
			"code":  "WORKSPACE_MEMORY_FULL",
			"limit": importWorkspaceMemoryCap,
		})
		return
	}

	// Per-user daily Twitter-import cap.
	dayStart := time.Now().UTC().Truncate(24 * time.Hour)
	var dailyCount int64
	database.DB.Model(&models.VibeMemory{}).
		Joins("JOIN vibe_workspaces ON vibe_workspaces.id = vibe_memories.workspace_id").
		Where("vibe_workspaces.user_id = ? AND vibe_memories.source = ? AND vibe_memories.created_at >= ?",
			userID, MemorySourceTwitterImport, dayStart).
		Count(&dailyCount)
	if dailyCount >= importTwitterDailyCap {
		c.JSON(http.StatusTooManyRequests, gin.H{
			"error": "daily Twitter import limit reached, please try again tomorrow",
			"code":  "RATE_LIMITED",
			"limit": importTwitterDailyCap,
		})
		return
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

	// Fetch profile.
	profile, err := services.FetchTwitterProfile(handle)
	if err != nil {
		_ = refundVibeWriteCredits(userID, creditCost)
		util.Log.Error("[vibe-import-twitter] fetch profile failed handle=%s err=%v", handle, err)
		c.JSON(http.StatusBadGateway, gin.H{
			"error": "failed to fetch Twitter profile: " + err.Error(),
			"code":  "TWITTER_FETCH_FAILED",
		})
		return
	}

	// Distill via LLM.
	suggestions, err := services.DistillTwitterProfile(profile)
	if err != nil {
		_ = refundVibeWriteCredits(userID, creditCost)
		util.Log.Error("[vibe-import-twitter] distill failed handle=%s err=%v", handle, err)
		c.JSON(http.StatusBadGateway, gin.H{
			"error": "LLM distillation failed: " + err.Error(),
			"code":  "DISTILL_FAILED",
		})
		return
	}
	if len(suggestions) == 0 {
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

	// Look up user for Pro check.
	var user models.User
	_ = database.DB.First(&user, "id = ?", userID).Error

	// Default = pending; auto_accept honored only for Pro users.
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
			Reason:      "Imported from @" + handle + ". " + s.Reason,
			Source:      MemorySourceTwitterImport,
			Status:      status,
		}
		if err := database.DB.Create(&mem).Error; err != nil {
			util.Log.Warn("[vibe-import-twitter] persist memory failed ws=%s cat=%s err=%v", ws.ID, s.Category, err)
			continue
		}
		persisted = append(persisted, mem)
	}

	util.Log.Info("[vibe-import-twitter] ws=%s handle=%s source=%s tweets=%d persisted=%d status=%s credits=%d",
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
