package services

import (
	"fmt"
	"time"

	"github.com/ensoul-labs/ensoul-server/database"
	"github.com/ensoul-labs/ensoul-server/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Credit costs per operation
const (
	CreditCostMessage      = 1
	CreditCostLongTweet    = 3
	CreditCostVariant3     = 3
	CreditCostVariant5     = 5
	CreditCostSoulContext  = 1 // additional
	CreditCostBatchGenerate = 15

	FreeCreditsPerMonth = 50
	ProCreditsPerMonth  = 5000
)

// DeductCredits attempts to deduct credits from the user's balance.
// Returns nil on success, error if insufficient credits.
func DeductCredits(userID uuid.UUID, amount int) error {
	return database.DB.Transaction(func(tx *gorm.DB) error {
		var user models.User
		if err := tx.Clauses().Where("id = ?", userID).First(&user).Error; err != nil {
			return fmt.Errorf("user not found")
		}

		// Check if credits need monthly reset
		if time.Now().After(user.CreditsReset) {
			resetCredits := FreeCreditsPerMonth
			if user.IsPro() {
				resetCredits = ProCreditsPerMonth
			}
			user.Credits = resetCredits
			user.CreditsReset = time.Now().Truncate(24 * time.Hour).AddDate(0, 1, 0)
			tx.Model(&user).Updates(map[string]interface{}{
				"credits":       user.Credits,
				"credits_reset": user.CreditsReset,
			})
		}

		if user.Credits < amount {
			return fmt.Errorf("insufficient credits: have %d, need %d", user.Credits, amount)
		}

		result := tx.Model(&models.User{}).Where("id = ? AND credits >= ?", userID, amount).
			Update("credits", gorm.Expr("credits - ?", amount))
		if result.RowsAffected == 0 {
			return fmt.Errorf("insufficient credits")
		}
		return nil
	})
}

// GetCreditsInfo returns the user's current credits info.
func GetCreditsInfo(userID uuid.UUID) (credits int, resetAt time.Time, isPro bool, err error) {
	var user models.User
	if err = database.DB.First(&user, "id = ?", userID).Error; err != nil {
		return
	}

	// Auto-reset if past reset date
	if time.Now().After(user.CreditsReset) {
		resetCredits := FreeCreditsPerMonth
		if user.IsPro() {
			resetCredits = ProCreditsPerMonth
		}
		nextReset := time.Now().Truncate(24 * time.Hour).AddDate(0, 1, 0)
		database.DB.Model(&user).Updates(map[string]interface{}{
			"credits":       resetCredits,
			"credits_reset": nextReset,
		})
		return resetCredits, nextReset, user.IsPro(), nil
	}

	return user.Credits, user.CreditsReset, user.IsPro(), nil
}
