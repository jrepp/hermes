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

## Readiness status

Phase 0 is execution-ready after the adversarial review. Start only the independent audits and small compile-time assertion work until dependent trajectories stabilize their data surfaces. Do not delete AutoMigrate, add required CI gates, or run broad docs filename migration until the track-specific pre-flight gates below are satisfied.

T6 is managed as five independent tracks so one cleanup stream cannot block or obscure the others:

| Track | v1.0 blocker? | Scope | Start condition |
|---|---:|---|---|
| Migrations bootstrap | yes | Remove server-start AutoMigrate and prove `cmd/hermes-migrate` bootstrap | Can start after bootstrap command and rollback path are documented. |
| E2E dashboards | yes | TODO-011/012/013 Playwright suites | Wait for stable T1/T2/T3/T5 data setup and auth/local-provider semantics. |
| API coverage | yes | TODO-002 endpoint/risk matrix and coverage gate | Can start with matrix work; gate waits for high-risk endpoint coverage. |
| TODO sweep | yes | Critical/high TODO closure or approved downgrade | Can start now; downgrade policy applies immediately. |
| Docs validation | no, except validation must scan files | `docs-project.yaml` scan fix; filename normalization isolated | Scan-count fix can start now; filename migration is a separate docs-only phase/PR. |

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
- Docs-validate scan fix: fix `docs-project.yaml` path resolution so validation actually scans files.
- Docs filename normalization: lowercase legacy ADR/RFC/MEMO filenames only as an isolated docs-only phase with link checking; no product-code changes in the same PR.

Out of scope (v1.x):

- TODO-007 TS type safety.
- TODO-008 hardcoded values.
- TODO-009 template system (depends on RFC-006).

## Dependencies

- T1, T2, T3 must be far enough along that the dashboards have stable data.
- T5 Phase 1 (local mode) gives Playwright a deterministic backend.
- ADR-010 governs Playwright: use `data-test-*` selectors; automated runs are headless with `--reporter=line --max-failures=1`; never use `--headed` in automation.
- ADR-019 requires `cmd/hermes` to stay pure-Go (`CGO_ENABLED=0`) and keeps migrations in `cmd/hermes-migrate`; do not pull SQLite drivers into the server binary.
- ADR-020 requires all schema changes to use core+deltas migration files; the indexer remains stateless and submits through APIs only.

## Phases & exit criteria

### Phase 0 — Pre-flight split and low-risk cleanup

- Create owned checklist rows for each track: migrations bootstrap, docs validation, E2E dashboards, API coverage, TODO sweep.
- Add TODO-001 compile-time interface assertions, because they are independent and low blast-radius.
- Define the local developer bootstrap command after AutoMigrate removal: empty database -> `hermes-migrate up` -> server starts.
- Define rollback plans for each track before making broad changes.

**Exit when:**

- Each track has an owner, start condition, rollback plan, and exit check.
- TODO-001 is archived, or any remaining work is explicitly carried by the relevant track.
- The migration bootstrap command is documented in the appropriate developer guide or referenced from this plan.

### Phase 1 — Migrations cleanup

- Implement the proper fix for the AutoMigrate constraint-rename issue by deleting server-start AutoMigrate entirely and relying on `cmd/hermes-migrate`.
- Delete the AutoMigrate code path from server start.
- All schema changes flow through delta migrations (ADR-020).
- Add an empty-database bootstrap test: `cmd/hermes-migrate up` prepares the database, then the pure-Go `cmd/hermes` server starts without invoking AutoMigrate.
- Add a `CGO_ENABLED=0` server build check so SQLite migration dependencies remain outside `cmd/hermes` (ADR-019).

**Exit when:**

- `rg "AutoMigrate" internal/ pkg/ cmd/` returns hits only in tests or `cmd/hermes-migrate`.
- Empty database -> `hermes-migrate up` -> `CGO_ENABLED=0` server startup is covered by an automated or explicitly documented smoke test.
- `database_migration_fix_session.md` archived under `docs-internal/archive/`.

### Phase 2 — Docs-validate scan fix

- Fix `docs-project.yaml` path resolution.
- Add a command-level docs validation check that fails if scanned file count is zero.
- Keep this phase separate from filename normalization.

**Exit when:**

- `./scripts/docs-validate.sh` exits 0 with non-zero file count, or exits non-zero only for already-known filename-format violations that are explicitly listed in the validation output.
- The validation wrapper fails or reports an error if the scanned file count is zero.

### Phase 3 — Docs filename normalization (isolated)

