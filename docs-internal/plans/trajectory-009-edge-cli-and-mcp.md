---
id: trajectory-009
title: "Trajectory T9 — Edge CLI and Agent MCP"
status: Draft
created: 2026-05-01
date: 2026-05-01
author: Hermes Team
project_id: hermes
doc_uuid: 0c537912-09e9-4aeb-af0f-daa3d493f008
type: Memo
subtype: Milestone
tags: [trajectory, edge, cli, mcp, local-workspace, search, agents]
related:
  - ADR-007
  - ADR-009
  - ADR-011
  - ADR-016
  - ADR-017
  - ADR-018
  - ADR-021
---

# Trajectory T9 — Edge CLI and Agent MCP

> Builds Hermes' edge-capable command surface and agent integration layer: local document discovery and repair, local/remote workspace providers, local search, and `hermes mcp`. This plan is implementation-shaped and must preserve the existing ADR-governed provider, search, document identity, and HCL configuration boundaries.

## Stable-release criteria served

None for v1.0 unless explicitly pulled into the v1.0 roadmap. This trajectory is a focused execution plan for edge/client work and agent workflows that can proceed independently of T7 federation and admin UI.

## Scope

In scope:

- A single `hermes` process with new subcommand groups: `docs`, `edge`, and `mcp`.
- A Go port of documentation discovery, frontmatter validation, repair, migration, and bulk-update workflows from the existing Python documentation tooling concepts.
- HCL-backed project and document-lane discovery for monorepos with multiple projects or schema lanes in one repository.
- Local-folder and remote-Hermes API provider orchestration through provider interfaces.
- Local lexical search using an embedded BM25 corpus.
- Local vector search using Qdrant as the vector store and Hermes embedding providers for vectors.
- Hybrid local search that can combine lexical and vector hits while keeping the search index a cache.
- `hermes mcp` for agent workflows over stdio by default, with optional streamable HTTP transport.
- Agent-facing tools for project discovery, document operations, search, sync, repair, and compact context retrieval.

Out of scope:

- Replacing the existing production/local backend-mediated search path or removing Meilisearch from local/CI. ADR-011 remains binding until revisited.
- Adding browser/frontend search-provider credentials. ADR-016 remains binding.
- Introducing YAML/JSON as a new Hermes project source of truth. ADR-021 requires HCL.
- Bidirectional federation semantics, delegated service-to-service auth, and admin UI. Those remain parked in T7 unless split into dedicated future trajectories.
- Persisted multi-instance conflict resolution beyond narrow edge sync status and explicit pull/push commands.

## Binding architectural constraints

- External integrations go through provider interfaces per [ADR-009](../adr/adr-009-provider-abstraction-architecture.md).
- Local filesystem workspace remains a first-class `workspace.Provider` per [ADR-007](../adr/adr-007-local-file-workspace-system.md).
- The frontend and browser clients never hold search credentials; search remains backend-mediated per [ADR-016](../adr/adr-016-search-and-auth-refactoring.md).
- V2 HTTP handlers use `func Handler(srv *server.Server) http.Handler`, with `server.Server` as the DI container, per [ADR-017](../adr/adr-017-api-refactoring-and-testing-strategy.md).
- Database truth and document UUID identity remain canonical; search indexes are caches and `pkg/docid` owns identity semantics per [ADR-017](../adr/adr-017-api-refactoring-and-testing-strategy.md) and [ADR-018](../adr/adr-018-document-identification-system.md).
- Project and lane configuration must be HCL, not YAML/JSON, per [ADR-021](../adr/adr-021-hcl-projects-configuration.md).

## Command surface

Use one binary: `hermes`.

Add these command groups:

