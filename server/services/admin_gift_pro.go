package services

import (
	"fmt"
	"time"

	"github.com/ensoul-labs/ensoul-server/database"
	"github.com/ensoul-labs/ensoul-server/models"
	"github.com/ensoul-labs/ensoul-server/util"
	"github.com/google/uuid"
)

// ═══════════════════════════════════════════════════════════════════════
// Admin Gift-Pro Service
//
// Single source of truth for promotional Pro grants. Operates directly on
// User.ProExpiresAt (which is what billing.go and IsPro() actually read),
// not the deprecated Subscription table. Idempotent semantics:
//
//   user.pro_expires_at = max(now, current) + months * 30 days
//
// Every call writes a GiftProLog row for full audit visibility.
// ═══════════════════════════════════════════════════════════════════════

// GiftProByUserID grants `months` of Pro to the given user (idempotent extend).
// Identifier may be either a UUID (preferred) or an email; the resolution is
// done by the caller — this function expects a concrete User row.
func GiftPro(user *models.User, months int, reason string, admin *models.AdminUser) (*models.GiftProLog, error) {
	if months < 1 || months > 24 {
		return nil, fmt.Errorf("months must be between 1 and 24")
	}
	now := time.Now().UTC()
	base := now
	if user.ProExpiresAt != nil && user.ProExpiresAt.After(now) {
		base = *user.ProExpiresAt
	}
	newExp := base.AddDate(0, months, 0)

	old := user.ProExpiresAt
	if err := database.DB.Model(user).Update("pro_expires_at", &newExp).Error; err != nil {
		return nil, fmt.Errorf("failed to update pro_expires_at: %w", err)
	}

	log := &models.GiftProLog{
		UserID:       user.ID,
		UserEmail:    user.Email,
		UserWallet:   user.WalletAddr,
		Months:       months,
		Reason:       reason,
		OldExpiresAt: old,
		NewExpiresAt: newExp,
	}
	if admin != nil {
		id := admin.ID
		log.AdminUserID = &id
		log.AdminName = admin.Username
	} else {
		log.AdminName = "api_key"
	}
	if err := database.DB.Create(log).Error; err != nil {
		return nil, fmt.Errorf("failed to write gift_pro_log: %w", err)
	}

	writeAuditLog(admin, "gift_pro", "user", user.ID.String(), map[string]interface{}{
		"months":         months,
		"reason":         reason,
		"old_expires_at": old,
		"new_expires_at": newExp,
	}, "")
	util.Log.Info("[admin] Gifted %d months Pro to user %s (was %v, now %s) — %s",
		months, user.ID, old, newExp.Format(time.RFC3339), reason)
	return log, nil
}

// GiftProByIdentifier resolves a user by UUID, email, or wallet address and
// then grants Pro. Returns the user (post-update) along with the log row.
func GiftProByIdentifier(identifier string, months int, reason string, admin *models.AdminUser) (*models.User, *models.GiftProLog, error) {
	var user models.User
	q := database.DB
	if id, err := uuid.Parse(identifier); err == nil {
		q = q.Where("id = ?", id)
	} else if len(identifier) == 42 && identifier[0] == '0' && identifier[1] == 'x' {
		q = q.Where("wallet_addr = ?", identifier)
	} else {
		q = q.Where("email = ?", identifier)
	}
	if err := q.First(&user).Error; err != nil {
		return nil, nil, fmt.Errorf("user not found")
	}
	log, err := GiftPro(&user, months, reason, admin)
	if err != nil {
		return nil, nil, err
	}
	// Reload with fresh expiry
	database.DB.First(&user, "id = ?", user.ID)
	return &user, log, nil
}

// GiftProLogParams controls listing.
type GiftProLogParams struct {
	Page     int
	PageSize int
	UserID   string // optional filter — UUID, email, or wallet
}

// ListGiftProLogs returns paginated gift_pro_logs, newest first.
func ListGiftProLogs(params GiftProLogParams) ([]models.GiftProLog, int64, error) {
	if params.Page < 1 {
		params.Page = 1
	}
	if params.PageSize < 1 || params.PageSize > 200 {
		params.PageSize = 50
	}
	q := database.DB.Model(&models.GiftProLog{})
	if params.UserID != "" {
		if id, err := uuid.Parse(params.UserID); err == nil {
			q = q.Where("user_id = ?", id)
		} else if len(params.UserID) == 42 {
			q = q.Where("user_wallet = ?", params.UserID)
		} else {
			q = q.Where("user_email = ?", params.UserID)
		}
	}
	var total int64
	q.Count(&total)
	var logs []models.GiftProLog
	err := q.Order("created_at DESC").
		Offset((params.Page - 1) * params.PageSize).
		Limit(params.PageSize).
		Find(&logs).Error
	return logs, total, err
}
