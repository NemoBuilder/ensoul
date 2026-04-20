package handlers

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/ensoul-labs/ensoul-server/config"
	"github.com/ensoul-labs/ensoul-server/database"
	"github.com/ensoul-labs/ensoul-server/models"
	"github.com/ensoul-labs/ensoul-server/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	maxWorkspacesFree = 1
	maxWorkspacesPro  = 10
)

// VibeWorkspaceList handles GET /api/vibe-write/workspaces
func VibeWorkspaceList(c *gin.Context) {
	userID, _, ok := getEmailSessionUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "login required"})
		return
	}

	var workspaces []models.VibeWorkspace
	database.DB.Where("user_id = ?", userID).Order("sort_order ASC, created_at ASC").Find(&workspaces)

	c.JSON(http.StatusOK, gin.H{"workspaces": workspaces})
}

// VibeWorkspaceCreate handles POST /api/vibe-write/workspaces
func VibeWorkspaceCreate(c *gin.Context) {
	userID, _, ok := getEmailSessionUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "login required"})
		return
	}

	var req struct {
		Name          string `json:"name" binding:"required"`
		TwitterHandle string `json:"twitter_handle"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}

	// Check workspace limit
	var count int64
	database.DB.Model(&models.VibeWorkspace{}).Where("user_id = ?", userID).Count(&count)

	var user models.User
	database.DB.First(&user, "id = ?", userID)
	maxWs := maxWorkspacesFree
	if user.IsPro() {
		maxWs = maxWorkspacesPro
	}
	if int(count) >= maxWs {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "workspace limit reached",
			"limit": maxWs,
			"is_pro": user.IsPro(),
		})
		return
	}

	ws := models.VibeWorkspace{
		UserID:        userID,
		Name:          req.Name,
		TwitterHandle: req.TwitterHandle,
	}
	if err := database.DB.Create(&ws).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create workspace"})
		return
	}

	c.JSON(http.StatusCreated, ws)
}

// VibeWorkspaceUpdate handles PUT /api/vibe-write/workspaces/:id
func VibeWorkspaceUpdate(c *gin.Context) {
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
		Name          *string `json:"name"`
		TwitterHandle *string `json:"twitter_handle"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	if req.Name != nil {
		ws.Name = *req.Name
	}
	if req.TwitterHandle != nil {
		ws.TwitterHandle = *req.TwitterHandle
	}

	database.DB.Save(&ws)
	c.JSON(http.StatusOK, ws)
}

// VibeWorkspaceDelete handles DELETE /api/vibe-write/workspaces/:id
func VibeWorkspaceDelete(c *gin.Context) {
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

	result := database.DB.Where("id = ? AND user_id = ?", wsID, userID).Delete(&models.VibeWorkspace{})
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "workspace not found"})
		return
	}

	// Soft-delete related chats and memories
	database.DB.Where("workspace_id = ?", wsID).Delete(&models.VibeMemory{})
	database.DB.Where("workspace_id = ?", wsID).Delete(&models.VibeChat{})

	c.JSON(http.StatusOK, gin.H{"message": "workspace deleted"})
}

// ── Memory endpoints ───────────────────────────────────────────

var freeMemoryCategories = map[string]bool{
	models.MemoryCategoryProfile: true,
	models.MemoryCategoryRules:   true,
}

var allMemoryCategories = map[string]bool{
	models.MemoryCategoryProfile:   true,
	models.MemoryCategoryKnowledge: true,
	models.MemoryCategoryNetwork:   true,
	models.MemoryCategoryArchive:   true,
	models.MemoryCategoryRules:     true,
}

// VibeMemoryList handles GET /api/vibe-write/workspaces/:id/memories
func VibeMemoryList(c *gin.Context) {
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

	// Verify workspace ownership
	var ws models.VibeWorkspace
	if err := database.DB.Where("id = ? AND user_id = ?", wsID, userID).First(&ws).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "workspace not found"})
		return
	}

	var memories []models.VibeMemory
	database.DB.Where("workspace_id = ?", wsID).Order("category ASC, sort_order ASC").Find(&memories)

	c.JSON(http.StatusOK, gin.H{"memories": memories})
}

