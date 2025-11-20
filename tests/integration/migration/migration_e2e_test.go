//go:build integration
// +build integration

package migration

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp-forge/hermes/pkg/docid"
	"github.com/hashicorp-forge/hermes/pkg/migration"
	"github.com/hashicorp-forge/hermes/pkg/workspace"
	s3adapter "github.com/hashicorp-forge/hermes/pkg/workspace/adapters/s3"
)

// TestMigrationE2E validates the complete RFC-089 migration system.
//
// Prerequisites:
//   - PostgreSQL must be running (via fixture)
//   - MinIO must be running on localhost:9000
//   - Database migrations must be applied (000011_add_s3_migration_tables)
//
// This test validates:
//   - Provider registration
//   - Migration job creation and lifecycle
//   - Document migration (copy strategy)
//   - Content validation with SHA-256 hashing
//   - Progress tracking
//   - Worker processing via outbox pattern
//   - End-to-end migration flow
func TestMigrationE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping migration E2E test in short mode")
	}

	ctx := context.Background()

	// Phase 0: Check all prerequisites before starting tests
	t.Run("Phase0_Prerequisites", func(t *testing.T) {
		checker := NewPrerequisiteChecker(t)
		checker.CheckAll(ctx)
		checker.PrintServiceInfo()
	})

	// Connect to database (using external docker-compose postgres, not testcontainers)
	postgresURL := "postgres://postgres:postgres@localhost:5433/hermes_testing?sslmode=disable"
	db, err := sql.Open("pgx", postgresURL)
	require.NoError(t, err, "Failed to connect to database")
	defer db.Close()

	// Verify database connection
	err = db.PingContext(ctx)
	require.NoError(t, err, "Failed to ping database")
	t.Log("✓ Database connection established")

	// Setup logger
	logger := hclog.New(&hclog.LoggerOptions{
		Name:   "migration-test",
		Level:  hclog.Debug,
		Output: testWriter{t},
	})

	// Phase 1: Database Prerequisites
	t.Run("Phase1_DatabasePrerequisites", func(t *testing.T) {
		testDatabasePrerequisites(t, ctx, db)
	})

	// Phase 2: Provider Registration
	var sourceProviderID, destProviderID int64
	t.Run("Phase2_ProviderRegistration", func(t *testing.T) {
		sourceProviderID, destProviderID = testProviderRegistration(t, ctx, db)
	})

	// Phase 3: Create Test Documents
	var testDocuments []testDocument
	t.Run("Phase3_CreateTestDocuments", func(t *testing.T) {
		testDocuments = createTestDocuments(t, ctx, logger)
	})

	// Phase 4: Migration Job Creation
	var jobID int64
	t.Run("Phase4_MigrationJobCreation", func(t *testing.T) {
		jobID = testMigrationJobCreation(t, ctx, db, sourceProviderID, destProviderID, testDocuments)
	})

	// Phase 5: Queue Documents for Migration
	t.Run("Phase5_QueueDocuments", func(t *testing.T) {
		testQueueDocuments(t, ctx, db, jobID, testDocuments)
	})

	// Phase 6: Start Migration Job
	t.Run("Phase6_StartMigrationJob", func(t *testing.T) {
		testStartMigrationJob(t, ctx, db, logger, jobID, sourceProviderID, destProviderID, testDocuments)
	})

	// Phase 7: Worker Processing
	t.Run("Phase7_WorkerProcessing", func(t *testing.T) {
		testWorkerProcessing(t, ctx, db, logger, jobID, sourceProviderID, destProviderID, testDocuments)
	})

	// Phase 8: Verify Migration Results
	t.Run("Phase8_VerifyMigrationResults", func(t *testing.T) {
		testVerifyMigrationResults(t, ctx, db, logger, jobID, testDocuments)
	})

	// Phase 9: Progress Tracking
	t.Run("Phase9_ProgressTracking", func(t *testing.T) {
		testProgressTracking(t, ctx, db, jobID)
	})

	// Phase 9b: Strong Signal Validation (NEW)
	t.Run("Phase9b_StrongSignalValidation", func(t *testing.T) {
		testStrongSignalValidation(t, ctx, db, logger, jobID, testDocuments, destProviderID)
	})

	// Phase 10: Cleanup
	t.Run("Phase10_Cleanup", func(t *testing.T) {
		testCleanup(t, ctx, db, jobID, sourceProviderID, destProviderID)
	})
}

