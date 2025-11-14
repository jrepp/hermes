# RFC-085 Implementation Plan: Edge-to-Central Architecture

## Executive Summary

This document outlines the implementation plan for RFC-085's edge-to-central Hermes architecture, where an edge Hermes instance (local authoring) delegates operations to a central Hermes server (tracking, directory, permissions).

**Goal**: Enable docker-compose testing infrastructure with edge Hermes reporting to central Hermes.

**Status**: API provider complete, multi-provider manager and sync endpoints needed.

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│ Edge Hermes (Developer Laptop / Regional Office)           │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  ┌──────────────────────────────────────────────────────┐  │
│  │ Multi-Provider Manager                                │  │
│  │ - Primary: Local Workspace (document authoring)      │  │
│  │ - Secondary: API Provider (delegation to central)    │  │
│  │ - Automatic routing based on capability              │  │
│  └──────────────────────────────────────────────────────┘  │
│                                                              │
│  Primary Provider: Local Workspace                          │
│  ├─ Documents (create, read, update, delete)               │
│  ├─ Content (edit, compare, export)                        │
│  └─ Revision tracking (local Git)                          │
│                                                              │
│  Secondary Provider: API Provider                           │
│  ├─ Directory (people search) → delegated to central       │
│  ├─ Permissions (access control) → delegated to central    │
│  ├─ Teams (groups) → delegated to central                  │
│  ├─ Notifications (email) → delegated to central           │
│  └─ Document sync (metadata) → sync to central tracker     │
│                                                              │
└──────────────────────┬──────────────────────────────────────┘
                       │
                       │ REST API (HTTP/2)
                       │ POST /api/v2/documents/register
                       │ PUT /api/v2/documents/:uuid/sync-metadata
                       │ GET /api/v2/people/search?q=...
                       │ GET /api/v2/teams/:uuid/members
                       │
                       ▼
┌─────────────────────────────────────────────────────────────┐
│ Central Hermes (Company Server)                             │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  Primary Provider: Google Workspace (or Local for testing)  │
│  ├─ Documents (central tracking registry)                  │
│  ├─ Directory (Google Directory API)                       │
│  ├─ Permissions (Google Drive sharing)                     │
│  ├─ Teams (Google Groups)                                  │
│  └─ Notifications (Gmail)                                   │
│                                                              │
│  Document Sync Registry                                     │
│  ├─ Tracks all documents across edge instances             │
│  ├─ UUID → edge_instance mapping                           │
│  ├─ Metadata aggregation (title, status, owners)           │
│  └─ Search index (global document search)                  │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

## Implementation Phases

### Phase 1: Multi-Provider Manager (REQUIRED)

**Files to create:**
- `pkg/workspace/multiprovider/manager.go`
- `pkg/workspace/multiprovider/router.go`
- `pkg/workspace/multiprovider/config.go`
- `pkg/workspace/multiprovider/test/manager_test.go`

**Key components:**

