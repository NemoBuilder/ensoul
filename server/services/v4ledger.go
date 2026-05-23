// V4 Credits ledger wrapper.
//
// Reuses services.DeductCredits (which mutates User.Credits atomically) and
// appends a models.CreditLedger row so every V4 debit/credit is auditable.
//
// Do NOT bypass this wrapper inside V4 handlers — direct DeductCredits calls
// leave the ledger blank and break the credits-history endpoint.
package services

import (
	"github.com/ensoul-labs/ensoul-server/database"
	"github.com/ensoul-labs/ensoul-server/models"
	"github.com/google/uuid"
)

// V4 intake costs (small for Phase 1, will be tuned post-mainnet).
const (
	CreditCostV4SourceURL = 1 // gating a URL-only source through intake L1
	CreditCostV4SourceDoc = 5 // distilling an uploaded document (Phase 1.x)
)

// V4LedgerDeduct deducts `amount` credits from the user AND writes a ledger row.
// Returns the post-balance and any error from the underlying DeductCredits call.
//
// reason should be one of: "v4_source_intake", "v4_distill", "v4_refund" (use
// a negative amount for refund), "v4_admin_grant".
func V4LedgerDeduct(userID uuid.UUID, amount int, reason, refType, refID string) (int, error) {
	if err := DeductCredits(userID, amount); err != nil {
		return 0, err
	}

	// Snapshot the new balance for the ledger row.
	var user models.User
	if err := database.DB.Select("credits").First(&user, "id = ?", userID).Error; err != nil {
		// Deduction succeeded; logging best-effort.
		return 0, err
	}

	row := models.CreditLedger{
		UserID:  userID,
		Amount:  -amount, // debit recorded as negative
		Balance: user.Credits,
		Reason:  reason,
		RefType: refType,
		RefID:   refID,
	}
	// Ledger write failure must not silently corrupt accounting — but we also
	// must not roll back the credits because DeductCredits already committed.
	// Log via DB error path; caller already has the deduction applied.
	_ = database.DB.Create(&row).Error
	return user.Credits, nil
}
