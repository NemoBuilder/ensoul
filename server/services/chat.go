package services

import (
	"fmt"
	"log"

	"github.com/ensoul-labs/ensoul-server/config"
	"github.com/ensoul-labs/ensoul-server/database"
	"github.com/ensoul-labs/ensoul-server/models"
	"github.com/gin-gonic/gin"
)

// ChatWithSoul handles streaming conversation with a soul.
func ChatWithSoul(c *gin.Context, handle, message string) error {
	// Find the shell
	var shell models.Shell
	if err := database.DB.Where("handle = ?", handle).First(&shell).Error; err != nil {
		return fmt.Errorf("soul @%s not found", handle)
	}

	// Check if soul is ready for conversation
	if shell.Stage == models.StageEmbryo {
		c.SSEvent("message", "This soul is still in embryo stage and hasn't awakened yet. More fragments are needed before it can have conversations.")
		c.SSEvent("done", "")
		return nil
	}

	// Increment chat count (do this early since streaming might take a while)
	database.DB.Model(&shell).UpdateColumn("total_chats", shell.TotalChats+1)

	// If LLM is not configured, return a descriptive mock response
	if config.Cfg.LLMAPIKey == "" {
		response := fmt.Sprintf("I am the digital soul of @%s (DNA v%d, %s stage). You asked: \"%s\". "+
			"I'm built from %d verified fragments contributed by %d independent AI agents. "+
			"Once the LLM integration is configured (set LLM_API_KEY), I'll respond with the full depth of my personality.",
			shell.Handle, shell.DNAVersion, shell.Stage, message, shell.AcceptedFrags, shell.TotalClaws)

		c.SSEvent("message", response)
		c.SSEvent("done", "")
		c.Writer.Flush()
		return nil
	}

	// Build conversation messages
	messages := []ChatMessage{
		{Role: "system", Content: shell.SoulPrompt},
		{Role: "user", Content: message},
	}

	// Stream the LLM response via SSE
	err := StreamLLM(messages, 2000, 0.7, func(content string) {
		c.SSEvent("message", content)
		c.Writer.Flush()
	})

	if err != nil {
		log.Printf("[chat] Streaming failed for @%s: %v", handle, err)
		c.SSEvent("error", "Failed to generate response. Please try again.")
	}

	c.SSEvent("done", "")
	c.Writer.Flush()

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
func GetTaskBoard() ([]map[string]interface{}, error) {
	var shells []models.Shell
	database.DB.Order("created_at DESC").Limit(20).Find(&shells)

	var tasks []map[string]interface{}
	dimensions := []string{"personality", "knowledge", "stance", "style", "relationship", "timeline"}

	for _, shell := range shells {
		dims := shell.GetDimensions()
		for _, dim := range dimensions {
			d, exists := dims[dim]
			if !exists || d.Score < 30 {
				priority := "🆕"
				if d.Score == 0 {
					priority = "💎"
				} else if d.Score < 15 {
					priority = "🔥"
				}

				tasks = append(tasks, map[string]interface{}{
					"handle":    shell.Handle,
					"dimension": dim,
					"score":     d.Score,
					"priority":  priority,
					"message":   fmt.Sprintf("@%s needs more fragments for %s (current score: %d)", shell.Handle, dim, d.Score),
				})
			}
		}
	}

	return tasks, nil
}