// VibeMemoryCreate handles POST /api/vibe-write/workspaces/:id/memories
func VibeMemoryCreate(c *gin.Context) {
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
		Category string `json:"category" binding:"required"`
		Content  string `json:"content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "category and content are required"})
		return
	}

	if !allMemoryCategories[req.Category] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid category"})
		return
	}

	// Check category access (Free vs Pro)
	var user models.User
	database.DB.First(&user, "id = ?", userID)
	if !user.IsPro() && !freeMemoryCategories[req.Category] {
		c.JSON(http.StatusForbidden, gin.H{
			"error":    "Pro required for this memory category",
			"category": req.Category,
		})
		return
	}

	mem := models.VibeMemory{
		WorkspaceID: wsID,
		Category:    req.Category,
		Content:     req.Content,
		Source:      "user",
	}
	if err := database.DB.Create(&mem).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create memory"})
		return
	}

	c.JSON(http.StatusCreated, mem)
}

// VibeMemoryUpdate handles PUT /api/vibe-write/memories/:memId
func VibeMemoryUpdate(c *gin.Context) {
	userID, _, ok := getEmailSessionUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "login required"})
		return
	}

	memID, err := uuid.Parse(c.Param("memId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid memory ID"})
		return
	}

	// Verify ownership via workspace
	var mem models.VibeMemory
	if err := database.DB.First(&mem, "id = ?", memID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "memory not found"})
		return
	}
	var ws models.VibeWorkspace
	if err := database.DB.Where("id = ? AND user_id = ?", mem.WorkspaceID, userID).First(&ws).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "not your memory"})
		return
	}

	var req struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	mem.Content = req.Content
	database.DB.Save(&mem)

	c.JSON(http.StatusOK, mem)
}

// VibeMemoryDelete handles DELETE /api/vibe-write/memories/:memId
func VibeMemoryDelete(c *gin.Context) {
	userID, _, ok := getEmailSessionUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "login required"})
		return
	}

	memID, err := uuid.Parse(c.Param("memId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid memory ID"})
		return
	}

	var mem models.VibeMemory
	if err := database.DB.First(&mem, "id = ?", memID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "memory not found"})
		return
	}
	var ws models.VibeWorkspace
	if err := database.DB.Where("id = ? AND user_id = ?", mem.WorkspaceID, userID).First(&ws).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "not your memory"})
		return
	}

	database.DB.Delete(&mem)
	c.JSON(http.StatusOK, gin.H{"message": "memory deleted"})
}

// ── Chat endpoints ────────────────────────────────────────────

// VibeChatList handles GET /api/vibe-write/workspaces/:id/chats
func VibeChatList(c *gin.Context) {
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

	var chats []models.VibeChat
	database.DB.Where("workspace_id = ?", wsID).Order("updated_at DESC").Find(&chats)

	c.JSON(http.StatusOK, gin.H{"chats": chats})
}

// VibeChatCreate handles POST /api/vibe-write/workspaces/:id/chats
func VibeChatCreate(c *gin.Context) {
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

	chat := models.VibeChat{
		WorkspaceID: wsID,
		UserID:      userID,
		Title:       "New Chat",
	}
	if err := database.DB.Create(&chat).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create chat"})
		return
	}

	c.JSON(http.StatusCreated, chat)
}

// VibeChatDelete handles DELETE /api/vibe-write/chats/:chatId
func VibeChatDelete(c *gin.Context) {
	userID, _, ok := getEmailSessionUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "login required"})
		return
	}

	chatID, err := uuid.Parse(c.Param("chatId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid chat ID"})
		return
	}

	result := database.DB.Where("id = ? AND user_id = ?", chatID, userID).Delete(&models.VibeChat{})
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "chat not found"})
		return
	}

	// Delete messages
	database.DB.Where("chat_id = ?", chatID).Delete(&models.VibeChatMessage{})

	c.JSON(http.StatusOK, gin.H{"message": "chat deleted"})
}

// VibeChatMessages handles GET /api/vibe-write/chats/:chatId/messages
func VibeChatMessages(c *gin.Context) {
	userID, _, ok := getEmailSessionUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "login required"})
		return
	}

	chatID, err := uuid.Parse(c.Param("chatId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid chat ID"})
		return
	}

	var chat models.VibeChat
	if err := database.DB.Where("id = ? AND user_id = ?", chatID, userID).First(&chat).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "chat not found"})
		return
	}

	var messages []models.VibeChatMessage
	database.DB.Where("chat_id = ?", chatID).Order("created_at ASC").Find(&messages)

	c.JSON(http.StatusOK, gin.H{"messages": messages})
}

// VibeChatSendMessage handles POST /api/vibe-write/chats/:chatId/messages
// Deducts credits and sends message to LLM (non-streaming for now).
func VibeChatSendMessage(c *gin.Context) {
	userID, _, ok := getEmailSessionUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "login required"})
		return
	}

	chatID, err := uuid.Parse(c.Param("chatId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid chat ID"})
		return
	}

	var chat models.VibeChat
	if err := database.DB.Where("id = ? AND user_id = ?", chatID, userID).First(&chat).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "chat not found"})
		return
	}

	var req struct {
		Content string `json:"content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "content is required"})
		return
	}

	// Deduct credits
	creditCost := services.CreditCostMessage
	if err := services.DeductCredits(userID, creditCost); err != nil {
		c.JSON(http.StatusPaymentRequired, gin.H{"error": err.Error(), "upgrade": true})
		return
	}

	// Save user message
	userMsg := models.VibeChatMessage{
		ChatID:  chatID,
		Role:    "user",
		Content: req.Content,
	}
	database.DB.Create(&userMsg)

	// Update chat title if first message
	var msgCount int64
	database.DB.Model(&models.VibeChatMessage{}).Where("chat_id = ?", chatID).Count(&msgCount)
	if msgCount == 1 {
		title := req.Content
		if len(title) > 50 {
			title = title[:50] + "..."
		}
		database.DB.Model(&chat).Update("title", title)
	}

	// Update chat timestamp
	database.DB.Model(&chat).Update("updated_at", time.Now())

	// --- Build LLM context ---

	// 1. Load workspace memories as system context
	var memories []models.VibeMemory
	database.DB.Where("workspace_id = ?", chat.WorkspaceID).Order("category ASC, sort_order ASC").Find(&memories)

	systemPrompt := buildVibeWriteSystemPrompt(memories)

	// 2. Load recent chat history (up to last 20 messages)
	var history []models.VibeChatMessage
	database.DB.Where("chat_id = ?", chatID).Order("created_at ASC").Find(&history)

	llmMessages := []services.ChatMessage{
		{Role: "system", Content: systemPrompt},
	}
	startIdx := 0
	if len(history) > 20 {
		startIdx = len(history) - 20
	}
	for _, msg := range history[startIdx:] {
		llmMessages = append(llmMessages, services.ChatMessage{Role: msg.Role, Content: msg.Content})
	}

	// 3. Call LLM
	_, _, llmModel, _ := config.Cfg.VibeWriteLLM()
	assistantContent, llmErr := services.CallVibeWriteLLM(llmMessages, 2000, 0.7)
	if llmErr != nil {
		// LLM failed — refund credits and return error
		database.DB.Model(&models.User{}).Where("id = ?", userID).
			Update("credits", gorm.Expr("credits + ?", creditCost))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "AI generation failed, credits refunded"})
		return
	}

	assistantMsg := models.VibeChatMessage{
		ChatID:      chatID,
		Role:        "assistant",
		Content:     assistantContent,
		CreditsCost: creditCost,
		Model:       llmModel,
	}
	database.DB.Create(&assistantMsg)

	c.JSON(http.StatusOK, gin.H{
		"user_message":      userMsg,
		"assistant_message": assistantMsg,
		"credits_used":      creditCost,
	})
}

