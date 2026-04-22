package services

import (
	"fmt"
	"strings"
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

	// Bump credits to Pro monthly quota if currently below it. We take max()
	// so that calling gift twice in the same period doesn't refund the credits
	// the user has already spent. Mirrors what subscription_created webhook does.
	updates := map[string]interface{}{
		"pro_expires_at": &newExp,
	}
	wasFreeOrLow := user.Credits < ProCreditsPerMonth
	if wasFreeOrLow {
		updates["credits"] = ProCreditsPerMonth
		updates["credits_reset"] = time.Now().Truncate(24 * time.Hour).AddDate(0, 1, 0)
	}
	if err := database.DB.Model(user).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("failed to update user pro state: %w", err)
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

// GiftProByIdentifier resolves a user by UUID or email and then grants Pro.
// V3 rule: gifts can ONLY be sent to email accounts. Wallet addresses are
// rejected, and the resolved user must have a non-empty email.
func GiftProByIdentifier(identifier string, months int, reason string, admin *models.AdminUser) (*models.User, *models.GiftProLog, error) {
	identifier = strings.TrimSpace(identifier)
	var user models.User
	q := database.DB
	switch {
	case len(identifier) == 42 && strings.HasPrefix(identifier, "0x"):
		return nil, nil, fmt.Errorf("gift pro requires an email or user id; wallet address is not allowed")
	default:
		if id, err := uuid.Parse(identifier); err == nil {
			q = q.Where("id = ?", id)
		} else if strings.Contains(identifier, "@") {
			q = q.Where("email = ?", strings.ToLower(identifier))
		} else {
			return nil, nil, fmt.Errorf("identifier must be a user id (uuid) or an email address")
		}
	}
	if err := q.First(&user).Error; err != nil {
		return nil, nil, fmt.Errorf("user not found")
	}
	if user.Email == "" {
		return nil, nil, fmt.Errorf("target user has no email; pro gifts can only be sent to email accounts")
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
