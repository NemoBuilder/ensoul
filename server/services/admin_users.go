package services

import (
	"fmt"
	"time"

	"github.com/ensoul-labs/ensoul-server/database"
	"github.com/ensoul-labs/ensoul-server/models"
	"github.com/ensoul-labs/ensoul-server/util"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ═══════════════════════════════════════════════════════════════════════
// User Management Service
// ═══════════════════════════════════════════════════════════════════════

// EnsureUser creates or updates a User record on login. Called from auth handler.
func EnsureUser(walletAddr string) {
	now := time.Now()
	var user models.User
	result := database.DB.Where("wallet_addr = ?", walletAddr).First(&user)
	if result.Error != nil {
		// New user — create
		user = models.User{
			WalletAddr:  walletAddr,
			Status:      models.UserStatusActive,
			FirstSeenAt: now,
			LastSeenAt:  now,
			LoginCount:  1,
		}
		database.DB.Create(&user)
		util.Log.Info("[user] Created new user: %s", walletAddr)
	} else {
		// Existing user — update last_seen and login_count
		database.DB.Model(&user).Updates(map[string]interface{}{
			"last_seen_at": now,
			"login_count":  gorm.Expr("login_count + 1"),
		})
	}
}

// IsUserBanned checks if a wallet address is banned. Returns (banned, reason).
func IsUserBanned(walletAddr string) (bool, string) {
	var user models.User
	if err := database.DB.Where("wallet_addr = ?", walletAddr).First(&user).Error; err != nil {
		return false, "" // no user record = not banned
	}
	if user.Status == models.UserStatusBanned {
		return true, user.BanReason
	}
	return false, ""
}

// AdminUserListParams holds query parameters for user listing.
type AdminUserListParams struct {
	Page         int
	PageSize     int
	Search       string
	Status       string
	Subscription string
	Sort         string
	Order        string
}

// AdminUserListItem is a single item in the admin user list response.
type AdminUserListItem struct {
	WalletAddr  string     `json:"wallet_addr"`
	Status      string     `json:"status"`
	FirstSeenAt time.Time  `json:"first_seen_at"`
	LastSeenAt  time.Time  `json:"last_seen_at"`
	LoginCount  int        `json:"login_count"`
	Note        string     `json:"note"`
	BanReason   string     `json:"ban_reason,omitempty"`
	BannedAt    *time.Time `json:"banned_at,omitempty"`
	SubTier     *string    `json:"sub_tier"`
	SubStatus   *string    `json:"sub_status"`
	SubExpires  *time.Time `json:"sub_expires_at"`
	SnipeCount  int64      `json:"snipe_count"`
}

// AdminListUsers returns a paginated, filtered list of users.
func AdminListUsers(params AdminUserListParams) ([]AdminUserListItem, int64, error) {
	// Default values
	if params.Page < 1 {
		params.Page = 1
	}
	if params.PageSize < 1 || params.PageSize > 100 {
		params.PageSize = 20
	}

	// Validate sort
	allowedSorts := map[string]string{
		"last_seen_at":  "users.last_seen_at",
		"first_seen_at": "users.first_seen_at",
		"login_count":   "users.login_count",
	}
	sortCol, ok := allowedSorts[params.Sort]
	if !ok {
		sortCol = "users.last_seen_at"
	}
	if params.Order != "asc" {
		params.Order = "desc"
	}

	// Build base query
	query := database.DB.Model(&models.User{}).Where("users.deleted_at IS NULL")

	// Filters
	if params.Search != "" {
		query = query.Where("users.wallet_addr ILIKE ?", params.Search+"%")
	}
	if params.Status != "" {
		query = query.Where("users.status = ?", params.Status)
	}

	// Subscription filter — needs subquery
	if params.Subscription == "pro" {
		query = query.Where("EXISTS (SELECT 1 FROM subscriptions s WHERE s.wallet_addr = users.wallet_addr AND s.status = ? AND s.tier = ? AND s.deleted_at IS NULL)", models.SubStatusActive, models.SubTierPro)
	} else if params.Subscription == "free" {
		query = query.Where("NOT EXISTS (SELECT 1 FROM subscriptions s WHERE s.wallet_addr = users.wallet_addr AND s.status = ? AND s.deleted_at IS NULL)", models.SubStatusActive)
	} else if params.Subscription == "expired" {
		query = query.Where("EXISTS (SELECT 1 FROM subscriptions s WHERE s.wallet_addr = users.wallet_addr AND s.status = ? AND s.deleted_at IS NULL)", models.SubStatusExpired)
	}

	// Count total
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Fetch users
	var users []models.User
	offset := (params.Page - 1) * params.PageSize
	if err := query.Order(fmt.Sprintf("%s %s", sortCol, params.Order)).
		Offset(offset).Limit(params.PageSize).Find(&users).Error; err != nil {
		return nil, 0, err
	}

	// Batch-fetch subscription info and snipe counts
	wallets := make([]string, len(users))
	for i, u := range users {
		wallets[i] = u.WalletAddr
	}

	// Latest active subscriptions
	type subInfo struct {
		WalletAddr string
		Tier       string
		Status     string
		ExpiresAt  time.Time
	}
	var subs []subInfo
	if len(wallets) > 0 {
		database.DB.Raw(`
			SELECT DISTINCT ON (wallet_addr) wallet_addr, tier, status, expires_at
			FROM subscriptions
			WHERE wallet_addr IN ? AND deleted_at IS NULL
			ORDER BY wallet_addr, created_at DESC
		`, wallets).Scan(&subs)
	}
	subMap := make(map[string]*subInfo)
	for i := range subs {
		subMap[subs[i].WalletAddr] = &subs[i]
	}

	// Snipe counts
	type snipeCount struct {
		WalletAddr string
		Count      int64
	}
	var snipes []snipeCount
	if len(wallets) > 0 {
		database.DB.Raw(`
			SELECT wallet_addr, COUNT(*) as count
			FROM sniper_replies
			WHERE wallet_addr IN ?
			GROUP BY wallet_addr
		`, wallets).Scan(&snipes)
	}
	snipeMap := make(map[string]int64)
	for _, s := range snipes {
		snipeMap[s.WalletAddr] = s.Count
	}

	// Build result
	items := make([]AdminUserListItem, len(users))
	for i, u := range users {
		item := AdminUserListItem{
			WalletAddr:  u.WalletAddr,
			Status:      u.Status,
			FirstSeenAt: u.FirstSeenAt,
			LastSeenAt:  u.LastSeenAt,
			LoginCount:  u.LoginCount,
			Note:        u.Note,
			BanReason:   u.BanReason,
			BannedAt:    u.BannedAt,
			SnipeCount:  snipeMap[u.WalletAddr],
		}
		if sub, ok := subMap[u.WalletAddr]; ok {
			item.SubTier = &sub.Tier
			item.SubStatus = &sub.Status
			item.SubExpires = &sub.ExpiresAt
		}
		items[i] = item
	}

	return items, total, nil
}

// AdminUserDetail is the full detail for a single user.
type AdminUserDetail struct {
	User                models.User          `json:"user"`
	Subscription        *models.Subscription `json:"subscription"`
	SubscriptionHistory []models.Subscription `json:"subscription_history"`
	Persona             *models.UserPersona  `json:"persona"`
	SelectedTags        []string             `json:"selected_tags"`
	MutedAccounts       []string             `json:"muted_accounts"`
	Stats               AdminUserStats       `json:"stats"`
}

// AdminUserStats holds aggregated stats for a user.
type AdminUserStats struct {
	TotalSnipes      int64   `json:"total_snipes"`
	TodaySnipes      int64   `json:"today_snipes"`
	TotalChats       int64   `json:"total_chats"`
	ShellsOwned      int64   `json:"shells_owned"`
	ClawsBound       int64   `json:"claws_bound"`
	TotalWithdrawals float64 `json:"total_withdrawals"`
}

// AdminGetUserDetail returns the full user detail for an admin view.
func AdminGetUserDetail(walletAddr string) (*AdminUserDetail, error) {
	var user models.User
	if err := database.DB.Where("wallet_addr = ?", walletAddr).First(&user).Error; err != nil {
		return nil, fmt.Errorf("user not found")
	}

	detail := &AdminUserDetail{User: user}

	// Active subscription
	var activeSub models.Subscription
	if err := database.DB.Where("wallet_addr = ? AND status = ?", walletAddr, models.SubStatusActive).
		First(&activeSub).Error; err == nil {
		detail.Subscription = &activeSub
	}

	// Subscription history (all, newest first)
	var history []models.Subscription
	database.DB.Where("wallet_addr = ?", walletAddr).Order("created_at DESC").Find(&history)
	detail.SubscriptionHistory = history

	// Persona
	var persona models.UserPersona
	if err := database.DB.Where("wallet_addr = ?", walletAddr).First(&persona).Error; err == nil {
		detail.Persona = &persona
	}

	// Selected tags
	var tags []models.UserSelectedTag
	database.DB.Where("wallet_addr = ?", walletAddr).Find(&tags)
	tagIDs := make([]string, len(tags))
	for i, t := range tags {
		tagIDs[i] = t.TagID
	}
	detail.SelectedTags = tagIDs

	// Muted accounts
	var muted []models.UserMutedAccount
	database.DB.Where("wallet_addr = ?", walletAddr).Find(&muted)
	handles := make([]string, len(muted))
	for i, m := range muted {
		handles[i] = m.Handle
	}
	detail.MutedAccounts = handles

	// Stats
	todayStart := time.Now().UTC().Truncate(24 * time.Hour)

	var totalSnipes, todaySnipes, totalChats, shellsOwned, clawsBound int64
	database.DB.Model(&models.SniperReply{}).Where("wallet_addr = ?", walletAddr).Count(&totalSnipes)
	database.DB.Model(&models.SniperReply{}).Where("wallet_addr = ? AND created_at >= ?", walletAddr, todayStart).Count(&todaySnipes)
	database.DB.Model(&models.ChatSession{}).Where("wallet_addr = ?", walletAddr).Count(&totalChats)
	database.DB.Model(&models.Shell{}).Where("owner_addr = ? AND deleted_at IS NULL", walletAddr).Count(&shellsOwned)
	database.DB.Model(&models.ClawBinding{}).Where("wallet_addr = ?", walletAddr).Count(&clawsBound)

	var totalWithdrawals float64
	database.DB.Model(&models.WithdrawRecord{}).Where("to_addr = ? AND status IN ?", walletAddr, []string{"sent", "confirmed"}).
		Select("COALESCE(SUM(amount), 0)").Scan(&totalWithdrawals)

	detail.Stats = AdminUserStats{
		TotalSnipes:      totalSnipes,
		TodaySnipes:      todaySnipes,
		TotalChats:       totalChats,
		ShellsOwned:      shellsOwned,
		ClawsBound:       clawsBound,
		TotalWithdrawals: totalWithdrawals,
	}

	return detail, nil
}

// AdminBanUser bans a user, destroys sessions, and cancels active subscription.
func AdminBanUser(walletAddr, reason string, admin *models.AdminUser) error {
	var user models.User
	if err := database.DB.Where("wallet_addr = ?", walletAddr).First(&user).Error; err != nil {
		return fmt.Errorf("user not found")
	}
	if user.Status == models.UserStatusBanned {
		return fmt.Errorf("user is already banned")
	}

	now := time.Now()
	adminName := ""
	if admin != nil {
		adminName = admin.Username
	}

	// Ban the user
	database.DB.Model(&user).Updates(map[string]interface{}{
		"status":    models.UserStatusBanned,
		"ban_reason": reason,
		"banned_at":  &now,
		"banned_by":  adminName,
	})

	// Destroy all wallet sessions (force logout)
	database.DB.Where("wallet_addr = ?", walletAddr).Delete(&models.WalletSession{})

	// Cancel active subscription
	database.DB.Model(&models.Subscription{}).
		Where("wallet_addr = ? AND status = ?", walletAddr, models.SubStatusActive).
		Update("status", models.SubStatusCancelled)

	// Audit log
	writeAuditLog(admin, "ban_user", "user", walletAddr, map[string]interface{}{
		"reason": reason,
	}, "")

	util.Log.Info("[admin] Banned user %s by %s: %s", walletAddr, adminName, reason)
	return nil
}

// AdminUnbanUser unbans a user.
func AdminUnbanUser(walletAddr string, admin *models.AdminUser) error {
	var user models.User
	if err := database.DB.Where("wallet_addr = ?", walletAddr).First(&user).Error; err != nil {
		return fmt.Errorf("user not found")
	}
	if user.Status != models.UserStatusBanned {
		return fmt.Errorf("user is not banned")
	}

	database.DB.Model(&user).Updates(map[string]interface{}{
		"status":     models.UserStatusActive,
		"ban_reason": "",
		"banned_at":  nil,
		"banned_by":  "",
	})

	writeAuditLog(admin, "unban_user", "user", walletAddr, nil, "")

	adminName := ""
	if admin != nil {
		adminName = admin.Username
	}
	util.Log.Info("[admin] Unbanned user %s by %s", walletAddr, adminName)
	return nil
}

// AdminUpdateUserNote updates the admin note on a user.
func AdminUpdateUserNote(walletAddr, note string, admin *models.AdminUser) error {
	result := database.DB.Model(&models.User{}).Where("wallet_addr = ?", walletAddr).Update("note", note)
	if result.RowsAffected == 0 {
		return fmt.Errorf("user not found")
	}

	writeAuditLog(admin, "update_note", "user", walletAddr, map[string]interface{}{
		"note": note,
	}, "")
	return nil
}

// AdminGrantSubscription creates a new subscription for a user (admin grant, no payment).
func AdminGrantSubscription(walletAddr, tier string, days int, reason string, admin *models.AdminUser) error {
	// Check tier is valid
	if _, ok := models.SubscriptionTiers[tier]; !ok {
		return fmt.Errorf("invalid tier: %s", tier)
	}

	// Check for existing active subscription
	var existing models.Subscription
	if err := database.DB.Where("wallet_addr = ? AND status = ?", walletAddr, models.SubStatusActive).
		First(&existing).Error; err == nil {
		return fmt.Errorf("user already has an active %s subscription (use extend instead)", existing.Tier)
	}

	// Ensure user exists
	var user models.User
	if err := database.DB.Where("wallet_addr = ?", walletAddr).First(&user).Error; err != nil {
		return fmt.Errorf("user not found")
	}

	tierCfg := models.SubscriptionTiers[tier]
	sub := &models.Subscription{
		WalletAddr:    walletAddr,
		Tier:          tier,
		LLMModel:      tierCfg.DefaultModel,
		Status:        models.SubStatusActive,
		ExpiresAt:     time.Now().UTC().Add(time.Duration(days) * 24 * time.Hour),
		PaymentTxHash: "admin_grant",
		PaymentToken:  "ADMIN",
		PaymentAmount: 0,
	}
	if err := database.DB.Create(sub).Error; err != nil {
		return fmt.Errorf("failed to create subscription: %w", err)
	}

	writeAuditLog(admin, "grant_subscription", "subscription", walletAddr, map[string]interface{}{
		"tier": tier, "days": days, "reason": reason, "subscription_id": sub.ID,
	}, "")

	util.Log.Info("[admin] Granted %s subscription (%d days) to %s — reason: %s", tier, days, walletAddr, reason)
	return nil
}

// AdminExtendSubscription extends the active subscription by N days.
func AdminExtendSubscription(walletAddr string, days int, reason string, admin *models.AdminUser) error {
	var sub models.Subscription
	if err := database.DB.Where("wallet_addr = ? AND status = ?", walletAddr, models.SubStatusActive).
		First(&sub).Error; err != nil {
		return fmt.Errorf("no active subscription found")
	}

	oldExpiry := sub.ExpiresAt
	sub.ExpiresAt = sub.ExpiresAt.Add(time.Duration(days) * 24 * time.Hour)
	database.DB.Save(&sub)

	writeAuditLog(admin, "extend_subscription", "subscription", walletAddr, map[string]interface{}{
		"days": days, "reason": reason,
		"old_expires_at": oldExpiry, "new_expires_at": sub.ExpiresAt,
		"subscription_id": sub.ID,
	}, "")

	util.Log.Info("[admin] Extended subscription for %s by %d days (new expiry: %s)", walletAddr, days, sub.ExpiresAt)
	return nil
}

// AdminRevokeSubscription cancels the active subscription.
func AdminRevokeSubscription(walletAddr, reason string, admin *models.AdminUser) error {
	var sub models.Subscription
	if err := database.DB.Where("wallet_addr = ? AND status = ?", walletAddr, models.SubStatusActive).
		First(&sub).Error; err != nil {
		return fmt.Errorf("no active subscription found")
	}

	sub.Status = models.SubStatusCancelled
	database.DB.Save(&sub)

	writeAuditLog(admin, "revoke_subscription", "subscription", walletAddr, map[string]interface{}{
		"reason": reason, "subscription_id": sub.ID, "tier": sub.Tier,
	}, "")

	util.Log.Info("[admin] Revoked subscription for %s — reason: %s", walletAddr, reason)
	return nil
}

// AdminUserOverviewStats holds aggregate numbers for the admin dashboard.
type AdminUserOverviewStats struct {
	TotalUsers       int64 `json:"total_users"`
	ActiveUsers      int64 `json:"active_users"`
	BannedUsers      int64 `json:"banned_users"`
	ProSubscribers   int64 `json:"pro_subscribers"`
	FreeUsers        int64 `json:"free_users"`
	TodayNewUsers    int64 `json:"today_new_users"`
	TodayActiveUsers int64 `json:"today_active_users"`
	WeeklyActiveUsers int64 `json:"weekly_active_users"`
}

// AdminGetUserStats returns aggregated user statistics.
func AdminGetUserStats() (*AdminUserOverviewStats, error) {
	stats := &AdminUserOverviewStats{}
	now := time.Now().UTC()
	todayStart := now.Truncate(24 * time.Hour)
	weekAgo := now.AddDate(0, 0, -7)

	database.DB.Model(&models.User{}).Where("deleted_at IS NULL").Count(&stats.TotalUsers)
	database.DB.Model(&models.User{}).Where("deleted_at IS NULL AND status = ?", models.UserStatusActive).Count(&stats.ActiveUsers)
	database.DB.Model(&models.User{}).Where("deleted_at IS NULL AND status = ?", models.UserStatusBanned).Count(&stats.BannedUsers)
	database.DB.Model(&models.User{}).Where("deleted_at IS NULL AND first_seen_at >= ?", todayStart).Count(&stats.TodayNewUsers)
	database.DB.Model(&models.User{}).Where("deleted_at IS NULL AND last_seen_at >= ?", todayStart).Count(&stats.TodayActiveUsers)
	database.DB.Model(&models.User{}).Where("deleted_at IS NULL AND last_seen_at >= ?", weekAgo).Count(&stats.WeeklyActiveUsers)

	// Pro subscribers = distinct wallets with active pro subscription
	database.DB.Raw(`
		SELECT COUNT(DISTINCT wallet_addr) FROM subscriptions
		WHERE status = ? AND tier = ? AND deleted_at IS NULL
	`, models.SubStatusActive, models.SubTierPro).Scan(&stats.ProSubscribers)

	stats.FreeUsers = stats.TotalUsers - stats.ProSubscribers

	return stats, nil
}

// AdminListAuditLogs returns paginated audit logs.
type AdminAuditLogParams struct {
	Page     int
	PageSize int
	Action   string
	TargetID string
}

func AdminListAuditLogs(params AdminAuditLogParams) ([]models.AdminAuditLog, int64, error) {
	if params.Page < 1 {
		params.Page = 1
	}
	if params.PageSize < 1 || params.PageSize > 100 {
		params.PageSize = 20
	}

	query := database.DB.Model(&models.AdminAuditLog{})
	if params.Action != "" {
		query = query.Where("action = ?", params.Action)
	}
	if params.TargetID != "" {
		query = query.Where("target_id = ?", params.TargetID)
	}

	var total int64
	query.Count(&total)

	var logs []models.AdminAuditLog
	offset := (params.Page - 1) * params.PageSize
	query.Order("created_at DESC").Offset(offset).Limit(params.PageSize).Find(&logs)

	return logs, total, nil
}

// writeAuditLog creates an audit log entry.
func writeAuditLog(admin *models.AdminUser, action, targetType, targetID string, detail map[string]interface{}, ip string) {
	log := models.AdminAuditLog{
		Action:     action,
		TargetType: targetType,
		TargetID:   targetID,
		IP:         ip,
	}
	if admin != nil {
		log.AdminUserID = admin.ID
		log.AdminName = admin.Username
	} else {
		log.AdminUserID = uuid.Nil
		log.AdminName = "api_key"
	}
	if detail != nil {
		log.Detail = models.JSON(detail)
	}
	database.DB.Create(&log)
}
