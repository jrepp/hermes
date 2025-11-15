package db

import (
	"fmt"

	"github.com/hashicorp-forge/hermes/internal/config"
	"github.com/hashicorp-forge/hermes/pkg/database"
	"github.com/hashicorp-forge/hermes/pkg/models"
	"gorm.io/driver/postgres"
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
// It now uses the shared pkg/database connection logic.
func NewDB(cfg config.Postgres) (*gorm.DB, error) {
	// Convert config.Postgres to database.Config
	dbConfig := database.Config{
		Host:     cfg.Host,
		Port:     cfg.Port,
		User:     cfg.User,
		Password: cfg.Password,
		DBName:   cfg.DBName,
		SSLMode:  "disable", // Default for backward compatibility
	}

	// Use shared database connection logic (no logger here for backward compatibility)
	db, err := database.Connect(dbConfig, nil)
	if err != nil {
		return nil, err
	}

	// Setup join tables (GORM-specific configuration)
	if err := setupJoinTables(db); err != nil {
		return nil, err
	}

	return db, nil
}

// NewDBWithConfig returns a new database connection using DatabaseConfig.
// NOTE: Server binary only supports PostgreSQL to avoid SQLite driver conflicts.
// For SQLite support, use the hermes-migrate binary.
//
// Deprecated: This function is kept for backward compatibility.
// New code should use database.Connect() from pkg/database instead.
func NewDBWithConfig(cfg DatabaseConfig) (*gorm.DB, error) {
	var dialector gorm.Dialector

	switch cfg.Driver {
	case "postgres":
		dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=disable",
			cfg.Host, cfg.User, cfg.Password, cfg.DBName, cfg.Port)
		dialector = postgres.Open(dsn)

	case "sqlite":
		return nil, fmt.Errorf("SQLite not supported in server binary (avoid driver conflicts). Use hermes-migrate for SQLite migrations. See docs-internal/SQLITE_DRIVER_CONFLICT.md")

	default:
		return nil, fmt.Errorf("unsupported database driver: %s (server only supports postgres)", cfg.Driver)
	}

	// Open database connection
	db, err := gorm.Open(dialector, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("error connecting to database: %w", err)
	}

	// NOTE: Migrations are now handled by the separate hermes-migrate binary.
	// The server expects the database to be pre-migrated.
	// See: docs-internal/SQLITE_DRIVER_CONFLICT.md for architecture details.
	//
	// Run migrations manually before starting the server:
	//   ./build/bin/hermes-migrate -driver=postgres -dsn="..."

	// Setup join tables (GORM-specific configuration)
	if err := setupJoinTables(db); err != nil {
		return nil, err
	}

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
