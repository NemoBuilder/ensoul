package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/ensoul-labs/ensoul-server/config"
	"github.com/ensoul-labs/ensoul-server/database"
	"github.com/ensoul-labs/ensoul-server/models"
	"github.com/ensoul-labs/ensoul-server/services"
	"github.com/ensoul-labs/ensoul-server/services/methodology"
	"github.com/ensoul-labs/ensoul-server/util"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// VibeChatStreamMessage handles POST /api/vibe-write/chats/:chatId/messages/stream
//
// SSE event protocol (V3 unified):
//
//	event: meta            { user_message_id, scenario, mode }
//	event: context         { used_memory_cats, soul_handles, soul_enhanced, methodology_slugs, output_langs, variant_count }
//	event: chunk           "<token text>"            — chat / translate mode
//	event: variant         { idx, content, recommended, reason, lang }   — reply / translate mode
//	event: memory_suggest  { id, category, content, reason }
//	event: soul_lock       { handle, upgrade: true } — Free user pasted a tweet from a Soul-owning author
//	event: done            { assistant_message_id, credits_used, total_chars, model, cleaned_content, pending_memories, mode }
//	event: error           "<message>"               — terminal; credits already refunded
//
// Modes:
//   - "reply":     request includes attached_tweet. Generates variant_count replies (Pro: ≤5, Free: 1)
//                  via JSON LLM call; emits one event:variant per candidate.
//   - "translate": request includes output_langs of length ≥ 2. Streams primary content as chunks,
//                  then translates into each extra lang and emits event:variant with lang set.
//   - "chat":      default — token streaming.
//
// Credits are deducted up-front and refunded on any failure path.
func VibeChatStreamMessage(c *gin.Context) {
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
		c.JSON(http.StatusNotFound, gin.H{"error": "chat not found", "code": "NOT_FOUND"})
		return
	}

	var req struct {
		Content       string                  `json:"content" binding:"required"`
		AttachedTweet *services.AttachedTweet `json:"attached_tweet,omitempty"`
		VariantCount  int                     `json:"variant_count,omitempty"`
		OutputLangs   []string                `json:"output_langs,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "content is required"})
		return
	}
	if len(req.Content) > 8000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "content too long (max 8000 characters)"})
		return
	}

	var user models.User
	if err := database.DB.First(&user, "id = ?", userID).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
		return
	}
	isPro := user.IsPro()

	tweet := services.NormalizeAttachedTweet(req.AttachedTweet)
	mode := "chat"
	if tweet != nil {
		mode = "reply"
	}

	wantVariants := req.VariantCount
	if mode == "reply" {
		if wantVariants <= 0 {
			if isPro {
				wantVariants = 3
			} else {
				wantVariants = 1
			}
		}
		if wantVariants > 1 && !isPro {
			c.JSON(http.StatusPaymentRequired, gin.H{
				"error":   "multiple reply variants require Pro",
				"code":    "PRO_REQUIRED",
				"reason":  "variant_count",
				"upgrade": true,
			})
			return
		}
		if wantVariants > 5 {
			wantVariants = 5
		}
	} else {
		wantVariants = 1
	}

	// Output language list: dedupe + first is primary, rest are translations
	primaryLang := ""
	var extraLangs []string
	if len(req.OutputLangs) > 0 {
		seen := map[string]bool{}
		for _, l := range req.OutputLangs {
			l = strings.ToLower(strings.TrimSpace(l))
			if l == "" || seen[l] {
				continue
			}
			seen[l] = true
			if primaryLang == "" {
				primaryLang = l
			} else {
				extraLangs = append(extraLangs, l)
			}
		}
	}
	if len(extraLangs) > 0 && mode == "chat" {
		mode = "translate"
	}

	creditCost := services.CreditCostMessage
	switch wantVariants {
	case 3:
		creditCost = services.CreditCostVariant3
	case 4, 5:
		creditCost = services.CreditCostVariant5
	}
	creditCost += len(extraLangs)

	// Soul lookup for attached tweet author (drives gating + bonus credit)
	var attachedSoul *models.Shell
	var soulLockedForFree bool
	if tweet != nil && tweet.AuthorHandle != "" {
		if shell, sErr := services.GetShellByHandle(tweet.AuthorHandle); sErr == nil && shell != nil && shell.MintTxHash != "" {
			attachedSoul = shell
			if isPro {
				creditCost += services.CreditCostSoulContext
			} else {
				soulLockedForFree = true
			}
		}
	}

	if err := services.DeductCredits(userID, creditCost); err != nil {
		c.JSON(http.StatusPaymentRequired, gin.H{
			"error":   err.Error(),
			"code":    "INSUFFICIENT_CREDITS",
			"upgrade": true,
			"need":    creditCost,
		})
		return
	}

	refundOnFail := func(reason string, err error) {
		if rErr := refundVibeWriteCredits(userID, creditCost); rErr != nil {
			util.Log.Error("[vibe-write/stream] %s and credit refund failed user=%s chat=%s err=%v refundErr=%v",
				reason, userID, chatID, err, rErr)
			return
		}
		util.Log.Error("[vibe-write/stream] %s, credits refunded user=%s chat=%s err=%v",
			reason, userID, chatID, err)
	}

	// Persist user message
	userContent := strings.TrimSpace(req.Content)
	if tweet != nil && userContent == "" {
		userContent = fmt.Sprintf("[reply to @%s]", tweet.AuthorHandle)
	}
	userMsg := models.VibeChatMessage{ChatID: chatID, Role: "user", Content: userContent}
	if err := database.DB.Create(&userMsg).Error; err != nil {
		refundOnFail("failed to save user message", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save message, credits refunded"})
		return
	}

	var msgCount int64
	database.DB.Model(&models.VibeChatMessage{}).Where("chat_id = ?", chatID).Count(&msgCount)
	if msgCount == 1 {
		title := userContent
		if len(title) > 50 {
			title = title[:50] + "..."
		}
		database.DB.Model(&chat).Update("title", title)
	}
	database.DB.Model(&chat).Update("updated_at", time.Now())

	// LLM context
	var memories []models.VibeMemory
	database.DB.Where("workspace_id = ? AND status = ?", chat.WorkspaceID, models.MemoryStatusAccepted).
		Order("category ASC, sort_order ASC").Find(&memories)
	systemPrompt, usedMemoryCats := buildVibeWriteSystemPrompt(memories)

	scenario := methodology.ScenarioGeneral
	var methodologySlugs []string
	routingInput := req.Content
	if tweet != nil {
		routingInput = req.Content + " " + tweet.Text + " reply"
	}
	if mLoad, mErr := methodology.Load(database.DB, routingInput, methodology.LoadOptions{MaxBodyChars: 12000}); mErr == nil && mLoad != nil {
		if section := mLoad.RenderPromptSection(); section != "" {
			systemPrompt += section
		}
		scenario = mLoad.Scenario
		methodologySlugs = mLoad.UsedSlugs
	} else if mErr != nil {
		util.Log.Warn("[vibe-write/stream] mentor methodology load failed (non-fatal): %v", mErr)
	}

	soulContext, soulHandles := extractSoulContext(req.Content)
	if soulContext != "" {
		systemPrompt += soulContext
	}
	attachedSoulContext := ""
	if attachedSoul != nil && isPro {
		attachedSoulContext, _ = extractSoulContext("@" + attachedSoul.Handle)
	}
	allSoulHandles := soulHandles
	if attachedSoul != nil {
		allSoulHandles = appendUnique(allSoulHandles, attachedSoul.Handle)
	}

	// SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache, no-transform")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Writer.WriteHeader(http.StatusOK)

	writeEvent := func(event string, payload interface{}) {
		var data string
		switch v := payload.(type) {
		case string:
			b, _ := json.Marshal(v)
			data = string(b)
		default:
			b, _ := json.Marshal(v)
			data = string(b)
		}
		fmt.Fprintf(c.Writer, "event: %s\ndata: %s\n\n", event, data)
		c.Writer.Flush()
	}

	writeEvent("meta", gin.H{
		"user_message_id": userMsg.ID,
		"scenario":        string(scenario),
		"mode":            mode,
	})

	writeEvent("context", gin.H{
		"used_memory_cats":  usedMemoryCats,
		"soul_handles":      allSoulHandles,
		"soul_enhanced":     attachedSoul != nil && isPro,
		"methodology_slugs": methodologySlugs,
		"output_langs":      append([]string{primaryLang}, extraLangs...),
		"variant_count":     wantVariants,
	})

	if soulLockedForFree && attachedSoul != nil {
		writeEvent("soul_lock", gin.H{
			"handle":  attachedSoul.Handle,
			"upgrade": true,
		})
	}

	_, _, llmModel, _ := config.Cfg.VibeWriteLLM()

	var (
		assistantContent string
		variants         []services.ReplyVariant
		earlySuggestions []services.MemorySuggestion
	)

	if mode == "reply" {
		primaryLangName := "auto (match the tweet language)"
		if primaryLang != "" {
			primaryLangName = services.LangNameForCode(primaryLang)
		}
		prompt := services.BuildReplyPrompt(tweet, req.Content, attachedSoulContext, wantVariants, primaryLangName)
		bundle, gErr := services.GenerateReplyVariants(prompt)
		if gErr != nil {
			refundOnFail("reply variant generation failed", gErr)
			writeEvent("error", "AI generation failed, credits refunded")
			return
		}
		variants = bundle.Variants
		earlySuggestions = bundle.MemorySuggestions

		for i, v := range variants {
			if v.Lang == "" {
				v.Lang = primaryLang
			}
			writeEvent("variant", gin.H{
				"idx":         i,
				"content":     v.Content,
				"recommended": v.Recommended,
				"reason":      v.Reason,
				"lang":        v.Lang,
			})
		}

		marshaled, _ := json.Marshal(gin.H{
			"mode":     "reply",
			"variants": variants,
			"tweet":    tweet,
		})
		assistantContent = string(marshaled)
	} else {
		// chat / translate mode — token streaming
		var history []models.VibeChatMessage
		database.DB.Where("chat_id = ?", chatID).Order("created_at ASC").Find(&history)
		llmMessages := []services.ChatMessage{{Role: "system", Content: systemPrompt}}
		startIdx := 0
		if len(history) > 20 {
			startIdx = len(history) - 20
		}
		for _, msg := range history[startIdx:] {
			llmMessages = append(llmMessages, services.ChatMessage{Role: msg.Role, Content: msg.Content})
		}

		var contentBuilder strings.Builder
		streamErr := services.StreamVibeWriteLLM(llmMessages, 4000, 0.7, func(chunk string) {
			if chunk == "" {
				return
			}
			contentBuilder.WriteString(chunk)
			writeEvent("chunk", chunk)
		})
		if streamErr != nil {
			refundOnFail("LLM stream failed", streamErr)
			writeEvent("error", "AI generation failed, credits refunded")
			return
		}
		raw := contentBuilder.String()
		if strings.TrimSpace(raw) == "" {
			refundOnFail("LLM returned empty response", fmt.Errorf("empty stream"))
			writeEvent("error", "AI returned empty response, credits refunded")
			return
		}
		cleaned, suggestions := services.ExtractMemorySuggestions(raw)
		assistantContent = cleaned
		earlySuggestions = suggestions

		if mode == "translate" {
			writeEvent("variant", gin.H{
				"idx":     0,
				"content": cleaned,
				"lang":    primaryLang,
			})
		}
	}

	// Translation pass for extra langs
	if len(extraLangs) > 0 {
		var srcText string
		if mode == "reply" {
			for _, v := range variants {
				if v.Recommended {
					srcText = v.Content
					break
				}
			}
			if srcText == "" && len(variants) > 0 {
				srcText = variants[0].Content
			}
		} else {
			srcText = assistantContent
		}
		baseIdx := len(variants)
		if mode == "translate" {
			baseIdx = 1
		}
		for i, lang := range extraLangs {
			if srcText == "" {
				continue
			}
			translated, tErr := services.TranslateText(srcText, services.LangNameForCode(lang))
			if tErr != nil {
				util.Log.Warn("[vibe-write/stream] translate to %s failed (non-fatal): %v", lang, tErr)
				continue
			}
			writeEvent("variant", gin.H{
				"idx":     baseIdx + i,
				"content": translated,
				"lang":    lang,
				"reason":  "translation",
			})
		}
	}

	assistantMsg := models.VibeChatMessage{
		ChatID:      chatID,
		Role:        "assistant",
		Content:     assistantContent,
		CreditsCost: creditCost,
		UsedSoul:    len(allSoulHandles) > 0 && (attachedSoul == nil || isPro),
		SoulHandles: allSoulHandles,
		MemoryCats:  usedMemoryCats,
		Model:       llmModel,
		Scenario:    string(scenario),
	}
	if err := database.DB.Create(&assistantMsg).Error; err != nil {
		refundOnFail("failed to save assistant message", err)
		writeEvent("error", "failed to save assistant message, credits refunded")
		return
	}

	pendingMems := persistMemorySuggestions(chat.WorkspaceID, earlySuggestions)
	for _, m := range pendingMems {
		writeEvent("memory_suggest", gin.H{
			"id":       m.ID,
			"category": m.Category,
			"content":  m.Content,
			"reason":   m.Reason,
		})
	}

	writeEvent("done", gin.H{
		"assistant_message_id": assistantMsg.ID,
		"credits_used":         creditCost,
		"total_chars":          len(assistantContent),
		"model":                llmModel,
		"cleaned_content":      assistantContent,
		"pending_memories":     pendingMems,
		"mode":                 mode,
	})
}

// appendUnique appends s to xs (case-insensitive de-dupe). Helper for soul handle merging.
func appendUnique(xs []string, s string) []string {
	low := strings.ToLower(s)
	for _, x := range xs {
		if strings.ToLower(x) == low {
			return xs
		}
	}
	return append(xs, s)
}
