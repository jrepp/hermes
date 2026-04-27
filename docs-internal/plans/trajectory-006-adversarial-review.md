---
id: trajectory-006-adversarial-review
title: "Adversarial Review — T6 E2E Coverage & Tech Debt"
status: Draft
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

> This review tries to break [Trajectory T6](trajectory-006-e2e-and-tech-debt.md) before execution. T6 collects necessary release gates, but it is currently a catch-all trajectory with sequencing, blast-radius, and validation risks.

## Run-readiness verdict

Start only the independent audits and small compile-time assertion work now. Defer broad docs filename migration, AutoMigrate deletion, and CI gate changes until their blast radius is isolated and the dependent trajectories provide stable data.

## Highest-risk failure modes

1. **T6 is too broad to manage as one execution stream.** It mixes migration architecture, docs tooling, Playwright, API coverage, TODO closure, and indexer cleanup. A failure in any one area can block the whole trajectory without clarifying the v1.0 risk.

2. **Deleting AutoMigrate can break local development unless migration bootstrap is excellent.** [ADR-019](../adr/adr-019-split-server-and-migrate-binaries.md) requires migrations in `cmd/hermes-migrate`, but developers still need an obvious path from empty DB to running server.

3. **Docs filename normalization can create massive link churn.** Renaming legacy ADR/RFC/MEMO files in the same trajectory as release gates risks merge conflicts and broken inbound links unrelated to v1.0 functionality.

4. **The Playwright gate depends on unstable upstream trajectories.** Dashboard E2E will be flaky if T1/T2/T3/T5 are still changing data setup, indexing lag, auth, or local provider semantics.

5. **Three consecutive green PRs is not enough if the test data model is weak.** A suite can be consistently green because it uses over-controlled fixtures that do not match production review/latest/recently-viewed behaviour.

6. **Coverage percentage can incentivize low-value tests.** `≥80 %` across v1 + v2 can be gamed by easy paths while missing authz, transaction rollback, error mapping, and provider failures.

7. **The high-priority TODO sweep can hide unresolved work by downgrading priority.** The plan allows downgrades with documented reason, but v1.0 should not pass if critical durability or correctness work is simply reclassified.

8. **`docs-validate` is known to be effectively no-op today.** The trajectory correctly calls this out, but the exit criterion must assert file count and warning policy so the team does not get a false green.

## Missing decisions before execution

- Define which T6 items are v1.0 blockers versus cleanup.
- Define the developer bootstrap command after AutoMigrate removal.
- Define docs rename strategy: all at once, category by category, or separate PRs.
- Define Playwright fixture ownership and whether tests run against local mode, testing compose, or both.
- Define API coverage quality gates by endpoint/risk, not only aggregate percentage.
- Define a policy for priority downgrades during the v1.0 freeze.

## Concrete pre-flight checklist

- Split T6 execution board into independent tracks: migrations, docs tooling, E2E, API coverage, TODO sweep.
- Add a migration bootstrap test: empty database → `hermes-migrate up` → server starts with `CGO_ENABLED=0` server binary.
- Add a command-level docs validation test that fails if scanned file count is zero.
- Run a link check after any filename migration and keep rename-only changes separate from content edits.
- Add Playwright data setup through supported APIs, not direct DB writes, unless the test explicitly targets migration internals.
- Add endpoint/risk coverage matrix for TODO-002 before writing more tests.

## Suggested plan edits

- Move docs filename normalization to its own sub-plan or phase with no product-code changes.
- Make Phase 0 include a documented replacement for AutoMigrate in local setup and CI.
- Change the TODO sweep exit criterion to require approval for any critical/high downgrade, with the reason linked from the TODO.
- Add ADR-010 commands explicitly: headless Playwright with `--reporter=line --max-failures=1`, never `--headed` in automation.

## Go/no-go gate

Use T6 as a release gate only after each track has an owner, a rollback plan, and an exit check that cannot pass accidentally because validation scanned zero files or fixtures bypassed the product path.