// testDatabasePrerequisites verifies database-specific prerequisites.
func testDatabasePrerequisites(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Log("=== Phase 1: Database Prerequisites ===")

	// Verify all migration-related tables exist
	tables := []string{
		"provider_storage",
		"migration_jobs",
		"migration_items",
		"migration_outbox",
	}

	for _, table := range tables {
		var exists bool
		err := db.QueryRowContext(ctx, `
			SELECT EXISTS (
				SELECT FROM information_schema.tables
				WHERE table_schema = 'public' AND table_name = $1
			)
		`, table).Scan(&exists)
		require.NoError(t, err, "Failed to check if table %s exists", table)
		require.True(t, exists, "❌ Table %s does not exist. Run: make db-migrate", table)
		t.Logf("✓ Table %s exists", table)
	}

	// Verify database is clean for testing
	var jobCount int
	db.QueryRowContext(ctx, "SELECT COUNT(*) FROM migration_jobs WHERE job_name LIKE 'e2e-test-%'").Scan(&jobCount)
	if jobCount > 0 {
		t.Logf("⚠️  Found %d existing e2e test jobs (will be cleaned up)", jobCount)
	}

	t.Log("✅ Database prerequisites met")
}

// testProviderRegistration creates source and destination providers in the database.
func testProviderRegistration(t *testing.T, ctx context.Context, db *sql.DB) (sourceID, destID int64) {
	t.Log("=== Phase 2: Provider Registration ===")

	// Clean up any existing test providers
	_, err := db.ExecContext(ctx, `
		DELETE FROM migration_outbox WHERE migration_job_id IN (
			SELECT id FROM migration_jobs WHERE job_name LIKE 'e2e-test-%'
		)
	`)
	require.NoError(t, err, "Failed to clean outbox")

	_, err = db.ExecContext(ctx, `
		DELETE FROM migration_items WHERE migration_job_id IN (
			SELECT id FROM migration_jobs WHERE job_name LIKE 'e2e-test-%'
		)
	`)
	require.NoError(t, err, "Failed to clean items")

	_, err = db.ExecContext(ctx, `DELETE FROM migration_jobs WHERE job_name LIKE 'e2e-test-%'`)
	require.NoError(t, err, "Failed to clean jobs")

	_, err = db.ExecContext(ctx, `
		DELETE FROM provider_storage WHERE provider_name IN ('e2e-test-source', 'e2e-test-dest')
	`)
	require.NoError(t, err, "Failed to clean providers")

	t.Log("✓ Cleaned up existing test data")

	// Register source provider (mock)
	err = db.QueryRowContext(ctx, `
		INSERT INTO provider_storage (
			provider_name, provider_type, config, status, is_primary, is_writable
		) VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id
	`, "e2e-test-source", "mock", json.RawMessage("{}"), "active", true, true).Scan(&sourceID)
	require.NoError(t, err, "Failed to register source provider")
	require.Greater(t, sourceID, int64(0), "Source provider ID should be positive")
	t.Logf("✓ Registered source provider (ID: %d)", sourceID)

	// Register destination provider (S3)
	s3Config := map[string]interface{}{
		"endpoint":           "http://localhost:9000",
		"region":             "us-east-1",
		"bucket":             "hermes-documents",
		"prefix":             "e2e-test",
		"access_key":         "minioadmin",
		"secret_key":         "minioadmin",
		"versioning_enabled": true,
		"metadata_store":     "manifest",
		"use_ssl":            false,
	}
	configJSON, _ := json.Marshal(s3Config)

	err = db.QueryRowContext(ctx, `
		INSERT INTO provider_storage (
			provider_name, provider_type, config, status, is_primary, is_writable
		) VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id
	`, "e2e-test-dest", "s3", configJSON, "active", false, true).Scan(&destID)
	require.NoError(t, err, "Failed to register destination provider")
	require.Greater(t, destID, int64(0), "Destination provider ID should be positive")
	t.Logf("✓ Registered destination provider (ID: %d)", destID)

	t.Log("✅ Provider registration complete")
	return sourceID, destID
}

// testDocument represents a test document for migration.
type testDocument struct {
	UUID       docid.UUID
	ProviderID string
	Name       string
	Content    string
	Hash       string
}