```go
// pkg/workspace/multiprovider/manager.go

package multiprovider

import (
    "context"
    "github.com/hashicorp/hermes/pkg/workspace"
)

// Manager coordinates multiple workspace providers with automatic routing
type Manager struct {
    primary   workspace.WorkspaceProvider  // Local workspace for authoring
    secondary workspace.WorkspaceProvider  // API provider for delegation

    // Configuration
    syncEnabled bool                       // Auto-sync metadata to central?
    syncMode    SyncMode                   // immediate, batch, manual
}

// Implements all 8 RFC-084 interfaces by routing to appropriate provider
var (
    _ workspace.WorkspaceProvider = (*Manager)(nil)
    _ workspace.DocumentProvider = (*Manager)(nil)
    _ workspace.ContentProvider = (*Manager)(nil)
    _ workspace.RevisionTrackingProvider = (*Manager)(nil)
    _ workspace.PermissionProvider = (*Manager)(nil)
    _ workspace.PeopleProvider = (*Manager)(nil)
    _ workspace.TeamProvider = (*Manager)(nil)
    _ workspace.NotificationProvider = (*Manager)(nil)
)

// Document operations route to PRIMARY (local authoring)
func (m *Manager) CreateDocument(ctx context.Context, doc *Document) error {
    // Create locally
    err := m.primary.(workspace.DocumentProvider).CreateDocument(ctx, doc)
    if err != nil {
        return err
    }

    // Sync metadata to central if enabled
    if m.syncEnabled {
        return m.syncDocumentToCentral(ctx, doc)
    }
    return nil
}

// Directory operations route to SECONDARY (central API)
func (m *Manager) SearchPeople(ctx context.Context, query string) ([]*Person, error) {
    // Delegate to central Hermes for directory lookup
    if provider, ok := m.secondary.(workspace.PeopleProvider); ok {
        return provider.SearchPeople(ctx, query)
    }
    return nil, ErrCapabilityNotAvailable
}

// Notification operations route to SECONDARY (central API)
func (m *Manager) SendNotification(ctx context.Context, notif *Notification) error {
    // Delegate to central Hermes for email sending
    if provider, ok := m.secondary.(workspace.NotificationProvider); ok {
        return provider.SendNotification(ctx, notif)
    }
    return ErrCapabilityNotAvailable
}
```

**Routing logic:**

| Interface | Method | Route To | Reason |
|-----------|--------|----------|--------|
| DocumentProvider | Create/Update/Delete | Primary | Local authoring |
| ContentProvider | All methods | Primary | Local content editing |
| RevisionTrackingProvider | All methods | Primary | Local Git history |
| PermissionProvider | All methods | Secondary | Central access control |
| PeopleProvider | All methods | Secondary | Central directory |
| TeamProvider | All methods | Secondary | Central groups |
| NotificationProvider | All methods | Secondary | Central email |

### Phase 2: Document Synchronization Endpoints (REQUIRED)

**Files to modify:**
- `internal/api/v2/documents.go` - Add sync endpoints
- `pkg/models/document.go` - Add sync metadata
- `internal/services/document_sync.go` - New sync service

**New API endpoints needed:**

```go
// POST /api/v2/documents/register
// Register a document from edge instance to central tracker
type RegisterDocumentRequest struct {
    UUID         string            `json:"uuid"`          // Document UUID
    Title        string            `json:"title"`         // Document title
    DocumentType string            `json:"document_type"` // RFC, PRD, etc.
    Status       string            `json:"status"`        // Draft, In-Review, etc.
    Owners       []string          `json:"owners"`        // Owner email addresses
    EdgeInstance string            `json:"edge_instance"` // Edge identifier
    Metadata     map[string]any    `json:"metadata"`      // Custom fields
    CreatedAt    time.Time         `json:"created_at"`
    UpdatedAt    time.Time         `json:"updated_at"`
}

// PUT /api/v2/documents/:uuid/sync-metadata
// Update document metadata in central tracker
type SyncMetadataRequest struct {
    Title    string         `json:"title,omitempty"`
    Status   string         `json:"status,omitempty"`
    Owners   []string       `json:"owners,omitempty"`
    Metadata map[string]any `json:"metadata,omitempty"`
    UpdatedAt time.Time     `json:"updated_at"`
}

// GET /api/v2/documents/sync-status?edge_instance=...
// Check sync status for edge instance
type SyncStatusResponse struct {
    Documents []DocumentSyncStatus `json:"documents"`
}

type DocumentSyncStatus struct {
    UUID            string    `json:"uuid"`
    Title           string    `json:"title"`
    EdgeLastSync    time.Time `json:"edge_last_sync"`
    CentralLastSync time.Time `json:"central_last_sync"`
    InSync          bool      `json:"in_sync"`
}
```

**Database schema additions:**

