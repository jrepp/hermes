# End-to-End Integration Tests

Comprehensive integration tests for the Hermes central-edge architecture, validating the complete system from edge document creation through notification delivery.

## Overview

These tests validate:
- **Service Health**: All services running and accessible
- **RFC-086**: Bearer token authentication for edge-to-central communication
- **RFC-085**: Edge document registry and synchronization
- **Meilisearch**: Search functionality and indexing
- **RFC-087**: Multi-backend notification system (Redpanda, Mailhog, notifiers)
- **System Stability**: No critical errors, all services remain healthy

## Architecture

```
Edge Hermes ──Bearer Auth──> Central Hermes ──> PostgreSQL
     ↓                             ↓                (edge_document_registry,
  Indexer                       Indexer             service_tokens)
     ↓                             ↓
Meilisearch                  Meilisearch
 (edge docs)                 (central docs)
                                  ↓
                             Notifications ──> Redpanda ──> Notifiers
                                                              ↓
                                                          Mailhog/Audit/Ntfy
```

## Prerequisites

### 1. Start Docker Compose Services

All services must be running before tests:

```bash
cd testing
docker compose up -d
```

This starts:
- `hermes-central` - Central Hermes API
- `hermes-edge` - Edge Hermes API
- `hermes-central-indexer` - Central indexer
- `postgres` - PostgreSQL database
- `meilisearch` - Search engine
- `redpanda` - Message broker
- `mailhog` - Test SMTP server
- `hermes-notifier-*` - Notification backends (audit, mail, ntfy)
- `dex` - Authentication service
- `web` - Frontend UI

### 2. Verify Services

Quick check:

```bash
# Check all services are up
docker compose ps

# Check central API
curl http://localhost:8000/health

# Check edge API
curl http://localhost:8002/health
```

## Running Tests

### Quick Prerequisites Check

Before running the full suite, verify prerequisites:

```bash
go test -tags=integration -v ./tests/integration/e2e -run TestPrerequisites
```

This checks:
- ✓ Docker Compose services running (12+ services)
- ✓ Central API reachable
- ✓ All required containers present

### Run Full E2E Test

```bash
# From project root
go test -tags=integration -v ./tests/integration/e2e -run TestComprehensiveE2E

# With timeout (recommended for slower systems)
go test -tags=integration -timeout=10m -v ./tests/integration/e2e -run TestComprehensiveE2E
```

### Run Specific Test Phase

```bash
# Phase 1: Service health
go test -tags=integration -v ./tests/integration/e2e -run TestComprehensiveE2E/Phase1_Prerequisites

# Phase 2: Authentication
go test -tags=integration -v ./tests/integration/e2e -run TestComprehensiveE2E/Phase2_Authentication

# Phase 3: Edge sync
go test -tags=integration -v ./tests/integration/e2e -run TestComprehensiveE2E/Phase3_EdgeToCentralSync

# Phase 4: Search
go test -tags=integration -v ./tests/integration/e2e -run TestComprehensiveE2E/Phase4_SearchIntegration

# Phase 5: Notifications
go test -tags=integration -v ./tests/integration/e2e -run TestComprehensiveE2E/Phase5_NotificationSystem

# Phase 6: End-to-end validation
go test -tags=integration -v ./tests/integration/e2e -run TestComprehensiveE2E/Phase6_EndToEndValidation
```

## Test Output

### Successful Run

