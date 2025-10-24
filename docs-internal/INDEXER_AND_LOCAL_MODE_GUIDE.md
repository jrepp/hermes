# Indexer Architecture and Local Mode Guide

**Date**: October 24, 2025  
**Status**: Complete  
**Related**: `docs-internal/rfc/LOCAL_DEVELOPER_MODE_WITH_CENTRAL_HERMES.md`

## Overview

This guide documents the stateless indexer architecture and local mode capabilities added to Hermes.

## Architecture

### Stateless Indexer Design

The indexer is a lightweight, stateless agent that:
1. Registers with a central Hermes instance
2. Sends periodic heartbeats
3. Scans local workspace directories
4. Submits document metadata to central Hermes
5. Can run multiple instances per central server

**Key Benefits**:
- No database required on indexer side
- Horizontal scaling (many indexers → one central server)
- Crash-tolerant (re-registers on restart)
- Easy deployment (single binary + config)

### Components

```
┌─────────────────────────────────────────────────────────┐
│                   Central Hermes Server                 │
│  ┌────────────┐  ┌──────────────┐  ┌─────────────────┐│
│  │ PostgreSQL │  │ Meilisearch  │  │ Indexer API     ││
│  │ Database   │  │ Search       │  │ (/api/v2/index*)││
│  └────────────┘  └──────────────┘  └─────────────────┘│
│         ▲                                      ▲        │
└─────────┼──────────────────────────────────────┼────────┘
          │                                      │
          │ (2) Heartbeats                      │ (3) Documents
          │                                      │
    ┌─────┴──────────────────────────────────────┴─────┐
    │            Hermes Indexer Agent                  │
    │  ┌────────────────┐  ┌─────────────────────────┐│
    │  │ Registration   │  │ Workspace Scanner       ││
    │  │ & Auth         │  │ (Local FS)              ││
    │  └────────────────┘  └─────────────────────────┘│
    └──────────────────────────────────────────────────┘
                            │
                            │ (1) Read Files
                            ▼
                  ┌──────────────────┐
                  │  Workspace Data  │
                  │  ├── docs/       │
                  │  └── drafts/     │
                  └──────────────────┘
```

### API Endpoints

#### 1. Registration
```
POST /api/v2/indexer/register
Authorization: Bearer <registration-token>

Request:
{
  "indexer_type": "local-workspace",
  "workspace_path": "/app/workspaces",
  "metadata": {}
}

Response:
{
  "indexer_id": "uuid",
  "api_token": "hermes-api-token-xxxxx"
}
```

#### 2. Heartbeat
```
POST /api/v2/indexer/heartbeat
Authorization: Bearer <api-token>

Request:
{
  "status": "healthy",
  "last_scan_at": "2025-10-24T22:00:00Z",
  "document_count": 42
}

Response:
{
  "acknowledged": true
}
```

#### 3. Document Submission (Stub)
```
POST /api/v2/indexer/documents
Authorization: Bearer <api-token>

Request:
{
  "documents": [
    {
      "path": "/workspaces/testing/docs/RFC-001.md",
      "title": "My RFC",
      "type": "RFC",
      "status": "published"
    }
  ]
}

Response:
{
  "processed": 1,
  "errors": []
}
```

## Database Migrations

### Migration 000005: Type Fixes

**Problem**: GORM model types didn't match migration SQL types, causing encoding errors.

**Fixes**:
1. `document_type_custom_fields.read_only`: INTEGER → BOOLEAN
2. `document_type_custom_fields.type`: TEXT → INTEGER

**Migration Strategy**:
- Drop default constraints
- Convert column types with USING clause
- Migrate existing data
- Re-apply defaults

### Migration 000004: Workspace Projects

**Problem**: Obsolete columns from legacy schema (`project_id`, `project_name`) conflicted with new model.

**Fixes**:
- Drop obsolete columns and indexes
- Add new columns (instance_uuid, global_project_id, name, etc.)
- Migrate data from legacy TEXT columns to JSONB

## Docker Integration

### Entrypoint Script

**Problem**: Docker named volumes mount with root ownership, preventing non-root user from writing.

**Solution**: `docker-entrypoint.sh`
- Runs as root initially
- Fixes ownership of `/app/shared` and `/app/workspace_data`
- Switches to `hermes` user (uid 1000)
- Executes the Hermes binary

**Key Implementation Details**:
- Handles both full paths (`/app/hermes server`) and commands (`server`)
- Properly escapes arguments for `su -c`
- Safe for repeated container restarts

### Health Checks

All services have health checks:
- **hermes**: `wget http://localhost:8000/health`
- **hermes-indexer**: Checks central Hermes health
- **postgres**: `pg_isready`
- **meilisearch**: `wget http://localhost:7700/health`
- **dex**: OpenID configuration endpoint
- **web**: `wget http://localhost:4200/`