- `hermes docs validate`: validate and optionally repair local documentation lanes.
- `hermes docs migrate`: normalize frontmatter, add missing `project_id`, generate missing UUIDs, migrate legacy timestamp fields, and remove deprecated fields.
- `hermes docs bulk update`: set, add, remove, or rename frontmatter fields across discovered documents.
- `hermes docs bulk timestamps`: derive immutable created timestamps from git history where missing.
- `hermes docs bulk compress-ids`: renumber documents in a configured lane and rewrite references.
- `hermes docs init`: create an HCL-backed local documentation workspace or lane scaffold.
- `hermes edge discover`: show effective projects, lanes, providers, workspace paths, and remote targets.
- `hermes edge index`: build or refresh local BM25/vector indexes for selected projects or lanes.
- `hermes edge search`: query local, remote, or hybrid search from the CLI.
- `hermes edge sync status`: show local/remote state, dirty documents, missing remote objects, and index freshness.
- `hermes edge sync pull`: pull remote document metadata/content into a local workspace provider when supported.
- `hermes edge sync push`: push local document changes to a remote provider only when explicitly requested.
- `hermes mcp`: start the agent MCP server over stdio by default.
- `hermes mcp -http :<port>`: start the MCP server over streamable HTTP at `/mcp` for clients that support it.

Do not add a separate process name unless a later performance or deployment requirement proves process isolation is needed.

## Package design

Add or evolve packages with narrow responsibilities:

- `pkg/docschema`: frontmatter schemas, status vocabularies, validation diagnostics, and schema selection for ADR, RFC, memo, PRD, and generic document lanes.
- `pkg/docdiscover`: project/lane discovery, file walking, ignore rules, template skipping, and document type inference.
- `pkg/docrepair`: frontmatter normalization, tag normalization, timestamp migration, code block repair, internal-link repair, and ID compression.
- `pkg/edge`: edge session/config loader, local/remote provider selection, effective capability model, and defer-to-remote policy.
- `pkg/edge/sync`: explicit pull/push/status operations, dry-run plans, conflict reporting, and provider capability checks.
- `pkg/search/bm25`: embedded thread-safe BM25 corpus with tokenization, stemming, incremental add/remove, score, search, and rebuild operations.
- `pkg/search/adapters/qdrant`: vector index adapter for local Qdrant.
- `pkg/search/hybrid`: fusion strategy for BM25 and vector results. Keep the first strategy simple and explainable.
- `internal/cmd/commands/docs`: CLI command group for documentation operations.
- `internal/cmd/commands/edge`: CLI command group for local edge operations.
- `internal/cmd/commands/mcp`: CLI command group for MCP startup.
- `internal/mcpserver`: Hermes MCP server, tool registration, harness detection, call logging, and tool handlers.

Keep package boundaries minimal. Do not create new abstractions until a second call site or clear provider boundary requires them.

## HCL project and lane model

Complete HCL-backed discovery before relying on monorepo behavior.

Required configuration capabilities:

- Multiple project files imported from a root projects HCL file.
- Multiple local lanes per project, each with folders, allowed extensions, schema, filename pattern, and frontmatter-required flag.
- Multiple docs roots per project for monorepos.
- Explicit template file/folder skips.
- Active provider selection per project using existing provider state concepts.
- Remote-Hermes provider configuration with URL, API version, auth method, and cache TTL.

Example shape to support, adjusted to exact existing HCL parser conventions during implementation:

```hcl
project "hermes-docs" {
  title         = "Hermes Documentation"
  friendly_name = "Hermes Docs"
  short_name    = "DOCS"
  status        = "active"

  provider "local" {
    migration_status = "active"
    workspace_path   = "docs"

    indexing {
      enabled            = true
      allowed_extensions = ["md", "mdx"]
      public_read_access = true
    }
  }

  lane "adr" {
    schema                    = "adr"
    roots                     = ["docs-internal"]
    folders                   = ["adr"]
    filename_pattern          = "^adr-(\\d{3})-(.+)\\.md$"
    enforce_filename_pattern  = true
    require_frontmatter       = true
  }

  lane "guides" {
    schema                    = "generic"
    roots                     = ["docs-internal"]
    folders                   = ["guides"]
    filename_pattern          = ".+\\.md$"
    enforce_filename_pattern  = false
    require_frontmatter       = false
  }
}
```

Implementation must reconcile this with `pkg/projectconfig/models.go`; do not introduce a second project loader.

## MCP server design

`hermes mcp` should be a first-class command, not an afterthought wrapper around CLI commands.

