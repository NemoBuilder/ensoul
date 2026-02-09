package main

import (
	"fmt"
	"log"
	"time"

	"github.com/ensoul-labs/ensoul-server/chain"
	"github.com/ensoul-labs/ensoul-server/config"
	"github.com/ensoul-labs/ensoul-server/database"
	"github.com/ensoul-labs/ensoul-server/router"
	"github.com/ensoul-labs/ensoul-server/services"
	"github.com/ensoul-labs/ensoul-server/util"
)

func main() {
	// Load configuration
	cfg := config.Load()

	// Initialize leveled logger (respects ENV and LOG_LEVEL)
	util.InitLogger(cfg.LogLevel)

	// Connect to database and run migrations
	database.Connect(cfg)

	// Initialize blockchain client and ERC-8004 contract bindings
	if err := chain.Init(); err != nil {
		util.Log.Warn("Chain initialization failed (on-chain features disabled): %v", err)
	}

	// Start background agent_id backfill (checks every 2 minutes)
	services.StartAgentIDBackfill(2 * time.Minute)

	// Start expired session cleanup (runs every hour)
	services.StartSessionCleanup(1 * time.Hour)

	// Start pending shell cleanup (checks every 5 min, deletes pending > 30 min)
	services.StartPendingShellCleanup(5 * time.Minute)

	// Setup routes
	r := router.Setup()

	// Start server
	addr := fmt.Sprintf(":%s", cfg.Port)
	util.Log.Info("Ensoul server starting on %s (env=%s, log=%s)", addr, cfg.Env, cfg.LogLevel)
	if err := r.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
