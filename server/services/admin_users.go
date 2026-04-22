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
// Returns the persisted user (nil only on DB failure).
func EnsureUser(walletAddr string) *models.User {
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
		if err := database.DB.Create(&user).Error; err != nil {
			util.Log.Error("[user] failed to create user %s: %v", walletAddr, err)
			return nil
		}
		util.Log.Info("[user] Created new user: %s", walletAddr)
	} else {
		// Existing user — update last_seen and login_count
		database.DB.Model(&user).Updates(map[string]interface{}{
			"last_seen_at": now,
			"login_count":  gorm.Expr("login_count + 1"),
		})
	}
	return &user
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
	Subscription string // pro | free | expired
	AuthType     string // wallet | email | linked
	Sort         string
	Order        string
}

// AdminUserListItem is a single item in the admin user list response.
// Identity is exposed via id (canonical), plus email and wallet_addr (either may be empty).
// auth_type is derived from which identity fields are populated.
type AdminUserListItem struct {
	ID            uuid.UUID  `json:"id"`
	AuthType      string     `json:"auth_type"`
	Email         string     `json:"email,omitempty"`
	EmailVerified bool       `json:"email_verified"`
	WalletAddr    string     `json:"wallet_addr,omitempty"`
	Status        string     `json:"status"`
	FirstSeenAt   time.Time  `json:"first_seen_at"`
	LastSeenAt    time.Time  `json:"last_seen_at"`
	LoginCount    int        `json:"login_count"`
	Note          string     `json:"note"`
	BanReason     string     `json:"ban_reason,omitempty"`
	BannedAt      *time.Time `json:"banned_at,omitempty"`
	IsPro         bool       `json:"is_pro"`
	ProExpiresAt  *time.Time `json:"pro_expires_at,omitempty"`
	SnipeCount    int64      `json:"snipe_count"`
}

