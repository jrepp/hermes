---
id: trajectory-003-adversarial-review
title: "Adversarial Review — T3 Notifications Hardening"
status: Final
created: 2026-04-27
date: 2026-04-27
author: Hermes Team
project_id: hermes
doc_uuid: 122a7bc1-30b7-4ebf-87bb-eb016c88a02c
type: Memo
subtype: Analysis
tags: [trajectory, adversarial-review, v1.0, notifications, async-email]
related:
  - RFC-008
  - RFC-013
  - TODO-004
---

# Adversarial Review — T3 Notifications Hardening

> This review tries to break [Trajectory T3](trajectory-003-notifications-hardening.md) before execution. The plan targets the right synchronous-email risk, but it needs a stricter delivery contract and stronger dependency handling around T1.

## Resolution

Resolved in [Trajectory T3](trajectory-003-notifications-hardening.md) on 2026-04-27. The source plan now requires delivery classes, recipient-scoped idempotency keys, explicit transaction boundaries, payload allowlists, synthetic probes, duplicate-notification triage, and a no-direct-publish rule for transactional notification types.

## Run-readiness verdict

Do not migrate request-path email call sites until T3 Phase 0 and Phase 1 pass. Direct publish fallback is now limited to best-effort diagnostics or explicitly owner-accepted non-workflow notifications; review workflows and other durable user mutations require transaction-bound enqueue.

## Highest-risk failure modes

1. **The plan can silently downgrade durability.** Phase 1 allows direct publish with a TODO if T1 is not complete. That removes request latency but can lose notifications on a crash after DB commit and before publish. The plan must identify whether that is acceptable per notification type.

2. **`notifications.Publish(ctx, msg)` hides transactional semantics.** A function with the same name can mean enqueue in DB transaction, publish to Redpanda, or fire-and-forget. Call sites need an explicit API shape that makes the chosen durability visible.

3. **Password-reset-style flows are mentioned but not handled.** Some email-like flows may be security-sensitive and time-bound. They may require synchronous confirmation, different retry rules, or exclusion from generic notification fanout.

4. **Duplicate notification behaviour is unspecified.** At-least-once delivery can send duplicate review requests or comments. Without idempotency keys and recipient-level dedupe, the migration can improve latency while degrading user trust.

5. **Template and PII boundaries are not part of the gate.** Message encryption is deferred, but the v1.0 path still needs to prevent sensitive content from landing in audit logs, DLQs, or ntfy topics.

6. **Metrics can show the backend is healthy while user-visible delivery is broken.** Per-backend success counters do not prove SMTP acceptance, recipient routing, or template correctness. The runbook needs synthetic probes or a deterministic test message path.

## Missing decisions before execution

- Classify notification types by durability: transactional, at-least-once, best-effort, or synchronous-required.
- Define idempotency keys and duplicate suppression policy per notification type.
- Define whether notification publishing participates in the same DB transaction as the triggering mutation.
- Define what fields are allowed in logs, audit backend, Redpanda payloads, and DLQ entries.
- Define test delivery probes for mail, ntfy, and audit backends.

## Concrete pre-flight checklist

- Add an audit table with columns: call site, notification type, recipient source, triggering DB mutation, required durability, idempotency key, allowed payload fields, migration path.
- Add crash-window tests for transactional notification types: commit mutation, crash publisher, restart, verify delivery eventually happens exactly once from the user's perspective.
- Add duplicate-event tests proving duplicate publishes do not create duplicate user-visible messages for the same event.
- Add a redaction test for DLQ/log/audit payloads.
- Add a synthetic notification probe to the testing compose stack and document expected metrics/logs.

## Suggested plan edits

- Replace the blanket direct-publish fallback with a per-notification durability matrix.
- Require T1 Phase 1 for any notification coupled to a durable user workflow, such as review requests.
- Add a Phase 1 exit criterion for idempotency and duplicate suppression.
- Add runbook content for duplicate notifications, not only missing notifications.

## Go/no-go gate

Start migration only when every email call site has an explicit delivery class and idempotency key, and the team accepts the crash-window semantics for that class.
