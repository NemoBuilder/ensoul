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

	// Initialize blockchain client and ERC-8004 contract bindings.
	// If the first attempt fails (e.g. RPC timeout), retries in background
	// with exponential backoff so the server can start serving immediately.
	chain.InitWithRetry(10)

	// Start background agent_id backfill (checks every 2 minutes)
	services.StartAgentIDBackfill(2 * time.Minute)

	// Start expired session cleanup (runs every hour)
	services.StartSessionCleanup(1 * time.Hour)

	// Start pending shell cleanup (checks every 5 min, deletes pending > 30 min)
	services.StartPendingShellCleanup(5 * time.Minute)

	// Start mining pool daily release reset (resets at 00:00 UTC)
	services.StartMiningDailyReset()

	// Start failed reward auto-retry (every 10 minutes, max 5 retries)
	services.StartRewardRetryScheduler()

	// Start Crab demand publisher (every 6 hours)
	services.StartCrabScheduler(6 * time.Hour)

	// Start tax wallet auto-mint scheduler (weekly, Monday 00:00 UTC)
	services.StartTaxWalletScheduler()

	// Start crypto payment pending-verifier (every 5 minutes)
	services.StartCryptoPaymentRefresh(5 * time.Minute)

	// Vibe Write v1 background workers removed in V3 cleanup (D Stage 2):
	//   - StartVibeWriteMonitor (KOL tweet monitor)
	//   - InitSSEHub / StartFeedRefresher (tag-based feed)
	//   - StartSubscriptionCleanup (legacy Subscription model superseded by User.ProExpiresAt)

	// Start monthly revenue settlement (1st of each month, 00:00 UTC)
	services.StartMonthlySettlement()

	// Setup routes
	r := router.Setup()

	// Start server
	addr := fmt.Sprintf(":%s", cfg.Port)
	util.Log.Info("Ensoul server starting on %s (env=%s, log=%s)", addr, cfg.Env, cfg.LogLevel)
	if err := r.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