- Rename legacy uppercase ADR/RFC/MEMO files to lowercase kebab-case (per templates guide) in docs-only changes.
- Fix all inbound links and `related:` references touched by the rename.
- Run a link check after every rename batch.

**Exit when:**

- `./scripts/docs-validate.sh` exits 0 with non-zero file count and no filename warnings.
- Link checking for changed docs passes.
- No product-code changes are bundled with the rename batch.

### Phase 4 — Playwright dashboards (TODO-011/012/013)

- Implement each suite using `data-test-*` selectors (ADR-010), headless only, `--reporter=line --max-failures=1`.
- Build test data through supported product APIs unless a test explicitly targets migration internals; avoid direct DB fixture writes for dashboard behavior.
- Cover local-mode/test-compose data setup once T5 Phase 1 and T1/T2/T3 data paths are stable enough to avoid fixture churn.
- Wire into CI required-checks.

**Exit when:**

- All three suites green in CI on three consecutive PRs.
- The three-PR signal uses production-shaped review/latest/recently-viewed fixtures, not over-controlled fixtures that bypass product paths.
- TODO-011/012/013 archived.

### Phase 5 — API coverage burn-down (TODO-002)

- Add an endpoint/risk coverage matrix for TODO-002 before writing more tests.
- Bring v1 + v2 API integration coverage to ≥80 %, including authn/authz, transaction rollback, error mapping, typed sentinel handling, and provider failure paths.
- Treat percentage as a trailing metric; high-risk endpoint rows in the matrix must pass before this phase closes.

**Exit when:**

- Endpoint/risk matrix rows for authz, rollback, error mapping, and provider failures are complete or have explicit owner-approved deferrals.
- Coverage gate enforced in CI.
- TODO-002 archived.

### Phase 6 — Tech-debt sweep

- `indexer-refactor.md` reviewed; if T2 cutover is done, archive it.
- Sweep for any remaining critical or high TODOs and either close them or downgrade them with owner approval and a linked reason.
- During the v1.0 freeze, do not pass this phase by reclassifying durability, correctness, auth, migration, or data-loss work without explicit approval.

**Exit when:**

- `docs-internal/plans/` contains zero open `priority: critical` and zero open `priority: high` items.
- Every critical/high downgrade made during T6 links to the approving owner and reason from the TODO or plan entry.

## Reviews / gates

Phase exit-only. Tracks may run in parallel only when their start conditions and rollback plans are satisfied. The T6 release gate passes only when each v1.0-blocking track has an exit check that cannot pass accidentally because validation scanned zero files or fixtures bypassed the product path.

## Risks

- **Flaky Playwright suites** undermine the gate. Mitigation: enforce `--max-failures=1` and run each new suite on three PRs before marking required.
- **AutoMigrate deletion** uncovers latent schema drift in dev environments. Mitigation: ship a one-time `hermes-migrate doctor` command in Phase 0.
- **Docs filename normalization** creates large link churn unrelated to release functionality. Mitigation: isolate rename-only work from product-code changes and link-check each batch.
- **Coverage percentage** rewards low-value tests. Mitigation: require endpoint/risk matrix completion for authz, rollback, error mapping, and provider failure paths before accepting the aggregate gate.
- **TODO downgrades** can hide unresolved release blockers. Mitigation: require owner approval and linked reasons for critical/high downgrades during the v1.0 freeze.

## References

- [TODO-001](todo-001-abstraction-interface-compile-checks.md), [TODO-002](todo-002-comprehensive-api-test-suite.md), [TODO-011](todo-011-e2e-test-awaiting-review-dashboard.md), [TODO-012](todo-012-e2e-test-latest-docs-dashboard.md), [TODO-013](todo-013-e2e-test-recently-viewed-sidebar.md)
- [`database_migration_fix_session.md`](database_migration_fix_session.md)
- [`indexer-refactor.md`](indexer-refactor.md)
- [ADR-010: Testing Strategy](../adr/adr-010-playwright-for-local-iteration.md) · [ADR-019: Split Server/Migrate Binaries](../adr/adr-019-split-server-and-migrate-binaries.md) · [ADR-020](../adr/adr-020-dual-database-support-stateless-indexer.md)
- [MEMO-032: E2E Testing Summary](../memo/memo-032-e2e-testing-summary.md) · [MEMO-063: SQLite Driver Conflict Investigation](../memo/memo-063-sqlite-driver-conflict-investigation.md)
- [Roadmap Implementation Tracker](roadmap-tracker.md)