// createTestDocuments creates test documents in memory (mock source).
func createTestDocuments(t *testing.T, ctx context.Context, logger hclog.Logger) []testDocument {
	t.Log("=== Phase 3: Create Test Documents ===")

	docs := make([]testDocument, 5)
	for i := 0; i < 5; i++ {
		uuid := docid.NewUUID()
		content := fmt.Sprintf("# Test Migration Document %d\n\nThis is test document number %d created at %s.\n\nContent for migration testing with RFC-089.",
			i+1, i+1, time.Now().Format(time.RFC3339))

		// Calculate content hash
		hash := computeContentHash(content)

		docs[i] = testDocument{
			UUID:       uuid,
			ProviderID: uuid.String(), // Mock provider uses UUID as provider ID
			Name:       fmt.Sprintf("Test Migration Doc %d", i+1),
			Content:    content,
			Hash:       hash,
		}

		t.Logf("✓ Created test document %d (UUID: %s, Hash: %s)", i+1, uuid.String()[:8]+"...", hash[:8]+"...")
	}

	t.Logf("✅ Created %d test documents", len(docs))
	return docs
}

// testMigrationJobCreation creates a migration job in the database.
func testMigrationJobCreation(t *testing.T, ctx context.Context, db *sql.DB, sourceID, destID int64, docs []testDocument) int64 {
	t.Log("=== Phase 4: Migration Job Creation ===")

	jobName := fmt.Sprintf("e2e-test-migration-%s", uuid.New().String()[:8])
	jobUUID := uuid.New()

	var jobID int64
	err := db.QueryRowContext(ctx, `
		INSERT INTO migration_jobs (
			job_uuid, job_name, source_provider_id, dest_provider_id,
			strategy, status, total_documents, concurrency, batch_size,
			dry_run, validate_after_migration, rollback_enabled, created_by
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		RETURNING id
	`, jobUUID, jobName, sourceID, destID, "copy", "pending", len(docs),
		5, 100, false, true, true, "e2e-test").Scan(&jobID)

	require.NoError(t, err, "Failed to create migration job")
	require.Greater(t, jobID, int64(0), "Job ID should be positive")

	t.Logf("✓ Created migration job: %s (ID: %d)", jobName, jobID)
	t.Log("✅ Migration job creation complete")

	return jobID
}

// testQueueDocuments queues documents for migration using the transactional outbox pattern.
func testQueueDocuments(t *testing.T, ctx context.Context, db *sql.DB, jobID int64, docs []testDocument) {
	t.Log("=== Phase 5: Queue Documents ===")

	for i, doc := range docs {
		// Begin transaction for atomic outbox pattern
		tx, err := db.BeginTx(ctx, nil)
		require.NoError(t, err, "Failed to begin transaction for document %d", i)

		// Create migration item
		var itemID int64
		err = tx.QueryRowContext(ctx, `
			INSERT INTO migration_items (
				migration_job_id, document_uuid, source_provider_id,
				status, max_attempts
			) VALUES ($1, $2, $3, $4, $5)
			RETURNING id
		`, jobID, doc.UUID, doc.ProviderID, "pending", 3).Scan(&itemID)
		require.NoError(t, err, "Failed to create migration item for document %d", i)

		// Create outbox event with correct payload structure matching TaskPayload
		idempotentKey := fmt.Sprintf("%d:%s", jobID, doc.UUID.String())
		payload := map[string]interface{}{
			"jobId":            jobID,
			"itemId":           itemID,
			"documentUuid":     doc.UUID.String(),
			"sourceProvider":   "e2e-test-source",
			"sourceProviderId": doc.ProviderID,
			"destProvider":     "e2e-test-dest",
			"strategy":         "copy",
			"dryRun":           false,
			"validate":         true,
			"attemptCount":     0,
			"maxAttempts":      3,
		}
		payloadJSON, _ := json.Marshal(payload)

		_, err = tx.ExecContext(ctx, `
			INSERT INTO migration_outbox (
				migration_job_id, migration_item_id, document_uuid, document_id,
				idempotent_key, event_type, provider_source, provider_dest,
				payload, status
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		`, jobID, itemID, doc.UUID, doc.ProviderID, idempotentKey,
			"migration.task.created", "e2e-test-source", "e2e-test-dest",
			payloadJSON, "pending")
		require.NoError(t, err, "Failed to create outbox event for document %d", i)

		// Commit transaction
		err = tx.Commit()
		require.NoError(t, err, "Failed to commit transaction for document %d", i)

		t.Logf("✓ Queued document %d (UUID: %s)", i+1, doc.UUID.String()[:8]+"...")
	}

	t.Logf("✅ Queued %d documents for migration", len(docs))
}

