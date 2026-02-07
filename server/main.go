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
)

func main() {
	// Load configuration
	cfg := config.Load()

	// Connect to database and run migrations
	database.Connect(cfg)

	// Initialize blockchain client and ERC-8004 contract bindings
	if err := chain.Init(); err != nil {
		log.Printf("WARNING: Chain initialization failed (on-chain features disabled): %v", err)
	}

	// Start background agent_id backfill (checks every 2 minutes)
	services.StartAgentIDBackfill(2 * time.Minute)

	// Setup routes
	r := router.Setup()

	// Start server
	addr := fmt.Sprintf(":%s", cfg.Port)
	log.Printf("Ensoul server starting on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