```
=== RUN   TestComprehensiveE2E
=== RUN   TestComprehensiveE2E/Phase1_Prerequisites
    comprehensive_e2e_test.go:123: === Phase 1: Service Health & Prerequisites ===
=== RUN   TestComprehensiveE2E/Phase1_Prerequisites/Central_Hermes_API
    comprehensive_e2e_test.go:161: ✓ Central Hermes API healthy
=== RUN   TestComprehensiveE2E/Phase1_Prerequisites/Edge_Hermes_API
    comprehensive_e2e_test.go:161: ✓ Edge Hermes API healthy
=== RUN   TestComprehensiveE2E/Phase1_Prerequisites/Meilisearch
    comprehensive_e2e_test.go:161: ✓ Meilisearch healthy
=== RUN   TestComprehensiveE2E/Phase1_Prerequisites/Mailhog
    comprehensive_e2e_test.go:161: ✓ Mailhog healthy
=== RUN   TestComprehensiveE2E/Phase1_Prerequisites/PostgreSQL
    comprehensive_e2e_test.go:177: ✓ PostgreSQL accessible via fixture
=== RUN   TestComprehensiveE2E/Phase1_Prerequisites/Redpanda
    comprehensive_e2e_test.go:194: ✓ Redpanda healthy
=== RUN   TestComprehensiveE2E/Phase1_Prerequisites/hermes-central-indexer
    comprehensive_e2e_test.go:221: ✓ Container hermes-central-indexer running
    comprehensive_e2e_test.go:225: ✅ All services healthy and operational

=== RUN   TestComprehensiveE2E/Phase2_Authentication
    comprehensive_e2e_test.go:232: === Phase 2: Bearer Token Authentication (RFC-086) ===
    comprehensive_e2e_test.go:240: Generated test token: hermes-edge-token-a1b2c3d4-5678...
=== RUN   TestComprehensiveE2E/Phase2_Authentication/StoreToken
    comprehensive_e2e_test.go:253: ✓ Token stored in service_tokens table
=== RUN   TestComprehensiveE2E/Phase2_Authentication/ValidTokenAccepted
    comprehensive_e2e_test.go:279: ✓ Valid bearer token accepted (HTTP 200)
=== RUN   TestComprehensiveE2E/Phase2_Authentication/InvalidTokenRejected
    comprehensive_e2e_test.go:299: ✓ Invalid token rejected (HTTP 401)
=== RUN   TestComprehensiveE2E/Phase2_Authentication/MissingAuthRejected
    comprehensive_e2e_test.go:315: ✓ Unauthenticated request rejected (HTTP 401)
    comprehensive_e2e_test.go:318: ✅ Bearer token authentication working correctly

[... continues through all phases ...]

    comprehensive_e2e_test.go:593: ╔════════════════════════════════════════════════════════╗
    comprehensive_e2e_test.go:594: ║  ✅ Comprehensive E2E Test Passed                     ║
    comprehensive_e2e_test.go:595: ║                                                        ║
    comprehensive_e2e_test.go:596: ║  Validated:                                            ║
    comprehensive_e2e_test.go:597: ║   ✓ Service Health & Connectivity                     ║
    comprehensive_e2e_test.go:598: ║   ✓ RFC-086 Bearer Token Authentication               ║
    comprehensive_e2e_test.go:599: ║   ✓ RFC-085 Edge-to-Central Synchronization           ║
    comprehensive_e2e_test.go:600: ║   ✓ Meilisearch Integration                           ║
    comprehensive_e2e_test.go:601: ║   ✓ RFC-087 Notification System                       ║
    comprehensive_e2e_test.go:602: ║   ✓ System Stability & Error-Free Operation           ║
    comprehensive_e2e_test.go:603: ╚════════════════════════════════════════════════════════╝
--- PASS: TestComprehensiveE2E (8.34s)
    --- PASS: TestComprehensiveE2E/Phase1_Prerequisites (2.15s)
    --- PASS: TestComprehensiveE2E/Phase2_Authentication (1.23s)
    --- PASS: TestComprehensiveE2E/Phase3_EdgeToCentralSync (0.87s)
    --- PASS: TestComprehensiveE2E/Phase4_SearchIntegration (0.45s)
    --- PASS: TestComprehensiveE2E/Phase5_NotificationSystem (1.12s)
    --- PASS: TestComprehensiveE2E/Phase6_EndToEndValidation (2.52s)
PASS
ok      github.com/hashicorp-forge/hermes/tests/integration/e2e    8.456s
```

### Error Example (Actionable)

```
=== RUN   TestComprehensiveE2E/Phase1_Prerequisites/Central_Hermes_API
    comprehensive_e2e_test.go:151: ❌ Central Hermes API unreachable: Get "http://localhost:8000/health": dial tcp [::1]:8000: connect: connection refused
       URL: http://localhost:8000/health
       Ensure docker-compose is running: cd testing && docker compose up -d
--- FAIL: TestComprehensiveE2E/Phase1_Prerequisites/Central_Hermes_API (0.01s)
```

The error message tells you:
- ❌ What failed
- 💡 Why it failed (connection refused)
- 🔧 How to fix it (run docker-compose up)

