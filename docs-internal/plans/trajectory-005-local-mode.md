---
id: trajectory-005
title: "Trajectory T5 — Local Mode & Workspace Abstraction"
status: Draft
created: 2026-04-27
date: 2026-04-27
author: Hermes Team
project_id: hermes
doc_uuid: 0f1c1a05-0330-4b08-95e4-c99166ba232a
type: Memo
subtype: Milestone
tags: [trajectory, roadmap, v1.0, local-mode, workspace-provider, e2e]
related:
  - ADR-007
  - ADR-015
  - ADR-021
  - RFC-007
  - RFC-009
  - MEMO-043
  - MEMO-044
---

# Trajectory T5 — Local Mode & Workspace Abstraction

> v1.0 must be runnable end-to-end without Google or Okta: `./hermes` against a local filesystem workspace and Dex auth, with all CRUD reachable from the Ember UI. RFC-009's Phase 1 (read paths) is largely done; v1.0 closes the editing path. The full in-browser editor (RFC-007) is a stretch goal — useful for E2E coverage in T6 but explicitly v1.x if it slips.

## Stable-release criteria served

- v1.0 criterion **#4 Storage** (local filesystem provider end-to-end).
- Supports v1.0 criterion **#7 E2E coverage** (T6) by giving Playwright a deterministic backend.

## Scope

In scope:

- Complete the write path on `pkg/workspace/local/` (create, edit, publish, delete) so the v2 API is functionally complete against a local workspace.
- HCL config flows for zero-config local mode (`./hermes` → working browser session).
- "Demo project" fixtures committed under `testing/projects/`.

Stretch / v1.x:

- RFC-007 in-browser editor (used by T6 for editing-flow E2E; fall back to API-driven test fixtures if it slips).
- RFC-001 bidirectional sync.

## Dependencies

- T4 Phase 2 (local provider participates in workspace router).
- ADR-021 HCL configuration — done.

## Phases & exit criteria

### Phase 0 — Write-path gap audit

- List each v2 endpoint that mutates documents and verify the local provider implements the corresponding `workspace.Provider` method.

**Exit when:**

- Audit table committed; gaps logged as Phase 1 tasks.

### Phase 1 — Write-path completion

- All listed methods implemented and unit tested.
- E2E smoke test: `./hermes` starts with the demo project, browser flow can create + publish a document, restart, document is still there.

**Exit when:**

- Smoke test green in CI on Linux + macOS.
- MEMO-043 checklist updated.

### Phase 2 — Zero-config polish

- `./hermes` with no config writes a default `~/.config/hermes/config.hcl` and chooses a sensible workspace dir.
- First-run experience documented under `docs-internal/guides/`.

**Exit when:**

- New-machine onboarding doc walks from `git clone` to logged-in browser in under 10 minutes.
- RFC-009 status = `Implemented` for the v1.0 subset; the in-browser editor remains `Proposed`.

### Phase 3 (stretch) — In-browser editor (RFC-007)

- Optional. If it lands, T6 can drop API-driven editing fixtures.

**Exit when:**

- Editor produces frontmatter+markdown output identical to the API path on a fixed corpus.

## Reviews / gates

Phase exit-only. Phase 3 is non-blocking for v1.0.

## Risks

- **Local-mode + multi-provider semantics** drift from Google. Mitigation: treat the `workspace.Provider` interface as the contract and reuse compliance tests across adapters.
- **Filesystem race conditions** on simultaneous writes. Mitigation: per-document file lock in `pkg/workspace/local/`.

## References

- [RFC-007: Local Editor E2E Testing](../rfc/rfc-007-local-editor-e2e-testing.md)
- [RFC-009: Simplified Local Mode](../rfc/rfc-009-simplified-local-mode.md)
- [ADR-007 / ADR-015 / ADR-021](../adr/)
- [MEMO-043: Simplified Mode Implementation Checklist](../memo/memo-043-simplified-mode-implementation-checklist.md)
- [MEMO-044: Simplified Mode Summary](../memo/memo-044-simplified-mode-summary.md)
- [Roadmap Implementation Tracker](roadmap-tracker.md)
