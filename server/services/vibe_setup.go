package services

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ensoul-labs/ensoul-server/util"
)

// VibeSetupResult is what the LLM produces from a Twitter profile.
type VibeSetupResult struct {
	Suggestions []MemorySuggestion `json:"suggestions"`
}

// DistillTwitterProfile turns a fetched Twitter profile into seed memory
// suggestions across the 5 categories (profile/knowledge/network/archive/rules).
//
// The caller decides whether to persist as pending or accepted.
func DistillTwitterProfile(profile *TwitterProfile) ([]MemorySuggestion, error) {
	if profile == nil || profile.User.Username == "" {
		return nil, fmt.Errorf("empty profile")
	}

	// Build a compact context for the LLM (cap tweets to keep prompt small).
	const maxTweets = 30
	tweets := profile.Tweets
	if len(tweets) > maxTweets {
		tweets = tweets[:maxTweets]
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Twitter Handle: @%s\n", profile.User.Username)
	fmt.Fprintf(&sb, "Display Name: %s\n", profile.User.Name)
	if profile.User.Description != "" {
		fmt.Fprintf(&sb, "Bio: %s\n", profile.User.Description)
	}
	if profile.Location != "" {
		fmt.Fprintf(&sb, "Location: %s\n", profile.Location)
	}
	fmt.Fprintf(&sb, "Followers: %d, Following: %d, Tweet Count: %d\n",
		profile.User.PublicMetrics.FollowersCount,
		profile.User.PublicMetrics.FollowingCount,
		profile.User.PublicMetrics.TweetCount,
	)
	sb.WriteString("\nRecent Tweets:\n")
	for i, tw := range tweets {
		text := strings.ReplaceAll(tw.Text, "\n", " ")
		if len(text) > 240 {
			text = text[:240] + "…"
		}
		fmt.Fprintf(&sb, "%d. %s\n", i+1, text)
	}

	system := `You are an analyst building a "writing memory" for an AI writing partner.
Given a Twitter profile and recent tweets, derive 4–8 concise memory entries that capture:
- profile: who the user is, their voice / tone / style
- knowledge: topics / niches they consistently talk about (and any deep expertise)
- network: communities, archetypal audience, frequent collaborators (general only — do NOT list random @mentions)
- archive: notable themes or hooks from past tweets the user might revisit
- rules: writing rules that match the user's observed style (length, emoji use, hashtag policy, language mix)

OUTPUT REQUIREMENTS — STRICT JSON ONLY, no markdown fences, no commentary:
{
  "suggestions": [
    {"category": "profile|knowledge|network|archive|rules",
     "content": "1–2 sentences in the user's primary language",
     "reason": "why this matters (English, brief)"}
  ]
}

- Each "content" must be self-contained and immediately usable as a memory.
- Use the user's primary language for "content" (detect from tweets).
- Keep "reason" in English under 20 words.
- Do not invent facts not supported by the profile/tweets.
- Skip categories that have no support rather than padding.
- CRITICAL JSON SAFETY: inside any string value, NEVER use the ASCII double
  quote character ". To quote anything inline, use 「」 / 『』 (CJK corner
  brackets) or single quotes '...'. Unescaped " inside a value will break
  the JSON parser.`

	messages := []ChatMessage{
		{Role: "system", Content: system},
		{Role: "user", Content: sb.String()},
	}

	var result VibeSetupResult
	raw, err := CallVibeWriteLLM(messages, 1200, 0.4)
	if err != nil {
		return nil, fmt.Errorf("LLM call failed: %w", err)
	}

	if err := parseLLMJSON(raw, &result); err != nil {
		// Best-effort: try to find the first {...} block and parse.
		if start := strings.Index(raw, "{"); start >= 0 {
			if end := strings.LastIndex(raw, "}"); end > start {
				if jerr := json.Unmarshal([]byte(raw[start:end+1]), &result); jerr == nil {
					goto OK
				}
			}
		}
		util.Log.Warn("[vibe-setup] failed to parse LLM JSON: %v; raw=%q", err, truncateForLog(raw, 200))
		return nil, fmt.Errorf("failed to parse LLM response: %w", err)
	}

OK:
	// Filter invalid entries.
	out := make([]MemorySuggestion, 0, len(result.Suggestions))
	for _, s := range result.Suggestions {
		s.Category = strings.ToLower(strings.TrimSpace(s.Category))
		s.Content = strings.TrimSpace(s.Content)
		if !validMemoryCategory(s.Category) || s.Content == "" {
			continue
		}
		out = append(out, s)
	}
	return out, nil
}

func truncateForLog(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