## Test Phases

### Phase 1: Service Health & Prerequisites

Validates all required services are running and healthy.

**Tests**:
- Central Hermes API (`/health`)
- Edge Hermes API (`/health`)
- Meilisearch (`/health`)
- Mailhog (HTTP API)
- PostgreSQL (via fixture)
- Redpanda (`rpk cluster health`)
- Critical containers (indexer, notifiers)

**Failure Actions**:
- Service unreachable → Check `docker compose ps`
- Container not running → `docker logs <container>`
- Database connection → Verify migrations applied

### Phase 2: Bearer Token Authentication (RFC-086)

Validates bearer token authentication for edge-to-central communication.

**Tests**:
- Token generation and SHA-256 hashing
- Token storage in `service_tokens` table
- Valid token accepted (HTTP 200)
- Invalid token rejected (HTTP 401)
- Missing auth header rejected (HTTP 401)

**Failure Actions**:
- Token storage fails → Check database schema: `\d service_tokens`
- Auth failures → Verify token hash: `SELECT * FROM service_tokens WHERE token_type = 'edge'`

### Phase 3: Edge-to-Central Synchronization (RFC-085)

Validates edge document registry and sync endpoints.

**Tests**:
- `/api/v2/edge/documents/sync-status` endpoint
- `/api/v2/edge/stats` endpoint
- `edge_document_registry` table accessibility

**Failure Actions**:
- Endpoint failures → Check central logs: `docker logs hermes-central --tail 50`
- Registry table errors → Verify migrations

### Phase 4: Search Integration

Validates Meilisearch functionality.

**Tests**:
- Basic search queries
- Filtered search (by document type, status)
- Index statistics

**Failure Actions**:
- Search failures → Check Meilisearch health: `curl http://localhost:7701/health`
- No results → Check indexer: `docker logs hermes-central-indexer`

### Phase 5: Notification System (RFC-087)

Validates Redpanda message broker and notification backends.

**Tests**:
- Notification topic exists
- Consumer group status
- Mailhog API accessibility
- Notifier backend activity

**Failure Actions**:
- Redpanda issues → `docker exec hermes-redpanda rpk cluster health`
- Consumer lag → `docker exec hermes-redpanda rpk group describe hermes-notifiers`
- Notifier issues → `docker logs hermes-notifier-audit`

### Phase 6: End-to-End Validation

Final system stability and health checks.

**Tests**:
- All services remain healthy
- No critical errors in logs
- Overall system status

## Test Configuration

Configuration constants (modify if needed):

```go
const (
    centralURL     = "http://localhost:8000"
    edgeURL        = "http://localhost:8002"
    meilisearchURL = "http://localhost:7701"
    mailhogURL     = "http://localhost:8025"

    meilisearchKey = "masterKey123"
    edgeInstance   = "edge-dev-1"

    serviceTimeout = 5 * time.Second
    indexTimeout   = 10 * time.Second
    notifyTimeout  = 15 * time.Second
)
```

## Troubleshooting

### Services Not Running

```bash
cd testing

# Check status
docker compose ps

# View logs
docker compose logs hermes-central
docker compose logs hermes-edge

# Restart
docker compose restart

# Full rebuild
docker compose up -d --build --force-recreate
```

### Authentication Failures

```bash
# Check service_tokens table
docker exec hermes-testing-postgres-1 psql -U postgres -d hermes_testing \
  -c "SELECT token_type, revoked, expires_at, created_at FROM service_tokens WHERE token_type = 'edge';"

# Check central logs for auth errors
docker logs hermes-central --tail 100 | grep -i auth
```

### Indexing Issues

```bash
# Check central indexer
docker logs hermes-central-indexer --tail 100

# Check Meilisearch
curl -H "Authorization: Bearer masterKey123" \
  http://localhost:7701/indexes/documents/stats | jq .

# Restart indexer
docker restart hermes-central-indexer
```

### Notification Issues

```bash
# Check Redpanda health
docker exec hermes-redpanda rpk cluster health

# Check topics
docker exec hermes-redpanda rpk topic list

# Check consumer group
docker exec hermes-redpanda rpk group describe hermes-notifiers

# Check notifier logs
docker logs hermes-notifier-audit --tail 100
docker logs hermes-notifier-mail --tail 100

# Check Mailhog emails
curl http://localhost:8025/api/v2/messages | jq '.total'
```

