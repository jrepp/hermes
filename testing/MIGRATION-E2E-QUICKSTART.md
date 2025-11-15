# RFC-089 Migration E2E Tests - Quick Start

## TL;DR

```bash
# 1. Start services
cd testing
docker compose up -d postgres minio

# 2. Apply migrations
make db-migrate

# 3. Run tests
./test-migration-e2e.sh
```

## Expected Output

```
=========================================
RFC-089 Migration E2E Integration Tests
=========================================

Step 1: Checking prerequisites
---------------------------------------
✓ Docker is running
✓ docker-compose.yml found
✓ Container 'hermes-postgres' is running
✓ Container 'hermes-minio' is running

Step 2: Checking service connectivity
---------------------------------------
Checking PostgreSQL... ✓ Available
Checking MinIO... ✓ Available

Step 3: Verifying MinIO configuration
---------------------------------------
✓ MinIO bucket 'hermes-documents' exists
ℹ️  MinIO Console: http://localhost:9001

Step 4: Verifying database migrations
---------------------------------------
✓ Migration 000011 (RFC-089 tables) is applied
Checking RFC-089 tables... ✓ All required tables exist

Step 5: Running Go integration tests
---------------------------------------
Running: go test -v -tags=integration ./tests/integration/migration/...

=== RUN   TestMigrationE2E
=== RUN   TestMigrationE2E/Phase1_Prerequisites
    === Phase 1: Prerequisites ===
    ✓ Table provider_storage exists
    ✓ Table migration_jobs exists
    ✓ Table migration_items exists
    ✓ Table migration_outbox exists
    ✅ All prerequisites met
[... more test output ...]

✅ All Migration E2E Tests Passed!

Additional Information:
  PostgreSQL:     localhost:5433
  MinIO S3 API:   http://localhost:9000
  MinIO Console:  http://localhost:9001
  Test Bucket:    hermes-documents
  Test Prefix:    e2e-test
```

## What Gets Tested?

- ✅ Provider registration (source + destination)
- ✅ Migration job creation and lifecycle
- ✅ Document queuing with transactional outbox pattern
- ✅ Worker processing with concurrent execution
- ✅ Content validation via SHA-256 hashing
- ✅ S3 document storage and retrieval
- ✅ Progress tracking and status updates
- ✅ Complete end-to-end migration flow

## Test Duration

- Full test suite: ~12-15 seconds
- 5 test documents migrated from mock → S3
- 10 test phases executed sequentially

## Requirements

| Service | Port | Status |
|---------|------|--------|
| PostgreSQL | 5433 | Required |
| MinIO | 9000 | Required |
| Docker | - | Required |

## Common Issues

### "Container not running"
```bash
docker compose up -d postgres minio
```

### "Migration 000011 not applied"
```bash
make db-migrate
```

### "MinIO bucket doesn't exist"
```bash
docker compose up -d minio-setup
```

## Manual Test Runs

### Run specific test phase
```bash
cd /path/to/hermes
export INTEGRATION_TEST=1
go test -v -tags=integration ./tests/integration/migration/ \
  -run TestMigrationE2E/Phase7_WorkerProcessing
```

### Run with debug logging
```bash
export INTEGRATION_TEST=1
export LOG_LEVEL=debug
go test -v -tags=integration ./tests/integration/migration/... 2>&1 | tee test.log
```

### Run in short mode (skips e2e)
```bash
go test -v -tags=integration -short ./tests/integration/migration/...
```

## Inspect Test Results

### Check PostgreSQL
```bash
docker exec -it hermes-postgres psql -U postgres -d hermes_test

# View migration jobs
SELECT * FROM migration_jobs WHERE job_name LIKE 'e2e-test-%';

# View migration items
SELECT mi.*, mj.job_name
FROM migration_items mi
JOIN migration_jobs mj ON mi.migration_job_id = mj.id
WHERE mj.job_name LIKE 'e2e-test-%';
```

### Check MinIO (S3)
```bash
# Via MinIO Console
open http://localhost:9001
# Login: minioadmin / minioadmin
# Browse: hermes-documents → e2e-test

# Via MinIO CLI
docker exec hermes-minio mc ls --recursive myminio/hermes-documents/e2e-test/
```

## Next Steps

After successful test run:
- Review full documentation: [MIGRATION-E2E-TESTING.md](MIGRATION-E2E-TESTING.md)
- Run API tests: `./test-rfc089-api.sh`
- Run worker tests: `./test-migration-worker.sh`
- Try comprehensive e2e: `./test-comprehensive-e2e.sh`

## Files Created

```
tests/integration/migration/
├── main_test.go                 # Test entry point
├── migration_e2e_test.go        # Comprehensive e2e test (10 phases)
└── fixture_test.go              # Test fixtures and helpers

testing/
├── test-migration-e2e.sh        # Convenience script (this runs everything)
├── MIGRATION-E2E-TESTING.md     # Full documentation
└── MIGRATION-E2E-QUICKSTART.md  # This file
```

## Test Standards

This test follows Hermes testing standards:

- ✅ Uses `//go:build integration` tag
- ✅ Uses testify/require and testify/assert
- ✅ Organized in sequential phases
- ✅ Emoji indicators for clarity
- ✅ Comprehensive error messages
- ✅ Cleanup after execution
- ✅ Idempotent (can run multiple times)
- ✅ Fast feedback (~12-15s total)

## Help

For detailed troubleshooting and advanced usage, see:
- [MIGRATION-E2E-TESTING.md](MIGRATION-E2E-TESTING.md)
- [RFC-089-TESTING-GUIDE.md](RFC-089-TESTING-GUIDE.md)