Server behavior:

- Start over stdio by default.
- Support `-http <addr>` for streamable HTTP, mounted at `/mcp`.
- Load Hermes config and project config once at startup.
- Construct providers through existing factories or adapter constructors.
- Keep a `Server` object that owns config, providers, search clients, project/lane discovery, logger, call log, and tool handlers.
- Register tools through one `registerTools()` function.
- Wrap every handler with panic recovery that returns MCP error results without crashing the process.
- Add initialize hooks to detect client/harness and select default response detail.
- Add before/after call hooks for bounded call logging: tool, action, duration, truncated args, and error summary.
- Provide a direct `CallTool(ctx, name, args)` method for unit tests.
- Close resources on SIGINT/SIGTERM and normal process exit.
- Keep tool outputs machine-readable JSON by default, with compact response modes for smaller-context clients.

Initial tools:

- `context`: actions `summary`, `project`, `document`, `search_context`. Returns compact context for the active repository/project.
- `project`: actions `list`, `get`, `discover`, `providers`, `lanes`.
- `document`: actions `list`, `get`, `content`, `update`, `validate`, `links`.
- `search`: actions `query`, `similar`, `hybrid`, `index_status`.
- `repair`: actions `plan`, `apply`, `migrate`, `bulk_update`, `timestamps`, `compress_ids`.
- `sync`: actions `status`, `plan_pull`, `pull`, `plan_push`, `push`.
- `artifact`: optional session-scoped scratch artifacts if needed by agent workflows; defer until a concrete need appears.

Tool rules:

- Any write-capable action must support dry-run/plan output before mutation.
- Any local document update must preserve frontmatter ordering as much as practical and never remove unknown fields unless an explicit migration action owns that behavior.
- Any remote mutation must require an explicit action such as `push` or `update`; search/context/read tools must not mutate remote state.
- Tool errors must be structured and actionable; avoid plain stack traces.
- Tool names should remain stable after first release. Prefer action expansion over many narrow one-off tool names.

## HTTP handling pattern

When this trajectory adds HTTP surfaces, use this shape:

- A server struct owns dependencies.
- Route registration is explicit in a `registerRoutes` or route package function.
- Middleware wraps the route tree after registration.
- HTTP servers set read, write, and idle timeouts.
- Shutdown uses context cancellation and bounded graceful shutdown.
- JSON responses go through a small helper that sets status and content type consistently.
- Request logging sanitizes method, path, remote address, status, duration, and request/session identifiers.
- Streaming/event endpoints use bounded buffers, non-blocking fan-out, keepalives, and replay when a protocol supports it.

For V2 Hermes API routes, adapt this pattern to ADR-017 rather than bypassing it. New V2 handlers must still be `func Handler(srv *server.Server) http.Handler` and must use `srv` dependencies.

## Phase 0 — RFC/ADR Compliance and Skeleton

- Read ADR-007, ADR-009, ADR-011, ADR-016, ADR-017, ADR-018, and ADR-021 before coding.
- Add command groups with help text and no-op or read-only placeholders where needed: `docs`, `edge`, `mcp`.
- Add `internal/mcpserver` skeleton with stdio startup, optional HTTP startup, tool registration, panic recovery, call logging, direct `CallTool`, and tests.
- Add package skeletons for `pkg/docschema`, `pkg/docdiscover`, `pkg/docrepair`, `pkg/edge`, and `pkg/search/bm25`.
- Add focused unit tests for command registration and MCP panic-safe dispatch.

**Exit when:**

- `hermes docs -h`, `hermes edge -h`, and `hermes mcp -h` work.
- `hermes mcp` can start over stdio and expose at least `project` and `context` placeholder/read-only tools.
- `hermes mcp -http :0` or test equivalent can mount streamable HTTP without blocking tests.
- Direct MCP `CallTool` tests cover success, unknown tool, handler error, and panic recovery.
- No ADR contradiction is introduced.

## Phase 1 — HCL Discovery and Documentation Validation

