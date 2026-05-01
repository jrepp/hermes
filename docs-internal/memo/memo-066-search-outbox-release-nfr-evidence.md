---
id: memo-066
title: "Search Outbox Release NFR Evidence"
status: Final
created: 2026-05-01
date: 2026-05-01
author: Hermes Team
project_id: hermes
doc_uuid: 7dd2fb2f-b727-47bb-9625-ea8314a3e3b7
type: Memo
subtype: Analysis
tags: [nfr, search, outbox, performance, trajectory-008]
related:
  - RFC-008
  - MEMO-065
---

# MEMO-066: Search Outbox Release NFR Evidence

> Records the full T8 Phase 1 search-outbox release-profile run. The scenario sustained the release target, exercised relay pause/resume, converged within the deadline, and produced no unexpected DLQ rows.

## Background

Trajectory T8 owns reusable non-functional reliability and performance evidence. Phase 1 covers the T1 search-outbox stress/restart scenario: 1,000 search-outbox-backed mutations/minute for 10 minutes, relay pause/resume every 60 seconds, convergence within 30 seconds after recovery, and zero data loss.

This memo supersedes the bounded trial interpretation in [MEMO-065](memo-065-search-outbox-nfr-trial.md) for release-evidence purposes. MEMO-065 remains useful as the initial harness trial and blocker record.

## Release Run

Command:

```bash
go test -timeout 15m -tags=integration,nfr ./tests/integration/nfr \
  -run TestNFR/SearchOutboxStress \
  -profile release \
  -duration 10m \
  -rate 1000/min \
  -restart-interval 60s \
  -convergence-deadline 30s \
  -relay-batch-size 1000 \
  -output tmp/nfr/search-outbox-batched-release
```

Environment:

- Git commit: `unknown` because the run included uncommitted relay and harness changes.
- Go version: `go1.26.2`
- OS/arch: `darwin/arm64`
- Backend: testcontainers PostgreSQL + Meilisearch

Inputs:

- Duration: 600 seconds
- Target rate: 1,000/minute
- Restart interval: 60 seconds
- Convergence deadline: 30 seconds
- Relay batch size: 1,000

Observed result:

- Status: passed
- Items generated: 9,869
- Items completed: 9,869
- Worker restarts: 10
- Max convergence: 5 seconds
- Max queue depth: 887
- Unexpected DLQ rows: 0
- Errors: none

## Interpretation

The search outbox release profile satisfies the T8 Phase 1 release-evidence gate in this local testcontainers environment. It sustained the target input rate for 10 minutes, exercised repeated relay pause/resume behavior, and converged all generated events into Meilisearch within the 30-second deadline.

Two implementation changes were necessary before the full release run passed:

- The relay now batches document and draft upsert/delete writes through the existing search provider batch APIs.
- The relay now bulk-claims and bulk-completes successful batched events, avoiding one database update per successful projection.

The NFR harness now records `relayBatchSize` and uses an explicit `-relay-batch-size` flag so release evidence is reproducible.

## Follow-Ups

- Commit the relay batching and NFR harness changes before treating this evidence as tied to a stable git SHA.
- Keep release-profile scenarios manual or scheduled until three consecutive runs pass without unrelated infrastructure flakes.
- Add sampled `/health` queue-age capture in a follow-up if T8 needs richer operational time-series evidence.

## References

- [Trajectory T8 — Reliability & Performance NFR Harness](../plans/trajectory-008-nfr-reliability-performance.md)
- [Trajectory T1 — Data Consistency & Outbox](../plans/trajectory-001-data-consistency-outbox.md)
- [MEMO-065: Search Outbox NFR Trial](memo-065-search-outbox-nfr-trial.md)
- [RFC-008: Outbox Pattern for Document Synchronization](../rfc/rfc-008-outbox-pattern-document-sync.md)