// testStartMigrationJob updates the job status to 'running'.
func testStartMigrationJob(t *testing.T, ctx context.Context, db *sql.DB, logger hclog.Logger, jobID, sourceID, destID int64, docs []testDocument) {
	t.Log("=== Phase 6: Start Migration Job ===")

	// Update job status to running
	result, err := db.ExecContext(ctx, `
		UPDATE migration_jobs
		SET status = 'running', started_at = NOW()
		WHERE id = $1
	`, jobID)
	require.NoError(t, err, "Failed to start migration job")

	rows, err := result.RowsAffected()
	require.NoError(t, err, "Failed to get rows affected")
	require.Equal(t, int64(1), rows, "Should update exactly one job")

	t.Logf("✓ Job %d status updated to 'running'", jobID)
	t.Log("✅ Migration job started")
}

// testWorkerProcessing runs the migration worker to process queued tasks.
func testWorkerProcessing(t *testing.T, ctx context.Context, db *sql.DB, logger hclog.Logger, jobID, sourceID, destID int64, docs []testDocument) {
	t.Log("=== Phase 7: Worker Processing ===")

	// Create mock source provider
	sourceProvider := &mockProvider{
		documents: make(map[string]*workspace.DocumentMetadata),
		content:   make(map[string]*workspace.DocumentContent),
		logger:    logger.Named("mock-source"),
	}

	// Populate mock provider with test documents
	for _, doc := range docs {
		now := time.Now()
		sourceProvider.documents[doc.ProviderID] = &workspace.DocumentMetadata{
			UUID:         doc.UUID,
			ProviderType: "mock",
			ProviderID:   doc.ProviderID,
			Name:         doc.Name,
			MimeType:     "text/markdown",
			CreatedTime:  now,
			ModifiedTime: now,
			ContentHash:  doc.Hash,
		}
		sourceProvider.content[doc.ProviderID] = &workspace.DocumentContent{
			UUID:         doc.UUID,
			ProviderID:   doc.ProviderID,
			Title:        doc.Name,
			Body:         doc.Content,
			Format:       "markdown",
			ContentHash:  doc.Hash,
			LastModified: now,
		}
	}

	// Create S3 destination provider
	s3Config := &s3adapter.Config{
		Endpoint:          "http://localhost:9000",
		Region:            "us-east-1",
		Bucket:            "hermes-documents",
		Prefix:            "e2e-test",
		AccessKey:         "minioadmin",
		SecretKey:         "minioadmin",
		VersioningEnabled: true,
		MetadataStore:     "manifest",
		UseSSL:            false,
	}
	destProvider, err := s3adapter.NewAdapter(s3Config, logger.Named("s3-dest"))
	require.NoError(t, err, "Failed to create S3 adapter")

	// Create provider map
	providers := map[string]workspace.WorkspaceProvider{
		"e2e-test-source": sourceProvider,
		"e2e-test-dest":   destProvider,
	}

	// Create migration manager
	manager := migration.NewManager(db, logger.Named("manager"))

	// Create migration worker
	workerConfig := &migration.WorkerConfig{
		PollInterval:   1 * time.Second, // Fast polling for tests
		MaxConcurrency: 3,
	}
	worker := migration.NewWorker(db, providers, logger.Named("worker"), workerConfig)

	// Start worker in background
	workerCtx, cancelWorker := context.WithTimeout(ctx, 30*time.Second)
	defer cancelWorker()

	workerDone := make(chan error, 1)
	go func() {
		workerDone <- worker.Start(workerCtx)
	}()

	t.Log("✓ Worker started, processing tasks...")

	// Wait for all documents to be processed
	timeout := time.After(30 * time.Second)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	lastMigrated := 0
	for {
		select {
		case <-timeout:
			cancelWorker()
			t.Fatal("❌ Timeout waiting for migration to complete")
		case err := <-workerDone:
			if err != nil && err != context.Canceled && err != context.DeadlineExceeded {
				t.Fatalf("❌ Worker error: %v", err)
			}
			// Worker stopped, check results
			goto checkResults
		case <-ticker.C:
			// Check progress
			var migrated, failed, pending int
			err := db.QueryRowContext(ctx, `
				SELECT
					COUNT(*) FILTER (WHERE status = 'completed') as migrated,
					COUNT(*) FILTER (WHERE status = 'failed') as failed,
					COUNT(*) FILTER (WHERE status = 'pending') as pending
				FROM migration_items
				WHERE migration_job_id = $1
			`, jobID).Scan(&migrated, &failed, &pending)
			require.NoError(t, err, "Failed to check progress")

			if migrated > lastMigrated {
				t.Logf("  Progress: %d/%d migrated, %d failed, %d pending", migrated, len(docs), failed, pending)
				lastMigrated = migrated
			}

			if migrated+failed >= len(docs) {
				cancelWorker()
				goto checkResults
			}
		}
	}

checkResults:
	// Verify all documents were processed
	var migrated, failed int
	err = db.QueryRowContext(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE status = 'completed') as migrated,
			COUNT(*) FILTER (WHERE status = 'failed') as failed
		FROM migration_items
		WHERE migration_job_id = $1
	`, jobID).Scan(&migrated, &failed)
	require.NoError(t, err, "Failed to get final counts")

	t.Logf("✓ Final results: %d migrated, %d failed out of %d total", migrated, failed, len(docs))
	assert.Equal(t, len(docs), migrated, "All documents should be migrated")
	assert.Equal(t, 0, failed, "No documents should fail")

	t.Log("✅ Worker processing complete")

	// Keep the manager reference to avoid unused variable warning
	_ = manager
}

// testVerifyMigrationResults verifies documents were correctly migrated to S3.
func testVerifyMigrationResults(t *testing.T, ctx context.Context, db *sql.DB, logger hclog.Logger, jobID int64, docs []testDocument) {
	t.Log("=== Phase 8: Verify Migration Results ===")

	// Create S3 adapter to verify documents
	s3Config := &s3adapter.Config{
		Endpoint:          "http://localhost:9000",
		Region:            "us-east-1",
		Bucket:            "hermes-documents",
		Prefix:            "e2e-test",
		AccessKey:         "minioadmin",
		SecretKey:         "minioadmin",
		VersioningEnabled: true,
		MetadataStore:     "manifest",
		UseSSL:            false,
	}
	s3Adapter, err := s3adapter.NewAdapter(s3Config, logger.Named("s3-verify"))
	require.NoError(t, err, "Failed to create S3 adapter for verification")

	// Verify each document in S3
	for i, doc := range docs {
		// Get migration item to find destination provider ID
		var destProviderID string
		err := db.QueryRowContext(ctx, `
			SELECT dest_provider_id
			FROM migration_items
			WHERE migration_job_id = $1 AND document_uuid = $2
		`, jobID, doc.UUID).Scan(&destProviderID)
		require.NoError(t, err, "Failed to get destination provider ID for document %d", i)
		require.NotEmpty(t, destProviderID, "Destination provider ID should not be empty for document %d", i)

		// Verify document exists in S3
		content, err := s3Adapter.GetContent(ctx, destProviderID)
		require.NoError(t, err, "Failed to get document %d from S3", i)
		require.NotNil(t, content, "Document %d should exist in S3", i)

		// Verify content matches
		assert.Equal(t, doc.Content, content.Body, "Content should match for document %d", i)

		// Strip "sha256:" prefix if present for comparison
		contentHash := strings.TrimPrefix(content.ContentHash, "sha256:")
		assert.Equal(t, doc.Hash, contentHash, "Content hash should match for document %d", i)

		t.Logf("✓ Verified document %d in S3 (UUID: %s)", i+1, doc.UUID.String()[:8]+"...")
	}

	// Verify content_match flags in database
	var totalCount, matchedCount int
	err = db.QueryRowContext(ctx, `
		SELECT
			COUNT(*) as total,
			COUNT(*) FILTER (WHERE content_match = true) as matched
		FROM migration_items
		WHERE migration_job_id = $1
	`, jobID).Scan(&totalCount, &matchedCount)
	require.NoError(t, err, "Failed to check content_match flags")
	assert.Equal(t, totalCount, matchedCount, "All documents should have content_match = true (expected %d, got %d)", totalCount, matchedCount)

	t.Logf("✅ Verified %d documents successfully migrated to S3", len(docs))
}

// testProgressTracking verifies migration progress tracking.
func testProgressTracking(t *testing.T, ctx context.Context, db *sql.DB, jobID int64) {
	t.Log("=== Phase 9: Progress Tracking ===")

	// Check job status
	var status string
	var totalDocs, migratedDocs, failedDocs, skippedDocs int
	err := db.QueryRowContext(ctx, `
		SELECT status, total_documents, migrated_documents, failed_documents, skipped_documents
		FROM migration_jobs
		WHERE id = $1
	`, jobID).Scan(&status, &totalDocs, &migratedDocs, &failedDocs, &skippedDocs)
	require.NoError(t, err, "Failed to get job progress")

	t.Logf("✓ Job status: %s", status)
	t.Logf("✓ Progress: %d/%d migrated, %d failed, %d skipped", migratedDocs, totalDocs, failedDocs, skippedDocs)

	// Verify outbox events were processed
	var pendingEvents int
	err = db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM migration_outbox
		WHERE migration_job_id = $1 AND status = 'pending'
	`, jobID).Scan(&pendingEvents)
	require.NoError(t, err, "Failed to count pending outbox events")

	assert.Equal(t, 0, pendingEvents, "All outbox events should be processed")
	t.Log("✓ All outbox events processed")

	t.Log("✅ Progress tracking verified")
}

