---
date: 2025-11-15
title: RFC Implementation Tracker & Roadmap
type: memo
status: active
author: Claude Code
tags: [rfc, roadmap, planning, implementation]
---

# RFC Implementation Tracker & Roadmap

**Date**: November 15, 2025
**Status**: Active Tracking Document
**Last Updated**: 2025-11-15 13:33 PST

## Executive Summary

This memo tracks the implementation status of all active RFCs and provides recommendations for prioritization. As of Nov 15, 2025:

- **RFC-089 (S3 Storage & Migrations)**: ✅ Phase 1 Complete, All Tests Passing
- **RFC-088 (Event-Driven Indexer)**: ⚙️ 40% Complete, Architecture Done
- **RFC-087 (Notifications)**: 🔔 85% Complete, Production Ready
- **RFC-090 (Admin Interface)**: 📝 Design Only, Not Started

**Recommended Next Focus**: Complete RFC-088 (Event-Driven Indexer) to unblock AI document features.

---

## Current Implementation Status

### ✅ RFC-089: S3 Storage Backend and Document Migration System

**Status**: Phase 1 Complete, All Tests Passing ✅
**Progress**: 90% Complete
**Last Milestone**: Migration E2E tests all passing (Nov 15, 2025)

#### Completed
- ✅ S3 adapter fully implemented (8 files, 2,040 LOC)
  - Full RFC-084 WorkspaceProvider interface
  - S3 versioning support
  - Metadata storage strategies (tags, manifest, DynamoDB planned)
  - Content validation with SHA-256 hashing
- ✅ Migration system with transactional outbox pattern
  - Migration manager and worker
  - Hash normalization for content validation
  - Retry logic and error handling
- ✅ Database schema (4 tables)
  - `provider_storage` - Provider registry
  - `migration_jobs` - Job tracking
  - `migration_items` - Per-document migration state
  - `migration_outbox` - Transactional event queue
- ✅ Comprehensive E2E integration tests
  - **All 10 test phases passing**
  - **All 20 strong signal validations passing**
  - Job completeness (7 checks)
  - Content integrity (3 checks)
  - Outbox integrity (4 checks)
  - Migration invariants (5 checks)
  - S3 storage (1 check)

#### Test Results (Nov 15, 2025)
```
--- PASS: TestMigrationE2E (5.77s)
    --- PASS: TestMigrationE2E/Phase0_Prerequisites (1.33s)
    --- PASS: TestMigrationE2E/Phase1_DatabasePrerequisites (0.04s)
    --- PASS: TestMigrationE2E/Phase2_ProviderRegistration (0.03s)
    --- PASS: TestMigrationE2E/Phase3_CreateTestDocuments (0.00s)
    --- PASS: TestMigrationE2E/Phase4_MigrationJobCreation (0.00s)
    --- PASS: TestMigrationE2E/Phase5_QueueDocuments (0.04s)
    --- PASS: TestMigrationE2E/Phase6_StartMigrationJob (0.01s)
    --- PASS: TestMigrationE2E/Phase7_WorkerProcessing (4.06s)
    --- PASS: TestMigrationE2E/Phase8_VerifyMigrationResults (0.09s)
    --- PASS: TestMigrationE2E/Phase9_ProgressTracking (0.01s)
    --- PASS: TestMigrationE2E/Phase9b_StrongSignalValidation (0.11s)
    --- PASS: TestMigrationE2E/Phase10_Cleanup (0.04s)

✅ SUMMARY: 20 passed, 0 failed, 20 total
```

#### Recent Fixes (Nov 15, 2025)
1. **Payload Structure Mismatch**
   - Fixed test to create flat payload matching `TaskPayload` struct
   - Changed from nested to proper field names
   - File: `tests/integration/migration/migration_e2e_test.go:323-336`

2. **Hash Format Normalization**
   - Added hash normalization in worker before content validation
   - Handles mismatch between providers (with/without `sha256:` prefix)
   - File: `pkg/migration/worker.go:267-307`

3. **Test Hash Comparison**
   - Added hash prefix stripping in Phase 8 verification
   - File: `tests/integration/migration/migration_e2e_test.go:559-564`