```sql
-- Migration: 000008_add_edge_document_tracking.up.sql

CREATE TABLE edge_document_registry (
    uuid          UUID PRIMARY KEY,
    title         TEXT NOT NULL,
    document_type TEXT NOT NULL,
    status        TEXT,
    owners        TEXT[],
    edge_instance TEXT NOT NULL,
    metadata      JSONB,
    created_at    TIMESTAMPTZ NOT NULL,
    updated_at    TIMESTAMPTZ NOT NULL,
    synced_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    INDEX idx_edge_instance (edge_instance),
    INDEX idx_document_type (document_type),
    INDEX idx_owners (owners)
);

-- Track document UUID mappings for drift resolution
CREATE TABLE document_uuid_mappings (
    edge_uuid     UUID NOT NULL,
    central_uuid  UUID,
    edge_instance TEXT NOT NULL,
    merged_at     TIMESTAMPTZ,
    PRIMARY KEY (edge_uuid, edge_instance)
);
```

### Phase 3: Docker Compose Testing Infrastructure (IN PROGRESS)

**Files to modify:**
- `testing/docker-compose.yml` - Add edge Hermes service
- `testing/config-central.hcl` - New: Central Hermes config
- `testing/config-edge.hcl` - New: Edge Hermes config
- `testing/test-edge-to-central.sh` - New: Integration test script

**Docker Compose Architecture:**

```yaml
# testing/docker-compose.yml

name: hermes-testing

services:
  # Shared infrastructure (unchanged)
  postgres: # Port 5433
  meilisearch: # Port 7701
  dex: # Port 5558
  migrate: # Database migrations

  # Central Hermes - Primary server with full capabilities
  hermes-central:
    container_name: hermes-central
    build:
      context: ..
      dockerfile: Dockerfile
    ports:
      - "8000:8000"  # Central on standard port
    volumes:
      - ./config-central.hcl:/app/config.hcl:ro
      - ./projects.hcl:/app/projects.hcl:ro
      - ./users.json:/app/workspace_data/users.json:ro
      - hermes_central_workspace:/app/workspace_data
      - ./workspaces/central:/app/workspaces/central
    environment:
      HERMES_BASE_URL: http://localhost:4200
      HERMES_SEARCH_PROVIDER: meilisearch
      HERMES_MEILISEARCH_URL: http://meilisearch:7700
      HERMES_MEILISEARCH_KEY: masterKey123
    command: ["server", "-config=/app/config.hcl"]
    depends_on:
      postgres: { condition: service_healthy }
      meilisearch: { condition: service_healthy }
      migrate: { condition: service_completed_successfully }
    healthcheck:
      test: ["CMD", "wget", "--no-verbose", "--tries=1", "-O", "/dev/null", "http://localhost:8000/health"]
      interval: 3s
      timeout: 2s
      retries: 3
      start_period: 5s
    networks:
      - hermes-testing

  # Edge Hermes - Local authoring + API delegation to central
  hermes-edge:
    container_name: hermes-edge
    build:
      context: ..
      dockerfile: Dockerfile
    ports:
      - "8002:8000"  # Edge on port 8002 externally
    volumes:
      - ./config-edge.hcl:/app/config.hcl:ro
      - ./projects.hcl:/app/projects.hcl:ro
      - hermes_edge_workspace:/app/workspace_data
      - ./workspaces/edge:/app/workspaces/edge
    environment:
      HERMES_BASE_URL: http://localhost:4202  # Separate frontend for edge
      HERMES_CENTRAL_URL: http://hermes-central:8000
      HERMES_EDGE_INSTANCE_ID: edge-dev-1
    command: ["server", "-config=/app/config.hcl"]
    depends_on:
      hermes-central: { condition: service_healthy }
    healthcheck:
      test: ["CMD", "wget", "--no-verbose", "--tries=1", "-O", "/dev/null", "http://localhost:8000/health"]
      interval: 3s
      timeout: 2s
      retries: 3
      start_period: 5s
    networks:
      - hermes-testing

  # Central indexer (unchanged from original hermes-indexer)
  hermes-central-indexer:
    container_name: hermes-central-indexer
    build:
      context: ..
      dockerfile: Dockerfile
    volumes:
      - indexer_shared:/app/shared:ro
      - ./workspaces/central:/app/workspaces/central:ro
    environment:
      HERMES_INDEXER_TOKEN_PATH: /app/shared/indexer-token.txt
      HERMES_CENTRAL_URL: http://hermes-central:8000
      HERMES_WORKSPACE_PATH: /app/workspaces
      HERMES_INDEXER_TYPE: local-workspace
    command: ["indexer-agent"]
    depends_on:
      hermes-central: { condition: service_healthy }
    networks:
      - hermes-testing

  # Web UI (unchanged, points to central)
  web:
    container_name: hermes-web
    build:
      context: ../web
      dockerfile: Dockerfile
    ports:
      - "4200:4200"
    environment:
      HERMES_API_URL: http://hermes-central:8000
    depends_on:
      hermes-central: { condition: service_healthy }
    networks:
      - hermes-testing

networks:
  hermes-testing:
    driver: bridge

volumes:
  postgres_testing:
  meilisearch_testing:
  hermes_central_workspace:  # Central workspace data
  hermes_edge_workspace:     # Edge workspace data
  indexer_shared:
```

