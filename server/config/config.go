package config

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
)

// Config holds all configuration for the application.
type Config struct {
	// Server
	Port     string
	Env      string // "production" or "development"
	LogLevel string // "debug", "info", "warn", "error"

	// Database
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	DBSSLMode  string

	// Blockchain
	BSCRPCURL              string
	IdentityRegistryAddr   string
	ReputationRegistryAddr string
	PrivateKey             string // Platform wallet private key for Soul minting
	ClawPKSecret           string // AES key for encrypting Claw private keys

	// LLM
	LLMProvider string // "openai" or "claude"
	LLMAPIKey   string
	LLMModel    string
	LLMBaseURL  string // Custom base URL for OpenAI-compatible APIs

	// Twitter (for seed extraction)
	TwitterBearerToken string

	// SocialData API (primary Twitter data source)
	SocialDataAPIKey  string
	SocialDataBaseURL string // default: https://api.socialdata.tools

	// Economic System — Contract Addresses
	EnsoulTokenAddr    string // $Ensoul ERC-20 token
	EnsoulLPAddr       string // $Ensoul/BNB LP (PancakeSwap V2)
	EnsoulMinterV2Addr string // EnsoulMinterV2 contract
	PancakeRouterAddr  string // PancakeSwap V2 Router
	USDTAddr           string // BSC USDT

	// Economic System — Wallet Keys (6-wallet scheme)
	AdminAPIKey   string // Admin API key for management endpoints
	AdminUsername string // Initial admin username (for seeding only)
	AdminPassword string // Initial admin password (for seeding only)

	SwapRPCURL string // Private RPC for swap txns (anti-sandwich), e.g. https://rpc-bsc.48.club

	TreasuryAddr          string // Treasury address (cold wallet, no private key on server)
	TaxWalletPrivateKey   string // Tax Wallet — receives 3% token tax, mints public Souls
	BuybackPrivateKey     string // Buyback Wallet — executes PancakeSwap swaps
	MiningPoolPrivateKey  string // Mining Pool Wallet — holds & distributes mining rewards
	RevenuePoolPrivateKey string // Revenue Pool Wallet — holds holder revenue for claim
}

// Global config instance
var Cfg *Config

// Load reads configuration from environment variables.
func Load() *Config {
	// Load .env file if present (ignore error in production)
	_ = godotenv.Load()

	cfg := &Config{
		Port:                   getEnv("PORT", "8990"),
		Env:                    getEnv("ENV", "development"),
		LogLevel:               getEnv("LOG_LEVEL", ""), // auto-set below
		DBHost:                 getEnv("DB_HOST", "localhost"),
		DBPort:                 getEnv("DB_PORT", "5432"),
		DBUser:                 getEnv("DB_USER", "ensoul"),
		DBPassword:             getEnv("DB_PASSWORD", "ensoul"),
		DBName:                 getEnv("DB_NAME", "ensoul"),
		DBSSLMode:              getEnv("DB_SSLMODE", "disable"),
		BSCRPCURL:              getEnv("BSC_RPC_URL", "https://bsc-dataseed.binance.org/"),
		IdentityRegistryAddr:   getEnv("IDENTITY_REGISTRY_ADDR", "0x8004A169FB4a3325136EB29fA0ceB6D2e539a432"),
		ReputationRegistryAddr: getEnv("REPUTATION_REGISTRY_ADDR", "0x8004BAa17C55a88189AE136b182e5fdA19dE9b63"),
		PrivateKey:             getEnv("PLATFORM_PRIVATE_KEY", ""),
		ClawPKSecret:           getEnv("CLAW_PK_SECRET", ""),
		LLMProvider:            getEnv("LLM_PROVIDER", "openai"),
		LLMAPIKey:              getEnv("LLM_API_KEY", ""),
		LLMModel:               getEnv("LLM_MODEL", "gpt-4o"),
		LLMBaseURL:             getEnv("LLM_BASE_URL", ""),
		TwitterBearerToken:     getEnv("TWITTER_BEARER_TOKEN", ""),
		SocialDataAPIKey:       getEnv("SOCIALDATA_API_KEY", ""),
		SocialDataBaseURL:      getEnv("SOCIALDATA_BASE_URL", ""),

		AdminAPIKey:   getEnv("ADMIN_API_KEY", ""),
		AdminUsername: getEnv("ADMIN_USERNAME", "admin"),
		AdminPassword: getEnv("ADMIN_PASSWORD", ""),

		// Economic System — Contract Addresses
		EnsoulTokenAddr:    getEnv("ENSOUL_TOKEN_ADDR", "0x38c4b187834f6351c5e523182f23ed64adf9ffff"),
		EnsoulLPAddr:       getEnv("ENSOUL_LP_ADDR", "0xeb966A3CFbc213ad759d9B185399797b632332952"),
		EnsoulMinterV2Addr: getEnv("ENSOUL_MINTER_V2_ADDR", ""),
		PancakeRouterAddr:  getEnv("PANCAKE_ROUTER_ADDR", "0x10ED43C718714eb63d5aA57B78B54704E256024E"),
		USDTAddr:           getEnv("USDT_ADDR", "0x55d398326f99059fF775485246999027B3197955"),

		// Anti-MEV: private RPC for swap transactions
		SwapRPCURL: getEnv("SWAP_RPC_URL", ""),

		// Economic System — Wallet Keys
		TreasuryAddr:          getEnv("TREASURY_ADDR", ""),
		TaxWalletPrivateKey:   getEnv("TAX_WALLET_PRIVATE_KEY", ""),
		BuybackPrivateKey:     getEnv("BUYBACK_PRIVATE_KEY", ""),
		MiningPoolPrivateKey:  getEnv("MINING_POOL_PRIVATE_KEY", ""),
		RevenuePoolPrivateKey: getEnv("REVENUE_POOL_PRIVATE_KEY", ""),
	}

	// Auto-set log level based on environment if not explicitly configured
	if cfg.LogLevel == "" {
		if cfg.IsProduction() {
			cfg.LogLevel = "info"
		} else {
			cfg.LogLevel = "debug"
		}
	}

	Cfg = cfg

	// Validate critical config
	if cfg.DBHost == "" || cfg.DBName == "" {
		log.Fatal("DB_HOST and DB_NAME are required")
	}

	return cfg
}

// DatabaseURL builds a PostgreSQL connection string from individual fields.
func (c *Config) DatabaseURL() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s",
		c.DBUser, c.DBPassword, c.DBHost, c.DBPort, c.DBName, c.DBSSLMode,
	)
}

// IsProduction returns true if running in production mode.
func (c *Config) IsProduction() bool {
	return c.Env == "production" || c.Env == "prod"
}

// getEnv reads an environment variable with a fallback default value.
func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
