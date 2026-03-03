package router

import (
	"net/http"

	"github.com/ensoul-labs/ensoul-server/config"
	"github.com/ensoul-labs/ensoul-server/handlers"
	"github.com/ensoul-labs/ensoul-server/middleware"
	"github.com/ensoul-labs/ensoul-server/models"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// Setup creates and configures the Gin router with all routes.
func Setup() *gin.Engine {
	// #6: Use release mode in production (suppresses debug logs, route dumps)
	if config.Cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.Default()

	// #7: Trust only loopback proxies (Nginx on same machine)
	r.SetTrustedProxies([]string{"127.0.0.1", "::1"})

	// CORS configuration
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:3000", "http://localhost:3410", "https://ensoul.ac", "https://www.ensoul.ac"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "X-Wallet-Address", "X-Wallet-Signature"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))

	// Health check
	r.GET("/api/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"service": "ensoul-server",
		})
	})

	api := r.Group("/api")
	{
		// Shell (Soul) endpoints
		shell := api.Group("/shell")
		{
			shell.GET("/mint-quota", handlers.ShellMintQuota)
			shell.GET("/mint-price", handlers.ShellMintPrice)
			shell.POST("/preview", middleware.RateLimit(middleware.GeneralLimiter), handlers.ShellPreview)
			shell.POST("/mint", middleware.RateLimit(middleware.RegisterLimiter), handlers.ShellMint)
			shell.POST("/mint-permit", middleware.RateLimit(middleware.GeneralLimiter), handlers.ShellMintPermit)
			shell.POST("/confirm", middleware.RateLimit(middleware.GeneralLimiter), handlers.ShellConfirmMint)
			shell.POST("/cancel", middleware.RateLimit(middleware.GeneralLimiter), handlers.ShellCancelMint)
			shell.GET("/list", handlers.ShellList)
			shell.GET("/:handle", handlers.ShellGetByHandle)
			shell.GET("/:handle/dimensions", handlers.ShellGetDimensions)
			shell.GET("/:handle/history", handlers.ShellGetHistory)
			shell.GET("/:handle/contributors", handlers.ShellContributors)
		}

		// Fragment endpoints
		fragment := api.Group("/fragment")
		{
			// [DEPRECATED] Single submit - returns 410 Gone, directing clients to /batch
			fragment.POST("/submit",
				middleware.RateLimit(middleware.SubmitLimiter),
				middleware.AuthClaw(),
				middleware.RequireClaimed(),
				handlers.FragmentSubmit,
			)
			// Batch submit: 3-6 dimensions per request, same 5-min cooldown per Claw
			fragment.POST("/batch",
				middleware.RateLimit(middleware.SubmitLimiter),
				middleware.AuthClaw(),
				middleware.RequireClaimed(),
				middleware.RateLimitByKey(middleware.ClawSubmitLimiter, func(c *gin.Context) string {
					if claw, exists := c.Get("claw"); exists {
						if cl, ok := claw.(*models.Claw); ok {
							return "claw:" + cl.ID.String()
						}
					}
					return ""
				}),
				handlers.FragmentBatch,
			)
			// List and get are public
			fragment.GET("/list", handlers.FragmentList)
			fragment.GET("/:id", handlers.FragmentGetByID)
		}

		// Claw endpoints
		claw := api.Group("/claw")
		{
			// Public endpoints
			claw.GET("/leaderboard", handlers.ClawLeaderboard)
			claw.GET("/profile/:id", handlers.ClawPublicProfile)
			// Registration is public (rate limited)
			claw.POST("/register", middleware.RateLimit(middleware.RegisterLimiter), handlers.ClawRegister)
			// Claim info is public (accessed via claim URL)
			claw.GET("/claim/:code", handlers.ClawClaimInfo)
			// Claim verification requires wallet session (so we can auto-bind)
			claw.POST("/claim/verify", middleware.AuthSession(), handlers.ClawClaimVerify)
			// These require Claw API key authentication
			claw.GET("/status", middleware.AuthClaw(), handlers.ClawStatus)
			claw.GET("/me", middleware.AuthClaw(), handlers.ClawMe)
			claw.GET("/dashboard", middleware.AuthClaw(), handlers.ClawDashboard)
			claw.GET("/contributions", middleware.AuthClaw(), handlers.ClawContributions)
			// Session-based Claw key management (bound to wallet)
			claw.POST("/keys", middleware.AuthSession(), handlers.ClawBindKey)
			claw.GET("/keys", middleware.AuthSession(), handlers.ClawListKeys)
			claw.DELETE("/keys/:id", middleware.AuthSession(), handlers.ClawUnbindKey)
			claw.GET("/keys/:id/dashboard", middleware.AuthSession(), handlers.ClawBoundDashboard)
			// Withdraw endpoints (session-based)
			claw.GET("/withdraw/check", middleware.AuthSession(), handlers.ClawWithdrawCheck)
			claw.POST("/withdraw", middleware.AuthSession(), handlers.ClawWithdraw)
			claw.GET("/withdraw/history", middleware.AuthSession(), handlers.ClawWithdrawHistory)
		}

		// Auth endpoints (wallet signature login)
		auth := api.Group("/auth")
		{
			auth.POST("/login", middleware.RateLimit(middleware.GeneralLimiter), handlers.AuthLogin)
			auth.POST("/logout", handlers.AuthLogout)
			auth.GET("/session", handlers.AuthSession)
		}

		// Chat endpoints
		chat := api.Group("/chat")
		{
			// Create a new session (public, but links to wallet if logged in)
			chat.POST("/:handle/session", middleware.RateLimit(middleware.SessionLimiter), handlers.ChatCreateSession)
			// Send message in a session (public, streams SSE — rate limited per IP)
			chat.POST("/sessions/:id/message", middleware.RateLimit(middleware.ChatLimiter), handlers.ChatSendMessage)
			// Get session with messages (public for guest sessions, owner-only for user sessions)
			chat.GET("/sessions/:id", handlers.ChatGetSession)
			// List user's sessions (requires login)
			chat.GET("/sessions", middleware.AuthSession(), handlers.ChatListSessions)
			// Delete a session (requires login + ownership)
			chat.DELETE("/sessions/:id", middleware.AuthSession(), handlers.ChatDeleteSession)
			// Share: create a public share link
			chat.POST("/share", middleware.RateLimit(middleware.GeneralLimiter), handlers.ChatCreateShare)
			// Share: get a public share by code (no auth)
			chat.GET("/share/:code", handlers.ChatGetShare)
		}

		// Stats endpoint — public
		api.GET("/stats", handlers.GetStats)

		// Task board — public
		api.GET("/tasks", handlers.GetTasks)

		// Economy dashboard — public
		api.GET("/economy/overview", handlers.EconomyOverview)

		// Mining endpoints (economic system)
		mining := api.Group("/mining")
		{
			mining.GET("/pool", handlers.MiningPoolStatus)
			mining.GET("/demands", handlers.MiningDemands)
			mining.GET("/rewards/:claw_id", handlers.MiningRewards)
		}

		// Soul Sniper endpoints (Phase 3 → Sniper 2.0)
		sniper := api.Group("/sniper")
		{
			// === Public endpoints (no auth) ===
			sniper.GET("/tags", handlers.SniperGetTags)
			sniper.GET("/feed", handlers.SniperGetFeed)
			sniper.GET("/feed/stream", handlers.SniperFeedStream)
			sniper.GET("/feed/refresh", handlers.SniperFeedRefresh)

			// === Session-required endpoints ===
			sniper.GET("/user/tags", middleware.AuthSession(), handlers.SniperGetUserTags)
			sniper.PUT("/user/tags", middleware.AuthSession(), handlers.SniperUpdateUserTags)
			sniper.GET("/user/muted", middleware.AuthSession(), handlers.SniperGetMuted)
			sniper.POST("/user/muted", middleware.AuthSession(), handlers.SniperMuteAccount)
			sniper.DELETE("/user/muted/:handle", middleware.AuthSession(), handlers.SniperUnmuteAccount)

			// Snipe: generate reply (Pro only)
			sniper.POST("/snipe", middleware.RateLimit(middleware.GeneralLimiter), middleware.AuthSession(), handlers.SniperSnipe)

			// Subscription management (kept from v1)
			sniper.GET("/subscribe-price", handlers.SniperSubscribePrice)
			sniper.POST("/subscribe", middleware.AuthSession(), handlers.SniperSubscribe)
			sniper.GET("/subscription", middleware.AuthSession(), handlers.SniperGetSubscription)

			// Reply history (kept from v1)
			sniper.GET("/replies", middleware.AuthSession(), handlers.SniperGetReplies)

			// Persona management (kept from v1)
			sniper.POST("/persona", middleware.AuthSession(), handlers.SniperSetPersona)
			sniper.GET("/persona", middleware.AuthSession(), handlers.SniperGetPersona)

			// Legacy v1 endpoints (deprecated but kept for compatibility)
			sniper.POST("/kols", middleware.AuthSession(), handlers.SniperAddKOL)
			sniper.GET("/kols", middleware.AuthSession(), handlers.SniperListKOLs)
			sniper.DELETE("/kols/:id", middleware.AuthSession(), handlers.SniperRemoveKOL)
			sniper.POST("/reply", middleware.RateLimit(middleware.GeneralLimiter), middleware.AuthSession(), handlers.SniperGenerateReply)
		}

		// Holder revenue endpoints (Phase 4)
		holder := api.Group("/holder")
		{
			holder.GET("/dashboard", middleware.AuthSession(), handlers.HolderDashboard)
			holder.GET("/revenue/:period", middleware.AuthSession(), handlers.HolderRevenuePeriod)
			holder.POST("/claim", middleware.AuthSession(), handlers.HolderClaimRevenue)
		}

		// KOL claim endpoints (Phase 4)
		claim := api.Group("/claim")
		{
			claim.POST("/initiate", middleware.AuthSession(), handlers.ClaimInitiate)
			claim.POST("/verify", middleware.AuthSession(), handlers.ClaimVerify)
			claim.GET("/:handle", handlers.ClaimStatus)
		}

		// Admin authentication (login/logout are public, no admin auth required)
		adminAuth := api.Group("/admin/auth")
		{
			adminAuth.POST("/login", middleware.RateLimit(middleware.GeneralLimiter), handlers.AdminLogin)
			adminAuth.POST("/logout", handlers.AdminLogout)
		}

		// Admin endpoints (protected by ADMIN_API_KEY or admin session cookie)
		admin := api.Group("/admin", middleware.AuthAdmin())
		{
			// Admin session info & password change
			admin.GET("/auth/me", handlers.AdminMe)
			admin.POST("/auth/password", handlers.AdminChangePassword)

			// Mint candidate management
			admin.GET("/candidates", handlers.AdminListCandidates)
			admin.POST("/candidates", handlers.AdminAddCandidate)
			admin.POST("/candidates/batch", handlers.AdminAddCandidatesBatch)
			admin.DELETE("/candidates/:handle", handlers.AdminRemoveCandidate)
			admin.POST("/candidates/refresh-all", handlers.AdminRefreshAllCandidates)
			admin.POST("/candidates/:handle/refresh", handlers.AdminRefreshCandidate)

			// Tax wallet operations
			admin.GET("/tax-wallet/status", handlers.AdminTaxWalletStatus)
			admin.POST("/tax-wallet/mint", handlers.AdminTriggerMint)
			admin.POST("/tax-wallet/mint/:handle", handlers.AdminMintSingle)

			// Mining pool deposit (moved here from mining group)
			admin.POST("/mining/deposit", handlers.MiningDeposit)

			// Mining reward retry (admin only)
			admin.GET("/mining/rewards/failed", handlers.MiningFailedRewards)
			admin.POST("/mining/rewards/:id/retry", handlers.MiningRetryReward)
			admin.POST("/mining/rewards/retry-all", handlers.MiningRetryAll)

			// Sniper 2.0: Tag management
			admin.GET("/sniper/tags", handlers.AdminSniperListTags)
			admin.POST("/sniper/tags", handlers.AdminSniperCreateTag)
			admin.PUT("/sniper/tags/:id", handlers.AdminSniperUpdateTag)
			admin.DELETE("/sniper/tags/:id", handlers.AdminSniperDeleteTag)
			admin.GET("/sniper/tags/:id/accounts", handlers.AdminSniperListTagAccounts)
			admin.POST("/sniper/tags/:id/accounts", handlers.AdminSniperAddTagAccount)
			admin.DELETE("/sniper/tags/:id/accounts/:handle", handlers.AdminSniperRemoveTagAccount)

			// Sniper 2.0: Tag candidate management
			admin.POST("/sniper/candidates/import", handlers.AdminSniperImportCandidates)
			admin.GET("/sniper/candidates", handlers.AdminSniperListCandidates)
			admin.POST("/sniper/candidates/:id/approve", handlers.AdminSniperApproveCandidate)
			admin.POST("/sniper/candidates/:id/reject", handlers.AdminSniperRejectCandidate)
			admin.POST("/sniper/candidates/batch", handlers.AdminSniperBatchReview)
		}
	}

	return r
}
