---
id: edge-cli-guide
title: "Edge CLI Guide"
status: Reference
created: 2026-05-01
author: Hermes Team
project_id: hermes
doc_uuid: 8e1d7d0d-ae77-4f59-a577-56ac6c08a6c1
type: Guide
subtype: Operations
tags: [edge, cli, docs, search, mcp]
related:
  - ADR-009
  - ADR-011
  - ADR-016
  - ADR-018
  - ADR-021
---

# Edge CLI Guide

> Use `hermes docs` and `hermes edge` to discover HCL-configured documentation lanes, validate and repair local Markdown, inspect sync readiness, and run local BM25 search without requiring a remote backend.

## Configuration

Edge commands read HCL project configuration. The default is `./testing/projects.hcl`.

Use a different project file with `-config`:

```bash
hermes edge discover -config ./testing/projects.hcl -root .
```

Documentation lanes are configured inside project files:

```hcl
project "docs" {
  title         = "Hermes Documentation"
  friendly_name = "Hermes Documentation"
  short_name    = "DOCS"
  status        = "active"

  provider "local" {
    migration_status = "active"
    workspace_path   = "docs"
  }

  lane "adr" {
    schema                    = "adr"
    roots                     = ["docs-internal"]
    folders                   = ["adr"]
    filename_pattern          = "^adr-(\\d{3})-(.+)\\.md$"
    enforce_filename_pattern  = true
    require_frontmatter       = true
    allowed_extensions        = ["md"]
    skip_templates            = ["_template-*", "templates/**"]
  }
}
```

HCL is the runtime source of truth. Do not introduce YAML or JSON project files for edge runtime configuration.

## Documentation Commands

Validate discovered documents:

```bash
hermes docs validate -config ./testing/projects.hcl -root . -project docs
```

Plan frontmatter migration without mutating files:

```bash
hermes docs migrate -config ./testing/projects.hcl -root . -project docs
```

Apply a frontmatter migration explicitly:

```bash
hermes docs migrate -config ./testing/projects.hcl -root . -project docs -apply
```

Bulk set a frontmatter field:

```bash
hermes docs bulk update -config ./testing/projects.hcl -root . -project docs -field owner -value docs-team
```

Backfill missing `created` fields with a degraded current-time fallback:

```bash
hermes docs bulk timestamps -config ./testing/projects.hcl -root . -project docs
```

Plan lane-local ID compression:

```bash
hermes docs bulk compress-ids -config ./testing/projects.hcl -root . -project docs -start-id 1
```

Check relative Markdown links:

```bash
hermes docs repair links -config ./testing/projects.hcl -root . -project docs
```

All write-capable docs commands default to dry-run. Add `-apply` to mutate local files.

## Edge Commands

Show effective projects, lanes, providers, and local paths:

```bash
hermes edge discover -config ./testing/projects.hcl -root .
```

Show sync readiness and local document states:

```bash
hermes edge sync status -config ./testing/projects.hcl -root . -project docs
```

Probe remote Hermes providers with a bounded health check:

```bash
hermes edge sync status -config ./testing/projects.hcl -root . -project docs -probe-remote -timeout 2s
```

Build an in-memory local BM25 index:

```bash
hermes edge index -config ./testing/projects.hcl -root . -project docs
```

Search locally with BM25:

```bash
hermes edge search -config ./testing/projects.hcl -root . -project docs "search outbox"
```

Search with Qdrant-backed vectors when Qdrant and an embedding provider are configured:

```bash
hermes edge search -config ./testing/projects.hcl -root . -project docs \
  -mode vector \
  -qdrant-url http://127.0.0.1:6333 \
  -qdrant-collection hermes_vectors \
  -embedding-model nomic-embed-text \
  -embedding-dimensions 768 \
  "search outbox"
```

Vector mode builds a transient local vector index for discovered documents, generates a query embedding with the configured model, and searches Qdrant through Hermes' `search.VectorIndex` provider boundary. BM25 remains the default and does not require Qdrant or embeddings.

Hybrid mode combines BM25 and vector scores when Qdrant and embeddings are configured. If vector configuration is missing or unavailable, hybrid mode returns BM25 results with a warning instead of failing the search.

Return JSON output for automation:

```bash
hermes edge search -config ./testing/projects.hcl -root . -project docs -format json "search outbox"
```

## Status Meanings

`clean`: Local document has required identity and no validation diagnostics.

`local-dirty`: Local validation found diagnostics such as missing required fields.

`missing-doc-uuid`: Document identity is incomplete and must be repaired before sync operations can safely reason about the document.

`remote-unavailable`: Local state is otherwise clean, but the configured remote Hermes provider could not be reached or probing is disabled.

`local-error`: The local file could not be read.

## Troubleshooting

Offline remote: rerun with `-probe-remote` only when the remote Hermes server should be reachable. Without probing, remote providers are reported as unavailable by design.

Qdrant unavailable: BM25 search works without Qdrant. Vector mode requires `-qdrant-url` or `HERMES_QDRANT_URL` and an embedding model from `-embedding-model` or `HERMES_EMBEDDING_MODEL`.

Embedding provider unavailable: local BM25 commands do not require embeddings. For Ollama embeddings, set `-ollama-url`, `OLLAMA_URL`, or `HERMES_OLLAMA_URL` when Ollama is not on its default URL. Hybrid mode falls back to BM25 with a warning when embeddings are unavailable.

Frontmatter migration conflicts: dry-run first, inspect the plan, then use `-apply` only when the changes are expected. Unknown frontmatter fields are preserved.

## Validation

Run focused checks after changing edge behavior:

```bash
go test ./pkg/edge ./pkg/docrepair ./pkg/search/bm25 ./pkg/docdiscover ./pkg/docschema ./pkg/projectconfig
go test ./internal/cmd/...
./scripts/docs-validate.sh --quick
```
