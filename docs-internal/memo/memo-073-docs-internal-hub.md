---
id: memo-073
title: "Docs Internal - Documentation Hub"
date: 2025-10-09
type: Guide
status: Final
tags: [documentation, index, onboarding]
related:
  - memo-035
  - memo-017
created: 2025-10-09
author: Hermes Team
project_id: hermes
doc_uuid: 41a2dccd-cbc7-4dce-a8ee-6d2a4baf4649
---

# Hermes Documentation Hub

**Welcome!** This directory contains all internal documentation for the Hermes project.

## Quick Start for New Developers

1. **Setup Environment**: Read [MEMO-035: Environment Setup](memo-035-env-setup.md)
2. **Dev Workflows**: Read [MEMO-017: Dev Quick Reference](memo-017-dev-quickref.md)
3. **Authentication**: Read [MEMO-023: Dex Quick Start](memo-023-dex-quickstart.md)
4. **Testing**: Read [MEMO-058: Playwright E2E Guide](memo-058-playwright-agent-guide.md)

## Documentation Structure

### `/adr/` - Architecture Decision Records

**Decision logs** explaining "why we chose X over Y" with context, alternatives, and consequences. 24 ADRs covering frontend, auth, storage, search, and infrastructure decisions.

**Key ADRs**:
- **ADR-073**: Provider Abstraction Architecture (core system design)
- **ADR-076**: Multi-Provider Auth Architecture
- **ADR-078**: Dex Authentication Implementation
- **ADR-080**: Search & Auth Refactoring
- **ADR-085**: Dual Database Support & Stateless Indexer

See [ADR index](../adr/adr-003-readme.md) for complete list.

### `/rfc/` - Request for Comments

**Design proposals** and architecture plans for major features. 18 active RFCs.

**Key RFCs**:
- **RFC-083**: Simplified Local Mode
- **RFC-088**: Event-Driven Indexer
- **RFC-087**: Notification Backend
- **RFC-089**: S3 Storage Backend & Migrations
- **RFC-051**: Outbox Pattern Design

See [RFC index](../rfc/rfc-002-readme.md) for complete list.

### `/memo/` - Implementation Notes & Guides

**Quick reference guides**, implementation summaries, session notes, and reference material.

**Essential Memos**:
- **MEMO-017**: Dev Quick Reference
- **MEMO-023**: Dex Quick Start
- **MEMO-035**: Environment Setup
- **MEMO-058**: Playwright E2E Guide
- **MEMO-071**: Auth Providers Guide

See [Memo index](memo-026-readme.md) for complete list.

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
- MEMO-023 (Dex Quick Start), MEMO-008 (Auth Provider Quick Ref), MEMO-071 (Auth Providers Guide)
- ADR-072 (Dex OIDC Decision), ADR-076 (Multi-Provider Auth), ADR-077 (Auth Provider Selection)

**Development Workflows**:
- MEMO-017 (Dev Quick Reference), MEMO-035 (Environment Setup)

**Testing**:
- MEMO-058 (Playwright E2E Guide), MEMO-092 (E2E Testing Summary)
- ADR-070 (Testing Docker Compose), ADR-074 (Playwright for Local Iteration)

**Search & Indexing**:
- MEMO-052 (Outbox Pattern Quick Ref)
- RFC-051 (Outbox Design), RFC-088 (Event-Driven Indexer)
- ADR-075 (Meilisearch Decision), ADR-080 (Search Refactoring)

**Frontend (Ember.js)**:
- MEMO-010 (Ember Dev Server)
- ADR-001, ADR-006, ADR-029, ADR-032, ADR-036 (Frontend decisions)

**Database & Persistence**:
- RFC-051 (Outbox Design), RFC-080 (Document Sync)
- ADR-083 (SQLite Conflict), ADR-085 (Dual DB Support)

### By Document Type

**Quick Start Guides** (read these first):
- MEMO-035, MEMO-017, MEMO-023

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