#### Next Steps
- [ ] REST API endpoints for migration management
  - POST /api/v2/migrations/jobs - Create migration job
  - GET /api/v2/migrations/jobs/:id - Job status and progress
  - GET /api/v2/migrations/jobs/:id/items - Item-level details
  - POST /api/v2/migrations/jobs/:id/retry - Retry failed items
  - GET /api/v2/providers - List storage providers
- [ ] Provider router implementation
  - Route reads to appropriate provider based on document location
  - Handle provider failover
  - Load balancing across replicas
- [ ] Documentation and operational guides
  - API documentation
  - Migration cookbook
  - Operator runbook
  - Example HCL configurations
- [ ] Admin UI integration (depends on RFC-090)

#### Files
**Core Implementation**:
- `pkg/migration/manager.go` - Migration orchestration
- `pkg/migration/worker.go` - Worker processing logic
- `pkg/migration/types.go` - Type definitions
- `pkg/workspace/adapters/s3/*.go` - S3 adapter (8 files)
- `pkg/workspace/router/router.go` - Multi-provider routing

**Database**:
- `internal/migrate/migrations/000011_add_s3_migration_tables.up.sql`
- `internal/migrate/migrations/000011_add_s3_migration_tables.down.sql`

**Tests**:
- `tests/integration/migration/migration_e2e_test.go` - E2E tests
- `tests/integration/migration/validation_test.go` - Strong signal validators
- `tests/integration/migration/prerequisites_test.go` - Setup validation
- `pkg/workspace/adapters/s3/adapter_test.go` - S3 adapter tests

**Documentation**:
- `docs-internal/rfc/RFC-089-s3-storage-backend-and-migrations.md` - Full RFC
- `docs-internal/rfc/RFC-089-IMPLEMENTATION-SUMMARY.md` - Progress tracking
- `testing/RFC-089-TESTING-GUIDE.md` - Testing documentation
- `testing/STATUS.md` - Quick status overview

#### Estimate to Completion
**Remaining Work**: 1 week
**Confidence**: High (core functionality complete and tested)

---

### ⚙️ RFC-088: Event-Driven Document Indexer with Pipeline Rulesets

**Status**: Phase 1 Complete, Phase 2 In Progress
**Progress**: 40% Complete
**Last Milestone**: Architecture refactoring complete (Nov 15, 2025)

#### Completed
- ✅ Architecture refactoring complete
  - Relay moved from separate binary into main hermes server
  - Indexer made completely database-independent (stateless)
  - Clean separation: server manages state, indexers process events
- ✅ Transactional outbox pattern
  - `document_revision_outbox` table
  - `document_revision_pipeline_executions` table
  - Idempotent keys: `{document_uuid}:{content_hash}`
- ✅ Publisher and relay infrastructure
  - Publisher: `pkg/indexer/publisher/publisher.go`
  - Relay: `pkg/indexer/relay/relay.go` (embedded in main server)
- ✅ Ruleset system for flexible document matching
  - Configurable conditions (equality, IN, gt/lt, contains)
  - Example: `document_type = "RFC"` → `["search_index", "llm_summary"]`
- ✅ Pipeline executor framework
  - Step execution engine
  - Error handling and retry logic
  - Per-step results recording (when DB enabled)
- ✅ Consumer worker (stateless)
  - Consumes from Redpanda
  - Reconstructs DocumentRevision from event payload
  - No database dependency
- ✅ Shared infrastructure
  - `pkg/database/database.go` - Centralized DB connections
  - `pkg/kafka/config.go` - Kafka/Redpanda helpers

#### In Progress
- ⏳ LLM integration (OpenAI/Ollama/Bedrock clients started)
  - OpenAI client: `pkg/llm/openai.go` - Structure defined, needs completion
  - Ollama client: `pkg/llm/ollama.go` - Local LLM support started
  - Bedrock client: `pkg/llm/bedrock.go` - AWS Bedrock integration started
  - Summary step: `pkg/indexer/pipeline/steps/llm_summary.go` - Framework ready

#### Next Steps
1. **Complete LLM Integration** (Week 1)
   - Finish OpenAI client implementation
   - Complete Ollama local LLM support
   - Complete AWS Bedrock integration
   - Test summary generation end-to-end
   - Add token usage tracking and rate limiting

