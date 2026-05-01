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

> v1.0 must be runnable end-to-end without Google or Okta: Hermes uses the local filesystem workspace behind `workspace.Provider` and Dex OIDC behind `auth.Provider`, with all supported document workflows reachable from the Ember UI. RFC-009's local-mode vision remains broader than the v1.0 target; this trajectory closes the v1.0 local workspace editing path without adding handler-level local shortcuts or new auth/config architecture. The full in-browser editor (RFC-007) is a stretch goal — useful for E2E coverage in T6 but explicitly v1.x if it slips.

## Stable-release criteria served

- v1.0 criterion **#4 Storage** (local filesystem provider end-to-end).
- Supports v1.0 criterion **#7 E2E coverage** (T6) by giving Playwright a deterministic backend.

## Scope

In scope:

- Complete the write path on `pkg/workspace/local/` through the `workspace.Provider` interface (ADR-007, ADR-009): create, edit, publish, delete, restore where supported, metadata preservation, review-state fields, recency timestamps, ownership, permissions, document type, stable UUID, provider ID, and search/indexing signals.
- HCL config flows for first-run local mode that select `providers { auth = "dex", workspace = "local", search = "meilisearch" }` without introducing YAML/JSON config (ADR-021).
- Dex-backed local authentication that uses the existing backend-centric OIDC flow and HttpOnly session cookie model (ADR-012, ADR-014). A new `local` or trust-based auth provider is out of scope unless a separate ADR/RFC revisits auth-provider selection.
- CI-safe demo project fixtures committed under `testing/projects/`, with names and imports that avoid accidental production-like loading.

Stretch / v1.x:

- RFC-007 in-browser editor (used by T6 for editing-flow E2E; fall back to API-driven test fixtures if it slips).
- RFC-001 bidirectional sync.
- RFC-009 embedded single-binary ideas such as SQLite-in-server, Bleve, trust-based local auth, YAML config, or bundled external services. These conflict with or extend current ADR-019, ADR-021, ADR-011, ADR-012, and ADR-014 constraints and require separate decision work before implementation.

## Dependencies

- T4 Phase 2 (local provider participates in workspace router).
- ADR-021 HCL configuration — done.
- ADR-007 / ADR-009 provider model — local workspace must remain a peer adapter, not a handler special case.
- ADR-012 / ADR-014 Dex OIDC session flow — local mode auth for v1.0 stays Dex-backed.
- ADR-011 / ADR-016 search routing — local writes trigger backend search/indexing flows; the frontend never receives search credentials.

## Provider compliance contract

T5 is not complete if local mode only passes a happy-path smoke test. The local provider must satisfy the same workspace semantics expected from non-local providers:

- All V2 handlers call `workspace.Provider` methods injected through `server.Server`; no local-provider branches in handlers except capability handling already allowed by ADR-015.
- Errors wrap typed workspace/search sentinels and are asserted with `errors.Is` (ADR-009, ADR-017).
- Document identity uses `pkg/docid`: stable UUID, provider ID, and project context survive restart, rename, publish, delete/restore, and provider migration tests (ADR-018).
- Frontmatter read/write preserves the semantic fields required by dashboards and search: title/name, document type, status/review state, owner/current user visibility, permissions, created/modified/published timestamps, UUID, provider ID, project, and content hash where present.
- Local writes emit or enqueue the same indexing path as other providers so search visibility converges through T1/T2-backed flows instead of a direct search side channel.

Compliance tests must be reusable across local and at least one non-local adapter. Local-specific filesystem tests may supplement them, but cannot replace them.

## Filesystem safety contract

Local writes must be safe under normal developer and CI usage:

- Write via same-directory temp file, flush file contents, rename atomically, and handle parent-directory sync where supported by the platform.
- Serialize same-process writes per logical document and detect stale writes using modified timestamp, content hash, or revision metadata.
- Treat external editor changes as first-class: reread before overwrite, reject or surface conflicts instead of silently dropping content.
- Recover cleanly from invalid frontmatter, duplicate UUIDs, missing files, partially written temp files, and delete/restore races.
- Document that the local provider is not a multi-process production locking system; tests must still cover concurrent same-process writes and externally modified files.

## First-run and demo-project policy

- Default config path: `${XDG_CONFIG_HOME:-~/.config}/hermes/config.hcl`, overridden by explicit `-config` and environment variables already supported by the config loader.
- Default workspace path: `${XDG_DATA_HOME:-~/.local/share}/hermes/workspaces/default` for native first-run, unless an explicit workspace path or testing profile is selected.
- First-run creation must be opt-out in CI and non-interactive contexts. If `$HOME` or the config/workspace parent is read-only, Hermes fails with an actionable message and does not partially initialize.
- Existing config or workspace paths are never overwritten without an explicit force/repair operation.
- Demo project files under `testing/projects/*.hcl` are loaded only by the testing/local profile. Template examples remain `_template-*` so ADR-021 loaders skip them.

