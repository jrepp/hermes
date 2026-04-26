# Comprehensive Central-Edge E2E Test Design

**Purpose**: Design a comprehensive end-to-end test that validates the full Hermes central-edge architecture including document creation, indexing, synchronization, and notifications.

**Status**: Design Complete
**Created**: 2025-11-14

---

## Overview

This test validates the complete Hermes system from document creation on an edge instance through to notification delivery, ensuring all components work together correctly.

### Architecture Under Test

```
┌─────────────────────────────────────────────────────────────────┐
│ Edge Hermes Instance                                             │
│ ┌─────────────────────────────────────────────────────────────┐ │
│ │ 1. Create Document (via API)                                 │ │
│ │    - RFC document in local workspace                         │ │
│ │    - File written to edge workspace volume                   │ │
│ └──────────────────────┬──────────────────────────────────────┘ │
│                        │                                          │
│                        ▼                                          │
│ ┌─────────────────────────────────────────────────────────────┐ │
│ │ 2. Edge Indexer                                              │ │
│ │    - Detects new document                                    │ │
│ │    - Indexes to edge Meilisearch                            │ │
│ │    - Registers with central registry (RFC-085)              │ │
│ └──────────────────────┬──────────────────────────────────────┘ │
└────────────────────────┼───────────────────────────────────────┘
                         │
                         ▼ (Edge-to-Central Sync API with Bearer Auth)
┌─────────────────────────────────────────────────────────────────┐
│ Central Hermes Instance                                          │
│ ┌─────────────────────────────────────────────────────────────┐ │
│ │ 3. Edge Document Registry (RFC-085/086)                      │ │
│ │    - Receives document registration                          │ │
│ │    - Validates bearer token authentication                   │ │
│ │    - Stores in edge_document_registry table                  │ │
│ └──────────────────────┬──────────────────────────────────────┘ │
│                        │                                          │
│                        ▼                                          │
│ ┌─────────────────────────────────────────────────────────────┐ │
│ │ 4. Central Indexer                                           │ │
│ │    - Indexes central workspace documents                     │ │
│ │    - Updates central Meilisearch                            │ │
│ └──────────────────────┬──────────────────────────────────────┘ │
│                        │                                          │
│                        ▼                                          │
│ ┌─────────────────────────────────────────────────────────────┐ │
│ │ 5. Workflow Action (via API)                                 │ │
│ │    - Approve document                                        │ │
│ │    - Triggers notification                                   │ │
│ └──────────────────────┬──────────────────────────────────────┘ │
└────────────────────────┼───────────────────────────────────────┘
                         │
                         ▼ (Publish to Redpanda)
┌─────────────────────────────────────────────────────────────────┐
│ Notification System (RFC-087)                                    │
│ ┌─────────────────────────────────────────────────────────────┐ │
│ │ 6. Redpanda Message Broker                                   │ │
│ │    - Receives notification message                           │ │
│ │    - Routes to consumer group                                │ │
│ └──────────────────────┬──────────────────────────────────────┘ │
│                        │                                          │
│          ┌─────────────┼─────────────┬──────────────┐            │
│          ▼             ▼             ▼              ▼            │
│    ┌─────────┐   ┌─────────┐   ┌─────────┐   ┌─────────┐       │
│    │ Audit   │   │ Mail    │   │ Ntfy    │   │ Slack   │       │
│    │ Backend │   │ Backend │   │ Backend │   │ Backend │       │
│    └────┬────┘   └────┬────┘   └────┬────┘   └────┬────┘       │
│         │             │             │              │            │
│         ▼             ▼             ▼              ▼            │
│    ┌─────────┐   ┌─────────┐   ┌─────────┐   ┌─────────┐       │
│    │ Logs    │   │ Mailhog │   │ Ntfy.sh │   │ Slack   │       │
│    │         │   │ (SMTP)  │   │         │   │ API     │       │
│    └─────────┘   └─────────┘   └─────────┘   └─────────┘       │
└─────────────────────────────────────────────────────────────────┘
```

---

## Test Phases

### Phase 1: Prerequisites & Service Health

**Purpose**: Verify all services are running and healthy

