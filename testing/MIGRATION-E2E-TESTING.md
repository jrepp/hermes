# RFC-089 Migration E2E Integration Testing Guide

## Overview

This guide explains how to run comprehensive end-to-end integration tests for the RFC-089 S3 storage backend and document migration system.

## Test Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    Integration Test Suite                        │
│  tests/integration/migration/migration_e2e_test.go               │
└────────────────┬────────────────────────────────────────────────┘
                 │
                 ├─── Phase 1: Prerequisites
                 │    • Verify database tables
                 │    • Check MinIO availability
                 │
                 ├─── Phase 2: Provider Registration
                 │    • Register mock source provider
                 │    • Register S3 destination provider
                 │
                 ├─── Phase 3: Create Test Documents
                 │    • Generate 5 test documents
                 │    • Calculate content hashes
                 │
                 ├─── Phase 4: Migration Job Creation
                 │    • Create job in database
                 │    • Configure copy strategy
                 │
                 ├─── Phase 5: Queue Documents
                 │    • Create migration items
                 │    • Create outbox events (transactional)
                 │
                 ├─── Phase 6: Start Migration Job
                 │    • Update job status to 'running'
                 │
                 ├─── Phase 7: Worker Processing
                 │    • Start migration worker
                 │    • Process outbox events
                 │    • Execute migrations
                 │    • Validate content
                 │
                 ├─── Phase 8: Verify Results
                 │    • Check documents in S3
                 │    • Verify content hashes match
                 │    • Validate migration_items records
                 │
                 ├─── Phase 9: Progress Tracking
                 │    • Verify job status
                 │    • Check counters (migrated/failed)
                 │    • Validate outbox processing
                 │
                 └─── Phase 10: Cleanup
                      • Delete test data from database
                      • Leave S3 documents for inspection
```

## Prerequisites

### 1. Required Services

The tests require the following services to be running:

- **PostgreSQL** (port 5433) - Managed by test fixture
- **MinIO** (port 9000) - S3-compatible object storage
- **Docker** - For running containers

### 2. Database Migrations

Ensure all database migrations are applied, especially:
- `000011_add_s3_migration_tables.up.sql` - RFC-089 tables

```bash
# Apply migrations
make db-migrate

# Or manually
go run main.go migrate -config testing/config-central.hcl
```

### 3. MinIO Setup

MinIO must be running and configured:

```bash
# Start MinIO via docker-compose
cd testing
docker compose up -d minio

# Verify MinIO is running
curl http://localhost:9000/minio/health/live

# Access MinIO Console (optional)
open http://localhost:9001
# Login: minioadmin / minioadmin
```

## Running the Tests

### Option 1: Use the Convenience Script (Recommended)

The easiest way to run the migration e2e tests:

```bash
cd testing
./test-migration-e2e.sh
```

This script will:
1. ✅ Check if Docker is running
2. ✅ Verify required containers are running (starts them if needed)
3. ✅ Check service connectivity (PostgreSQL, MinIO)
4. ✅ Verify MinIO bucket exists and versioning is enabled
5. ✅ Verify database migrations are applied
6. ✅ Run Go integration tests with proper environment variables
7. ✅ Display detailed test results and next steps

### Option 2: Run Go Tests Directly

For more control or debugging:

```bash
# Set environment variables
export INTEGRATION_TEST=1
export POSTGRES_HOST=localhost
export POSTGRES_PORT=5433
export POSTGRES_USER=postgres
export POSTGRES_PASSWORD=postgres
export POSTGRES_DB=hermes_test
export MINIO_ENDPOINT=http://localhost:9000
export MINIO_ACCESS_KEY=minioadmin
export MINIO_SECRET_KEY=minioadmin

# Run tests
cd /path/to/hermes
go test -v -tags=integration -timeout=5m ./tests/integration/migration/...
```

### Option 3: Run Specific Test Phases

You can run specific test phases:

```bash
# Run only a specific phase
go test -v -tags=integration ./tests/integration/migration/ -run TestMigrationE2E/Phase1_Prerequisites

