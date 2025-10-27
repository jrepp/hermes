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

	"github.com/hashicorp-forge/hermes/internal/migrate"
)

func main() {
	// Command-line flags
	driver := flag.String("driver", "postgres", "Database driver (postgres|sqlite)")
	dsn := flag.String("dsn", "", "Database connection string")
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
	}

	flag.Parse()

	if *help {
		flag.Usage()
		os.Exit(0)
	}

	// Validate required flags
	if *dsn == "" {
		log.Fatal("Error: -dsn flag is required\n\nRun with -help for usage information.")
	}

	if *driver != "postgres" && *driver != "sqlite" {
		log.Fatalf("Error: unsupported driver '%s' (must be 'postgres' or 'sqlite')\n", *driver)
	}

	// Connect to database
	log.Printf("Connecting to %s database...\n", *driver)
	sqlDB, err := sql.Open(*driver, *dsn)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v\n", err)
	}
	defer sqlDB.Close()

	// Verify connection
	if err := sqlDB.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v\n", err)
	}
	log.Printf("✓ Connected to database\n")

	// Run migrations
	log.Printf("Running migrations...\n")
	if err := migrate.RunMigrations(sqlDB, *driver); err != nil {
		log.Fatalf("Migration failed: %v\n", err)
	}

	log.Printf("✅ All migrations completed successfully!\n")
}
