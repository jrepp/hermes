---
id: trajectory-001-adversarial-review
title: "Adversarial Review — T1 Data Consistency & Outbox"
status: Resolved
created: 2026-04-27
date: 2026-04-27
author: Hermes Team
project_id: hermes
doc_uuid: c2b9923e-a1d3-44fa-8f41-0b381dd73e5b
type: Memo
subtype: Analysis
tags: [trajectory, adversarial-review, v1.0, outbox, data-consistency]
related:
  - ADR-009
  - ADR-017
  - ADR-020
  - RFC-008
---

# Adversarial Review — T1 Data Consistency & Outbox

> This review tried to break [Trajectory T1](trajectory-001-data-consistency-outbox.md) before execution. The blocking decisions are now folded into the source plan and [RFC-008](../rfc/rfc-008-outbox-pattern-document-sync.md); Phase 0 is ready to execute, and Phase 1 remains gated on converting the audit matrix into owned implementation work.

## Run-readiness verdict

Phase 0 can start. Do not start Phase 1 until the source plan's Phase 0 exit criteria are complete: owned audit rows, confirmed transaction owners, CI direct-write guard design, and RFC-008's accepted search-outbox contract still intact.

## Resolution summary

- **Event ownership resolved:** RFC-008 now specifies a dedicated `search_outbox_events` table for search projection events, separate from `document_revision_outbox`, preserving the ADR-020 stateless-indexer boundary.
- **Event identity resolved:** RFC-008 defines operation-specific event types and idempotency keys for document, draft, review-state, project, delete, publish, and backfill/reindex flows.
- **Ordering resolved:** RFC-008 requires per-aggregate `sequence` allocation in the same transaction as the authoritative mutation; same-aggregate events block behind failed/DLQ events, unrelated aggregates continue.
- **Transaction boundary resolved:** the source plan now requires every API-layer search projection enqueue to happen inside the same `*gorm.DB` transaction as the DB mutation.
- **Replay/DLQ resolved:** RFC-008 now defines retry, DLQ rows, retry/rebuild/replay modes, and operator skip semantics.
- **Inventory resolved for Phase 0:** the source plan now includes a concrete audit matrix for current v2 direct write sites in drafts, reviews/publish, documents, approvals, and projects.

## Highest-risk failure modes

1. **Ambiguous outbox ownership could create two competing event models.** Resolved by RFC-008: T1 uses `search_outbox_events` for search projection events; `document_revision_outbox` remains revision/indexer-owned. This is aligned with [ADR-020](../adr/adr-020-dual-database-support-stateless-indexer.md)'s stateless-indexer model.

2. **The idempotency key can be wrong for deletes, review-state changes, and publish transitions.** Resolved by RFC-008's event table and per-operation idempotency keys; content hashes are not the sole identity for deletes or metadata-only events.

3. **The plan assumes every mutation already has a clean transaction boundary.** Partially resolved: the source plan now requires each audit row to name the transaction owner before Phase 1. Implementation still needs to prove helpers require a transaction handle.

4. **Search-index convergence tests can pass while database truth is still under-specified.** Resolved in plan/RFC: Phase 1 tests must compare final search contents against canonical DB rows, consistent with [ADR-017](../adr/adr-017-api-refactoring-and-testing-strategy.md).

5. **The v1 migration is under-estimated.** Partially resolved: the plan now includes current v2 write sites and requires a v1/compatibility helper audit before Phase 2 closes. No current `FIXME: Data consistency` comments remain, so the gate shifted to direct write detection.

6. **DLQ semantics are too vague for a durability trajectory.** Resolved by RFC-008's DLQ and replay contract.

7. **Grep-based CI can be too weak or too strong.** Resolved as a Phase 0 exit criterion: design the guard to fail direct API-layer `SearchProvider.*Index().Index/Delete` writes while allowing relay, backfill/reindex tooling, read-only search endpoints, and tests.

## Missing decisions before execution

- T1 events are search projection events in `search_outbox_events`.
- Idempotency keys are defined per operation in RFC-008.
- Same-aggregate ordering and failed/DLQ blocking are defined in RFC-008.
- DB transaction ownership is now required per audit row before Phase 1.
- Relay retry, DLQ, and replay behavior are defined in RFC-008.
- Outbox lag signals are defined in RFC-008's observability contract.

## Concrete pre-flight checklist

- Source plan now has the audit matrix; Phase 0 must convert it into owned implementation work.
- Phase 1 exit criteria now require relay-stopped create -> update -> publish -> review-state change -> delete convergence against database truth.
- Phase 1 exit criteria now require duplicate/reorder idempotency testing.
- Phase 1 exit criteria now require poison-message testing and same-aggregate blocking documentation.
- Phase 0 exit criteria now require CI direct-write guard design.

## Suggested plan edits

- Done: Phase 0 exit criteria now require operation-specific event identity and ordering semantics.
- Done: RFC-008 acceptance now includes replay/DLQ sections.
- Done: Phase 1 now includes a load/restart smoke test; Phase 3 keeps the full operational load test.
- Done: DLQ counter language was replaced with DLQ inspection, retry, skip, rebuild-current, and replay contract language.

## Go/no-go gate

Start Phase 0 now. Start Phase 1 only when Phase 0 can answer this question for every audit row: after any process crash between request start and search update, which durable `search_outbox_events` row causes the search cache to converge, which DB transaction created it, and how is duplicate replay proven safe?
