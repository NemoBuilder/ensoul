package services

import (
	"fmt"
	"time"

	"github.com/ensoul-labs/ensoul-server/database"
	"github.com/ensoul-labs/ensoul-server/models"
	"github.com/ensoul-labs/ensoul-server/util"
	"github.com/google/uuid"
)

// ──────────────────────────────────────────────────────────────────────────────
// Tag Service — Sniper 2.0 tag-based content system
// ──────────────────────────────────────────────────────────────────────────────

// TagWithAccounts is the response type for GET /api/sniper/tags.
type TagWithAccounts struct {
	models.SniperTag
	Accounts []TagAccountInfo `json:"accounts"`
}

// TagAccountInfo is the public account info within a tag.
type TagAccountInfo struct {
	Handle           string `json:"handle"`
	DisplayName      string `json:"name"`
	RealtimePriority bool   `json:"realtime_priority"`
}

// GetAllTags returns all active tags with their associated accounts.
func GetAllTags() ([]TagWithAccounts, []string, error) {
	var tags []models.SniperTag
	if err := database.DB.Where("active = ?", true).
		Order("sort_order ASC, created_at ASC").
		Find(&tags).Error; err != nil {
		return nil, nil, fmt.Errorf("failed to fetch tags: %w", err)
	}

	result := make([]TagWithAccounts, 0, len(tags))
	var defaults []string

	for _, tag := range tags {
		// Fetch accounts for this tag
		var accounts []models.SniperTagAccount
		database.DB.Where("tag_id = ?", tag.ID).
			Order("sort_order ASC, created_at ASC").
			Find(&accounts)

		acctInfos := make([]TagAccountInfo, 0, len(accounts))
		for _, a := range accounts {
			acctInfos = append(acctInfos, TagAccountInfo{
				Handle:           a.Handle,
				DisplayName:      a.DisplayName,
				RealtimePriority: a.RealtimePriority,
			})
		}

		result = append(result, TagWithAccounts{
			SniperTag: tag,
			Accounts:  acctInfos,
		})

		if tag.IsDefault {
			defaults = append(defaults, tag.ID)
		}
	}

	return result, defaults, nil
}

// GetHandlesForTags returns all unique handles associated with the given tag IDs.
func GetHandlesForTags(tagIDs []string) ([]string, error) {
	var accounts []models.SniperTagAccount
	if err := database.DB.Where("tag_id IN ?", tagIDs).Find(&accounts).Error; err != nil {
		return nil, err
	}

	// Deduplicate handles
	seen := make(map[string]bool)
	var handles []string
	for _, a := range accounts {
		if !seen[a.Handle] {
			seen[a.Handle] = true
			handles = append(handles, a.Handle)
		}
	}
	return handles, nil
}

// GetHandleToTagsMap returns a mapping of handle → []tagID for the given tags.
func GetHandleToTagsMap(tagIDs []string) (map[string][]string, error) {
	var accounts []models.SniperTagAccount
	if err := database.DB.Where("tag_id IN ?", tagIDs).Find(&accounts).Error; err != nil {
		return nil, err
	}

	result := make(map[string][]string)
	for _, a := range accounts {
		result[a.Handle] = append(result[a.Handle], a.TagID)
	}
	return result, nil
}

// ──────────────────────────────────────────────────────────────────────────────
// User Tag Preferences
// ──────────────────────────────────────────────────────────────────────────────

// GetUserSelectedTags returns the tag IDs a user has selected.
// If the user has no selections, returns the default tags.
func GetUserSelectedTags(walletAddr string) ([]string, error) {
	var selections []models.UserSelectedTag
	if err := database.DB.Where("wallet_addr = ?", walletAddr).Find(&selections).Error; err != nil {
		return nil, err
	}

	if len(selections) == 0 {
		// Return defaults
		return getDefaultTagIDs(), nil
	}

	tagIDs := make([]string, 0, len(selections))
	for _, s := range selections {
		tagIDs = append(tagIDs, s.TagID)
	}
	return tagIDs, nil
}

// UpdateUserSelectedTags replaces a user's tag selections.
func UpdateUserSelectedTags(walletAddr string, tagIDs []string) error {
	// Validate all tag IDs exist
	var count int64
	database.DB.Model(&models.SniperTag{}).Where("id IN ? AND active = ?", tagIDs, true).Count(&count)
	if int(count) != len(tagIDs) {
		return fmt.Errorf("one or more invalid tag IDs")
	}

	// Transaction: delete old + insert new
	tx := database.DB.Begin()

	if err := tx.Where("wallet_addr = ?", walletAddr).Delete(&models.UserSelectedTag{}).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to clear old selections: %w", err)
	}

	for _, tagID := range tagIDs {
		sel := models.UserSelectedTag{
			WalletAddr: walletAddr,
			TagID:      tagID,
		}
		if err := tx.Create(&sel).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to save tag selection: %w", err)
		}
	}

	tx.Commit()
	util.Log.Info("[sniper] Updated tag selections for %s: %v", walletAddr, tagIDs)
	return nil
}

