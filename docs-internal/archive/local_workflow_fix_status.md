# Local Workflow Fix - Current Status

**Date**: 2025-10-24  
**Status**: 🔧 In Progress - Database Schema Fixes  
**Branch**: `jrepp/dev-tidy`

## 🎯 Goal
Get the local development workflow fully operational with native backend + frontend.

## 📊 Current State

### ✅ Completed
1. **Testing Environment**: Docker compose working (`make up`)
2. **Config System**: `testing/config-dex.hcl` with proper Dex OIDC integration
3. **Indexer Refactor**: Phase 1 & 2 complete (see `indexer-refactor.md`)
4. **Database Migrations**: Base migrations in place (000001, 000002)

### 🔴 Blocking Issues

#### Issue 1: GORM AutoMigrate Constraint Renaming Bug
**Status**: 🔴 CRITICAL BLOCKER  
**Root Cause**: GORM's AutoMigrate tries to rename `uniqueIndex` constraints on fresh databases, causing errors like:
```
ERROR: constraint "uni_indexer_folders_google_drive_id" of relation "indexer_folders" does not exist (SQLSTATE 42704)
ERROR: constraint "uni_workspace_projects_project_uuid" of relation "workspace_projects" does not exist
```

**Affected Models** (models with `uniqueIndex` GORM tag):
- `IndexerFolder` - `GoogleDriveID string gorm:"uniqueIndex"`
- `WorkspaceProject` - `ProjectUUID uuid.UUID gorm:"uniqueIndex"`  
- Potentially others...

**Impact**: Backend won't start - fails during database initialization.

**Solution Options**:
1. ✅ **Remove from AutoMigrate, add to SQL migrations** (cleanest)
2. ⚠️ **Change GORM tags to avoid rename** (workaround)
3. ❌ **Disable AutoMigrate entirely** (breaks other models)

**Action Taken**: 
- Removed `IndexerFolder` from AutoMigrate
- Added `indexer_folders` table to `000002_indexer_core.up.sql`
- **NEXT**: Remove `WorkspaceProject` and add to migrations

#### Issue 2: Missing Database Columns (RESOLVED for document_types)
**File**: `internal/db/migrations/000001_core_schema.up.sql`  
**Status**: ✅ Partially Fixed  
**Solution**: Created `000003_add_document_type_fields.up.sql` migration

#### Issue 3: Token File Permissions (RESOLVED)
**Status**: ✅ Fixed in `testing/config-dex.hcl`

## 🛠️ Immediate Fix Plan

### Current Status: Database Migrations Incomplete

**Root Problems Identified**:
1. ✅ **GORM uniqueIndex bug** - Temporarily disabled AutoMigrate to bypass
2. ⚠️ **Missing columns in migrations** - `document_types`, `workspace_projects`, others  
3. ⚠️ **Auth requirement** - Non-Google workspace requires Okta or Dex

**Pragmatic Solution** (as suggested):
1. ✅ Temporarily disabled AutoMigrate (commented out in `internal/db/db.go`)
2. ✅ Added `indexer_folders` table to migration `000002`
3. ✅ Created `000003_add_document_type_fields` for missing document_type columns
4. 🔄 **NEXT**: Add missing `workspace_projects` columns
5. 🔄 **THEN**: Configure Dex auth or add bypass for local development

**Files Modified**:
- `internal/db/db.go` - Commented out AutoMigrate (lines 99-107)
- `pkg/models/gorm.go` - Commented out `IndexerFolder`
- `internal/db/migrations/000002_indexer_core.up.sql` - Added indexer_folders table
- `internal/db/migrations/000003_add_document_type_fields.up.sql` - NEW migration
- `testing/config-minimal-native.hcl` - NEW minimal config for native development

### Step 1: Add Missing Columns to document_types (DONE)
Create migration: `000003_add_document_type_fields.up.sql`

```sql
-- Add missing document type fields
ALTER TABLE document_types 
  ADD COLUMN IF NOT EXISTS flight_icon TEXT,
  ADD COLUMN IF NOT EXISTS more_info_link_text TEXT,
  ADD COLUMN IF NOT EXISTS more_info_link_url TEXT,
  ADD COLUMN IF NOT EXISTS checks JSONB;
```

### Step 2: Verify Other Tables (NEXT)
Systematic comparison:
1. Read each model in `pkg/models/`
2. Read corresponding table in `000001_core_schema.up.sql`
3. Identify missing columns
4. Create additional migration if needed

### Step 3: Test Native Backend
```bash
# Build and run
make bin
./hermes server -config=testing/config-dex.hcl

# Verify health
curl -I http://localhost:8000/health

# Check logs for migration errors
```

### Step 4: Test Frontend
```bash
cd web
make web/proxy  # Auto-detects backend on 8000
```

### Step 5: Run E2E Validation
```bash
cd tests/e2e-playwright
npx playwright test --reporter=line --max-failures=1
```

## 📝 Notes from Investigation

### AutoMigrate Current Behavior
- **File**: `internal/db/db.go` lines 99-107
- Still runs after SQL migrations for backward compatibility
- Adds missing columns automatically
- **Problem**: Makes migrations incomplete/untested

### Migration System
- Uses `golang-migrate/migrate`
- Located in `internal/db/migrations/`
- Runs before AutoMigrate
- **Current State**: Only 2 migrations (core + indexer)

### Known Working Configurations
- ✅ Testing environment (Docker) works with AutoMigrate
- ✅ Dex OIDC authentication functional
- ✅ Meilisearch search provider operational
- ✅ Local workspace provider tested

## 🎬 Next Actions

1. **Immediate**: Create `000003_add_document_type_fields` migration
2. **Short-term**: Audit all tables vs models, create fixes
3. **Medium-term**: Remove AutoMigrate dependency (TODO in code)
4. **Long-term**: Add migration testing to CI

## 📚 Related Documents
- `docs-internal/INDEXER_API_IMPLEMENTATION_CHECKLIST.md` - API design
- `docs-internal/todos/indexer-refactor.md` - Phase tracking
- `docs-internal/DATABASE_MIGRATION_REFACTORING_SUMMARY.md` - Migration strategy
- `internal/db/migrations/readme.md` - Migration guidelines

## 🔗 References
**Models**: `pkg/models/*.go`  
**Migrations**: `internal/db/migrations/*.sql`  
**DB Setup**: `internal/db/db.go`  
**Config**: `testing/config-dex.hcl`