- Replace MVP/hardcoded project loading with real HCL import support or add the minimal missing loader behavior needed for project/lane discovery.
- Add HCL lane model for documentation discovery.
- Implement recursive Markdown discovery for configured lanes.
- Implement schema validation for ADR, RFC, memo, PRD, and generic documents.
- Implement plain Markdown generic lanes where `require_frontmatter = false`.
- Implement CLI and MCP read-only validation output.
- Keep YAML project files out of Hermes configuration; any legacy YAML import must be an explicit migration source, not runtime source of truth.

**Exit when:**

- A monorepo fixture with at least two projects and mixed lanes discovers the expected files.
- `docs-internal/` can be modeled as a local Hermes documentation workspace without scanning templates as live docs.
- Validation produces stable diagnostics with file, line when available, field, severity, and suggested fix.
- Unit tests cover filename patterns, optional frontmatter, multiple roots, template skips, invalid HCL, and unknown schemas.

## Phase 2 — Documentation Repair and Migration

- Port frontmatter normalization, tag normalization, timestamp migration, code-block checks/fixes, internal-link validation, bulk field updates, and ID compression.
- Use dry-run plans as the default for MCP write-capable repair actions.
- Preserve unknown frontmatter fields unless an explicit migration action removes a known deprecated field.
- Use git history only when available; return degraded warnings when not in a git worktree.
- Add CLI parity for `validate`, `migrate`, `bulk update`, `bulk timestamps`, and `bulk compress-ids`.

**Exit when:**

- Repair commands support dry-run and apply modes.
- Tests cover no-op, apply, dry-run-not-mutating, malformed frontmatter, missing git history, ID rewrite, and stale/missing reference reporting.
- Running validation after apply is idempotent on fixtures.
- Existing docs validation still passes or existing known validation gaps are explicitly unchanged.

## Phase 3 — Edge Providers and Sync Status

- Build an edge configuration loader that combines Hermes config, project HCL, active project, and provider capability discovery.
- Use local folder and remote Hermes API providers through workspace interfaces.
- Implement `edge discover` and `edge sync status`.
- Implement explicit sync planning for pull/push without automatically mutating remote state.
- Add remote capability probing with timeouts and graceful offline fallback.
- Preserve stable document UUID and provider ID semantics.

**Exit when:**

- `hermes edge discover` shows projects, lanes, local paths, active providers, and remote providers.
- `hermes edge sync status` distinguishes clean, local-only, remote-only, divergent, and unavailable remote states.
- Sync planning tests cover offline remote, missing document UUID, provider capability mismatch, local dirty file, remote newer file, and unknown provider.
- No handler or CLI path calls concrete external clients directly where a provider interface exists.

## Phase 4 — Local Search: BM25, Qdrant, Hybrid

- Port embedded BM25 into `pkg/search/bm25` with thread-safe add, remove, rebuild, score, and search.
- Add document text builders for Hermes documents and local lanes.
- Add Qdrant vector adapter using existing embedding providers.
- Add local index metadata: indexed at, document hash, provider ID, project, lane, and embedding model.
- Add `edge index` and `edge search` for local BM25, vector, and hybrid modes.
- Keep local search a cache. Do not make it authoritative for document truth.

**Exit when:**

- BM25 unit tests cover tokenization, identifier splitting, ranking, no-match, removal, rebuild, and concurrent access.
- Local index tests cover changed file reindex, deleted file deindex, and unchanged file skip.
- Qdrant integration is opt-in and skipped cleanly when Qdrant is unavailable.
- Hybrid search returns deterministic ordering for equal scores and explains score components in debug/JSON output.
- Existing Meilisearch/Algolia provider tests continue to pass.

## Phase 5 — MCP Tool Completeness

- Implement initial read tools: `context`, `project`, `document`, `search`.
- Implement write-capable tools behind dry-run plans: `repair`, `sync`, `document update`.
- Add response detail levels: `full`, `compact`, and `minimal`.
- Add structured error envelopes and warning arrays.
- Add tests for every tool/action pair through direct `CallTool`.
- Add an integration smoke test that starts the MCP server and performs initialize plus at least one tool call when test infrastructure supports it.

**Exit when:**