### Test Timeouts

If tests timeout, increase the timeout:

```bash
# 10 minute timeout
go test -tags=integration -timeout=10m -v ./tests/integration/e2e

# Or set timeout in test
# Edit comprehensive_e2e_test.go and increase timeout constants
```

### Database Issues

```bash
# Check PostgreSQL
docker exec hermes-testing-postgres-1 psql -U postgres -d hermes_testing -c "SELECT version();"

# Check tables
docker exec hermes-testing-postgres-1 psql -U postgres -d hermes_testing -c "\dt"

# Run migrations
cd cmd/hermes-migrate
go run . -database "postgres://postgres:postgres@localhost:5433/hermes_testing?sslmode=disable"
```

## CI/CD Integration

### GitHub Actions

```yaml
name: E2E Integration Tests

on: [push, pull_request]

jobs:
  e2e-tests:
    runs-on: ubuntu-latest
    timeout-minutes: 30

    steps:
      - uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.23'

      - name: Start Docker Compose services
        run: |
          cd testing
          docker compose up -d --build
        timeout-minutes: 10

      - name: Wait for services to be ready
        run: |
          timeout 120 bash -c 'until curl -f http://localhost:8000/health; do sleep 2; done'
          timeout 120 bash -c 'until curl -f http://localhost:8002/health; do sleep 2; done'
          timeout 120 bash -c 'until curl -f http://localhost:7701/health; do sleep 2; done'

      - name: Run prerequisites check
        run: |
          go test -tags=integration -v ./tests/integration/e2e -run TestPrerequisites

      - name: Run comprehensive E2E tests
        run: |
          go test -tags=integration -timeout=15m -v ./tests/integration/e2e -run TestComprehensiveE2E

      - name: Collect logs on failure
        if: failure()
        run: |
          cd testing
          docker compose logs > ../e2e-logs.txt
          docker exec hermes-testing-postgres-1 pg_dump -U postgres hermes_testing > ../db-dump.sql

      - name: Upload artifacts on failure
        if: failure()
        uses: actions/upload-artifact@v4
        with:
          name: e2e-test-artifacts
          path: |
            e2e-logs.txt
            db-dump.sql

      - name: Cleanup
        if: always()
        run: |
          cd testing
          docker compose down -v
```

## Extending Tests

### Adding a New Test Phase

```go
func TestComprehensiveE2E(t *testing.T) {
    // ... existing phases ...

    t.Run("Phase7_MyNewPhase", func(t *testing.T) {
        testMyNewFeature(t, ctx)
    })
}

func testMyNewFeature(t *testing.T, ctx context.Context) {
    t.Log("=== Phase 7: My New Feature ===")

    t.Run("FeatureTest1", func(t *testing.T) {
        // Test implementation
        // Use descriptive error messages:
        // t.Fatalf("❌ Feature failed: %v\n   Expected: X\n   Got: Y\n   Fix: Do Z", err)
    })

    t.Log("✅ My new feature operational")
}
```

### Adding Helper Functions

```go
// Helper functions should:
// 1. Use t.Helper() to show correct line numbers in errors
// 2. Provide actionable error messages
// 3. Include relevant context in logs

func myHelper(t *testing.T, ctx context.Context, param string) result {
    t.Helper()

    // Implementation...

    if err != nil {
        t.Fatalf("❌ Helper failed: %v\n   Parameter: %s\n   Check: some debugging command",
            err, param)
    }

    return result
}
```

## Related Tests

- `tests/integration/edgesync/` - Edge sync authentication tests
- `tests/integration/notifications/` - Notification system tests
- `testing/test-notifications-e2e.sh` - Shell-based notification tests (legacy)

## References

- **RFC-085**: Multi-Provider Architecture with Document Synchronization
- **RFC-086**: Authentication and Bearer Token Management
- **RFC-087**: Multi-Backend Notification System
- **Docker Compose**: `testing/docker-compose.yml`
- **Design Doc**: `testing/COMPREHENSIVE_E2E_TEST_DESIGN.md`

---

**Test Coverage**: 6 phases, 30+ test scenarios
**Status**: ✅ Production ready
**Last Updated**: 2025-11-14