**Tests**:
1. ✓ Docker Compose services are running
2. ✓ PostgreSQL is accessible and healthy
3. ✓ Meilisearch is accessible and healthy
4. ✓ Redpanda is accessible and healthy
5. ✓ Central Hermes API is responding
6. ✓ Edge Hermes API is responding
7. ✓ Dex authentication is accessible
8. ✓ Mailhog is accessible
9. ✓ Central indexer is running
10. ✓ Notifier services (audit, mail, ntfy) are running

**Validation**:
- HTTP health checks return 200 OK
- Database connections succeed
- Service containers are in "Up" state

---

### Phase 2: Authentication & Token Management

**Purpose**: Validate RFC-085/086 bearer token authentication

**Tests**:
1. ✓ Create edge sync token for edge instance
2. ✓ Token is stored in service_tokens table
3. ✓ Token hash is correctly computed (SHA-256)
4. ✓ Edge can authenticate to central using token
5. ✓ Central rejects invalid tokens
6. ✓ Central rejects unauthenticated requests

**Validation**:
- Token generation succeeds
- Token authentication returns 200 OK
- Invalid token returns 401 Unauthorized
- Token exists in database with correct hash

**Files Used**:
- `testing/create-edge-token.sh`
- Database: `service_tokens` table

---

### Phase 3: Edge Document Creation & Local Indexing

**Purpose**: Create a document on the edge and verify local indexing

**Tests**:
1. ✓ Create test RFC document via Edge API
2. ✓ Document file is written to edge workspace
3. ✓ Edge indexer detects the new document
4. ✓ Document is indexed in edge Meilisearch
5. ✓ Document is searchable from edge
6. ✓ Document metadata is correct

**Test Document**:
```yaml
---
hermes-uuid: test-e2e-{timestamp}
document-type: RFC
document-number: RFC-999
status: WIP
title: "E2E Test Document"
owners:
  - test-user@example.com
product: Hermes
tags:
  - test
  - e2e
---

# RFC-999: E2E Test Document

This is an end-to-end test document created at {timestamp}.

## Purpose
Validate the complete Hermes central-edge architecture.

## Test Steps
1. Create document on edge
2. Verify edge indexing
3. Sync to central
4. Verify central indexing
5. Trigger notification
6. Verify delivery
```

**API Endpoints**:
- `POST http://localhost:8002/api/v2/documents` (Edge)
- `GET http://localhost:8002/api/v2/documents/{uuid}` (Edge)
- `GET http://localhost:7701/indexes/documents/search` (Edge Meilisearch)

**Validation**:
- Document UUID returned from create API
- Document file exists at expected path
- Search returns the document
- Document metadata matches input

---

### Phase 4: Edge-to-Central Synchronization

**Purpose**: Validate RFC-085 edge-to-central document synchronization

**Tests**:
1. ✓ Edge registers document with central
2. ✓ Bearer token authentication succeeds
3. ✓ Document appears in edge_document_registry table
4. ✓ Document metadata is correctly synced
5. ✓ Central can query edge document status
6. ✓ Sync status is updated

**API Endpoints**:
- `POST http://localhost:8000/api/v2/edge/documents/register` (Central)
- `GET http://localhost:8000/api/v2/edge/documents/sync-status?edge_instance=edge-dev-1` (Central)
- `GET http://localhost:8000/api/v2/edge/stats` (Central)

**Database Validation**:
```sql
-- Verify edge document registration
SELECT
    uuid,
    title,
    edge_instance,
    synced_at,
    last_sync_status
FROM edge_document_registry
WHERE uuid = '{test-document-uuid}';
```

**Expected Results**:
- Document exists in edge_document_registry
- edge_instance = 'edge-dev-1'
- last_sync_status = 'success'
- synced_at is recent timestamp

---

### Phase 5: Central Document Creation & Indexing

**Purpose**: Create a document on central and verify indexing

**Tests**:
1. ✓ Create test document via Central API
2. ✓ Document file is written to central workspace
3. ✓ Central indexer detects the new document
4. ✓ Document is indexed in central Meilisearch
5. ✓ Document is searchable from central
6. ✓ Document permissions are applied