// getDefaultTagIDs returns IDs of tags marked as default.
func getDefaultTagIDs() []string {
	var tags []models.SniperTag
	database.DB.Where("is_default = ? AND active = ?", true, true).Find(&tags)

	ids := make([]string, 0, len(tags))
	for _, t := range tags {
		ids = append(ids, t.ID)
	}
	return ids
}

// ──────────────────────────────────────────────────────────────────────────────
// User Muted Accounts
// ──────────────────────────────────────────────────────────────────────────────

// GetUserMutedHandles returns all handles the user has muted.
func GetUserMutedHandles(walletAddr string) ([]string, error) {
	var muted []models.UserMutedAccount
	if err := database.DB.Where("wallet_addr = ?", walletAddr).Find(&muted).Error; err != nil {
		return nil, err
	}

	handles := make([]string, 0, len(muted))
	for _, m := range muted {
		handles = append(handles, m.Handle)
	}
	return handles, nil
}

// MuteAccount adds a handle to the user's mute list.
func MuteAccount(walletAddr, handle string) error {
	muted := models.UserMutedAccount{
		WalletAddr: walletAddr,
		Handle:     handle,
	}
	if err := database.DB.Create(&muted).Error; err != nil {
		// Likely duplicate
		return fmt.Errorf("@%s is already muted", handle)
	}
	util.Log.Info("[sniper] %s muted @%s", walletAddr, handle)
	return nil
}

// UnmuteAccount removes a handle from the user's mute list.
func UnmuteAccount(walletAddr, handle string) error {
	result := database.DB.Where("wallet_addr = ? AND handle = ?", walletAddr, handle).
		Delete(&models.UserMutedAccount{})
	if result.RowsAffected == 0 {
		return fmt.Errorf("@%s is not in your mute list", handle)
	}
	util.Log.Info("[sniper] %s unmuted @%s", walletAddr, handle)
	return nil
}

// ──────────────────────────────────────────────────────────────────────────────
// Admin: Tag CRUD
// ──────────────────────────────────────────────────────────────────────────────

// AdminCreateTag creates a new tag.
func AdminCreateTag(tag *models.SniperTag) error {
	if tag.ID == "" {
		return fmt.Errorf("tag ID is required")
	}
	return database.DB.Create(tag).Error
}

// AdminUpdateTag updates an existing tag.
func AdminUpdateTag(tagID string, updates map[string]interface{}) error {
	result := database.DB.Model(&models.SniperTag{}).Where("id = ?", tagID).Updates(updates)
	if result.RowsAffected == 0 {
		return fmt.Errorf("tag %s not found", tagID)
	}
	return nil
}

// AdminAddAccountToTag adds an account to a tag.
func AdminAddAccountToTag(tagID, handle, displayName string, realtimePriority bool) error {
	acct := models.SniperTagAccount{
		TagID:            tagID,
		Handle:           handle,
		DisplayName:      displayName,
		RealtimePriority: realtimePriority,
	}
	if err := database.DB.Create(&acct).Error; err != nil {
		return fmt.Errorf("failed to add @%s to tag %s (may already exist): %w", handle, tagID, err)
	}
	util.Log.Info("[sniper-admin] Added @%s to tag %s", handle, tagID)
	return nil
}

// AdminRemoveAccountFromTag removes an account from a tag.
func AdminRemoveAccountFromTag(tagID, handle string) error {
	result := database.DB.Where("tag_id = ? AND handle = ?", tagID, handle).
		Delete(&models.SniperTagAccount{})
	if result.RowsAffected == 0 {
		return fmt.Errorf("@%s not found in tag %s", handle, tagID)
	}
	return nil
}

