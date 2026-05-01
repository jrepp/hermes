---
id: trajectory-001
title: "Trajectory T1 — Data Consistency & Outbox"
status: Draft
created: 2026-04-27
date: 2026-04-27
author: Hermes Team
project_id: hermes
doc_uuid: 4cc824b5-51ad-4db1-bc75-309a2ed120fa
type: Memo
subtype: Milestone
tags: [trajectory, roadmap, v1.0, outbox, data-consistency, search]
related:
  - ADR-009
  - ADR-017
  - ADR-020
  - RFC-005
  - RFC-008
---

# Trajectory T1 — Data Consistency & Outbox

> Closes the **critical** gap blocking v1.0: dual writes between PostgreSQL and the search index can diverge. Implements the transactional search outbox across document, draft, review-state, publish/delete, and project mutation paths so the database remains the source of truth and the search index is a recoverable cache.

## Readiness status

Phase 0 is complete. RFC-008 is accepted with the required event identity, transaction-boundary, ordering, replay, DLQ, and observability contracts; the audit matrix has owned checklist rows; and `scripts/check-search-direct-writes.sh` is wired into CI with a T1 baseline so new direct search writes fail while the audited legacy sites are migrated.

## Adversarial review disposition

The T1 adversarial review is resolved and folded into this source plan plus [RFC-008](../rfc/rfc-008-outbox-pattern-document-sync.md). The controls to preserve during implementation are:

- T1 owns search projection events in `search_outbox_events`; `document_revision_outbox` remains revision/indexer-owned per [ADR-020](../adr/adr-020-dual-database-support-stateless-indexer.md).
- Event identity, idempotency keys, same-aggregate ordering, retry, replay, DLQ, and observability semantics are defined by RFC-008.
- Every audited API-layer search projection enqueue must happen inside the same `*gorm.DB` transaction as the authoritative database mutation.
- Phase 1 tests must compare final search contents against canonical database rows, not just assert that search writes happened.
- Direct API-layer `SearchProvider.*Index().Index/Delete` calls must be blocked by a CI guard, with explicit exemptions for relay, backfill/reindex tooling, read-only search endpoints, canary tooling, and tests.

## Stable-release criteria served

- v1.0 criterion **#1 Durability**.
- Unblocks v1.0 criterion **#2 Async correctness** (T3).
- Unblocks completion of v1.0 criterion **#5 Provider abstraction** (TODO-003).

## Scope

In scope:

- Outbox table(s) and relay covering document create/update/delete/publish, draft mutations, and review-state changes.
- Replacement of all direct API-layer `SearchProvider.*Index().Index/Delete` calls with outbox writes inside the same DB transaction as the authoritative mutation.
- Removal or explicit exemption of wrapper methods that hide direct search writes, such as project indexing helpers.
- Reuse of the relay infrastructure shipped for RFC-014 (`pkg/indexer/relay/`) where possible; otherwise a parallel relay sized for search-index events.

Out of scope:

- Multi-region replication.
- LLM/embedding pipeline (covered by T2).
- Notification delivery (covered by T3, but T3 depends on the outbox primitives this trajectory builds).

## Dependencies

- ADR-017 (typed errors, `server.Server` DI) — done.
- ADR-020 (core+deltas migrations, stateless indexer boundary) — binding for schema and indexer interaction.
- RFC-014 outbox + relay implementation — done; reuse the table pattern and relay loop.
- RFC-008 accepted search-outbox contract — done.
- TODO-003 progress on `search.Provider` abstraction — remaining write handlers will be migrated as part of Phase 2 here.

## Phase 0 implementation checklist

Phase 0 inventory found no remaining `FIXME: Data consistency` comments under `internal/`, so the implementation gate is direct search writes rather than FIXME removal. Each row is owned by T1 and the audited v2 mutation direct writes are now migrated to transactional outbox events:

