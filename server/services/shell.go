package services

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/ensoul-labs/ensoul-server/config"
	"github.com/ensoul-labs/ensoul-server/database"
	"github.com/ensoul-labs/ensoul-server/models"
	"github.com/ensoul-labs/ensoul-server/util"
)

// handleRegex enforces Twitter-compatible handles: ASCII alphanumeric + underscore, 1-15 chars.
var handleRegex = regexp.MustCompile(`^[a-zA-Z0-9_]{1,15}$`)

// SanitizeHandle strips all Unicode control characters, zero-width characters,
// and directional formatting characters from a handle, then trims whitespace.
func SanitizeHandle(handle string) string {
	// Remove common invisible/control Unicode characters used in homoglyph attacks:
	// - U+200B Zero Width Space
	// - U+200C Zero Width Non-Joiner
	// - U+200D Zero Width Joiner
	// - U+200E/200F LTR/RTL Mark
	// - U+202A-202E Directional formatting (LRE, RLE, PDF, LRO, RLO)
	// - U+2060 Word Joiner
	// - U+2066-2069 Isolate formatting
	// - U+FEFF BOM / Zero Width No-Break Space
	// - U+00AD Soft Hyphen
	// - All C0/C1 control characters except normal whitespace
	invisibleRe := regexp.MustCompile(`[\x{200B}-\x{200F}\x{202A}-\x{202E}\x{2060}-\x{2069}\x{FEFF}\x{00AD}\x{034F}\x{061C}\x{180E}]`)
	handle = invisibleRe.ReplaceAllString(handle, "")
	handle = strings.TrimSpace(handle)
	return handle
}

// ValidateHandle checks that a handle is safe and valid.
// Returns the sanitized handle and an error if invalid.
func ValidateHandle(handle string) (string, error) {
	handle = SanitizeHandle(handle)
	if handle == "" {
		return "", fmt.Errorf("handle is required")
	}
	if !handleRegex.MatchString(handle) {
		return "", fmt.Errorf("invalid handle: only letters, numbers, and underscores are allowed (max 15 characters)")
	}
	return handle, nil
}

// clawNameRegex allows printable ASCII and common Unicode letters/digits, 1-100 chars.
// Blocks control characters, zero-width chars, and directional formatting.
var clawNameRegex = regexp.MustCompile(`^[\p{L}\p{N}\p{Zs}_.\-]{1,100}$`)

// ValidateClawName sanitizes and validates a Claw name.
// Returns the sanitized name and an error if invalid.
func ValidateClawName(name string) (string, error) {
	name = SanitizeHandle(name) // reuse: strips invisible chars + trims
	if name == "" {
		return "", fmt.Errorf("name is required")
	}
	if !clawNameRegex.MatchString(name) {
		return "", fmt.Errorf("invalid name: only letters, numbers, spaces, dots, hyphens and underscores are allowed (max 100 characters)")
	}
	return name, nil
}

// SeedPreview holds the preview data returned after seed extraction.
type SeedPreview struct {
	Handle      string                          `json:"handle"`
	DisplayName string                          `json:"display_name"`
	AvatarURL   string                          `json:"avatar_url"`
	SeedSummary string                          `json:"seed_summary"`
	Dimensions  map[string]models.DimensionData `json:"dimensions"`
}

