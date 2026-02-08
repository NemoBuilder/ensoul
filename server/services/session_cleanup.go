package services

import (
	"log"
	"time"

	"github.com/ensoul-labs/ensoul-server/database"
	"github.com/ensoul-labs/ensoul-server/models"
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
	log.Printf("[cleanup] Expired session cleanup started (every %v)", interval)
}

func cleanExpiredSessions() {
	result := database.DB.Where("expires_at < ?", time.Now()).Delete(&models.WalletSession{})
	if result.RowsAffected > 0 {
		log.Printf("[cleanup] Removed %d expired sessions", result.RowsAffected)
	}
}
