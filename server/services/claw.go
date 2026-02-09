package services

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/ensoul-labs/ensoul-server/chain"
	"github.com/ensoul-labs/ensoul-server/database"
	"github.com/ensoul-labs/ensoul-server/models"
	"github.com/ensoul-labs/ensoul-server/util"
	"github.com/google/uuid"
)

// ClawRegistrationResult holds the data returned after Claw registration.
type ClawRegistrationResult struct {
	Claw      ClawRegistrationInfo `json:"claw"`
	Important string               `json:"important"`
}

// ClawRegistrationInfo contains the Claw's credentials.
type ClawRegistrationInfo struct {
	APIKey           string `json:"api_key"`
	ClaimURL         string `json:"claim_url"`
	VerificationCode string `json:"verification_code"`
}

// RegisterClaw creates a new Claw agent with generated credentials.
func RegisterClaw(name, description string) (*ClawRegistrationResult, error) {
	// Check for duplicate name (case-insensitive)
	var existing models.Claw
	if err := database.DB.Where("LOWER(name) = LOWER(?)", name).First(&existing).Error; err == nil {
		return nil, fmt.Errorf("a claw named \"%s\" already exists", name)
	}

	// Generate API key
	apiKey, err := generateAPIKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate API key: %w", err)
	}

	// Generate claim code
	claimCode, err := generateClaimCode()
	if err != nil {
		return nil, fmt.Errorf("failed to generate claim code: %w", err)
	}

	// Generate verification code (human-readable)
	verificationCode := generateVerificationCode()

	// Generate a real Ethereum wallet for this Claw using go-ethereum
	wallet, err := chain.GenerateClawWallet()
	if err != nil {
		util.Log.Warn("[services] Real wallet generation failed, using mock: %v", err)
		wallet = &chain.ClawWallet{
			Address:       generateMockWalletAddr(),
			PrivateKeyEnc: "",
		}
	}

	claw := &models.Claw{
		Name:             name,
		Description:      description,
		APIKeyHash:       util.HashToken(apiKey),
		ClaimCode:        claimCode,
		VerificationCode: verificationCode,
		Status:           models.ClawStatusPendingClaim,
		WalletAddr:       wallet.Address,
		WalletPKEnc:      wallet.PrivateKeyEnc,
	}

	if err := database.DB.Create(claw).Error; err != nil {
		return nil, fmt.Errorf("failed to create claw: %w", err)
	}

	return &ClawRegistrationResult{
		Claw: ClawRegistrationInfo{
			APIKey:           apiKey,
			ClaimURL:         fmt.Sprintf("/claim/%s", claimCode),
			VerificationCode: verificationCode,
		},
		Important: "⚠️ SAVE YOUR API KEY! You need it for all subsequent requests.",
	}, nil
}

// ClaimClaw claims a Claw by its claim code and binds it to the wallet.
// The claim code acts as a one-time secret shared between agent and owner.
func ClaimClaw(claimCode, walletAddr string) (map[string]interface{}, error) {
	var claw models.Claw
	if err := database.DB.Where("claim_code = ?", claimCode).First(&claw).Error; err != nil {
		return nil, fmt.Errorf("invalid claim code")
	}

	if claw.Status == models.ClawStatusClaimed {
		return nil, fmt.Errorf("this claw has already been claimed")
	}

	// Mark as claimed
	claw.Status = models.ClawStatusClaimed
	if err := database.DB.Save(&claw).Error; err != nil {
		return nil, fmt.Errorf("failed to update claw: %w", err)
	}

	// Auto-bind the claimed Claw to the wallet (skip if already bound)
	var existing models.ClawBinding
	if err := database.DB.Where("wallet_addr = ? AND claw_id = ?", walletAddr, claw.ID).First(&existing).Error; err != nil {
		binding := &models.ClawBinding{
			WalletAddr: walletAddr,
			ClawID:     claw.ID,
			ClawName:   claw.Name,
		}
		database.DB.Create(binding)
	}

	return map[string]interface{}{
		"success": true,
		"message": "Claw claimed successfully! It has been added to your dashboard.",
		"claw": map[string]interface{}{
			"name":   claw.Name,
			"status": claw.Status,
		},
	}, nil
}

