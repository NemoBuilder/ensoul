package services

import (
	"fmt"
	"regexp"
	"strings"
)

// ReplyVariant is one generated reply candidate.
type ReplyVariant struct {
	Content     string `json:"content"`
	Recommended bool   `json:"recommended"`
	Reason      string `json:"reason"`
	Lang        string `json:"lang,omitempty"` // optional, used by translation flow
}

// ReplyVariantBundle is the LLM JSON output for the reply pipeline.
type ReplyVariantBundle struct {
	Variants           []ReplyVariant      `json:"variants"`
	MemorySuggestions  []MemorySuggestion  `json:"memory_suggestions,omitempty"`
}

// AttachedTweet is the structured tweet payload the user pastes/attaches.
type AttachedTweet struct {
	URL          string `json:"url,omitempty"`
	AuthorHandle string `json:"author_handle,omitempty"`
	Text         string `json:"text,omitempty"`
}

var tweetURLRegex = regexp.MustCompile(`https?://(?:twitter\.com|x\.com)/([A-Za-z0-9_]{1,15})/status/(\d+)`)

// NormalizeAttachedTweet fills missing fields from the URL when possible.
// Returns nil if the input has no usable info.
func NormalizeAttachedTweet(t *AttachedTweet) *AttachedTweet {
	if t == nil {
		return nil
	}
	if t.URL != "" {
		if m := tweetURLRegex.FindStringSubmatch(t.URL); len(m) == 3 {
			if t.AuthorHandle == "" {
				t.AuthorHandle = strings.ToLower(m[1])
			}
		}
	}
	t.AuthorHandle = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(t.AuthorHandle)), "@")
	if t.AuthorHandle == "" && t.Text == "" {
		return nil
	}
	return t
}

// BuildReplyPrompt creates the LLM prompt for generating multi-variant replies.
// soulContext (may be empty) is the formatted Soul block already extracted by
// extractSoulContext; if non-empty the model is instructed to leverage it.
func BuildReplyPrompt(tweet *AttachedTweet, userIntent string, soulContext string, variantCount int, outputLang string) string {
	if variantCount < 1 {
		variantCount = 1
	}
	if variantCount > 5 {
		variantCount = 5
	}
	if outputLang == "" {
		outputLang = "auto (match the tweet's language)"
	}

	var sb strings.Builder
	sb.WriteString("You are crafting reply candidates to a tweet. Produce STRICTLY a JSON object that matches this schema:\n")
	sb.WriteString("{\n")
	sb.WriteString("  \"variants\": [{\"content\": string, \"recommended\": boolean, \"reason\": string}],\n")
	sb.WriteString("  \"memory_suggestions\": [{\"category\": one_of[profile,knowledge,network,archive,rules], \"content\": string, \"reason\": string}]\n")
	sb.WriteString("}\n\n")

	sb.WriteString(fmt.Sprintf("Generate exactly %d distinct reply variant(s). Mark exactly ONE as recommended=true. ", variantCount))
	sb.WriteString("Each `reason` must briefly explain (≤25 words) why this approach works (e.g. agreement, contrarian hook, question, pattern interrupt). ")
	sb.WriteString("`memory_suggestions` is OPTIONAL — include 0 or 1 entries only when you genuinely learned something new about the user or this person.\n\n")

	sb.WriteString("=== TWEET ===\n")
	if tweet.AuthorHandle != "" {
		sb.WriteString(fmt.Sprintf("Author: @%s\n", tweet.AuthorHandle))
	}
	if tweet.URL != "" {
		sb.WriteString(fmt.Sprintf("URL: %s\n", tweet.URL))
	}
	sb.WriteString("Text:\n")
	sb.WriteString(strings.TrimSpace(tweet.Text))
	sb.WriteString("\n\n")

	if soulContext != "" {
		sb.WriteString(soulContext)
		sb.WriteString("\nUse the Soul context above to tune tone & topic, but do NOT cite raw scores.\n")
	}

	if userIntent = strings.TrimSpace(userIntent); userIntent != "" {
		sb.WriteString("\n=== USER INTENT / NOTES ===\n")
		sb.WriteString(userIntent)
		sb.WriteString("\n")
	}

	sb.WriteString(fmt.Sprintf("\nOutput language: %s.\n", outputLang))
	sb.WriteString("Constraints: each `content` ≤ 280 characters, no hashtag spam, no @mentions of the original author.\n")
	sb.WriteString("Return ONLY the JSON object, no prose, no markdown fences.\n")
	return sb.String()
}

// GenerateReplyVariants calls the Vibe Write LLM in JSON mode and returns the bundle.
func GenerateReplyVariants(prompt string) (*ReplyVariantBundle, error) {
	messages := []ChatMessage{
		{Role: "system", Content: "You are an expert Twitter reply strategist. Output valid JSON only."},
		{Role: "user", Content: prompt},
	}
	var bundle ReplyVariantBundle
	if err := CallVibeWriteLLMJSON(messages, 2000, 0.7, &bundle); err != nil {
		return nil, err
	}
	if len(bundle.Variants) == 0 {
		return nil, fmt.Errorf("LLM returned zero variants")
	}
	// Ensure exactly one recommended (first one if none flagged)
	hasRec := false
	for i := range bundle.Variants {
		if bundle.Variants[i].Recommended {
			if hasRec {
				bundle.Variants[i].Recommended = false
			}
			hasRec = true
		}
	}
	if !hasRec {
		bundle.Variants[0].Recommended = true
	}
	return &bundle, nil
}

// TranslateText asks the LLM to produce a native-quality translation of `text`
// into `targetLang` (e.g. "English", "Chinese"). Returns the translated string.
func TranslateText(text, targetLang string) (string, error) {
	if strings.TrimSpace(text) == "" {
		return "", fmt.Errorf("empty text")
	}
	messages := []ChatMessage{
		{Role: "system", Content: "You translate tweets/short replies. Output ONLY the translated text — no quotes, no commentary, no language label. Preserve tone, brevity, and rhetorical structure. Do NOT translate proper nouns / @handles / $tickers / URLs."},
		{Role: "user", Content: fmt.Sprintf("Translate to %s. Keep it natural and idiomatic for native speakers, not literal:\n\n%s", targetLang, strings.TrimSpace(text))},
	}
	out, err := CallVibeWriteLLM(messages, 800, 0.4)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// LangNameForCode returns a human-readable language name for a 2-letter code.
func LangNameForCode(code string) string {
	switch strings.ToLower(strings.TrimSpace(code)) {
	case "zh", "zh-cn", "zh-hans":
		return "Chinese (Simplified)"
	case "zh-tw", "zh-hk":
		return "Chinese (Traditional)"
	case "en":
		return "English"
	case "ja":
		return "Japanese"
	case "ko":
		return "Korean"
	case "es":
		return "Spanish"
	case "fr":
		return "French"
	case "de":
		return "German"
	case "ru":
		return "Russian"
	case "pt":
		return "Portuguese"
	case "ar":
		return "Arabic"
	case "id":
		return "Indonesian"
	case "vi":
		return "Vietnamese"
	case "th":
		return "Thai"
	case "tr":
		return "Turkish"
	case "hi":
		return "Hindi"
	default:
		return code
	}
}
