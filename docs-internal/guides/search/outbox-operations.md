---
id: search-outbox-operations
title: "Search Outbox Operations"
status: Reference
created: 2026-04-30
author: Hermes Team
project_id: hermes
doc_uuid: b1349f81-6d51-45d0-a28a-9f0e7e8fdbc5
type: Guide
subtype: Playbook
tags: [search, outbox, operations, dlq]
related:
  - RFC-008
---

# Search Outbox Operations

> Use this playbook to inspect failed search projection events, retry recoverable failures, and explicitly skip poison events that block later events for the same aggregate.

## Inspect Failures

Check queue depth and lag from the server health endpoint:

```bash
curl -s http://localhost:8000/health
```

When the database is available, the response includes `searchOutbox.pending`, `searchOutbox.failed`, `searchOutbox.dlq`, `searchOutbox.oldestPendingAgeSeconds`, `searchOutbox.oldestFailedAgeSeconds`, and `searchOutbox.oldestDlqAgeSeconds`.

Recommended alert thresholds for polling `/health`:

- Warning: `searchOutbox.oldestPendingAgeSeconds > 300` for 5 minutes.
- Warning: `searchOutbox.failed > 0` or `searchOutbox.oldestFailedAgeSeconds > 300` for 5 minutes.
- Critical: `searchOutbox.dlq > 0` or `searchOutbox.oldestDlqAgeSeconds > 0` for 5 minutes.
- Critical: `searchOutbox.processing > 0` with no pending-age decrease for 10 minutes.

These thresholds intentionally key off age as well as counts so short retry backoff windows do not page operators, while durable failed or DLQ rows do.

List failed and DLQ events:

```bash
hermes operator search-outbox -config /path/to/hermes.hcl list -limit 50
```

Each row includes the event ID, status, aggregate, sequence, attempt count, event type, and last error.

## Retry An Event

Use retry when the underlying search backend or payload issue has been fixed:

```bash
hermes operator search-outbox -config /path/to/hermes.hcl retry 123
```

Retry sets the event back to `pending`, clears the last error, and makes it immediately available to the relay.

## Skip A Poison Event

Use skip only when replaying the event would be harmful or obsolete and later events for the same aggregate should proceed:

```bash
hermes operator search-outbox -config /path/to/hermes.hcl -note "superseded by later document update" skip 123
```

The note is required. Skipping changes the event status to `skipped`, which unblocks later same-aggregate events because the relay only blocks on pending, processing, failed, and DLQ predecessors.

## Rebuild Current Projection

If several events are skipped or the search index is suspected to have drifted, enqueue a fresh projection from database truth:

```bash
hermes operator search-outbox -config /path/to/hermes.hcl rebuild-current 123
```

The command reads event `123`, finds the same aggregate in the database, and enqueues a new `search.backfill` event. If the database row no longer exists, it enqueues a delete projection for the same aggregate.
