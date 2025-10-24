//go:build integration
// +build integration

package instance

import (
	"fmt"
	"log"
	"os"
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/hashicorp-forge/hermes/pkg/models"
	"github.com/hashicorp-forge/hermes/tests/integration"
)

var (
	// Global test resources
	testDB      *gorm.DB
	testFixture *integration.TestFixture
)

// TestMain is the entry point for instance integration tests.
// It starts containers and runs tests.
func TestMain(m *testing.M) {
	log.Println("🚀 Starting instance integration tests")

	// Setup: Start containers (PostgreSQL)
	if err := integration.SetupFixtureSuite(); err != nil {
		log.Printf("⚠️  Failed to setup integration test fixture: %v\n", err)
		log.Println("⚠️  Container-dependent tests will be skipped")
	}

	// Get fixture (may panic if setup failed, catch it)
	func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("⚠️  Fixture not available: %v\n", r)
				testFixture = nil
			}
		}()
		testFixture = integration.GetFixture()
	}()

	if testFixture != nil {
		// Setup database connection
		db, err := setupDatabase()
		if err != nil {
			log.Printf("⚠️  Failed to setup database: %v\n", err)
			log.Println("⚠️  Database-dependent tests will be skipped")
		} else {
			testDB = db
			log.Println("✓ Database setup complete")
		}
	}

	// If testDB is still nil, try external database via environment variables
	if testDB == nil {
		log.Println("🔌 Attempting to connect to external database...")
		if db, err := setupExternalDatabase(); err == nil {
			testDB = db
			log.Println("✓ External database setup complete")
		} else {
			log.Printf("⚠️  Failed to setup external database: %v\n", err)
		}
	}

	log.Println("✓ Test setup complete, running tests...")

	// Run tests
	code := m.Run()

	// Teardown: Close database and stop containers
	if testDB != nil {
		if db, err := testDB.DB(); err == nil {
			db.Close()
		}
	}
	if testFixture != nil {
		integration.TeardownFixtureSuite()
	}

	log.Println("✓ Test teardown complete")

	// Exit with test result code
	os.Exit(code)
}

// setupDatabase creates a GORM connection and runs migrations
func setupDatabase() (*gorm.DB, error) {
	// Connect to PostgreSQL from testcontainer
	dsn := testFixture.PostgresURL
	if dsn == "" {
		return nil, fmt.Errorf("PostgreSQL URL not available from fixture")
	}

	// Create GORM connection
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent), // Quiet logs during tests
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Run migrations for instance-related tables
	if err := db.AutoMigrate(
		&models.HermesInstance{},
		&models.WorkspaceProject{},
		&models.Document{},
		&models.DocumentRevision{},
	); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return db, nil
}

// setupExternalDatabase connects to an external PostgreSQL database using environment variables
// Defaults are set for the testing Docker Compose environment
func setupExternalDatabase() (*gorm.DB, error) {
	host := getEnv("POSTGRES_HOST", "localhost")
	port := getEnv("POSTGRES_PORT", "5433") // testing environment port
	user := getEnv("POSTGRES_USER", "postgres")
	password := getEnv("POSTGRES_PASSWORD", "postgres")
	dbname := getEnv("POSTGRES_DB", "hermes_testing") // testing environment database name
	sslmode := getEnv("POSTGRES_SSLMODE", "disable")

	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		host, port, user, password, dbname, sslmode)

	// Create GORM connection
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent), // Quiet logs during tests
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to external database: %w", err)
	}

	// Run migrations for instance-related tables
	if err := db.AutoMigrate(
		&models.HermesInstance{},
		&models.WorkspaceProject{},
		&models.Document{},
		&models.DocumentRevision{},
	); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return db, nil
}

// getEnv retrieves an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
