---
id: trajectory-005-adversarial-review
title: "Adversarial Review — T5 Local Mode & Workspace Abstraction"
status: Draft
created: 2026-04-27
date: 2026-04-27
author: Hermes Team
project_id: hermes
doc_uuid: 0a33695f-dbdf-456a-a0c4-8249606de8fa
type: Memo
subtype: Analysis
tags: [trajectory, adversarial-review, v1.0, local-mode, workspace-provider]
related:
  - ADR-007
  - ADR-009
  - ADR-021
  - RFC-009
---

# Adversarial Review — T5 Local Mode & Workspace Abstraction

> This review tries to break [Trajectory T5](trajectory-005-local-mode.md) before execution. Local mode is the right deterministic backend for v1.0, but the plan needs stronger data-model, filesystem-safety, auth, and first-run constraints.

## Run-readiness verdict

Phase 0 can start, but Phase 1 should not begin until the local provider compliance contract is explicit. A local provider that only works for the happy-path smoke test can undermine T6 Playwright coverage and hide provider-abstraction bugs.

## Highest-risk failure modes

1. **The local provider may become a special case instead of a provider peer.** [ADR-009](../adr/adr-009-provider-abstraction-architecture.md) and [ADR-007](../adr/adr-007-local-file-workspace-system.md) require local Markdown+YAML to sit behind `workspace.Provider`. Any local-mode shortcut in handlers weakens provider abstraction just before v1.0.

2. **Create/edit/publish/delete is not enough to match real document workflows.** Dashboard E2E depends on review state, recency, current user, document type, timestamps, and search indexing. The local write path must preserve the same semantic fields as Google-backed flows.

3. **Filesystem concurrency is underestimated.** A per-document lock helps within one process, but does not protect against multiple Hermes processes, external editor writes, partial writes, or file watchers seeing half-written content.

4. **Zero-config can write surprising files.** Automatically creating `~/.config/hermes/config.hcl` and choosing a workspace dir needs opt-out, collision handling, and clear behaviour in CI, containers, and read-only home directories.

5. **Auth assumptions are not pinned.** The opening says local mode uses Dex auth, while zero-config says `./hermes` should reach a working browser session. If Dex is required, zero-config is not actually single-binary. If local auth is used, that touches auth-provider decisions and must conform to existing ADRs.

6. **The smoke test can pass while persistence is lossy.** Restart persistence should verify file content, frontmatter, UUID, provider ID, timestamps, status, and search visibility, not just that a title reappears.

7. **Demo fixtures can become accidental production defaults.** `testing/projects/*.hcl` files are loaded by convention while `_template-*` files are skipped per ADR-021. Demo fixtures need names and docs that avoid accidental use in production-like runs.

## Missing decisions before execution

- Define local-provider compliance tests shared with Google/S3 providers.
- Define atomic write strategy: temp file, fsync expectations, rename, lock scope, and external modification handling.
- Define zero-config auth mode and whether Dex is mandatory.
- Define default config path, workspace path, collision behaviour, and CI override.
- Define frontmatter schema compatibility and migration behaviour for existing local files.
- Define how local writes trigger T1/T2 search/indexing flows.

## Concrete pre-flight checklist

- Add a provider compliance test suite that every workspace adapter can run.
- Add local filesystem tests for concurrent writes, partial write recovery, external modification, invalid frontmatter, duplicate UUIDs, and delete/restore behaviour.
- Add a first-run test using a temporary home directory with no pre-existing config.
- Add a read-only-home test that fails gracefully with actionable guidance.
- Add a restart test that verifies document identity, content, metadata, and search visibility.
- Add a CI-safe demo project naming convention and document which `testing/projects/*.hcl` files load by default.

## Suggested plan edits

- Add an auth subsection to Phase 2 that states the exact local-mode auth provider and how it follows ADR-012/ADR-014/ADR-021 constraints.
- Move compliance tests into Phase 1 exit criteria.
- Replace "sensible workspace dir" with an explicit path policy and override mechanism.
- Make the smoke test cover search/index convergence, not only workspace persistence.

## Go/no-go gate

Treat local mode as ready only when the same provider compliance tests pass against local and at least one non-local provider, with no handler-level local shortcuts.