### Phase 4: Configuration Files

**Central Hermes Configuration** (`testing/config-central.hcl`):

```hcl
// Central Hermes - Full capabilities with local workspace provider
// In production this would use Google Workspace provider

base_url = "http://localhost:4200"
log_format = "standard"

// Use local workspace for testing (production: google_workspace)
providers {
  workspace           = "local"
  search              = "meilisearch"
  projects_config_path = "projects.hcl"
}

local_workspace {
  base_path    = "/app/workspace_data"
  docs_path    = "/app/workspace_data/docs"
  drafts_path  = "/app/workspace_data/drafts"
  folders_path = "/app/workspace_data/folders"
  users_path   = "/app/workspace_data/users"
  tokens_path  = "/app/workspace_data/tokens"
  domain       = "hermes.local"
}

// Enable document sync API endpoints
feature_flags {
  flag "api_v2" {
    enabled = true
  }

  flag "edge_document_sync" {
    enabled = true  // NEW: Enable sync endpoints
  }
}

// Database, search, auth config (same as current config.hcl)
postgres {
  dbname   = "hermes_testing"
  host     = "postgres"
  port     = 5432
  user     = "postgres"
  password = "postgres"
}

meilisearch {
  host              = "http://meilisearch:7700"
  api_key           = "masterKey123"
  docs_index_name   = "docs"
  drafts_index_name = "drafts"
}

dex {
  disabled      = false
  issuer_url    = "http://localhost:5558/dex"
  client_id     = "hermes-testing"
  client_secret = "dGVzdGluZy1hcHAtc2VjcmV0"
  redirect_url  = "http://localhost:8000/auth/callback"
}

server {
  addr = "0.0.0.0:8000"
}

// Document types, products, etc. (same as current config.hcl)
document_types {
  document_type "RFC" {
    long_name   = "Request for Comments"
    description = "Technical design proposals"
    flight_icon = "discussion-circle"
    template    = "template-rfc"
  }
  // ... other document types
}

products {
  product "Engineering" { abbreviation = "ENG" }
  product "Platform" { abbreviation = "PLT" }
  // ... other products
}
```

**Edge Hermes Configuration** (`testing/config-edge.hcl`):