**API Endpoints**:
- `POST http://localhost:8000/api/v2/documents` (Central)
- `GET http://localhost:8000/api/v2/documents/{uuid}` (Central)
- `GET http://localhost:7701/indexes/documents/search` (Central Meilisearch)

**Validation**:
- Document indexed in Meilisearch
- Search returns document with correct fields
- Document accessible via API

---

### Phase 6: Workflow Actions & Notifications

**Purpose**: Trigger document approval workflow and validate notification delivery

**Tests**:
1. ✓ Approve document via API
2. ✓ Notification is published to Redpanda
3. ✓ Notification message is well-formed
4. ✓ Template is resolved server-side
5. ✓ Notifiers consume the message
6. ✓ Audit backend logs the notification
7. ✓ Mail backend sends email to Mailhog
8. ✓ Ntfy backend sends push notification

**API Endpoints**:
- `POST http://localhost:8000/api/v2/documents/{uuid}/approve` (Central)
- `GET http://localhost:8025/api/v2/messages` (Mailhog API)

**Notification Validation**:

**Audit Logs** (Docker logs):
```bash
docker logs hermes-notifier-audit --tail 100 2>&1
```

Expected patterns:
- Notification ID appears
- Subject: "RFC-999 approved by {approver}"
- Body contains document title
- Body contains approver name
- Body contains document URL
- "✓ Acknowledged" confirmation

**Mailhog** (HTTP API):
```bash
curl -s http://localhost:8025/api/v2/messages
```

Expected:
- Email message exists
- To: test recipient
- Subject matches notification
- HTML body contains document link
- Plain text body contains document info

**Redpanda Topics**:
```bash
docker exec hermes-redpanda rpk topic list
docker exec hermes-redpanda rpk topic describe hermes.notifications
docker exec hermes-redpanda rpk group describe hermes-notifiers
```

Expected:
- Topic `hermes.notifications` exists
- Consumer group `hermes-notifiers` is Stable
- No consumer lag
- Message count increases

---

### Phase 7: Search Integration

**Purpose**: Validate search works across edge and central

**Tests**:
1. ✓ Search for test document on edge returns results
2. ✓ Search for test document on central returns results
3. ✓ Search filters work (status, type, owner)
4. ✓ Edge documents are queryable from central registry
5. ✓ Search relevance is correct

**Search Queries**:

**Edge Search**:
```bash
curl -s "http://localhost:7701/indexes/documents/search" \
  -H "Authorization: Bearer masterKey123" \
  -X POST \
  -d '{"q": "E2E Test Document", "limit": 10}'
```

**Central Search**:
```bash
curl -s "http://localhost:7701/indexes/documents/search" \
  -H "Authorization: Bearer masterKey123" \
  -X POST \
  -d '{"q": "E2E Test Document", "limit": 10}'
```

**Validation**:
- Results contain test document
- Ranking is reasonable
- Metadata is complete

---

### Phase 8: End-to-End Validation

**Purpose**: Comprehensive validation of the complete flow

**Tests**:
1. ✓ Document created on edge is findable via search
2. ✓ Edge document is registered with central
3. ✓ Central document workflow triggers notification
4. ✓ Notification reaches all backends
5. ✓ Email is delivered to Mailhog
6. ✓ Audit logs contain complete trace
7. ✓ No errors in any service logs
8. ✓ Performance metrics are acceptable

**Success Criteria**:
- All services remain healthy throughout test
- No errors in logs
- End-to-end latency < 30 seconds
- All expected outputs present

**Performance Benchmarks**:
- Document creation: < 1s
- Indexing latency: < 5s
- Sync to central: < 3s
- Notification delivery: < 10s
- Total end-to-end: < 30s

---

## Test Implementation

### Shell Script: `test-comprehensive-e2e.sh`

**Location**: `testing/test-comprehensive-e2e.sh`

**Features**:
- Colored output for readability
- Progress tracking with counters
- Detailed error messages
- JSON validation
- Database query validation
- Log analysis
- Cleanup on failure
- Summary report

