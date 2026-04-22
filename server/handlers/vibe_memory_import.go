package handlers

import (
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/ensoul-labs/ensoul-server/database"
	"github.com/ensoul-labs/ensoul-server/models"
	"github.com/ensoul-labs/ensoul-server/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Anti-abuse caps for the Smart Import endpoint.
// These are NOT a Pro/Free differentiator — all users share the same limits.
// They exist solely to prevent runaway LLM cost from automated abuse.
const (
	importMaxTextChars       = 20000 // single request payload cap
	importWorkspaceMemoryCap = 500   // total memories per workspace
	importDailyEntryCap      = 1000  // memories created via import per user per UTC day
)

// VibeMemoryImport handles POST /api/vibe-write/workspaces/:id/memories/import
//
// Body:
//
//	{ "text": "...free-form text...", "mode": "review" | "auto-accept" }
//
// Behaviour:
//   - Calls the Vibe Write LLM to atomize `text` into 1..50 categorized memory
//     suggestions across the 5 standard categories.
//   - Persists each as `Source="import"` with status `pending` (review mode,
//     default) or `accepted` (auto-accept).
//   - Drops entries that exact-match (after normalisation) any existing memory
//     in the same workspace + category.
//
// Limits (anti-abuse, not Pro gating):
//   - text length ≤ 20,000 chars
//   - workspace total memories ≤ 500
//   - per-user import-source memories per UTC day ≤ 1000
func VibeMemoryImport(c *gin.Context) {
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

	var ws models.VibeWorkspace
	if err := database.DB.Where("id = ? AND user_id = ?", wsID, userID).First(&ws).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "workspace not found"})
		return
	}

	var req struct {
		Text string `json:"text" binding:"required"`
		Mode string `json:"mode,omitempty"` // "review" (default) | "auto-accept"
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "text is required"})
		return
	}

	if utf8.RuneCountInString(req.Text) > importMaxTextChars {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "text is too long, please split into smaller chunks",
			"code":  "TEXT_TOO_LONG",
			"limit": importMaxTextChars,
		})
		return
	}

	// Reject content that doesn't look like real text (binary file uploaded
	// after rename, etc.). Heuristic: a high ratio of UTF-8 replacement
	// chars or NULs means whatever was decoded isn't going to produce
	// useful memories — fail fast instead of burning an LLM call.
	if looksBinary(req.Text) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "input doesn't look like readable text",
			"code":  "INVALID_CONTENT",
		})
		return
	}

	// Workspace cap.
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

	// Per-user daily cap (count import-source memories created in any of this
	// user's workspaces within the current UTC day).
	dayStart := time.Now().UTC().Truncate(24 * time.Hour)
	var dailyCount int64
	database.DB.Model(&models.VibeMemory{}).
		Joins("JOIN vibe_workspaces ON vibe_workspaces.id = vibe_memories.workspace_id").
		Where("vibe_workspaces.user_id = ? AND vibe_memories.source = ? AND vibe_memories.created_at >= ?",
			userID, "import", dayStart).
		Count(&dailyCount)
	if dailyCount >= importDailyEntryCap {
		c.JSON(http.StatusTooManyRequests, gin.H{
			"error": "daily import limit reached, please try again tomorrow",
			"code":  "RATE_LIMITED",
		})
		return
	}

	// Call LLM.
	suggestions, err := services.ExtractMemoriesFromText(req.Text)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{
			"error": "AI extraction failed, please try again",
			"code":  "LLM_FAILED",
		})
		return
	}

	// Build dedup set of existing memories (normalised by category).
	type normKey struct {
		cat  string
		norm string
	}
	existing := map[normKey]bool{}
	{
		var existingMems []models.VibeMemory
		database.DB.Where("workspace_id = ?", wsID).Find(&existingMems)
		for _, m := range existingMems {
			existing[normKey{cat: m.Category, norm: services.NormalizeMemoryContent(m.Content)}] = true
		}
	}

	// Determine status.
	status := models.MemoryStatusPending
	if req.Mode == "auto-accept" {
		status = models.MemoryStatusAccepted
	}

	created := make([]models.VibeMemory, 0, len(suggestions))
	dedupDropped := 0
	for _, s := range suggestions {
		// Stop if workspace cap would be exceeded (defensive — we may have
		// hundreds of existing rows + many new suggestions).
		if int64(len(created))+wsCount >= importWorkspaceMemoryCap {
			break
		}
		key := normKey{cat: s.Category, norm: services.NormalizeMemoryContent(s.Content)}
		if existing[key] {
			dedupDropped++
			continue
		}
		existing[key] = true

		mem := models.VibeMemory{
			WorkspaceID: wsID,
			Category:    s.Category,
			Content:     s.Content,
			Reason:      s.Reason,
			Source:      "import",
			Status:      status,
		}
		if err := database.DB.Create(&mem).Error; err != nil {
			continue
		}
		created = append(created, mem)
	}

	c.JSON(http.StatusOK, gin.H{
		"suggestions": created,
		"stats": gin.H{
			"input_chars":     len(req.Text),
			"generated_count": len(suggestions),
			"created_count":   len(created),
			"dedup_dropped":   dedupDropped,
			"mode":            req.Mode,
		},
	})
}

// looksBinary returns true when the input contains an unusual number of NUL
// bytes or UTF-8 replacement runes — a strong signal that an actual binary
// file was decoded rather than a real text document.
func looksBinary(s string) bool {
	if s == "" {
		return false
	}
	// Any NUL byte means binary.
	if strings.ContainsRune(s, 0) {
		return true
	}
	// >5% of runes being U+FFFD means decode mostly failed.
	const repl = '\uFFFD'
	bad, total := 0, 0
	for _, r := range s {
		total++
		if r == repl {
			bad++
		}
		if total >= 4000 {
			break
		}
	}
	return total > 0 && bad*20 > total
}