// deriveAuthType returns "email" / "wallet" / "linked" / "unknown" from a User row.
func deriveAuthType(u *models.User) string {
	hasEmail := u.Email != ""
	hasWallet := u.WalletAddr != ""
	switch {
	case hasEmail && hasWallet:
		return "linked"
	case hasEmail:
		return "email"
	case hasWallet:
		return "wallet"
	default:
		return "unknown"
	}
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

	// Search across email and wallet
	if params.Search != "" {
		needle := "%" + params.Search + "%"
		query = query.Where("users.wallet_addr ILIKE ? OR users.email ILIKE ?", needle, needle)
	}
	if params.Status != "" {
		query = query.Where("users.status = ?", params.Status)
	}

	// Auth type filter
	switch params.AuthType {
	case "email":
		// pure email or linked → has email at minimum
		query = query.Where("users.email <> ''")
	case "wallet":
		query = query.Where("users.wallet_addr <> '' AND users.email = ''")
	case "linked":
		query = query.Where("users.wallet_addr <> '' AND users.email <> ''")
	}

	// Subscription filter — drives off users.pro_expires_at (V3 source of truth)
	now := time.Now().UTC()
	switch params.Subscription {
	case "pro":
		query = query.Where("users.pro_expires_at IS NOT NULL AND users.pro_expires_at > ?", now)
	case "free":
		query = query.Where("users.pro_expires_at IS NULL OR users.pro_expires_at <= ?", now)
	case "expired":
		query = query.Where("users.pro_expires_at IS NOT NULL AND users.pro_expires_at <= ?", now)
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

	// Snipe counts (still keyed by wallet_addr until vibe_write_replies fully
	// migrated to user_id; pure-email users will have count 0).
	wallets := make([]string, 0, len(users))
	for _, u := range users {
		if u.WalletAddr != "" {
			wallets = append(wallets, u.WalletAddr)
		}
	}
	type snipeCount struct {
		WalletAddr string
		Count      int64
	}
	snipeMap := make(map[string]int64)
	if len(wallets) > 0 {
		var snipes []snipeCount
		database.DB.Raw(`
			SELECT wallet_addr, COUNT(*) as count
			FROM vibe_write_replies
			WHERE wallet_addr IN ?
			GROUP BY wallet_addr
		`, wallets).Scan(&snipes)
		for _, s := range snipes {
			snipeMap[s.WalletAddr] = s.Count
		}
	}

	// Build result
	items := make([]AdminUserListItem, len(users))
	for i, u := range users {
		item := AdminUserListItem{
			ID:            u.ID,
			AuthType:      deriveAuthType(&u),
			Email:         u.Email,
			EmailVerified: u.EmailVerified,
			WalletAddr:    u.WalletAddr,
			Status:        u.Status,
			FirstSeenAt:   u.FirstSeenAt,
			LastSeenAt:    u.LastSeenAt,
			LoginCount:    u.LoginCount,
			Note:          u.Note,
			BanReason:     u.BanReason,
			BannedAt:      u.BannedAt,
			IsPro:         u.IsPro(),
			ProExpiresAt:  u.ProExpiresAt,
			SnipeCount:    snipeMap[u.WalletAddr],
		}
		items[i] = item
	}

	return items, total, nil
}

// AdminUserDetail is the full detail for a single user.
type AdminUserDetail struct {
	User                models.User           `json:"user"`
	AuthType            string                `json:"auth_type"`
	IsPro               bool                  `json:"is_pro"`
	Subscription        *models.Subscription  `json:"subscription"`
	SubscriptionHistory []models.Subscription `json:"subscription_history"`
	Persona             *models.UserPersona   `json:"persona"`
	SelectedTags        []string              `json:"selected_tags"`
	MutedAccounts       []string              `json:"muted_accounts"`
	Stats               AdminUserStats        `json:"stats"`
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
// Lookup is by users.id (UUID). Wallet-keyed business stats are filled only
// when the user has a non-empty wallet_addr; pure-email users see zeros.
func AdminGetUserDetail(userID uuid.UUID) (*AdminUserDetail, error) {
	var user models.User
	if err := database.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		return nil, fmt.Errorf("user not found")
	}

	detail := &AdminUserDetail{
		User:     user,
		AuthType: deriveAuthType(&user),
		IsPro:    user.IsPro(),
	}

	wallet := user.WalletAddr // may be empty

	// Active subscription (legacy table, still scanned for visibility)
	if wallet != "" {
		var activeSub models.Subscription
		if err := database.DB.Where("wallet_addr = ? AND status = ?", wallet, models.SubStatusActive).
			First(&activeSub).Error; err == nil {
			detail.Subscription = &activeSub
		}

		// Subscription history (all, newest first)
		var history []models.Subscription
		database.DB.Where("wallet_addr = ?", wallet).Order("created_at DESC").Find(&history)
		detail.SubscriptionHistory = history

		// Persona
		var persona models.UserPersona
		if err := database.DB.Where("wallet_addr = ?", wallet).First(&persona).Error; err == nil {
			detail.Persona = &persona
		}

		// Selected tags
		var tags []models.UserSelectedTag
		database.DB.Where("wallet_addr = ?", wallet).Find(&tags)
		tagIDs := make([]string, len(tags))
		for i, t := range tags {
			tagIDs[i] = t.TagID
		}
		detail.SelectedTags = tagIDs

		// Muted accounts
		var muted []models.UserMutedAccount
		database.DB.Where("wallet_addr = ?", wallet).Find(&muted)
		handles := make([]string, len(muted))
		for i, m := range muted {
			handles[i] = m.Handle
		}
		detail.MutedAccounts = handles

		// Stats
		todayStart := time.Now().UTC().Truncate(24 * time.Hour)
		var totalSnipes, todaySnipes, totalChats, shellsOwned, clawsBound int64
		database.DB.Model(&models.VibeWriteReply{}).Where("wallet_addr = ?", wallet).Count(&totalSnipes)
		database.DB.Model(&models.VibeWriteReply{}).Where("wallet_addr = ? AND created_at >= ?", wallet, todayStart).Count(&todaySnipes)
		database.DB.Model(&models.ChatSession{}).Where("wallet_addr = ?", wallet).Count(&totalChats)
		database.DB.Model(&models.Shell{}).Where("owner_addr = ? AND deleted_at IS NULL", wallet).Count(&shellsOwned)
		database.DB.Model(&models.ClawBinding{}).Where("wallet_addr = ?", wallet).Count(&clawsBound)

		var totalWithdrawals float64
		database.DB.Model(&models.WithdrawRecord{}).Where("to_addr = ? AND status IN ?", wallet, []string{"sent", "confirmed"}).
			Select("COALESCE(SUM(amount), 0)").Scan(&totalWithdrawals)

		detail.Stats = AdminUserStats{
			TotalSnipes:      totalSnipes,
			TodaySnipes:      todaySnipes,
			TotalChats:       totalChats,
			ShellsOwned:      shellsOwned,
			ClawsBound:       clawsBound,
			TotalWithdrawals: totalWithdrawals,
		}
	}

	// Vibe Write chats are keyed by user_id and apply to email users too
	var totalVibeChats int64
	database.DB.Model(&models.VibeChat{}).Where("user_id = ?", user.ID).Count(&totalVibeChats)
	detail.Stats.TotalChats += totalVibeChats

	return detail, nil
}

// AdminBanUser bans a user, destroys all sessions (wallet + email).
func AdminBanUser(userID uuid.UUID, reason string, admin *models.AdminUser) error {
	var user models.User
	if err := database.DB.Where("id = ?", userID).First(&user).Error; err != nil {
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

	database.DB.Model(&user).Updates(map[string]interface{}{
		"status":     models.UserStatusBanned,
		"ban_reason": reason,
		"banned_at":  &now,
		"banned_by":  adminName,
	})

	// Destroy all sessions to force logout (both auth paths)
	if user.WalletAddr != "" {
		database.DB.Where("wallet_addr = ?", user.WalletAddr).Delete(&models.WalletSession{})
	}
	database.DB.Where("user_id = ?", user.ID).Delete(&models.EmailSession{})

	// Cancel any active legacy subscription record for visibility
	if user.WalletAddr != "" {
		database.DB.Model(&models.Subscription{}).
			Where("wallet_addr = ? AND status = ?", user.WalletAddr, models.SubStatusActive).
			Update("status", models.SubStatusCancelled)
	}

	writeAuditLog(admin, "ban_user", "user", user.ID.String(), map[string]interface{}{
		"reason":      reason,
		"email":       user.Email,
		"wallet_addr": user.WalletAddr,
	}, "")

	util.Log.Info("[admin] Banned user %s (email=%q wallet=%q) by %s: %s", user.ID, user.Email, user.WalletAddr, adminName, reason)
	return nil
}

// AdminUnbanUser unbans a user.
func AdminUnbanUser(userID uuid.UUID, admin *models.AdminUser) error {
	var user models.User
	if err := database.DB.Where("id = ?", userID).First(&user).Error; err != nil {
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

	writeAuditLog(admin, "unban_user", "user", user.ID.String(), nil, "")

	adminName := ""
	if admin != nil {
		adminName = admin.Username
	}
	util.Log.Info("[admin] Unbanned user %s by %s", user.ID, adminName)
	return nil
}

// AdminUpdateUserNote updates the admin note on a user.
func AdminUpdateUserNote(userID uuid.UUID, note string, admin *models.AdminUser) error {
	result := database.DB.Model(&models.User{}).Where("id = ?", userID).Update("note", note)
	if result.RowsAffected == 0 {
		return fmt.Errorf("user not found")
	}

	writeAuditLog(admin, "update_note", "user", userID.String(), map[string]interface{}{
		"note": note,
	}, "")
	return nil
}

// AdminGrantSubscription grants Pro to a user. V3: writes users.pro_expires_at
// directly (single source of truth). The legacy subscriptions table is left alone.
// `tier` must equal "pro" — other tiers are rejected.
func AdminGrantSubscription(userID uuid.UUID, tier string, days int, reason string, admin *models.AdminUser) error {
	if tier != models.SubTierPro {
		return fmt.Errorf("only 'pro' tier is supported via grant")
	}

	var user models.User
	if err := database.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		return fmt.Errorf("user not found")
	}
	if user.IsPro() {
		return fmt.Errorf("user already has active Pro until %s (use extend instead)", user.ProExpiresAt.Format(time.RFC3339))
	}

	newExp := time.Now().UTC().Add(time.Duration(days) * 24 * time.Hour)
	if err := database.DB.Model(&user).Update("pro_expires_at", &newExp).Error; err != nil {
		return fmt.Errorf("failed to grant pro: %w", err)
	}

	writeAuditLog(admin, "grant_subscription", "user", user.ID.String(), map[string]interface{}{
		"tier":           tier,
		"days":           days,
		"reason":         reason,
		"new_expires_at": newExp,
	}, "")

	util.Log.Info("[admin] Granted %s (%d days) to user %s — reason: %s", tier, days, user.ID, reason)
	return nil
}

// AdminExtendSubscription extends users.pro_expires_at by N days.
func AdminExtendSubscription(userID uuid.UUID, days int, reason string, admin *models.AdminUser) error {
	var user models.User
	if err := database.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		return fmt.Errorf("user not found")
	}
	if user.ProExpiresAt == nil {
		return fmt.Errorf("user has no Pro expiry to extend (use grant instead)")
	}

	old := *user.ProExpiresAt
	base := old
	now := time.Now().UTC()
	if base.Before(now) {
		base = now
	}
	newExp := base.Add(time.Duration(days) * 24 * time.Hour)
	if err := database.DB.Model(&user).Update("pro_expires_at", &newExp).Error; err != nil {
		return fmt.Errorf("failed to extend pro: %w", err)
	}

	writeAuditLog(admin, "extend_subscription", "user", user.ID.String(), map[string]interface{}{
		"days":           days,
		"reason":         reason,
		"old_expires_at": old,
		"new_expires_at": newExp,
	}, "")

	util.Log.Info("[admin] Extended Pro for user %s by %d days (new expiry: %s)", user.ID, days, newExp)
	return nil
}

// AdminRevokeSubscription clears users.pro_expires_at (immediate revoke).
func AdminRevokeSubscription(userID uuid.UUID, reason string, admin *models.AdminUser) error {
	var user models.User
	if err := database.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		return fmt.Errorf("user not found")
	}
	if user.ProExpiresAt == nil {
		return fmt.Errorf("user has no Pro to revoke")
	}

	old := *user.ProExpiresAt
	if err := database.DB.Model(&user).Update("pro_expires_at", nil).Error; err != nil {
		return fmt.Errorf("failed to revoke pro: %w", err)
	}

	writeAuditLog(admin, "revoke_subscription", "user", user.ID.String(), map[string]interface{}{
		"reason":         reason,
		"old_expires_at": old,
	}, "")

	util.Log.Info("[admin] Revoked Pro for user %s — reason: %s", user.ID, reason)
	return nil
}