2. **Embeddings Pipeline** (Week 2)
   - Implement embeddings pipeline step
   - Integrate with vector store (Meilisearch or Pinecone)
   - Enable semantic search capabilities
   - Test embedding generation and storage

3. **Testing & Production** (Week 3)
   - Integration tests with Redpanda
   - E2E tests: document change → indexed with summary
   - Load testing (1000+ docs/hour)
   - Update API handlers to use publisher
   - Run old and new indexers in parallel for validation

#### Files
**Core Infrastructure**:
- `pkg/indexer/publisher/publisher.go` - Event publishing
- `pkg/indexer/relay/relay.go` - Outbox → Redpanda relay
- `pkg/indexer/consumer/consumer.go` - Event consumer
- `pkg/indexer/pipeline/executor.go` - Pipeline execution engine
- `pkg/indexer/ruleset/ruleset.go` - Document matching rules

**Pipeline Steps**:
- `pkg/indexer/pipeline/steps/search_index.go` - Meilisearch indexing
- `pkg/indexer/pipeline/steps/llm_summary.go` - AI summary generation

**LLM Clients** (In Progress):
- `pkg/llm/openai.go` - OpenAI integration
- `pkg/llm/ollama.go` - Local LLM support
- `pkg/llm/bedrock.go` - AWS Bedrock integration

**Database**:
- `internal/migrate/migrations/000010_add_document_revision_outbox.up.sql`
- `pkg/models/document_revision_outbox.go`
- `pkg/models/document_revision_pipeline_execution.go`

**Binaries**:
- `cmd/hermes/main.go` - Main server (includes relay)
- `cmd/hermes-indexer/main.go` - Stateless indexer worker
- `internal/cmd/commands/server/server.go` - Server with relay goroutine

**Tests**:
- `tests/integration/indexer/e2e_test.go` - E2E tests (planned)
- `pkg/indexer/*/` - Unit tests throughout

**Documentation**:
- `docs-internal/rfc/RFC-088-event-driven-indexer.md` - Full RFC
- `docs-internal/rfc/RFC-088-IMPLEMENTATION-SUMMARY.md` - Progress tracking
- `docs-internal/rfc/RFC-088-ARCHITECTURE-REFACTORING.md` - Architecture changes
- `docs-internal/rfc/RFC-088-TESTING-STATUS.md` - Test status
- `configs/indexer-worker-example.hcl` - Configuration example

#### Estimate to Completion
**Remaining Work**: 2-3 weeks
**Confidence**: Medium (LLM integration is main unknown)

#### Success Criteria
- [ ] 100% of documents have AI summaries within 1 hour
- [ ] Semantic search returns relevant results (>80% accuracy)
- [ ] Indexer processes 1000+ docs/hour
- [ ] Zero data loss during migration from old indexer

---

### 🔔 RFC-087: Multi-Backend Notification System

**Status**: Phase 1-3 Complete, Production Ready
**Progress**: 85% Complete
**Last Milestone**: All core features implemented (Nov 14, 2025)

#### Completed
- ✅ Phase 1: Foundation & Core Infrastructure
  - Message schema with server-side template resolution
  - Template system with embedded templates (4 notification types)
  - Template validation (prevents `<no value>` and unexpanded `{{...}}`)
  - Backend registry with HCL configuration
  - Audit backend for compliance logging
  - Mail backend (SMTP with TLS)
  - Ntfy backend for push notifications
  - Backend-specific message filtering
  - Notification provider (server-side)
  - Docker Compose configuration
  - Integration tests (all passing)
  - HCL configuration system

- ✅ Phase 2: E2E Testing and Deployment
  - All 3 notifier services running (audit, mail, ntfy)
  - Redpanda broker configured and healthy
  - Consumer group stable with 0 lag
  - Backend-specific filtering working
  - Full E2E message flow verified

- ✅ Phase 3: Critical Reliability Features
  - Producer durability (idempotent producer, compression, retry)
  - Backend error handling (proper error types, retryable classification)
  - Retry logic (exponential backoff, max retries, retry metadata)
  - Dead Letter Queue (DLQ publisher, monitor, failure tracking)
  - Graceful shutdown (signal handling, in-flight tracking)

