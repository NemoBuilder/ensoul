package services

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ensoul-labs/ensoul-server/database"
	"github.com/ensoul-labs/ensoul-server/models"
	"github.com/ensoul-labs/ensoul-server/util"
)

// ReplyStyle represents a generated reply variant.
type ReplyStyle struct {
	Style   string `json:"style"`
	Content string `json:"content"`
	Model   string `json:"model"`
}

// GenerateReplies generates reply suggestions for a KOL's tweet.
// Uses the Soul's personality + user's persona to craft contextual replies.
func GenerateReplies(walletAddr, handle, tweetID, tweetText string) (*models.SniperReply, error) {
	// Verify subscription
	sub, err := GetActiveSubscription(walletAddr)
	if err != nil {
		return nil, fmt.Errorf("active subscription required: %w", err)
	}

	// Check daily limit
	canReply, used, err := CheckDailyReplyLimit(walletAddr, sub)
	if err != nil {
		return nil, err
	}
	if !canReply {
		tierCfg := models.SubscriptionTiers[sub.Tier]
		return nil, fmt.Errorf("daily reply limit reached (%d/%d)", used, tierCfg.DailyReplies)
	}

	// Check the KOL is in user's tracking list
	var kol models.SniperKOL
	if err := database.DB.Where("subscription_id = ? AND handle = ?", sub.ID, handle).
		First(&kol).Error; err != nil {
		return nil, fmt.Errorf("@%s is not in your tracking list", handle)
	}

	// Check for duplicate tweet reply
	var existing models.SniperReply
	if err := database.DB.Where("wallet_addr = ? AND tweet_id = ?", walletAddr, tweetID).
		First(&existing).Error; err == nil {
		return &existing, nil // Return cached reply
	}

	// Load the Soul
	shell, err := GetShellByHandle(handle)
	if err != nil {
		return nil, fmt.Errorf("soul @%s not found", handle)
	}

	// Load user persona
	persona := getUserPersona(walletAddr)

	// Build the reply generation prompt
	replies, err := generateReplyVariants(shell, persona, tweetText, sub.LLMModel)
	if err != nil {
		return nil, fmt.Errorf("reply generation failed: %w", err)
	}

	// Convert replies to JSON
	repliesJSON, _ := json.Marshal(replies)

	// Save reply record
	sniperReply := &models.SniperReply{
		ShellID:    shell.ID,
		WalletAddr: walletAddr,
		TweetID:    tweetID,
		TweetText:  tweetText,
		Replies:    models.JSON{},
	}
	// Store replies as JSON
	json.Unmarshal(repliesJSON, &sniperReply.Replies)

	if err := database.DB.Create(sniperReply).Error; err != nil {
		return nil, fmt.Errorf("failed to save reply: %w", err)
	}

	// Record usage for holder revenue calculation
	RecordUsage(shell.ID)

	util.Log.Info("[sniper] Generated %d replies for @%s tweet %s (user: %s)",
		len(replies), handle, tweetID, walletAddr)

	return sniperReply, nil
}

// generateReplyVariants uses LLM to generate 3 reply styles.
func generateReplyVariants(shell *models.Shell, persona *models.UserPersona, tweetText, llmModel string) ([]ReplyStyle, error) {
	// Build rich soul context
	soulContext := buildRichSoulPrompt(shell)

	// Build persona context
	var personaSection string
	if persona != nil {
		personaSection = fmt.Sprintf(`
=== YOUR PERSONA (the person replying) ===
Bio: %s
Communication Style: %s
Reference Materials: %s
Preferred Language: %s
`, persona.Bio, persona.Style, persona.Materials, persona.Language)
	} else {
		personaSection = "\n=== YOUR PERSONA ===\nNo persona configured. Generate replies in a general professional tone.\n"
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
- Reflect the soul's personality and the user's persona
- Be relevant to the tweet content

Respond in JSON format ONLY:
[
  {"style": "insightful", "content": "..."},
  {"style": "witty", "content": "..."},
  {"style": "supportive", "content": "..."}
]`, soulContext, personaSection, shell.Handle, tweetText)

	var replies []ReplyStyle
	err := CallLLMJSON([]ChatMessage{
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

// MonitorKOLTweets checks for new tweets from all tracked KOLs across all active subscriptions.
// Called periodically by the scheduler.
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

// StartSniperMonitor starts the background KOL tweet monitor.
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
