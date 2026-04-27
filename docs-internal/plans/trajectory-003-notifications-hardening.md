---
id: trajectory-003
title: "Trajectory T3 — Notifications Hardening"
status: Draft
created: 2026-04-27
date: 2026-04-27
author: Hermes Team
project_id: hermes
doc_uuid: 2269a5df-dc31-4932-a4e7-661effe2cbdb
type: Memo
subtype: Milestone
tags: [trajectory, roadmap, v1.0, notifications, async-email, redpanda]
related:
  - RFC-008
  - RFC-013
  - MEMO-051
  - TODO-004
---

# Trajectory T3 — Notifications Hardening

> RFC-013 ships notifications via audit/mail/ntfy backends and is functionally production-ready (~85 %). v1.0 needs the synchronous email path in API handlers replaced with notification publishes (TODO-004), plus enough operational hardening to debug failures.

## Readiness status

Not execution-ready until Phase 0 completes. The adversarial review is resolved by this hardened contract: request-path email migration must not silently downgrade durability, every call site must declare its delivery class and idempotency key, and transactional notification types must use a DB-transaction-bound enqueue path once T1 outbox primitives are available.

## Stable-release criteria served

- v1.0 criterion **#2 Async correctness** (no request-path SMTP).
- Indirectly supports criterion **#1 Durability** by routing notifications through the outbox once T1 lands.

## Scope

In scope:

- Replace synchronous `SendEmail` in `internal/api/documents.go` and `internal/api/reviews.go` (and v2 equivalents) with notification publishes.
- Replace synchronous email helpers in current v2 write paths: `internal/api/v2/documents.go`, `internal/api/v2/reviews.go`, `internal/api/v2/approvals.go`, `internal/api/v2/patch_helpers.go`, and any v1 equivalents that remain active.
- Document the publish-then-deliver contract per notification type: synchronous-required, transactional, at-least-once, or best-effort.
- Operational visibility: DLQ size/age metrics, oldest pending age, per-backend success/failure counters, structured redacted logs, and deterministic delivery probes.

Out of scope (deferred to v1.x):

- Slack, Discord, Teams backends.
- Redis-based deduplication.
- Generic user notification preferences.
- Message-body encryption. Redaction and allowed-field rules are in scope for v1.0 because audit logs, DLQs, and ntfy topics must not expose unnecessary sensitive content.

## Dependencies

- T1 outbox primitives for transactional notifications. Direct publish before T1 is allowed only for Phase 0 probes or notification types explicitly classified as best-effort.
- RFC-013 backends — done.
- RFC-008 accepted outbox semantics — reuse the same transaction, idempotency, retry, DLQ, and observability vocabulary where possible.

## Delivery classes

| Class | User contract | Transaction boundary | Examples | Direct publish allowed? |
|-------|---------------|----------------------|----------|-------------------------|
| `synchronous-required` | Caller must know delivery or handoff result before success is reported. | Not converted in T3 without a separate design. | Password reset, security-sensitive verification flows, if discovered. | No. Keep synchronous or split into a dedicated security flow. |
| `transactional` | Notification is part of a durable user workflow and must survive process crashes after the mutation commits. | Enqueue notification event in the same DB transaction as the triggering mutation. | Review request, approval, publish/subscriber notification, ownership transfer. | No. Requires T1 Phase 1-style transactional enqueue. |
| `at-least-once` | Durable enough to retry, but duplicate delivery is possible and must be suppressed at the recipient/event level. | Prefer transaction-bound enqueue; may use a durable notification outbox if not coupled to the triggering mutation. | Operational or audit-heavy notifications without workflow correctness impact. | Only with explicit owner acceptance in the audit matrix. |
| `best-effort` | Missing notification is acceptable and does not affect workflow correctness. | No DB transaction requirement. | Synthetic probes, local-dev smoke messages, non-critical diagnostics. | Yes, with structured log and metric. |

Transactional and at-least-once notifications must carry an idempotency key with enough recipient scope to suppress duplicate user-visible messages. Use the form `notifications:{notification_type}:{triggering_aggregate}:{event_sequence}:{recipient_id_or_email}:{backend}` unless the audit matrix documents a stronger key.