- Every initial MCP tool has direct handler tests for success and at least one failure mode.
- Write-capable MCP actions prove dry-run does not mutate files or remote state.
- Tool outputs are valid JSON for machine-readable actions.
- Call logging redacts or truncates large arguments and never logs known secret fields.
- Stdio startup and HTTP startup both have tests or documented manual verification commands.

## Phase 6 — Documentation and Operational Readiness

- Add an internal guide for edge CLI usage.
- Add an internal guide for MCP client configuration.
- Add examples for monorepo HCL lanes.
- Add troubleshooting for offline remote, Qdrant unavailable, embedding provider unavailable, and frontmatter migration conflicts.
- Update ADR/RFC docs only if implementation discovers a real architectural contradiction.

**Exit when:**

- Guides document the command surface, MCP configuration, provider model, and local search setup.
- The plans index links this trajectory.
- `./scripts/docs-validate.sh --quick` has been run and its result recorded in the implementation summary or commit message.
- A final summary lists completed phases, skipped/deferred items, test evidence, and any ADR follow-up needed.

## Completion criteria

This trajectory is complete when all of the following are true:

- `hermes docs` can discover, validate, repair, migrate, and bulk-update configured local documentation lanes from HCL.
- `hermes edge` can discover projects/providers, index local documents, search locally/hybrid, and report sync status without requiring a remote backend.
- `hermes edge` defers remote-capable behavior to the backend when online and reports graceful fallback when offline.
- `hermes mcp` serves stdio by default and optional streamable HTTP at `/mcp`.
- MCP tools cover project discovery, document reads/content, validation, search, repair planning/apply, and sync status/planning.
- All write-capable MCP tools support dry-run planning and have tests proving no mutation in dry-run mode.
- Local BM25 is covered by unit tests and local Qdrant is covered by opt-in integration tests.
- The implementation preserves ADR-009 provider boundaries, ADR-017 V2 DI boundaries, ADR-018 identity semantics, and ADR-021 HCL project configuration.
- Focused tests and docs validation pass, or any unavailable external-service tests are explicitly documented with skip/fallback behavior.

## Implementation status — 2026-05-01

Completed in this implementation:

- `hermes docs` discovers, validates, repairs, migrates, and bulk-updates HCL-configured local documentation lanes.
- `hermes edge` discovers projects/providers, reports local sync status, builds an in-memory BM25 index, and searches local documents without requiring a remote backend.
- `hermes mcp` serves stdio by default and streamable HTTP at `/mcp`, with direct tool-call tests and an HTTP initialize plus tool-call smoke test.
- MCP tools cover project discovery, document reads/content, validation, search, repair planning/apply, and sync status/planning.
- Write-capable MCP actions default to dry-run planning where applicable, and tests cover dry-run document updates without mutation.
- BM25 has focused unit coverage, and Qdrant has an opt-in health integration test gated by `HERMES_TEST_QDRANT_URL`.
- Focused tests and `./scripts/docs-validate.sh --quick` passed locally on 2026-05-01.

Deferred intentionally:

- Full Qdrant vector indexing/search and embedding-provider wiring. The Qdrant package is limited to configuration, bounded health checks, and a clear not-implemented search skeleton.
- True BM25/vector score fusion. Current `hybrid` responses are BM25-backed with debug score information.
- Persisted local search cache and unchanged-file skip metadata.
- Remote pull/push mutations and remote document diffing. Current sync behavior is status and planning only.
- Git-history timestamp derivation beyond degraded local fallback.
- Link/reference rewriting during lane-local ID compression.

## Review gates

- Gate 1 after Phase 1: review HCL lane model against ADR-021 before broad repair/indexing code depends on it.
- Gate 2 after Phase 3: review provider usage against ADR-009 and ADR-018 before adding push/pull mutations.
- Gate 3 after Phase 4: review search architecture against ADR-011 and ADR-016 before calling local hybrid search user-facing default behavior.
- Gate 4 before completion: review MCP write tools for dry-run safety, secret redaction, and remote mutation guardrails.

## Risks