// buildVibeWriteSystemPrompt constructs the system prompt from workspace memories.
func buildVibeWriteSystemPrompt(memories []models.VibeMemory) string {
	var sb strings.Builder

	sb.WriteString(`You are Vibe Write, an AI writing partner for Twitter/X. Your goal is to help users craft engaging, authentic tweets and threads.

Guidelines:
- Write in the user's natural voice and style (use their profile/rules memories)
- Keep tweets under 280 characters unless writing a thread
- Be creative, concise, and engaging
- When generating tweet variants, provide 3 options: insightful, witty, casual
- Support both English and Chinese (match the user's language)
- Use knowledge memories to inform content accuracy
- Use network memories to understand the user's audience and relationships
`)

	// Group memories by category
	grouped := map[string][]string{}
	for _, m := range memories {
		grouped[m.Category] = append(grouped[m.Category], m.Content)
	}

	categoryLabels := map[string]string{
		"profile":   "USER PROFILE (who the user is)",
		"rules":     "WRITING RULES (style preferences and constraints)",
		"knowledge": "KNOWLEDGE (topics and expertise)",
		"network":   "NETWORK (audience and relationships)",
		"archive":   "ARCHIVE (past successful tweets for reference)",
	}

	for _, cat := range []string{"profile", "rules", "knowledge", "network", "archive"} {
		items, ok := grouped[cat]
		if !ok || len(items) == 0 {
			continue
		}
		sb.WriteString(fmt.Sprintf("\n=== %s ===\n", categoryLabels[cat]))
		for _, item := range items {
			sb.WriteString("- ")
			sb.WriteString(strings.TrimSpace(item))
			sb.WriteString("\n")
		}
	}

	return sb.String()
}
