package database

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/ensoul-labs/ensoul-server/config"
	"github.com/ensoul-labs/ensoul-server/models"
	"github.com/ensoul-labs/ensoul-server/services/methodology"
	"github.com/ensoul-labs/ensoul-server/util"
	"golang.org/x/crypto/bcrypt"
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
		&models.EmailSession{},
		&models.ClawBinding{},
		&models.ChatSession{},
		&models.ChatMessage{},
		&models.ChatShare{},
		// Economic system models
		&models.MiningPool{},
		&models.FragmentDemand{},
		&models.MiningReward{},
		&models.BuybackRecord{},
		&models.PublicSoul{},
		&models.MintCandidate{},
		// Vibe Write models
		&models.Subscription{},
		&models.VibeWriteKOL{},
		&models.VibeWriteReply{},
		&models.UserPersona{},
		// Vibe Write 2.0 tag-based models
		&models.VibeWriteTag{},
		&models.VibeWriteTagAccount{},
		&models.TagCandidate{},
		&models.UserSelectedTag{},
		&models.UserMutedAccount{},
		// Vibe Write 2.0+: Multi-dimensional tagging
		&models.TagDimension{},
		&models.TagDimensionValue{},
		&models.VibeWriteTagDimension{},
		&models.ExternalSnipeUsage{},
		// Holder Revenue & KOL Claim models
		&models.HolderRevenue{},
		&models.RevenuePool{},
		&models.KOLClaim{},
		&models.SoulUsage{},
		&models.UsedPaymentTx{},
		// Withdraw records
		&models.WithdrawRecord{},
		// Admin authentication models
		&models.AdminUser{},
		&models.AdminSession{},
		// User management models
		&models.User{},
		&models.EmailCode{},
		&models.AdminAuditLog{},
		&models.GiftProLog{},
		// Crypto payment (BSC USDT/BNB)
		&models.CryptoPayment{},
		// Vibe Write 2.0 workspace models
		&models.VibeWorkspace{},
		&models.VibeMemory{},
		&models.VibeChat{},
		&models.VibeChatMessage{},
		// Vibe Write 2.0 mentor methodology
		&models.MentorMethodology{},
		// ── V4 Galaxy knowledge-graph protocol ──
		&models.Galaxy{},
		&models.GalaxyRole{},
		&models.Source{},
		&models.Atom{},
		&models.GalaxyApplication{},
		&models.CreditLedger{},
		&models.Epoch{},
		&models.Launch{},
		&models.LaunchDeposit{},
		&models.BuybackEvent{},
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

	// Step 3: Backfill content_hash for existing fragments that were created
	// before the content protection feature. Idempotent: skips fragments
	// that already have a hash.
	backfillContentHashes()

	// Step 4: Seed the initial admin user from environment variables.
	// Idempotent: only creates if no admin user exists yet.
	seedAdminUser(cfg)

	// Step 5: Seed Vibe Write 2.0 default tags and accounts.
	// Idempotent: only creates if no tags exist yet.
	seedVibeWriteTags()

	// Step 6: Backfill User records from existing wallet_sessions.
	// Idempotent: only inserts users that don't exist yet.
	backfillUsers()

	// Step 7: Auto-seed mentor methodology pack if not yet present.
	// Idempotent: skips when records of same source already exist.
	// Override pack location with METHODOLOGY_DIR env var.
	seedMentorMethodology()

	// Step 8: Replace GORM's plain unique indexes on users.email / users.wallet_addr
	// with PARTIAL unique indexes that ignore empty strings and soft-deleted rows.
	// Without this, the second email-only signup collides with the first
	// (both would write wallet_addr = ''), causing a 500 on /api/auth/email/verify.
	ensureUserPartialUniqueIndexes()

	// Step 9: Add user_id columns to legacy business tables and backfill from
	// wallet_addr → users.id. Lets the V3 admin layer treat users.id as the
	// single source of truth across mixed wallet/email signups.
	ensureUserIDColumns()

	return DB
}

