# RFC-089 Testing Guide

## Overview

This guide explains how to test the RFC-089 S3 storage backend and migration system implementation.

## Current Status

✅ **Phase 1: Complete** - S3 adapter, migration system, database schema
✅ **Phase 2: Complete** - API endpoints integrated into server
⏳ **Phase 3: In Progress** - Testing and worker integration

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                   API Layer (Server)                     │
│  • /api/v2/providers - Provider management               │
│  • /api/v2/migrations - Migration job management         │
└────────────────┬────────────────────────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────────────────────────┐
│                Migration Manager (pkg/migration)         │
│  • Job lifecycle management                              │
│  • Document queuing                                      │
│  • Progress tracking                                     │
└────────────────┬────────────────────────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────────────────────────┐
│              Migration Outbox (Database)                 │
│  • Transactional outbox pattern                          │
│  • Idempotency keys                                      │
│  • Retry logic                                           │
└────────────────┬────────────────────────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────────────────────────┐
│               Migration Worker (Optional)                │
│  • Polls outbox for tasks                                │
│  • Executes migrations                                   │
│  • Validates content                                     │
│  • Updates progress                                      │
└─────────────────────────────────────────────────────────┘
```

## Test Infrastructure

### Prerequisites

```bash
# 1. Start test infrastructure
cd testing
docker compose up -d postgres minio

# 2. Verify services are healthy
docker compose ps

# Expected output:
# - postgres (healthy)
# - minio (healthy)
```

### Database Setup

```bash
# Apply migrations
make db-migrate

# Verify migration tables exist
docker compose exec postgres psql -U postgres -d hermes_test -c "\dt"

# Should show:
# - provider_storage
# - migration_jobs
# - migration_items
# - migration_outbox
```

## Testing

### 1. Unit Tests

**S3 Adapter Tests:**
```bash
INTEGRATION_TEST=1 go test -v ./pkg/workspace/adapters/s3
```

Expected: ✅ 10/10 tests passing

**Migration System Tests:**
```bash
INTEGRATION_TEST=1 go test -v ./pkg/migration
```

Expected: ✅ 7/7 tests passing

**Router Tests:**
```bash
go test -v ./pkg/workspace/router
```

Expected: ✅ 9/9 tests passing

### 2. API Integration Tests

**Start the Server:**
```bash
# Terminal 1
go run main.go server -config testing/config-rfc089-migration.hcl

# Server should start on http://localhost:8000
```

**Run API Tests:**
```bash
# Terminal 2
./testing/test-rfc089-api.sh

# Or with custom base URL:
API_BASE=http://localhost:8000 ./testing/test-rfc089-api.sh
```

**Manual API Testing:**

```bash
# Health check
curl http://localhost:8000/health

# List providers
curl http://localhost:8000/api/v2/providers | jq

# Register a provider
curl -X POST http://localhost:8000/api/v2/providers \
  -H "Content-Type: application/json" \
  -d '{
    "providerName": "s3-test",
    "providerType": "s3",
    "config": {
      "endpoint": "http://minio:9000",
      "region": "us-east-1",
      "bucket": "hermes-documents",
      "access_key": "minioadmin",
      "secret_key": "minioadmin"
    },
    "isPrimary": false,
    "isWritable": true,
    "status": "active"
  }'

# Get provider details
curl http://localhost:8000/api/v2/providers/1 | jq

# List migration jobs
curl http://localhost:8000/api/v2/migrations/jobs | jq

# Create migration job
curl -X POST http://localhost:8000/api/v2/migrations/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "jobName": "test-migration",
    "sourceProvider": "provider-1",
    "destProvider": "provider-2",
    "strategy": "copy",
    "concurrency": 5,
    "batchSize": 100,
    "dryRun": true,
    "validate": true
  }'

# Get job progress
curl http://localhost:8000/api/v2/migrations/jobs/1/progress | jq
```

### 3. End-to-End Migration Test

```bash
# Coming soon: Full E2E test with actual document migration
./testing/test-e2e-migration.sh
```

## API Reference

### Provider Management

#### List Providers
```
GET /api/v2/providers
GET /api/v2/providers?status=active
```

#### Register Provider
```
POST /api/v2/providers
Content-Type: application/json

{
  "providerName": "s3-archive",
  "providerType": "s3",
  "config": { ... },
  "capabilities": { ... },
  "isPrimary": false,
  "isWritable": true,
  "status": "active"
}
```

#### Get Provider
```
GET /api/v2/providers/:id
```

#### Update Provider
```
PATCH /api/v2/providers/:id
Content-Type: application/json