**Usage**:
```bash
cd testing

# Start services
docker compose up -d

# Run test
./test-comprehensive-e2e.sh

# With verbose output
./test-comprehensive-e2e.sh --verbose

# Skip cleanup on failure
./test-comprehensive-e2e.sh --no-cleanup

# Test specific phase
./test-comprehensive-e2e.sh --phase=6
```

**Exit Codes**:
- 0: All tests passed
- 1: Service health check failed
- 2: Authentication test failed
- 3: Document creation failed
- 4: Synchronization failed
- 5: Indexing failed
- 6: Notification failed
- 7: Search failed
- 99: Unexpected error

---

### Go Integration Test: `tests/integration/e2e/comprehensive_test.go`

**Location**: `tests/integration/e2e/comprehensive_test.go`

**Features**:
- Uses testify for assertions
- Parallel test execution
- Comprehensive error messages
- Automatic cleanup with defer
- Table-driven subtests
- Integration with existing fixture

**Tests**:
```go
func TestComprehensiveE2E(t *testing.T) {
    t.Run("Prerequisites", testPrerequisites)
    t.Run("Authentication", testAuthentication)
    t.Run("EdgeDocumentCreation", testEdgeDocumentCreation)
    t.Run("EdgeToCenter alSync", testEdgeToCentralSync)
    t.Run("CentralDocumentCreation", testCentralDocumentCreation)
    t.Run("WorkflowAndNotifications", testWorkflowAndNotifications)
    t.Run("SearchIntegration", testSearchIntegration)
    t.Run("EndToEndValidation", testEndToEndValidation)
}
```

**Usage**:
```bash
# Run from project root
go test -tags=integration -v ./tests/integration/e2e/...

# Run specific test
go test -tags=integration -v ./tests/integration/e2e/... -run TestComprehensiveE2E/EdgeDocumentCreation

# Run with coverage
go test -tags=integration -coverprofile=coverage.out ./tests/integration/e2e/...
```

---

## Test Data

### Test Users
- **test-user@example.com**: Document creator
- **approver@example.com**: Document approver
- **reviewer@example.com**: Document reviewer

### Test Documents
- **RFC-999**: Created on edge
- **RFC-998**: Created on central
- **PRD-100**: Product requirement doc (multi-owner)

### Test Configuration

**Edge Instance**:
- ID: `edge-dev-1`
- URL: `http://localhost:8002`
- Workspace: `/app/workspaces/edge`

**Central Instance**:
- URL: `http://localhost:8000`
- Workspace: `/app/workspaces/central`
- Frontend: `http://localhost:4200`

---

## Validation Queries

### Database Queries

**Check edge document registration**:
```sql
SELECT
    uuid,
    title,
    document_type,
    status,
    edge_instance,
    synced_at,
    last_sync_status,
    created_at
FROM edge_document_registry
WHERE edge_instance = 'edge-dev-1'
ORDER BY created_at DESC
LIMIT 10;
```

**Check service tokens**:
```sql
SELECT
    id,
    token_type,
    expires_at,
    revoked,
    created_at
FROM service_tokens
WHERE token_type = 'edge'
AND revoked = false
ORDER BY created_at DESC;
```

**Check indexer activity**:
```sql
-- Note: This assumes indexer tracking table exists
-- May need to be added as part of indexer improvements
SELECT
    document_uuid,
    indexed_at,
    index_name,
    status
FROM indexer_activity
WHERE indexed_at > NOW() - INTERVAL '1 hour'
ORDER BY indexed_at DESC;
```

### Docker Commands

**Check service health**:
```bash
docker compose ps
docker compose logs hermes-central --tail 50
docker compose logs hermes-edge --tail 50
docker compose logs hermes-central-indexer --tail 50
docker compose logs hermes-notifier-audit --tail 50
```

**Check Redpanda**:
```bash
# Topic list
docker exec hermes-redpanda rpk topic list

# Consume messages
docker exec hermes-redpanda rpk topic consume hermes.notifications --num 10

# Consumer groups
docker exec hermes-redpanda rpk group list
docker exec hermes-redpanda rpk group describe hermes-notifiers
```

**Check Mailhog**:
```bash
# List emails via API
curl -s http://localhost:8025/api/v2/messages | jq .

# Count emails
curl -s http://localhost:8025/api/v2/messages | jq '.total'

# Search emails
curl -s http://localhost:8025/api/v2/search?query=RFC-999 | jq .
```

