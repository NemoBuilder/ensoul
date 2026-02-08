package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"

	"github.com/ensoul-labs/ensoul-server/config"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// migrate_hash_secrets migrates plaintext API keys and session tokens to SHA-256 hashes.
//
// Run ONCE before deploying the new code:
//
//	go run cmd/migrate_hash/main.go
//
// This script connects directly to the DB (bypassing AutoMigrate) and:
//  1. Adds api_key_hash / token_hash columns (nullable) if missing
//  2. Reads plaintext values, computes SHA-256, writes hashes
//  3. Clears plaintext columns
//  4. Sets NOT NULL constraint on the new columns
//
// IMPORTANT: After this migration, the original API keys are gone from the DB.
// Users must already have their keys saved. New registrations return the key once.

func hashToken(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func main() {
	cfg := config.Load()

	// Connect directly — do NOT use database.Connect() which runs AutoMigrate
	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL()), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	log.Println("Connected to database (migration mode, no AutoMigrate)")

	// --- Step 1: Add new columns if they don't exist (NULLABLE first!) ---
	log.Println("Step 1: Ensuring new columns exist (nullable)...")

	db.Exec(`ALTER TABLE claws ADD COLUMN IF NOT EXISTS api_key_hash VARCHAR(64)`)
	db.Exec(`ALTER TABLE wallet_sessions ADD COLUMN IF NOT EXISTS token_hash VARCHAR(64)`)

	// --- Step 2: Migrate Claw API keys ---
	log.Println("Step 2: Migrating Claw API keys...")

	type ClawRow struct {
		ID     string
		APIKey string `gorm:"column:api_key"`
	}
	var claws []ClawRow
	db.Raw(`SELECT id, api_key FROM claws WHERE api_key IS NOT NULL AND api_key != '' AND (api_key_hash IS NULL OR api_key_hash = '')`).Scan(&claws)

	migrated := 0
	for _, c := range claws {
		h := hashToken(c.APIKey)
		result := db.Exec(`UPDATE claws SET api_key_hash = ?, api_key = '' WHERE id = ?`, h, c.ID)
		if result.Error != nil {
			log.Printf("  ERROR migrating claw %s: %v", c.ID, result.Error)
		} else {
			migrated++
		}
	}
	log.Printf("  Migrated %d / %d Claw API keys", migrated, len(claws))

	// --- Step 3: Migrate Session tokens ---
	log.Println("Step 3: Migrating Session tokens...")

	type SessionRow struct {
		ID    string
		Token string `gorm:"column:token"`
	}
	var sessions []SessionRow
	db.Raw(`SELECT id, token FROM wallet_sessions WHERE token IS NOT NULL AND token != '' AND (token_hash IS NULL OR token_hash = '')`).Scan(&sessions)

	migrated = 0
	for _, s := range sessions {
		h := hashToken(s.Token)
		result := db.Exec(`UPDATE wallet_sessions SET token_hash = ?, token = '' WHERE id = ?`, h, s.ID)
		if result.Error != nil {
			log.Printf("  ERROR migrating session %s: %v", s.ID, result.Error)
		} else {
			migrated++
		}
	}
	log.Printf("  Migrated %d / %d Session tokens", migrated, len(sessions))

	// --- Step 4: Set NOT NULL + unique index on new columns ---
	log.Println("Step 4: Setting constraints...")

	// Fill any remaining NULLs with a placeholder (shouldn't exist but safety first)
	db.Exec(`UPDATE claws SET api_key_hash = 'migrated_empty_' || id WHERE api_key_hash IS NULL OR api_key_hash = ''`)
	db.Exec(`UPDATE wallet_sessions SET token_hash = 'migrated_empty_' || id WHERE token_hash IS NULL OR token_hash = ''`)

	// Now safe to set NOT NULL
	db.Exec(`ALTER TABLE claws ALTER COLUMN api_key_hash SET NOT NULL`)
	db.Exec(`ALTER TABLE wallet_sessions ALTER COLUMN token_hash SET NOT NULL`)

	// Create unique indexes
	db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_claws_api_key_hash ON claws(api_key_hash)`)
	db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_wallet_sessions_token_hash ON wallet_sessions(token_hash)`)

	// --- Step 5: Drop old columns (safe since data is migrated) ---
	log.Println("Step 5: Dropping old plaintext columns...")
	db.Exec(`ALTER TABLE claws DROP COLUMN IF EXISTS api_key`)
	db.Exec(`ALTER TABLE wallet_sessions DROP COLUMN IF EXISTS token`)

	fmt.Println()
	log.Println("✅ Migration complete!")
	log.Println("   Plaintext api_key and token columns have been dropped.")
	log.Println("   You can now start the server with the new code.")
}
