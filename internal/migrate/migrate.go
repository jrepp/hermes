// Package migrate provides migrate functionality.
package migrate

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	migratedb "github.com/golang-migrate/migrate/v4/database"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/database/sqlite"
	"github.com/golang-migrate/migrate/v4/source"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	"github.com/hashicorp-forge/hermes/pkg/database"
)

const (
	// Database driver names
	driverPostgres = "postgres"
	driverSQLite   = "sqlite"
)

//go:embed migrations/*.sql migrations/db-specific/*.sql
var migrationsFS embed.FS

// RunMigrations applies all pending migrations for the given database driver.
// Supports both PostgreSQL and SQLite with core + database-specific migrations.
//
// Migrations land in the connection's default schema. Use RunMigrationsInSchema
// to target a specific one.
func RunMigrations(db *sql.DB, driver string) error {
	return RunMigrationsInSchema(db, driver, "")
}

// RunMigrationsInSchema applies all pending migrations inside schema.
//
// The schema is created if it does not exist, and golang-migrate is told to
// keep its own version table there too. That last part matters: a shared
// version table across schemas would make the first site's migration look
// already-applied to every other site, so sites two onward would come up with
// no tables and no error.
//
// An empty schema means the connection's default, which is the single-tenant
// behaviour. SQLite has no schemas, so a non-empty schema is an error there.
func RunMigrationsInSchema(db *sql.DB, driver, schema string) error {
	// Validate driver
	if driver != driverPostgres && driver != driverSQLite {
		return fmt.Errorf("unsupported database driver: %s (supported: postgres, sqlite)", driver)
	}
	if schema != "" && driver != driverPostgres {
		return fmt.Errorf("schema-scoped migrations require postgres, not %s", driver)
	}

	// Create source driver from embedded migrations
	sourceDriver, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("failed to load migration source: %w", err)
	}

	if schema == "" {
		databaseDriver, err := unscopedDriver(db, driver)
		if err != nil {
			return err
		}

		return runAll(sourceDriver, databaseDriver, db, driver, "")
	}

	if err := database.ValidateSchemaName(schema); err != nil {
		return err
	}
	if _, err := db.Exec(
		"CREATE SCHEMA IF NOT EXISTS " + database.QuoteSchemaName(schema),
	); err != nil {
		return fmt.Errorf("failed to create schema %q: %w", schema, err)
	}

	// Everything runs on one pinned connection with search_path set on it.
	//
	// A *sql.DB hands out whichever pooled connection is free, and
	// `SET search_path` is a session setting, so setting it on the pool would
	// apply to an arbitrary subset of the statements that follow. Worse,
	// golang-migrate's SchemaName only decides where its own version table
	// lives -- it does not scope the migration SQL at all -- so without this
	// the tables would be created in the default schema while the version
	// table correctly recorded them as migrated.
	ctx := context.Background()
	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("acquiring a connection for schema %q: %w", schema, err)
	}
	defer func() { _ = conn.Close() }()

	if _, err := conn.ExecContext(
		ctx, "SET search_path TO "+database.QuoteSchemaName(schema)+",public",
	); err != nil {
		return fmt.Errorf("setting search_path to %q: %w", schema, err)
	}

	databaseDriver, err := postgres.WithConnection(ctx, conn, &postgres.Config{
		SchemaName: schema,
	})
	if err != nil {
		return fmt.Errorf("failed to create postgres driver: %w", err)
	}

	return runAll(sourceDriver, databaseDriver, connExecer{conn}, driver, schema)
}

// unscopedDriver builds the migration driver for the connection's own schema,
// which is the single-tenant path.
func unscopedDriver(db *sql.DB, driver string) (migratedb.Driver, error) {
	switch driver {
	case driverPostgres:
		d, err := postgres.WithInstance(db, &postgres.Config{})
		if err != nil {
			return nil, fmt.Errorf("failed to create postgres driver: %w", err)
		}

		return d, nil
	case driverSQLite:
		d, err := sqlite.WithInstance(db, &sqlite.Config{})
		if err != nil {
			return nil, fmt.Errorf("failed to create sqlite driver: %w", err)
		}

		return d, nil
	}

	return nil, fmt.Errorf("unsupported database driver: %s", driver)
}

