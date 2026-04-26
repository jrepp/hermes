---
id: memo-031
title: Docs Internal - Documentation Hub
type: Guide
status: Final
tags: [documentation, index, onboarding]
related: [memo-027, memo-014]
created: 2025-10-09
author: Hermes Team
project_id: hermes
doc_uuid: 41a2dccd-cbc7-4dce-a8ee-6d2a4baf4649
---

# Hermes Documentation Hub

**Welcome!** This directory contains all internal documentation for the Hermes project.

## Quick Start for New Developers

1. **Setup Environment**: Read [MEMO-027: Environment Setup](memo-024-env-setup.md)
2. **Dev Workflows**: Read [MEMO-014: Dev Quick Reference](memo-012-dev-quickref.md)
3. **Authentication**: Read [MEMO-020: Dex Quick Start](memo-017-dex-quickstart.md)
4. **Testing**: Read [MEMO-029: Playwright E2E Guide](memo-026-playwright-agent-guide.md)

## Documentation Structure

### `/adr/` - Architecture Decision Records

**Decision logs** explaining "why we chose X over Y" with context, alternatives, and consequences. 22 ADRs covering frontend, auth, storage, search, and infrastructure decisions.

**Key ADRs**:
- **ADR-009**: Provider Abstraction Architecture (core system design)
- **ADR-012**: Multi-Provider Auth Architecture
- **ADR-014**: Dex Authentication Implementation
- **ADR-016**: Backend-Mediated Search & Runtime Auth Header Selection
- **ADR-017**: API Refactoring & Testing Strategy (V2 handler shape, testcontainers)
- **ADR-020**: Dual Database Support & Stateless Indexer

See [ADR index](../adr/adr-002-readme.md) for complete list.

### `/rfc/` - Request for Comments

**Design proposals** and architecture plans for major features. 18 active RFCs.

**Key RFCs**:
- **RFC-009**: Simplified Local Mode
- **RFC-014**: Event-Driven Indexer
- **RFC-013**: Notification Backend
- **RFC-015**: S3 Storage Backend & Migrations
- **RFC-005**: Outbox Pattern Design

See [RFC index](../rfc/rfc-002-readme.md) for complete list.

### `/memo/` - Implementation Notes & Guides

**Quick reference guides**, implementation summaries, session notes, and reference material.

**Essential Memos**:
- **MEMO-014**: Dev Quick Reference
- **MEMO-020**: Dex Quick Start
- **MEMO-027**: Environment Setup
- **MEMO-029**: Playwright E2E Guide
- **MEMO-030**: Auth Providers Guide

See [Memo index](memo-020-readme.md) for complete list.

### `/plans/` - Work Items

**Non-durable planning artifacts** — tracked work items with priority and status. When completed, knowledge is folded into durable docs (ADRs, RFCs, memos) and the plan is archived.

**Active Plans**:
- **TODO-002**: API Test Suite (29% complete)
- **TODO-003**: Search Provider Migration (71% complete)
- **TODO-005**: Data Consistency (critical)

See [Plans index](../plans/readme.md) for full backlog.

### `/archive/` - Archived Documents

**Completed and superseded documents** retained for reference. Includes archived weekly progress logs, completed plans, and one-time task records.

See [Archive index](../archive/readme.md) for contents.

### `/ember-development-guide/` - Frontend Guide

Comprehensive 8-section guide covering TypeScript setup, component development, service architecture, testing, linting, build, common pitfalls, and migration.

## Finding Documentation

### By Topic

**Authentication**:
- MEMO-020 (Dex Quick Start), MEMO-007 (Auth Provider Quick Ref), MEMO-030 (Auth Providers Guide)
- ADR-008 (Dex OIDC Decision), ADR-012 (Multi-Provider Auth), ADR-013 (Auth Provider Selection)

**Development Workflows**:
- MEMO-014 (Dev Quick Reference), MEMO-027 (Environment Setup)

**Testing**:
- MEMO-029 (Playwright E2E Guide), MEMO-032 (E2E Testing Summary)
- ADR-006 (Testing Docker Compose), ADR-010 (Playwright for Local Iteration)

**Search & Indexing**:
- MEMO-028 (Outbox Pattern Quick Ref)
- RFC-005 (Outbox Design), RFC-014 (Event-Driven Indexer)
- ADR-011 (Meilisearch Decision), ADR-016 (Search Refactoring)

**Frontend (Ember.js)**:
- MEMO-009 (Ember Dev Server)
- ADR-001, ADR-003 (Frontend decisions); MEMO-060, MEMO-061, MEMO-064 (demoted from ADR 036, ADR-006, ADR 029)

**Database & Persistence**:
- RFC-005 (Outbox Design), RFC-008 (Document Sync), RFC-020 (API + testcontainers narrative), RFC-021 (`pkg/docid` narrative), RFC-022 (core+deltas + stateless indexer narrative)
- ADR-019 (Split Server / Migrate Binaries), ADR-020 (Dual DB Support), MEMO-063 (SQLite driver investigation)

### By Document Type

**Quick Start Guides** (read these first):
- MEMO-027, MEMO-014, MEMO-020

**Architecture Documents** (understand the design):
- RFCs in `/rfc/` for proposed designs
- ADRs in `/adr/` for finalized decisions

**Implementation Notes** (how it was built):
- Memos in `/memo/` for guides and summaries

## When to Create What

| Type | When | Durability |
|------|------|------------|
| **ADR** | Finalized decision with long-term impact and alternatives | Durable |
| **RFC** | Design proposal before implementation | Durable |
| **Memo** | Implementation notes, guides, summaries, reference material | Durable |
| **Plan** | Work item, checklist, or progress tracking | Non-durable |

**Lifecycle**: RFC → (implement) → ADR (finalize decision) + Memo (record how). Plans track progress toward implementation; when done, fold into durable docs and archive.

---

**Last Updated**: April 2026
**Maintainer**: Hermes Team