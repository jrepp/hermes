---
id: edge-mcp-guide
title: "MCP Client Guide"
status: Reference
created: 2026-05-01
author: Hermes Team
project_id: hermes
doc_uuid: d2c897e1-1331-46eb-a4a9-8d4b28ff7ebb
type: Guide
subtype: Integration
tags: [edge, mcp, agents, cli]
related:
  - ADR-009
  - ADR-016
  - ADR-018
  - ADR-021
---

# MCP Client Guide

> Use `hermes mcp` to expose project discovery, document reads, validation, local search, repair planning, and sync status to agent clients over stdio or streamable HTTP.

## Start The Server

Stdio is the default transport:

```bash
hermes mcp -projects-config ./testing/projects.hcl -root .
```

Use streamable HTTP when a client supports it:

```bash
hermes mcp -transport http -http-addr 127.0.0.1:8060 -endpoint /mcp -projects-config ./testing/projects.hcl -root .
```

The HTTP endpoint is `/mcp` by default.

## Tool Overview

`project`: HCL-backed project inspection.

Actions: `list`, `discover`, `get`, `providers`, `lanes`.

`context`: compact repository/project context.

Actions: `summary`, `project`, `document`, `search_context`, `validate`, `calls`, `initialize_events`.

`document`: local document discovery, reads, validation, link checks, and guarded content updates.

Actions: `list`, `get`, `content`, `validate`, `links`, `update`.

`search`: local edge search.

Actions: `query`, `hybrid`, `index_status`, `similar`.

`repair`: dry-run-first documentation repair and migration.

Actions: `plan`, `apply`, `migrate`, `bulk_update`, `timestamps`, `compress_ids`, `links`.

`sync`: read-only edge sync status and plans.

Actions: `status`, `plan_pull`, `plan_push`, `pull`, `push`.

`pull` and `push` currently return safe not-implemented errors. Use `plan_pull` and `plan_push` for dry-run status.

## Response Detail

Document responses accept `detail`:

`full`: include all available fields and content where requested.

`compact`: omit large content fields and truncate long lists.

`minimal`: return only the action and compact result fields such as counts, diagnostics, warnings, or plans.

Example arguments:

```json
{
  "action": "list",
  "project": "docs",
  "detail": "compact"
}
```

## Dry-Run Safety

Write-capable actions default to dry-run behavior.

Document update dry-run:

```json
{
  "action": "update",
  "project": "docs",
  "path": "docs-internal/guides/edge/cli.md",
  "content": "# Replacement content",
  "dry_run": true
}
```

Document update apply:

```json
{
  "action": "update",
  "project": "docs",
  "path": "docs-internal/guides/edge/cli.md",
  "content": "# Replacement content",
  "dry_run": false
}
```

Repair plan:

```json
{
  "action": "plan",
  "project": "docs"
}
```

Repair apply:

```json
{
  "action": "apply",
  "project": "docs",
  "dry_run": false
}
```

## Local Search

BM25 query:

```json
{
  "action": "query",
  "project": "docs",
  "query": "search outbox",
  "limit": 5
}
```

Hybrid query combines BM25 and vector scores when Qdrant and embeddings are configured. If vector configuration is unavailable, hybrid returns BM25 results with a warning. Vector search is opt-in and is not enabled by default.

```json
{
  "action": "hybrid",
  "project": "docs",
  "query": "search outbox",
  "debug": true
}
```

`similar` builds a transient local vector index and searches Qdrant when Qdrant and an embedding provider are configured:

```json
{
  "action": "similar",
  "project": "docs",
  "query": "search outbox",
  "qdrant_url": "http://127.0.0.1:6333",
  "qdrant_collection": "hermes_vectors",
  "embedding_model": "nomic-embed-text",
  "embedding_dimensions": 768,
  "limit": 5
}
```

The same values can come from `HERMES_QDRANT_URL`, `HERMES_QDRANT_COLLECTION`, `HERMES_EMBEDDING_MODEL`, and `HERMES_EMBEDDING_DIMENSIONS`.

## Troubleshooting

Offline remote: use `sync.status` with `probe_remote: true` only when the remote Hermes server should be reachable. Offline remote state is reported as `unavailable` without blocking local reads.

Qdrant unavailable: use `search.query` or `search.hybrid` for BM25-backed local search. Vector `similar` requires Qdrant and embeddings.

Embedding provider unavailable: local BM25 search and document tools do not require embeddings.

Frontmatter migration conflicts: use `repair.plan` first. Apply only after reviewing the returned changes and warnings.

Secret handling: MCP call logging truncates large argument/result fields and redacts keys containing `token`, `secret`, `password`, or `credential`.

## Validation

Run focused checks after changing MCP behavior:

```bash
go test ./internal/mcpserver
go test ./internal/cmd/...
./scripts/docs-validate.sh --quick
```
