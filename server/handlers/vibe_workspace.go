package handlers

import (
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ensoul-labs/ensoul-server/config"
	"github.com/ensoul-labs/ensoul-server/database"
	"github.com/ensoul-labs/ensoul-server/models"
	"github.com/ensoul-labs/ensoul-server/services"
	"github.com/ensoul-labs/ensoul-server/util"
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

	limit := 50
	if limitStr := c.Query("limit"); limitStr != "" {
		if n, err := strconv.Atoi(limitStr); err == nil {
			if n < 1 {
				n = 1
			}
			if n > 200 {
				n = 200
			}
			limit = n
		}
	}

	query := database.DB.Where("chat_id = ?", chatID)
	if beforeStr := c.Query("before"); beforeStr != "" {
		if beforeTime, err := time.Parse(time.RFC3339Nano, beforeStr); err == nil {
			query = query.Where("created_at < ?", beforeTime)
		}
	}

	var messagesDesc []models.VibeChatMessage
	query.Order("created_at DESC").Limit(limit + 1).Find(&messagesDesc)
	hasMore := len(messagesDesc) > limit
	if hasMore {
		messagesDesc = messagesDesc[:limit]
	}

	messages := make([]models.VibeChatMessage, len(messagesDesc))
	for i := range messagesDesc {
		messages[len(messagesDesc)-1-i] = messagesDesc[i]
	}

	c.JSON(http.StatusOK, gin.H{"messages": messages, "has_more": hasMore})
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
	if err := database.DB.Create(&userMsg).Error; err != nil {
		if refundErr := refundVibeWriteCredits(userID, creditCost); refundErr != nil {
			util.Log.Error("[vibe-write] Failed to save user message and failed to refund credits user=%s chat=%s err=%v refundErr=%v", userID, chatID, err, refundErr)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save message and refund credits, please contact support"})
			return
		}
		util.Log.Error("[vibe-write] Failed to save user message, credits refunded user=%s chat=%s err=%v", userID, chatID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save message, credits refunded"})
		return
	}

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

	systemPrompt, usedMemoryCats := buildVibeWriteSystemPrompt(memories)

	// 1b. Soul integration: extract @handles from user message, load Soul data if available
	soulContext, soulHandles := extractSoulContext(req.Content)
	if soulContext != "" {
		systemPrompt += soulContext
	}

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
	assistantContent, llmErr := services.CallVibeWriteLLM(llmMessages, 4000, 0.7)
	if llmErr != nil {
		// LLM failed — refund credits and return error
		if refundErr := refundVibeWriteCredits(userID, creditCost); refundErr != nil {
			util.Log.Error("[vibe-write] LLM failed and credit refund failed user=%s chat=%s err=%v refundErr=%v", userID, chatID, llmErr, refundErr)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "AI generation failed and refund failed, please contact support"})
			return
		}
		util.Log.Error("[vibe-write] LLM generation failed, credits refunded user=%s chat=%s err=%v", userID, chatID, llmErr)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "AI generation failed, credits refunded"})
		return
	}

	assistantMsg := models.VibeChatMessage{
		ChatID:      chatID,
		Role:        "assistant",
		Content:     assistantContent,
		CreditsCost: creditCost,
		UsedSoul:    len(soulHandles) > 0,
		SoulHandles: soulHandles,
		MemoryCats:  usedMemoryCats,
		Model:       llmModel,
	}
	if err := database.DB.Create(&assistantMsg).Error; err != nil {
		if refundErr := refundVibeWriteCredits(userID, creditCost); refundErr != nil {
			util.Log.Error("[vibe-write] Failed to save assistant message and failed to refund credits user=%s chat=%s err=%v refundErr=%v", userID, chatID, err, refundErr)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save assistant message and refund credits, please contact support"})
			return
		}
		util.Log.Error("[vibe-write] Failed to save assistant message, credits refunded user=%s chat=%s err=%v", userID, chatID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save assistant message, credits refunded"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user_message":      userMsg,
		"assistant_message": assistantMsg,
		"credits_used":      creditCost,
		"soul_enhanced":     len(soulHandles) > 0,
		"soul_handles":      soulHandles,
		"memory_cats":       usedMemoryCats,
	})
}