// testStrongSignalValidation runs comprehensive validation checks with strong signals.
func testStrongSignalValidation(t *testing.T, ctx context.Context, db *sql.DB, logger hclog.Logger, jobID int64, docs []testDocument, destProviderID int64) {
	t.Log("=== Phase 9b: Strong Signal Validation ===")

	// Create validator
	validator := NewMigrationValidator(t, db, logger)

	// S3 configuration for validation
	s3Config := &s3adapter.Config{
		Endpoint:          "http://localhost:9000",
		Region:            "us-east-1",
		Bucket:            "hermes-documents",
		Prefix:            "e2e-test",
		AccessKey:         "minioadmin",
		SecretKey:         "minioadmin",
		VersioningEnabled: true,
		MetadataStore:     "manifest",
		UseSSL:            false,
	}

	var allResults []ValidationResult

	// Validation 1: Job Completeness
	t.Log("Running validation: Job Completeness")
	results := validator.ValidateJobCompleteness(ctx, jobID, len(docs))
	allResults = append(allResults, results...)
	for _, r := range results {
		if r.Passed {
			t.Logf("  ✓ %s", r.Name)
		} else {
			t.Logf("  ✗ %s: expected %v, got %v", r.Name, r.ExpectedVal, r.ActualVal)
		}
	}

	// Validation 2: Content Integrity
	t.Log("Running validation: Content Integrity")
	results = validator.ValidateContentIntegrity(ctx, jobID, docs, s3Config)
	allResults = append(allResults, results...)
	for _, r := range results {
		if r.Passed {
			t.Logf("  ✓ %s", r.Name)
		} else {
			t.Logf("  ✗ %s: expected %v, got %v", r.Name, r.ExpectedVal, r.ActualVal)
		}
	}

	// Validation 3: Outbox Integrity
	t.Log("Running validation: Outbox Integrity")
	results = validator.ValidateOutboxIntegrity(ctx, jobID)
	allResults = append(allResults, results...)
	for _, r := range results {
		if r.Passed {
			t.Logf("  ✓ %s", r.Name)
		} else {
			t.Logf("  ✗ %s: expected %v, got %v", r.Name, r.ExpectedVal, r.ActualVal)
		}
	}

	// Validation 4: Migration Invariants
	t.Log("Running validation: Migration Invariants")
	results = validator.ValidateMigrationInvariants(ctx, jobID, len(docs))
	allResults = append(allResults, results...)
	for _, r := range results {
		if r.Passed {
			t.Logf("  ✓ %s", r.Name)
		} else {
			t.Logf("  ✗ %s: expected %v, got %v", r.Name, r.ExpectedVal, r.ActualVal)
		}
	}

	// Validation 5: S3 Storage
	t.Log("Running validation: S3 Storage")
	results = validator.ValidateS3Storage(ctx, jobID, s3Config)
	allResults = append(allResults, results...)
	for _, r := range results {
		if r.Passed {
			t.Logf("  ✓ %s", r.Name)
		} else {
			t.Logf("  ✗ %s: expected %v, got %v", r.Name, r.ExpectedVal, r.ActualVal)
		}
	}

	// Print comprehensive validation report
	validator.PrintValidationReport(allResults)

	// Assert all validations passed
	validator.AssertAllValidationsPassed(allResults)

	t.Log("✅ All strong signal validations passed")
}

