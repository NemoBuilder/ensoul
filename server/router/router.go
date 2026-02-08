package router

import (
	"net/http"

	"github.com/ensoul-labs/ensoul-server/config"
	"github.com/ensoul-labs/ensoul-server/handlers"
	"github.com/ensoul-labs/ensoul-server/middleware"
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

	// CORS configuration — allow frontend dev server
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:3410", "https://ensoul.ac"},
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
			shell.POST("/preview", middleware.RateLimit(middleware.GeneralLimiter), handlers.ShellPreview)
			shell.POST("/mint", middleware.RateLimit(middleware.RegisterLimiter), handlers.ShellMint)
			shell.POST("/confirm", middleware.RateLimit(middleware.GeneralLimiter), handlers.ShellConfirmMint)
			shell.GET("/list", handlers.ShellList)
			shell.GET("/:handle", handlers.ShellGetByHandle)
			shell.GET("/:handle/dimensions", handlers.ShellGetDimensions)
			shell.GET("/:handle/history", handlers.ShellGetHistory)
			shell.GET("/:handle/contributors", handlers.ShellContributors)
		}

		// Fragment endpoints
		fragment := api.Group("/fragment")
		{
			// Submit requires authenticated + claimed Claw
			fragment.POST("/submit", middleware.RateLimit(middleware.SubmitLimiter), middleware.AuthClaw(), middleware.RequireClaimed(), handlers.FragmentSubmit)
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
		}

		// Stats endpoint — public
		api.GET("/stats", handlers.GetStats)

		// Task board — public
		api.GET("/tasks", handlers.GetTasks)
	}

	return r
}