#### Operational Status
- ✅ All 3 notifier services running and healthy
- ✅ Message publishing and consumption working end-to-end
- ✅ Template resolution producing correct output
- ✅ Backend routing functioning correctly
- ✅ Docker Compose environment fully operational

#### Nice-to-Have Enhancements (Not Critical)
- [ ] Message encryption (PII protection)
- [ ] Prometheus metrics and Grafana dashboards
- [ ] Additional backends (Slack, Discord, MS Teams)
- [ ] Rate limiting and circuit breakers
- [ ] Retry topic implementation (currently using exponential backoff in main topic)
- [ ] Message ordering and partitioning (basic support via partition keys)
- [ ] Duplicate message handling (Redis-based deduplication)
- [ ] Transaction support and outbox pattern (for guaranteed delivery)
- [ ] Template injection prevention (basic validation in place)

#### Files
**Core Implementation**:
- `pkg/notifications/publisher.go` - Message publishing
- `pkg/notifications/backends/registry.go` - Backend registry
- `pkg/notifications/backends/audit.go` - Audit backend
- `pkg/notifications/backends/mail.go` - SMTP email backend
- `pkg/notifications/backends/ntfy.go` - Push notification backend
- `pkg/notifications/retry.go` - Retry logic
- `pkg/notifications/dlq.go` - Dead Letter Queue
- `internal/notifications/provider.go` - Server-side provider
- `internal/notifications/templates.go` - Template system

**Binaries**:
- `cmd/hermes-notify/main.go` - Notification worker

**Templates**:
- `internal/notifications/templates/*/subject.tmpl` - Email subjects
- `internal/notifications/templates/*/body.md.tmpl` - Markdown bodies
- `internal/notifications/templates/*/body.html.tmpl` - HTML email bodies

**Tests**:
- `tests/integration/notifications/e2e_test.go` - E2E tests
- `tests/integration/notifications/notifications_test.go` - Integration tests

**Configuration**:
- `testing/notifier-audit.hcl` - Audit notifier config
- `testing/notifier-mail.hcl` - Mail notifier config
- `testing/notifier-ntfy.hcl` - Ntfy notifier config

**Documentation**:
- `docs-internal/rfc/RFC-087-NOTIFICATION-BACKEND.md` - Main RFC
- `docs-internal/rfc/RFC-087-IMPLEMENTATION-STATUS.md` - Progress tracking
- `docs-internal/rfc/RFC-087-ADDENDUM.md` - Critical fixes
- `docs-internal/rfc/RFC-087-TEMPLATE-SCHEME.md` - Template architecture
- `docs-internal/rfc/RFC-087-MESSAGE-SCHEMA.md` - Message format
- `docs-internal/rfc/RFC-087-BACKENDS.md` - Backend implementations

#### Estimate to Completion
**Remaining Work**: Optional enhancements, 1 week if desired
**Confidence**: High (production ready now)

#### Recommendation
Deploy current implementation to production. Enhancements can be added later based on operational needs.

---

### 📝 RFC-090: Hermes Admin Interface

**Status**: Design Only, Not Started
**Progress**: 0% Complete
**Last Milestone**: RFC document completed (Nov 15, 2025)

#### Design Complete
- ✅ Comprehensive RFC written (68,804 bytes)
- ✅ Architecture defined (React/TypeScript SPA)
- ✅ API endpoints specified
- ✅ Database schema designed
- ✅ UI mockups described

#### Planned Features
1. **User Identity Management**
   - View unified user identities across OAuth providers
   - Link/unlink OAuth identities
   - Audit trail for identity changes
   - Resolve "Who is this user?" questions

2. **Project Administration**
   - Project lifecycle management
   - Permissions and metadata
   - Team assignments

3. **Migration Orchestration** (depends on RFC-089)
   - Schedule and monitor migrations
   - Retry failed migrations
   - View migration progress

4. **Indexer Health Dashboard** (depends on RFC-088)
   - Real-time pipeline health monitoring
   - Consumer lag metrics
   - Failed execution management
   - Performance analytics

5. **Documentation Analytics**
   - Document usage patterns
   - Team productivity metrics
   - Search analytics
   - Creation trends

