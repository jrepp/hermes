# RFC-089 Migration E2E Integration Tests (Go-Based)

## Quick Start

```bash
# Start services
make test-services-up

# Run migrations (if needed)
make db-migrate-test

# Run migration tests
make test-migration
```

That's it! No bash scripts required.

## Available Make Commands

### Test Execution

```bash
# Run full migration e2e test suite (verbose, ~12-15s)
make test-migration

# Run migration tests without verbose output (faster)
make test-migration-quick

# Run specific test phase
make test-migration-phase PHASE=Phase7_WorkerProcessing
make test-migration-phase PHASE=Phase9b_StrongSignalValidation
```

### Service Management

```bash
# Start required services (PostgreSQL, MinIO, Redpanda)
make test-services-up

# Stop all test services
make test-services-down

# View service logs
make test-services-logs
```

### Database Management

```bash
# Run database migrations
make db-migrate

# Run migrations for test environment specifically
make db-migrate-test
```

### General Testing

```bash
# Run all tests (unit + integration)
make test

# Run all integration tests
make test-integration

# Run tests with coverage report
make test-coverage
```

## Test Architecture

### Pure Go Implementation

All testing logic is implemented in Go:

```
tests/integration/migration/
├── main_test.go              - Test entry point
├── prerequisites_test.go     - Service health checks (NEW)
├── migration_e2e_test.go     - 10-phase e2e test
├── validation_test.go        - Strong signal validation
└── fixture_test.go           - Test fixtures
```

### No Bash Scripts Required

The Go tests handle everything:
- ✅ Docker container checks
- ✅ Service health verification
- ✅ Database connectivity
- ✅ Migration table validation
- ✅ MinIO bucket setup
- ✅ Automatic service startup attempts
- ✅ Clear error messages with remediation steps

## Test Phases

### Phase 0: Prerequisites (Automatic)

Before running any tests, the framework automatically checks:

```go
checker := NewPrerequisiteChecker(t)
checker.CheckAll(ctx)
```

**What gets checked:**
1. ✅ Docker daemon is running
2. ✅ Required containers are running (postgres, minio)
3. ✅ Service connectivity (ports accessible)
4. ✅ Database connection
5. ✅ Migration tables exist
6. ✅ MinIO bucket exists and versioning enabled

**If prerequisites fail:**
- Clear error messages explain what's missing
- Actionable remediation steps provided
- Tests fail fast with helpful context

### Phase 1-10: Test Execution

Once prerequisites pass:
1. Database Prerequisites
2. Provider Registration
3. Create Test Documents
4. Migration Job Creation
5. Queue Documents
6. Start Migration Job
7. Worker Processing
8. Verify Results
9. Progress Tracking
9b. **Strong Signal Validation** (27+ checks)
10. Cleanup

## Example Output

```
=== RUN   TestMigrationE2E
=== RUN   TestMigrationE2E/Phase0_Prerequisites
    === Checking Prerequisites ===
    Checking Docker daemon...
    ✓ Docker is running
    Checking required containers...
    ✓ Postgres container is running
    ✓ Minio container is running
    Checking service connectivity...
    ✓ PostgreSQL is accessible
    ✓ MinIO is accessible
    Checking database connection...
    ✓ Database connection successful
    Checking migration tables...
    ✓ Table provider_storage exists
    ✓ Table migration_jobs exists
    ✓ Table migration_items exists
    ✓ Table migration_outbox exists
    ✓ Database migrations up to date (version 11)
    Checking MinIO bucket...
    ✓ Bucket 'hermes-documents' exists
    ✓ Bucket versioning is enabled
    ✅ All prerequisites met
    === Service Information ===
    PostgreSQL:    postgres://postgres:postgres@localhost:5433/hermes_testing?sslmode=disable
    MinIO S3 API:  http://localhost:9000
    MinIO Console: http://localhost:9001 (minioadmin/minioadmin)
    Test Bucket:   hermes-documents
    ===========================

=== RUN   TestMigrationE2E/Phase1_DatabasePrerequisites
    === Phase 1: Database Prerequisites ===
    ✓ Table provider_storage exists
    ✓ Table migration_jobs exists
    ✓ Table migration_items exists
    ✓ Table migration_outbox exists
    ✅ Database prerequisites met

[... test execution continues ...]

=== RUN   TestMigrationE2E/Phase9b_StrongSignalValidation
    === Phase 9b: Strong Signal Validation ===
    Running validation: Job Completeness
      ✓ JobExists
      ✓ TotalDocumentsCorrect
      ✓ DocumentCountInvariant
      [... 24 more checks ...]

    ======================================================================
      MIGRATION VALIDATION REPORT
    ======================================================================
    ✅ PASS: JobExists
    [... 27 passing validations ...]
    ======================================================================
      SUMMARY: 27 passed, 0 failed, 27 total
    ======================================================================

--- PASS: TestMigrationE2E (12.45s)
PASS
```

## Error Handling

### Container Not Running

```
❌ Postgres container is not running
   Run: cd testing && docker compose up -d postgres
```

**Fix:**
```bash
make test-services-up
```

### Migration Tables Missing

```
❌ Required table 'migration_jobs' does not exist
   Migration 000011_add_s3_migration_tables is not applied
   Run: make db-migrate
```

**Fix:**
```bash
make db-migrate-test
```