// ensureUserIDColumns adds nullable user_id columns to legacy business tables
// (subscriptions, vibe_write_replies, claw_bindings, claws, holder_revenues,
// user_personas, used_payment_tx) and backfills them by joining on wallet_addr.
// Idempotent and safe to run on every startup.
func ensureUserIDColumns() {
	addCols := []string{
		`ALTER TABLE subscriptions      ADD COLUMN IF NOT EXISTS user_id UUID`,
		`ALTER TABLE vibe_write_replies ADD COLUMN IF NOT EXISTS user_id UUID`,
		`ALTER TABLE claw_bindings      ADD COLUMN IF NOT EXISTS user_id UUID`,
		`ALTER TABLE claws              ADD COLUMN IF NOT EXISTS claimed_by_user_id UUID`,
		`ALTER TABLE holder_revenues    ADD COLUMN IF NOT EXISTS user_id UUID`,
		`ALTER TABLE user_personas      ADD COLUMN IF NOT EXISTS user_id UUID`,
		`ALTER TABLE used_payment_tx    ADD COLUMN IF NOT EXISTS user_id UUID`,
	}
	for _, s := range addCols {
		if err := DB.Exec(s).Error; err != nil {
			util.Log.Warn("[migrate] add user_id column failed (non-fatal): %v — sql: %s", err, s)
		}
	}

	backfills := []string{
		`UPDATE subscriptions s SET user_id = u.id FROM users u
			WHERE s.wallet_addr <> '' AND s.wallet_addr = u.wallet_addr AND s.user_id IS NULL`,
		`UPDATE vibe_write_replies r SET user_id = u.id FROM users u
			WHERE r.wallet_addr <> '' AND r.wallet_addr = u.wallet_addr AND r.user_id IS NULL`,
		`UPDATE claw_bindings b SET user_id = u.id FROM users u
			WHERE b.wallet_addr <> '' AND b.wallet_addr = u.wallet_addr AND b.user_id IS NULL`,
		`UPDATE claws c SET claimed_by_user_id = u.id FROM users u
			WHERE c.wallet_addr <> '' AND c.wallet_addr = u.wallet_addr AND c.claimed_by_user_id IS NULL`,
		`UPDATE holder_revenues h SET user_id = u.id FROM users u
			WHERE h.wallet_addr <> '' AND h.wallet_addr = u.wallet_addr AND h.user_id IS NULL`,
		`UPDATE user_personas p SET user_id = u.id FROM users u
			WHERE p.wallet_addr <> '' AND p.wallet_addr = u.wallet_addr AND p.user_id IS NULL`,
		`UPDATE used_payment_tx t SET user_id = u.id FROM users u
			WHERE t.wallet_addr <> '' AND t.wallet_addr = u.wallet_addr AND t.user_id IS NULL`,
	}
	for _, s := range backfills {
		if err := DB.Exec(s).Error; err != nil {
			util.Log.Warn("[migrate] backfill user_id failed (non-fatal): %v — sql: %s", err, s)
		}
	}

	indexes := []string{
		`CREATE INDEX IF NOT EXISTS idx_subscriptions_user      ON subscriptions(user_id)        WHERE user_id IS NOT NULL`,
		`CREATE INDEX IF NOT EXISTS idx_vibe_write_replies_user ON vibe_write_replies(user_id)   WHERE user_id IS NOT NULL`,
		`CREATE INDEX IF NOT EXISTS idx_claw_bindings_user      ON claw_bindings(user_id)        WHERE user_id IS NOT NULL`,
		`CREATE INDEX IF NOT EXISTS idx_claws_claimed_by_user   ON claws(claimed_by_user_id)     WHERE claimed_by_user_id IS NOT NULL`,
		`CREATE INDEX IF NOT EXISTS idx_holder_revenues_user    ON holder_revenues(user_id)      WHERE user_id IS NOT NULL`,
		`CREATE INDEX IF NOT EXISTS idx_user_personas_user      ON user_personas(user_id)        WHERE user_id IS NOT NULL`,
		`CREATE INDEX IF NOT EXISTS idx_used_payment_tx_user    ON used_payment_tx(user_id)      WHERE user_id IS NOT NULL`,
	}
	for _, s := range indexes {
		if err := DB.Exec(s).Error; err != nil {
			util.Log.Warn("[migrate] create user_id index failed (non-fatal): %v — sql: %s", err, s)
		}
	}
	util.Log.Info("[migrate] user_id columns + backfill on business tables ensured")
}

