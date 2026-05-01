---
id: trajectory-008
title: "Trajectory T8 — Reliability & Performance NFR Harness"
status: Draft
created: 2026-04-30
date: 2026-04-30
author: Hermes Team
project_id: hermes
doc_uuid: 1b0d9420-62a5-410e-8a1e-e7f3845ff43a
type: Memo
subtype: Milestone
tags: [trajectory, roadmap, nfr, reliability, performance, load-test]
related:
  - ADR-010
  - ADR-017
  - ADR-020
  - RFC-008
---

# Trajectory T8 — Reliability & Performance NFR Harness

> Owns Hermes non-functional reliability and performance validation so product trajectories do not each invent one-off stress tests. The first slice is the search-outbox stress/restart harness deferred from T1; the second planned slice is indexer throughput evidence deferred from T2.

## Stable-release criteria served

- v1.0 criterion **#1 Durability**: prove async projection systems converge after worker disruption.
- v1.0 criterion **#2 Async correctness**: prove background relays remain observable and recoverable under load.
- v1.0 release confidence: provide repeatable NFR evidence without coupling every feature trajectory to bespoke load tooling.

## Scope

In scope:

- A reusable stress harness for Hermes background workers and async projection paths.
- Load, restart, and convergence scenarios for search outbox as the first implementation slice.
- Indexer throughput and consumer-lag scenarios for the event-driven indexer cutover.
- A standard result memo format under `docs-internal/memo/` for measured NFR runs.
- Agent-friendly local and CI commands that avoid interactive/headed test modes per ADR-010.

Out of scope:

- Feature-level correctness tests already owned by T1-T6.
- Production capacity planning or SLO commitments beyond documented test thresholds.
- New observability infrastructure such as Prometheus exporters unless a separate ADR/RFC accepts that direction.

## Phase 0 — Harness Design

- Define the NFR harness entry point and command shape.
- Define scenario configuration: mutation rate, duration, relay restart cadence, convergence deadline, backend profile, and output path.
- Define result memo schema: environment, command, dataset shape, observed throughput, restart count, convergence time, failures, and follow-up actions.
- Confirm the harness can run with existing integration dependencies: PostgreSQL, Meilisearch, and testcontainers where feasible.

**Exit when:**

- Harness design is documented in this trajectory or a linked guide.
- Search-outbox scenario inputs and pass/fail thresholds are explicit.
- CI policy is explicit: whether the stress harness is required, optional, nightly, or manual-only for v1.0.

## Phase 1 — Search-Outbox Stress/Restart Scenario

- Generate at least 1,000 search-outbox-backed mutations/minute for 10 minutes against PostgreSQL + Meilisearch.
- Restart or pause/resume the relay every 60 seconds during the run.
- Verify final search contents converge to database truth within 30 seconds after relay recovery.
- Verify zero data loss, no stuck `processing` rows after visibility timeout, and no unexpected DLQ rows.
- Capture `/health` outbox fields during the run: queue depth, pending age, failed age, and DLQ age.

**Exit when:**

- Scenario can be run by a documented command without interactive steps.
- The result is recorded as a memo under `docs-internal/memo/`.
- Any failure creates a follow-up plan or updates this trajectory with a blocked status and owner.

## Phase 2 — Event-Driven Indexer Throughput Scenario

- Generate at least 1,000 documents/hour through the event-driven indexer pipeline.
- Track Redpanda consumer lag and keep peak lag below 100 messages during the run.
- Verify the `search_index` projection converges through `search.Provider` without worker database credentials.
- Record environment, corpus shape, throughput, peak lag, convergence lag, failures, and rollback notes in a result memo.

**Exit when:**

- Scenario can be run by a documented command without interactive steps.
- The result is linked from T2 Phase 2, or T2 records an explicit owner-approved deferral.

## Phase 3 — Generalize NFR Coverage

- Add additional scenarios only after the search-outbox scenario has stabilized.
- Candidate scenarios: notification outbox delivery, indexer API submission, dashboard query latency, and local workspace filesystem churn.
- Keep each scenario independently runnable and independently reportable.

**Exit when:**

- At least two non-search-outbox scenarios use the same harness conventions.
- The result memo format is stable enough to move into `docs-internal/templates/` or a guide if it becomes evergreen.

## Reviews / gates

This trajectory owns NFR evidence, not feature correctness. A feature trajectory may reference a T8 scenario as release evidence, but should not block on broad NFR harness work unless the scenario is explicitly listed in its exit criteria.

## Risks

- **Harness flakiness hides real regressions.** Mitigation: start manual/nightly until variance is understood; keep CI-gating separate from harness creation.
- **Stress tests become too expensive for normal PRs.** Mitigation: make duration/rate configurable and document a smaller smoke profile separately from the release-evidence profile.
- **Database/search testcontainers mask production behavior.** Mitigation: record environment details in every result memo and avoid treating local results as production SLO proof.

## References

- [Trajectory T1 — Data Consistency & Outbox](trajectory-001-data-consistency-outbox.md)
- [Trajectory T2 — Event-Driven Indexer Cutover](trajectory-002-indexer-cutover.md)
- [RFC-008: Outbox Pattern for Document Synchronization](../rfc/rfc-008-outbox-pattern-document-sync.md)
- [ADR-010: Playwright for Local Iteration](../adr/adr-010-playwright-for-local-iteration.md)
- [ADR-017: API Refactoring and Testing Strategy](../adr/adr-017-api-refactoring-and-testing-strategy.md)
- [ADR-020: Core+Deltas Migrations and Stateless Indexer](../adr/adr-020-dual-database-support-stateless-indexer.md)
