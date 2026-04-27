---
id: TODO-004
title: Implement Asynchronous Email Sending
date: 2025-10-09
type: TODO
priority: high
status: open
tags: [email, async, performance, api, documents, reviews]
related:
  - RFC-008
  - RFC-013
---

# Implement Asynchronous Email Sending

## Description

Multiple API endpoints currently send emails synchronously, which blocks the HTTP response until the email is sent. This creates a poor user experience and potential timeout issues.

This TODO is owned by [Trajectory T3 — Notifications Hardening](trajectory-003-notifications-hardening.md). Follow T3's hardened delivery contract rather than introducing a standalone email queue: workflow-coupled notifications must use transaction-bound enqueue with recipient-scoped idempotency, while direct publish is reserved for best-effort diagnostics.

## Code References

### Documents API (v1)
- **File**: `internal/api/documents.go`
- **Lines**: 516, 545-547
```go
// TODO: use a template for email content.
// TODO: use an asynchronous method for sending emails because we
//       currently block returning the HTTP response until sent.
// TODO: SendEmail is not part of workspace.Provider interface yet
```

### Documents API (v2)
- **File**: `internal/api/v2/documents.go`
- **Line**: 863
```go
// TODO: use an asynchronous method for sending emails because we
//       currently block returning the HTTP response until sent.
```

### Reviews API (v1)
- **Files**: `internal/api/reviews.go`
- **Lines**: 472, 524
```go
// TODO: use an asynchronous method for sending emails because we
//       currently block returning the HTTP response until sent.
```

### Reviews API (v2)
- **File**: `internal/api/v2/reviews.go`
- **Lines**: 557, 701
```go
// TODO: use an asynchronous method for sending emails because we
//       currently block returning the HTTP response until sent.
```

## Required Solution

Use the RFC-013 notification backend through the T3 delivery contract:

- Classify each call site as `synchronous-required`, `transactional`, `at-least-once`, or `best-effort` before migration.
- For `transactional` rows, enqueue the notification event in the same DB transaction as the triggering mutation once T1 outbox primitives are available.
- For `at-least-once` rows, define duplicate suppression and obtain owner acceptance if the notification is not transaction-bound.
- For `best-effort` diagnostics, direct publish is acceptable with metrics and structured logs.
- Do not use in-process Go channels for workflow notifications; they lose events on process crash and fail the T3 durability gate.

## Tasks

- [ ] Complete the T3 Phase 0 call-site audit matrix.
- [ ] Add or reuse transaction-bound notification enqueue for workflow-coupled notifications.
- [ ] Add recipient/backend-scoped idempotency keys and duplicate suppression tests.
- [ ] Update synchronous request-handler email calls to use the correct durability-specific notification API.
- [ ] Add payload allowlist/redaction tests for logs, audit output, Redpanda payloads, DLQ rows, and ntfy topics.
- [ ] Add notification status tracking, DLQ metrics, per-backend metrics, and synthetic delivery probes.
- [ ] Add operator runbook coverage for missing and duplicate notifications.

## Impact

**Files Affected**: 4 files, ~6 locations
**Complexity**: Medium-High
**User Experience**: High impact - removes blocking HTTP calls

## Related Work

This connects to RFC-008 (Outbox Pattern) which proposes the same pattern for document search indexing.
It also depends on RFC-013 for the notification backend and on T3 for the per-notification delivery contract.

## References

- RFC-008 - Outbox Pattern for Document Synchronization
- RFC-013 - Multi-Backend Notification System
- `docs-internal/plans/trajectory-003-notifications-hardening.md` - T3 delivery contract and gates
- `internal/api/documents.go` - Document status change emails
- `internal/api/v2/documents.go` - V2 document emails
- `internal/api/reviews.go` - V1 review notification emails
- `internal/api/v2/reviews.go` - V2 review notification emails
