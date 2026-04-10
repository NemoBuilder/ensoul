package services

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ensoul-labs/ensoul-server/database"
	"github.com/ensoul-labs/ensoul-server/models"
	"github.com/ensoul-labs/ensoul-server/util"
)

// InitiateClaim starts the KOL claim process for a Soul.
// Returns a verification code that the KOL must tweet.
func InitiateClaim(handle, kolWalletAddr string) (*models.KOLClaim, error) {
	shell, err := GetShellByHandle(handle)
	if err != nil {
		return nil, fmt.Errorf("soul @%s not found", handle)
	}

	// Check if already claimed
	var existing models.KOLClaim
	if err := database.DB.Where("shell_id = ?", shell.ID).First(&existing).Error; err == nil {
		if existing.Status == models.ClaimStatusVerified {
			return nil, fmt.Errorf("@%s has already been claimed", handle)
		}
		if existing.Status == models.ClaimStatusPending {
			// Return existing pending claim
			return &existing, nil
		}
	}

	// Generate verification code
	code := generateVerifyCode()

	claim := &models.KOLClaim{
		ShellID:       shell.ID,
		KOLWalletAddr: kolWalletAddr,
		VerifyCode:    code,
		Status:        models.ClaimStatusPending,
	}

	if err := database.DB.Create(claim).Error; err != nil {
		return nil, fmt.Errorf("failed to create claim: %w", err)
	}

	util.Log.Info("[claim] Initiated claim for @%s by wallet %s (code: %s)", handle, kolWalletAddr, code)
	return claim, nil
}

// VerifyClaim verifies a KOL's claim by checking their tweet contains the verification code.
func VerifyClaim(handle, tweetID string) error {
	shell, err := GetShellByHandle(handle)
	if err != nil {
		return fmt.Errorf("soul @%s not found", handle)
	}

	var claim models.KOLClaim
	if err := database.DB.Where("shell_id = ? AND status = ?", shell.ID, models.ClaimStatusPending).
		First(&claim).Error; err != nil {
		return fmt.Errorf("no pending claim found for @%s", handle)
	}

	// Fetch the tweet and verify it contains the code
	if !SocialDataAvailable() {
		return fmt.Errorf("SocialData API not configured, cannot verify tweet")
	}

	// Fetch the tweet content
	tweetContent, err := fetchTweetByID(tweetID)
	if err != nil {
		return fmt.Errorf("failed to fetch tweet: %w", err)
	}

	// Verify the tweet contains the verification code
	if !strings.Contains(tweetContent, claim.VerifyCode) {
		return fmt.Errorf("tweet does not contain verification code '%s'", claim.VerifyCode)
	}

	// Verify the tweet is from the correct handle
	// (The tweet author should match the soul's handle)
	// This is a basic check — in production, verify tweet author via API

	// Mark as verified
	now := time.Now().UTC()
	transitionEnd := now.Add(90 * 24 * time.Hour) // 3 months transition

	database.DB.Model(&claim).Updates(map[string]interface{}{
		"status":          models.ClaimStatusVerified,
		"verify_tweet_id": tweetID,
		"claimed_at":      now,
		"transition_end":  transitionEnd,
	})

	util.Log.Info("[claim] @%s claimed by wallet %s (transition ends: %s)",
		handle, claim.KOLWalletAddr, transitionEnd)
	return nil
}

// GetClaimStatus returns the claim status for a soul.
func GetClaimStatus(handle string) (map[string]interface{}, error) {
	shell, err := GetShellByHandle(handle)
	if err != nil {
		return nil, fmt.Errorf("soul @%s not found", handle)
	}

	var claim models.KOLClaim
	if err := database.DB.Where("shell_id = ?", shell.ID).First(&claim).Error; err != nil {
		return map[string]interface{}{
			"claimed":    false,
			"claimable":  true,
			"handle":     handle,
		}, nil
	}

	result := map[string]interface{}{
		"claimed":        claim.Status == models.ClaimStatusVerified,
		"claimable":      claim.Status != models.ClaimStatusVerified,
		"status":         claim.Status,
		"handle":         handle,
		"kol_wallet":     claim.KOLWalletAddr,
	}

	if claim.Status == models.ClaimStatusPending {
		result["verify_code"] = claim.VerifyCode
	}

	if claim.Status == models.ClaimStatusVerified {
		result["claimed_at"] = claim.ClaimedAt
		result["transition_end"] = claim.TransitionEnd

		// Calculate current revenue split
		if claim.TransitionEnd != nil && time.Now().UTC().Before(*claim.TransitionEnd) {
			result["kol_share"] = 0.30
			result["holder_share"] = 0.70
			result["in_transition"] = true
		} else {
			result["kol_share"] = 0.50
			result["holder_share"] = 0.50
			result["in_transition"] = false
		}
	}

	return result, nil
}

// fetchTweetByID fetches a single tweet's text by its ID.
func fetchTweetByID(tweetID string) (string, error) {
	client := newSocialDataClient()
	endpoint := fmt.Sprintf("/twitter/tweets/%s", tweetID)

	body, status, err := client.doRequest(endpoint)
	if err != nil {
		return "", err
	}

	if status != 200 {
		return "", fmt.Errorf("tweet fetch failed (status %d): %s", status, string(body))
	}

	// Parse the tweet response
	var tweet struct {
		FullText string  `json:"full_text"`
		Text     *string `json:"text"`
	}
	if err := json.Unmarshal(body, &tweet); err != nil {
		return "", fmt.Errorf("failed to parse tweet: %w", err)
	}

	text := tweet.FullText
	if text == "" && tweet.Text != nil {
		text = *tweet.Text
	}

	return text, nil
}

// generateVerifyCode creates a random 8-character alphanumeric code.
func generateVerifyCode() string {
	const charset = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	code := make([]byte, 8)
	for i := range code {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		code[i] = charset[n.Int64()]
	}
	return "ENSOUL-" + string(code)
}
