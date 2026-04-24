//go:build integration
// +build integration

package indexer

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/hashicorp-forge/hermes/pkg/models"
	"github.com/hashicorp-forge/hermes/tests/integration"
)

var (
	// Global test resources
	testDB          *gorm.DB
	testFixture     *integration.TestFixture
	ollamaBaseURL   string
	ollamaAvailable bool
	summarizeModel  string
	embeddingModel  string
)

// TestMain is the entry point for indexer integration tests.
// It starts containers, verifies Ollama, and runs tests.
func TestMain(m *testing.M) {
	log.Println("🚀 Starting indexer integration tests with Ollama")

	// Setup: Start containers (PostgreSQL, Meilisearch)
	// Skip container setup if it fails (allows simple Ollama tests to run)
	if err := integration.SetupFixtureSuite(); err != nil {
		log.Printf("⚠️  Failed to setup integration test fixture: %v\n", err)
		log.Println("⚠️  Container-dependent tests will be skipped")
		log.Println("✓ Simple Ollama tests can still run")
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

	// Check Ollama availability
	ollamaBaseURL = getEnv("OLLAMA_BASE_URL", "http://localhost:11434")
	summarizeModel = getEnv("OLLAMA_SUMMARIZE_MODEL", "llama3.2")
	embeddingModel = getEnv("OLLAMA_EMBEDDING_MODEL", "nomic-embed-text")

	ollamaAvailable = checkOllamaAvailable(ollamaBaseURL)
	if !ollamaAvailable {
		log.Printf("⚠️  Ollama not available at %s\n", ollamaBaseURL)
		log.Println("⚠️  Ollama-dependent tests will be skipped")
		log.Println("💡 Start Ollama: ollama serve")
		log.Println("💡 Pull models: ollama pull llama3.2 && ollama pull nomic-embed-text")
	} else {
		log.Printf("✓ Ollama available at %s\n", ollamaBaseURL)
		if err := checkOllamaModels(ollamaBaseURL, []string{summarizeModel, embeddingModel}); err != nil {
			log.Printf("⚠️  Ollama models not available: %v\n", err)
			log.Printf("💡 Pull models: ollama pull %s && ollama pull %s\n", summarizeModel, embeddingModel)
			ollamaAvailable = false
		} else {
			log.Printf("✓ Ollama models available: %s, %s\n", summarizeModel, embeddingModel)
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

	// Run migrations for indexer-related tables
	if err := db.AutoMigrate(
		&models.Document{},
		&models.DocumentRevision{},
		&models.DocumentSummary{},
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

	// Run migrations for indexer-related tables
	if err := db.AutoMigrate(
		&models.Document{},
		&models.DocumentRevision{},
		&models.DocumentSummary{},
	); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return db, nil
}

// checkOllamaAvailable verifies Ollama is running
func checkOllamaAvailable(baseURL string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", baseURL+"/api/version", http.NoBody)
	if err != nil {
		return false
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// checkOllamaModels verifies required models are available
func checkOllamaModels(baseURL string, models []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for _, model := range models {
		// Try to get model info (Ollama /api/show endpoint)
		reqBody := fmt.Sprintf(`{"name":%q}`, model)
		req, err := http.NewRequestWithContext(ctx, "POST", baseURL+"/api/show",
			bytes.NewBufferString(reqBody))
		if err != nil {
			return fmt.Errorf("failed to create request for model %s: %w", model, err)
		}
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("model %s not available: %w", model, err)
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("model %s not found (status: %d)", model, resp.StatusCode)
		}
	}

	return nil
}

// getEnv returns environment variable value or default
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