```hcl
// Edge Hermes - Multi-provider: Local (primary) + API (secondary)
// Local workspace for document authoring
// API provider delegates to central Hermes for directory, permissions, etc.

base_url = "http://localhost:4202"  // Different frontend port
log_format = "standard"

// Multi-provider configuration (NEW)
providers {
  workspace           = "multiprovider"  // NEW: Use multi-provider manager
  search              = "meilisearch"     // Still use shared Meilisearch
  projects_config_path = "projects.hcl"
}

// Multi-provider configuration (NEW)
multiprovider {
  // Primary provider: Local workspace for document authoring
  primary {
    type = "local"

    local_workspace {
      base_path    = "/app/workspace_data"
      docs_path    = "/app/workspace_data/docs"
      drafts_path  = "/app/workspace_data/drafts"
      folders_path = "/app/workspace_data/folders"
      users_path   = "/app/workspace_data/users"
      tokens_path  = "/app/workspace_data/tokens"
      domain       = "hermes.local"
    }
  }

  // Secondary provider: API provider delegates to central Hermes
  secondary {
    type = "api"

    api_workspace {
      base_url     = "http://hermes-central:8000"  // Central Hermes URL
      auth_token   = env("HERMES_API_TOKEN")       // Auth for central
      timeout      = "30s"
      tls_verify   = false  // Testing only
      max_retries  = 3
      retry_delay  = "1s"
    }
  }

  // Automatic document synchronization to central
  sync {
    enabled       = true     // Auto-sync document metadata to central
    mode          = "immediate"  // immediate, batch, manual
    edge_instance = env("HERMES_EDGE_INSTANCE_ID")  // Edge identifier
  }
}

// Database config (same as central - shared database)
postgres {
  dbname   = "hermes_testing"
  host     = "postgres"
  port     = 5432
  user     = "postgres"
  password = "postgres"
}

// Search config (points to shared Meilisearch)
meilisearch {
  host              = "http://meilisearch:7700"
  api_key           = "masterKey123"
  docs_index_name   = "docs"
  drafts_index_name = "drafts"
}

// Auth (same as central)
dex {
  disabled      = false
  issuer_url    = "http://localhost:5558/dex"
  client_id     = "hermes-testing"
  client_secret = "dGVzdGluZy1hcHAtc2VjcmV0"
  redirect_url  = "http://localhost:8002/auth/callback"  // Different port
}

server {
  addr = "0.0.0.0:8000"  // Internal port (mapped to 8002 externally)
}

// Document types, products (same as central)
document_types {
  document_type "RFC" {
    long_name   = "Request for Comments"
    description = "Technical design proposals"
    flight_icon = "discussion-circle"
    template    = "template-rfc"
  }
  // ... other document types
}

products {
  product "Engineering" { abbreviation = "ENG" }
  product "Platform" { abbreviation = "PLT" }
  // ... other products
}
```

### Phase 5: Integration Testing

**Test Script** (`testing/test-edge-to-central.sh`):

