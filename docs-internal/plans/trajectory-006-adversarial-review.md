---
id: trajectory-006-adversarial-review
title: "Adversarial Review — T6 E2E Coverage & Tech Debt"
status: Resolved
created: 2026-04-27
date: 2026-04-27
author: Hermes Team
project_id: hermes
doc_uuid: 3d33fe0b-6be7-45d2-8ad8-8298b149debf
type: Memo
subtype: Analysis
tags: [trajectory, adversarial-review, v1.0, e2e, tech-debt, migrations]
related:
  - ADR-010
  - ADR-019
  - ADR-020
  - MEMO-063
---

# Adversarial Review — T6 E2E Coverage & Tech Debt

> This review tried to break [Trajectory T6](trajectory-006-e2e-and-tech-debt.md) before execution. The blocking sequencing, blast-radius, fixture, validation, and downgrade-policy concerns are now folded into the source plan; Phase 0 is ready to execute, while broad changes remain gated by track-specific pre-flight checks.

## Run-readiness verdict

Phase 0 can start. Start only the independent audits and small compile-time assertion work until each track has an owner, rollback plan, and non-accidental exit check. Defer broad docs filename migration, AutoMigrate deletion, required CI gates, and Playwright dashboard gating until their track-specific start conditions are satisfied.

## Resolution summary

- **Scope split resolved:** the source plan now treats migrations bootstrap, E2E dashboards, API coverage, TODO sweep, and docs validation as independent tracks with separate start conditions.
- **AutoMigrate bootstrap resolved:** Phase 0 must define the local bootstrap command, and Phase 1 must prove empty DB -> `hermes-migrate up` -> pure-Go server startup without server-start AutoMigrate.
- **Docs churn resolved:** docs scan-count repair is separated from filename normalization; filename migration is an isolated docs-only phase with link checking.
- **Playwright fixture risk resolved:** dashboard suites must use ADR-010 headless commands and production-shaped fixtures created through product APIs unless testing migration internals.
- **Coverage quality resolved:** TODO-002 now requires an endpoint/risk matrix for authz, rollback, error mapping, typed sentinel handling, and provider failures before the aggregate percentage gate can close.
- **TODO downgrade risk resolved:** critical/high downgrades during the v1.0 freeze require owner approval and linked rationale.

## Highest-risk failure modes

1. **T6 is too broad to manage as one execution stream.** Resolved in the source plan by splitting T6 into migrations bootstrap, E2E dashboards, API coverage, TODO sweep, and docs validation tracks.

2. **Deleting AutoMigrate can break local development unless migration bootstrap is excellent.** Resolved as a Phase 0/1 gate: define the bootstrap command and prove empty DB -> `hermes-migrate up` -> `CGO_ENABLED=0` server startup. [ADR-019](../adr/adr-019-split-server-and-migrate-binaries.md) still owns the split-binary rule.

3. **Docs filename normalization can create massive link churn.** Resolved by isolating filename normalization into a docs-only phase separate from the docs scan-count fix and all product-code changes.

4. **The Playwright gate depends on unstable upstream trajectories.** Partially resolved: the source plan now gates Playwright dashboard execution on stable T1/T2/T3/T5 data setup and auth/local-provider semantics.

5. **Three consecutive green PRs is not enough if the test data model is weak.** Resolved in the plan: the three-PR signal must use production-shaped review/latest/recently-viewed fixtures created through product APIs unless a test explicitly targets migration internals.

6. **Coverage percentage can incentivize low-value tests.** Resolved by requiring an endpoint/risk matrix for high-risk paths before accepting the aggregate `>=80 %` gate.

7. **The high-priority TODO sweep can hide unresolved work by downgrading priority.** Resolved by requiring owner approval and linked reasons for critical/high downgrades during the v1.0 freeze.

8. **`docs-validate` is known to be effectively no-op today.** Resolved by adding a scan-count check that fails or reports an error if validation scans zero files.

## Missing decisions before execution

- Source plan now classifies each track's v1.0-blocker status.
- Phase 0 now requires the developer bootstrap command after AutoMigrate removal.
- Source plan now isolates docs filename normalization into a docs-only phase/PR.
- Source plan now requires Playwright fixture setup through supported APIs and gates execution on stable upstream trajectories.
- TODO-002 now requires API coverage quality gates by endpoint/risk, not only aggregate percentage.
- Source plan now requires owner approval and linked reasons for priority downgrades during the v1.0 freeze.

## Concrete pre-flight checklist

- Done: source plan now splits T6 into independent tracks.
- Done: Phase 1 now requires empty database -> `hermes-migrate up` -> `CGO_ENABLED=0` server startup coverage.
- Done: Phase 2 now requires docs validation to fail or report an error if scanned file count is zero.
- Done: Phase 3 now requires link checking and docs-only filename migration batches.
- Done: Phase 4 now requires Playwright data setup through supported APIs unless the test targets migration internals.
- Done: Phase 5 now requires an endpoint/risk coverage matrix for TODO-002 before more tests.

## Suggested plan edits

- Done: docs filename normalization moved to its own phase with no product-code changes.
- Done: Phase 0 requires a documented replacement for AutoMigrate in local setup; Phase 1 requires CI/smoke-test proof.
- Done: TODO sweep exit criteria require approval for any critical/high downgrade, with the reason linked from the TODO or plan entry.
- Done: ADR-010 commands are explicit: headless Playwright with `--reporter=line --max-failures=1`, never `--headed` in automation.

## Go/no-go gate

Start Phase 0 now. Use T6 as a release gate only after each track has an owner, a rollback plan, and an exit check that cannot pass accidentally because validation scanned zero files or fixtures bypassed the product path.