#### Phased Implementation Plan
**Phase 1**: Migration Dashboard (2 weeks)
- View all migration jobs
- Monitor progress
- Retry failed items
- Cancel jobs
**Dependency**: RFC-089 API endpoints

**Phase 2**: Indexer Health Dashboard (1 week)
- Pipeline execution stats
- Consumer lag metrics
- Failed executions
- Retry management
**Dependency**: RFC-088 in production

**Phase 3**: Identity Management (1 week)
- View unified identities
- Link/unlink OAuth providers
- Audit trail

**Phase 4**: Analytics & Insights (2 weeks)
- Document usage patterns
- Team productivity metrics
- Search analytics

#### Files
**Documentation**:
- `docs-internal/rfc/RFC-090-admin-interface.md` - Complete RFC design

**To Be Created**:
- Frontend: React/TypeScript SPA
- Backend: `internal/api/v2/admin/*.go` - Admin API endpoints
- Database: Additional tables for identity management

#### Estimate to Completion
**Total Work**: 6-8 weeks (all phases)
**Confidence**: Medium (depends on RFC-088 and RFC-089 completion)

#### Recommendation
Start with Phase 1 (Migration Dashboard) after RFC-089 API endpoints are complete. This provides immediate value and builds momentum for subsequent phases.

---

## Prioritization & Roadmap

### Priority Matrix

| RFC | Status | Impact | Effort | Priority | Start When |
|-----|--------|--------|--------|----------|------------|
| **RFC-088** | 40% | HIGH | Medium (2-3w) | **1st** | **Now** |
| **RFC-089 API** | 90% | HIGH | Small (1w) | **2nd** | After RFC-088 |
| **RFC-090 Phase 1** | 0% | HIGH | Medium (2w) | **3rd** | After RFC-089 API |
| **RFC-090 Phase 2** | 0% | HIGH | Medium (1w) | **4th** | After Phase 1 |
| **RFC-087 Hardening** | 85% | Medium | Small (1w) | **5th** | Later |

### Recommended Sequence

#### **Weeks 1-2: RFC-088 LLM Integration**
**Goal**: Enable AI-generated document summaries

**Tasks**:
- Complete OpenAI client (`pkg/llm/openai.go`)
- Complete Ollama client (`pkg/llm/ollama.go`)
- Complete Bedrock client (`pkg/llm/bedrock.go`)
- Finish LLM summary step (`pkg/indexer/pipeline/steps/llm_summary.go`)
- Test end-to-end summary generation
- Add token usage tracking and rate limiting

**Deliverables**:
- ✅ AI summaries for all documents
- ✅ Support for multiple LLM providers
- ✅ Token tracking and cost monitoring

**Success Criteria**:
- 100% of documents get summaries within 1 hour
- Token usage tracked and reported
- Summary quality validated by users

---

#### **Week 3: RFC-088 Embeddings & Semantic Search**
**Goal**: Enable semantic document search

**Tasks**:
- Implement embeddings pipeline step
- Integrate with vector store (Meilisearch or Pinecone)
- Update search index with embeddings
- Test semantic search queries

**Deliverables**:
- ✅ Document embeddings generated
- ✅ Semantic search functional
- ✅ Improved search relevance

**Success Criteria**:
- Semantic search returns relevant results (>80% accuracy)
- Search latency < 200ms for 10,000 documents

---

#### **Week 4: RFC-088 Production Testing & Migration**
**Goal**: Production-ready indexer deployment

**Tasks**:
- Integration tests with Redpanda
- E2E tests: document change → indexed with summary
- Load testing (1000+ docs/hour)
- Update API handlers to use publisher
- Run old and new indexers in parallel
- Validate consistency
- Decommission old indexer

**Deliverables**:
- ✅ Production-ready indexer
- ✅ Migration from old system complete
- ✅ Zero data loss verified

**Success Criteria**:
- Processes 1000+ docs/hour
- Zero data loss during migration
- All documents indexed with summaries

---

#### **Week 5: RFC-089 API Endpoints**
**Goal**: REST API for migration management

**Tasks**:
- POST /api/v2/migrations/jobs - Create migration job
- GET /api/v2/migrations/jobs/:id - Job status
- GET /api/v2/migrations/jobs/:id/items - Item progress
- POST /api/v2/migrations/jobs/:id/retry - Retry failed
- GET /api/v2/providers - List providers
- Implement provider router
- Write API documentation
- Create operator runbook