# Run multiple phases
go test -v -tags=integration ./tests/integration/migration/ -run "TestMigrationE2E/(Phase1|Phase2)"
```

## Test Coverage

### What's Tested

✅ **Provider Registration**
- Source provider (mock) registration
- Destination provider (S3) registration
- Provider status and capabilities

✅ **Migration Job Lifecycle**
- Job creation with proper configuration
- Status transitions (pending → running → completed)
- Progress tracking (total, migrated, failed, skipped)

✅ **Document Migration**
- Copying documents between providers
- Content preservation and validation
- SHA-256 hash verification

✅ **Transactional Outbox Pattern**
- Atomic creation of migration_items + outbox events
- Idempotency keys prevent duplicate processing
- Event status tracking (pending → published)

✅ **Worker Processing**
- Outbox polling and event consumption
- Concurrent document processing
- Retry logic for failures
- Progress updates

✅ **Content Validation**
- SHA-256 content hash calculation
- Source vs destination content comparison
- content_match flag verification

✅ **S3 Integration**
- Document creation in MinIO
- Version tracking (S3 versioning enabled)
- Metadata storage (manifest strategy)
- Content retrieval and verification

✅ **Database Integrity**
- Foreign key relationships maintained
- Transaction atomicity
- Cascade deletes work correctly

### What's NOT Tested (Future Work)

⏳ **Move Strategy**
- Document deletion from source after migration
- Requires additional cleanup logic

⏳ **Mirror Strategy**
- Bi-directional synchronization
- Conflict resolution
- Real-time updates

⏳ **Error Scenarios**
- Network failures during migration
- S3 quota exceeded
- Provider authentication failures
- Partial migration recovery

⏳ **Performance Testing**
- Large document sets (1000+ documents)
- Concurrent migration jobs
- Memory usage under load

⏳ **API Integration**
- REST API endpoints for job management
- WebSocket progress updates
- Authentication/authorization

## Test Output

### Successful Test Run

```
=== RUN   TestMigrationE2E
=== RUN   TestMigrationE2E/Phase1_Prerequisites
    === Phase 1: Prerequisites ===
    ✓ Table provider_storage exists
    ✓ Table migration_jobs exists
    ✓ Table migration_items exists
    ✓ Table migration_outbox exists
    ✅ All prerequisites met

=== RUN   TestMigrationE2E/Phase2_ProviderRegistration
    === Phase 2: Provider Registration ===
    ✓ Cleaned up existing test data
    ✓ Registered source provider (ID: 1)
    ✓ Registered destination provider (ID: 2)
    ✅ Provider registration complete

=== RUN   TestMigrationE2E/Phase3_CreateTestDocuments
    === Phase 3: Create Test Documents ===
    ✓ Created test document 1 (UUID: 7e8f4a2c..., Hash: abc12345...)
    ✓ Created test document 2 (UUID: 9c1a3b5d..., Hash: def67890...)
    [...]
    ✅ Created 5 test documents

=== RUN   TestMigrationE2E/Phase7_WorkerProcessing
    === Phase 7: Worker Processing ===
    ✓ Worker started, processing tasks...
      Progress: 2/5 migrated, 0 failed, 3 pending
      Progress: 4/5 migrated, 0 failed, 1 pending
      Progress: 5/5 migrated, 0 failed, 0 pending
    ✓ Final results: 5 migrated, 0 failed out of 5 total
    ✅ Worker processing complete

[...]

--- PASS: TestMigrationE2E (12.34s)
    --- PASS: TestMigrationE2E/Phase1_Prerequisites (0.05s)
    --- PASS: TestMigrationE2E/Phase2_ProviderRegistration (0.12s)
    --- PASS: TestMigrationE2E/Phase3_CreateTestDocuments (0.02s)
    --- PASS: TestMigrationE2E/Phase4_MigrationJobCreation (0.03s)
    --- PASS: TestMigrationE2E/Phase5_QueueDocuments (0.15s)
    --- PASS: TestMigrationE2E/Phase6_StartMigrationJob (0.02s)
    --- PASS: TestMigrationE2E/Phase7_WorkerProcessing (8.45s)
    --- PASS: TestMigrationE2E/Phase8_VerifyMigrationResults (2.10s)
    --- PASS: TestMigrationE2E/Phase9_ProgressTracking (0.03s)
    --- PASS: TestMigrationE2E/Phase10_Cleanup (0.08s)
