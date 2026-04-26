---
id: rfc-002
created: 2026-04-24
author: Hermes Team
project_id: hermes
doc_uuid: d5212400-cb04-45ef-afec-161c50c6f587
status: Draft
title: "RFC Documents (Request for Comments)"
---

# RFC Documents (Request for Comments)

Architecture proposals, design documents, and implementation specifications for major features in Hermes. These describe proposed or in-progress designs — finalized decisions should be recorded as ADRs.

## Quick Stats

- **Total RFCs**: 18
- **Scope**: architecture proposals, design documents, feature specifications

## Index by Category

### Core Architecture

| ID | Title | Status | Description |
|----|-------|--------|-------------|
| [083](rfc-083-simplified-local-mode.md) | Simplified Local Mode | Proposed | Zero-config single-binary CMS with embedded database and search |
| [084](rfc-084-provider-interface-refactoring.md) | Provider Interface Refactoring | In Progress | Multi-backend provider interface refactoring |
| [085](rfc-085-api-provider-remote-delegation.md) | API Provider Remote Delegation | In Progress | Multi-provider architecture with pass-through, UUID merging |
| [088](rfc-088-event-driven-indexer.md) | Event-Driven Indexer | In Progress | Event-driven document indexer with pipeline rulesets |

### Data & Storage

| ID | Title | Status | Description |
|----|-------|--------|-------------|
| [051](rfc-051-outbox-pattern-design.md) | Outbox Pattern Design | Design Phase | Async search index updates with transactional consistency |
| [080](rfc-080-outbox-pattern-document-sync.md) | Outbox Pattern Document Sync | Design Phase | Transactional outbox pattern for DB/search consistency |
| [089](rfc-089-s3-storage-backend-and-migrations.md) | S3 Storage Backend & Migrations | In Progress | S3 storage backend, document migration between providers |
| [091](rfc-091-document-revisions-and-migration.md) | Document Revisions & Migration | Design Phase | Provider-project-document-revision model |
| [092](rfc-092-instance-identity.md) | Instance Identity | Approved | Composite instance + project identity in distributed system |

### Indexing

| ID | Title | Status | Description |
|----|-------|--------|-------------|
| [003](rfc-003-rfc-indexer-architecture.md) | Indexer Architecture | Partially Implemented | Distributed document discovery/indexing architecture |
| [004](rfc-004-rfc-indexer-architecture.md) | Indexer Architecture (Consolidated) | Partially Implemented | Consolidated indexer architecture design |

### Notifications

| ID | Title | Status | Description |
|----|-------|--------|-------------|
| [086](rfc-086-authentication-bearer-tokens.md) | Authentication Bearer Tokens | Design Phase | Bearer token management for delegated operations |
| [087](rfc-087-notification-backend.md) | Notification Backend | In Progress | Multi-backend notification system with Redpanda message queues |

### Features

| ID | Title | Status | Description |
|----|-------|--------|-------------|
| [001](rfc-001-local-developer-mode-with-central-hermes.md) | Local Developer Mode | Draft | Local dev mode with SQLite + bidirectional sync |
| [078](rfc-078-new-document-types.md) | New Document Types | Proposed | ADR, Memo, FRD, PATH document types |
| [079](rfc-079-local-editor-e2e-testing.md) | Local Editor E2E Testing | Proposed | In-browser editor for local dev E2E testing |
| [090](rfc-090-admin-interface.md) | Admin Interface | Proposed | Comprehensive admin UI |
| [093](rfc-093-projectconfig-integration.md) | ProjectConfig Integration | Planning | Integrating pkg/projectconfig into server/API layer |

## Document Format

Each RFC follows this structure:

```markdown
# RFC-NNN: Title

**Status**: Design Phase | In Progress | Implemented | Superseded
**Date**: YYYY-MM-DD
**Related**: Links to related RFCs/ADRs

## Context
## Proposal/Solution
## Benefits
## Implementation Status
## References
```

## Contributing

1. Assign next sequential ID (NNN format)
2. Use descriptive kebab-case filename: `rfc-NNN-description.md`
3. Include context, proposal, benefits, implementation plan
4. Link related RFCs and ADRs
5. Update this README index
6. When a proposal is finalized and implemented, promote to ADR

## Relationship to ADRs

RFCs describe proposed designs. Once a design is implemented and the decision is finalized, it should be recorded as an ADR. The RFC may remain as reference, but the ADR is the authoritative record of the decision.
