---
id: trajectory-004-adversarial-review
title: "Adversarial Review — T4 Storage & Migration Surface"
status: Draft
created: 2026-04-27
date: 2026-04-27
author: Hermes Team
project_id: hermes
doc_uuid: 7de54a3a-46a6-4ced-8e51-d600f40adb4b
type: Memo
subtype: Analysis
tags: [trajectory, adversarial-review, v1.0, storage, migration, workspace-provider]
related:
  - ADR-009
  - ADR-017
  - ADR-019
  - RFC-015
---

# Adversarial Review — T4 Storage & Migration Surface

> This review tries to break [Trajectory T4](trajectory-004-storage-migration.md) before execution. The missing REST surface is the right next step, but authz, migration state-machine semantics, and router behaviour must be frozen before handlers are implemented.

## Run-readiness verdict

Do not start Phase 1 handlers until Phase 0 defines the migration state machine, authorization model, and provider-router read/write semantics. Otherwise the API may expose irreversible migration operations without clear safety boundaries.

## Highest-risk failure modes

1. **The auth model is treated as a design detail, but it controls data movement.** `POST /api/v2/migrations/jobs` can copy or expose whole projects. Defaulting to admin-only is sensible for v1.0, but the plan needs explicit authorization checks and tests before any endpoint lands.

2. **The migration API can be too CRUD-shaped for a workflow.** Create, get, list items, and retry are not enough if jobs need pause, cancel, resume, dry-run, validation-only, or rollback semantics. Missing lifecycle verbs can force operators back to database edits.

3. **Provider-router behaviour is under-specified for concurrent migration.** During migration, a document may exist in source and destination. The router needs a deterministic rule for reads, writes, conflict handling, and cutover timing.

4. **The three-hop Google → local → S3 test may hide the most dangerous case.** Content integrity can pass while permissions, metadata, comments, review state, document identity, and revision history are lost or altered.

5. **The plan risks violating provider abstraction boundaries.** [ADR-009](../adr/adr-009-provider-abstraction-architecture.md) requires external integrations through provider interfaces. Handlers should not special-case S3 or local filesystem behaviour outside provider/router abstractions.

6. **`golangci-lint clean` is not a meaningful exit criterion if the project does not already enforce it.** It can turn a migration API trajectory into a repo-wide lint cleanup.

7. **Documentation is too late for a dangerous operator workflow.** The runbook should exist before or during API design so it drives the endpoint contract, not after implementation.

## Missing decisions before execution

- Define the migration job state machine and allowed transitions.
- Define authorization: who can create, inspect, retry, cancel, and cut over migrations.
- Define router rules for source/destination reads and writes during migration.
- Define rollback or compensation semantics after partial migration.
- Define what content and metadata invariants must survive migration.
- Define idempotency keys for job creation and item retry.

## Concrete pre-flight checklist

- Add an API contract table with endpoint, auth role, idempotency behaviour, request, response, errors, and state transition.
- Add a migration state diagram before writing handlers.
- Add tests for unauthorized users, cross-project access, duplicate job creation, retry of succeeded items, retry of failed items, and partial provider outage.
- Add invariant tests for document UUID, provider ID mapping, title, type, status, timestamps, permissions, review state, and content hash.
- Add a router test for reads and writes while an item is mid-migration.

## Suggested plan edits

- Make admin-only authorization explicit for v1.0 and defer project-owner delegation to v1.x.
- Add `cancel` or explicitly document why cancellation is unsupported for v1.0.
- Replace `golangci-lint clean` with targeted package/test commands unless lint is already green repo-wide.
- Move the operator runbook draft into Phase 0 and use it to validate the API surface.

## Go/no-go gate

Start handlers only when an operator can read the API contract and know how to safely start, observe, retry, and stop a migration without touching the database.
