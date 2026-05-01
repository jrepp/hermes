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

- Done: harness entry point is Go integration tests under `tests/integration/nfr/`, selected by `-tags=integration,nfr` and `-run TestNFR/<scenario>`. The harness must not require headed browser sessions or interactive prompts.
- Done: scenario configuration is flag-driven with environment fallbacks: `-profile smoke|release`, `-duration`, `-rate`, `-restart-interval`, `-convergence-deadline`, `-max-items`, `-output`, and `-backend local|testcontainers|external`.
- Done: smoke profile is for local/PR confidence and release profile is for v1.0 evidence. Smoke profile may reduce duration/rate, but must exercise the same code path and emit the same result schema.
- Done: result artifacts are JSON plus a memo-ready Markdown summary written under `tmp/nfr/<scenario>/<timestamp>/` by default, with `-output` override for CI artifacts.
- Done: existing dependencies are the default runtime: PostgreSQL and Meilisearch via testcontainers where feasible. Redpanda is added only for indexer scenarios. External backends are opt-in with explicit URLs/credentials.
- Done: initial harness contract scaffold exists under `tests/integration/nfr/` and verifies flag defaults plus `result.json` / `summary.md` artifact emission.

### Harness Command Shape

Search-outbox smoke profile:

```bash
go test -tags=integration,nfr ./tests/integration/nfr \
  -run 'TestNFR/SearchOutboxStress' \
  -profile smoke \
  -reporter=line
```

Search-outbox release-evidence profile:

```bash
go test -tags=integration,nfr ./tests/integration/nfr \
  -run 'TestNFR/SearchOutboxStress' \
  -profile release \
  -duration 10m \
  -rate 1000/min \
  -restart-interval 60s \
  -convergence-deadline 30s \
  -output tmp/nfr/search-outbox/$(date -u +%Y%m%dT%H%M%SZ)
```

Indexer throughput release-evidence profile:

```bash
go test -tags=integration,nfr ./tests/integration/nfr \
  -run 'TestNFR/IndexerThroughput' \
  -profile release \
  -duration 60m \
  -rate 1000/hour \
  -convergence-deadline 30s \
  -output tmp/nfr/indexer/$(date -u +%Y%m%dT%H%M%SZ)
```

### Profiles

| Profile | Purpose | Required in PR CI? | Default duration | Default rate | Notes |
|---|---:|---:|---:|---:|---|
| `smoke` | Fast local confidence | no | 2m | scenario-specific small rate | Must cover restart/recovery behavior when the scenario has a restart dimension. |
| `release` | v1.0 evidence | no, manual or nightly until variance is known | scenario-specific | scenario-specific | Results must be recorded as a memo before a trajectory cites them as release evidence. |

### Scenario Configuration Contract

- `profile`: selects defaults; explicit flags override profile defaults.
- `duration`: wall-clock mutation/input generation duration.
- `rate`: target input rate using `/min` or `/hour` suffix.
- `restart-interval`: pause/resume or restart cadence for worker/relay scenarios; `0` disables restarts.
- `convergence-deadline`: maximum allowed catch-up time after input stops or worker recovers.
- `max-items`: optional cap for bounded trials; `0` means run until `duration` elapses.
- `backend`: `testcontainers` by default for repeatability; `external` requires explicit endpoint environment variables.
- `output`: directory for `result.json`, `summary.md`, logs, and any sampled health/lag time series.

### Result Schema

Every scenario writes `result.json` with these top-level fields:

```json
{
  "scenario": "search-outbox-stress",
  "profile": "release",
  "startedAt": "2026-04-30T00:00:00Z",
  "finishedAt": "2026-04-30T00:10:45Z",
  "command": "go test ...",
  "environment": {
    "gitCommit": "<sha>",
    "goVersion": "<version>",
    "os": "<goos/goarch>",
    "backend": "testcontainers"
  },
  "inputs": {
    "durationSeconds": 600,
    "targetRate": "1000/min",
    "restartIntervalSeconds": 60,
    "convergenceDeadlineSeconds": 30,
    "maxItems": 0
  },
  "observations": {
    "itemsGenerated": 10000,
    "itemsCompleted": 10000,
    "workerRestarts": 10,
    "maxConvergenceSeconds": 18,
    "maxQueueDepth": 250,
    "unexpectedDLQ": 0,
    "errors": []
  },
  "passed": true,
  "followUps": []
}
```

The paired `summary.md` must be memo-ready and include:

- command and git commit
- environment and backend profile
- scenario inputs and thresholds
- observed throughput, lag, convergence, restarts, failures, and DLQ counts
- pass/fail conclusion
- follow-up links or explicit "none"

### CI Policy

- T8 NFR scenarios are not required PR checks during Phase 0 or Phase 1.
- Smoke profiles may be added as optional CI checks once they are stable and under the normal integration-test budget.
- Release profiles are manual or scheduled/nightly until at least three consecutive runs pass without unrelated infrastructure flakes.
- A product trajectory may cite a release-profile T8 result only when the result memo is committed or linked from `docs-internal/memo/`.

**Exit when:**

- Done: harness design is documented in this trajectory.
- Done: search-outbox scenario inputs and pass/fail thresholds are explicit.
- Done: CI policy is explicit: release profiles are manual/nightly until stability is proven; PR gating is not required in Phase 0.

## Phase 1 — Search-Outbox Stress/Restart Scenario

- Done: smoke-capable search-outbox NFR scenario exists under `tests/integration/nfr/` and exercises event generation, relay pause/resume, convergence, Meilisearch verification, and artifact emission.
- Done: bounded release-rate trial is recorded in [MEMO-065](../memo/memo-065-search-outbox-nfr-trial.md); full 10-minute release evidence remains open.
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