| Endpoint / flow | Handler file | Mutation type | Current search call | Desired event type | Transaction boundary | Test coverage target | Owner |
|-----------------|--------------|---------------|---------------------|--------------------|----------------------|----------------------|-------|
| Draft create | `internal/api/v2/drafts.go` | draft create | migrated to `search_outbox_events` | `draft.created` | Same transaction as draft DB create; workspace creation may precede DB transaction | Relay-stopped draft create converges from DB to drafts index | T1 |
| Draft patch | `internal/api/v2/drafts.go` | draft update | migrated to `search_outbox_events` | `draft.updated` | Same transaction as draft DB update | Duplicate draft update replay is idempotent | T1 |
| Draft delete | `internal/api/v2/drafts.go` | draft delete | migrated to `search_outbox_events` | `draft.deleted` | Same transaction as draft DB/tombstone delete; record resulting DB truth | Delete replay leaves no draft object | T1 |
| Publish / review creation | `internal/api/v2/reviews.go` | publish transition | migrated to `search_outbox_events` | `document.published`, `draft.published`, plus `link.created` | Same transaction as document/review state transition | Create -> update -> publish with relay stopped converges to documents index, removes draft, and creates go-link redirect | T1 |
| Review-state read/compare path | `internal/api/v2/reviews.go` | post-write consistency read | removed from handler post-processing | remove or move to relay/test assertion | Not a durable mutation | Covered by DB-to-search convergence test, not handler readback | T1 |
| Document patch | `internal/api/v2/documents.go` | document update | migrated to `search_outbox_events` | `document.updated` | Same transaction as document DB patch | Patch while relay stopped converges from DB truth | T1 |
| Approval/review state change | `internal/api/v2/approvals.go` | review-state change | migrated to `search_outbox_events` | `review_state.changed` | Same transaction as review DB update | Review-state change replay is idempotent | T1 |
| Project create | `internal/api/v2/projects.go` | project create | migrated to `search_outbox_events` | `project.created` | Same transaction as project DB create | Project search projection converges after relay restart | T1 |
| Project patch | `internal/api/v2/projects.go` | project update | migrated to `search_outbox_events` | `project.updated` | Same transaction as project DB update | Duplicate project update replay is idempotent | T1 |

Allowed search reads or read-path usage: `internal/api/v2/search.go`, `GetObject` calls used to serve read-only related-resource or legacy compatibility paths, and tests. These are not mutation dual writes but should be revisited separately when database-backed reads replace search-backed compatibility behavior.

### Transaction owners

- Draft create: `createDraftWithSearchOutbox` wraps document create plus `draft.created` enqueue in one transaction after workspace document creation returns the provider ID.
- Draft patch: `updateDraftWithSearchOutbox` wraps document update plus `draft.updated` enqueue in one transaction after workspace-side sharing/header/title changes succeed.
- Draft delete: workspace delete remains outside the DB transaction; `deleteDraftWithSearchOutbox` wraps database delete plus `draft.deleted` enqueue in one transaction, with database truth defining the projection.
- Publish / review creation: `reviews.go` now enqueues `document.published`, `draft.published`, and go-link `link.created` in the existing review transaction before commit; post-response direct indexing and search readback comparison are removed.
- Document patch: the document update path currently performs DB mutation before post-response indexing; Phase 1 must wrap the DB update and `document.updated` enqueue in one transaction.
- Approval/review-state change: `updateReviewStateWithSearchOutbox` now wraps review-row changes, file-revision creation, optional group-approver DB updates, and `review_state.changed` enqueue in one transaction; `indexAndValidateDocument` is removed from the request path.
- Project create: `models.Project.Create` currently owns the DB write; Phase 1 must wrap project create and `project.created` enqueue in one transaction.
- Project patch: `models.Project.Update` currently owns the DB write; Phase 1 must wrap project update and `project.updated` enqueue in one transaction.

### Migrated write paths