// ensureUserPartialUniqueIndexes drops the legacy non-partial unique indexes on
// users.email and users.wallet_addr (created by an earlier `uniqueIndex` GORM
// tag) and replaces them with partial unique indexes that skip empty strings
// and soft-deleted rows. Idempotent and safe to run on every startup.
func ensureUserPartialUniqueIndexes() {
	stmts := []string{
		// Drop any pre-existing non-partial unique index that GORM may have created
		`DROP INDEX IF EXISTS idx_users_email`,
		`DROP INDEX IF EXISTS idx_users_wallet_addr`,
		// Plain (non-unique) indexes for fast lookups — these match the new
		// `index` GORM tag and may already exist; CREATE IF NOT EXISTS is safe.
		`CREATE INDEX IF NOT EXISTS idx_users_email_lookup    ON users (email)       WHERE deleted_at IS NULL`,
		`CREATE INDEX IF NOT EXISTS idx_users_wallet_lookup   ON users (wallet_addr) WHERE deleted_at IS NULL`,
		// Partial unique indexes — ignore empty values and soft-deleted rows
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email_unique
			ON users (email)
			WHERE email <> '' AND deleted_at IS NULL`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_users_wallet_addr_unique
			ON users (wallet_addr)
			WHERE wallet_addr <> '' AND deleted_at IS NULL`,
	}
	for _, s := range stmts {
		if err := DB.Exec(s).Error; err != nil {
			util.Log.Warn("[migrate] partial-unique-index step failed (non-fatal): %v — sql: %s", err, s)
		}
	}
	util.Log.Info("[migrate] partial unique indexes on users.email / users.wallet_addr ensured")
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

// backfillContentHashes computes SHA-256 content hashes for fragments that
// were created before the content protection feature was added.
// Processes in batches of 500 to avoid memory issues with large datasets.
func backfillContentHashes() {
	var count int64
	DB.Model(&models.Fragment{}).Where("content_hash = '' OR content_hash IS NULL").Count(&count)
	if count == 0 {
		return
	}

	util.Log.Info("Backfilling content_hash for %d fragments...", count)

	batchSize := 500
	updated := 0
	for {
		var fragments []models.Fragment
		DB.Where("content_hash = '' OR content_hash IS NULL").
			Limit(batchSize).Find(&fragments)
		if len(fragments) == 0 {
			break
		}
		for _, f := range fragments {
			h := sha256.Sum256([]byte(f.Content))
			hash := hex.EncodeToString(h[:])
			DB.Model(&f).Update("content_hash", hash)
		}
		updated += len(fragments)
		util.Log.Info("  backfilled %d / %d fragments", updated, count)
	}

	util.Log.Info("Content hash backfill completed: %d fragments updated", updated)
}

// seedAdminUser creates the initial admin account from ADMIN_USERNAME / ADMIN_PASSWORD
// environment variables. This only runs if no AdminUser records exist yet.
// The password is stored as a bcrypt hash. After first run, the env vars can be removed.
func seedAdminUser(cfg *config.Config) {
	if cfg.AdminPassword == "" {
		return // No password configured, skip seeding
	}

	// Check if any admin user exists
	var count int64
	DB.Model(&models.AdminUser{}).Count(&count)
	if count > 0 {
		return // Admin users already exist, skip
	}

	username := cfg.AdminUsername
	if username == "" {
		username = "admin"
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(cfg.AdminPassword), bcrypt.DefaultCost)
	if err != nil {
		util.Log.Error("Failed to hash admin password: %v", err)
		return
	}

	admin := &models.AdminUser{
		Username:     username,
		PasswordHash: string(hash),
		Role:         models.AdminRoleSuperAdmin,
	}
	if err := DB.Create(admin).Error; err != nil {
		util.Log.Error("Failed to seed admin user: %v", err)
		return
	}

	util.Log.Info("Seeded initial admin user: %s (role=%s)", username, models.AdminRoleSuperAdmin)
}

// seedVibeWriteTags creates the default Vibe Write 2.0 tags and their associated accounts.
// Idempotent: only runs if no VibeWriteTag records exist.
func seedVibeWriteTags() {
	var count int64
	DB.Model(&models.VibeWriteTag{}).Count(&count)
	if count > 0 {
		return // Tags already seeded
	}

	util.Log.Info("Seeding Vibe Write 2.0 default tags and accounts...")

	// Tag definitions: {id, name, name_en, icon, category, description, is_default, sort_order}
	type tagDef struct {
		ID          string
		Name        string
		NameEN      string
		Icon        string
		Category    string
		Description string
		IsDefault   bool
		SortOrder   int
	}

	tags := []tagDef{
		// Ecosystem tags
		{"bnb_official", "BNB官方", "BNB Official", "🔶", "ecosystem", "BNB Chain ecosystem official accounts", true, 1},
		{"bnb_kol", "BNB-KOL", "BNB KOLs", "🔶", "ecosystem", "BNB Chain key opinion leaders", false, 2},
		{"sol_official", "SOL官方", "SOL Official", "🟣", "ecosystem", "Solana ecosystem official accounts", true, 3},
		{"sol_kol", "SOL-KOL", "SOL KOLs", "🟣", "ecosystem", "Solana key opinion leaders", false, 4},
		{"base_official", "Base官方", "Base Official", "🔵", "ecosystem", "Base / Coinbase ecosystem official accounts", false, 5},
		{"base_kol", "Base-KOL", "Base KOLs", "🔵", "ecosystem", "Base ecosystem key opinion leaders", false, 6},
		// Track tags
		{"ai_track", "AI赛道", "AI Track", "🤖", "track", "AI + Crypto projects and researchers", true, 10},
		{"defi_track", "DeFi赛道", "DeFi Track", "💰", "track", "DeFi protocols and analysts", false, 11},
		{"prediction", "预测市场", "Prediction Markets", "🎲", "track", "Prediction market protocols", false, 12},
		{"media", "聚合媒体", "Crypto Media", "📰", "track", "Crypto news and media aggregators", false, 13},
	}

	for _, t := range tags {
		tag := models.VibeWriteTag{
			ID:          t.ID,
			Name:        t.Name,
			NameEN:      t.NameEN,
			Icon:        t.Icon,
			Category:    t.Category,
			Description: t.Description,
			IsDefault:   t.IsDefault,
			Active:      true,
			SortOrder:   t.SortOrder,
		}
		DB.Create(&tag)
	}

	// Account definitions: {tag_id, handle, display_name, realtime_priority}
	type acctDef struct {
		TagID            string
		Handle           string
		DisplayName      string
		RealtimePriority bool
	}

	accounts := []acctDef{
		// BNB Official
		{"bnb_official", "bnbchain", "BNB Chain", true},
		{"bnb_official", "BinanceLabs", "Binance Labs", true},
		{"bnb_official", "BNBChainDev", "BNB Chain Dev", false},
		{"bnb_official", "PancakeSwap", "PancakeSwap", false},
		{"bnb_official", "ABORINGZ", "caBoring", false},
		{"bnb_official", "BinanceWallet", "Binance Wallet", false},

		// BNB KOL
		{"bnb_kol", "cz_binance", "CZ", true},
		{"bnb_kol", "haboringz", "Bo", false},
		{"bnb_kol", "BinanceResearch", "Binance Research", false},

		// SOL Official
		{"sol_official", "solana", "Solana", true},
		{"sol_official", "JupiterExchange", "Jupiter", true},
		{"sol_official", "RaydiumProtocol", "Raydium", false},
		{"sol_official", "phantom", "Phantom", false},
		{"sol_official", "MagicEden", "Magic Eden", false},

		// SOL KOL
		{"sol_kol", "0xMert_", "Mert", true},
		{"sol_kol", "aaboringz", "toly", false},

		// Base Official
		{"base_official", "base", "Base", true},
		{"base_official", "coinbase", "Coinbase", true},
		{"base_official", "BuildOnBase", "Build On Base", false},

		// Base KOL
		{"base_kol", "jessepollak", "Jesse Pollak", true},

		// AI Track
		{"ai_track", "ai16zdao", "ai16z", true},
		{"ai_track", "virtuals_io", "Virtuals Protocol", true},
		{"ai_track", "griffaindotcom", "GRIFFAIN", false},
		{"ai_track", "auaboringz", "Autonolas", false},
		{"ai_track", "0xzerebro", "Zerebro", false},

		// DeFi Track
		{"defi_track", "AaveAave", "Aave", true},
		{"defi_track", "Uniswap", "Uniswap", true},
		{"defi_track", "CurveFinance", "Curve", false},
		{"defi_track", "MakerDAO", "Maker", false},
		{"defi_track", "1inch", "1inch", false},

		// Prediction Markets
		{"prediction", "Polymarket", "Polymarket", true},
		{"prediction", "AzuroProtocol", "Azuro", false},

		// Media
		{"media", "CoinDesk", "CoinDesk", true},
		{"media", "WuBlockchain", "Wu Blockchain", true},
		{"media", "BlockBeatsAsia", "BlockBeats", false},
		{"media", "TheBlock__", "The Block", false},
	}

	for _, a := range accounts {
		acct := models.VibeWriteTagAccount{
			TagID:            a.TagID,
			Handle:           a.Handle,
			DisplayName:      a.DisplayName,
			RealtimePriority: a.RealtimePriority,
		}
		DB.Create(&acct)
	}

	util.Log.Info("Seeded %d tags and %d accounts for Vibe Write 2.0", len(tags), len(accounts))
}

// backfillUsers creates User records from existing wallet_sessions and subscriptions.
// Idempotent: uses ON CONFLICT DO NOTHING, safe to run on every startup.
func backfillUsers() {
	result := DB.Exec(`
		INSERT INTO users (id, wallet_addr, status, first_seen_at, last_seen_at, login_count, created_at, updated_at)
		SELECT
			gen_random_uuid(),
			ws.wallet_addr,
			'active',
			MIN(ws.created_at),
			MAX(ws.created_at),
			COUNT(*),
			NOW(),
			NOW()
		FROM wallet_sessions ws
		GROUP BY ws.wallet_addr
		ON CONFLICT (wallet_addr) DO NOTHING
	`)
	if result.RowsAffected > 0 {
		util.Log.Info("Backfilled %d User records from wallet_sessions", result.RowsAffected)
	}

	// Also backfill from subscriptions (users who may have paid but session expired)
	result2 := DB.Exec(`
		INSERT INTO users (id, wallet_addr, status, first_seen_at, last_seen_at, login_count, created_at, updated_at)
		SELECT
			gen_random_uuid(),
			s.wallet_addr,
			'active',
			MIN(s.created_at),
			MAX(s.created_at),
			0,
			NOW(),
			NOW()
		FROM subscriptions s
		WHERE s.deleted_at IS NULL
		GROUP BY s.wallet_addr
		ON CONFLICT (wallet_addr) DO NOTHING
	`)
	if result2.RowsAffected > 0 {
		util.Log.Info("Backfilled %d additional User records from subscriptions", result2.RowsAffected)
	}
}

// seedMentorMethodology auto-seeds the default methodology pack on startup.
// Idempotent: skips if records of same source already exist. To force-update
// after editing markdown sources, use:
//
//	go run ./cmd/seed_methodology --force
func seedMentorMethodology() {
	spec := methodology.DefaultPack()
	res, err := methodology.SeedPack(DB, spec, false)
	if err != nil {
		util.Log.Warn("Mentor methodology auto-seed skipped: %v (use cmd/seed_methodology to bootstrap manually)", err)
		return
	}
	if res.Skipped {
		util.Log.Debug("Mentor methodology already seeded: %s", res.Reason)
		return
	}
	util.Log.Info("Seeded mentor methodology pack %s: inserted=%d updated=%d", spec.Source, res.Inserted, res.Updated)
}
