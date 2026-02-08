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
	Port string
	Env  string // "production" or "development"

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
