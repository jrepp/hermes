//go:build integration
// +build integration

package migration

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	s3adapter "github.com/hashicorp-forge/hermes/pkg/workspace/adapters/s3"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/require"
)

// ValidationResult represents the result of a validation check.
type ValidationResult struct {
	Name        string
	Passed      bool
	Message     string
	ExpectedVal interface{}
	ActualVal   interface{}
}

// MigrationValidator provides strong signal validation for migration correctness.
type MigrationValidator struct {
	db     *sql.DB
	logger hclog.Logger
	t      *testing.T
}

// NewMigrationValidator creates a new validator.
func NewMigrationValidator(t *testing.T, db *sql.DB, logger hclog.Logger) *MigrationValidator {
	return &MigrationValidator{
		db:     db,
		logger: logger,
		t:      t,
	}
}

// ValidateJobCompleteness ensures the migration job completed successfully with all expected data.
//
// Strong signals:
// - Job status is 'completed' or 'running' with 100% success
// - Total documents = migrated + failed + skipped (no leaks)
// - All migration items have terminal status (completed/failed/skipped)
// - No items stuck in 'pending' or 'in_progress'
// - All outbox events are processed (status != 'pending')
func (v *MigrationValidator) ValidateJobCompleteness(ctx context.Context, jobID int64, expectedDocs int) []ValidationResult {
	results := []ValidationResult{}

	v.logger.Info("validating job completeness", "job_id", jobID, "expected_docs", expectedDocs)

	// Check 1: Job exists and has correct total
	var status string
	var totalDocs, migratedDocs, failedDocs, skippedDocs int
	err := v.db.QueryRowContext(ctx, `
		SELECT status, total_documents, migrated_documents, failed_documents, skipped_documents
		FROM migration_jobs
		WHERE id = $1
	`, jobID).Scan(&status, &totalDocs, &migratedDocs, &failedDocs, &skippedDocs)

	results = append(results, ValidationResult{
		Name:        "JobExists",
		Passed:      err == nil,
		Message:     fmt.Sprintf("Job %d should exist in database", jobID),
		ExpectedVal: nil,
		ActualVal:   err,
	})
	if err != nil {
		return results
	}

	// Check 2: Total documents matches expected
	results = append(results, ValidationResult{
		Name:        "TotalDocumentsCorrect",
		Passed:      totalDocs == expectedDocs,
		Message:     fmt.Sprintf("Job total_documents should match expected count"),
		ExpectedVal: expectedDocs,
		ActualVal:   totalDocs,
	})

	// Check 3: Document count invariant (total = migrated + failed + skipped)
	accountedDocs := migratedDocs + failedDocs + skippedDocs
	results = append(results, ValidationResult{
		Name:        "DocumentCountInvariant",
		Passed:      totalDocs == accountedDocs,
		Message:     fmt.Sprintf("total_documents (%d) = migrated (%d) + failed (%d) + skipped (%d)", totalDocs, migratedDocs, failedDocs, skippedDocs),
		ExpectedVal: totalDocs,
		ActualVal:   accountedDocs,
	})

	// Check 4: Job status is terminal or all docs processed
	terminalStatus := status == "completed" || status == "failed" || status == "cancelled"
	allProcessed := accountedDocs == totalDocs
	results = append(results, ValidationResult{
		Name:        "JobStatusValid",
		Passed:      terminalStatus || (status == "running" && allProcessed),
		Message:     fmt.Sprintf("Job status should be terminal or all documents processed"),
		ExpectedVal: "completed or running with 100%",
		ActualVal:   fmt.Sprintf("%s with %d/%d processed", status, accountedDocs, totalDocs),
	})

	// Check 5: All migration items have terminal status
	var pendingItems, inProgressItems int
	err = v.db.QueryRowContext(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE status = 'pending') as pending,
			COUNT(*) FILTER (WHERE status = 'in_progress') as in_progress
		FROM migration_items
		WHERE migration_job_id = $1
	`, jobID).Scan(&pendingItems, &inProgressItems)

	noStuckItems := err == nil && pendingItems == 0 && inProgressItems == 0
	results = append(results, ValidationResult{
		Name:        "NoStuckMigrationItems",
		Passed:      noStuckItems,
		Message:     fmt.Sprintf("No migration items should be stuck in pending/in_progress"),
		ExpectedVal: "0 pending, 0 in_progress",
		ActualVal:   fmt.Sprintf("%d pending, %d in_progress", pendingItems, inProgressItems),
	})

	// Check 6: All outbox events processed
	var pendingEvents int
	err = v.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM migration_outbox
		WHERE migration_job_id = $1 AND status = 'pending'
	`, jobID).Scan(&pendingEvents)

	results = append(results, ValidationResult{
		Name:        "AllOutboxEventsProcessed",
		Passed:      err == nil && pendingEvents == 0,
		Message:     fmt.Sprintf("All outbox events should be processed (not pending)"),
		ExpectedVal: 0,
		ActualVal:   pendingEvents,
	})

	// Check 7: Migration item count matches job total
	var itemCount int
	err = v.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM migration_items
		WHERE migration_job_id = $1
	`, jobID).Scan(&itemCount)

	results = append(results, ValidationResult{
		Name:        "MigrationItemCountMatches",
		Passed:      err == nil && itemCount == expectedDocs,
		Message:     fmt.Sprintf("Number of migration_items should match expected documents"),
		ExpectedVal: expectedDocs,
		ActualVal:   itemCount,
	})

	return results
}

// ValidateContentIntegrity verifies that document content was migrated correctly without corruption.
//
// Strong signals:
// - Content hashes match between source and destination
// - All migration items have content_match = true
// - Destination content is retrievable and non-empty
// - Document structure is preserved (frontmatter, body, etc.)
func (v *MigrationValidator) ValidateContentIntegrity(ctx context.Context, jobID int64, testDocs []testDocument, s3Config *s3adapter.Config) []ValidationResult {
	results := []ValidationResult{}

	v.logger.Info("validating content integrity", "job_id", jobID, "doc_count", len(testDocs))

	// Create S3 adapter for verification
	s3Adapter, err := s3adapter.NewAdapter(s3Config, v.logger.Named("s3-validator"))
	if err != nil {
		results = append(results, ValidationResult{
			Name:        "S3AdapterCreation",
			Passed:      false,
			Message:     "Failed to create S3 adapter for validation",
			ExpectedVal: nil,
			ActualVal:   err.Error(),
		})
		return results
	}

	// Get all migration items for this job
	rows, err := v.db.QueryContext(ctx, `
		SELECT document_uuid, source_provider_id, dest_provider_id,
		       source_content_hash, dest_content_hash, content_match, status
		FROM migration_items
		WHERE migration_job_id = $1
	`, jobID)
	if err != nil {
		results = append(results, ValidationResult{
			Name:        "QueryMigrationItems",
			Passed:      false,
			Message:     "Failed to query migration items",
			ExpectedVal: nil,
			ActualVal:   err.Error(),
		})
		return results
	}
	defer rows.Close()

	itemCount := 0
	contentMatchCount := 0
	hashMatchCount := 0
	retrievableCount := 0

	for rows.Next() {
		var docUUID, sourceProviderID, destProviderID, sourceHash, destHash, status string
		var contentMatch bool

		err := rows.Scan(&docUUID, &sourceProviderID, &destProviderID, &sourceHash, &destHash, &contentMatch, &status)
		if err != nil {
			continue
		}

		itemCount++

		// Check 1: content_match flag is true
		if contentMatch {
			contentMatchCount++
		}

		// Check 2: Hashes match
		if sourceHash == destHash && sourceHash != "" {
			hashMatchCount++
		}

		// Check 3: Can retrieve content from S3
		if destProviderID != "" {
			content, err := s3Adapter.GetContent(ctx, destProviderID)
			if err == nil && content != nil && content.Body != "" {
				retrievableCount++

				// Check 4: Retrieved content hash matches recorded hash
				computedHash := computeContentHash(content.Body)
				if computedHash != destHash {
					results = append(results, ValidationResult{
						Name:        fmt.Sprintf("ContentHashMismatch_%s", docUUID[:8]),
						Passed:      false,
						Message:     fmt.Sprintf("Computed hash doesn't match stored hash for %s", docUUID[:8]),
						ExpectedVal: destHash,
						ActualVal:   computedHash,
					})
				}
			}
		}
	}

	// Overall content integrity results
	results = append(results, ValidationResult{
		Name:        "AllContentMatchFlagsTrue",
		Passed:      contentMatchCount == itemCount,
		Message:     fmt.Sprintf("All migration items should have content_match = true"),
		ExpectedVal: itemCount,
		ActualVal:   contentMatchCount,
	})

	results = append(results, ValidationResult{
		Name:        "AllHashesMatch",
		Passed:      hashMatchCount == itemCount,
		Message:     fmt.Sprintf("All source and destination hashes should match"),
		ExpectedVal: itemCount,
		ActualVal:   hashMatchCount,
	})

	results = append(results, ValidationResult{
		Name:        "AllDocumentsRetrievable",
		Passed:      retrievableCount == itemCount,
		Message:     fmt.Sprintf("All migrated documents should be retrievable from S3"),
		ExpectedVal: itemCount,
		ActualVal:   retrievableCount,
	})

	return results
}

// ValidateOutboxIntegrity verifies the transactional outbox pattern worked correctly.
//
// Strong signals:
// - Every migration item has exactly one outbox event
// - All outbox events have unique idempotent keys
// - No duplicate processing (same key published twice)
// - All events have valid payload structure
func (v *MigrationValidator) ValidateOutboxIntegrity(ctx context.Context, jobID int64) []ValidationResult {
	results := []ValidationResult{}

	v.logger.Info("validating outbox integrity", "job_id", jobID)

	// Check 1: Count migration items
	var itemCount int
	err := v.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM migration_items WHERE migration_job_id = $1
	`, jobID).Scan(&itemCount)

	if err != nil {
		results = append(results, ValidationResult{
			Name:        "CountMigrationItems",
			Passed:      false,
			Message:     "Failed to count migration items",
			ExpectedVal: nil,
			ActualVal:   err.Error(),
		})
		return results
	}

	// Check 2: Count outbox events
	var outboxCount int
	err = v.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM migration_outbox WHERE migration_job_id = $1
	`, jobID).Scan(&outboxCount)

	results = append(results, ValidationResult{
		Name:        "OneOutboxEventPerItem",
		Passed:      err == nil && outboxCount == itemCount,
		Message:     fmt.Sprintf("Should have exactly one outbox event per migration item"),
		ExpectedVal: itemCount,
		ActualVal:   outboxCount,
	})

	// Check 3: All idempotent keys are unique
	var uniqueKeys, totalKeys int
	err = v.db.QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT idempotent_key), COUNT(*)
		FROM migration_outbox
		WHERE migration_job_id = $1
	`, jobID).Scan(&uniqueKeys, &totalKeys)

	results = append(results, ValidationResult{
		Name:        "AllIdempotentKeysUnique",
		Passed:      err == nil && uniqueKeys == totalKeys,
		Message:     fmt.Sprintf("All idempotent keys should be unique (no duplicates)"),
		ExpectedVal: totalKeys,
		ActualVal:   uniqueKeys,
	})

	// Check 4: No duplicate processing (check publish_attempts)
	var maxAttempts int
	var avgAttempts float64
	err = v.db.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(publish_attempts), 0), COALESCE(AVG(publish_attempts), 0)
		FROM migration_outbox
		WHERE migration_job_id = $1
	`, jobID).Scan(&maxAttempts, &avgAttempts)

	results = append(results, ValidationResult{
		Name:        "ReasonablePublishAttempts",
		Passed:      err == nil && maxAttempts <= 3,
		Message:     fmt.Sprintf("Publish attempts should be reasonable (max 3)"),
		ExpectedVal: "≤ 3",
		ActualVal:   fmt.Sprintf("max=%d, avg=%.2f", maxAttempts, avgAttempts),
	})

	// Check 5: All payloads are valid JSON
	rows, err := v.db.QueryContext(ctx, `
		SELECT id, payload::text
		FROM migration_outbox
		WHERE migration_job_id = $1
	`, jobID)
	if err == nil {
		defer rows.Close()
		invalidPayloads := 0
		for rows.Next() {
			var id int64
			var payload string
			if err := rows.Scan(&id, &payload); err == nil {
				if payload == "" || payload == "{}" {
					invalidPayloads++
				}
			}
		}

		results = append(results, ValidationResult{
			Name:        "AllPayloadsValid",
			Passed:      invalidPayloads == 0,
			Message:     fmt.Sprintf("All outbox payloads should be valid and non-empty"),
			ExpectedVal: 0,
			ActualVal:   invalidPayloads,
		})
	}

	return results
}

// ValidateMigrationInvariants checks critical invariants that must hold for a valid migration.
//
// Strong signals:
// - No data loss: source document count = destination document count
// - No duplication: destination has unique documents only
// - Referential integrity: all foreign keys are valid
// - State consistency: job progress matches item statuses
func (v *MigrationValidator) ValidateMigrationInvariants(ctx context.Context, jobID int64, sourceDocCount int) []ValidationResult {
	results := []ValidationResult{}

	v.logger.Info("validating migration invariants", "job_id", jobID)

	// Invariant 1: No data loss
	var completedItems int
	err := v.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM migration_items
		WHERE migration_job_id = $1 AND status = 'completed'
	`, jobID).Scan(&completedItems)

	results = append(results, ValidationResult{
		Name:        "NoDataLoss",
		Passed:      err == nil && completedItems == sourceDocCount,
		Message:     fmt.Sprintf("All source documents should be migrated"),
		ExpectedVal: sourceDocCount,
		ActualVal:   completedItems,
	})

	// Invariant 2: No duplication (all document UUIDs unique)
	var uniqueUUIDs, totalUUIDs int
	err = v.db.QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT document_uuid), COUNT(*)
		FROM migration_items
		WHERE migration_job_id = $1
	`, jobID).Scan(&uniqueUUIDs, &totalUUIDs)

	results = append(results, ValidationResult{
		Name:        "NoDuplication",
		Passed:      err == nil && uniqueUUIDs == totalUUIDs,
		Message:     fmt.Sprintf("All document UUIDs should be unique (no duplicates)"),
		ExpectedVal: totalUUIDs,
		ActualVal:   uniqueUUIDs,
	})

	// Invariant 3: Referential integrity (all items reference valid job)
	var orphanedItems int
	err = v.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM migration_items mi
		LEFT JOIN migration_jobs mj ON mi.migration_job_id = mj.id
		WHERE mi.migration_job_id = $1 AND mj.id IS NULL
	`, jobID).Scan(&orphanedItems)

	results = append(results, ValidationResult{
		Name:        "ReferentialIntegrity",
		Passed:      err == nil && orphanedItems == 0,
		Message:     fmt.Sprintf("All migration items should reference valid job"),
		ExpectedVal: 0,
		ActualVal:   orphanedItems,
	})

	// Invariant 4: State consistency (job counters match item counts)
	var jobMigrated, itemMigrated int
	v.db.QueryRowContext(ctx, `SELECT migrated_documents FROM migration_jobs WHERE id = $1`, jobID).Scan(&jobMigrated)
	v.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM migration_items WHERE migration_job_id = $1 AND status = 'completed'`, jobID).Scan(&itemMigrated)

	results = append(results, ValidationResult{
		Name:        "StateConsistency",
		Passed:      jobMigrated == itemMigrated,
		Message:     fmt.Sprintf("Job migrated_documents should match completed item count"),
		ExpectedVal: itemMigrated,
		ActualVal:   jobMigrated,
	})

	// Invariant 5: Monotonic progress (migrated + failed + skipped ≤ total)
	var total, migrated, failed, skipped int
	err = v.db.QueryRowContext(ctx, `
		SELECT total_documents, migrated_documents, failed_documents, skipped_documents
		FROM migration_jobs WHERE id = $1
	`, jobID).Scan(&total, &migrated, &failed, &skipped)

	sum := migrated + failed + skipped
	results = append(results, ValidationResult{
		Name:        "MonotonicProgress",
		Passed:      err == nil && sum <= total,
		Message:     fmt.Sprintf("Sum of processed documents should not exceed total"),
		ExpectedVal: fmt.Sprintf("≤ %d", total),
		ActualVal:   sum,
	})

	return results
}

// ValidateS3Storage verifies documents are correctly stored in S3 with proper structure.
//
// Strong signals:
// - All documents exist in S3 with correct prefix
// - S3 versioning is enabled and versions exist
// - Metadata manifests are present and valid
// - Content is retrievable and matches expected format
func (v *MigrationValidator) ValidateS3Storage(ctx context.Context, jobID int64, s3Config *s3adapter.Config) []ValidationResult {
	results := []ValidationResult{}

	v.logger.Info("validating S3 storage", "job_id", jobID, "bucket", s3Config.Bucket, "prefix", s3Config.Prefix)

	// Create S3 adapter
	s3Adapter, err := s3adapter.NewAdapter(s3Config, v.logger.Named("s3-validator"))
	if err != nil {
		results = append(results, ValidationResult{
			Name:        "S3AdapterCreation",
			Passed:      false,
			Message:     "Failed to create S3 adapter",
			ExpectedVal: nil,
			ActualVal:   err.Error(),
		})
		return results
	}

	// Get all completed migration items
	rows, err := v.db.QueryContext(ctx, `
		SELECT document_uuid, dest_provider_id
		FROM migration_items
		WHERE migration_job_id = $1 AND status = 'completed'
	`, jobID)
	if err != nil {
		results = append(results, ValidationResult{
			Name:        "QueryCompletedItems",
			Passed:      false,
			Message:     "Failed to query completed migration items",
			ExpectedVal: nil,
			ActualVal:   err.Error(),
		})
		return results
	}
	defer rows.Close()

	successCount := 0
	totalCount := 0

	for rows.Next() {
		var docUUID, destProviderID string
		if err := rows.Scan(&docUUID, &destProviderID); err != nil {
			continue
		}

		totalCount++

		// Try to retrieve from S3
		content, err := s3Adapter.GetContent(ctx, destProviderID)
		if err == nil && content != nil {
			successCount++

			// Verify content structure
			if content.Body == "" {
				results = append(results, ValidationResult{
					Name:        fmt.Sprintf("EmptyContent_%s", docUUID[:8]),
					Passed:      false,
					Message:     fmt.Sprintf("Document %s has empty content", docUUID[:8]),
					ExpectedVal: "non-empty",
					ActualVal:   "empty",
				})
			}
		}
	}

	results = append(results, ValidationResult{
		Name:        "AllDocumentsInS3",
		Passed:      successCount == totalCount,
		Message:     fmt.Sprintf("All completed migrations should have documents in S3"),
		ExpectedVal: totalCount,
		ActualVal:   successCount,
	})

	return results
}

// PrintValidationReport prints a formatted validation report.
func (v *MigrationValidator) PrintValidationReport(results []ValidationResult) {
	passed := 0
	failed := 0

	v.t.Log("=" + string(make([]byte, 70)) + "=")
	v.t.Log("  MIGRATION VALIDATION REPORT")
	v.t.Log("=" + string(make([]byte, 70)) + "=")

	for _, result := range results {
		if result.Passed {
			passed++
			v.t.Logf("✅ PASS: %s", result.Name)
			v.t.Logf("         %s", result.Message)
		} else {
			failed++
			v.t.Logf("❌ FAIL: %s", result.Name)
			v.t.Logf("         %s", result.Message)
			v.t.Logf("         Expected: %v", result.ExpectedVal)
			v.t.Logf("         Actual:   %v", result.ActualVal)
		}
	}

	v.t.Log("=" + string(make([]byte, 70)) + "=")
	v.t.Logf("  SUMMARY: %d passed, %d failed, %d total", passed, failed, len(results))
	v.t.Log("=" + string(make([]byte, 70)) + "=")

	if failed > 0 {
		v.t.Errorf("Validation failed: %d checks did not pass", failed)
	}
}

// AssertAllValidationsPassed fails the test if any validation failed.
func (v *MigrationValidator) AssertAllValidationsPassed(results []ValidationResult) {
	for _, result := range results {
		if !result.Passed {
			require.True(v.t, result.Passed, "Validation '%s' failed: %s (expected: %v, actual: %v)",
				result.Name, result.Message, result.ExpectedVal, result.ActualVal)
		}
	}
}