// testCleanup cleans up test data from the database.
func testCleanup(t *testing.T, ctx context.Context, db *sql.DB, jobID, sourceID, destID int64) {
	t.Log("=== Phase 10: Cleanup ===")

	// Delete outbox events
	_, err := db.ExecContext(ctx, `DELETE FROM migration_outbox WHERE migration_job_id = $1`, jobID)
	require.NoError(t, err, "Failed to delete outbox events")
	t.Log("✓ Deleted outbox events")

	// Delete migration items
	_, err = db.ExecContext(ctx, `DELETE FROM migration_items WHERE migration_job_id = $1`, jobID)
	require.NoError(t, err, "Failed to delete migration items")
	t.Log("✓ Deleted migration items")

	// Delete migration job
	_, err = db.ExecContext(ctx, `DELETE FROM migration_jobs WHERE id = $1`, jobID)
	require.NoError(t, err, "Failed to delete migration job")
	t.Log("✓ Deleted migration job")

	// Delete providers
	_, err = db.ExecContext(ctx, `DELETE FROM provider_storage WHERE id IN ($1, $2)`, sourceID, destID)
	require.NoError(t, err, "Failed to delete providers")
	t.Log("✓ Deleted test providers")

	// Note: We don't clean up S3 documents as they may be useful for inspection
	t.Log("ℹ️  S3 documents left for inspection (prefix: e2e-test)")

	t.Log("✅ Cleanup complete")
}