### MinIO Bucket Missing

```
⚠️  Bucket 'hermes-documents' does not exist, creating...
✓ Bucket 'hermes-documents' created with versioning enabled
```

The test **automatically creates and configures** the bucket!

## Advantages Over Bash Scripts

### 1. Type Safety

```go
// Compile-time type checking
checker := NewPrerequisiteChecker(t)
db := checker.checkDatabaseConnection(ctx)  // Returns *sql.DB
```

vs bash:
```bash
# No type checking, runtime errors
DB_URL="..."
```

### 2. Better Error Handling

```go
if err := db.PingContext(ctx); err != nil {
    t.Fatalf("❌ Failed to ping database: %v\n   Run: docker compose logs postgres", err)
}
```

vs bash:
```bash
# Hard to provide context
psql $DB_URL || exit 1
```

### 3. Integrated Test Framework

```go
require.NoError(t, err, "Database connection should succeed")
assert.Equal(t, 5, migratedCount, "All documents should be migrated")
```

vs bash:
```bash
# Manual assertions
if [ "$COUNT" != "5" ]; then
    echo "Test failed"
    exit 1
fi
```

### 4. Single Language

- No shell script debugging
- No bash quirks or portability issues
- IDE support (autocomplete, refactoring)
- Unified error handling

### 5. Testable

```go
// Can unit test the prerequisite checker
func TestPrerequisiteChecker_CheckDockerRunning(t *testing.T) {
    // Test logic
}
```

vs bash:
```bash
# Bash scripts are hard to unit test
```

## Common Workflows

### Full Test Run

```bash
# From project root
make test-migration
```

### Quick Iteration

```bash
# Run without verbose output
make test-migration-quick

# Run single phase
make test-migration-phase PHASE=Phase7_WorkerProcessing
```

### Clean Slate

```bash
# Stop services, clean database, restart
make test-services-down
make test-services-up
make db-migrate-test
make test-migration
```

### Debug Service Issues

```bash
# View live logs
make test-services-logs

# Or specific service
cd testing && docker compose logs -f postgres
cd testing && docker compose logs -f minio
```

## Integration with CI/CD

### GitHub Actions Example

```yaml
name: Migration Tests

on: [push, pull_request]

jobs:
  migration-e2e:
    runs-on: ubuntu-latest

    steps:
      - uses: actions/checkout@v3

      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.21'

      - name: Start services
        run: make test-services-up

      - name: Run migrations
        run: make db-migrate-test

      - name: Run migration tests
        run: make test-migration
```

### GitLab CI Example

```yaml
migration-tests:
  stage: test
  services:
    - postgres:15
    - minio/minio:latest
  script:
    - make test-services-up
    - make db-migrate-test
    - make test-migration
```

## Prerequisites

### Required Tools

- Go 1.21+
- Docker
- docker-compose
- make
- netcat (for port checking)

### Required Services

Started automatically with `make test-services-up`:
- PostgreSQL (port 5433)
- MinIO (port 9000)
- Redpanda (port 9092) - optional for full tests

### Database Migrations

Applied with `make db-migrate-test`:
- Requires migration `000011_add_s3_migration_tables`

## Troubleshooting

### Tests Won't Start

```bash
# Check Docker is running
docker info

# Check services are up
docker compose ps

# View service logs
make test-services-logs
```

### Tests Fail Immediately

```bash
# The Go tests will tell you exactly what's wrong!
# Example outputs:

"❌ Docker is not running or not accessible"
→ Start Docker Desktop

"❌ Postgres container is not running"
→ make test-services-up

"❌ Required table 'migration_jobs' does not exist"
→ make db-migrate-test
```

### Slow Test Execution

```bash
# Use quick mode (no verbose output)
make test-migration-quick

# Run only specific phases
make test-migration-phase PHASE=Phase7_WorkerProcessing
```

## Migration from Bash Scripts

### Old Approach (Deprecated)

```bash
#!/bin/bash
./testing/test-migration-e2e.sh
```

**Problems:**
- Shell script complexity
- Hard to debug
- Platform differences (macOS vs Linux)
- No type safety
- Separate from Go ecosystem

### New Approach (Current)

```bash
make test-migration
```

**Benefits:**
- Pure Go implementation
- Type-safe
- Better error messages
- Platform independent
- Integrated with test framework
- Single command

## Performance

- **Setup (Phase 0):** ~0.5s (prerequisite checks)
- **Test execution (Phase 1-10):** ~12-15s
- **Total:** ~12-15s

Same performance as bash scripts, but with:
- ✅ Better error messages
- ✅ Type safety
- ✅ Easier maintenance
- ✅ Single language

## Summary

### Simple Commands

```bash
make test-services-up    # Start services
make db-migrate-test     # Run migrations
make test-migration      # Run tests
```

### Key Benefits

- ✅ **No bash scripts** - Pure Go implementation
- ✅ **Automatic checks** - Prerequisites verified before tests
- ✅ **Clear errors** - Actionable error messages
- ✅ **Type safe** - Compile-time checking
- ✅ **Fast** - Same performance as bash
- ✅ **Maintainable** - Single language, testable code

### Result

Production-ready e2e integration tests with:
- 27+ strong signal validations
- Automatic prerequisite checking
- Clear error messages
- Simple make commands
- No bash script complexity

**Just run:** `make test-migration`