## Local Mode

### Use Case

Developer wants to run Hermes locally for a specific project without setting up central infrastructure.

### Setup

1. **Initialize workspace**:
   ```bash
   cd ~/projects/my-project
   mkdir -p .hermes/workspace_data/{docs,drafts,templates}
   cp testing/local-hermes-example/config.hcl .hermes/
   ```

2. **Configure**:
   - Edit `.hermes/config.hcl`
   - Set database to SQLite
   - Point workspace to `.hermes/workspace_data`
   - Disable or configure search provider

3. **Run**:
   ```bash
   hermes server -config=.hermes/config.hcl
   ```

4. **Access**:
   - Open http://localhost:8000
   - Create and manage documents locally

### Optional: Sync to Central Hermes

Enable indexer in config:
```hcl
indexer {
  enabled        = true
  central_url    = "https://hermes.company.com"
  workspace_path = ".hermes/workspace_data"
}
```

Run indexer agent:
```bash
hermes indexer-agent -config=.hermes/config.hcl
```

Documents sync every 5 minutes to central server.

## Testing

### Phase 4 Integration Tests

Script: `testing/phase4-integration-test.sh`

Tests:
1. ✅ Server health
2. ✅ Indexer registration
3. ✅ Token file accessibility
4. ✅ Workspace projects loaded
5. ✅ Search provider health
6. ✅ Database connection
7. ✅ Migration version
8. ✅ Dex OIDC provider
9. ✅ Frontend serving

**Run**:
```bash
cd testing && ./phase4-integration-test.sh
```

### Manual Testing

**Test Indexer Registration**:
```bash
# Check server logs for registration
docker compose logs hermes | grep "indexer registered"

# Check indexer logs
docker compose logs hermes-indexer | grep "Registered as indexer"
```

**Test Heartbeat**:
```bash
# Wait 5+ minutes, then check
docker compose logs hermes | grep heartbeat
```

**Test Token Generation**:
```bash
# Token file should exist
docker compose exec hermes cat /app/shared/indexer-token.txt
```

**Test Workspace Projects**:
```bash
# Should show 2 projects loaded
docker compose logs hermes | grep "workspace projects"
```

## Troubleshooting

### Permission Denied on /app/shared

**Symptom**: `open /app/shared/indexer-token.txt: permission denied`

**Fix**: Entrypoint script should fix this automatically. If not:
```bash
# Check entrypoint is being used
docker inspect hermes:latest | grep Entrypoint

# Rebuild with no cache
docker compose build --no-cache hermes
```

### Encoding Errors

**Symptom**: `failed to encode false into binary format for int4`

**Fix**: Migration 000005 should fix this. Verify:
```bash
# Check migration version
docker compose exec postgres psql -U postgres -d hermes_testing -c "SELECT version FROM schema_migrations;"

# Should show version 5
```

### Indexer Not Registering

**Symptom**: Indexer logs show "Waiting for registration token" indefinitely

**Fix**:
1. Check server started successfully
2. Check token file was created: `docker compose exec hermes ls -la /app/shared/`
3. Check volume mount: `docker compose config | grep -A5 hermes-indexer`
4. Check server logs: `docker compose logs hermes | grep "generated indexer"`

### Migration Dirty State

**Symptom**: `Dirty database version 5. Fix and force version.`

**Fix**: Drop and recreate database:
```bash
docker compose down -v
docker compose up -d
```

## Next Steps

### Production Deployment

1. **Central Hermes**:
   - Deploy with PostgreSQL and Meilisearch
   - Configure Dex/Okta for authentication
   - Enable HTTPS with TLS certificates
   - Set up monitoring and logging

2. **Indexer Agents**:
   - Deploy on developer machines or CI/CD runners
   - Distribute registration tokens securely
   - Monitor heartbeats in central Hermes
   - Scale horizontally as needed

### Future Enhancements

1. **Document Submission API**: Implement full document CRUD via indexer API
2. **Conflict Resolution**: Handle concurrent edits from multiple indexers
3. **Selective Sync**: Only sync changed documents (checksums/timestamps)
4. **Batch Operations**: Bulk document submission for large workspaces
5. **Offline Mode**: Queue operations when central server unavailable
6. **Monitoring**: Metrics for indexer health, sync lag, error rates

## References

- RFC: `docs-internal/rfc/LOCAL_DEVELOPER_MODE_WITH_CENTRAL_HERMES.md`
- Progress: `docs-internal/todos/LOCAL_WORKFLOW_PROGRESS_SUMMARY.md`
- Phase 4 Complete: `docs-internal/todos/PHASE4_INTEGRATION_TEST_COMPLETE.md`
- Test Script: `testing/phase4-integration-test.sh`
- Local Example: `testing/local-hermes-example/`