**Deliverables**:
- ✅ REST API for migrations
- ✅ Provider routing functional
- ✅ Operational documentation

**Success Criteria**:
- API can create and monitor migrations
- Provider routing directs reads correctly
- Can migrate 10,000 docs with 100% integrity

---

#### **Weeks 6-7: RFC-090 Phase 1 (Migration Dashboard)**
**Goal**: Admin UI for migration management

**Tasks**:
- React/TypeScript frontend setup
- Migration jobs list view
- Job detail view with progress
- Retry failed items UI
- Cancel job functionality
- Real-time progress updates (WebSocket)

**Deliverables**:
- ✅ Migration dashboard UI
- ✅ Real-time progress monitoring
- ✅ Admin can manage migrations without API calls

**Success Criteria**:
- Admin can schedule migrations via UI
- Real-time progress visible
- Failed items can be retried with one click

---

#### **Week 8: RFC-090 Phase 2 (Indexer Health Dashboard)**
**Goal**: Operational visibility for indexer system

**Tasks**:
- Pipeline execution stats view
- Consumer lag monitoring
- Failed execution management
- Retry controls
- Performance graphs

**Deliverables**:
- ✅ Indexer health dashboard
- ✅ Operational metrics visible
- ✅ Admin can diagnose issues

**Success Criteria**:
- Admin can see indexer health at a glance
- Failed executions identifiable and retryable
- No database queries needed for troubleshooting

---

#### **Week 9+: RFC-090 Phase 3-4 (Identity & Analytics)**
**Goal**: Complete admin interface

**Tasks**:
- Identity management UI
- OAuth provider linking
- Audit trail
- Document analytics
- Team productivity metrics

**Deliverables**:
- ✅ Complete admin interface
- ✅ Identity management functional
- ✅ Analytics dashboards

---

### Why This Order?

1. **RFC-088 First**: Unblocks AI features that teams are waiting for. Core infrastructure already done, just needs LLM integration.

2. **RFC-089 API Second**: Quick win (1 week) that enables migration capabilities. Builds on tested foundation.

3. **RFC-090 Phases 3-4**: Provides UI for managing systems built in earlier weeks. Each phase builds on previous work.

4. **RFC-087 Hardening Last**: Already production ready. Enhancements are nice-to-have, not critical.

---

## Success Metrics

### RFC-088 (Event-Driven Indexer)
- [ ] 100% of documents have AI summaries within 1 hour of creation/update
- [ ] Semantic search returns relevant results (>80% accuracy on test queries)
- [ ] Indexer processes 1000+ documents/hour under load
- [ ] Zero data loss during migration from old indexer
- [ ] Consumer lag < 100 messages at peak load
- [ ] Token usage tracked and under budget

### RFC-089 (S3 Storage & Migrations)
- [ ] API can create and monitor migration jobs via REST endpoints
- [ ] Provider routing directs reads to correct storage backend
- [ ] Can migrate 10,000 documents with 100% content integrity
- [ ] Zero downtime during provider failover
- [ ] Migration jobs complete in < 2 hours for 1000 documents

### RFC-090 (Admin Interface)
- [ ] Admin can view all migrations in one dashboard
- [ ] Admin can monitor indexer health without database queries
- [ ] Identity management reduces support tickets by 50%
- [ ] Analytics dashboard used weekly by leadership
- [ ] Time to resolve admin tasks reduced by 75%

### RFC-087 (Notifications)
- [ ] 100% notification delivery rate (with retries)
- [ ] < 5 second latency from event to notification
- [ ] DLQ messages < 0.1% of total
- [ ] Zero lost notifications
- [ ] Multiple backend support operational

---

## Risk Assessment

### RFC-088 Risks
**Risk**: LLM integration complexity
**Mitigation**: Start with OpenAI (simpler), add Ollama/Bedrock later
**Impact**: Medium

**Risk**: Token costs exceed budget
**Mitigation**: Implement token tracking early, add rate limiting
**Impact**: Medium

**Risk**: Embeddings integration performance
**Mitigation**: Test with Meilisearch first (simpler), evaluate Pinecone later
**Impact**: Low

