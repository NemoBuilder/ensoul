package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/ensoul-labs/ensoul-server/config"
	"github.com/ensoul-labs/ensoul-server/models"
	"github.com/ensoul-labs/ensoul-server/services"
	"github.com/ensoul-labs/ensoul-server/util"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// backfill_seed re-generates seed_summary and dimensions for existing shells
// that have poor-quality seed data (generated from mock Twitter fallback).
//
// Usage:
//   go run cmd/backfill_seed/main.go                  # dry-run, preview only
//   go run cmd/backfill_seed/main.go -apply            # actually write to DB
//   go run cmd/backfill_seed/main.go -apply -handle elonmusk  # single shell
//   go run cmd/backfill_seed/main.go -apply -all       # re-seed ALL shells
//
// Detection heuristic: a shell has bad seed if its seed_summary contains
// "API not configured", "no information", "Mock tweet", "pending LLM analysis",
// or is shorter than 30 characters.

func main() {
	apply := flag.Bool("apply", false, "Actually write changes to DB (default: dry-run)")
	handle := flag.String("handle", "", "Re-seed a specific handle only")
	all := flag.Bool("all", false, "Re-seed ALL shells, not just bad ones")
	flag.Parse()

	util.InitLogger("debug")

	cfg := config.Load()

	if cfg.LLMAPIKey == "" {
		log.Fatal("LLM_API_KEY must be set to regenerate seeds")
	}

	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL()), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	log.Println("Connected to database")

	// Query shells to backfill
	var shells []models.Shell
	query := db.Order("created_at ASC")

	if *handle != "" {
		query = query.Where("handle = ?", *handle)
	} else if !*all {
		// Only target shells with bad seed data
		query = query.Where(`
			seed_summary ILIKE '%API not configured%'
			OR seed_summary ILIKE '%no information%'
			OR seed_summary ILIKE '%Mock tweet%'
			OR seed_summary ILIKE '%pending LLM%'
			OR seed_summary ILIKE '%LLM analysis unavailable%'
			OR seed_summary ILIKE '%Bio not available%'
			OR LENGTH(seed_summary) < 30
			OR seed_summary = ''
			OR seed_summary IS NULL
		`)
	}

	if err := query.Find(&shells).Error; err != nil {
		log.Fatalf("Failed to query shells: %v", err)
	}

	if len(shells) == 0 {
		log.Println("No shells need backfilling")
		return
	}

	log.Printf("Found %d shell(s) to backfill (apply=%v)\n", len(shells), *apply)
	fmt.Println("─────────────────────────────────────────────────────")

	success := 0
	failed := 0

	for i, s := range shells {
		log.Printf("[%d/%d] @%s (current seed: %.60s...)\n", i+1, len(shells), s.Handle, truncate(s.SeedSummary, 60))

		// Generate new seed via LLM (uses public knowledge if no Twitter API)
		preview, err := services.GenerateSeedPreview(s.Handle)
		if err != nil {
			log.Printf("  ✗ Failed: %v\n", err)
			failed++
			continue
		}

		log.Printf("  ✓ New seed: %.80s...\n", truncate(preview.SeedSummary, 80))

		// Show dimension scores
		for dim, data := range preview.Dimensions {
			log.Printf("    %s: %d — %s\n", dim, data.Score, truncate(data.Summary, 50))
		}

		// Show twitter_meta (avatar, banner, followers, etc.)
		if preview.TwitterMeta != nil {
			log.Printf("  📋 TwitterMeta:")
			for k, v := range preview.TwitterMeta {
				log.Printf("    %s: %v\n", k, v)
			}
		}

		if *apply {
			// Serialize dimensions to JSON for the JSONB column
			dimJSON, err := json.Marshal(preview.Dimensions)
			if err != nil {
				log.Printf("  ✗ Failed to marshal dimensions: %v\n", err)
				failed++
				continue
			}

			// Serialize twitter_meta (banner, followers, bio, location, etc.)
			metaJSON, err := json.Marshal(preview.TwitterMeta)
			if err != nil {
				log.Printf("  ✗ Failed to marshal twitter_meta: %v\n", err)
				failed++
				continue
			}

			err = db.Model(&models.Shell{}).Where("id = ?", s.ID).Updates(map[string]interface{}{
				"seed_summary": preview.SeedSummary,
				"dimensions":   json.RawMessage(dimJSON),
				"display_name": preview.DisplayName,
				"avatar_url":   preview.AvatarURL,
				"twitter_meta": json.RawMessage(metaJSON),
			}).Error

			if err != nil {
				log.Printf("  ✗ DB update failed: %v\n", err)
				failed++
				continue
			}
			log.Printf("  ✓ Saved to DB\n")
		} else {
			log.Printf("  (dry-run, not saved)\n")
		}

		success++

		// Rate limit: avoid hammering the LLM API
		if i < len(shells)-1 {
			time.Sleep(2 * time.Second)
		}
	}

	fmt.Println("─────────────────────────────────────────────────────")
	log.Printf("Done. Success: %d, Failed: %d, Total: %d\n", success, failed, len(shells))
	if !*apply && success > 0 {
		log.Println("Run with -apply to write changes to the database.")
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
