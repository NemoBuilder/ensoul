package database

import (
	"github.com/ensoul-labs/ensoul-server/config"
	"github.com/ensoul-labs/ensoul-server/models"
	"github.com/ensoul-labs/ensoul-server/util"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// DB is the global database instance.
var DB *gorm.DB

// Connect initializes the database connection and runs auto-migration.
func Connect(cfg *config.Config) *gorm.DB {
	var err error

	// Use Warn-level GORM logging in production (suppress SQL query dumps)
	gormLogLevel := logger.Info
	if cfg.IsProduction() {
		gormLogLevel = logger.Warn
	}

	DB, err = gorm.Open(postgres.Open(cfg.DatabaseURL()), &gorm.Config{
		Logger: logger.Default.LogMode(gormLogLevel),
	})
	if err != nil {
		util.Log.Fatal("Failed to connect to database: %v", err)
	}

	util.Log.Info("Database connected successfully")

	// gen_random_uuid() is built into PostgreSQL 13+, no extension needed.
	// For PostgreSQL 12 or earlier, uncomment the next line:
	// DB.Exec("CREATE EXTENSION IF NOT EXISTS \"pgcrypto\"")

	// Auto-migrate all models
	if err := DB.AutoMigrate(
		&models.Shell{},
		&models.Fragment{},
		&models.Claw{},
		&models.Ensouling{},
		&models.WalletSession{},
		&models.ClawBinding{},
		&models.ChatSession{},
		&models.ChatMessage{},
	); err != nil {
		util.Log.Fatal("Failed to migrate database: %v", err)
	}

	util.Log.Info("Database migration completed")
	return DB
}