### RFC-089 Risks
**Risk**: Provider routing bugs cause data loss
**Mitigation**: Extensive testing, canary deployments
**Impact**: High (but well-tested)

**Risk**: Migration scale issues
**Mitigation**: Start with small batches, load test thoroughly
**Impact**: Medium

### RFC-090 Risks
**Risk**: Scope creep (too many features)
**Mitigation**: Stick to phased approach, resist feature additions
**Impact**: High

**Risk**: Frontend complexity
**Mitigation**: Use proven React patterns, keep UI simple
**Impact**: Medium

### RFC-087 Risks
**Risk**: None - already production ready
**Mitigation**: N/A
**Impact**: None

---

## Resource Requirements

### Development Time
- **RFC-088**: 2-3 weeks (1 developer)
- **RFC-089 API**: 1 week (1 developer)
- **RFC-090**: 6-8 weeks (1-2 developers)
- **RFC-087 Hardening**: Optional, 1 week

**Total**: ~10-13 weeks for complete implementation

### Infrastructure
- Redpanda/Kafka cluster (already deployed)
- Meilisearch (already deployed)
- OpenAI API key + budget
- Optional: Ollama server for local LLM
- Optional: Pinecone for vector storage
- MinIO/S3 storage (already deployed for testing)

### Testing Requirements
- Integration test environment
- Load testing tools
- E2E test automation
- Monitoring and observability tools

---

## Decision Log

### 2025-11-15: Prioritize RFC-088 Over RFC-089 API
**Decision**: Complete RFC-088 (indexer) before RFC-089 API endpoints
**Rationale**: Higher user impact, already 40% complete, unblocks AI features
**Alternative Considered**: RFC-089 API first (rejected due to lower urgency)

### 2025-11-15: Phased Approach for RFC-090
**Decision**: Break RFC-090 into 4 phases instead of all-at-once
**Rationale**: Reduces risk, delivers value incrementally, easier to manage
**Alternative Considered**: Full implementation (rejected due to scope)

### 2025-11-15: Defer RFC-087 Hardening
**Decision**: Skip RFC-087 enhancements for now
**Rationale**: Already production ready, enhancements are nice-to-have
**Alternative Considered**: Add encryption first (rejected due to time)

---

## Next Actions

### Immediate (This Week)
1. ✅ Commit RFC-089 test fixes - **DONE**
2. 🎯 Start RFC-088 LLM integration
   - Set up OpenAI API credentials
   - Complete openai.go client implementation
   - Write integration tests for summary generation

### Week 1-2
1. Complete LLM clients (OpenAI, Ollama, Bedrock)
2. Finish LLM summary pipeline step
3. Test end-to-end summary generation
4. Add token usage tracking

### Week 3
1. Implement embeddings pipeline step
2. Integrate with vector store
3. Test semantic search

### Week 4
1. Production testing and validation
2. Migration from old indexer
3. Update API handlers

### Week 5+
1. RFC-089 API endpoints
2. RFC-090 Phase 1 (Migration Dashboard)
3. Continue through roadmap phases

---

## Related Documents

- [RFC-088: Event-Driven Indexer](../rfc/RFC-088-event-driven-indexer.md)
- [RFC-089: S3 Storage & Migrations](../rfc/RFC-089-s3-storage-backend-and-migrations.md)
- [RFC-087: Notification Backend](../rfc/RFC-087-NOTIFICATION-BACKEND.md)
- [RFC-090: Admin Interface](../rfc/RFC-090-admin-interface.md)
- [RFC-088 Implementation Summary](../rfc/RFC-088-IMPLEMENTATION-SUMMARY.md)
- [RFC-089 Implementation Summary](../rfc/RFC-089-IMPLEMENTATION-SUMMARY.md)
- [RFC-087 Implementation Status](../rfc/RFC-087-IMPLEMENTATION-STATUS.md)

---

## Approval & Sign-off

**Prepared By**: Claude Code
**Review Status**: Draft
**Target Audience**: Development team, product management, leadership

**Next Review**: After RFC-088 completion (Week 3)
**Document Owner**: Development Lead

---

**Last Updated**: 2025-11-15 13:33 PST
**Version**: 1.0
**Status**: Active Tracking
