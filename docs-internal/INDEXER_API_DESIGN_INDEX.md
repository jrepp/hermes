# Indexer API Design - Complete Documentation Index

**Date**: October 23, 2025  
**Status**: ✅ Design Phase Complete - Ready for Implementation  
**Update**: ✅ Revised with Project-Based Normalization (recommended approach)

## 🔥 Important Update: Project-Based Normalization

**RECOMMENDED**: See `INDEXER_API_PROJECT_NORMALIZATION.md` for the updated design that normalizes workspace provider data through project references instead of duplicating it in every document.

**Key Changes**:
- ✅ Documents reference `project_id` (not inline workspace metadata)
- ✅ Projects table stores provider configuration (single source of truth)
- ✅ Smaller database footprint, better consistency
- ✅ Natural support for migration tracking

## 📚 Documentation Structure

This design work is documented across multiple files for different audiences:

### 1. Executive Summary
**File**: `INDEXER_API_DESIGN_SUMMARY.md`  
**Audience**: Developers, Product Managers, Architects  
**Content**: High-level overview, key endpoints, workspace providers, advantages

**Key Sections**:
- API Endpoints Overview (4 main endpoints)
- Workspace Provider Types (GitHub, Local, Hermes, Google)
- Architecture Flow (Indexer → API → Database)
- Project Config Integration
- Database Schema Changes
- Next Steps

### 2. Visual Diagrams
**File**: `INDEXER_API_DIAGRAMS.md`  
**Audience**: Visual learners, System architects  
**Content**: ASCII diagrams showing flows and architecture

**Diagrams**:
- Complete API-based indexer flow (from discovery to storage)
- Workspace provider types and resolution
- Project config → workspace resolution
- Command pipeline with API client
- Migration strategy (DB → API)
- Testing environment integration

### 3. Detailed API Specification
**File**: `INDEXER_IMPLEMENTATION_GUIDE.md` (section added starting line ~1200)  
**Audience**: Backend developers implementing the API  
**Content**: Complete API specification with request/response schemas

**Sections**:
- Endpoint specifications (POST, PUT, GET)
- Request body schemas with JSON examples
- Response schemas with status codes
- Workspace provider metadata formats
- Authentication (service tokens, OIDC)
- Database schema DDL
- Integration test usage examples

### 4. Architecture Context
**File**: `INDEXER_REFACTOR_PLAN.md` (section added: "API-Based Architecture")  
**Audience**: Architects, Senior developers  
**Content**: Architectural rationale and design decisions

**Sections**:
- Overview and benefits
- Architecture flow diagram (updated)
- Workspace provider resolution
- Advantages over direct DB access (comparison table)
- Migration strategy (phased approach)
- Authentication approaches
- Project config integration

### 5. Project-Based Normalization (UPDATED)
**File**: `INDEXER_API_PROJECT_NORMALIZATION.md`  
**Audience**: All developers (MUST READ)  
**Content**: Updated design using project references instead of inline workspace metadata

**Key Benefits**:
- Single source of truth for provider config
- Smaller database (no duplicated JSON)
- Natural migration tracking
- Easier config management

### 6. Implementation Checklist
**File**: `INDEXER_API_IMPLEMENTATION_CHECKLIST.md`  
**Audience**: Developers implementing the changes  
**Content**: Step-by-step implementation guide with time estimates (updated with project normalization)

**Phases** (12-18 hours total):
1. API Implementation (4-5 hours)
2. API Client (2-3 hours)
3. Command Refactoring (2-3 hours)
4. Project Config Integration (1-2 hours)
5. Test Updates (1-2 hours)
6. Testing & Validation (2-3 hours)

## 🎯 Quick Navigation

### I want to understand the overall design
→ **START HERE**: `INDEXER_API_PROJECT_NORMALIZATION.md` (updated design)  
→ Then read: `INDEXER_API_DESIGN_SUMMARY.md` (original design)

### I need to see the architecture visually
→ Check `INDEXER_API_DIAGRAMS.md`

### I'm implementing the API endpoints
→ Use `INDEXER_IMPLEMENTATION_GUIDE.md` (line 1200+)  
→ Follow `INDEXER_API_IMPLEMENTATION_CHECKLIST.md` Phase 1

### I'm updating the integration test
→ Use `INDEXER_API_IMPLEMENTATION_CHECKLIST.md` Phases 2-5  
→ Reference `INDEXER_API_DIAGRAMS.md` for testing flow

### I need to understand the big picture
→ Read `INDEXER_REFACTOR_PLAN.md` API-Based Architecture section

### I want code examples
→ Check `INDEXER_IMPLEMENTATION_GUIDE.md` Integration Test Usage  
→ See `INDEXER_API_IMPLEMENTATION_CHECKLIST.md` code snippets

## 📋 Design Deliverables

✅ **API Endpoints**: 4 main endpoints defined with full specs
- POST /api/v2/indexer/documents
- POST /api/v2/indexer/documents/:uuid/revisions
- PUT /api/v2/indexer/documents/:uuid/summary
- PUT /api/v2/indexer/documents/:uuid/embeddings

✅ **Workspace Providers**: 4 types supported
- GitHub (repository, branch, path, commit SHA)
- Local (filesystem path, project root)
- Hermes (remote endpoint, document ID)
- Google (backward compatibility)

✅ **Database Schema**: Complete DDL provided
- documents table: 4 new fields
- document_revisions table: full schema
- document_embeddings table: pgvector support