// mockProvider implements workspace.WorkspaceProvider for testing.
type mockProvider struct {
	documents map[string]*workspace.DocumentMetadata
	content   map[string]*workspace.DocumentContent
	logger    hclog.Logger
}

// DocumentProvider interface
func (m *mockProvider) GetDocument(ctx context.Context, providerID string) (*workspace.DocumentMetadata, error) {
	if doc, ok := m.documents[providerID]; ok {
		return doc, nil
	}
	return nil, fmt.Errorf("document not found: %s", providerID)
}

func (m *mockProvider) GetDocumentByUUID(ctx context.Context, uuid docid.UUID) (*workspace.DocumentMetadata, error) {
	for _, doc := range m.documents {
		if doc.UUID == uuid {
			return doc, nil
		}
	}
	return nil, fmt.Errorf("document not found: %s", uuid.String())
}

func (m *mockProvider) CreateDocument(ctx context.Context, templateID, destFolderID, name string) (*workspace.DocumentMetadata, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockProvider) CreateDocumentWithUUID(ctx context.Context, uuid docid.UUID, templateID, destFolderID, name string) (*workspace.DocumentMetadata, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockProvider) RegisterDocument(ctx context.Context, doc *workspace.DocumentMetadata) (*workspace.DocumentMetadata, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockProvider) CopyDocument(ctx context.Context, sourceProviderID, destFolderID, newName string) (*workspace.DocumentMetadata, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockProvider) MoveDocument(ctx context.Context, providerID, destFolderID string) (*workspace.DocumentMetadata, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockProvider) DeleteDocument(ctx context.Context, providerID string) error {
	return fmt.Errorf("not implemented")
}

func (m *mockProvider) RenameDocument(ctx context.Context, providerID, newName string) error {
	return fmt.Errorf("not implemented")
}

func (m *mockProvider) CreateFolder(ctx context.Context, name, parentID string) (*workspace.DocumentMetadata, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockProvider) GetSubfolder(ctx context.Context, parentID, name string) (string, error) {
	return "", fmt.Errorf("not implemented")
}

// ContentProvider interface
func (m *mockProvider) GetContent(ctx context.Context, providerID string) (*workspace.DocumentContent, error) {
	if content, ok := m.content[providerID]; ok {
		return content, nil
	}
	return nil, fmt.Errorf("content not found: %s", providerID)
}