// AdminListTags returns ALL tags (including inactive) for admin management.
func AdminListTags() ([]TagWithAccounts, error) {
	var tags []models.SniperTag
	if err := database.DB.Order("sort_order ASC, created_at ASC").
		Find(&tags).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch tags: %w", err)
	}

	result := make([]TagWithAccounts, 0, len(tags))
	for _, tag := range tags {
		var accounts []models.SniperTagAccount
		database.DB.Where("tag_id = ?", tag.ID).
			Order("sort_order ASC, created_at ASC").
			Find(&accounts)

		acctInfos := make([]TagAccountInfo, 0, len(accounts))
		for _, a := range accounts {
			acctInfos = append(acctInfos, TagAccountInfo{
				Handle:           a.Handle,
				DisplayName:      a.DisplayName,
				RealtimePriority: a.RealtimePriority,
			})
		}
		result = append(result, TagWithAccounts{
			SniperTag: tag,
			Accounts:  acctInfos,
		})
	}
	return result, nil
}

// AdminDeleteTag soft-deletes a tag by setting active=false and removing its accounts.
func AdminDeleteTag(tagID string) error {
	var tag models.SniperTag
	if err := database.DB.First(&tag, "id = ?", tagID).Error; err != nil {
		return fmt.Errorf("tag %s not found", tagID)
	}

	// Remove all associated accounts
	database.DB.Where("tag_id = ?", tagID).Delete(&models.SniperTagAccount{})

	// Set tag inactive
	tag.Active = false
	database.DB.Save(&tag)

	util.Log.Info("[sniper-admin] Deleted tag %s (%s)", tagID, tag.Name)
	return nil
}

// AdminListTagAccounts returns all accounts for a given tag.
func AdminListTagAccounts(tagID string) ([]models.SniperTagAccount, error) {
	var accounts []models.SniperTagAccount
	if err := database.DB.Where("tag_id = ?", tagID).
		Order("sort_order ASC, created_at ASC").
		Find(&accounts).Error; err != nil {
		return nil, err
	}
	return accounts, nil
}

// ──────────────────────────────────────────────────────────────────────────────
// Admin: Tag Candidate Management (AI-assisted)
// ──────────────────────────────────────────────────────────────────────────────

// AdminImportCandidates imports handles for AI classification and admin review.
func AdminImportCandidates(handles []string, source, sourceDetail string) (int, error) {
	imported := 0

	for _, handle := range handles {
		handle = cleanHandle(handle)
		if handle == "" {
			continue
		}

		// Check if already exists in candidates or tag_accounts
		var existCount int64
		database.DB.Model(&models.TagCandidate{}).
			Where("handle = ? AND status = ?", handle, models.TagCandidatePending).
			Count(&existCount)
		if existCount > 0 {
			continue // skip existing pending candidate
		}

		// Fetch profile from SocialData
		var displayName, bio string
		var followers int

		if SocialDataAvailable() {
			client := newSocialDataClient()
			user, err := client.FetchUser(handle)
			if err == nil {
				displayName = user.Name
				bio = user.Description
				followers = user.FollowersCount
			} else {
				util.Log.Warn("[sniper-admin] Failed to fetch profile for @%s: %v", handle, err)
			}
		}

		// AI classify: get recommended tags
		recommendedTags, aiReason := classifyAccountTags(handle, displayName, bio)

		candidate := models.TagCandidate{
			Handle:          handle,
			DisplayName:     displayName,
			Bio:             bio,
			FollowersCount:  followers,
			Source:          source,
			SourceDetail:    sourceDetail,
			RecommendedTags: recommendedTags,
			AIReason:        aiReason,
			Status:          models.TagCandidatePending,
		}

		if err := database.DB.Create(&candidate).Error; err != nil {
			util.Log.Warn("[sniper-admin] Failed to create candidate for @%s: %v", handle, err)
			continue
		}
		imported++
	}

	util.Log.Info("[sniper-admin] Imported %d candidates (source: %s)", imported, source)
	return imported, nil
}

// AdminListCandidates returns candidates with optional status filter.
func AdminListSniperCandidates(status string, limit, offset int) ([]models.TagCandidate, int64, error) {
	query := database.DB.Model(&models.TagCandidate{})
	if status != "" {
		query = query.Where("status = ?", status)
	}

	var total int64
	query.Count(&total)

	if limit <= 0 {
		limit = 50
	}

	var candidates []models.TagCandidate
	if err := query.Order("created_at DESC").
		Offset(offset).Limit(limit).
		Find(&candidates).Error; err != nil {
		return nil, 0, err
	}

	return candidates, total, nil
}