## Phase 0 audit matrix

Phase 0 must verify these rows against code before implementation. Line numbers are intentionally omitted because this matrix is the durable migration contract, not a point-in-time grep result.

| Flow | Handler file(s) | Notification type | Recipient source | Triggering mutation | Required durability | Idempotency key | Allowed payload fields | Migration path |
|------|-----------------|-------------------|------------------|---------------------|---------------------|-----------------|------------------------|----------------|
| Review requested from document update | `internal/api/v2/documents.go`, v1 equivalent if active | `review_requested` | Reviewers selected by request/workflow | Document review-state mutation | `transactional` | `notifications:review_requested:{document_uuid}:{review_sequence}:{reviewer_email}:mail` | Document UUID, provider ID, project ID, short name/title, reviewer email/name, requester display name, document URL; no body/content diffs. | Enqueue inside same DB transaction as review-state write; deliver through RFC-013 mail/audit backends. |
| Review requested from publish/review flow | `internal/api/v2/reviews.go`, v1 equivalent if active | `review_requested` | Reviewers on review request | Review request creation or publish transition | `transactional` | `notifications:review_requested:{document_uuid}:{review_sequence}:{reviewer_email}:mail` | Same as above. | Same transaction as review request creation. |
| Document approved | `internal/api/v2/approvals.go`, v1 equivalent if active | `document_approved` | Document owner/requester and configured watchers | Approval/review-state mutation | `transactional` | `notifications:document_approved:{document_uuid}:{approval_sequence}:{recipient_email}:mail` | Document UUID, project ID, title/short name, approver display name, recipient email/name, document URL; no approval comments unless explicitly reviewed for PII. | Same transaction as approval state update. |
| New owner assigned | `internal/api/v2/patch_helpers.go`, v1 equivalent if active | `new_owner` | New owner field after patch | Document owner update | `transactional` | `notifications:new_owner:{document_uuid}:{owner_change_sequence}:{new_owner_email}:mail` | Document UUID, project ID, title/short name, old/new owner display names/emails, document URL; no document body. | Same transaction as document owner patch. |
| Document published to subscribers | `internal/api/v2/reviews.go`, v1 equivalent if active | `document_published` | Subscribers/watchers resolved from project/document state | Publish transition | `at-least-once` unless product owner requires transactional | `notifications:document_published:{document_uuid}:{publish_sequence}:{subscriber_email}:mail` | Document UUID, project ID, title/short name, publisher display name, subscriber email/name, document URL; no body/content. | Prefer same transaction as publish state change; if classified best-effort by owner, document acceptance before direct publish. |
| `/me` test email or user-triggered diagnostic | `internal/api/v2/me.go` | `diagnostic_email` | Current authenticated user | No durable workflow mutation | `best-effort` | `notifications:diagnostic_email:{request_id}:{user_email}:mail` | Recipient email/name, request ID, static diagnostic content only. | Direct publish is acceptable with timeout, log, and failure metric. |

Security-sensitive password-reset-style flows are not known in the current API audit. If Phase 0 finds one, classify it as `synchronous-required` until a separate security-token delivery design exists.

## Phases & exit criteria

### Phase 0 — Call-site audit

- Enumerate every `mail.Send`, `email.Send`, `email.Send*`, `SendEmail`, and `SendEmailWithTemplate` call in `internal/api/` and `internal/api/v2/`.
- Confirm or update the audit matrix above with call site, notification type, recipient source, triggering DB mutation, required durability, idempotency key, allowed payload fields, and migration path.
- Decide whether `document_published` is transactional or accepted as at-least-once/best-effort by the product owner; default to transaction-bound until decided.
- Identify any password-reset-style or security-sensitive flow and exclude it from generic notification fanout unless a dedicated synchronous-required design is approved.

**Exit when:**

- Audit matrix is accurate against current code and every row has an owner-approved delivery class.
- No transactional row depends on direct publish fallback.
- Allowed payload fields are explicit for logs, audit backend, Redpanda payloads, DLQ rows, and ntfy topics.

### Phase 1 — Transactional enqueue contract

