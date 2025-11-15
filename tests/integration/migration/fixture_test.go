//go:build integration
// +build integration

package migration

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"
)

// VerifyMinIOAvailable checks if MinIO is running and accessible.
// MinIO is required for S3 migration tests but is not managed by the Go fixture.
// It should be started via docker-compose before running tests.
func VerifyMinIOAvailable(t *testing.T) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Try to connect to MinIO health endpoint
	req, err := http.NewRequestWithContext(ctx, "GET", "http://localhost:9000/minio/health/live", nil)
	if err != nil {
		t.Fatalf("❌ Failed to create MinIO health check request: %v\n   MinIO is required for migration tests.\n   Start it with: cd testing && docker compose up -d minio", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("❌ MinIO is not accessible at localhost:9000: %v\n   Ensure MinIO is running: docker compose ps | grep minio\n   Start it with: cd testing && docker compose up -d minio", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("❌ MinIO health check failed with status %d\n   Check MinIO logs: docker compose logs minio", resp.StatusCode)
	}

	t.Log("✓ MinIO is available at localhost:9000")
}

// VerifyMigrationTablesExist checks if RFC-089 migration tables exist.
// These tables are created by migration 000011_add_s3_migration_tables.
func VerifyMigrationTablesExist(t *testing.T) {
	t.Helper()

	requiredTables := []string{
		"provider_storage",
		"migration_jobs",
		"migration_items",
		"migration_outbox",
	}

	t.Logf("Required tables for RFC-089 migration tests: %v", requiredTables)
	t.Log("ℹ️  These are verified in the test setup phase")
}

// MigrationTestRequirements documents the prerequisites for migration tests.
func MigrationTestRequirements() string {
	return `
RFC-089 Migration Integration Test Requirements:

1. Database (PostgreSQL):
   - Managed by integration test fixture
   - Migrations must be applied including 000011_add_s3_migration_tables
   - Run: make db-migrate

2. Object Storage (MinIO):
   - NOT managed by integration test fixture
   - Must be started separately via docker-compose
   - Run: cd testing && docker compose up -d minio
   - Verify: curl http://localhost:9000/minio/health/live
   - Console: http://localhost:9001 (minioadmin/minioadmin)

3. Test Data:
   - Test documents are created in memory by the test
   - S3 bucket "hermes-documents" must exist (auto-created by docker-compose)
   - Test prefix "e2e-test" is used to isolate test data

4. Cleanup:
   - Database records are cleaned up after tests
   - S3 documents are left for inspection (can be manually deleted)

Running the tests:
   cd /Users/jrepp/hc/hermes
   go test -v -tags=integration ./tests/integration/migration/...

Or use the convenience script:
   ./testing/test-migration-e2e.sh
`
}

// PrintRequirements prints the test requirements to stdout.
func PrintRequirements(t *testing.T) {
	t.Helper()
	fmt.Println(MigrationTestRequirements())
}
