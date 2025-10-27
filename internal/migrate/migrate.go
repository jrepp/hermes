package migrate

import (
	"database/sql"
	"embed"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/database/sqlite"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed migrations/*.sql migrations/db-specific/*.sql
var migrationsFS embed.FS

// RunMigrations applies all pending migrations for the given database driver.
// Supports both PostgreSQL and SQLite with core + database-specific migrations.
func RunMigrations(db *sql.DB, driver string) error {
	// Validate driver
	if driver != "postgres" && driver != "sqlite" {
		return fmt.Errorf("unsupported database driver: %s (supported: postgres, sqlite)", driver)
	}

	// Create source driver from embedded migrations
	sourceDriver, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("failed to load migration source: %w", err)
	}

	// Create database driver based on type
	var databaseDriver database.Driver
	switch driver {
	case "postgres":
		databaseDriver, err = postgres.WithInstance(db, &postgres.Config{})
		if err != nil {
			return fmt.Errorf("failed to create postgres driver: %w", err)
		}
	case "sqlite":
		databaseDriver, err = sqlite.WithInstance(db, &sqlite.Config{})
		if err != nil {
			return fmt.Errorf("failed to create sqlite driver: %w", err)
		}
	}

	// Create migration instance
	m, err := migrate.NewWithInstance(
		"iofs", sourceDriver,
		driver, databaseDriver,
	)
	if err != nil {
		return fmt.Errorf("failed to create migration instance: %w", err)
	}

	// Run core migrations (works for both databases)
	if err := m.Up(); err != nil {
		if err != migrate.ErrNoChange {
			return fmt.Errorf("core migration failed: %w", err)
		}
	}

	// Apply database-specific enhancements
	if err := applyDatabaseSpecificMigrations(db, driver); err != nil {
		return fmt.Errorf("database-specific migrations failed: %w", err)
	}

	return nil
}

// applyDatabaseSpecificMigrations applies PostgreSQL or SQLite specific schema enhancements.
// These migrations are applied after core migrations and handle database-specific features.
func applyDatabaseSpecificMigrations(db *sql.DB, driver string) error {
	var migrations []string

	switch driver {
	case "postgres":
		// PostgreSQL-specific migrations (extensions, UUID types, CITEXT)
		migrations = []string{
			"db-specific/000003_indexer_postgres.up.sql",
			"db-specific/000005_postgres_extras.up.sql",
		}
	case "sqlite":
		// SQLite-specific migrations (PRAGMAs, optimizations)
		migrations = []string{
			"db-specific/000004_indexer_sqlite.up.sql",
			"db-specific/000006_sqlite_extras.up.sql",
		}
	}

	for _, migrationFile := range migrations {
		sqlBytes, err := migrationsFS.ReadFile("migrations/" + migrationFile)
		if err != nil {
			// If file doesn't exist, skip (some migrations may not have DB-specific changes)
			continue
		}

		sql := string(sqlBytes)
		if _, err := db.Exec(sql); err != nil {
			return fmt.Errorf("failed to apply %s: %w", migrationFile, err)
		}
	}

	return nil
}

// GetMigrationVersion returns the current migration version.
func GetMigrationVersion(db *sql.DB, driver string) (version uint, dirty bool, err error) {
	sourceDriver, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return 0, false, fmt.Errorf("failed to load migration source: %w", err)
	}

	var databaseDriver database.Driver
	switch driver {
	case "postgres":
		databaseDriver, err = postgres.WithInstance(db, &postgres.Config{})
	case "sqlite":
		databaseDriver, err = sqlite.WithInstance(db, &sqlite.Config{})
	default:
		return 0, false, fmt.Errorf("unsupported database driver: %s", driver)
	}
	if err != nil {
		return 0, false, fmt.Errorf("failed to create database driver: %w", err)
	}

	m, err := migrate.NewWithInstance(
		"iofs", sourceDriver,
		driver, databaseDriver,
	)
	if err != nil {
		return 0, false, fmt.Errorf("failed to create migration instance: %w", err)
	}

	return m.Version()
}