func (m *mockProvider) GetContentByUUID(ctx context.Context, uuid docid.UUID) (*workspace.DocumentContent, error) {
	for _, content := range m.content {
		if content.UUID == uuid {
			return content, nil
		}
	}
	return nil, fmt.Errorf("content not found for UUID: %s", uuid.String())
}

func (m *mockProvider) UpdateContent(ctx context.Context, providerID, content string) (*workspace.DocumentContent, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockProvider) GetContentBatch(ctx context.Context, providerIDs []string) ([]*workspace.DocumentContent, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockProvider) CompareContent(ctx context.Context, providerID1, providerID2 string) (*workspace.ContentComparison, error) {
	return nil, fmt.Errorf("not implemented")
}

// RevisionTrackingProvider interface
func (m *mockProvider) GetRevisionHistory(ctx context.Context, providerID string, limit int) ([]*workspace.BackendRevision, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockProvider) GetRevision(ctx context.Context, providerID, revisionID string) (*workspace.BackendRevision, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockProvider) GetRevisionContent(ctx context.Context, providerID, revisionID string) (*workspace.DocumentContent, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockProvider) CompareRevisions(ctx context.Context, providerID, revisionID1, revisionID2 string) (*workspace.ContentComparison, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockProvider) KeepRevisionForever(ctx context.Context, providerID, revisionID string) error {
	return fmt.Errorf("not implemented")
}

func (m *mockProvider) GetAllDocumentRevisions(ctx context.Context, uuid docid.UUID) ([]*workspace.RevisionInfo, error) {
	return nil, fmt.Errorf("not implemented")
}

// PermissionProvider interface (stubs)
func (m *mockProvider) ShareDocument(ctx context.Context, providerID, email, role string) error {
	return fmt.Errorf("not implemented")
}

func (m *mockProvider) ShareDocumentWithDomain(ctx context.Context, providerID, domain, role string) error {
	return fmt.Errorf("not implemented")
}

func (m *mockProvider) ListPermissions(ctx context.Context, providerID string) ([]*workspace.FilePermission, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockProvider) RemovePermission(ctx context.Context, providerID, permissionID string) error {
	return fmt.Errorf("not implemented")
}

func (m *mockProvider) UpdatePermission(ctx context.Context, providerID, permissionID, newRole string) error {
	return fmt.Errorf("not implemented")
}

// PeopleProvider interface (stubs)
func (m *mockProvider) SearchPeople(ctx context.Context, query string) ([]*workspace.UserIdentity, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockProvider) GetPerson(ctx context.Context, email string) (*workspace.UserIdentity, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockProvider) GetPersonByUnifiedID(ctx context.Context, unifiedID string) (*workspace.UserIdentity, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockProvider) ResolveIdentity(ctx context.Context, email string) (*workspace.UserIdentity, error) {
	return nil, fmt.Errorf("not implemented")
}

// TeamProvider interface (stubs)
func (m *mockProvider) ListTeams(ctx context.Context, domain, query string, maxResults int64) ([]*workspace.Team, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockProvider) GetTeam(ctx context.Context, teamID string) (*workspace.Team, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockProvider) GetUserTeams(ctx context.Context, userEmail string) ([]*workspace.Team, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockProvider) GetTeamMembers(ctx context.Context, teamID string) ([]*workspace.UserIdentity, error) {
	return nil, fmt.Errorf("not implemented")
}

// NotificationProvider interface (stubs)
func (m *mockProvider) SendEmail(ctx context.Context, to []string, from, subject, body string) error {
	return fmt.Errorf("not implemented")
}

func (m *mockProvider) SendEmailWithTemplate(ctx context.Context, to []string, template string, data map[string]any) error {
	return fmt.Errorf("not implemented")
}

// testWriter wraps *testing.T to implement io.Writer for hclog.
type testWriter struct {
	t *testing.T
}

func (tw testWriter) Write(p []byte) (n int, err error) {
	tw.t.Log(string(p))
	return len(p), nil
}

// computeContentHash computes SHA-256 hash of content string.
func computeContentHash(content string) string {
	h := sha256.New()
	h.Write([]byte(content))
	return hex.EncodeToString(h.Sum(nil))
}
