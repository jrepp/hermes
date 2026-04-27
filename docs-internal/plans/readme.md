# Plans Directory

This directory contains non-durable planning artifacts for the Hermes project. There are two kinds:

1. **Trajectory plans (`trajectory-NNN-*.md`)** — durable, phased completion plans tied to the v1.0 roadmap defined in [`/ROADMAP.md`](../../ROADMAP.md). Each trajectory groups a body of work with explicit phase exit criteria. These stay in place across the v1.0 release; only their internal status changes.
2. **TODO plans (`todo-NNN-*.md`)** — narrower, single-feature work items. Each TODO is owned by exactly one trajectory. When a TODO completes, fold its durable knowledge into the relevant ADR/RFC/memo and archive the TODO under `docs-internal/archive/`.

## v1.0 Trajectories

The detailed implementation tracker is [`roadmap-tracker.md`](roadmap-tracker.md) (formerly MEMO-003).

| ID | Trajectory | v1.0? | Owns these TODOs |
|---|---|---|---|
| [T1](trajectory-001-data-consistency-outbox.md) | Data Consistency & Outbox | yes | TODO-005, parts of TODO-003 |
| [T2](trajectory-002-indexer-cutover.md) | Event-Driven Indexer Cutover | yes | (RFC-014 driven) |
| [T3](trajectory-003-notifications-hardening.md) | Notifications Hardening | yes | TODO-004 |
| [T4](trajectory-004-storage-migration.md) | Storage & Migration Surface | yes | TODO-010 |
| [T5](trajectory-005-local-mode.md) | Local Mode & Workspace Abstraction | yes | (RFC-009 driven) |
| [T6](trajectory-006-e2e-and-tech-debt.md) | E2E Coverage & Tech Debt | yes | TODO-001, TODO-002, TODO-011/012/013, `database_migration_fix_session.md`, `indexer-refactor.md` |
| [T7](trajectory-007-federation-and-admin.md) | Federation & Admin UI | **no — v1.x** | TODO-007, TODO-008, TODO-009 (deferred) |

### Adversarial Reviews

Read these before running a trajectory. They list failure modes, missing decisions, and go/no-go gates intended to harden each plan before implementation starts.

- [T1 adversarial review](trajectory-001-adversarial-review.md)
- [T2 adversarial review](trajectory-002-adversarial-review.md)
- [T3 adversarial review](trajectory-003-adversarial-review.md)
- [T4 adversarial review](trajectory-004-adversarial-review.md)
- [T5 adversarial review](trajectory-005-adversarial-review.md)
- [T6 adversarial review](trajectory-006-adversarial-review.md)
- [T7 adversarial review](trajectory-007-adversarial-review.md)

## Naming Convention

- **Trajectories:** `trajectory-NNN-short-description.md` — frontmatter follows the memo schema (`type: Memo`, `subtype: Milestone`).
- **TODOs:** `todo-NNN-short-description.md` — frontmatter as below.

## TODO Front Matter Format

```yaml
---
id: TODO-NNN
title: Full Title of the Plan
date: YYYY-MM-DD
type: TODO
priority: low|medium|high|critical
status: open|in-progress|blocked|completed
progress: N%
tags: [tag1, tag2, tag3]
related:
  - ADR-XXX
  - RFC-YYY
  - MEMO-ZZZ
---
```

## Current Plans

### TODO-001: Add Compile-Time Interface Checks for Abstractions
- **Priority**: Medium | **Status**: Open
- Add compile-time checks for workspace and search provider interfaces

### TODO-002: Build Comprehensive API Test Suite
- **Priority**: High | **Status**: In Progress (29%)
- Build complete integration test coverage for v1 and v2 APIs

### TODO-003: Migrate API Handlers to Search Provider Abstraction
- **Priority**: High | **Status**: In Progress (71%)
- Migrate all handlers from direct Algolia calls to SearchProvider abstraction

### TODO-004: Implement Asynchronous Email Sending
- **Priority**: High | **Status**: Open
- Move email sending to background workers to avoid blocking HTTP responses

### TODO-005: Fix Data Consistency Between Search Index and Database
- **Priority**: Critical | **Status**: Open
- Implement outbox pattern to ensure consistency between search and database

### TODO-007: Improve TypeScript Type Safety Across Codebase
- **Priority**: Medium | **Status**: Open
- Replace `any` types and complete HDS type definitions

### TODO-008: Make Configuration Values Configurable
- **Priority**: Low | **Status**: Open
- Move hardcoded values (ports, timeouts, etc.) to configuration

### TODO-009: Implement Document Template System
- **Priority**: Medium | **Status**: Open
- Create template system for emails, document headers, and custom fields

### TODO-010: Remove Legacy Algolia Products Requirement
- **Priority**: Low | **Status**: Open
- Remove products from Algolia indexing once fully migrated to database

### TODO-011: E2E Test 'Awaiting Review' Dashboard View
- **Priority**: High | **Status**: Open
- Test that documents awaiting user's review appear correctly in dashboard

### TODO-012: E2E Test 'Latest Docs' Dashboard View
- **Priority**: High | **Status**: Open
- Test that latest published documents appear chronologically in dashboard

### TODO-013: E2E Test 'Recently Viewed' Sidebar
- **Priority**: High | **Status**: Open
- Test that recently viewed documents appear correctly in dashboard sidebar

### Unnumbered Plans

- **database_migration_fix_session.md** - GORM AutoMigrate constraint renaming bug workaround notes
- **indexer-refactor.md** - Detailed phased checklist for indexer refactoring (most phases complete)

## Archived Plans

Completed or superseded plans are moved to `docs-internal/archive/`:
- TODO-006 (superseded by TODO-014, then completed)
- TODO-014 (completed)
- Various local workflow and integration test completion summaries

## Lifecycle

1. **Create**: New plan with next sequential ID
2. **Track**: Update progress and status as work proceeds
3. **Complete**: When done, fold durable knowledge into ADR/RFC/memo
4. **Archive**: Move completed plan to `docs-internal/archive/`

## Related Documentation

- **ADRs**: `docs-internal/adr/` — Finalized architectural decisions
- **RFCs**: `docs-internal/rfc/` — Design proposals and architecture plans
- **Memos**: `docs-internal/memo/` — Implementation notes and guides
- **Archive**: `docs-internal/archive/` — Completed and superseded items
