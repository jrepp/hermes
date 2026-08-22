---
id: rfc-002
created: 2026-04-24
author: Hermes Team
project_id: hermes
doc_uuid: d5212400-cb04-45ef-afec-161c50c6f587
status: Draft
title: RFC Documents (Request for Comments)
tags: []
---

# RFC Documents (Request for Comments)

Architecture proposals, design documents, and implementation specifications for major features in Hermes. These describe proposed or in-progress designs — finalized decisions should be recorded as ADRs.

> **Authoring a new RFC?** Start from [`docs-internal/templates/readme.md`](../templates/readme.md) and use [`rfc-template.md`](../templates/rfc-template.md). The guide pins frontmatter conventions and the ADR/RFC split.

## Quick Stats

- **Total RFCs**: 19
- **Scope**: architecture proposals, design documents, feature specifications

## Index by Category

### Core Architecture

| ID | Title | Status | Description |
|----|-------|--------|-------------|
| [083](rfc-009-simplified-local-mode.md) | Simplified Local Mode | Proposed | Zero-config single-binary CMS with embedded database and search |
| [084](rfc-010-provider-interface-refactoring.md) | Provider Interface Refactoring | In Progress | Multi-backend provider interface refactoring |
| [085](rfc-011-api-provider-remote-delegation.md) | API Provider Remote Delegation | In Progress | Multi-provider architecture with pass-through, UUID merging |
| [088](rfc-014-event-driven-indexer.md) | Event-Driven Indexer | In Progress | Event-driven document indexer with pipeline rulesets |

### Data & Storage

| ID | Title | Status | Description |
|----|-------|--------|-------------|
| [051](rfc-005-outbox-pattern-design.md) | Outbox Pattern Design | Design Phase | Async search index updates with transactional consistency |
| [080](rfc-008-outbox-pattern-document-sync.md) | Outbox Pattern Document Sync | Accepted | Transactional search outbox pattern for DB/search consistency |
| [089](rfc-015-s3-storage-backend-and-migrations.md) | S3 Storage Backend & Migrations | Accepted | S3 storage backend and narrowed v1.0 migration API/router contract |
| [091](rfc-017-document-revisions-and-migration.md) | Document Revisions & Migration | Design Phase | Provider-project-document-revision model |
| [092](rfc-018-instance-identity.md) | Instance Identity | Approved | Composite instance + project identity in distributed system |
| [023](rfc-023-multi-domain-hosting-and-signed-sessions.md) | Multi-Domain Hosting & Signed Sessions | In Progress | One listener serving many subdomains as isolated tenants, with HMAC-signed domain-bound sessions |

### Indexing

| ID | Title | Status | Description |
|----|-------|--------|-------------|
| [003](rfc-003-rfc-indexer-architecture.md) | Indexer Architecture | Partially Implemented | Distributed document discovery/indexing architecture |
| [004](rfc-004-rfc-indexer-architecture.md) | Indexer Architecture (Consolidated) | Partially Implemented | Consolidated indexer architecture design |

### Notifications

| ID | Title | Status | Description |
|----|-------|--------|-------------|
| [086](rfc-012-authentication-bearer-tokens.md) | Authentication Bearer Tokens | Design Phase | Bearer token management for delegated operations |
| [087](rfc-013-notification-backend.md) | Notification Backend | In Progress | Multi-backend notification system with Redpanda message queues |

### Features

| ID | Title | Status | Description |
|----|-------|--------|-------------|
| [001](rfc-001-local-developer-mode-with-central-hermes.md) | Local Developer Mode | Draft | Local dev mode with SQLite + bidirectional sync |
| [078](rfc-006-new-document-types.md) | New Document Types | Proposed | ADR, Memo, FRD, PATH document types |
| [079](rfc-007-local-editor-e2e-testing.md) | Local Editor E2E Testing | Proposed | In-browser editor for local dev E2E testing |
| [090](rfc-016-admin-interface.md) | Admin Interface | Proposed | Comprehensive admin UI |
| [093](rfc-019-projectconfig-integration.md) | ProjectConfig Integration | Planning | Integrating pkg/projectconfig into server/API layer |

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
