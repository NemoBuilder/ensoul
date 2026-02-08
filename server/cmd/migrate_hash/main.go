package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"

	"github.com/ensoul-labs/ensoul-server/config"
	"github.com/ensoul-labs/ensoul-server/database"
)

// migrate_hash_secrets migrates plaintext API keys and session tokens to SHA-256 hashes.
//
// Run ONCE before deploying the new code:
//   go run cmd/migrate_hash/main.go
//
// This script:
//   1. Reads all Claw rows that still have a plaintext api_key column
//   2. Computes SHA-256(api_key) and stores it in a new api_key_hash column
//   3. Clears the old api_key column
//   4. Does the same for wallet_sessions.token → token_hash
//
// IMPORTANT: After this migration, the original API keys are gone from the DB.
// Users must already have their keys saved. New registrations return the key once.

func hashToken(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func main() {
	cfg := config.Load()
	database.Connect(cfg)

	db := database.DB

	// --- Step 1: Add new columns if they don't exist ---
	log.Println("Step 1: Ensuring new columns exist...")

	db.Exec(`ALTER TABLE claws ADD COLUMN IF NOT EXISTS api_key_hash VARCHAR(64)`)
	db.Exec(`ALTER TABLE wallet_sessions ADD COLUMN IF NOT EXISTS token_hash VARCHAR(64)`)

	// --- Step 2: Migrate Claw API keys ---
	log.Println("Step 2: Migrating Claw API keys...")

	type ClawRow struct {
		ID     string
		APIKey string
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
		Token string
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

	// --- Step 4: Create unique indexes on new columns ---
	log.Println("Step 4: Creating indexes...")
	db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_claws_api_key_hash ON claws(api_key_hash) WHERE api_key_hash IS NOT NULL AND api_key_hash != ''`)
	db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_wallet_sessions_token_hash ON wallet_sessions(token_hash) WHERE token_hash IS NOT NULL AND token_hash != ''`)

	// --- Step 5: Optionally drop old columns (commented out for safety) ---
	// Uncomment these after verifying the migration worked:
	// db.Exec(`ALTER TABLE claws DROP COLUMN IF EXISTS api_key`)
	// db.Exec(`ALTER TABLE wallet_sessions DROP COLUMN IF EXISTS token`)

	fmt.Println()
	log.Println("✅ Migration complete!")
	log.Println("   Old plaintext values have been cleared (set to empty string).")
	log.Println("   You can drop the old columns after verifying everything works:")
	log.Println("   ALTER TABLE claws DROP COLUMN IF EXISTS api_key;")
	log.Println("   ALTER TABLE wallet_sessions DROP COLUMN IF EXISTS token;")
}
