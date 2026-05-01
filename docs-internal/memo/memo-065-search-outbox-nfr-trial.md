---
id: memo-065
title: "Search Outbox NFR Trial"
status: Final
created: 2026-05-01
date: 2026-05-01
author: Hermes Team
project_id: hermes
doc_uuid: 96900eb2-c1c9-4e4e-8134-bf9d4eebf599
type: Memo
subtype: Analysis
tags: [nfr, search, outbox, performance, trajectory-008]
related:
  - RFC-008
  - trajectory-001
  - trajectory-008
---

# MEMO-065: Search Outbox NFR Trial

> Records the first T8 search-outbox NFR trial after the harness was added. The bounded release-rate trial passed, but the full 10-minute release-evidence run is not complete yet.

## Background

Trajectory T8 owns reusable non-functional reliability and performance evidence. The first scenario is the T1 search-outbox stress/restart case: 1,000 search-outbox-backed mutations/minute for 10 minutes, relay restart or pause/resume every 60 seconds, convergence within 30 seconds after recovery, and zero data loss.

## Trial Run

Command:

```bash
go test -tags=integration,nfr ./tests/integration/nfr \
  -run TestNFR/SearchOutboxStress \
  -profile release \
  -duration 2m \
  -rate 1000/min \
  -restart-interval 10s \
  -convergence-deadline 30s \
  -max-items 50 \
  -output tmp/nfr/search-outbox-trial
```

Environment:

- Git commit before the trial-result commit: `ddc516f4`
- Go version: `go1.26.2`
- OS/arch: `darwin/arm64`
- Backend: testcontainers PostgreSQL + Meilisearch

Observed result:

- Status: passed
- Items generated: 50
- Items completed: 50
- Target rate: 1,000/minute
- Max queue depth: 15
- Unexpected DLQ rows: 0
- Max convergence recorded by harness: 30 seconds
- Worker restarts recorded: 0

## Interpretation

This is a bounded release-rate smoke trial, not full release evidence. It proves the harness can drive the search outbox at the target input rate for a small item cap and converge all generated items through Meilisearch without DLQ rows.

It does not satisfy the full T8 Phase 1 evidence gate because `-max-items 50` ended generation before the configured restart interval could exercise pause/resume behavior, and the duration was shorter than the required 10 minutes.

An earlier unbounded release-rate trial with `-duration 30s`, `-rate 1000/min`, and `-restart-interval 10s` exceeded the 2-minute agent command timeout before producing artifacts. That does not prove a system failure, but it shows the current harness and relay behavior need a longer runner budget and/or more efficient batch processing before full release evidence can be collected reliably in this environment.

## Follow-Ups

- Run full release evidence outside the 2-minute agent command budget or with a larger explicit timeout.
- Add batched relay processing or provider batch writes if full release evidence cannot converge within the T8 deadline.
- Keep this memo linked from T8 until a full release-evidence memo supersedes it.

## References

- [Trajectory T8 — Reliability & Performance NFR Harness](../plans/trajectory-008-nfr-reliability-performance.md)
- [Trajectory T1 — Data Consistency & Outbox](../plans/trajectory-001-data-consistency-outbox.md)
- [RFC-008: Outbox Pattern for Document Synchronization](../rfc/rfc-008-outbox-pattern-document-sync.md)
