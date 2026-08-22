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
		fmt.Fprintf(os.Stderr, "  Every site in a config file:\n")
		fmt.Fprintf(os.Stderr, "    %s -dsn=\"...\" -config=local/config.hcl\n\n", os.Args[0])
	}

	flag.Parse()

	if *help {
		flag.Usage()
		return 0
	}

	// Validate required flags
	if *dsn == "" {
		log.Fatal("Error: -dsn flag is required\n\nRun with -help for usage information.")
	}

	if *driver != "postgres" && *driver != "sqlite" {
		log.Fatalf("Error: unsupported driver '%s' (must be 'postgres' or 'sqlite')\n", *driver)
	}

	if *schema != "" && *configFile != "" {
		log.Fatal("Error: -schema and -config are mutually exclusive\n\n" +
			"-config migrates every site into its own schema; -schema names a single one.")
	}

	// Resolve the schemas to migrate. Empty means the connection's own schema,
	// which is the single-tenant behaviour.
	schemas := []string{*schema}
	if *configFile != "" {
		resolved, err := siteSchemas(*configFile)
		if err != nil {
			log.Printf("Error: %v\n", err)
			return 1
		}
		if len(resolved) == 0 {
			log.Printf("Error: %s declares no sites; drop -config to migrate the default schema\n",
				*configFile)
			return 1
		}
		schemas = resolved
	}

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

// siteSchemas reads a Hermes config file and returns one schema name per
// configured site, in the order they are declared.
//
// It goes through the same registry the server uses, so a config the server
// would refuse to start on -- two sites claiming one hostname, say -- is also
// one this tool refuses to migrate, rather than quietly creating schemas for
// a layout that will never serve traffic.
func siteSchemas(configFile string) ([]string, error) {
	cfg, err := config.NewConfig(configFile, "")
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", configFile, err)
	}

	registry, err := sites.NewRegistry(cfg)
	if err != nil {
		return nil, fmt.Errorf("reading sites from %s: %w", configFile, err)
	}

	schemas := make([]string, 0, registry.Len())
	for _, site := range registry.Sites() {
		log.Printf("  site %s -> schema %s\n", site.Domain.String(), site.SchemaName)
		schemas = append(schemas, site.SchemaName)
	}

	return schemas, nil
}
