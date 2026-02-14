package services

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

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
	// Twitter handles are case-insensitive; normalize to lowercase
	// to prevent duplicate shells like "X" vs "x", "Grok" vs "grok".
	handle = strings.ToLower(handle)
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
	name = SanitizeHandle(name) // reuse: strips invisible chars + trims + lowercase
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
	TwitterMeta map[string]interface{}          `json:"twitter_meta,omitempty"`
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
			TwitterMeta: buildTwitterMeta(profile),
		}, nil
	}

	// Build the LLM prompt for seed extraction
	isMock := IsMockProfile(profile)

	var dataSection string
	if isMock {
		// No real Twitter data — ask LLM to use its own public knowledge
		dataSection = fmt.Sprintf(`=== DATA SOURCE ===
NOTE: Real-time Twitter data is not available for @%s.
Use your own knowledge about this public figure to create the seed profile.
Base your analysis on publicly known information: their career, public statements,
known personality traits, areas of expertise, notable positions, and public persona.
If you do not have sufficient knowledge about @%s, provide your best assessment
with lower scores and honest summaries indicating limited information.`, handle, handle)
	} else {
		tweetsText := FormatTweetsForLLM(profile.Tweets)

		// Build extended profile section — include SocialData fields when available
		var profileExtra string
		if profile.Location != "" {
			profileExtra += fmt.Sprintf("Location: %s\n", profile.Location)
		}
		if profile.CreatedAt != "" {
			profileExtra += fmt.Sprintf("Account Created: %s\n", profile.CreatedAt)
		}
		if profile.Verified {
			profileExtra += "Verified: Yes\n"
		}
		if profile.ListedCount > 0 {
			profileExtra += fmt.Sprintf("Listed Count: %d (reflects influence)\n", profile.ListedCount)
		}
		if profile.User.PublicMetrics.FollowingCount > 0 {
			profileExtra += fmt.Sprintf("Following: %d\n", profile.User.PublicMetrics.FollowingCount)
		}
		if profile.FavouritesCount > 0 {
			profileExtra += fmt.Sprintf("Favourites: %d\n", profile.FavouritesCount)
		}
		profileExtra += fmt.Sprintf("Data Source: %s\n", profile.DataSource)

		dataSection = fmt.Sprintf(`=== PROFILE ===
Handle: @%s
Display Name: %s
Bio: %s
Followers: %d
%s
=== RECENT TWEETS ===
%s`, handle, profile.User.Name, profile.User.Description,
			profile.User.PublicMetrics.FollowersCount, profileExtra, tweetsText)
	}

	seedPrompt := fmt.Sprintf(`You are the seed extraction engine for Ensoul, a decentralized soul construction protocol.

Analyze the following information to create an initial personality profile.

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
}`, dataSection)

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
			TwitterMeta: buildTwitterMeta(profile),
		}, nil
	}

	util.Log.Debug("[seed] Seed extraction for @%s complete via LLM", handle)

	return &SeedPreview{
		Handle:      handle,
		DisplayName: profile.User.Name,
		AvatarURL:   normalizeAvatarURL(profile.User.ProfileImageURL, handle),
		SeedSummary: result.SeedSummary,
		Dimensions:  result.Dimensions,
		TwitterMeta: buildTwitterMeta(profile),
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

// buildTwitterMeta extracts display-friendly metadata from a TwitterProfile
// for storage in the Shell's twitter_meta JSONB column.
func buildTwitterMeta(profile *TwitterProfile) map[string]interface{} {
	meta := map[string]interface{}{
		"followers_count": profile.User.PublicMetrics.FollowersCount,
		"following_count": profile.User.PublicMetrics.FollowingCount,
		"tweet_count":     profile.User.PublicMetrics.TweetCount,
		"data_source":     profile.DataSource,
		"bio":             profile.User.Description,
	}
	if profile.Location != "" {
		meta["location"] = profile.Location
	}
	if profile.Verified {
		meta["verified"] = true
	}
	if profile.CreatedAt != "" {
		meta["account_created_at"] = profile.CreatedAt
	}
	if profile.BannerURL != "" {
		meta["banner_url"] = profile.BannerURL
	}
	if profile.ListedCount > 0 {
		meta["listed_count"] = profile.ListedCount
	}
	if profile.FavouritesCount > 0 {
		meta["favourites_count"] = profile.FavouritesCount
	}
	return meta
}

// PendingMintTimeout is how long a pending mint reservation lasts before cleanup.
const PendingMintTimeout = 30 * time.Minute

// MintShell creates a new shell in the database with stage=pending.
// The shell is only fully activated after ConfirmMint is called with a tx_hash.
// If the same wallet retries the same handle (e.g. after a failed signing),
// the old pending record is replaced.
func MintShell(handle, ownerAddr string, preview *SeedPreview) (*models.Shell, error) {
	// Check for existing shell
	var existing models.Shell
	if err := database.DB.Where("LOWER(handle) = ?", handle).First(&existing).Error; err == nil {
		if existing.Stage == models.StagePending {
			// Same wallet retrying → cascade-delete old pending and re-create
			if strings.EqualFold(existing.OwnerAddr, ownerAddr) {
				HardDeleteShell(existing.ID)
				util.Log.Info("[services] Replaced pending shell @%s for same wallet %s", handle, ownerAddr)
			} else {
				// Different wallet has a pending reservation
				if time.Since(existing.CreatedAt) > PendingMintTimeout {
					// Expired pending → cascade-delete and allow
					HardDeleteShell(existing.ID)
					util.Log.Info("[services] Cleared expired pending shell @%s (was %s)", handle, existing.OwnerAddr)
				} else {
					return nil, fmt.Errorf("@%s is being minted by another user, please try again later", handle)
				}
			}
		} else {
			return nil, fmt.Errorf("a soul for @%s already exists", handle)
		}
	}

	// Limit: each wallet can mint at most 5 confirmed shells (exclude pending)
	var mintCount int64
	database.DB.Model(&models.Shell{}).Where("owner_addr = ? AND stage != ?", ownerAddr, models.StagePending).Count(&mintCount)
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

	// Build twitter_meta JSON
	twitterMeta := models.JSON{}
	for k, v := range preview.TwitterMeta {
		twitterMeta[k] = v
	}

	// Create shell record (pending until on-chain confirmation)
	shell := &models.Shell{
		Handle:      handle,
		OwnerAddr:   ownerAddr,
		Stage:       models.StagePending,
		DNAVersion:  1,
		SeedSummary: preview.SeedSummary,
		SoulPrompt:  buildInitialSoulPrompt(handle, preview.SeedSummary),
		Dimensions:  dims,
		AvatarURL:   preview.AvatarURL,
		DisplayName: preview.DisplayName,
		TwitterMeta: twitterMeta,
	}

	if err := database.DB.Create(shell).Error; err != nil {
		return nil, fmt.Errorf("failed to create shell: %w", err)
	}

	util.Log.Info("[services] Shell @%s created in DB (owner: %s)", handle, ownerAddr)

	return shell, nil
}

// ConfirmMint updates a shell record with on-chain data after the user mints.
// Transitions the shell from pending → embryo.
// Only the original minter wallet can confirm, and only pending shells can be confirmed.
func ConfirmMint(handle, txHash string, agentID uint64, walletAddr string) error {
	// Atomic update: only succeeds if stage is still pending AND wallet matches
	result := database.DB.Model(&models.Shell{}).
		Where("LOWER(handle) = ? AND stage = ? AND LOWER(owner_addr) = LOWER(?)", handle, models.StagePending, walletAddr).
		Updates(map[string]interface{}{
			"agent_id":     &agentID,
			"mint_tx_hash": txHash,
			"stage":        models.StageEmbryo,
		})
	if result.Error != nil {
		return fmt.Errorf("failed to update shell: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		// Check why: not found, wrong stage, or wrong wallet?
		var shell models.Shell
		if err := database.DB.Where("LOWER(handle) = ?", handle).First(&shell).Error; err != nil {
			return fmt.Errorf("shell @%s not found", handle)
		}
		if shell.Stage != models.StagePending {
			return fmt.Errorf("shell @%s is not in pending state (stage=%s)", handle, shell.Stage)
		}
		return fmt.Errorf("wallet mismatch: only the original minter can confirm")
	}
	util.Log.Info("[services] Shell @%s confirmed on-chain: agentId=%d, tx=%s", handle, agentID, txHash)
	return nil
}

// CancelPendingMint removes a pending shell record when the on-chain mint fails.
// Only the same wallet that created the pending record can cancel it.
// Uses atomic SELECT + stage check to prevent TOCTOU race with ConfirmMint.
func CancelPendingMint(handle, walletAddr string) error {
	var shell models.Shell
	// Atomic query: only find shells that are still pending AND owned by this wallet
	if err := database.DB.Where("LOWER(handle) = ? AND stage = ? AND LOWER(owner_addr) = LOWER(?)",
		handle, models.StagePending, walletAddr).First(&shell).Error; err != nil {
		// Provide a specific error message based on what went wrong
		var any models.Shell
		if errAny := database.DB.Where("LOWER(handle) = ?", handle).First(&any).Error; errAny != nil {
			return fmt.Errorf("shell @%s not found", handle)
		}
		if any.Stage != models.StagePending {
			return fmt.Errorf("shell @%s is no longer pending (stage=%s), cannot cancel", handle, any.Stage)
		}
		return fmt.Errorf("only the original minter can cancel this pending shell")
	}

	HardDeleteShell(shell.ID)
	util.Log.Info("[services] Pending shell @%s cancelled by owner %s (chain mint failed)", handle, walletAddr)
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

	// Always exclude unconfirmed shells (pending or no tx_hash) from listings
	query = query.Where("stage != ? AND mint_tx_hash != ''", models.StagePending)

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

	// Strip soul_prompt from public listings — it's the core paid asset
	for i := range shells {
		shells[i].SoulPrompt = ""
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
	if err := database.DB.Where("LOWER(handle) = ?", handle).First(&shell).Error; err != nil {
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

	// Strip new_prompt from history — it's the core paid asset
	for i := range history {
		history[i].NewPrompt = ""
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
	// Never update the stage of a pending shell via this function;
	// pending → embryo transition is handled exclusively by ConfirmMint.
	if shell.Stage == models.StagePending {
		return
	}

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