- **YAML config leaks into Hermes runtime.** Mitigation: translate documentation-lane concepts into HCL and treat YAML only as legacy import data if needed.
- **MCP tools mutate too easily.** Mitigation: make dry-run plans mandatory for write-capable actions and require explicit `apply`/`push` actions.
- **Local search becomes a second source of truth.** Mitigation: local indexes store hashes and timestamps but always point back to provider/document identity.
- **Qdrant adds local setup friction.** Mitigation: BM25 works without Qdrant; vector search is opt-in and reports clear unavailable status.
- **Monorepo discovery scans too broadly.** Mitigation: require configured roots/folders and explicit template skips; tests must include accidental broad-scan cases.
- **CLI grows inconsistent with current command framework.** Mitigation: use existing `internal/cmd` patterns and keep one binary.

## Launch prompt

Use this prompt to start implementation in a fresh agent session:

```text
You are implementing Hermes Trajectory T9: Edge CLI and Agent MCP.

Read these first and conform to them: AGENTS.md, docs-internal/plans/trajectory-009-edge-cli-and-mcp.md, ADR-007, ADR-009, ADR-011, ADR-016, ADR-017, ADR-018, and ADR-021. Do not mention any internal source project names in docs, comments, commits, or user-facing output.

Work autonomously until blocked. Use small, focused commits only when explicitly asked. Do not revert unrelated worktree changes. Keep the implementation minimal and provider-aligned.

Completion enforcement:
- Implement by phases in T9 order. Do not start a later phase until the current phase exit criteria pass or are explicitly marked blocked with rationale.
- Before writing code in a phase, restate the phase exit criteria as a checklist in your working notes/todos.
- Every write-capable docs/edge/MCP action must have a dry-run or plan mode before mutation.
- Runtime project/lane config must be HCL-backed. Do not introduce YAML/JSON as Hermes' runtime project source of truth.
- All external integrations must go through existing provider interfaces or a narrowly proposed provider addition.
- V2 HTTP handlers must follow `func Handler(srv *server.Server) http.Handler` if any are added.
- Search indexes are caches; document identity remains UUID/provider/project based through `pkg/docid` semantics.
- MCP must support stdio by default, optional streamable HTTP at `/mcp`, panic-safe tool dispatch, direct `CallTool` tests, initialize hooks, bounded call logging, and structured JSON tool responses.
- MCP write tools must prove dry-run does not mutate files or remote state.
- Local BM25 must include tokenization/ranking/removal/rebuild/concurrency tests. Qdrant integration must be opt-in and skip cleanly when unavailable.
- Run focused tests for changed packages and `./scripts/docs-validate.sh --quick` before declaring a phase complete. If a test requires unavailable local services, document the exact command and skip/fallback reason.

Start with Phase 0 only:
1. Add `docs`, `edge`, and `mcp` command groups with help text.
2. Add MCP server skeleton with stdio startup, optional HTTP startup, tool registration, panic recovery, direct `CallTool`, initialize hook, and bounded call logging.
3. Add placeholder/read-only `project` and `context` tools.
4. Add package skeletons only where needed for Phase 0.
5. Add tests for command registration and MCP success, unknown tool, handler error, and panic recovery.
6. Run focused tests and docs quick validation.

Stop after Phase 0 is complete and report: changed files, tests run, pass/fail status, any ADR concerns, and the next phase checklist.
```

## References

- [ADR-007: Local File Workspace System](../adr/adr-007-local-file-workspace-system.md)
- [ADR-009: Provider Abstraction Architecture](../adr/adr-009-provider-abstraction-architecture.md)
- [ADR-011: Meilisearch as Local Search Solution](../adr/adr-011-meilisearch-as-local-search-solution.md)
- [ADR-016: Backend-Mediated Search and Runtime Auth Header Selection](../adr/adr-016-search-and-auth-refactoring.md)
- [ADR-017: V2 API Provider Abstraction Pattern](../adr/adr-017-api-refactoring-and-testing-strategy.md)
- [ADR-018: Document Identification System](../adr/adr-018-document-identification-system.md)
- [ADR-021: HCL for Per-Project Configuration](../adr/adr-021-hcl-projects-configuration.md)
