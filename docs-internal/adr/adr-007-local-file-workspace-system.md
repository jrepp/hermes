---
id: adr-007
title: Local File Workspace System
date: 2025-10-09
type: ADR
subtype: Backend Architecture
decision_type: Backend Architecture
status: Accepted
tags: [backend, workspace, local-workspace, filesystem]
related: [ADR-009, ADR-015]
created: 2026-04-24
deciders: Hermes Team
project_id: hermes
doc_uuid: 56b73a28-5daf-43ac-8b23-66539a5df990
---

# Local File Workspace System

> Hermes supports a **local filesystem workspace** (`pkg/workspace/local/`) as a first-class implementation of `workspace.Provider`. Documents are Markdown files with YAML frontmatter under a configured `root_path`; users live in `users.json`. It is the default workspace for development, testing, and CI, and is selected via `providers.workspace = "local"`.

## Context

Hermes was originally coupled to Google Workspace (Drive + Docs). That coupling forced every developer and CI job to provision OAuth credentials, hit Google APIs (with quota and 100–500 ms latency), and mock a sprawling API surface for tests. We needed a workspace implementation that worked offline, was deterministic, and slotted in behind the same `workspace.Provider` interface as Google.

## Decision

Implement a local-filesystem workspace adapter that conforms to `workspace.Provider` (ADR-009), so callers cannot tell whether they are talking to Google or to disk.

**Layout under `root_path`:**

```text
workspace_data/
├── drafts/{doc-id}.md            # or {doc-id}/content.md
├── docs/{doc-id}/content.md      # published
├── templates/template-*.md       # {{variable}} placeholders
└── users.json                    # user directory
```

**Document format:** Markdown body with a YAML frontmatter block carrying Google-compatible metadata (`id`, `name`, `created_time`, `modified_time`, `owner`, `permissions_json`).

**Selection:** `providers.workspace = "local" | "google"` in HCL config.

## Consequences

### Positive
- Zero external dependencies; works offline, in CI, and in agent sandboxes.
- Deterministic state — files can be inspected, diffed in git, and reset by recreating `root_path`.
- Drop-in replacement for the Google adapter; no handler changes required.
- Suitable substrate for templates and seed data shipped with the testing stack.

### Negative
- No real-time collaboration, comments, or rich Google Docs features.
- Not intended for multi-user production deployments.
- Document-level history relies on git; there is no per-document version log.

## Alternatives Considered

- **SQLite store:** ACID and queryable, but binary and not human-editable; loses the "open the file in your editor" property.
- **MinIO / S3-compatible:** Production-like, but adds a service for no local-dev benefit.
- **In-memory only:** Fast but loses state between processes; breaks multi-step manual testing.
- **Git-as-storage:** Versioning for free, but git operations are slow and conflict-prone in the hot path.

## References

- `pkg/workspace/local/` — implementation
- `testing/workspace_data/` — seeded fixtures
- ADR-009 (provider abstraction), ADR-015 (document editor)