PASS
ok      github.com/hashicorp-forge/hermes/tests/integration/migration   13.456s
```

## Troubleshooting

### Test Fails: "Table does not exist"

**Problem:** Migration 000011 not applied

**Solution:**
```bash
# Check current migration version
docker exec hermes-postgres psql -U postgres -d hermes_test -c \
  "SELECT version FROM schema_migrations ORDER BY version DESC LIMIT 1"

# Apply migrations
make db-migrate
```

### Test Fails: "MinIO is not accessible"

**Problem:** MinIO container not running

**Solution:**
```bash
# Check if MinIO is running
docker ps | grep minio

# Start MinIO
cd testing
docker compose up -d minio

# Check logs
docker compose logs minio
```

### Test Fails: "Failed to create S3 adapter"

**Problem:** MinIO credentials incorrect or bucket doesn't exist

**Solution:**
```bash
# Verify MinIO is accessible
curl http://localhost:9000/minio/health/live

# Create bucket manually
docker exec hermes-minio mc mb myminio/hermes-documents
docker exec hermes-minio mc version enable myminio/hermes-documents

# Or restart minio-setup
docker compose up -d minio-setup
```

### Test Hangs During Worker Processing

**Problem:** Worker not processing outbox events

**Solution:**
```bash
# Check outbox events status
docker exec hermes-postgres psql -U postgres -d hermes_test -c \
  "SELECT status, COUNT(*) FROM migration_outbox GROUP BY status"

# Check if events are stuck in 'pending'
# This might indicate a worker issue - check test logs

# Manually verify worker can connect to database
# The test should fail after 30 seconds timeout
```

### Test Fails: "Content hash mismatch"

**Problem:** Document content changed during migration

**Solution:**
- This indicates a bug in the migration code
- Check S3 adapter logs for encoding issues
- Verify no content transformation is happening unexpectedly
- Compare source and destination content manually

### S3 Documents Not Cleaned Up

**Problem:** Previous test runs left documents in MinIO

**Solution:**
```bash
# List documents
docker exec hermes-minio mc ls myminio/hermes-documents/e2e-test/

# Remove test documents
docker exec hermes-minio mc rm --recursive --force myminio/hermes-documents/e2e-test/

# Or via MinIO Console
open http://localhost:9001
# Navigate to hermes-documents bucket, delete e2e-test prefix
```

## Test Data

### Test Documents

Each test run creates 5 test documents:

- **Format:** Markdown
- **Naming:** "Test Migration Doc 1" through "Test Migration Doc 5"
- **Content:** Simple markdown with title, timestamp, description
- **UUID:** Randomly generated for each run
- **Hash:** SHA-256 of content

### Database Records

After a successful test run:

```sql
-- Check providers (should be cleaned up)
SELECT * FROM provider_storage WHERE provider_name LIKE 'e2e-test-%';

-- Check jobs (should be cleaned up)
SELECT * FROM migration_jobs WHERE job_name LIKE 'e2e-test-%';

-- Check items (should be cleaned up)
SELECT * FROM migration_items WHERE migration_job_id IN (
  SELECT id FROM migration_jobs WHERE job_name LIKE 'e2e-test-%'
);

-- Check outbox (should be cleaned up)
SELECT * FROM migration_outbox WHERE migration_job_id IN (
  SELECT id FROM migration_jobs WHERE job_name LIKE 'e2e-test-%'
);
```

### S3 Objects

Test documents are stored in MinIO:

- **Bucket:** `hermes-documents`
- **Prefix:** `e2e-test/`
- **Format:** `e2e-test/{project}/{uuid}.md`
- **Metadata:** Stored in `.metadata.json` manifest files
- **Versioning:** Enabled (can see version history)

**Inspect via MinIO Console:**
```bash
open http://localhost:9001
# Login: minioadmin / minioadmin
# Navigate to: hermes-documents bucket → e2e-test prefix
```

**Inspect via MinIO CLI:**
```bash
# List all test documents
docker exec hermes-minio mc ls --recursive myminio/hermes-documents/e2e-test/

