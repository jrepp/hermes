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

Phase 0 is execution-ready after the adversarial review. RFC-008 is now accepted with the required event identity, transaction-boundary, ordering, replay, DLQ, and observability contracts. Phase 1 must not start until the audit matrix below is converted into tracked implementation issues or checklist rows with owners.

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

## Phase 0 audit matrix

Phase 0 inventory found no remaining `FIXME: Data consistency` comments under `internal/`, so the implementation gate is direct search writes rather than FIXME removal. Current write sites to migrate or explicitly exempt:

| Endpoint / flow | Handler file | Mutation type | Current search call | Desired event type | Transaction boundary | Test coverage target | Owner |
|-----------------|--------------|---------------|---------------------|--------------------|----------------------|----------------------|-------|
| Draft create | `internal/api/v2/drafts.go:415` | draft create | `DraftIndex().Index` in post-response goroutine | `draft.created` | Same transaction as draft DB create; workspace creation may precede DB transaction | Relay-stopped draft create converges from DB to drafts index | T1 |
| Draft patch | `internal/api/v2/drafts.go:1493` | draft update | `DraftIndex().Index` in post-response goroutine | `draft.updated` | Same transaction as draft DB update | Duplicate draft update replay is idempotent | T1 |
| Draft delete | `internal/api/v2/drafts.go:1008` | draft delete | `DraftIndex().Delete` inline after workspace delete | `draft.deleted` | Same transaction as draft DB/tombstone delete; record resulting DB truth | Delete replay leaves no draft object | T1 |
| Publish / review creation | `internal/api/v2/reviews.go:665` and `:677` | publish transition | `DocumentIndex().Index`, `DraftIndex().Delete` in post-response goroutine | `document.published` plus `draft.published` | Same transaction as document/review state transition | Create -> update -> publish with relay stopped converges to documents index and removes draft | T1 |
| Review-state read/compare path | `internal/api/v2/reviews.go:742` | post-write consistency read | `DocumentIndex().GetObject` for comparison | remove or move to relay/test assertion | Not a durable mutation | Covered by DB-to-search convergence test, not handler readback | T1 |
| Document patch | `internal/api/v2/documents.go:803` | document update | `DocumentIndex().Index` in post-response goroutine | `document.updated` | Same transaction as document DB patch | Patch while relay stopped converges from DB truth | T1 |
| Approval/review state change | `internal/api/v2/approvals.go:675` | review-state change | `DocumentIndex().Index` via `indexAndValidateDocument` | `review_state.changed` | Same transaction as review DB update | Review-state change replay is idempotent | T1 |
| Project create | `internal/api/v2/projects.go:297` / `:635` | project create | `ProjectIndex().Index` via `saveProjectInAlgolia` | `project.created` | Same transaction as project DB create | Project search projection converges after relay restart | T1 |
| Project patch | `internal/api/v2/projects.go:552` / `:635` | project update | `ProjectIndex().Index` via `saveProjectInAlgolia` | `project.updated` | Same transaction as project DB update | Duplicate project update replay is idempotent | T1 |

Allowed search reads or read-path usage: `internal/api/v2/search.go`, `GetObject` calls used to serve read-only related-resource or legacy compatibility paths, and tests. These are not mutation dual writes but should be revisited separately when database-backed reads replace search-backed compatibility behavior.

## Phases & exit criteria

### Phase 0 — Audit & finalize RFC-008

- Convert the audit matrix above into tracked implementation checklist rows with owners.
- Use RFC-008's accepted decision: introduce a dedicated `search_outbox_events` table for search projection events rather than extending `document_revision_outbox`.
- Confirm operation-specific idempotency keys for create, update, publish, delete, draft update, review-state, project, backfill, and reindex events.
- Confirm per-aggregate sequence allocation and same-aggregate blocking behavior for failed/DLQ events.
- Confirm the DB transaction owner for every row in the audit matrix.

**Exit when:**

- RFC-008 remains `Accepted` with event shape, idempotency, ordering, retry, DLQ, replay, and observability semantics.
- Audit matrix lists every direct API-layer search write with mutation type, desired event type, transaction boundary, coverage target, and owner.
- CI guard design is documented: fail direct API-layer `SearchProvider.*Index().Index/Delete` writes while allowing relay, backfill/reindex tooling, read-only search endpoints, and tests.

### Phase 1 — Schema, relay, and write-path integration (v2 first)

- Migration adds `search_outbox_events` via `cmd/hermes-migrate` using ADR-020 core+deltas files (no AutoMigrate, no SQLite driver in `cmd/hermes`).
- Outbox enqueue helper requires a transaction handle and allocates the per-aggregate sequence used in RFC-008 idempotency keys.
- Relay loop (reused or new) drains outbox rows into the existing `search.Provider` index `Index/Delete` calls with retry, backoff, visibility-timeout recovery, DLQ rows, and replay modes.
- Load/restart smoke test is added here, not deferred, to catch relay infrastructure regressions early.

**Exit when:**

- Integration test (testcontainers + Meilisearch) demonstrates: stop the relay, perform create -> update -> publish -> review-state change -> delete, restart relay, and verify final search contents exactly match database truth.
- Duplicate/reorder test inserts repeated events and verifies idempotent convergence.
- Poison-message test proves one bad aggregate does not block unrelated aggregates and documents same-aggregate blocking.
- All v2 mutation handlers compile with no direct `SearchProvider.*Index().Index/Delete` references outside allowed packages (enforced by CI).

### Phase 2 — v1 handler migration & FIXME removal

- Migrate the audit matrix write sites: drafts, documents, reviews/publish, approvals/review state, and projects.
- Audit any v1 or compatibility helpers that hide a `search.Provider` write and migrate or explicitly exempt each one.
- Legacy direct-write code paths deleted (no flag-gating; v1 and v2 share the outbox when v1 mutation paths exist).

**Exit when:**

- `rg "FIXME: Data consistency" internal/` returns zero results and the CI direct-write guard passes.
- TODO-005 status = `completed`, archived under `docs-internal/archive/`.
- The integration test suite from Phase 1 also runs against v1 endpoints.

### Phase 3 — Operational verification

- Outbox lag metric exposed (Prometheus or `/healthz` JSON) with a documented SLO.
- DLQ inspection, retry, skip, and rebuild-current runbook added under `docs-internal/guides/`.
- Load test: 1,000 mutations/min for 10 minutes with relay restarts every 60 s; index converges within 30 s of relay recovery; zero data loss.

**Exit when:**

- Runbook exists and links from this trajectory and from RFC-008.
- Load test result is recorded as a memo under `docs-internal/memo/`.

## Reviews / gates

Gates are exit-criteria only (per roadmap policy — no recurring cadence). Each phase must pass its exit criteria before the next begins. Promote RFC-008 to `Implemented` when Phase 3 closes.

## Risks

- **Outbox + relay coupling with RFC-014.** If T2 changes the relay contract, this trajectory must re-test. Mitigation: pin the relay interface in Phase 1 and own that interface jointly with T2.
- **Long-tail v1 handlers.** v1 has more dual-write sites than v2; Phase 2 may surface scope creep. Mitigation: timebox the audit in Phase 0.
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
