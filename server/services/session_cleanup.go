package services

import (
	"time"

	"github.com/ensoul-labs/ensoul-server/database"
	"github.com/ensoul-labs/ensoul-server/models"
	"github.com/ensoul-labs/ensoul-server/util"
)

// StartSessionCleanup periodically removes expired wallet sessions from the database.
func StartSessionCleanup(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			cleanExpiredSessions()
		}
	}()
	util.Log.Info("[cleanup] Expired session cleanup started (every %v)", interval)
}

func cleanExpiredSessions() {
	result := database.DB.Where("expires_at < ?", time.Now()).Delete(&models.WalletSession{})
	if result.RowsAffected > 0 {
		util.Log.Debug("[cleanup] Removed %d expired sessions", result.RowsAffected)
	}
}