// AdminUserOverviewStats holds aggregate numbers for the admin dashboard.
// Identity-axis breakdown matches the V3 list page filter set.
type AdminUserOverviewStats struct {
	TotalUsers        int64 `json:"total_users"`
	ActiveUsers       int64 `json:"active_users"`
	BannedUsers       int64 `json:"banned_users"`
	WalletOnlyUsers   int64 `json:"wallet_only_users"`
	EmailOnlyUsers    int64 `json:"email_only_users"`
	LinkedUsers       int64 `json:"linked_users"`
	ProSubscribers    int64 `json:"pro_subscribers"`
	FreeUsers         int64 `json:"free_users"`
	TodayNewUsers     int64 `json:"today_new_users"`
	TodayActiveUsers  int64 `json:"today_active_users"`
	WeeklyActiveUsers int64 `json:"weekly_active_users"`
}

// AdminGetUserStats returns aggregated user statistics.
func AdminGetUserStats() (*AdminUserOverviewStats, error) {
	stats := &AdminUserOverviewStats{}
	now := time.Now().UTC()
	todayStart := now.Truncate(24 * time.Hour)
	weekAgo := now.AddDate(0, 0, -7)

	base := database.DB.Model(&models.User{}).Where("deleted_at IS NULL")

	base.Session(&gorm.Session{}).Count(&stats.TotalUsers)
	base.Session(&gorm.Session{}).Where("status = ?", models.UserStatusActive).Count(&stats.ActiveUsers)
	base.Session(&gorm.Session{}).Where("status = ?", models.UserStatusBanned).Count(&stats.BannedUsers)
	base.Session(&gorm.Session{}).Where("first_seen_at >= ?", todayStart).Count(&stats.TodayNewUsers)
	base.Session(&gorm.Session{}).Where("last_seen_at >= ?", todayStart).Count(&stats.TodayActiveUsers)
	base.Session(&gorm.Session{}).Where("last_seen_at >= ?", weekAgo).Count(&stats.WeeklyActiveUsers)

	// Identity-axis breakdown
	base.Session(&gorm.Session{}).Where("wallet_addr <> '' AND email = ''").Count(&stats.WalletOnlyUsers)
	base.Session(&gorm.Session{}).Where("wallet_addr = '' AND email <> ''").Count(&stats.EmailOnlyUsers)
	base.Session(&gorm.Session{}).Where("wallet_addr <> '' AND email <> ''").Count(&stats.LinkedUsers)

	// Pro subscribers — V3: drives off users.pro_expires_at
	base.Session(&gorm.Session{}).Where("pro_expires_at IS NOT NULL AND pro_expires_at > ?", now).Count(&stats.ProSubscribers)
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

// WriteAuditLog is the public wrapper around writeAuditLog so other packages
// (e.g. admin handlers in different files) can record audit entries.
func WriteAuditLog(admin *models.AdminUser, action, targetType, targetID string, detail map[string]interface{}, ip string) {
	writeAuditLog(admin, action, targetType, targetID, detail, ip)
}