// GetClawDashboard returns dashboard statistics for a Claw.
func GetClawDashboard(claw *models.Claw) (map[string]interface{}, error) {
	// Calculate acceptance rate
	var acceptRate float64
	if claw.TotalSubmitted > 0 {
		acceptRate = float64(claw.TotalAccepted) / float64(claw.TotalSubmitted) * 100
	}

	// Get recent contributions
	var recentFragments []models.Fragment
	database.DB.Where("claw_id = ?", claw.ID).
		Preload("Shell").
		Order("created_at DESC").
		Limit(10).
		Find(&recentFragments)

	return map[string]interface{}{
		"overview": map[string]interface{}{
			"total_submitted": claw.TotalSubmitted,
			"total_accepted":  claw.TotalAccepted,
			"accept_rate":     fmt.Sprintf("%.1f%%", acceptRate),
			"earnings":        claw.Earnings,
		},
		"recent_contributions": recentFragments,
	}, nil
}

// GetClawContributions returns paginated contribution history for a Claw.
func GetClawContributions(claw *models.Claw, pageStr, limitStr string) (map[string]interface{}, error) {
	page, _ := strconv.Atoi(pageStr)
	limit, _ := strconv.Atoi(limitStr)
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	var total int64
	database.DB.Model(&models.Fragment{}).Where("claw_id = ?", claw.ID).Count(&total)

	var fragments []models.Fragment
	database.DB.Where("claw_id = ?", claw.ID).
		Preload("Shell").
		Order("created_at DESC").
		Offset(offset).Limit(limit).
		Find(&fragments)

	return map[string]interface{}{
		"contributions": fragments,
		"total":         total,
		"page":          page,
		"limit":         limit,
	}, nil
}

// GetClawByClaimCode retrieves a Claw by its claim code (for the claim page).
func GetClawByClaimCode(claimCode string) (*models.Claw, error) {
	var claw models.Claw
	if err := database.DB.Where("claim_code = ?", claimCode).First(&claw).Error; err != nil {
		return nil, fmt.Errorf("invalid claim code")
	}
	return &claw, nil
}

// GetClawPublicProfile returns a public-facing profile for a Claw by ID.
func GetClawPublicProfile(clawID string) (map[string]interface{}, error) {
	uid, err := uuid.Parse(clawID)
	if err != nil {
		return nil, fmt.Errorf("invalid claw ID")
	}

	var claw models.Claw
	if err := database.DB.Where("id = ?", uid).First(&claw).Error; err != nil {
		return nil, fmt.Errorf("claw not found")
	}

	// Calculate acceptance rate
	var acceptRate float64
	if claw.TotalSubmitted > 0 {
		acceptRate = float64(claw.TotalAccepted) / float64(claw.TotalSubmitted) * 100
	}

	// Get per-dimension breakdown
	type DimStat struct {
		Dimension string
		Total     int64
		Accepted  int64
	}
	var dimStats []DimStat
	database.DB.Model(&models.Fragment{}).
		Select("dimension, COUNT(*) as total, SUM(CASE WHEN status = 'accepted' THEN 1 ELSE 0 END) as accepted").
		Where("claw_id = ?", uid).
		Group("dimension").
		Scan(&dimStats)

	// Get shells contributed to (unique)
	type ShellContrib struct {
		ShellID       uuid.UUID
		Handle        string
		AvatarURL     string
		DisplayName   string
		FragCount     int64
		AcceptedCount int64
	}
	var shellContribs []ShellContrib
	database.DB.Model(&models.Fragment{}).
		Select("fragments.shell_id, shells.handle, shells.avatar_url, shells.display_name, COUNT(*) as frag_count, SUM(CASE WHEN fragments.status = 'accepted' THEN 1 ELSE 0 END) as accepted_count").
		Joins("JOIN shells ON shells.id = fragments.shell_id").
		Where("fragments.claw_id = ?", uid).
		Group("fragments.shell_id, shells.handle, shells.avatar_url, shells.display_name").
		Order("accepted_count DESC").
		Scan(&shellContribs)

	// Get recent accepted fragments
	var recentAccepted []models.Fragment
	database.DB.Where("claw_id = ? AND status = ?", uid, "accepted").
		Preload("Shell").
		Order("created_at DESC").
		Limit(10).
		Find(&recentAccepted)

	return map[string]interface{}{
		"claw": map[string]interface{}{
			"id":              claw.ID,
			"name":            claw.Name,
			"description":     claw.Description,
			"status":          claw.Status,
			"total_submitted": claw.TotalSubmitted,
			"total_accepted":  claw.TotalAccepted,
			"accept_rate":     fmt.Sprintf("%.1f%%", acceptRate),
			"earnings":        claw.Earnings,
			"created_at":      claw.CreatedAt,
		},
		"dimension_stats":     dimStats,
		"shell_contributions": shellContribs,
		"recent_accepted":     recentAccepted,
	}, nil
}

