package services

import (
	"fmt"
	"strings"
	"time"

	"github.com/ensoul-labs/ensoul-server/database"
	"github.com/ensoul-labs/ensoul-server/models"
	"github.com/ensoul-labs/ensoul-server/util"
	"github.com/google/uuid"
)

// ReplyStyle represents a generated reply variant.
type ReplyStyle struct {
	Style   string `json:"style"`
	Content string `json:"content"`
	Model   string `json:"model"`
}

// GenerateReplies is DEPRECATED — use Snipe() instead.
// Kept for backward compatibility with old API clients.
func GenerateReplies(walletAddr, handle, tweetID, tweetText string) (*models.SniperReply, error) {
	return Snipe(walletAddr, handle, tweetID, tweetText, "", "")
}

// Snipe generates reply suggestions for any tweet (Sniper 2.0).
// Soul association is optional — if the author has a Soul, it's used for persona enrichment.
// Requires an active Pro subscription.
func Snipe(walletAddr, authorHandle, tweetID, tweetText, tagID, language string) (*models.SniperReply, error) {
	// 1. Verify Pro subscription
	sub, err := GetActiveSubscription(walletAddr)
	if err != nil {
		return nil, fmt.Errorf("Pro subscription required to snipe")
	}

	if sub.Tier != models.SubTierPro {
		return nil, fmt.Errorf("Pro subscription required to snipe")
	}

	// 2. Check daily limit (50/day for Pro)
	canReply, used, err := CheckDailyReplyLimit(walletAddr, sub)
	if err != nil {
		return nil, err
	}
	if !canReply {
		tierCfg := models.SubscriptionTiers[sub.Tier]
		return nil, fmt.Errorf("daily snipe limit reached (%d/%d)", used, tierCfg.DailyReplies)
	}

	// 3. Check for cached reply (same user + tweet)
	var existing models.SniperReply
	if err := database.DB.Where("wallet_addr = ? AND tweet_id = ?", walletAddr, tweetID).
		First(&existing).Error; err == nil {
		// Validate cached data — old records may have empty replies due to a storage bug
		if variants, ok := existing.Replies["variants"]; ok {
			if arr, ok := variants.([]interface{}); ok && len(arr) > 0 {
				return &existing, nil
			}
		}
		// Invalid cache: delete and regenerate
		database.DB.Delete(&existing)
	}

	// 4. Try to find the author's Soul (optional)
	var shell *models.Shell
	usedSoul := false
	shellByHandle, err := GetShellByHandle(authorHandle)
	if err == nil {
		shell = shellByHandle
		usedSoul = true
	}

	// 5. Load user persona (optional)
	persona := getUserPersona(walletAddr)

	// 6. Generate reply variants (language from request overrides persona setting)
	snipeLang := language
	if snipeLang == "" && persona != nil {
		snipeLang = persona.Language
	}
	if snipeLang == "" {
		snipeLang = "en"
	}
	replies, err := generateSnipeVariants(shell, persona, authorHandle, tweetText, sub.LLMModel, snipeLang)
	if err != nil {
		return nil, fmt.Errorf("reply generation failed: %w", err)
	}

	// 7. Build replies JSON — wrap in object for models.JSON (map) compatibility
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

	// 9. Save sniper reply record
	var shellID *uuid.UUID
	if shell != nil {
		shellID = &shell.ID
	}

	sniperReply := &models.SniperReply{
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

	if err := database.DB.Create(sniperReply).Error; err != nil {
		return nil, fmt.Errorf("failed to save reply: %w", err)
	}

	// 10. Record usage for holder revenue (only if Soul was used)
	if shell != nil {
		RecordUsage(shell.ID, walletAddr)
	}

	util.Log.Info("[sniper] Snipe: generated %d replies for @%s tweet %s (user: %s, soul: %v)",
		len(replies), authorHandle, tweetID, walletAddr, usedSoul)

	return sniperReply, nil
}

// generateSnipeVariants uses LLM to generate 3 reply styles.
// shell is optional (nil = no Soul persona, use generic crypto persona).
func generateSnipeVariants(shell *models.Shell, persona *models.UserPersona, authorHandle, tweetText, llmModel, language string) ([]ReplyStyle, error) {
	// Build soul context (optional)
	var soulSection string
	if shell != nil {
		soulSection = buildRichSoulPrompt(shell)
	} else {
		soulSection = `
=== AUTHOR CONTEXT ===
No Soul profile available for @` + authorHandle + `.
Use your general knowledge of the crypto/web3 community to generate relevant replies.
Analyze the tweet content to determine the appropriate tone and topic.`
	}

	// Build persona context (optional)
	var personaSection string
	if persona != nil {
		personaSection = fmt.Sprintf(`
=== YOUR PERSONA (the person replying) ===
Bio: %s
Communication Style: %s
Reference Materials: %s
Preferred Language: %s
`, persona.Bio, persona.Style, persona.Materials, language)
	} else {
		personaSection = fmt.Sprintf("\n=== YOUR PERSONA ===\nNo persona configured. Generate replies in a professional crypto-native tone.\nPreferred Language: %s\n", language)
	}

	prompt := fmt.Sprintf(`You are a reply generation engine for Soul Sniper.

%s

%s

=== TWEET TO REPLY TO ===
@%s posted: "%s"

=== YOUR TASK ===
Generate exactly 3 reply variants in different styles:
1. "insightful" — A thoughtful, knowledge-driven reply that adds value
2. "witty" — A clever, engaging reply with personality
3. "supportive" — A warm, encouraging reply that builds rapport

Each reply MUST:
- Be under 280 characters (Twitter limit)
- Sound natural, not AI-generated
- Be relevant to the tweet content
- If a Soul profile is available, reflect that personality
- If a persona is configured, reflect that style
- MUST be written in the Preferred Language specified above (if "auto", match the tweet's language)

Respond in JSON format ONLY:
[
  {"style": "insightful", "content": "..."},
  {"style": "witty", "content": "..."},
  {"style": "supportive", "content": "..."}
]`, soulSection, personaSection, authorHandle, tweetText)

	var replies []ReplyStyle
	err := CallSniperLLMJSON([]ChatMessage{
		{Role: "system", Content: "You are a precise reply generation engine. Output valid JSON only, no markdown."},
		{Role: "user", Content: prompt},
	}, 1000, 0.8, &replies)

	if err != nil {
		return nil, fmt.Errorf("LLM reply generation failed: %w", err)
	}

	// Tag each reply with the model used
	for i := range replies {
		replies[i].Model = llmModel
	}

	return replies, nil
}

// generateReplyVariants is DEPRECATED — use generateSnipeVariants.
// Kept for backward compatibility.
func generateReplyVariants(shell *models.Shell, persona *models.UserPersona, tweetText, llmModel string) ([]ReplyStyle, error) {
	lang := "en"
	if persona != nil && persona.Language != "" {
		lang = persona.Language
	}
	return generateSnipeVariants(shell, persona, shell.Handle, tweetText, llmModel, lang)
}

// getUserPersona returns the user's persona, or nil if not set.
func getUserPersona(walletAddr string) *models.UserPersona {
	var persona models.UserPersona
	if err := database.DB.Where("wallet_addr = ?", walletAddr).First(&persona).Error; err != nil {
		return nil
	}
	return &persona
}

// SetUserPersona creates or updates the user's persona.
func SetUserPersona(walletAddr, bio, style, materials, language string) (*models.UserPersona, error) {
	if language == "" {
		language = "en"
	}

	var persona models.UserPersona
	if err := database.DB.Where("wallet_addr = ?", walletAddr).First(&persona).Error; err != nil {
		// Create new
		persona = models.UserPersona{
			WalletAddr: walletAddr,
			Bio:        bio,
			Style:      style,
			Materials:  materials,
			Language:   language,
		}
		if err := database.DB.Create(&persona).Error; err != nil {
			return nil, fmt.Errorf("failed to create persona: %w", err)
		}
	} else {
		// Update existing
		persona.Bio = bio
		persona.Style = style
		persona.Materials = materials
		persona.Language = language
		database.DB.Save(&persona)
	}

	return &persona, nil
}

// GetUserPersona returns the user's persona for display.
func GetUserPersona(walletAddr string) (*models.UserPersona, error) {
	p := getUserPersona(walletAddr)
	if p == nil {
		return nil, fmt.Errorf("no persona configured")
	}
	return p, nil
}

// GetUserReplies returns the user's recent sniper replies.
func GetUserReplies(walletAddr string, limit int) ([]models.SniperReply, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	var replies []models.SniperReply
	if err := database.DB.Where("wallet_addr = ?", walletAddr).
		Preload("Shell").
		Order("created_at DESC").
		Limit(limit).
		Find(&replies).Error; err != nil {
		return nil, err
	}

	return replies, nil
}

// MonitorKOLTweets is DEPRECATED — replaced by tag-based feed refresher in Sniper 2.0.
// Kept for backward compatibility. New system uses StartFeedRefresher().
func MonitorKOLTweets() {
	// Get all active subscriptions with their KOLs
	var subs []models.Subscription
	database.DB.Where("status = ?", models.SubStatusActive).Find(&subs)

	if len(subs) == 0 {
		return
	}

	// Collect unique handles across all subscriptions
	handleToSubs := make(map[string][]models.Subscription)
	for _, sub := range subs {
		var kols []models.SniperKOL
		database.DB.Where("subscription_id = ?", sub.ID).Find(&kols)
		for _, kol := range kols {
			handleToSubs[strings.ToLower(kol.Handle)] = append(handleToSubs[strings.ToLower(kol.Handle)], sub)
		}
	}

	util.Log.Info("[sniper] Monitoring %d unique KOLs across %d subscriptions", len(handleToSubs), len(subs))

	for handle, subscribers := range handleToSubs {
		tweets, err := fetchRecentTweets(handle)
		if err != nil {
			util.Log.Warn("[sniper] Failed to fetch tweets for @%s: %v", handle, err)
			continue
		}

		for _, tweet := range tweets {
			for _, sub := range subscribers {
				// Check if reply already exists
				var existing models.SniperReply
				if err := database.DB.Where("wallet_addr = ? AND tweet_id = ?", sub.WalletAddr, tweet.ID).
					First(&existing).Error; err == nil {
					continue // Already generated
				}

				// Check daily limit
				canReply, _, _ := CheckDailyReplyLimit(sub.WalletAddr, &sub)
				if !canReply {
					continue
				}

				// Generate replies asynchronously
				go func(wallet, h, tID, tText string) {
					if _, err := GenerateReplies(wallet, h, tID, tText); err != nil {
						util.Log.Warn("[sniper] Auto-reply failed for @%s tweet %s: %v", h, tID, err)
					}
				}(sub.WalletAddr, handle, tweet.ID, tweet.Text)
			}
		}
	}
}

// fetchRecentTweets fetches the latest tweets from a handle (last 1 hour).
func fetchRecentTweets(handle string) ([]TwitterTweet, error) {
	if !SocialDataAvailable() {
		return nil, fmt.Errorf("SocialData API not configured")
	}

	client := newSocialDataClient()
	user, err := client.FetchUser(handle)
	if err != nil {
		return nil, err
	}

	tweets, err := client.FetchTweets(user.IDStr, 5)
	if err != nil {
		return nil, err
	}

	// Filter to tweets from the last hour
	cutoff := time.Now().Add(-1 * time.Hour)
	var recent []TwitterTweet
	for _, t := range tweets {
		text := t.FullText
		if text == "" && t.Text != nil {
			text = *t.Text
		}
		// Parse tweet time — SocialData format: "Mon Jan 02 15:04:05 +0000 2006"
		tweetTime, err := time.Parse("Mon Jan 02 15:04:05 +0000 2006", t.TweetCreatedAt)
		if err != nil {
			continue
		}
		if tweetTime.After(cutoff) {
			recent = append(recent, TwitterTweet{
				ID:        t.IDStr,
				Text:      text,
				CreatedAt: t.TweetCreatedAt,
			})
		}
	}

	return recent, nil
}

// StartSniperMonitor is DEPRECATED — replaced by StartFeedRefresher() in Sniper 2.0.
func StartSniperMonitor(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			MonitorKOLTweets()
		}
	}()
	util.Log.Info("[sniper] KOL tweet monitor started (interval: %s)", interval)
}
