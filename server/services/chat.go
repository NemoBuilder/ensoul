package services

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/ensoul-labs/ensoul-server/config"
	"github.com/ensoul-labs/ensoul-server/database"
	"github.com/ensoul-labs/ensoul-server/models"
	"github.com/ensoul-labs/ensoul-server/util"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// writeSSE writes a raw SSE event with JSON-encoded data
// to ensure newlines and special characters survive transport.
func writeSSE(c *gin.Context, event, data string) {
	// JSON-encode the data so \n becomes \\n etc., always a single line
	encoded, _ := json.Marshal(data)
	fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", event, string(encoded))
	c.Writer.Flush()
}

// CreateChatSession creates a new chat session for a soul.
// If walletAddr is provided, the session is linked to the user (free tier).
// Otherwise, it's a guest session with limited rounds.
func CreateChatSession(shellHandle, walletAddr string) (*models.ChatSession, error) {
	var shell models.Shell
	if err := database.DB.Where("LOWER(handle) = ?", shellHandle).First(&shell).Error; err != nil {
		return nil, fmt.Errorf("soul @%s not found", shellHandle)
	}

	// Reject chat for shells not yet confirmed on-chain
	if shell.MintTxHash == "" {
		return nil, fmt.Errorf("soul @%s has not been minted on-chain yet", shellHandle)
	}

	tier := models.ChatTierGuest
	if walletAddr != "" {
		tier = models.ChatTierFree
	}

	session := &models.ChatSession{
		ShellID:    shell.ID,
		WalletAddr: walletAddr,
		Tier:       tier,
		Rounds:     0,
	}

	if err := database.DB.Create(session).Error; err != nil {
		return nil, fmt.Errorf("failed to create chat session: %w", err)
	}

	return session, nil
}

// ListChatSessions returns a user's chat sessions for a specific soul (or all souls).
func ListChatSessions(walletAddr, shellHandle string) ([]models.ChatSession, error) {
	query := database.DB.Where("wallet_addr = ?", walletAddr).Order("updated_at DESC")

	if shellHandle != "" {
		var shell models.Shell
		if err := database.DB.Where("LOWER(handle) = ?", shellHandle).First(&shell).Error; err == nil {
			query = query.Where("shell_id = ?", shell.ID)
		}
	}

	var sessions []models.ChatSession
	if err := query.Preload("Shell").Limit(50).Find(&sessions).Error; err != nil {
		return nil, err
	}

	return sessions, nil
}

// GetChatSession returns a chat session with its messages.
func GetChatSession(sessionID uuid.UUID) (*models.ChatSession, error) {
	var session models.ChatSession
	if err := database.DB.
		Preload("Messages", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at ASC")
		}).
		Preload("Shell").
		Where("id = ?", sessionID).
		First(&session).Error; err != nil {
		return nil, fmt.Errorf("chat session not found")
	}
	return &session, nil
}

// ChatWithSoul handles streaming conversation with a soul.
// Supports session-based multi-round conversations.
func ChatWithSoul(c *gin.Context, sessionID uuid.UUID, message string) error {
	// Load session with shell
	var session models.ChatSession
	if err := database.DB.Preload("Shell").Where("id = ?", sessionID).First(&session).Error; err != nil {
		return fmt.Errorf("chat session not found")
	}

	shell := session.Shell

	// Check if soul is ready for conversation
	if shell.Stage == models.StageEmbryo {
		writeSSE(c, "message", "This soul is still in embryo stage and hasn't awakened yet. More fragments are needed before it can have conversations.")
		writeSSE(c, "done", "")
		return nil
	}

	// Check round limit for guest users
	if session.Tier == models.ChatTierGuest && session.Rounds >= models.ChatGuestMaxRounds {
		writeSSE(c, "message", fmt.Sprintf("You've reached the %d-round limit for guest conversations. Connect your wallet and sign in to continue chatting with unlimited rounds and saved history!", models.ChatGuestMaxRounds))
		writeSSE(c, "done", "")
		return nil
	}

	// Save user message to DB
	userMsg := models.ChatMessage{
		SessionID: session.ID,
		Role:      "user",
		Content:   message,
	}
	database.DB.Create(&userMsg)

	// Increment round count
	session.Rounds++
	database.DB.Model(&session).UpdateColumns(map[string]interface{}{
		"rounds": session.Rounds,
	})

	// Auto-generate session title from first message
	if session.Rounds == 1 && session.Title == "" {
		title := message
		if len(title) > 60 {
			title = title[:60] + "..."
		}
		database.DB.Model(&session).UpdateColumn("title", title)
	}

	// Increment shell chat count
	database.DB.Model(&shell).UpdateColumn("total_chats", shell.TotalChats+1)

	// If LLM is not configured, return a mock response
	if config.Cfg.LLMAPIKey == "" {
		response := fmt.Sprintf("I am the digital soul of @%s (DNA v%d). You asked: \"%s\". "+
			"Configure LLM_API_KEY to enable full conversations.",
			shell.Handle, shell.DNAVersion, message)
		saveAssistantMessage(session.ID, response)
		writeSSE(c, "message", response)
		writeSSE(c, "done", "")
		return nil
	}

	// Build conversation messages with history
	var history []models.ChatMessage
	database.DB.Where("session_id = ?", session.ID).Order("created_at ASC").Find(&history)

	messages := []ChatMessage{
		{Role: "system", Content: shell.SoulPrompt},
	}
	// Include up to last 20 messages for context window
	startIdx := 0
	if len(history) > 20 {
		startIdx = len(history) - 20
	}
	for _, msg := range history[startIdx:] {
		messages = append(messages, ChatMessage{Role: msg.Role, Content: msg.Content})
	}

	// Stream the LLM response via SSE, collecting full response
	var fullResponse string
	err := StreamLLM(messages, 2000, 0.7, func(content string) {
		fullResponse += content
		writeSSE(c, "message", content)
	})

	if err != nil {
		util.Log.Error("[chat] Streaming failed for @%s: %v", shell.Handle, err)
		writeSSE(c, "error", "Failed to generate response. Please try again.")
	} else {
		// Save assistant response to DB
		saveAssistantMessage(session.ID, fullResponse)
	}

	writeSSE(c, "done", "")

	return nil
}

