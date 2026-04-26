# Plans Directory

This directory contains non-durable planning artifacts for the Hermes project. Plans are work items that track progress toward implementation. When a plan is completed, its knowledge should be folded into durable documentation (ADRs, RFCs, or memos), and the plan itself should be archived.

## Naming Convention

All plan files follow the pattern: `todo-NNN-short-description.md`

- **NNN**: Zero-padded 3-digit sequential number (001, 002, 003, etc.)
- **short-description**: Kebab-case brief description of the plan

## Front Matter Format

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