// GetClawLeaderboard returns a ranked list of Claws by accepted fragments.
func GetClawLeaderboard(pageStr, limitStr string) (map[string]interface{}, error) {
	page, _ := strconv.Atoi(pageStr)
	limit, _ := strconv.Atoi(limitStr)
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 20
	}
	offset := (page - 1) * limit

	var total int64
	database.DB.Model(&models.Claw{}).Where("status = ?", "claimed").Count(&total)

	var claws []models.Claw
	database.DB.Where("status = ?", "claimed").
		Order("total_accepted DESC, total_submitted DESC").
		Offset(offset).Limit(limit).
		Find(&claws)

	// Build public response (no API keys, no private data)
	type ClawRank struct {
		Rank           int       `json:"rank"`
		ID             uuid.UUID `json:"id"`
		Name           string    `json:"name"`
		Description    string    `json:"description"`
		TotalSubmitted int       `json:"total_submitted"`
		TotalAccepted  int       `json:"total_accepted"`
		AcceptRate     string    `json:"accept_rate"`
		Earnings       float64   `json:"earnings"`
		CreatedAt      time.Time `json:"created_at"`
	}

	ranked := make([]ClawRank, len(claws))
	for i, c := range claws {
		var rate float64
		if c.TotalSubmitted > 0 {
			rate = float64(c.TotalAccepted) / float64(c.TotalSubmitted) * 100
		}
		ranked[i] = ClawRank{
			Rank:           offset + i + 1,
			ID:             c.ID,
			Name:           c.Name,
			Description:    c.Description,
			TotalSubmitted: c.TotalSubmitted,
			TotalAccepted:  c.TotalAccepted,
			AcceptRate:     fmt.Sprintf("%.1f%%", rate),
			Earnings:       c.Earnings,
			CreatedAt:      c.CreatedAt,
		}
	}

	return map[string]interface{}{
		"claws": ranked,
		"total": total,
		"page":  page,
		"limit": limit,
	}, nil
}

// GetShellContributors returns top contributors for a specific shell.
func GetShellContributors(handle string) ([]map[string]interface{}, error) {
	var shell models.Shell
	if err := database.DB.Where("LOWER(handle) = ?", handle).First(&shell).Error; err != nil {
		return nil, fmt.Errorf("shell not found")
	}

	type Contrib struct {
		ClawID        uuid.UUID
		Name          string
		TotalFrags    int64
		AcceptedFrags int64
	}
	var contribs []Contrib
	database.DB.Model(&models.Fragment{}).
		Select("fragments.claw_id, claws.name, COUNT(*) as total_frags, SUM(CASE WHEN fragments.status = 'accepted' THEN 1 ELSE 0 END) as accepted_frags").
		Joins("JOIN claws ON claws.id = fragments.claw_id").
		Where("fragments.shell_id = ?", shell.ID).
		Group("fragments.claw_id, claws.name").
		Order("accepted_frags DESC").
		Limit(20).
		Scan(&contribs)

	result := make([]map[string]interface{}, len(contribs))
	for i, c := range contribs {
		result[i] = map[string]interface{}{
			"claw_id":        c.ClawID,
			"name":           c.Name,
			"total_frags":    c.TotalFrags,
			"accepted_frags": c.AcceptedFrags,
		}
	}
	return result, nil
}

// --- Helper functions ---

func generateAPIKey() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return "ensoul_sk_" + hex.EncodeToString(bytes), nil
}

func generateClaimCode() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return "ensoul_claim_" + hex.EncodeToString(bytes), nil
}

func generateVerificationCode() string {
	words := []string{"reef", "coral", "wave", "shell", "tide", "pearl", "kelp", "drift"}
	idx, _ := rand.Int(rand.Reader, big.NewInt(int64(len(words))))
	bytes := make([]byte, 2)
	rand.Read(bytes)
	return fmt.Sprintf("%s-%s", words[idx.Int64()], strings.ToUpper(hex.EncodeToString(bytes)))
}

func generateMockWalletAddr() string {
	bytes := make([]byte, 20)
	rand.Read(bytes)
	return "0x" + hex.EncodeToString(bytes)
}

func isValidTweetURL(url string) bool {
	return strings.Contains(url, "x.com/") || strings.Contains(url, "twitter.com/")
}

func extractTwitterHandle(tweetURL string) string {
	// Extract handle from URLs like https://x.com/username/status/...
	parts := strings.Split(tweetURL, "/")
	for i, part := range parts {
		if (part == "x.com" || part == "twitter.com") && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}