- Project create/update: `internal/api/v2/projects.go` now uses `createProjectWithSearchOutbox` and `updateProjectWithSearchOutbox`; `saveProjectInAlgolia` and direct `ProjectIndex().Index` writes are removed from the API handler baseline.
- Document patch: `internal/api/v2/documents.go` now uses `upsertDocumentWithSearchOutbox`; the post-response `DocumentIndex().Index` goroutine is removed from the API handler baseline.
- Approval/review-state changes: `internal/api/v2/approvals.go` now uses `updateReviewStateWithSearchOutbox`; the direct `DocumentIndex().Index` helper and handler readback comparison are removed from the API handler baseline.
- Publish / review creation: `internal/api/v2/reviews.go` now uses `enqueueReviewCreatedSearchOutbox`; the direct document upsert, draft delete, go-link redirect write, and handler readback comparison are removed from the API handler baseline.
- Draft create/update/delete: `internal/api/v2/drafts.go` now uses `createDraftWithSearchOutbox`, `updateDraftWithSearchOutbox`, and `deleteDraftWithSearchOutbox`; direct draft index writes are removed from the API handler baseline.

### CI direct-write guard

`scripts/check-search-direct-writes.sh` is the Phase 0 guard. It is wired into `.github/workflows/parallel-ci.yml` as the `search-direct-writes` lint category.

- The guard scans API Go files for `SearchProvider.*Index().Index/Delete`, equivalent direct `provider.*Index().Index/Delete` calls, and the known wrapper helpers `saveProjectInAlgolia` and `indexAndValidateDocument`.
- It permits tests, `pkg/search/outbox/`, `pkg/indexer/relay/`, canary tooling, and future backfill/reindex commands.
- The audited v2 mutation direct-write baselines have been reduced to zero. Remaining search usage in v2 API handlers is read-path search, tests, or explicitly exempted tooling.

## Phases & exit criteria

### Phase 0 — Audit & finalize RFC-008

- Completed: converted the audit matrix above into owned implementation checklist rows.
- Completed: RFC-008's accepted decision uses a dedicated `search_outbox_events` table for search projection events rather than extending `document_revision_outbox`.
- Completed: RFC-008 confirms operation-specific idempotency keys for create, update, publish, delete, draft update, review-state, project, backfill, and reindex events.
- Completed: RFC-008 confirms per-aggregate sequence allocation and same-aggregate blocking behavior for failed/DLQ events.
- Completed: transaction-owner expectations are listed above for every row in the audit matrix.

**Exit when:**

- Done: RFC-008 remains `Accepted` with event shape, idempotency, ordering, retry, DLQ, replay, and observability semantics.
- Done: audit matrix lists every direct API-layer search write with mutation type, desired event type, transaction boundary, coverage target, and owner.
- Done: CI guard is implemented and documented: fail new direct API-layer `SearchProvider.*Index().Index/Delete` writes while allowing relay, backfill/reindex tooling, canary, read-only search endpoints, and tests.

### Phase 1 — Schema, relay, and write-path integration (v2 first)

- Done: migration adds `search_outbox_events` via `cmd/hermes-migrate` migrations (no SQLite driver in `cmd/hermes`). The current migration tree is still pre-core-suffix, so this uses the existing runner-compatible `000014_add_search_outbox_events.*.sql` layout while preserving the ADR-020 table boundary.
- Done: `pkg/models.SearchOutboxEvent`, `SearchOutboxSequence`, and enqueue helpers require a transaction handle, validate RFC-008 event basics, and allocate per-aggregate sequence numbers inside the caller's transaction.
- Done: `pkg/search/outbox.Relay` claims due events, applies `Index/Delete` through `search.Provider`, recovers stale `processing` rows, retries with backoff, moves exhausted rows to DLQ, and blocks later same-aggregate events behind pending/processing/failed/DLQ predecessors.
- Done: search outbox relay is wired into the server lifecycle when both DB and `search.Provider` are present; it uses the command context for shutdown.
- Done: operator controls can list failed/DLQ events, retry failed/DLQ/skipped events, and skip failed/DLQ poison events with an operator note.
- Expanded Meilisearch convergence coverage surfaced and fixed a links adapter primary-key mismatch: links now preserve public path-like `objectID` values and use an internal Meilisearch-safe `linkID` for lookup/delete.
- Load/restart smoke test is added here, not deferred, to catch relay infrastructure regressions early.