// AdminApproveCandidate approves a candidate and creates tag-account associations.
func AdminApproveCandidate(candidateID uuid.UUID, approvedTagIDs []string, realtimePriority bool, adminWallet string) error {
	var candidate models.TagCandidate
	if err := database.DB.First(&candidate, candidateID).Error; err != nil {
		return fmt.Errorf("candidate not found")
	}

	if candidate.Status != models.TagCandidatePending {
		return fmt.Errorf("candidate is already %s", candidate.Status)
	}

	// Create tag-account associations
	for _, tagID := range approvedTagIDs {
		err := AdminAddAccountToTag(tagID, candidate.Handle, candidate.DisplayName, realtimePriority)
		if err != nil {
			util.Log.Warn("[sniper-admin] Approve: failed to add @%s to tag %s: %v", candidate.Handle, tagID, err)
		}
	}

	// Update candidate status
	now := time.Now()
	candidate.Status = models.TagCandidateApproved
	candidate.ApprovedTags = models.JSON{"tags": approvedTagIDs}
	candidate.RealtimePriority = realtimePriority
	candidate.ReviewedBy = adminWallet
	candidate.ReviewedAt = &now
	database.DB.Save(&candidate)

	util.Log.Info("[sniper-admin] Approved candidate @%s → tags %v", candidate.Handle, approvedTagIDs)
	return nil
}

// AdminRejectCandidate marks a candidate as rejected.
func AdminRejectCandidate(candidateID uuid.UUID, adminWallet string) error {
	var candidate models.TagCandidate
	if err := database.DB.First(&candidate, candidateID).Error; err != nil {
		return fmt.Errorf("candidate not found")
	}

	if candidate.Status != models.TagCandidatePending {
		return fmt.Errorf("candidate is already %s", candidate.Status)
	}

	now := time.Now()
	candidate.Status = models.TagCandidateRejected
	candidate.ReviewedBy = adminWallet
	candidate.ReviewedAt = &now
	database.DB.Save(&candidate)

	util.Log.Info("[sniper-admin] Rejected candidate @%s", candidate.Handle)
	return nil
}

// classifyAccountTags uses LLM to recommend tags for an account.
func classifyAccountTags(handle, displayName, bio string) (models.JSON, string) {
	// Get all active tags for the prompt
	var tags []models.SniperTag
	database.DB.Where("active = ?", true).Find(&tags)

	if len(tags) == 0 {
		return nil, "no tags configured"
	}

	tagList := ""
	for _, t := range tags {
		tagList += fmt.Sprintf("- %s: %s (%s)\n", t.ID, t.NameEN, t.Description)
	}

	prompt := fmt.Sprintf(`You are a Twitter account classifier for a crypto content platform.

Given the following Twitter account and the available tags, determine which tags this account belongs to.

Account:
- Handle: @%s
- Display Name: %s
- Bio: %s

Available Tags:
%s

Respond in JSON format ONLY:
{
  "tags": [
    {"id": "tag_id_here", "confidence": 0.85}
  ],
  "reason": "Brief explanation of why these tags were chosen"
}

Rules:
- Only include tags with confidence >= 0.6
- An account can belong to multiple tags
- If the account doesn't clearly fit any tag, return empty tags array`, handle, displayName, bio, tagList)

	type classResult struct {
		Tags   []map[string]interface{} `json:"tags"`
		Reason string                   `json:"reason"`
	}

	var result classResult
	err := CallLLMJSON([]ChatMessage{
		{Role: "system", Content: "You are a precise classifier. Output valid JSON only."},
		{Role: "user", Content: prompt},
	}, 500, 0.3, &result)

	if err != nil {
		util.Log.Warn("[sniper-admin] LLM classification failed for @%s: %v", handle, err)
		return nil, "classification failed: " + err.Error()
	}

	// Convert to JSON model
	tagsJSON := models.JSON{}
	if len(result.Tags) > 0 {
		tagsJSON["tags"] = result.Tags
	}

	return tagsJSON, result.Reason
}

// cleanHandle strips @ prefix and whitespace from a handle.
func cleanHandle(handle string) string {
	handle = trimSpaces(handle)
	if len(handle) > 0 && handle[0] == '@' {
		handle = handle[1:]
	}
	return handle
}

// trimSpaces removes leading/trailing whitespace.
func trimSpaces(s string) string {
	result := ""
	started := false
	lastNonSpace := -1

	for i, c := range s {
		if c != ' ' && c != '\t' && c != '\n' && c != '\r' {
			if !started {
				started = true
			}
			lastNonSpace = i
		}
	}

	if !started {
		return ""
	}

	started = false
	for i, c := range s {
		if !started && (c == ' ' || c == '\t' || c == '\n' || c == '\r') {
			continue
		}
		started = true
		if i <= lastNonSpace {
			result += string(c)
		}
	}

	return result
}