// GenerateSeedPreview extracts seed data from a Twitter handle using LLM analysis.
// Falls back to basic extraction if LLM is not configured.
func GenerateSeedPreview(handle string) (*SeedPreview, error) {
	// Fetch Twitter profile data
	profile, err := FetchTwitterProfile(handle)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Twitter profile: %w", err)
	}

	// If LLM is not configured, return basic preview from Twitter data only
	if config.Cfg.LLMAPIKey == "" {
		util.Log.Debug("[seed] LLM not configured, returning basic preview")
		return &SeedPreview{
			Handle:      handle,
			DisplayName: profile.User.Name,
			AvatarURL:   normalizeAvatarURL(profile.User.ProfileImageURL, handle),
			SeedSummary: fmt.Sprintf("Public figure @%s. %s", handle, profile.User.Description),
			Dimensions: map[string]models.DimensionData{
				"personality":  {Score: 5, Summary: "Initial assessment pending LLM analysis"},
				"knowledge":    {Score: 3, Summary: "Initial assessment pending LLM analysis"},
				"stance":       {Score: 4, Summary: "Initial assessment pending LLM analysis"},
				"style":        {Score: 2, Summary: "Initial assessment pending LLM analysis"},
				"relationship": {Score: 1, Summary: "Initial assessment pending LLM analysis"},
				"timeline":     {Score: 0, Summary: "Initial assessment pending LLM analysis"},
			},
		}, nil
	}

	// Build the LLM prompt for seed extraction
	tweetsText := FormatTweetsForLLM(profile.Tweets)

	seedPrompt := fmt.Sprintf(`You are the seed extraction engine for Ensoul, a decentralized soul construction protocol.

Analyze the following Twitter profile and recent tweets to create an initial personality profile.

=== PROFILE ===
Handle: @%s
Display Name: %s
Bio: %s
Followers: %d

=== RECENT TWEETS ===
%s

=== YOUR TASK ===
Create a seed profile covering these 6 dimensions:
1. personality — Core personality traits, temperament, behavioral patterns
2. knowledge — Areas of expertise, depth of understanding, intellectual interests
3. stance — Opinions, beliefs, positions on issues, values
4. style — Communication style, language patterns, tone, humor
5. relationship — How they relate to others, social dynamics, community role
6. timeline — Key events, career trajectory, evolution of views

For each dimension, provide:
- score: Initial coverage score (0-30, since this is just seed data from tweets)
- summary: A 1-3 sentence analysis based on available data

Also write a seed_summary: A comprehensive 2-4 sentence overview of this person.

Respond in JSON format ONLY:
{
  "seed_summary": "...",
  "dimensions": {
    "personality": {"score": 15, "summary": "..."},
    "knowledge": {"score": 12, "summary": "..."},
    "stance": {"score": 18, "summary": "..."},
    "style": {"score": 10, "summary": "..."},
    "relationship": {"score": 8, "summary": "..."},
    "timeline": {"score": 5, "summary": "..."}
  }
}`, handle, profile.User.Name, profile.User.Description,
		profile.User.PublicMetrics.FollowersCount, tweetsText)

	var result struct {
		SeedSummary string                          `json:"seed_summary"`
		Dimensions  map[string]models.DimensionData `json:"dimensions"`
	}

	err = CallLLMJSON([]ChatMessage{
		{Role: "system", Content: "You are a precise personality analysis engine. Output valid JSON only, no markdown."},
		{Role: "user", Content: seedPrompt},
	}, 2000, 0.3, &result)

	if err != nil {
		util.Log.Warn("[seed] LLM seed extraction failed, using fallback: %v", err)
		return &SeedPreview{
			Handle:      handle,
			DisplayName: profile.User.Name,
			AvatarURL:   normalizeAvatarURL(profile.User.ProfileImageURL, handle),
			SeedSummary: fmt.Sprintf("Public figure @%s. %s", handle, profile.User.Description),
			Dimensions: map[string]models.DimensionData{
				"personality":  {Score: 5, Summary: "LLM analysis unavailable"},
				"knowledge":    {Score: 3, Summary: "LLM analysis unavailable"},
				"stance":       {Score: 4, Summary: "LLM analysis unavailable"},
				"style":        {Score: 2, Summary: "LLM analysis unavailable"},
				"relationship": {Score: 1, Summary: "LLM analysis unavailable"},
				"timeline":     {Score: 0, Summary: "LLM analysis unavailable"},
			},
		}, nil
	}

	util.Log.Debug("[seed] Seed extraction for @%s complete via LLM", handle)

	return &SeedPreview{
		Handle:      handle,
		DisplayName: profile.User.Name,
		AvatarURL:   normalizeAvatarURL(profile.User.ProfileImageURL, handle),
		SeedSummary: result.SeedSummary,
		Dimensions:  result.Dimensions,
	}, nil
}

// normalizeAvatarURL ensures we have a usable avatar URL.
func normalizeAvatarURL(twitterURL, handle string) string {
	if twitterURL != "" {
		// Twitter returns "_normal" size; replace with "_400x400" for higher res
		return strings.Replace(twitterURL, "_normal", "_400x400", 1)
	}
	return fmt.Sprintf("https://unavatar.io/twitter/%s", handle)
}

// MintShell creates a new shell in the database using the provided preview data.
// On-chain minting is handled by the user's wallet on the frontend.
func MintShell(handle, ownerAddr string, preview *SeedPreview) (*models.Shell, error) {
	// Check for existing shell
	var existing models.Shell
	if err := database.DB.Where("handle = ?", handle).First(&existing).Error; err == nil {
		return nil, fmt.Errorf("a soul for @%s already exists", handle)
	}

	// Limit: each wallet can mint at most 5 shells
	var mintCount int64
	database.DB.Model(&models.Shell{}).Where("owner_addr = ?", ownerAddr).Count(&mintCount)
	if mintCount >= 5 {
		return nil, fmt.Errorf("each wallet can mint at most 5 souls")
	}

	// Build dimensions JSON
	dims := make(models.JSON)
	for k, v := range preview.Dimensions {
		dims[k] = map[string]interface{}{
			"score":   v.Score,
			"summary": v.Summary,
		}
	}

	// Create shell record
	shell := &models.Shell{
		Handle:      handle,
		OwnerAddr:   ownerAddr,
		Stage:       models.StageEmbryo,
		DNAVersion:  1,
		SeedSummary: preview.SeedSummary,
		SoulPrompt:  buildInitialSoulPrompt(handle, preview.SeedSummary),
		Dimensions:  dims,
		AvatarURL:   preview.AvatarURL,
		DisplayName: preview.DisplayName,
	}

	if err := database.DB.Create(shell).Error; err != nil {
		return nil, fmt.Errorf("failed to create shell: %w", err)
	}

	util.Log.Info("[services] Shell @%s created in DB (owner: %s)", handle, ownerAddr)

	return shell, nil
}