**Exit when:**

- Done for relay-level multi-event convergence: `tests/integration/search/outbox_relay_test.go` uses testcontainers + Meilisearch to prove stopped-then-processed events converge document upsert, draft deletion, review-state document update, and go-link redirect projection through `search.Provider`.
- Done: duplicate/reorder relay tests insert out-of-created-order same-aggregate sequence events and repeated final-state payloads, then verify the final projection converges in sequence order.
- Done: poison-message relay tests prove one malformed aggregate blocks only later same-aggregate events while unrelated aggregates continue to completion.
- Done: all audited v2 mutation handlers compile with no direct `SearchProvider.*Index().Index/Delete` references outside allowed packages (enforced by CI).

### Phase 2 — v1 handler migration & FIXME removal

- Done for audited v2 handlers: drafts, documents, reviews/publish, approvals/review state, and projects now enqueue transactional search outbox events.
- Done: v1/compatibility audit found no hidden `internal/api` direct mutation writes; remaining API search usage is read-only, and canary writes stay explicitly exempted operational probes.
- Done: legacy direct-write code paths were deleted from audited API mutation paths (no flag-gating); v1-compatible surfaces now share the v2 outbox-backed mutation paths where they exist.

**Exit when:**

- `rg "FIXME: Data consistency" internal/` returns zero results and the CI direct-write guard passes.
- TODO-005 status = `completed`, archived under `docs-internal/archive/`.
- The integration test suite from Phase 1 also runs against v1 endpoints.

### Phase 3 — Operational verification

- Done: outbox queue depth and lag are exposed in `/health` JSON when the database is available; [Search Outbox Operations](../guides/search/outbox-operations.md) documents operator inspection.
- Done: `hermes operator search-outbox` supports DLQ inspection, retry, skip, and rebuild-current; [Search Outbox Operations](../guides/search/outbox-operations.md) documents each operation.
- Load test: 1,000 mutations/min for 10 minutes with relay restarts every 60 s; index converges within 30 s of relay recovery; zero data loss.

**Exit when:**

- Runbook exists and links from this trajectory and from RFC-008.
- Load test result is recorded as a memo under `docs-internal/memo/`.

## Reviews / gates

Gates are exit-criteria only (per roadmap policy — no recurring cadence). Each phase must pass its exit criteria before the next begins. Promote RFC-008 to `Implemented` when Phase 3 closes.

## Risks

- **Outbox + relay coupling with RFC-014.** If T2 changes the relay contract, this trajectory must re-test. Mitigation: pin the relay interface in Phase 1 and own that interface jointly with T2.
- **Long-tail v1 handlers.** Phase 2 audit found no hidden `internal/api` direct search mutation writes, but future compatibility paths can regress. Mitigation: keep the CI guard scoped to API-layer mutation writes and require explicit exemptions for operational probes only.
- **Same-aggregate DLQ blocking.** A poison event can block later updates for the same document or project. Mitigation: expose DLQ inspection, retry, and explicit skip with operator note; unrelated aggregates continue processing.
- **Workspace + DB transaction gaps.** Workspace provider calls cannot participate in the DB transaction. Mitigation: database truth wins; enqueue the search projection event inside the DB transaction after the resulting DB state is known.

## References

- [RFC-008: Outbox Pattern for Document Synchronization](../rfc/rfc-008-outbox-pattern-document-sync.md) (Accepted)
- [TODO-005: Fix Data Consistency](todo-005-data-consistency-search-database.md)
- [TODO-003: Migrate Handlers to Search Provider](todo-003-migrate-handlers-to-search-provider.md)
- [ADR-009: Provider Abstraction Architecture](../adr/adr-009-provider-abstraction-architecture.md)
- [ADR-017: API Refactoring and Testing Strategy](../adr/adr-017-api-refactoring-and-testing-strategy.md)
- [ADR-020: Core+Deltas Migrations and Stateless Indexer](../adr/adr-020-dual-database-support-stateless-indexer.md)
- [Roadmap Implementation Tracker](roadmap-tracker.md)