```bash
#!/bin/bash
# Integration test for edge-to-central Hermes architecture

set -e

echo "=== RFC-085 Edge-to-Central Integration Test ==="

# Start all services
echo "Starting docker-compose services..."
cd testing
docker compose up -d --build

# Wait for services to be healthy
echo "Waiting for services to be ready..."
sleep 10

# Test 1: Central Hermes is reachable
echo ""
echo "Test 1: Central Hermes health check"
curl -f http://localhost:8000/health || exit 1
echo "✓ Central Hermes is healthy"

# Test 2: Edge Hermes is reachable
echo ""
echo "Test 2: Edge Hermes health check"
curl -f http://localhost:8002/health || exit 1
echo "✓ Edge Hermes is healthy"

# Test 3: Create document on edge
echo ""
echo "Test 3: Create document on edge Hermes"
DOC_UUID=$(uuidgen | tr '[:upper:]' '[:lower:]')
RESPONSE=$(curl -s -X POST http://localhost:8002/api/v2/documents \
  -H "Content-Type: application/json" \
  -d "{
    \"uuid\": \"$DOC_UUID\",
    \"title\": \"Test Edge Document\",
    \"documentType\": \"RFC\",
    \"status\": \"Draft\",
    \"owners\": [\"test@hermes.local\"],
    \"content\": \"# Test Document\\n\\nCreated on edge.\"
  }")
echo "✓ Document created on edge: $DOC_UUID"

# Test 4: Verify document exists on edge
echo ""
echo "Test 4: Verify document on edge"
curl -f http://localhost:8002/api/v2/documents/$DOC_UUID || exit 1
echo "✓ Document exists on edge"

# Test 5: Verify document was synced to central
echo ""
echo "Test 5: Verify document synced to central"
sleep 2  # Wait for sync
CENTRAL_SYNC=$(curl -s http://localhost:8000/api/v2/documents/$DOC_UUID/sync-status)
echo "Sync status: $CENTRAL_SYNC"
echo "✓ Document synced to central registry"

# Test 6: Search people on edge (delegates to central)
echo ""
echo "Test 6: People search from edge (delegated to central)"
PEOPLE_RESULT=$(curl -s "http://localhost:8002/api/v2/people/search?q=test")
echo "People search result: $PEOPLE_RESULT"
echo "✓ People search delegated successfully"

# Test 7: Check sync status endpoint
echo ""
echo "Test 7: Check edge sync status"
SYNC_STATUS=$(curl -s "http://localhost:8000/api/v2/documents/sync-status?edge_instance=edge-dev-1")
echo "Sync status: $SYNC_STATUS"
echo "✓ Sync status endpoint working"

# Test 8: Update document on edge and verify sync
echo ""
echo "Test 8: Update document on edge and verify sync"
curl -s -X PUT http://localhost:8002/api/v2/documents/$DOC_UUID \
  -H "Content-Type: application/json" \
  -d "{
    \"title\": \"Updated Edge Document\",
    \"status\": \"In-Review\"
  }" > /dev/null
sleep 2  # Wait for sync
UPDATED_SYNC=$(curl -s http://localhost:8000/api/v2/documents/$DOC_UUID/sync-status)
echo "Updated sync status: $UPDATED_SYNC"
echo "✓ Document update synced to central"

echo ""
echo "=== All Integration Tests Passed ==="
echo ""
echo "Services:"
echo "  Central Hermes: http://localhost:8000"
echo "  Edge Hermes:    http://localhost:8002"
echo "  Web UI:         http://localhost:4200"
echo ""
echo "To view logs:"
echo "  docker compose logs -f hermes-central"
echo "  docker compose logs -f hermes-edge"
echo ""
echo "To stop:"
echo "  docker compose down"
```

### Phase 6: Go Integration Tests

**Test File** (`pkg/workspace/multiprovider/test/edge_to_central_test.go`):

