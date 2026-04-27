---
id: trajectory-004
title: "Trajectory T4 — Storage & Migration Surface"
status: Draft
created: 2026-04-27
date: 2026-04-27
author: Hermes Team
project_id: hermes
doc_uuid: ba79445f-e3f9-4bc2-9062-2f7cd6b3d6b8
type: Memo
subtype: Milestone
tags: [trajectory, roadmap, v1.0, storage, s3, migration, workspace-provider]
related:
  - ADR-007
  - ADR-009
  - ADR-015
  - ADR-017
  - ADR-018
  - ADR-019
  - RFC-015
  - MEMO-059
---

# Trajectory T4 — Storage & Migration Surface

> RFC-015 has the S3 adapter, migration schema, and worker foundation in place. v1.0 needs a deliberately small REST surface, provider router, and operator runbook so admins can start, inspect, retry, and stop copy-style migrations without editing database rows.

## Stable-release criteria served

- v1.0 criterion **#4 Storage** (one non-Google workspace provider end-to-end).
- v1.0 criterion **#5 Provider abstraction complete** (workspace side).

## Scope

In scope:

- REST API: `POST /api/v2/migrations/jobs`, `GET /api/v2/migrations/jobs/:id`, `GET /api/v2/migrations/jobs/:id/items`, `POST /api/v2/migrations/jobs/:id/retry`, `GET /api/v2/providers`.
- REST API safety verb: `POST /api/v2/migrations/jobs/:id/cancel` for jobs that have not completed. Rollback remains out of scope; cancellation is best-effort and never deletes already-written destination objects.
- Provider router (`pkg/workspace/router/`) wired into the v2 handler chain so reads land at the correct backend.
- E2E test that drives a full copy migration through the API (not just the worker).
- Operator runbook draft + example HCL.

Out of scope (deferred to v1.x):

- DynamoDB metadata storage strategy.
- Cross-region replication.
- Admin UI for migration management (T7).
- Multi-writable storage, automatic mirroring, and conflict-resolution policy.
- Rollback of already-copied destination objects.
- Scheduled or recurring migrations.

## Dependencies

- RFC-015 Phase 1 (worker + schema) — done.
- ADR-019 (split server / migrate binaries) — done.

## Phases & exit criteria

### Phase 0 — API and safety contract freeze

- Document request/response shapes for the five migration endpoints plus `GET /api/v2/providers`.
- Freeze v1.0 authorization as **admin-only** for create, retry, cancel, and provider listing. Project-owner delegation is deferred to v1.x and must go through RFC-012 or a successor.
- Freeze the migration state machine and allowed transitions:
  - Job states: `pending`, `running`, `cancelling`, `cancelled`, `completed`, `failed`.
  - Item states: `pending`, `in_progress`, `completed`, `failed`, `skipped`.
  - Retry is allowed only for failed retryable items on non-terminal jobs; retrying completed items returns `409`.
  - Cancel is allowed for `pending` and `running` jobs; completed and failed jobs are terminal for v1.0.
- Freeze provider-router read/write semantics during migration:
  - Reads use the canonical provider recorded for the document/project unless the request names a specific provider-qualified `docid.CompositeID`.
  - Copy migrations do not change the canonical provider or route writes to the destination.
  - Cutover is a separate explicit operation and is not part of T4 v1.0.
- Define v1.0 invariants: `docid.UUID`, provider mapping, project, title, document type, status, content hash, and migration audit timestamps must be validated; permissions, comments, and review state must be either preserved by the provider pair or reported as lossy in the job result.
- Define idempotency for job creation, retry, and cancellation with an `Idempotency-Key` header scoped by project, source provider, destination provider, and filter body.
- Draft the operator runbook before handler implementation and use it to validate the API contract.

**Exit when:**

- API contract, state machine, router semantics, authz model, idempotency rules, and invariant list are committed in [RFC-015](../rfc/rfc-015-s3-storage-backend-and-migrations.md).
- The [T4 adversarial review](trajectory-004-adversarial-review.md) go/no-go gate is satisfied: an operator can start, observe, retry, and stop a migration without database edits.

### Phase 1 — Handlers + router

- Handlers implement the single-parameter form (ADR-017) and use typed errors.
- Provider router selects backend per `provider_storage` row.
- Handlers enforce the admin-only model and project scoping.
- Unit + integration tests in `tests/integration/migration/` cover unauthorized users, cross-project access, duplicate job creation, retry of succeeded items, retry of failed items, cancel of pending/running jobs, and partial provider outage.

**Exit when:**

- Existing migration E2E test suite still green; new API E2E test passes.
- Targeted package tests for migration handlers, manager, and router pass. Run repo-wide lint only if it is already green or scoped by CI to changed packages.

### Phase 2 — Local-filesystem provider parity

- Confirm `pkg/workspace/local/` participates in migrations the same way S3 and Google do.
- E2E test: migrate a test corpus from Google → local → S3 with full content integrity and explicit reports for any provider-specific metadata that cannot be preserved.
- Router tests cover reads and writes while a migration item is `in_progress`.

**Exit when:**

- Three-hop migration test green in CI.
- Invariant assertions cover document UUID, provider ID mapping, project, title, document type, status, timestamps, content hash, and lossy metadata reporting.
- TODO-010 (legacy Algolia products) closed if not already.

### Phase 3 — Documentation & cutover readiness

- Finalize runbook: "How to migrate a project from Google to S3" under `docs-internal/guides/`.
- Example HCL committed under `testing/` and `configs/`.

**Exit when:**

- Runbook reviewed; linked from RFC-015 and MEMO-059.
- RFC-015 status can move to `Implemented` only for the v1.0 API/router scope. Admin UI, multi-writable storage, recurring migrations, and rollback remain explicitly deferred to T7/v1.x.

## Reviews / gates

Phase exit-only. Phase 1 must not begin until Phase 0 freezes the API contract and satisfies the adversarial-review go/no-go gate. T4 may ship Phase 0–2 for v1.0 and complete Phase 3 polish in a 1.0.x patch if needed.

## Risks

- **API auth model debate** stalls Phase 0. Mitigation: default to admin-only for v1.0; revisit per-project delegation in v1.x via RFC-012.
- **Provider router latency** affects hot-path reads. Mitigation: bench in Phase 1; cache provider lookups per request.

## References

- [RFC-015: S3 Storage Backend and Migrations](../rfc/rfc-015-s3-storage-backend-and-migrations.md)
- [ADR-007 / ADR-015](../adr/) — workspace provider abstraction.
- [ADR-019: Split Server and Migrate Binaries](../adr/adr-019-split-server-and-migrate-binaries.md)
- [MEMO-059: S3 Storage Implementation Summary](../memo/memo-059-s3-storage-implementation-summary.md)
- [Migrate a Project from Google Workspace to S3](../guides/workspace/migrate-google-to-s3.md)
- [Roadmap Implementation Tracker](roadmap-tracker.md)