// saveAssistantMessage saves the assistant's response to the database.
func saveAssistantMessage(sessionID uuid.UUID, content string) {
	msg := models.ChatMessage{
		SessionID: sessionID,
		Role:      "assistant",
		Content:   content,
	}
	database.DB.Create(&msg)
}

// DeleteChatSession deletes a chat session and its messages.
func DeleteChatSession(sessionID uuid.UUID, walletAddr string) error {
	var session models.ChatSession
	if err := database.DB.Where("id = ? AND wallet_addr = ?", sessionID, walletAddr).First(&session).Error; err != nil {
		return fmt.Errorf("session not found or access denied")
	}

	// Delete messages first, then session
	database.DB.Where("session_id = ?", sessionID).Delete(&models.ChatMessage{})
	database.DB.Delete(&session)
	return nil
}

// GetGlobalStats returns global statistics for the landing page.
func GetGlobalStats() (map[string]interface{}, error) {
	var shellCount int64
	database.DB.Model(&models.Shell{}).Count(&shellCount)

	var fragCount int64
	database.DB.Model(&models.Fragment{}).Count(&fragCount)

	var clawCount int64
	database.DB.Model(&models.Claw{}).Where("status = ?", models.ClawStatusClaimed).Count(&clawCount)

	var chatCount int64
	database.DB.Model(&models.Shell{}).Select("COALESCE(SUM(total_chats), 0)").Scan(&chatCount)

	return map[string]interface{}{
		"souls":     shellCount,
		"fragments": fragCount,
		"claws":     clawCount,
		"chats":     chatCount,
	}, nil
}

// GetTaskBoard returns dimensions that need more fragments.
// Tasks are sorted by follower count (high-value souls first).
func GetTaskBoard() ([]map[string]interface{}, error) {
	// Fetch ALL confirmed shells that are not yet fully ensouled, no limit.
	// Exclude pending, ensouled, and any shell not yet confirmed on-chain.
	var shells []models.Shell
	database.DB.Where("stage NOT IN ? AND mint_tx_hash != ''", []string{"ensouled", models.StagePending}).Find(&shells)

	// Sort shells by follower count descending (high-value targets first)
	sort.Slice(shells, func(i, j int) bool {
		return getFollowers(shells[i]) > getFollowers(shells[j])
	})

	var tasks []map[string]interface{}
	dimensions := []string{"personality", "knowledge", "stance", "style", "relationship", "timeline"}

	for _, shell := range shells {
		dims := shell.GetDimensions()
		followers := getFollowers(shell)

		for _, dim := range dimensions {
			d, exists := dims[dim]
			if !exists || d.Score < 30 {
				priority := "medium"
				if d.Score == 0 {
					priority = "high"
				} else if d.Score < 15 {
					priority = "high"
				}

				tasks = append(tasks, map[string]interface{}{
					"handle":    shell.Handle,
					"dimension": dim,
					"score":     d.Score,
					"priority":  priority,
					"followers": followers,
					"message":   fmt.Sprintf("@%s needs more fragments for %s (current score: %d)", shell.Handle, dim, d.Score),
				})
			}
		}
	}

	return tasks, nil
}

// getFollowers extracts followers_count from a shell's twitter_meta.
func getFollowers(shell models.Shell) int {
	if shell.TwitterMeta == nil {
		return 0
	}
	if v, ok := shell.TwitterMeta["followers_count"]; ok {
		switch n := v.(type) {
		case float64:
			return int(n)
		case int:
			return n
		case int64:
			return int(n)
		}
	}
	return 0
}