## Phases & exit criteria

### Phase 0 — Write-path gap audit

- List each V2 endpoint that mutates documents and verify the local provider implements the corresponding `workspace.Provider` method through DI.
- Produce the provider compliance matrix for local and at least one non-local provider: supported operations, required metadata fields, expected sentinel errors, search/indexing side effects, and capability differences.
- Audit first-run config/auth assumptions against ADR-012, ADR-014, ADR-019, and ADR-021; record any RFC-009 items that need separate decision work before they can land.

**Exit when:**

- Audit table committed; gaps logged as Phase 1 tasks.
- Compliance test plan committed and linked from MEMO-043.
- Auth and first-run policy above is accepted for v1.0, or this trajectory is paused for a new ADR/RFC.

### Phase 1 — Write-path completion

- All listed methods implemented and unit tested through `workspace.Provider`; no handler-level local shortcuts.
- Shared provider compliance tests pass for local and at least one non-local provider.
- Local filesystem tests cover concurrent same-process writes, atomic-write recovery, external modification conflict, invalid frontmatter, duplicate UUIDs, delete/restore, and metadata preservation.
- E2E smoke test: local profile starts with the demo project, browser/API flow can create, edit, publish, delete/restore where supported, restart, and verify the document's content, frontmatter, UUID, provider ID, timestamps, status/review metadata, and search visibility.

**Exit when:**

- Smoke test green in CI on Linux + macOS.
- Provider compliance suite is green in CI for local and one non-local provider.
- MEMO-043 checklist updated.

### Phase 2 — Zero-config polish

- First-run native mode writes a default HCL config only after applying the first-run policy above, selects an explicit default workspace path, and configures Dex/local/Meilisearch providers.
- Auth behavior is documented as Dex OIDC for v1.0: if Dex is not reachable, the browser flow fails gracefully with setup guidance rather than silently changing auth mode.
- First-run experience documented under `docs-internal/guides/`, including CI opt-out, read-only-home behavior, explicit config/workspace overrides, and demo project loading rules.

**Exit when:**

- New-machine onboarding doc walks from `git clone` to logged-in browser in under 10 minutes.
- First-run tests pass with an empty temporary home, an existing config, an existing workspace, a read-only home, and CI/non-interactive mode.
- RFC-009 status = `Implemented` for the v1.0 subset; the in-browser editor remains `Proposed`.

### Phase 3 (stretch) — In-browser editor (RFC-007)

- Optional. If it lands, T6 can drop API-driven editing fixtures.

**Exit when:**

- Editor produces frontmatter+markdown output identical to the API path on a fixed corpus.

## Reviews / gates

- Phase 0 cannot exit until the provider compliance contract and auth/first-run policy are explicit.
- Phase 1 cannot exit unless the shared compliance suite passes against local and at least one non-local provider.
- Phase 2 cannot exit unless first-run behavior is deterministic, opt-out in CI/non-interactive contexts, and safe for read-only or pre-existing paths.
- Phase 3 is non-blocking for v1.0.

## Risks

- **Local-mode + multi-provider semantics** drift from Google. Mitigation: treat the `workspace.Provider` interface as the contract and reuse compliance tests across adapters.
- **Filesystem race conditions** on simultaneous writes. Mitigation: atomic write/rename, same-process per-document serialization, stale-write detection, external modification tests, and explicit non-goal for multi-process production locking.
- **Auth scope creep** into a new local/trust provider. Mitigation: v1.0 local mode uses Dex OIDC only; new auth modes require ADR/RFC work.
- **RFC-009 drift from current ADRs** around YAML, embedded database/search, and single-binary assumptions. Mitigation: T5 implements only the ADR-conformant v1.0 subset and records broader RFC-009 items as v1.x decision work.
- **Demo fixtures load accidentally in production-like runs.** Mitigation: keep templates prefixed `_template-*`, document active testing project files, and require explicit testing/local profile selection.

## References

- [RFC-007: Local Editor E2E Testing](../rfc/rfc-007-local-editor-e2e-testing.md)
- [RFC-009: Simplified Local Mode](../rfc/rfc-009-simplified-local-mode.md)
- [T5 Adversarial Review](trajectory-005-adversarial-review.md)
- [ADR-007 / ADR-015 / ADR-021](../adr/)
- [MEMO-043: Simplified Mode Implementation Checklist](../memo/memo-043-simplified-mode-implementation-checklist.md)
- [MEMO-044: Simplified Mode Summary](../memo/memo-044-simplified-mode-summary.md)
- [Roadmap Implementation Tracker](roadmap-tracker.md)
