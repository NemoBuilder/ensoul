package services

import (
	"fmt"
	"time"

	"github.com/ensoul-labs/ensoul-server/config"
	"github.com/ensoul-labs/ensoul-server/database"
	"github.com/ensoul-labs/ensoul-server/models"
	"github.com/ensoul-labs/ensoul-server/util"
	"github.com/google/uuid"
)

// ──────────────────────────────────────────────────────────────────────────────
// External Snipe API — Requirement ③
// API-key authenticated, no wallet/subscription required
// ──────────────────────────────────────────────────────────────────────────────

// ExternalSnipe generates reply suggestions for a tweet via external API.
// Bypasses wallet auth and Pro subscription — uses API key + daily limit.
func ExternalSnipe(callerID, authorHandle, tweetID, tweetText, tagID, language string) (*models.VibeWriteReply, error) {
	// 1. Check daily limit for this caller
	canSnipe, used, limit, err := CheckExternalSnipeLimit(callerID)
	if err != nil {
		return nil, fmt.Errorf("rate limit check failed: %w", err)
	}
	if !canSnipe {
		return nil, fmt.Errorf("external snipe daily limit reached (%d/%d)", used, limit)
	}

	// 2. Check for cached reply (same caller + tweet)
	walletAddr := "external:" + callerID
	var existing models.VibeWriteReply
	if err := database.DB.Where("wallet_addr = ? AND tweet_id = ?", walletAddr, tweetID).
		First(&existing).Error; err == nil {
		// Validate cached data
		if variants, ok := existing.Replies["variants"]; ok {
			if arr, ok := variants.([]interface{}); ok && len(arr) > 0 {
				return &existing, nil
			}
		}
		database.DB.Delete(&existing)
	}

	// 3. Try to find the author's Soul (optional enrichment)
	var shell *models.Shell
	usedSoul := false
	shellByHandle, err := GetShellByHandle(authorHandle)
	if err == nil {
		shell = shellByHandle
		usedSoul = true
	}

	// 4. No user persona for external calls — use default
	snipeLang := language
	if snipeLang == "" {
		snipeLang = "en"
	}

	// 5. Use default LLM model
	_, _, llmModel, _ := config.Cfg.VibeWriteLLM()

	// 6. Generate reply variants
	replies, err := generateSnipeVariants(shell, nil, authorHandle, tweetText, llmModel, snipeLang)
	if err != nil {
		return nil, fmt.Errorf("reply generation failed: %w", err)
	}

	// 7. Build replies JSON
	var repliesAny []interface{}
	for _, r := range replies {
		repliesAny = append(repliesAny, map[string]interface{}{
			"style":   r.Style,
			"content": r.Content,
			"model":   r.Model,
		})
	}

	// 8. Build tweet URL
	tweetURL := fmt.Sprintf("https://twitter.com/%s/status/%s", authorHandle, tweetID)

	// 9. Save vibe-write reply record
	var shellID *uuid.UUID
	if shell != nil {
		shellID = &shell.ID
	}

	VibeWriteReply := &models.VibeWriteReply{
		ShellID:      shellID,
		WalletAddr:   walletAddr,
		TweetID:      tweetID,
		TweetText:    tweetText,
		Replies:      models.JSON{"variants": repliesAny},
		AuthorHandle: authorHandle,
		TagID:        tagID,
		TweetURL:     tweetURL,
		UsedSoul:     usedSoul,
	}

	if err := database.DB.Create(VibeWriteReply).Error; err != nil {
		return nil, fmt.Errorf("failed to save reply: %w", err)
	}

	// 10. Record usage (if Soul was used)
	if shell != nil {
		RecordUsage(shell.ID, walletAddr)
	}

	// 11. Increment external usage counter
	IncrementExternalSnipeUsage(callerID)

	util.Log.Info("[vibe-write] ExternalSnipe: generated %d replies for @%s tweet %s (caller: %s, soul: %v)",
		len(replies), authorHandle, tweetID, callerID, usedSoul)

	return VibeWriteReply, nil
}

// CheckExternalSnipeLimit checks if the caller has remaining daily quota.
// Returns (canSnipe, usedToday, limit, error).
func CheckExternalSnipeLimit(callerID string) (bool, int, int, error) {
	limit := config.Cfg.ExternalSnipeDailyLimit
	if limit <= 0 {
		limit = 200
	}

	today := time.Now().UTC().Format("2006-01-02")

	var usage models.ExternalSnipeUsage
	err := database.DB.Where("caller_id = ? AND date = ?", callerID, today).First(&usage).Error
	if err != nil {
		// No record yet — first call today
		return true, 0, limit, nil
	}

	if usage.Count >= limit {
		return false, usage.Count, limit, nil
	}

	return true, usage.Count, limit, nil
}

// IncrementExternalSnipeUsage bumps the daily usage counter for a caller.
func IncrementExternalSnipeUsage(callerID string) {
	today := time.Now().UTC().Format("2006-01-02")

	var usage models.ExternalSnipeUsage
	err := database.DB.Where("caller_id = ? AND date = ?", callerID, today).First(&usage).Error
	if err != nil {
		// Create new record
		usage = models.ExternalSnipeUsage{
			CallerID: callerID,
			Date:     today,
			Count:    1,
		}
		database.DB.Create(&usage)
		return
	}

	usage.Count++
	database.DB.Save(&usage)
}