✅ **Request/Response Schemas**: All defined with JSON examples

✅ **Authentication**: 2 approaches documented
- Service tokens (production)
- OIDC/Dex (testing)

✅ **Migration Strategy**: Phased approach (3 phases)
- Phase 1: Dual support (DB + API)
- Phase 2: Deprecation warnings
- Phase 3: API only

✅ **Integration Plan**: Complete with project config

✅ **Diagrams**: 6 major flow diagrams

✅ **Implementation Checklist**: 6 phases, 12-18 hours

## 🔍 Key Design Decisions

### 1. API-Based vs Direct DB Access
**Decision**: API-based  
**Rationale**: 
- Separation of concerns
- Support external document sources
- Service isolation and scalability
- Proper authentication/authorization
- Easier testing (mock API responses)

**Documented in**: `INDEXER_REFACTOR_PLAN.md` (Advantages table)

### 2. Workspace Provider Metadata
**Decision**: Store as JSONB in workspace_provider_metadata field  
**Rationale**:
- Flexible schema for different provider types
- No need for separate tables per provider
- Easy to add new providers
- PostgreSQL JSONB supports indexing

**Documented in**: `INDEXER_IMPLEMENTATION_GUIDE.md` (Database Schema)

### 3. Revision Tracking by Content Hash
**Decision**: Use SHA-256 content hash as primary identifier  
**Rationale**:
- Automatic duplicate detection
- Works across providers
- Efficient comparison
- UNIQUE constraint prevents duplicates

**Documented in**: `INDEXER_API_DESIGN_SUMMARY.md` (Revision Tracking)

### 4. Project Config Integration
**Decision**: Use HCL project config to resolve workspaces  
**Rationale**:
- Consistent with distributed projects architecture
- Single source of truth for workspace configuration
- Supports multiple workspace types
- Easier testing (switch between test/prod configs)

**Documented in**: `INDEXER_REFACTOR_PLAN.md` (Project Config Integration)

### 5. Upsert Semantics for Documents
**Decision**: POST creates or updates (by UUID)  
**Rationale**:
- Idempotent operations (safe to retry)
- Handles re-indexing naturally
- Simpler client code
- Returns status (created vs updated)

**Documented in**: `INDEXER_IMPLEMENTATION_GUIDE.md` (Endpoint 1)

## 🧪 Testing Strategy

### Unit Tests
**Location**: `internal/api/v2/indexer_test.go`  
**Coverage**: 
- Happy path for each endpoint
- Error cases (404, 409, 400, 401)
- Duplicate detection
- Content hash mismatch

### Integration Tests
**Location**: `tests/integration/indexer/full_pipeline_test.go`  
**Coverage**:
- Full pipeline with API client
- Project config resolution
- Document creation via API
- Revision tracking via API
- Summary storage via API
- Meilisearch indexing
- End-to-end verification

**Documented in**: `INDEXER_API_IMPLEMENTATION_CHECKLIST.md` Phase 6

## 🚀 Implementation Order

1. **Database Migrations** (30 min)
   - Add fields to documents table
   - Create document_revisions table
   - Create document_embeddings table

2. **Models** (1 hour)
   - Update Document model
   - Create DocumentRevision model
   - Create DocumentEmbedding model

3. **API Handlers** (4-5 hours)
   - IndexerDocumentsHandler
   - IndexerRevisionsHandler
   - IndexerSummaryHandler
   - IndexerEmbeddingsHandler

4. **Route Registration** (30 min)
   - Add routes to server.go
   - Apply auth middleware

5. **API Client** (2-3 hours)
   - Create IndexerAPIClient struct
   - Implement methods for each endpoint
   - Add error handling

6. **Command Refactoring** (2-3 hours)
   - Update TrackCommand
   - Update TrackRevisionCommand
   - Update SummarizeCommand

7. **Project Config Integration** (1-2 hours)
   - Load project config in test
   - Create ProjectWorkspaceDiscoverCommand
   - Update discovery flow

8. **Testing** (2-3 hours)
   - Unit tests for API handlers
   - Integration test updates
   - End-to-end verification

**Total**: 12-18 hours

## 📞 Support Resources

### Questions About Design
- Check `INDEXER_API_DESIGN_SUMMARY.md` first
- Review diagrams in `INDEXER_API_DIAGRAMS.md`
- Read rationale in `INDEXER_REFACTOR_PLAN.md`

### Implementation Questions
- Follow `INDEXER_API_IMPLEMENTATION_CHECKLIST.md`
- Reference API spec in `INDEXER_IMPLEMENTATION_GUIDE.md`
- Check code examples in checklist

### Architecture Questions
- Review `INDEXER_REFACTOR_PLAN.md` API-Based Architecture
- Check advantages comparison table
- Review migration strategy

## ✅ Acceptance Criteria

Before marking design as complete:
- [x] All 4 API endpoints defined with full specs
- [x] Request/response schemas documented with JSON examples
- [x] Database schema changes specified with DDL
- [x] Workspace provider types defined (4 types)
- [x] Authentication approaches documented (2 methods)
- [x] Migration strategy outlined (3 phases)
- [x] Architecture diagrams created (6 diagrams)
- [x] Implementation checklist completed (6 phases)
- [x] Integration test plan documented
- [x] Success criteria defined

## 🎉 Design Complete!

All design work is complete and documented. Ready for implementation.

**Start here**: `INDEXER_API_IMPLEMENTATION_CHECKLIST.md` Phase 1
