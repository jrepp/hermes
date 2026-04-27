---
id: trajectory-006
title: "Trajectory T6 — E2E Coverage & Tech Debt"
status: Draft
created: 2026-04-27
date: 2026-04-27
author: Hermes Team
project_id: hermes
doc_uuid: 4cd38991-6f07-4059-a871-f03c4de81548
type: Memo
subtype: Milestone
tags: [trajectory, roadmap, v1.0, e2e, playwright, tech-debt, migrations, testing]
related:
  - ADR-010
  - ADR-019
  - ADR-020
  - MEMO-032
  - MEMO-063
---

# Trajectory T6 — E2E Coverage & Tech Debt

> Closes v1.0 criteria #6 (no AutoMigrate at server start), #7 (Playwright dashboards green), and #8 (no critical/high TODOs). Also burns down the small but blocking tech-debt items so v1.0 ships clean.

## Stable-release criteria served

- v1.0 criterion **#6 Migrations** (AutoMigrate gone).
- v1.0 criterion **#7 E2E coverage** (TODO-011/012/013 green in CI).
- v1.0 criterion **#8 No critical or high TODOs open.**
- v1.0 criterion **#9 Docs-validate green.**

## Scope

In scope:

- TODO-011, TODO-012, TODO-013 — Playwright suites for *Awaiting Review*, *Latest Docs*, *Recently Viewed*.
- TODO-002 — push API test coverage past 80 % across v1 + v2.
- `database_migration_fix_session.md` — proper fix for GORM AutoMigrate constraint-rename bug; AutoMigrate code path **deleted**, not flag-disabled.
- `indexer-refactor.md` — finalize and archive once T2 cutover lands.
- TODO-001 — compile-time interface assertions.
- Docs-validate cleanup: lowercase legacy ADR/RFC/MEMO filenames, fix `docs-project.yaml` path resolution so validation actually scans files.

Out of scope (v1.x):

- TODO-007 TS type safety.
- TODO-008 hardcoded values.
- TODO-009 template system (depends on RFC-006).

## Dependencies

- T1, T2, T3 must be far enough along that the dashboards have stable data.
- T5 Phase 1 (local mode) gives Playwright a deterministic backend.

## Phases & exit criteria

### Phase 0 — Migrations cleanup

- Implement the proper fix for the AutoMigrate constraint-rename issue (or, if no fix is feasible, delete AutoMigrate entirely and rely on `cmd/hermes-migrate`).
- Delete the AutoMigrate code path from server start.
- All schema changes flow through delta migrations (ADR-020).

**Exit when:**

- `rg "AutoMigrate" internal/ pkg/ cmd/` returns hits only in tests or `cmd/hermes-migrate`.
- `database_migration_fix_session.md` archived under `docs-internal/archive/`.

### Phase 1 — Docs-validate green

- Fix `docs-project.yaml` path resolution.
- Rename legacy uppercase ADR/RFC/MEMO files to lowercase kebab-case (per templates guide).
- Fix all inbound `related:` references touched by the rename.

**Exit when:**

- `./scripts/docs-validate.sh` exits 0 with non-zero file count and no warnings.

### Phase 2 — Playwright dashboards (TODO-011/012/013)

- Implement each suite using `data-test-*` selectors (ADR-010), headless only, `--reporter=line --max-failures=1`.
- Wire into CI required-checks.

**Exit when:**

- All three suites green in CI on three consecutive PRs.
- TODO-011/012/013 archived.

### Phase 3 — API coverage burn-down (TODO-002)

- Bring v1 + v2 API integration coverage to ≥80 %, including authn/authz and error paths.

**Exit when:**

- Coverage gate enforced in CI.
- TODO-002 archived.

### Phase 4 — Tech-debt sweep

- TODO-001 compile-time assertions added.
- `indexer-refactor.md` reviewed; if T2 cutover is done, archive it.
- Sweep for any remaining critical or high TODOs and either close or downgrade them with a documented reason.

**Exit when:**

- `docs-internal/plans/` contains zero open `priority: critical` and zero open `priority: high` items.

## Reviews / gates

Phase exit-only. Phase 1 may run in parallel with Phase 0 (independent files).

## Risks

- **Flaky Playwright suites** undermine the gate. Mitigation: enforce `--max-failures=1` and run each new suite on three PRs before marking required.
- **AutoMigrate deletion** uncovers latent schema drift in dev environments. Mitigation: ship a one-time `hermes-migrate doctor` command in Phase 0.

## References

- [TODO-001](todo-001-abstraction-interface-compile-checks.md), [TODO-002](todo-002-comprehensive-api-test-suite.md), [TODO-011](todo-011-e2e-test-awaiting-review-dashboard.md), [TODO-012](todo-012-e2e-test-latest-docs-dashboard.md), [TODO-013](todo-013-e2e-test-recently-viewed-sidebar.md)
- [`database_migration_fix_session.md`](database_migration_fix_session.md)
- [`indexer-refactor.md`](indexer-refactor.md)
- [ADR-010: Testing Strategy](../adr/) · [ADR-019: Split Server/Migrate Binaries](../adr/adr-019-split-server-and-migrate-binaries.md) · [ADR-020](../adr/adr-020-dual-database-support-stateless-indexer.md)
- [MEMO-032: E2E Testing Summary](../memo/memo-032-e2e-testing-summary.md) · [MEMO-063: SQLite Driver Conflict Investigation](../memo/memo-063-sqlite-driver-conflict-investigation.md)
- [Roadmap Implementation Tracker](roadmap-tracker.md)
