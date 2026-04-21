// Package main is a CLI to manually seed/upgrade mentor methodology packs.
//
// Usage:
//
//	go run ./cmd/seed_methodology              # seed if pack not yet present (idempotent)
//	go run ./cmd/seed_methodology --force      # overwrite same-source records
//	go run ./cmd/seed_methodology --dir <path> # custom pack dir
//
// In normal operation the server auto-seeds on startup; this CLI is only
// needed for forced upgrades or non-standard environments.
package main

import (
	"flag"
	"log"
	"path/filepath"

	"github.com/ensoul-labs/ensoul-server/config"
	"github.com/ensoul-labs/ensoul-server/models"
	"github.com/ensoul-labs/ensoul-server/services/methodology"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	defaults := methodology.DefaultPack()
	var (
		dir       = flag.String("dir", defaults.Dir, "methodology pack directory")
		source    = flag.String("source", defaults.Source, "source attribution tag")
		sourceURL = flag.String("source-url", defaults.SourceURL, "source URL")
		locale    = flag.String("locale", defaults.Locale, "locale code")
		version   = flag.String("version", defaults.Version, "version tag")
		force     = flag.Bool("force", false, "overwrite existing records of same source")
	)
	flag.Parse()

	abs, err := filepath.Abs(*dir)
	if err != nil {
		log.Fatalf("resolve dir: %v", err)
	}
	log.Printf("Seeding pack: %s (source=%s, locale=%s, force=%v)", abs, *source, *locale, *force)

	cfg := config.Load()
	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL()), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		log.Fatalf("connect db: %v", err)
	}
	if err := db.AutoMigrate(&models.MentorMethodology{}); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	res, err := methodology.SeedPack(db, methodology.PackSpec{
		Dir:       abs,
		Source:    *source,
		SourceURL: *sourceURL,
		Locale:    *locale,
		Version:   *version,
	}, *force)
	if err != nil {
		log.Fatalf("seed: %v", err)
	}
	if res.Skipped {
		log.Printf("Skipped: %s", res.Reason)
		return
	}
	log.Printf("Done. Inserted=%d, Updated=%d", res.Inserted, res.Updated)
}