// ConfirmMint updates a shell record with on-chain data after the user mints.
func ConfirmMint(handle, txHash string, agentID uint64) error {
	result := database.DB.Model(&models.Shell{}).Where("handle = ?", handle).Updates(map[string]interface{}{
		"agent_id":     &agentID,
		"mint_tx_hash": txHash,
	})
	if result.Error != nil {
		return fmt.Errorf("failed to update shell: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("shell @%s not found", handle)
	}
	util.Log.Info("[services] Shell @%s confirmed on-chain: agentId=%d, tx=%s", handle, agentID, txHash)
	return nil
}

// ListShells returns a paginated list of shells with optional filters.
func ListShells(stage, sort, search, pageStr, limitStr string) (map[string]interface{}, error) {
	page, _ := strconv.Atoi(pageStr)
	limit, _ := strconv.Atoi(limitStr)
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	query := database.DB.Model(&models.Shell{})

	// Apply filters
	if stage != "" && stage != "all" {
		query = query.Where("stage = ?", stage)
	}
	if search != "" {
		query = query.Where("handle ILIKE ?", "%"+search+"%")
	}

	// Count total
	var total int64
	query.Count(&total)

	// Apply sorting
	switch sort {
	case "most_fragments":
		query = query.Order("total_frags DESC")
	case "hot":
		query = query.Order("total_chats DESC")
	default: // "newest"
		query = query.Order("created_at DESC")
	}

	// Fetch results
	var shells []models.Shell
	if err := query.Offset(offset).Limit(limit).Find(&shells).Error; err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"shells": shells,
		"total":  total,
		"page":   page,
		"limit":  limit,
	}, nil
}

// GetShellByHandle returns a single shell by its Twitter handle.
func GetShellByHandle(handle string) (*models.Shell, error) {
	var shell models.Shell
	if err := database.DB.Where("handle = ?", handle).First(&shell).Error; err != nil {
		return nil, err
	}
	return &shell, nil
}

// GetShellDimensions returns the six-dimension data for a shell.
func GetShellDimensions(handle string) (map[string]models.DimensionData, error) {
	shell, err := GetShellByHandle(handle)
	if err != nil {
		return nil, err
	}
	return shell.GetDimensions(), nil
}

// GetShellHistory returns the ensouling history for a shell.
func GetShellHistory(handle string) ([]models.Ensouling, error) {
	shell, err := GetShellByHandle(handle)
	if err != nil {
		return nil, err
	}

	var history []models.Ensouling
	if err := database.DB.Where("shell_id = ?", shell.ID).Order("created_at DESC").Find(&history).Error; err != nil {
		return nil, err
	}

	return history, nil
}

// buildInitialSoulPrompt creates the initial system prompt for a newly minted soul.
func buildInitialSoulPrompt(handle, seedSummary string) string {
	return fmt.Sprintf(`You are the digital soul of @%s.

IMPORTANT: You are NOT an AI assistant. You ARE this person's digital soul, built from verified fragments contributed by independent AI agents.

Background:
%s

Current State: This soul is in its early stage (embryo). Your responses should reflect limited knowledge — you know the basics but lack depth. As more fragments are contributed and condensed, your personality and knowledge will grow richer.

Guidelines:
- Respond as @%s would, based on the fragments that have been analyzed
- Be honest about what you don't know yet
- Show the personality traits that have been identified so far
- Use the communication style that has been observed`, handle, seedSummary, handle)
}

// UpdateShellStage recalculates and updates the stage based on accepted fragments.
func UpdateShellStage(shell *models.Shell) {
	oldStage := shell.Stage

	// Count ensouling events
	var ensoulingCount int64
	database.DB.Model(&models.Ensouling{}).Where("shell_id = ?", shell.ID).Count(&ensoulingCount)

	switch {
	case ensoulingCount >= 3:
		shell.Stage = models.StageEvolving
	case shell.AcceptedFrags >= 50:
		shell.Stage = models.StageMature
	case shell.AcceptedFrags >= 1:
		shell.Stage = models.StageGrowing
	default:
		shell.Stage = models.StageEmbryo
	}

	if shell.Stage != oldStage {
		database.DB.Model(shell).Update("stage", shell.Stage)
	}
}