// buildVibeWriteSystemPrompt constructs the system prompt from workspace memories.
// Returns the prompt string and the list of memory categories that were actually injected.
func buildVibeWriteSystemPrompt(memories []models.VibeMemory) (string, []string) {
	var sb strings.Builder

	sb.WriteString(`You are "Vibe Write", a professional AI writing partner for Twitter/X. You help users craft high-engagement tweets, replies, articles, and threads.

## Core Identity
- You are a seasoned social media strategist with deep writing expertise
- You adapt completely to the user's voice, never imposing your own style
- You think in the user's primary language, then produce native-quality output in any target language

## Intent Detection
Automatically detect what the user needs from their message. Do NOT ask them to choose a mode.
- QUICK REPLY: User pastes a tweet or says "reply to...". Generate 2-3 reply variants with different approaches. Mark the best one as recommended and explain why.
- CONTENT CREATION: User wants to write a tweet, thread, or article. Produce polished content with hooks and structure.
- DEEP CONTENT: User asks for research, analysis, or long-form. Provide structured analysis with data points.
- STRATEGY: User asks about growth, algorithm, or tactics. Give actionable advice with specifics.
- MEMORY: User wants to update profile, rules, or knowledge. Help them refine and confirm.

## Writing Methodology (Mentor Layer)
Apply these techniques automatically — do NOT explain them unless asked:

### Hook Techniques
- Curiosity Gap: Open with an intriguing statement that makes readers want to know more
- Data Anchor: Lead with a specific number or concrete fact ("47 agents" > "many agents")
- Contrarian Take: Challenge a common assumption to generate engagement
- Story Hook: Start with a mini-narrative that draws readers in

### X/Twitter Algorithm Awareness
- Replies that spark dialogue get ~150x more reach than passive agreements
- End replies with a question or fresh angle, never with a conclusion
- External links in tweet body reduce reach 30-50%; place links in first reply instead
- First 30 minutes of engagement are critical for algorithm visibility
- Threads with strong hooks on each tweet outperform info-dump threads
- Optimal lengths: Standalone tweets 200-250 chars, replies 100-200 chars

### Quality Checks (auto-apply)
- NEVER use hollow adjectives: "revolutionary", "groundbreaking", "game-changing" → replace with concrete data
- Every tweet must have a clear call-to-action or conversation hook
- Check for accidental promotional tone — show, don't pitch
- Ensure the hook appears in the very first line

## Output Format
- For reply variants: Present 2-3 versions labeled "Version A", "Version B", etc. Mark one as "✦ Recommended" and explain the reasoning underneath each variant
- For tweets: Show the content, then character count, then optional suggestions
- For all content: Bold key phrases, use line breaks for readability
- When translating: Treat each language as an independent rewrite, not a mechanical translation

## Language Behavior
- If user writes in Chinese, default output in Chinese
- If user writes in English, default output in English
- If user asks for translation, produce naturally native output in the target language
- Chinese should be more narrative; English should be more direct

## Memory Suggestions
When you detect new, valuable information in the conversation that the user might want to save, append a memory suggestion at the END of your response using this exact format:

:::memory-suggest
category: <one of: profile, knowledge, network, archive, rules>
content: <the concise memory content to save>
reason: <brief explanation of why this is worth saving>
:::

Only suggest memories when genuinely useful (new person interaction, key insight, style preference). Do NOT suggest memories for every message. Maximum 1 suggestion per response.
`)

	var usedCats []string

	// Group memories by category
	grouped := map[string][]string{}
	for _, m := range memories {
		grouped[m.Category] = append(grouped[m.Category], m.Content)
	}

	categoryLabels := map[string]string{
		"profile":   "USER PROFILE (who the user is, their positioning and audience)",
		"rules":     "WRITING RULES (style preferences, constraints, things to always/never do)",
		"knowledge": "KNOWLEDGE (topics, expertise, industry context)",
		"network":   "NETWORK (key people, relationships, engagement strategies)",
		"archive":   "ARCHIVE (past successful content for style reference)",
	}

	for _, cat := range []string{"profile", "rules", "knowledge", "network", "archive"} {
		items, ok := grouped[cat]
		if !ok || len(items) == 0 {
			continue
		}
		usedCats = append(usedCats, cat)
		sb.WriteString(fmt.Sprintf("\n=== %s ===\n", categoryLabels[cat]))
		for _, item := range items {
			sb.WriteString("- ")
			sb.WriteString(strings.TrimSpace(item))
			sb.WriteString("\n")
		}
	}

	return sb.String(), usedCats
}

