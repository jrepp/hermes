package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"

	// Import database drivers as needed
	// Note: We only import lib/pq for postgres. SQLite driver is imported
	// by golang-migrate/migrate/v4/database/sqlite internally via modernc.org/sqlite
	_ "github.com/lib/pq" // PostgreSQL driver

	"github.com/hashicorp-forge/hermes/internal/config"
	"github.com/hashicorp-forge/hermes/internal/db"
	"github.com/hashicorp-forge/hermes/internal/migrate"
	"github.com/hashicorp-forge/hermes/internal/sites"
)

func main() {
	os.Exit(run())
}

func run() int {
	// Command-line flags
	driver := flag.String("driver", "postgres", "Database driver (postgres|sqlite)")
	dsn := flag.String("dsn", "", "Database connection string")
	schema := flag.String("schema", "",
		"PostgreSQL schema to migrate into (default: the connection's schema)")
	configFile := flag.String("config", "",
		"Hermes config file; migrates every site it declares, each into its own schema")
	help := flag.Bool("help", false, "Show help message")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [OPTIONS]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Hermes Database Migration Tool\n\n")
		fmt.Fprintf(os.Stderr, "This binary handles all database schema migrations for Hermes.\n")
		fmt.Fprintf(os.Stderr, "It supports both PostgreSQL and SQLite databases.\n\n")
		fmt.Fprintf(os.Stderr, "OPTIONS:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nEXAMPLES:\n\n")
		fmt.Fprintf(os.Stderr, "  PostgreSQL:\n")
		fmt.Fprintf(os.Stderr, "    %s -driver=postgres -dsn=\"host=localhost user=postgres password=postgres dbname=hermes port=5432 sslmode=disable\"\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  SQLite:\n")
		fmt.Fprintf(os.Stderr, "    %s -driver=sqlite -dsn=\".hermes/hermes.db\"\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  One site schema:\n")
		fmt.Fprintf(os.Stderr, "    %s -dsn=\"...\" -schema=site_docs_jrepp_com\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  Every site in a config file (connections come from the config):\n")
		fmt.Fprintf(os.Stderr, "    %s -config=local/config.hcl\n\n", os.Args[0])
	}

	flag.Parse()

	if *help {
		flag.Usage()
		return 0
	}

	// Validate required flags. -config carries its own connections.
	if *dsn == "" && *configFile == "" {
		log.Fatal("Error: -dsn flag is required\n\nRun with -help for usage information.")
	}

	if *driver != "postgres" && *driver != "sqlite" {
		log.Fatalf("Error: unsupported driver '%s' (must be 'postgres' or 'sqlite')\n", *driver)
	}

	if *schema != "" && *configFile != "" {
		log.Fatal("Error: -schema and -config are mutually exclusive\n\n" +
			"-config migrates every site into its own schema; -schema names a single one.")
	}

	// -config takes over completely: each site carries its own connection, its
	// own schema, and its own extensions to install, so there is nothing left
	// for a single -dsn to say. Running the same code the server runs is the
	// point -- a migration tool that does less than the server leaves the
	// database in a state the server then has to fix or refuse.
	if *configFile != "" {
		if err := migrateSitesFromConfig(*configFile); err != nil {
			log.Printf("Error: %v\n", err)
			return 1
		}
		log.Printf("✅ All migrations completed successfully!\n")

		return 0
	}

	// Single-tenant, or one named schema of one database.
	schemas := []string{*schema}

	// Connect to database
	log.Printf("Connecting to %s database...\n", *driver)
	sqlDB, err := sql.Open(*driver, *dsn)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v\n", err)
	}
	defer func() { _ = sqlDB.Close() }()

	// Verify connection
	if err := sqlDB.Ping(); err != nil {
		log.Printf("Failed to ping database: %v\n", err)
		return 1
	}
	log.Printf("✓ Connected to database\n")

	// Run migrations. Each schema is migrated independently -- golang-migrate
	// keeps its version table inside the target schema -- so a site added to
	// an existing deployment starts from zero rather than inheriting another
	// site's version and coming up with no tables.
	for _, sc := range schemas {
		if sc == "" {
			log.Printf("Running migrations...\n")
		} else {
			log.Printf("Running migrations in schema %s...\n", sc)
		}
		if err := migrate.RunMigrationsInSchema(sqlDB, *driver, sc); err != nil {
			log.Printf("Migration failed: %v\n", err)
			return 1
		}
	}

	log.Printf("✅ All migrations completed successfully!\n")
	return 0
}

// migrateSitesFromConfig migrates every site a config file declares, each into
// its own schema of its own database.
//
// It goes through the same registry and the same migration path the server
// uses, so a config the server would refuse to start on is refused here too
// rather than half-applied, and the resulting schema is exactly what the
// server expects -- extensions installed, tenant columns stamped, and all.
func migrateSitesFromConfig(configFile string) error {
	cfg, err := config.NewConfig(configFile, "")
	if err != nil {
		return fmt.Errorf("reading %s: %w", configFile, err)
	}

	// Same override the server applies, and for the same reason: the registry
	// resolves each site's connection from this, so it has to be right before
	// the registry is built.
	if val, ok := os.LookupEnv("HERMES_SERVER_POSTGRES_PASSWORD"); ok && cfg.Postgres != nil {
		cfg.Postgres.Password = val
	}

	registry, err := sites.NewRegistry(cfg)
	if err != nil {
		return fmt.Errorf("reading sites from %s: %w", configFile, err)
	}
	if registry.Len() == 0 {
		return fmt.Errorf(
			"%s declares no sites; drop -config and pass -dsn to migrate a single database",
			configFile)
	}

	for _, site := range registry.Sites() {
		log.Printf("  site %s -> %s/%s schema %s\n",
			site.Domain.String(), site.Postgres.Host, site.Postgres.DBName, site.SchemaName)
	}

	var pg config.Postgres
	if cfg.Postgres != nil {
		pg = *cfg.Postgres
	}

	return db.MigrateSites(pg, registry, nil)
}
