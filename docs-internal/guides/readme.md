---
id: guides-readme
title: "Hermes Internal Guides"
status: Reference
created: 2026-04-26
author: Hermes Team
project_id: hermes
doc_uuid: ae8146c5-d318-46bc-bbd3-544d26d86c3f
type: Guide
tags: [guides, reference, hub]
---

# Hermes Internal Guides

> Book-structured reference docs for setting up, running, and integrating Hermes. Guides describe how to do something step by step. They are not architectural decisions (those live in `adr/`), proposals (`rfc/`), or one-off operational notes (`memo/`).

## When to write a guide vs. a memo

| Situation | Where it goes |
|---|---|
| "How do I set up Dex locally?" — repeatable steps, broad audience, evergreen | **`guides/`** |
| "We hit bug X on 2026-04-20 and fixed it by Y" — incident, dated, narrow audience | **`memo/`** |
| "ember-power-select 8 doesn't work with ember-concurrency 3 — here's why" | **`memo/`** (compatibility note) |
| "Our auth flow has these binding rules" | **`adr/`** |
| "We propose a new search architecture with X, Y, Z" | **`rfc/`** |

## Layout

```
guides/
├── readme.md                       # this file
├── auth/                           # authentication providers and flows
├── deploy/                         # installing and operating a deployment
├── dev/                            # developer environment + workflow
├── edge/                           # edge CLI and MCP client workflows
├── indexer/                        # indexer architecture and operation
├── integrations/                   # third-party integrations (Jira, Ollama, …)
├── models/                         # AI model cards and provenance notes
├── patterns/                       # reusable design patterns
├── search/                         # search backends (Algolia, Meilisearch)
├── setup/                          # installation and setup wizards
├── storage/                        # databases (PostgreSQL, …)
└── workspace/                      # workspace providers (local, …)
```

## Index

### Authentication (`auth/`)

- [Auth Providers Overview](auth/providers.md) — concepts, configuration, provider matrix.
- [Auth Provider Selection Quick Reference](auth/quickref.md) — picking the right provider for an environment.
- [Auth Provider Testing](auth/providers-testing.md) — manual and automated test recipes.
- [Dex Local OIDC](auth/dex.md) — running Dex with the static-password connector for development.
- [Dex Quick Start](auth/dex-quickstart.md) — minimal commands to bring Dex up locally.
- [Google Workspace Setup](auth/google-workspace.md) — credentials, scopes, OAuth client.

### Deployment (`deploy/`)

- [Multi-Domain Deployment](deploy/multi-domain.md) — one process serving several subdomains as isolated tenants, behind nginx: PostgreSQL setup, per-site schemas, systemd, TLS, adding and removing sites.

### Search (`search/`)

- [Algolia Setup](search/algolia.md) — production search backend.
- [Meilisearch Setup](search/meilisearch.md) — local development search backend.
- [Search Outbox Operations](search/outbox-operations.md) — inspect, retry, and skip failed search projection events.

### Developer Workflow (`dev/`)

- [Developer Quick Reference](dev/quickref.md) — common commands and patterns.
- [Environment Setup](dev/env-setup.md) — bootstrap a fresh dev box.
- [Environment Variables](dev/env-vars.md) — required and optional env vars.
- [Ember Development Server](dev/ember-dev-server.md) — dev server, proxy, upgrade strategy.
- [Makefile Targets](dev/makefile-targets.md) — root `Makefile` quick-start commands.
- [AI Prompt Templates](dev/ai-prompt-templates.md) — reusable prompt scaffolds.
- [Playwright Agent Guide](dev/playwright-agent.md) — running E2E tests interactively and headless.

### Edge (`edge/`)

- [Edge CLI Guide](edge/cli.md) — local documentation discovery, validation, repair, sync status, and BM25 search.
- [MCP Client Guide](edge/mcp.md) — configuring `hermes mcp` and using agent tools safely.

### Indexer (`indexer/`)

- [Indexer Overview](indexer/overview.md) — what the indexer does and how to run it.

### Integrations (`integrations/`)

- [Jira Integration](integrations/jira.md)
- [Ollama (Local LLM)](integrations/ollama.md) — local Llama on macOS.

### Models (`models/`)

- [Model Cards](models/readme.md) — provenance and operational notes for AI models used by Hermes.
- [EmbeddingGemma 300M](models/embeddinggemma-300m.md) — local embedding model candidate for Ollama-compatible pipelines.

### Patterns (`patterns/`)

- [Outbox Pattern](patterns/outbox.md) — document tracking and outbox quick reference.

### Setup (`setup/`)

- [Setup Wizard](setup/wizard.md) — zero-config to guided configuration.
- [Setup Wizard: Ollama](setup/ollama.md) — Ollama-specific wizard flow.

### Storage (`storage/`)

- [PostgreSQL Setup](storage/postgresql.md) — primary database setup.

### Workspace (`workspace/`)

- [Local Workspace Provider](workspace/local.md) — local Markdown+YAML workspace setup for testing.
- [Migrate a Project from Google Workspace to S3](workspace/migrate-google-to-s3.md) — v1.0 operator playbook for copy-style storage migrations.

## Authoring

Guides use the same frontmatter conventions as memos but with `type: Guide` and `status` from `{ Draft, Reference, Archived }`. There is no global `guide-NNN` numbering; filenames are topical (e.g. `auth/dex.md`). Cross-reference guides by relative path (`[Dex setup](../guides/auth/dex.md)`).

See [`docs-internal/templates/readme.md`](../templates/readme.md) for the full doc-type matrix and authoring rules.