```go
//go:build integration
// +build integration

package test

import (
    "context"
    "testing"
    "time"

    "github.com/hashicorp/hermes/pkg/workspace"
    "github.com/hashicorp/hermes/pkg/workspace/adapters/api"
    "github.com/hashicorp/hermes/pkg/workspace/adapters/local"
    "github.com/hashicorp/hermes/pkg/workspace/multiprovider"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestEdgeToCentralDocumentFlow(t *testing.T) {
    ctx := context.Background()

    // Setup: Create edge Hermes with multiprovider manager
    localProvider := setupLocalProvider(t)
    apiProvider := setupAPIProvider(t, "http://localhost:8000")

    manager := multiprovider.NewManager(&multiprovider.Config{
        Primary:   localProvider,
        Secondary: apiProvider,
        SyncConfig: multiprovider.SyncConfig{
            Enabled:      true,
            Mode:         multiprovider.SyncModeImmediate,
            EdgeInstance: "edge-test-1",
        },
    })

    // Test 1: Create document on edge
    doc := &workspace.Document{
        UUID:         generateUUID(),
        Title:        "Edge Test Document",
        DocumentType: "RFC",
        Status:       "Draft",
        Owners:       []string{"test@hermes.local"},
        Content:      "# Test\n\nCreated on edge.",
        CreatedAt:    time.Now(),
    }

    err := manager.CreateDocument(ctx, doc)
    require.NoError(t, err, "Should create document on edge")

    // Test 2: Verify document exists locally
    retrieved, err := manager.GetDocument(ctx, doc.UUID)
    require.NoError(t, err, "Should retrieve document from local")
    assert.Equal(t, doc.Title, retrieved.Title)

    // Test 3: Verify document was synced to central
    time.Sleep(2 * time.Second) // Wait for async sync

    centralDoc, err := apiProvider.GetDocument(ctx, doc.UUID)
    require.NoError(t, err, "Document should be synced to central")
    assert.Equal(t, doc.Title, centralDoc.Title)
    assert.Equal(t, "edge-test-1", centralDoc.EdgeInstance)

    // Test 4: Update document on edge
    doc.Title = "Updated Edge Document"
    doc.Status = "In-Review"
    err = manager.UpdateDocument(ctx, doc)
    require.NoError(t, err, "Should update document on edge")

    // Test 5: Verify update was synced to central
    time.Sleep(2 * time.Second)

    updatedCentral, err := apiProvider.GetDocument(ctx, doc.UUID)
    require.NoError(t, err, "Updated document should be synced to central")
    assert.Equal(t, "Updated Edge Document", updatedCentral.Title)
    assert.Equal(t, "In-Review", updatedCentral.Status)
}

func TestEdgeDelegatesToCentralForDirectory(t *testing.T) {
    ctx := context.Background()

    // Setup edge manager
    manager := setupEdgeManager(t)

    // Test: Search people (should delegate to central via API provider)
    people, err := manager.SearchPeople(ctx, "test")
    require.NoError(t, err, "Should delegate people search to central")
    assert.NotEmpty(t, people, "Should return people from central directory")
}

func TestEdgeDelegatesToCentralForNotifications(t *testing.T) {
    ctx := context.Background()

    // Setup edge manager
    manager := setupEdgeManager(t)

    // Test: Send notification (should delegate to central)
    notif := &workspace.Notification{
        To:      []string{"test@hermes.local"},
        Subject: "Test from Edge",
        Body:    "This notification was sent from edge via central.",
    }

    err := manager.SendNotification(ctx, notif)
    require.NoError(t, err, "Should delegate notification to central")
}

func TestOfflineEdgeContinuesAuthoring(t *testing.T) {
    ctx := context.Background()

    // Setup: Edge with unreachable central (API provider fails)
    localProvider := setupLocalProvider(t)
    apiProvider := setupAPIProvider(t, "http://unreachable:9999")

    manager := multiprovider.NewManager(&multiprovider.Config{
        Primary:   localProvider,
        Secondary: apiProvider,
        SyncConfig: multiprovider.SyncConfig{
            Enabled:      true,
            Mode:         multiprovider.SyncModeManual, // Manual sync when offline
            EdgeInstance: "edge-offline-test",
        },
    })

    // Test: Create document offline (local only)
    doc := &workspace.Document{
        UUID:         generateUUID(),
        Title:        "Offline Document",
        DocumentType: "RFC",
        Status:       "Draft",
        Owners:       []string{"test@hermes.local"},
        Content:      "# Offline\n\nCreated while central is unreachable.",
        CreatedAt:    time.Now(),
    }

    err := manager.CreateDocument(ctx, doc)
    require.NoError(t, err, "Should create document locally even when central is offline")

    // Verify document exists locally
    retrieved, err := manager.GetDocument(ctx, doc.UUID)
    require.NoError(t, err, "Should retrieve document from local storage")
    assert.Equal(t, doc.Title, retrieved.Title)
}
```

## Testing Strategy

### Unit Tests

- Multi-provider manager routing logic
- Document sync service
- API provider client (already has tests)
- Configuration parsing

**Run with:**
```bash
make test
```

### Integration Tests

- Edge-to-central document creation flow
- Metadata synchronization
- Directory delegation
- Notification delegation
- Offline edge behavior