- Add or reuse a notification outbox enqueue helper that requires a transaction handle for `transactional` rows.
- The API shape must make durability visible at the call site; avoid a generic `notifications.Publish(ctx, msg)` wrapper for transactional paths. Prefer names such as `EnqueueTransactionalNotification(tx, msg)` and `PublishBestEffort(ctx, msg)`.
- Require idempotency key creation before enqueue/publish and enforce duplicate-safe handling per `(notification_type, triggering aggregate, sequence, recipient, backend)`.
- Add payload redaction/allowlist tests covering logs, audit backend output, Redpanda payloads, DLQ rows, and ntfy topics.

**Exit when:**

- Transactional enqueue participates in the same DB transaction as the triggering mutation and survives a crash after commit but before delivery.
- Duplicate event tests prove repeated enqueue/publish attempts do not create duplicate user-visible messages for the same recipient/backend/event.
- Best-effort publish API exists only for rows classified `best-effort` or explicitly owner-accepted at-least-once exceptions.

### Phase 2 — Migration to publishes

- Replace each audited synchronous email call with the correct durability-specific API.
- Transactional rows route through the notification outbox; best-effort diagnostic rows may direct publish.
- Remove now-unused SMTP plumbing from request handlers after the corresponding call sites are migrated.
- Keep workspace provider email methods only where still needed by provider compatibility; do not leave request handlers calling them synchronously.

**Exit when:**

- `rg "email\.Send|SendEmail|SendEmailWithTemplate" internal/api internal/api/v2` returns zero synchronous request-handler email sends, excluding tests and approved compatibility adapters.
- Crash-window test: commit the triggering mutation, stop/crash the publisher before delivery, restart, and verify the notification is eventually delivered once from the user's perspective.
- TODO-004 status = `completed`, archived.

### Phase 3 — Operational hardening

- DLQ size + age metrics exposed.
- Oldest pending notification age and per-backend success/failure counters exposed on `/metrics`.
- Synthetic test notification path for mail, ntfy, and audit backends exists in the testing compose stack.
- Operator runbook for "notifications are not arriving" added to `docs-internal/guides/`.
- Operator runbook includes duplicate-notification triage, not only missing-notification triage.

**Exit when:**

- Metrics scrape-tested in the testing compose stack.
- Synthetic probe verifies SMTP/Mailhog acceptance, ntfy routing, and audit backend output with deterministic test content.
- Runbook exists and is linked from MEMO-051.

## Reviews / gates

Phase exit-only. Do not start request-path migration until Phase 0 delivery classes and Phase 1 transactional enqueue semantics are approved. Promote RFC-013 to `Implemented` when Phase 3 closes (the optional enhancements remain explicitly v1.x).

## Risks

- **Lost notifications during the migration window** if request-path email is replaced with direct publish before T1-style transactional enqueue exists. Mitigation: no direct publish fallback for transactional rows; only best-effort diagnostics may direct publish.
- **Duplicate user-visible notifications** under at-least-once delivery. Mitigation: idempotency key includes recipient/backend scope and tests prove duplicate suppression.
- **Sensitive payload leakage** through audit logs, DLQs, or public ntfy topics. Mitigation: enforce allowed-field payload construction and redaction tests before migration.
- **Operator confusion** between outbox lag and notification lag. Mitigation: distinct metric names (`outbox_relay_lag_ms` vs `notifications_publish_lag_ms`).
- **Security-sensitive flows accidentally routed through generic fanout.** Mitigation: classify password-reset-style flows as `synchronous-required` and exclude them from T3 unless separately designed.

## References

- [RFC-013: Multi-Backend Notification System](../rfc/rfc-013-notification-backend.md)
- [RFC-008: Outbox Pattern for Document Synchronization](../rfc/rfc-008-outbox-pattern-document-sync.md)
- [TODO-004: Asynchronous Email Sending](todo-004-async-email-sending.md)
- [MEMO-051: Notification Implementation Status](../memo/memo-051-notification-implementation-status.md)
- [Adversarial Review — T3 Notifications Hardening](trajectory-003-adversarial-review.md)
- [Roadmap Implementation Tracker](roadmap-tracker.md)
