---
id: TODO-005
title: Fix Data Consistency Between Search Index and Database
date: 2025-10-09
type: TODO
priority: critical
status: in-progress
progress: 98%
tags: [data-consistency, search, database, fixme, bug]
related:
  - RFC-008
  - TODO-003
---

# Fix Data Consistency Between Search Index and Database

## Description

Direct API-layer writes to both PostgreSQL and the search index (Algolia/Meilisearch) can diverge. Earlier FIXME comments have been removed, but the direct-write consistency risk remains in current v2 mutation handlers. This can lead to:
- Stale search results showing outdated document data
- Missing documents in search that exist in database
- Search showing documents that were deleted from database

## Current Code References

- `internal/api/v2/drafts.go` — migrated draft create/update/delete writes to transactional `search_outbox_events`.
- `internal/api/v2/reviews.go` — migrated publish/review flow, including go-link redirect projection, to transactional `search_outbox_events`.
- `internal/api/v2/documents.go` — migrated document patch writes to transactional `search_outbox_events`.
- `internal/api/v2/approvals.go` — migrated approval/review-state changes to transactional `search_outbox_events`.
- `internal/api/v2/projects.go` — migrated project create/patch writes to transactional `search_outbox_events`.

`rg "FIXME: Data consistency" internal/` currently returns zero results; TODO-005 should now track the direct-write migration described in [Trajectory T1](trajectory-001-data-consistency-outbox.md).

## Root Cause

The code performs direct writes to both database and search index without:
1. Transaction guarantees across both systems
2. Retry logic for failed index updates
3. Reconciliation process for detecting drift
4. Audit trail of index operations

## Proposed Solution

Implement the **search outbox pattern** from accepted RFC-008:

### Phase 1: Write to Outbox
1. Wrap each authoritative database mutation in a transaction.
2. Write a `search_outbox_events` row in the same transaction.
3. Commit both atomically.

### Phase 2: Background Processor
1. Relay reads due `search_outbox_events` rows.
2. Applies operations to search through `search.Provider`.
3. Marks outbox entries as completed.
4. Retries with exponential backoff and moves poison events to inspectable DLQ rows.

### Phase 3: Reconciliation
1. Backfill/rebuild tooling emits `search.backfill` events from database truth.
2. Integration tests compare search cache contents against database rows after relay restart.
3. Operators can retry, skip, or rebuild current projection from DLQ rows.

## Example Implementation

```go
// Instead of direct index update:
err := srv.SearchProvider.Index(document)

// Use outbox:
tx := db.Begin()
tx.Create(&document)
tx.Create(&OutboxEvent{
    Type: "document.index",
    Payload: document,
    Status: "pending",
})
tx.Commit()
```

## Tasks

- [x] Accept RFC-008 with event identity, idempotency, ordering, retry, DLQ, replay, and observability contract
- [x] Add T1 audit matrix for current direct API-layer search writes
- [x] Confirm transaction-owner expectations for every audited direct-write flow
- [x] Add CI guard baseline for new direct API-layer search writes
- [x] Design `search_outbox_events` migration
- [x] Add `SearchOutboxEvent` model and transaction-required enqueue helper
- [x] Add transaction-bound per-aggregate sequence allocation helper
- [x] Create background relay for processing search outbox events
- [x] Add retry logic with exponential backoff
- [x] Wire search outbox relay into server lifecycle
- [x] Migrate project create/update direct search writes to transactional outbox
- [x] Migrate document patch direct search write to transactional outbox
- [x] Migrate approval/review-state direct search write to transactional outbox
- [x] Migrate publish/review direct search writes to transactional outbox
- [x] Migrate draft create/update/delete direct search writes to transactional outbox
- [x] Implement transactional outbox writes in all audited v2 document/draft/review/project operations
- [x] Implement DLQ inspection/retry/skip tooling
- [x] Implement rebuild-current tooling
- [x] Expose health metrics for lag, failures, and DLQ count
- [x] Add expanded testcontainers + Meilisearch relay convergence test for document, draft, review-state, and go-link projection events
- [x] Add duplicate/reorder and poison-message relay tests for ordering and unrelated-aggregate progress
- [x] Audit v1/compatibility API paths for hidden direct search mutation writes
- [x] Add health-based alert thresholds for lag, failures, and DLQ age
- [x] Move sustained stress/restart harness ownership to T8 NFR plan
- [x] Add CI guard for direct API-layer search writes

## Impact

**Files Affected**: v2 drafts, documents, reviews, approvals, projects, outbox model/migrations, relay package, tests, and operations docs
**Complexity**: High  
**Risk**: Critical - data inconsistency affects search reliability

## Testing Strategy

1. **Unit Tests**: Mock outbox writes and worker processing
2. **Integration Tests**: Simulate DB success + index failure
3. **Chaos Tests**: Kill processes mid-operation, verify recovery
4. **Monitoring**: Track outbox queue depth and processing lag

## Related Work

- **RFC-008**: Outbox Pattern for Document Synchronization (accepted design)
- **Trajectory T1**: Data Consistency & Outbox (execution plan and audit matrix)
- **Trajectory T8**: Reliability & Performance NFR Harness (stress/restart evidence)
- **TODO-003**: Migrate handlers to SearchProvider (prerequisite)

## References

- [Trajectory T1 — Data Consistency & Outbox](trajectory-001-data-consistency-outbox.md)
- [RFC-008 — Outbox Pattern for Document Synchronization](../rfc/rfc-008-outbox-pattern-document-sync.md)