// runAll applies the core migrations and then the database-specific extras.
//
// exec is whatever the extras should run on; it must resolve to the same
// schema the core migrations used, or the two halves of the schema end up in
// different places.
func runAll(
	sourceDriver source.Driver, databaseDriver migratedb.Driver,
	exec execer, driver, schema string,
) error {
	m, err := migrate.NewWithInstance("iofs", sourceDriver, driver, databaseDriver)
	if err != nil {
		return fmt.Errorf("failed to create migration instance: %w", err)
	}

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("core migration failed: %w", err)
	}

	if err := applyDatabaseSpecificMigrations(exec, driver); err != nil {
		return fmt.Errorf("database-specific migrations failed: %w", err)
	}

	if schema != "" {
		if err := verifyTablesLandedInSchema(exec, schema); err != nil {
			return err
		}
	}

	return nil
}

// execer is the subset of *sql.DB and *sql.Conn that the statement-by-statement
// migrations and the post-migration check need.
type execer interface {
	Exec(query string, args ...any) (sql.Result, error)
	QueryRow(query string, args ...any) *sql.Row
}

type connExecer struct{ conn *sql.Conn }

func (c connExecer) Exec(query string, args ...any) (sql.Result, error) {
	return c.conn.ExecContext(context.Background(), query, args...)
}

func (c connExecer) QueryRow(query string, args ...any) *sql.Row {
	return c.conn.QueryRowContext(context.Background(), query, args...)
}

// verifyTablesLandedInSchema confirms the migration actually populated the
// site's own schema.
//
// Every site's search_path ends in public so that extension types resolve,
// which means an unqualified statement can reach a table in public. PostgreSQL
// resolves `CREATE TABLE IF NOT EXISTS` against the creation schema rather
// than against everything visible, so tables do land where they should -- but
// that is a guarantee worth checking rather than assuming, because the failure
// it would cause is every site sharing one set of rows with no error anywhere.
//
// The check is cheap and runs once per site at startup.
func verifyTablesLandedInSchema(db execer, schema string) error {
	// A few representative tables from different migrations rather than all of
	// them; the failure mode is all-or-nothing.
	for _, table := range []string{"documents", "products", "users", "workspace_projects"} {
		var found sql.NullString
		if err := db.QueryRow(
			"SELECT to_regclass($1)::text", schema+"."+table,
		).Scan(&found); err != nil {
			return fmt.Errorf("checking for %s.%s: %w", schema, table, err)
		}
		if !found.Valid {
			return fmt.Errorf(
				"migration reported success but %s.%s does not exist.\n"+
					"This happens when the public schema already holds Hermes tables: "+
					"every site's search_path includes public, so CREATE TABLE IF NOT "+
					"EXISTS finds them and does nothing, and all sites end up sharing "+
					"one set of tables.\n"+
					"Move the existing tables into a site schema before enabling "+
					"multi-site hosting",
				schema, table)
		}
	}

	return nil
}

// applyDatabaseSpecificMigrations applies PostgreSQL or SQLite specific schema enhancements.
// These migrations are applied after core migrations and handle database-specific features.
func applyDatabaseSpecificMigrations(db execer, driver string) error {
	var migrations []string

	switch driver {
	case driverPostgres:
		// PostgreSQL-specific migrations (extensions, UUID types, CITEXT)
		migrations = []string{
			"db-specific/000003_indexer_postgres.up.sql",
			"db-specific/000005_postgres_extras.up.sql",
		}
	case driverSQLite:
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

		sqlContent := string(sqlBytes)
		if _, err := db.Exec(sqlContent); err != nil {
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

	var databaseDriver migratedb.Driver
	switch driver {
	case driverPostgres:
		databaseDriver, err = postgres.WithInstance(db, &postgres.Config{})
	case driverSQLite:
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
