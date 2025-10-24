package db

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/hashicorp-forge/hermes/internal/config"
	"github.com/hashicorp-forge/hermes/pkg/models"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// DatabaseConfig holds configuration for database connection.
// Supports both PostgreSQL and SQLite.
type DatabaseConfig struct {
	Driver string // "postgres" or "sqlite"

	// PostgreSQL config
	Host     string
	Port     int
	User     string
	Password string
	DBName   string

	// SQLite config
	Path string // e.g., ".hermes/hermes.db"
}

// NewDB returns a new migrated database.
// This maintains backward compatibility with existing code using config.Postgres.
func NewDB(cfg config.Postgres) (*gorm.DB, error) {
	// Convert config.Postgres to DatabaseConfig
	dbConfig := DatabaseConfig{
		Driver:   "postgres", // Default to postgres for existing configs
		Host:     cfg.Host,
		Port:     cfg.Port,
		User:     cfg.User,
		Password: cfg.Password,
		DBName:   cfg.DBName,
	}
	return NewDBWithConfig(dbConfig)
}

// NewDBWithConfig returns a new migrated database connection using DatabaseConfig.
// Supports both PostgreSQL and SQLite.
func NewDBWithConfig(cfg DatabaseConfig) (*gorm.DB, error) {
	var dialector gorm.Dialector
	var driver string

	switch cfg.Driver {
	case "postgres":
		dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=disable",
			cfg.Host, cfg.User, cfg.Password, cfg.DBName, cfg.Port)
		dialector = postgres.Open(dsn)
		driver = "postgres"

	case "sqlite":
		// Ensure directory exists for SQLite database
		if cfg.Path != "" {
			dir := filepath.Dir(cfg.Path)
			if err := os.MkdirAll(dir, 0755); err != nil {
				return nil, fmt.Errorf("error creating database directory: %w", err)
			}
		}
		dialector = sqlite.Open(cfg.Path)
		driver = "sqlite"

	default:
		return nil, fmt.Errorf("unsupported database driver: %s (supported: postgres, sqlite)", cfg.Driver)
	}

	// Open database connection
	db, err := gorm.Open(dialector, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("error connecting to database: %w", err)
	}

	// Get underlying sql.DB for migrations and extensions
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("error getting sql.DB: %w", err)
	}

	// Run migrations (includes database-specific setup)
	if err := RunMigrations(sqlDB, driver); err != nil {
		return nil, fmt.Errorf("error running migrations: %w", err)
	}

	// Setup join tables (GORM-specific configuration)
	if err := setupJoinTables(db); err != nil {
		return nil, err
	}

	// TEMPORARY WORKAROUND: Disable AutoMigrate to avoid GORM constraint renaming bug
	// See: docs-internal/todos/LOCAL_WORKFLOW_FIX_STATUS.md
	//
	// Problem: GORM tries to rename uniqueIndex constraints even on fresh databases:
	//   - ERROR: constraint "uni_indexer_folders_google_drive_id" does not exist
	//   - ERROR: constraint "uni_workspace_projects_project_uuid" does not exist
	//
	// TODO: Complete SQL migrations for ALL models, then remove AutoMigrate entirely
	// For now: Comment out to get server running, re-enable after fixing migrations
	/*
		if err := db.AutoMigrate(
			models.ModelsToAutoMigrate()...,
		); err != nil {
			return nil, fmt.Errorf("error migrating database: %w", err)
		}
	*/

	return db, nil
}

// setupJoinTables configures GORM join tables for many-to-many relationships.
func setupJoinTables(db *gorm.DB) error {
	if err := db.SetupJoinTable(
		models.Document{},
		"Approvers",
		&models.DocumentReview{},
	); err != nil {
		return fmt.Errorf("error setting up DocumentReviews join table: %w", err)
	}

	if err := db.SetupJoinTable(
		models.User{},
		"RecentlyViewedDocs",
		&models.RecentlyViewedDoc{},
	); err != nil {
		return fmt.Errorf("error setting up RecentlyViewedDocs join table: %w", err)
	}

	if err := db.SetupJoinTable(
		models.User{},
		"RecentlyViewedProjects",
		&models.RecentlyViewedProject{},
	); err != nil {
		return fmt.Errorf("error setting up RecentlyViewedProjects join table: %w", err)
	}

	return nil
}
