package database

import (
	"github.com/ensoul-labs/ensoul-server/config"
	"github.com/ensoul-labs/ensoul-server/models"
	"github.com/ensoul-labs/ensoul-server/util"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// DB is the global database instance.
var DB *gorm.DB

// Connect initializes the database connection and runs auto-migration.
func Connect(cfg *config.Config) *gorm.DB {
	var err error

	// Use Warn-level GORM logging in production (suppress SQL query dumps)
	gormLogLevel := logger.Info
	if cfg.IsProduction() {
		gormLogLevel = logger.Warn
	}

	DB, err = gorm.Open(postgres.Open(cfg.DatabaseURL()), &gorm.Config{
		Logger: logger.Default.LogMode(gormLogLevel),
	})
	if err != nil {
		util.Log.Fatal("Failed to connect to database: %v", err)
	}

	util.Log.Info("Database connected successfully")

	// gen_random_uuid() is built into PostgreSQL 13+, no extension needed.
	// For PostgreSQL 12 or earlier, uncomment the next line:
	// DB.Exec("CREATE EXTENSION IF NOT EXISTS \"pgcrypto\"")

	// Auto-migrate all models
	if err := DB.AutoMigrate(
		&models.Shell{},
		&models.Fragment{},
		&models.Claw{},
		&models.Ensouling{},
		&models.WalletSession{},
		&models.ClawBinding{},
		&models.ChatSession{},
		&models.ChatMessage{},
	); err != nil {
		util.Log.Fatal("Failed to migrate database: %v", err)
	}

	util.Log.Info("Database migration completed")

	// Step 1: Soft-delete case-insensitive duplicate handles FIRST (while they still
	// have distinct values like "X" vs "x"), to avoid unique constraint violations.
	cleanupDuplicateHandles()

	// Step 2: Now that duplicates are removed, normalize all remaining handles
	// to lowercase. "VitalikButerin" → "vitalikbuterin", etc.
	normalizeHandlesToLower()

	return DB
}

// normalizeHandlesToLower converts all shell handles to lowercase in-place.
// Twitter handles are case-insensitive, so "VitalikButerin" → "vitalikbuterin".
// This is idempotent: if all handles are already lowercase, no rows are updated.
func normalizeHandlesToLower() {
	result := DB.Exec(`UPDATE shells SET handle = LOWER(handle) WHERE handle != LOWER(handle) AND deleted_at IS NULL`)
	if result.RowsAffected > 0 {
		util.Log.Info("Normalized %d shell handles to lowercase", result.RowsAffected)
	}
}

// cleanupDuplicateHandles soft-deletes shell records that are case-insensitive
// duplicates. For each group of duplicates, the oldest record (smallest ID) is
// kept and the rest are soft-deleted.
func cleanupDuplicateHandles() {
	type dup struct {
		LowerHandle string
		Cnt         int
	}
	var dups []dup
	DB.Raw(`
		SELECT LOWER(handle) AS lower_handle, COUNT(*) AS cnt
		FROM shells
		WHERE deleted_at IS NULL
		GROUP BY LOWER(handle)
		HAVING COUNT(*) > 1
	`).Scan(&dups)

	if len(dups) == 0 {
		return
	}

	util.Log.Info("Found %d duplicate handle groups, cleaning up...", len(dups))

	for _, d := range dups {
		// Find all shells with this lower-case handle, ordered by created_at ASC
		var shells []models.Shell
		DB.Unscoped().
			Where("LOWER(handle) = ? AND deleted_at IS NULL", d.LowerHandle).
			Order("created_at ASC").
			Find(&shells)

		if len(shells) <= 1 {
			continue
		}

		// Keep the first (oldest), soft-delete the rest
		keep := shells[0]
		for _, s := range shells[1:] {
			util.Log.Info("Soft-deleting duplicate shell: %s (id=%s), keeping: %s (id=%s)",
				s.Handle, s.ID, keep.Handle, keep.ID)
			DB.Delete(&s) // GORM soft delete: sets deleted_at
		}
	}

	util.Log.Info("Duplicate handle cleanup completed")
}