{
  "status": "readonly",
  "isWritable": false
}
```

#### Delete Provider
```
DELETE /api/v2/providers/:id
```

#### Get Provider Health
```
GET /api/v2/providers/:id/health
```

### Migration Management

#### List Jobs
```
GET /api/v2/migrations/jobs
GET /api/v2/migrations/jobs?status=running
GET /api/v2/migrations/jobs?limit=50
```

#### Create Job
```
POST /api/v2/migrations/jobs
Content-Type: application/json

{
  "jobName": "migrate-to-s3",
  "sourceProvider": "provider-1",
  "destProvider": "provider-2",
  "strategy": "copy|move|mirror",
  "documentUuids": ["uuid1", "uuid2"],  // Optional
  "filterCriteria": {},  // Optional
  "concurrency": 5,
  "batchSize": 100,
  "dryRun": false,
  "validate": true
}
```

#### Get Job
```
GET /api/v2/migrations/jobs/:id
```

#### Start Job
```
POST /api/v2/migrations/jobs/:id/start
```

#### Pause Job
```
POST /api/v2/migrations/jobs/:id/pause
```

#### Cancel Job
```
DELETE /api/v2/migrations/jobs/:id
```

#### Get Progress
```
GET /api/v2/migrations/jobs/:id/progress

Response:
{
  "total": 1000,
  "migrated": 850,
  "failed": 5,
  "skipped": 10,
  "pending": 135,
  "percent": 86.5,
  "rate": 12.3,  // docs/second
  "etaSeconds": 11
}
```

#### List Items
```
GET /api/v2/migrations/jobs/:id/items
GET /api/v2/migrations/jobs/:id/items?status=failed
GET /api/v2/migrations/jobs/:id/items?limit=100
```

## Known Issues & TODO

### Worker Integration
- ⏳ Migration worker not yet started in server process
- ⏳ Need to integrate Router with server startup
- ⏳ Worker requires provider map from Router

**Workaround:** For now, migrations can be triggered via API but won't execute automatically. Manual worker execution:

```go
// Example: Run worker manually in a test
worker := migration.NewWorker(db, providerMap, logger, &migration.WorkerConfig{
    PollInterval:   5 * time.Second,
    MaxConcurrency: 5,
})
ctx := context.Background()
go worker.Start(ctx)
```

### Authentication
- ⚠️  Current API endpoints don't enforce authentication
- TODO: Add authentication middleware for production
- Workaround: Use behind authenticated reverse proxy

### Provider Auto-Registration
- ⏳ Providers from config not automatically registered in database
- TODO: Add config → database sync on server startup
- Workaround: Register providers via API after server starts

## Next Steps

### Phase 3: Worker & Scheduler (Est: 2-3 days)
1. **Integrate migration worker into server startup**
   - Add worker goroutine similar to outbox relay
   - Connect router to worker
   - Add graceful shutdown

2. **Implement scheduling**
   - Cron-based job execution
   - Auto-migration rules
   - Age-based archival

### Phase 4: Admin UI (Est: 4-5 days)
1. Migration dashboard
2. Provider management UI
3. Job monitoring and control

### Phase 5: Monitoring (Est: 2-3 days)
1. Prometheus metrics
2. Structured logging
3. Alerting rules

## Performance Characteristics

**S3 Adapter:**
- Write: ~40-50 docs/sec (local MinIO)
- Read: ~100-150 docs/sec (local MinIO)
- Batch operations: 5-10x faster with concurrency

**Migration System:**
- Default: 5 concurrent workers
- Batch size: 100 documents
- Expected throughput: 200-500 docs/sec

## Troubleshooting

### Server won't start
```bash
# Check if database is accessible
docker compose exec postgres psql -U postgres -d hermes_test -c "SELECT 1"

# Check if MinIO is accessible
curl http://localhost:9000/minio/health/live
```

### API returns 404
```bash
# Verify server is running and listening
curl http://localhost:8000/health

# Check server logs for routing issues
```

### Migration job stays in "pending"
```bash
# Check outbox table
docker compose exec postgres psql -U postgres -d hermes_test \
  -c "SELECT * FROM migration_outbox WHERE status='pending'"

# Worker not running - see "Worker Integration" above
```

### Provider registration fails
```bash
# Check if migration 000011 is applied
docker compose exec postgres psql -U postgres -d hermes_test \
  -c "SELECT version FROM schema_migrations ORDER BY version DESC LIMIT 5"

# Should include: 000011
```

## Support

- **RFC Documentation:** `docs-internal/rfc/RFC-089-s3-storage-backend-and-migrations.md`
- **Implementation Summary:** `docs-internal/rfc/RFC-089-IMPLEMENTATION-SUMMARY.md`
- **Source Code:**
  - S3 Adapter: `pkg/workspace/adapters/s3/`
  - Migration System: `pkg/migration/`
  - API Handlers: `internal/api/v2/migrations.go`, `internal/api/v2/providers.go`
  - Multi-Provider Router: `pkg/workspace/router/`