---

## Expected Outputs

### Successful Test Run Output

```
=================================================================
Hermes Comprehensive E2E Test
=================================================================
Testing: Central + Edge + Indexer + Notifications
Timestamp: 2025-11-14T10:30:00Z

=================================================================
Phase 1: Prerequisites & Service Health
=================================================================
✓ Docker Compose services running (12/12)
✓ PostgreSQL accessible (5432)
✓ Meilisearch healthy (http://localhost:7701/health)
✓ Redpanda healthy (19092)
✓ Central Hermes API responding (http://localhost:8000/health)
✓ Edge Hermes API responding (http://localhost:8002/health)
✓ Dex authentication accessible (http://localhost:5558)
✓ Mailhog accessible (http://localhost:8025)
✓ Central indexer running
✓ Notifier services running (audit, mail, ntfy)

=================================================================
Phase 2: Authentication & Token Management
=================================================================
✓ Created edge sync token: hermes-edge-token-a1b2c3d4...
✓ Token stored in service_tokens table
✓ Token hash computed correctly (SHA-256)
✓ Edge authenticated to central (HTTP 200)
✓ Invalid token rejected (HTTP 401)
✓ Unauthenticated request rejected (HTTP 401)

=================================================================
Phase 3: Edge Document Creation & Local Indexing
=================================================================
✓ Created document: test-e2e-1731582600 (RFC-999)
✓ Document file written: /app/workspaces/edge/docs/RFC-999.md
✓ Edge indexer detected document (5.2s)
✓ Document indexed in edge Meilisearch
✓ Document searchable from edge (3 results)
✓ Document metadata correct

=================================================================
Phase 4: Edge-to-Central Synchronization
=================================================================
✓ Edge registered document with central
✓ Bearer token authentication succeeded
✓ Document in edge_document_registry table
✓ Document metadata synced correctly
✓ Central can query edge document status
✓ Sync status: success (synced_at: 2025-11-14T10:30:10Z)

=================================================================
Phase 5: Central Document Creation & Indexing
=================================================================
✓ Created document: test-e2e-central-1731582600 (RFC-998)
✓ Document file written: /app/workspaces/central/docs/RFC-998.md
✓ Central indexer detected document (4.8s)
✓ Document indexed in central Meilisearch
✓ Document searchable from central
✓ Document permissions applied

=================================================================
Phase 6: Workflow Actions & Notifications
=================================================================
✓ Approved document RFC-999
✓ Notification published to Redpanda (ID: notif-1731582615)
✓ Notification message well-formed (JSON valid)
✓ Template resolved server-side
✓ Audit backend logged notification (12.3s)
✓ Mail backend sent email to Mailhog
✓ Ntfy backend sent push notification
✓ Email in Mailhog: RFC-999 approved by Approver
✓ Audit log contains: Subject, Body, DocumentURL

=================================================================
Phase 7: Search Integration
=================================================================
✓ Edge search returns test document (1 result)
✓ Central search returns test document (1 result)
✓ Filter by status works (2 results)
✓ Filter by type works (1 result)
✓ Edge documents queryable from central registry
✓ Search relevance correct

=================================================================
Phase 8: End-to-End Validation
=================================================================
✓ Edge document findable via search
✓ Edge document registered with central
✓ Central workflow triggered notification
✓ Notification reached all backends (audit, mail, ntfy)
✓ Email delivered to Mailhog
✓ Audit logs contain complete trace
✓ No errors in service logs
✓ Performance: End-to-end completed in 27.4s

=================================================================
Test Summary
=================================================================

Total Tests: 58
Passed: 58
Failed: 0

Performance Metrics:
- Document creation: 0.8s
- Edge indexing: 5.2s
- Sync to central: 2.3s
- Central indexing: 4.8s
- Notification delivery: 12.3s
- Total end-to-end: 27.4s

╔════════════════════════════════════════════════════════╗
║  ✓ All E2E tests passed successfully!                 ║
║                                                        ║
║  Hermes Central-Edge Architecture: VERIFIED ✓         ║
║  RFC-085 Edge Sync: VERIFIED ✓                        ║
║  RFC-086 Authentication: VERIFIED ✓                   ║
║  RFC-087 Notifications: VERIFIED ✓                    ║
╚════════════════════════════════════════════════════════╝
```