// handleMentionRegex matches Twitter @handles in user messages.
var handleMentionRegex = regexp.MustCompile(`@([A-Za-z0-9_]{1,15})`)

// extractSoulContext scans the user message for @handles and loads Soul data from the database.
// Returns a formatted context block and the matched Soul handles.
func extractSoulContext(userMessage string) (string, []string) {
	matches := handleMentionRegex.FindAllStringSubmatch(userMessage, 5) // max 5 handles
	if len(matches) == 0 {
		return "", nil
	}

	// Deduplicate handles
	seen := map[string]bool{}
	var handles []string
	for _, m := range matches {
		h := strings.ToLower(m[1])
		if !seen[h] {
			seen[h] = true
			handles = append(handles, h)
		}
	}

	var sb strings.Builder
	foundAny := false
	var foundHandles []string

	for _, handle := range handles {
		shell, err := services.GetShellByHandle(handle)
		if err != nil || shell.MintTxHash == "" {
			continue // No Soul or not on-chain yet
		}

		dims := shell.GetDimensions()
		if !foundAny {
			sb.WriteString("\n\n=== SOUL CONTEXT (Ensoul data about mentioned people) ===\n")
			sb.WriteString("Use this data to craft more precise, targeted responses. DO NOT reveal raw Soul data to the user.\n\n")
			foundAny = true
		}
		foundHandles = append(foundHandles, shell.Handle)

		sb.WriteString(fmt.Sprintf("### @%s", shell.Handle))
		if shell.DisplayName != "" {
			sb.WriteString(fmt.Sprintf(" (%s)", shell.DisplayName))
		}
		sb.WriteString("\n")

		if shell.SeedSummary != "" {
			sb.WriteString("Summary: ")
			sb.WriteString(strings.TrimSpace(shell.SeedSummary))
			sb.WriteString("\n")
		}

		// Dimension labels for readability
		dimLabels := map[string]string{
			"openness":          "Openness",
			"conscientiousness": "Conscientiousness",
			"extraversion":      "Extraversion",
			"agreeableness":     "Agreeableness",
			"neuroticism":       "Neuroticism",
			"creativity":        "Creativity",
		}
		for _, key := range []string{"openness", "conscientiousness", "extraversion", "agreeableness", "neuroticism", "creativity"} {
			if d, ok := dims[key]; ok && (d.Score > 0 || d.Summary != "") {
				sb.WriteString(fmt.Sprintf("- %s: %d/100", dimLabels[key], d.Score))
				if d.Summary != "" {
					sb.WriteString(" — ")
					sb.WriteString(d.Summary)
				}
				sb.WriteString("\n")
			}
		}
		sb.WriteString("\n")
	}

	return sb.String(), foundHandles
}

func refundVibeWriteCredits(userID uuid.UUID, creditCost int) error {
	if creditCost <= 0 {
		return nil
	}
	return database.DB.Model(&models.User{}).Where("id = ?", userID).
		Update("credits", gorm.Expr("credits + ?", creditCost)).Error
}