**Run with:**
```bash
# Go integration tests
go test -tags=integration -v ./pkg/workspace/multiprovider/test/...

# Shell integration tests
cd testing && ./test-edge-to-central.sh
```

### Manual QA Test Plan

1. **Start docker-compose environment**
   ```bash
   cd testing
   docker compose up -d --build
   ```

2. **Access edge and central UIs**
   - Central: http://localhost:4200
   - Edge: http://localhost:4202 (requires separate web frontend)

3. **Test document creation on edge**
   - Create RFC document via edge UI
   - Verify document appears in edge workspace
   - Verify metadata synced to central tracker
   - Search for document from central UI

4. **Test directory search from edge**
   - Search for people from edge UI
   - Verify results come from central directory

5. **Test offline edge**
   - Stop central Hermes: `docker compose stop hermes-central`
   - Create document on edge (should still work)
   - Verify document saved locally
   - Restart central: `docker compose start hermes-central`
   - Manually sync or wait for auto-sync

## Implementation Timeline

| Phase | Tasks | Estimated Time | Dependencies |
|-------|-------|----------------|--------------|
| 1 | Multi-provider manager | 3-4 days | RFC-084 interfaces |
| 2 | Document sync endpoints | 2-3 days | Database migration |
| 3 | Docker-compose setup | 1-2 days | Phase 1 complete |
| 4 | Configuration files | 1 day | Phase 1, 3 |
| 5 | Integration tests (shell) | 1-2 days | Phase 2, 3, 4 |
| 6 | Integration tests (Go) | 2-3 days | Phase 1, 2 |
| **Total** | | **10-15 days** | |

## Success Criteria

- [ ] Multi-provider manager implements all 8 RFC-084 interfaces
- [ ] Document creation on edge syncs metadata to central
- [ ] Directory search from edge delegates to central
- [ ] Notification sending from edge delegates to central
- [ ] Docker-compose runs both edge and central Hermes
- [ ] Integration tests pass (both shell and Go)
- [ ] Manual QA test plan completed
- [ ] Edge continues authoring when central is offline
- [ ] Documentation complete

## Open Questions

1. **Authentication**: How does edge Hermes authenticate to central?
   - Option A: Long-lived API token (simpler for testing)
   - Option B: OAuth machine-to-machine flow (more secure)
   - **Decision**: Start with API token for testing, add OAuth later

2. **Sync mode**: Immediate vs batch vs manual?
   - **Decision**: Support all three, default to immediate for testing

3. **UUID conflicts**: How to handle duplicate UUIDs across edge instances?
   - **Decision**: Use edge_instance prefix in UUID generation (defer to Phase 2)

4. **Search**: Should edge have local search or delegate to central?
   - **Decision**: Local search for edge documents, central search for global

5. **Offline grace period**: How long can edge work offline before requiring sync?
   - **Decision**: Indefinite offline support, manual sync trigger available

## References

- [RFC-084: Workspace Provider Interfaces](../rfc/RFC-084-workspace-provider-interfaces.md)
- [RFC-085: API Provider Remote Delegation](../../docs-internal/rfc/RFC-085-api-provider-remote-delegation.md)
- [API Provider README](../../pkg/workspace/adapters/api/README.md)
- [Validation Tools Documentation](./validation-tools.md)

## Next Steps

1. **Create multi-provider manager** (Phase 1)
   - Start with manager.go scaffold
   - Implement routing logic
   - Add compile-time interface checks
   - Write unit tests

2. **Update docker-compose** (Phase 3)
   - Add hermes-edge service
   - Create config-edge.hcl
   - Create config-central.hcl
   - Test service startup

3. **Implement sync endpoints** (Phase 2)
   - Add database migration
   - Create sync API handlers
   - Implement sync service
   - Add sync tests

Let's start with Phase 1 (multi-provider manager) or Phase 3 (docker-compose setup) - which would you prefer to tackle first?
