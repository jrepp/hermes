# Hermes Roadmap to v1.0 Stable

> Public, durable view of the work required to bring Hermes to a stable v1.0 release. The detailed implementation tracker lives at [`docs-internal/plans/roadmap-tracker.md`](docs-internal/plans/roadmap-tracker.md). Per-trajectory plans live under [`docs-internal/plans/`](docs-internal/plans/).

This file is intentionally short. It states **what v1.0 means**, **the trajectories that close the gap**, and **where to look for detail**. It is updated whenever a trajectory moves between phases or its exit criteria change.

## Status snapshot

- **All 21 ADRs are Accepted.** No pending architectural decisions block v1.0.
- **Critical gap blocking v1.0:** data consistency between database and search index (TODO-005, RFC-008) is now in implementation hardening, with sustained stress evidence owned by T8.
- **Largest in-flight effort:** event-driven indexer cutover (RFC-014) and S3 storage / migration system (RFC-015).
- **Largest greenfield effort planned for v1.x (post-1.0):** multi-provider federation (RFC-010 → RFC-011 → RFC-012) and admin interface (RFC-016).

## What "v1.0 Stable" means

v1.0 ships when **every** criterion below is met. These are intentionally outcome-shaped, not task-shaped, and each maps to a trajectory below.

1. **Durability** — Every document write that mutates the search index is covered by the outbox pattern (RFC-008). No more `FIXME: Data consistency check` comments in `internal/api/drafts.go`.
2. **Async correctness** — No request-path goroutine sends email; all notifications flow through the RFC-013 backend with retries + DLQ.
3. **Indexer cutover** — The event-driven indexer (RFC-014) is the only indexer running in production. The legacy indexer is deleted, not just disabled.
4. **Storage** — At least one non-Google workspace provider (local filesystem **or** S3) is supported end-to-end through the v2 API, with migration jobs exercised in CI.
5. **Provider abstraction complete** — All v2 handlers go through `search.Provider` and `workspace.Provider`. No direct Algolia or Google client calls remain in `internal/api/v2/`.
6. **Migrations** — `cmd/hermes-migrate` is the only path that mutates schema; GORM AutoMigrate is removed from server start, not just disabled.
7. **E2E coverage** — Playwright suites for "Awaiting Review", "Latest Docs", and "Recently Viewed" pass in CI on every PR (TODO-011/012/013).
8. **No critical or high-priority TODOs open.** Medium/low TODOs may roll into v1.x.
9. **Docs-validate green.** `./scripts/docs-validate.sh` exits 0 with no filename or frontmatter violations.

Anything not on that list is **explicitly v1.x or later**: multi-provider federation, admin UI, additional notification backends, semantic-search/LLM enhancements beyond what RFC-014 already requires, and template-system expansion.

## Completion trajectories

Each trajectory is a self-contained body of work with phases, exit criteria, and review gates. Status is summarised here; the source of truth is the linked plan. T8 is a cross-cutting NFR evidence trajectory: it supports v1.0 confidence but does not replace feature correctness gates in T1-T6.

| ID | Trajectory | v1.0? | Status | Plan |
|---|---|---|---|---|
| **T1** | Data Consistency & Outbox | yes | Phase 3 (operational verification; stress evidence moved to T8) | [`plans/trajectory-001-data-consistency-outbox.md`](docs-internal/plans/trajectory-001-data-consistency-outbox.md) |
| **T2** | Event-Driven Indexer Cutover | yes | Phase 2 (LLM + embeddings in flight, ~40%) | [`plans/trajectory-002-indexer-cutover.md`](docs-internal/plans/trajectory-002-indexer-cutover.md) |
| **T3** | Notifications Hardening | yes (subset) | Phase 3 (production-ready, hardening pending) | [`plans/trajectory-003-notifications-hardening.md`](docs-internal/plans/trajectory-003-notifications-hardening.md) |
| **T4** | Storage & Migration Surface | yes (Phase 1–2) | Phase 1 done, API surface pending | [`plans/trajectory-004-storage-migration.md`](docs-internal/plans/trajectory-004-storage-migration.md) |
| **T5** | Local Mode & Workspace Abstraction | yes (Phase 1) | Phase 2 (in-browser editor pending) | [`plans/trajectory-005-local-mode.md`](docs-internal/plans/trajectory-005-local-mode.md) |
| **T6** | E2E Coverage & Tech Debt | yes | Phase 0 (test scaffolding only) | [`plans/trajectory-006-e2e-and-tech-debt.md`](docs-internal/plans/trajectory-006-e2e-and-tech-debt.md) |
| **T7** | Multi-Provider Federation & Admin UI | **no — v1.x** | Design only | [`plans/trajectory-007-federation-and-admin.md`](docs-internal/plans/trajectory-007-federation-and-admin.md) |
| **T8** | Reliability & Performance NFR Harness | supports v1.0 evidence | Phase 0 (harness design) | [`plans/trajectory-008-nfr-reliability-performance.md`](docs-internal/plans/trajectory-008-nfr-reliability-performance.md) |

## How to contribute

- **To pick up work:** open the trajectory plan, find the first phase whose exit criteria are unmet, and claim a task in its checklist.
- **To propose new architecture:** write an RFC under `docs-internal/rfc/`. Do not modify a trajectory's exit criteria without an accompanying RFC update.
- **To record an architectural decision:** write an ADR under `docs-internal/adr/` and update [`docs-internal/adr/adr-002-readme.md`](docs-internal/adr/adr-002-readme.md).
- **All doc work** must follow [`docs-internal/templates/readme.md`](docs-internal/templates/readme.md) and pass `./scripts/docs-validate.sh`.

## Source of truth

- Architecture rules: [`docs-internal/adr/`](docs-internal/adr/) (binding).
- Active proposals: [`docs-internal/rfc/`](docs-internal/rfc/).
- Detailed implementation tracker (status, decision log, risk): [`docs-internal/plans/roadmap-tracker.md`](docs-internal/plans/roadmap-tracker.md).
- Per-trajectory phase plans: [`docs-internal/plans/`](docs-internal/plans/).
