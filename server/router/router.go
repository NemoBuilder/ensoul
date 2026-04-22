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
			shell.GET("/by-owner/:address", handlers.ShellByOwner)
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
				middleware.RequireMiningApproved(),
				handlers.FragmentSubmit,
			)
			// Batch submit: 3-6 dimensions per request, same 5-min cooldown per Claw
			fragment.POST("/batch",
				middleware.RateLimit(middleware.SubmitLimiter),
				middleware.AuthClaw(),
				middleware.RequireClaimed(),
				middleware.RequireMiningApproved(),
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

			// Email auth endpoints
			auth.POST("/email/send-code", middleware.RateLimit(middleware.RegisterLimiter), handlers.EmailSendCode)
			auth.POST("/email/verify", middleware.RateLimit(middleware.GeneralLimiter), handlers.EmailVerify)
			auth.POST("/email/logout", handlers.EmailLogout)
			auth.GET("/email/session", handlers.EmailSessionInfo)

			// Password auth endpoints
			auth.POST("/email/password-login", middleware.RateLimit(middleware.GeneralLimiter), handlers.PasswordLogin)
			auth.POST("/email/set-password", handlers.PasswordSet)
			auth.GET("/email/has-password", middleware.RateLimit(middleware.GeneralLimiter), handlers.PasswordCheck)

			// Account binding endpoints (cross-link email ↔ wallet on the same User)
			auth.POST("/bind/wallet", middleware.RateLimit(middleware.GeneralLimiter), handlers.AuthBindWallet)
			auth.POST("/bind/email/send", middleware.RateLimit(middleware.RegisterLimiter), handlers.AuthBindEmailSend)
			auth.POST("/bind/email", middleware.RateLimit(middleware.GeneralLimiter), handlers.AuthBindEmail)
		}

		// Billing endpoints (LemonSqueezy)
		billing := api.Group("/billing")
		{
			billing.POST("/checkout", middleware.RateLimit(middleware.GeneralLimiter), handlers.BillingCheckout)
			billing.POST("/webhook", handlers.BillingWebhook) // No auth — verified via signature
			billing.GET("/status", handlers.BillingStatus)
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

		// Vibe Write 2.0 workspace endpoints (email auth required)
		vw := api.Group("/vibe-write")
		{
			// Tweet URL → structured AttachedTweet (used by chat input auto-attach)
			vw.GET("/fetch-tweet", handlers.VibeFetchTweet)

			// Workspaces
			vw.GET("/workspaces", handlers.VibeWorkspaceList)
			vw.POST("/workspaces", handlers.VibeWorkspaceCreate)
			vw.PUT("/workspaces/:id", handlers.VibeWorkspaceUpdate)
			vw.DELETE("/workspaces/:id", handlers.VibeWorkspaceDelete)
			vw.POST("/workspaces/:id/setup", middleware.RateLimit(middleware.GeneralLimiter), handlers.VibeWorkspaceSetup)

			// Memories (per workspace)
			vw.GET("/workspaces/:id/memories", handlers.VibeMemoryList)
			vw.POST("/workspaces/:id/memories", handlers.VibeMemoryCreate)
			vw.POST("/workspaces/:id/memories/import", middleware.RateLimit(middleware.GeneralLimiter), handlers.VibeMemoryImport)
			vw.POST("/workspaces/:id/memories/import-twitter", middleware.RateLimit(middleware.GeneralLimiter), handlers.VibeMemoryImportTwitter)
			vw.PUT("/memories/:memId", handlers.VibeMemoryUpdate)
			vw.DELETE("/memories/:memId", handlers.VibeMemoryDelete)
			vw.POST("/memories/:memId/review", handlers.VibeMemoryReview)

			// Chats (per workspace)
			vw.GET("/workspaces/:id/chats", handlers.VibeChatList)
			vw.POST("/workspaces/:id/chats", handlers.VibeChatCreate)
			vw.DELETE("/chats/:chatId", handlers.VibeChatDelete)
			vw.GET("/chats/:chatId/messages", handlers.VibeChatMessages)
			vw.POST("/chats/:chatId/messages", middleware.RateLimit(middleware.ChatLimiter), handlers.VibeChatSendMessage)
			vw.POST("/chats/:chatId/messages/stream", middleware.RateLimit(middleware.ChatLimiter), handlers.VibeChatStreamMessage)
			vw.POST("/messages/:msgId/feedback", handlers.VibeMessageFeedback)
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

		// Vibe Write endpoints (Phase 3 → Vibe Write 2.0)
		// Legacy v1 routes (feed/tags/dimensions/snipe/subscribe/persona/kols)
		// removed in Sprint D cleanup. Active routes are in the
		// vw := api.Group("/vibe-write") block above.

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
			admin.POST("/candidates/import-following", handlers.AdminImportKOLFollowing)
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

			// Vibe Write 2.0: legacy admin tag/dimension endpoints removed in Sprint D.
			// Methodology admin replaces this surface — see admin.GET("/methodology") below.

			// User management
			admin.GET("/users/stats", handlers.AdminUserStats)
			admin.GET("/users", handlers.AdminListUsers)
			admin.GET("/users/:id", handlers.AdminGetUser)
			admin.POST("/users/:id/ban", handlers.AdminBanUser)
			admin.POST("/users/:id/unban", handlers.AdminUnbanUser)
			admin.PUT("/users/:id/note", handlers.AdminUpdateUserNote)
			admin.POST("/users/:id/subscription/grant", handlers.AdminGrantSubscription)
			admin.POST("/users/:id/subscription/extend", handlers.AdminExtendSubscription)
			admin.POST("/users/:id/subscription/revoke", handlers.AdminRevokeSubscription)

			// Gift Pro (promotional grants — operates on User.ProExpiresAt directly)
			admin.POST("/gift-pro", handlers.AdminGiftPro)
			admin.GET("/gift-pro/logs", handlers.AdminListGiftProLogs)

			// Claw management (mining approval)
			admin.GET("/claws/stats", handlers.AdminClawStats)
			admin.GET("/claws", handlers.AdminListClaws)
			admin.POST("/claws/:id/approve", handlers.AdminApproveClaw)
			admin.POST("/claws/:id/reject", handlers.AdminRejectClaw)
			admin.POST("/claws/batch-approve", handlers.AdminBatchApproveClaws)

			// Audit log
			admin.GET("/audit-log", handlers.AdminAuditLog)

			// Mentor methodology CRUD (Vibe Write 2.0 brain)
			admin.GET("/methodology", handlers.AdminListMethodology)
			admin.GET("/methodology/:id", handlers.AdminGetMethodology)
			admin.POST("/methodology", handlers.AdminCreateMethodology)
			admin.PUT("/methodology/:id", handlers.AdminUpdateMethodology)
			admin.DELETE("/methodology/:id", handlers.AdminDeleteMethodology)
			admin.POST("/methodology/preview", handlers.AdminPreviewMethodology)
			admin.GET("/methodology/feedback", handlers.AdminMethodologyFeedback)
		}
	}

	return r
}