---

## Troubleshooting

### Common Issues

**1. Services not starting**
```bash
# Check logs
docker compose logs

# Restart services
docker compose restart

# Rebuild if needed
docker compose up -d --build --force-recreate
```

**2. Authentication failures**
```bash
# Regenerate token
cd testing
./create-edge-token.sh edge-dev-1

# Verify token in database
docker exec hermes-testing-postgres-1 psql -U postgres -d hermes_testing \
  -c "SELECT * FROM service_tokens WHERE token_type = 'edge';"
```

**3. Indexing delays**
```bash
# Check indexer logs
docker logs hermes-central-indexer --tail 100

# Verify Meilisearch
curl -H "Authorization: Bearer masterKey123" \
  http://localhost:7701/indexes/documents/stats | jq .
```

**4. Notification not delivered**
```bash
# Check Redpanda
docker exec hermes-redpanda rpk group describe hermes-notifiers

# Check notifier logs
docker logs hermes-notifier-audit --tail 100
docker logs hermes-notifier-mail --tail 100

# Check Mailhog
curl http://localhost:8025/api/v2/messages | jq '.total'
```

**5. Search not returning results**
```bash
# Reindex
curl -X POST http://localhost:8000/api/v2/admin/reindex

# Check Meilisearch
curl -H "Authorization: Bearer masterKey123" \
  http://localhost:7701/indexes | jq .
```

---

## CI/CD Integration

### GitHub Actions Workflow

```yaml
name: Comprehensive E2E Tests

on:
  push:
    branches: [main, develop]
  pull_request:
    branches: [main]
  schedule:
    - cron: '0 2 * * *'  # Daily at 2 AM

jobs:
  e2e-test:
    runs-on: ubuntu-latest
    timeout-minutes: 30

    steps:
      - uses: actions/checkout@v3

      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v2

      - name: Start services
        run: |
          cd testing
          docker compose up -d --build
        timeout-minutes: 10

      - name: Wait for services
        run: |
          cd testing
          ./wait-for-services.sh
        timeout-minutes: 5

      - name: Run comprehensive E2E test
        run: |
          cd testing
          ./test-comprehensive-e2e.sh --verbose
        timeout-minutes: 10

      - name: Collect logs on failure
        if: failure()
        run: |
          cd testing
          docker compose logs > e2e-logs.txt
          docker exec hermes-testing-postgres-1 \
            pg_dump -U postgres hermes_testing > db-dump.sql

      - name: Upload logs
        if: failure()
        uses: actions/upload-artifact@v3
        with:
          name: e2e-test-logs
          path: |
            testing/e2e-logs.txt
            testing/db-dump.sql

      - name: Cleanup
        if: always()
        run: |
          cd testing
          docker compose down -v
```

---

## Future Enhancements

### Short-term
1. Add WebSocket notification testing
2. Add multi-user concurrent access tests
3. Add permission boundary tests
4. Add document versioning tests
5. Add conflict resolution tests

### Medium-term
1. Add performance benchmarking
2. Add load testing (100+ documents)
3. Add chaos engineering (service failures)
4. Add network partition testing
5. Add data corruption recovery tests

### Long-term
1. Add multi-edge-instance tests
2. Add federation testing
3. Add disaster recovery tests
4. Add upgrade/migration tests
5. Add security penetration tests

---

## References

- **RFC-085**: Multi-Provider Architecture with Document Synchronization
- **RFC-086**: Authentication and Bearer Token Management
- **RFC-087**: Multi-Backend Notification System
- **Existing Tests**:
  - `testing/test-edge-sync-auth.sh`
  - `testing/test-notifications-e2e.sh`
  - `tests/integration/edgesync/`
  - `tests/integration/notifications/`
- **Docker Compose**: `testing/docker-compose.yml`

---

**Last Updated**: 2025-11-14
**Test Coverage**: 58 comprehensive test scenarios
**Status**: Design complete, ready for implementation
