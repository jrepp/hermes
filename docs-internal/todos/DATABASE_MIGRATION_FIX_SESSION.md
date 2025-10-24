# Database Migration Fix - Work Session Summary

**Date**: 2025-10-24  
**Branch**: `jrepp/dev-tidy`  
**Status**: 🔧 In Progress - Partial Fix Implemented

## 🎯 Objective
Fix local development workflow to get native backend + frontend running.

## 🔍 Root Cause Analysis

### Problem: GORM AutoMigrate Constraint Renaming Bug

When running `hermes server` against a fresh PostgreSQL database, the server crashes with:
```
error migrating database: ERROR: constraint "uni_indexer_folders_google_drive_id" 
of relation "indexer_folders" does not exist (SQLSTATE 42704)
```

**Why This Happens**:
1. Models use GORM tag `gorm:"uniqueIndex"` (e.g., `IndexerFolder.GoogleDriveID`)
2. On fresh database, GORM's AutoMigrate tries to **rename** a constraint that doesn't exist yet
3. This is a known GORM issue with uniqueIndex on fresh schemas

**Affected Models** (all have `uniqueIndex` GORM tags):
- `IndexerFolder` - `GoogleDriveID`
- `WorkspaceProject` - `ProjectUUID`
- `Document` - `DocumentUUID`
- `DocumentRelatedResource` - composite uniqueIndex
- `ProjectRelatedResource` - composite uniqueIndex
- `IndexerToken` - `TokenHash` (already excluded from AutoMigrate)
- `HermesInstance` - `InstanceUUID`, `InstanceID` (already excluded)

## ✅ Fixes Implemented

### 1. Disabled AutoMigrate (Temporary Workaround)
**File**: `internal/db/db.go` lines 99-107

```go
// TEMPORARY WORKAROUND: Disable AutoMigrate to avoid GORM constraint renaming bug
// See: docs-internal/todos/LOCAL_WORKFLOW_FIX_STATUS.md
/*
if err := db.AutoMigrate(
    models.ModelsToAutoMigrate()...,
); err != nil {
    return nil, fmt.Errorf("error migrating database: %w", err)
}
*/
```

**Rationale**: Bypasses GORM bug while we complete SQL migrations.

### 2. Added indexer_folders to Migration
**File**: `internal/db/migrations/000002_indexer_core.up.sql`

Added complete table definition:
```sql
CREATE TABLE IF NOT EXISTS indexer_folders (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP,
    google_drive_id TEXT NOT NULL UNIQUE,
    last_indexed_at TIMESTAMP
);
```

**File**: `pkg/models/gorm.go` line 27

Commented out from AutoMigrate list:
```go
// &IndexerFolder{}, // Commented out - causing GORM constraint rename bug
```

### 3. Created Missing document_types Columns Migration
**Files**: 
- `internal/db/migrations/000003_add_document_type_fields.up.sql` (NEW)
- `internal/db/migrations/000003_add_document_type_fields.down.sql` (NEW)

Added missing columns:
- `flight_icon TEXT`
- `more_info_link_text TEXT`
- `more_info_link_url TEXT`
- `checks JSONB`

### 4. Created Minimal Native Config
**File**: `testing/config-minimal-native.hcl` (NEW)

Minimal configuration for native development without Docker:
- PostgreSQL on port 5432 (native, not Docker)
- Meilisearch on port 7700 (native)
- Local workspace provider
- Dex auth disabled (needs fix)
- Google Workspace with placeholder values (required by config schema)

## ⚠️ Remaining Issues

### Issue 1: workspace_projects Table Incomplete
**Error**: `ERROR: column "name" does not exist (SQLSTATE 42703)`

The `workspace_projects` table in migrations is missing columns that exist in the `WorkspaceProject` model.

**Next Action**: 
1. Compare `pkg/models/workspace_project.go` against migration schema
2. Create `000004_fix_workspace_projects.up.sql` migration
3. Add all missing columns

### Issue 2: Authentication Required
**Error**: `error: when using non-Google workspace providers, Okta or Dex authentication must be enabled`

The server requires authentication when using local workspace provider.

**Options**:
1. Enable Dex on port 5556 (requires separate process)
2. Add development mode bypass in code
3. Use testing environment (Docker) which has Dex configured

### Issue 3: Other Models May Have Missing Columns
Models still in AutoMigrate list may have incomplete migration schemas. Full audit needed.

## 📊 Progress Metrics

**Database Initialization**:
- ✅ Migrations run successfully
- ✅ Tables created
- ✅ Instance UUID initialized
- ❌ Server startup blocked by auth + missing columns

**Files Modified**: 7
- 2 new migration files
- 1 new config file
- 2 modified core files (db.go, gorm.go)
- 1 migration updated (000002)
- 1 status document created

## 🎬 Next Steps

### Immediate (Get Server Running)
1. **Add missing workspace_projects columns** (create 000004 migration)
2. **Configure Dex auth** OR **add development mode bypass**
3. **Test full startup** → Health endpoint responding

### Short-term (Complete Migrations)
1. Audit ALL models vs migrations (systematic comparison)
2. Create migrations for missing columns
3. Re-enable AutoMigrate and verify no errors
4. Run full test suite

### Long-term (Remove AutoMigrate)
1. Complete all table schemas in migrations
2. Remove AutoMigrate entirely
3. Add migration testing to CI
4. Document migration workflow

## 📝 Lessons Learned

1. **GORM uniqueIndex behavior**: Always test migrations on fresh database, not just existing schemas
2. **Migration completeness**: Original 000001 migration was a starting point, never completed
3. **Config complexity**: google_workspace block is required even when disabled
4. **Testing environments**: Need both native and Docker workflows documented

## 🔗 References

**Models**: `pkg/models/*.go`  
**Migrations**: `internal/db/migrations/*.sql`  
**DB Init**: `internal/db/db.go`  
**AutoMigrate List**: `pkg/models/gorm.go`  
**Configs**: `testing/*.hcl`  

**Related Docs**:
- `docs-internal/DATABASE_MIGRATION_REFACTORING_SUMMARY.md`
- `docs-internal/todos/LOCAL_WORKFLOW_FIX_STATUS.md`
- `internal/db/migrations/README.md`