# View a specific document
docker exec hermes-minio mc cat myminio/hermes-documents/e2e-test/{path}/doc.md

# View document versions
docker exec hermes-minio mc ls --versions myminio/hermes-documents/e2e-test/{path}/doc.md
```

## Integration with CI/CD

### GitHub Actions Example

```yaml
name: Migration E2E Tests

on:
  push:
    branches: [main, develop]
  pull_request:
    branches: [main, develop]

jobs:
  migration-e2e:
    runs-on: ubuntu-latest

    services:
      postgres:
        image: postgres:15-alpine
        env:
          POSTGRES_PASSWORD: postgres
          POSTGRES_DB: hermes_test
        ports:
          - 5433:5432
        options: >-
          --health-cmd pg_isready
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5

      minio:
        image: minio/minio:latest
        env:
          MINIO_ROOT_USER: minioadmin
          MINIO_ROOT_PASSWORD: minioadmin
        ports:
          - 9000:9000
        options: >-
          --health-cmd "curl -f http://localhost:9000/minio/health/live"
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5
        command: server /data

    steps:
      - uses: actions/checkout@v3

      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.21'

      - name: Setup MinIO bucket
        run: |
          docker run --rm --network=host minio/mc \
            alias set myminio http://localhost:9000 minioadmin minioadmin
          docker run --rm --network=host minio/mc \
            mb myminio/hermes-documents --ignore-existing
          docker run --rm --network=host minio/mc \
            version enable myminio/hermes-documents

      - name: Apply migrations
        run: |
          make db-migrate
        env:
          POSTGRES_HOST: localhost
          POSTGRES_PORT: 5433

      - name: Run Migration E2E Tests
        run: |
          go test -v -tags=integration -timeout=10m ./tests/integration/migration/...
        env:
          INTEGRATION_TEST: 1
          POSTGRES_HOST: localhost
          POSTGRES_PORT: 5433
          POSTGRES_USER: postgres
          POSTGRES_PASSWORD: postgres
          POSTGRES_DB: hermes_test
          MINIO_ENDPOINT: http://localhost:9000
          MINIO_ACCESS_KEY: minioadmin
          MINIO_SECRET_KEY: minioadmin
```

## Performance Benchmarks

Expected performance on standard development hardware:

| Phase | Duration | Notes |
|-------|----------|-------|
| Phase 1-6 | ~0.5s | Database operations |
| Phase 7 (Worker) | ~8-10s | 5 documents, 3 workers |
| Phase 8 (Verify) | ~2-3s | S3 content retrieval |
| Phase 9-10 | ~0.2s | Cleanup |
| **Total** | **~12-15s** | Full test suite |

**Scaling:**
- 10 documents: ~15-20s
- 50 documents: ~45-60s
- 100 documents: ~90-120s

## Related Documentation

- **RFC-089:** [docs-internal/rfc/RFC-089-s3-storage-backend-and-migrations.md](../docs-internal/rfc/RFC-089-s3-storage-backend-and-migrations.md)
- **Implementation Summary:** [docs-internal/rfc/RFC-089-IMPLEMENTATION-SUMMARY.md](../docs-internal/rfc/RFC-089-IMPLEMENTATION-SUMMARY.md)
- **Testing Guide:** [testing/RFC-089-TESTING-GUIDE.md](RFC-089-TESTING-GUIDE.md)
- **API Tests:** [testing/test-rfc089-api.sh](test-rfc089-api.sh)
- **Worker Tests:** [testing/test-migration-worker.sh](test-migration-worker.sh)

## Contributing

When adding new migration features, please:

1. Add test phases to `migration_e2e_test.go`
2. Update this documentation
3. Ensure tests are idempotent (can run multiple times)
4. Clean up test data in Phase 10
5. Use descriptive phase names with emoji indicators
6. Add helpful error messages for common failures

## Support

For issues or questions:
- Check troubleshooting section above
- Review test logs for detailed error messages
- Check docker-compose logs: `docker compose logs postgres minio`
- Open an issue with test output and environment details
