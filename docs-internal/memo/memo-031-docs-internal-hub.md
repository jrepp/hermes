---
id: memo-031
title: "Docs-Internal Cross-Cutting Hub"
type: Memo
subtype: Reference
status: Reference
tags: [documentation, index, onboarding, hub]
related: [memo-023]
created: 2025-10-09
author: Hermes Team
project_id: hermes
doc_uuid: 41a2dccd-cbc7-4dce-a8ee-6d2a4baf4649
---

# Docs-Internal Cross-Cutting Hub

> Topic-oriented entry point into Hermes internal documentation. For type-oriented indexes, see the per-folder readmes (`adr/`, `rfc/`, `memo/`, `guides/`).

## Quick Start for New Developers

1. **Set up environment** → [`guides/dev/env-setup.md`](../guides/dev/env-setup.md)
2. **Daily commands** → [`guides/dev/quickref.md`](../guides/dev/quickref.md)
3. **Local auth (Dex)** → [`guides/auth/dex-quickstart.md`](../guides/auth/dex-quickstart.md)
4. **E2E testing** → [`guides/dev/playwright-agent.md`](../guides/dev/playwright-agent.md)

## Core Indexes

- [ADR index](../adr/adr-002-readme.md) — binding architectural decisions
- [RFC index](../rfc/rfc-002-readme.md) — design proposals
- [Guides index](../guides/readme.md) — evergreen step-by-step references
- [Memo index](memo-023-readme.md) — investigations, milestones, demoted ADRs
- [Plans index](../plans/readme.md) — non-durable work items
- [Templates & authoring](../templates/readme.md) — start here when writing any new doc

## Finding Documentation by Topic

### Authentication

- Setup: [`guides/auth/dex-quickstart.md`](../guides/auth/dex-quickstart.md), [`guides/auth/dex.md`](../guides/auth/dex.md), [`guides/auth/google-workspace.md`](../guides/auth/google-workspace.md)
- Selection / matrix: [`guides/auth/providers.md`](../guides/auth/providers.md), [`guides/auth/quickref.md`](../guides/auth/quickref.md)
- Decisions: [ADR-008](../adr/adr-008-dex-oidc-authentication-for-development.md) (Dex OIDC), [ADR-012](../adr/adr-012-multi-provider-auth-architecture.md) (Multi-Provider Auth), [ADR-013](../adr/adr-013-auth-provider-selection.md) (Provider Selection), [ADR-014](../adr/adr-014-dex-authentication-implementation.md) (Dex implementation)
- Reference: [memo-062](memo-062-multi-provider-auth-diagrams.md) (auth diagrams)

### Developer Workflows

- [`guides/dev/quickref.md`](../guides/dev/quickref.md), [`guides/dev/env-setup.md`](../guides/dev/env-setup.md), [`guides/dev/env-vars.md`](../guides/dev/env-vars.md), [`guides/dev/makefile-targets.md`](../guides/dev/makefile-targets.md), [`guides/dev/ember-dev-server.md`](../guides/dev/ember-dev-server.md)

### Testing

- Guide: [`guides/dev/playwright-agent.md`](../guides/dev/playwright-agent.md)
- Decisions: [ADR-006](../adr/adr-006-testing-docker-compose-environment.md) (Testing Docker Compose), [ADR-010](../adr/adr-010-playwright-for-local-iteration.md) (Playwright)
- Memo: [memo-032](memo-032-e2e-testing-summary.md) (E2E testing summary)

### Search & Indexing

- Setup: [`guides/search/algolia.md`](../guides/search/algolia.md), [`guides/search/meilisearch.md`](../guides/search/meilisearch.md), [`guides/indexer/overview.md`](../guides/indexer/overview.md)
- Patterns: [`guides/patterns/outbox.md`](../guides/patterns/outbox.md)
- Decisions: [ADR-011](../adr/adr-011-meilisearch-as-local-search-solution.md) (Meilisearch), [ADR-016](../adr/adr-016-search-and-auth-refactoring.md) (Search refactoring)
- Proposals: [RFC-005](../rfc/rfc-005-outbox-pattern-design.md) (Outbox), [RFC-014](../rfc/rfc-014-event-driven-indexer.md) (Event-driven indexer)

### Frontend (Ember.js)

- Guide: [`guides/dev/ember-dev-server.md`](../guides/dev/ember-dev-server.md)
- Decisions: [ADR-001](../adr/adr-001-stay-with-classic-ember-build.md), [ADR-003](../adr/adr-003-direct-fetch-for-singleton-endpoints.md), [ADR-005](../adr/adr-005-frontend-async-timeout-policy.md)
- Demoted-ADR memos: [memo-060](memo-060-fix-ember-location-type.md) (locationType), [memo-061](memo-061-ember-animated-stub-components.md) (ember-animated stubs), [memo-064](memo-064-ember-concurrency-power-select-incompatibility.md) (ember-concurrency / ember-power-select)

### Database & Persistence

- Setup: [`guides/storage/postgresql.md`](../guides/storage/postgresql.md)
- Decisions: [ADR-019](../adr/adr-019-split-server-and-migrate-binaries.md) (Split binaries), [ADR-020](../adr/adr-020-dual-database-support-stateless-indexer.md) (Dual DB)
- Proposals: [RFC-005](../rfc/rfc-005-outbox-pattern-design.md) (Outbox), [RFC-021](../rfc/rfc-021-document-identification-system.md) (`pkg/docid`), [RFC-022](../rfc/rfc-022-database-deltas-and-stateless-indexer.md) (core+deltas)
- Investigation: [memo-063](memo-063-sqlite-driver-conflict-investigation.md) (SQLite driver conflict)

### Workspace Providers

- Setup: [`guides/workspace/local.md`](../guides/workspace/local.md)
- Decisions: [ADR-007](../adr/adr-007-local-file-workspace-system.md) (Local file workspace), [ADR-009](../adr/adr-009-provider-abstraction-architecture.md) (Provider abstraction)

### Integrations

- [`guides/integrations/jira.md`](../guides/integrations/jira.md), [`guides/integrations/ollama.md`](../guides/integrations/ollama.md)

### Setup Wizard

- [`guides/setup/wizard.md`](../guides/setup/wizard.md), [`guides/setup/ollama.md`](../guides/setup/ollama.md)

### AI Agents (meta)

- Analysis: [memo-004](memo-004-agent-usage-analysis.md), [memo-005](memo-005-ai-agent-capabilities-and-limitations.md), [memo-006](memo-006-human-enablement-patterns.md), [memo-008](memo-008-ai-session-playbook.md), [memo-016](memo-016-dev-velocity-analysis.md)
- Templates: [`guides/dev/ai-prompt-templates.md`](../guides/dev/ai-prompt-templates.md)

## When to Create What

| Type | When | Durability |
|------|------|------------|
| **ADR** | Binding decision with long-term force | Durable |
| **RFC** | Design proposal before / during implementation | Durable |
| **Guide** | Evergreen step-by-step setup or workflow content | Durable |
| **Memo** | Dated narrative — investigation, milestone, post-mortem, demoted ADR | Durable |
| **Plan** | Work item, checklist, or progress tracking | Non-durable |

**Lifecycle**: RFC → (implement) → ADR (finalize decision) + Guide (record how to set up) + Memo (record what we learned). Plans track progress; when done, fold knowledge into durable docs and archive.

See [`templates/readme.md`](../templates/readme.md) for the full doc-type matrix and authoring rules.
