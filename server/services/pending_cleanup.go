package services

import (
	"time"

	"github.com/ensoul-labs/ensoul-server/database"
	"github.com/ensoul-labs/ensoul-server/models"
	"github.com/ensoul-labs/ensoul-server/util"
	"github.com/google/uuid"
)

// HardDeleteShell removes a shell and all its FK-dependent records permanently.
// Order: chat_messages → chat_sessions → fragments → ensoulings → shell.
func HardDeleteShell(shellID uuid.UUID) {
	// 1. Delete chat messages belonging to sessions of this shell
	database.DB.Unscoped().Exec(
		"DELETE FROM chat_messages WHERE session_id IN (SELECT id FROM chat_sessions WHERE shell_id = ?)", shellID,
	)
	// 2. Delete chat sessions
	database.DB.Unscoped().Where("shell_id = ?", shellID).Delete(&models.ChatSession{})
	// 3. Delete fragments
	database.DB.Unscoped().Where("shell_id = ?", shellID).Delete(&models.Fragment{})
	// 4. Delete ensoulings
	database.DB.Unscoped().Where("shell_id = ?", shellID).Delete(&models.Ensouling{})
	// 5. Delete the shell itself
	database.DB.Unscoped().Where("id = ?", shellID).Delete(&models.Shell{})
}

// StartPendingShellCleanup periodically hard-deletes pending shells
// that were never confirmed on-chain (i.e. the user abandoned the mint).
func StartPendingShellCleanup(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			cleanPendingShells()
		}
	}()
	util.Log.Info("[cleanup] Pending shell cleanup started (every %v, timeout %v)", interval, PendingMintTimeout)
}

func cleanPendingShells() {
	cutoff := time.Now().Add(-PendingMintTimeout)
	var expired []models.Shell
	database.DB.Where("stage = ? AND created_at < ?", models.StagePending, cutoff).Find(&expired)
	if len(expired) == 0 {
		return
	}
	for _, s := range expired {
		HardDeleteShell(s.ID)
		util.Log.Info("[cleanup] Hard-deleted expired pending shell @%s (id=%s)", s.Handle, s.ID)
	}
	util.Log.Info("[cleanup] Cleaned up %d expired pending shells", len(expired))
}
