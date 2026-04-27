---
id: trajectory-002
title: "Trajectory T2 — Event-Driven Indexer Cutover"
status: Draft
created: 2026-04-27
date: 2026-04-27
author: Hermes Team
project_id: hermes
doc_uuid: ecfb808d-5ac1-4c21-beb5-ab538c79f238
type: Memo
subtype: Milestone
tags: [trajectory, roadmap, v1.0, indexer, redpanda, search]
related:
  - ADR-016
  - ADR-020
  - RFC-014
  - RFC-022
  - MEMO-053
  - MEMO-054
  - MEMO-058
---

# Trajectory T2 — Event-Driven Indexer Cutover

> Completes the v1.0-critical slice of RFC-014: the event-driven indexer reproduces the legacy search projection, runs in parallel, validates parity, and **deletes** the legacy indexer. LLM summaries, embeddings, and semantic search are optional v1.x enrichment unless explicitly promoted by a later release decision.

## Readiness status

Phase 0 can start after the adversarial review. The critical path is now the `search_index` cutover only: canonical search projection, stateless-worker compliance, release-artifact rollback, and deterministic tests must pass before production cutover. LLM and embedding work may proceed only as an enrichment track that cannot block legacy indexer deletion.

## Stable-release criteria served

- v1.0 criterion **#3 Indexer cutover** (legacy indexer removed, not just disabled).

## Scope

In scope:

- `search_index` pipeline step completion through `search.Provider`, with the frontend still behind backend `/1/indexes/*` per ADR-016.
- Canonical indexed-document projection for parity comparisons, excluding intentionally new enrichment fields.
- Parallel-run + parity validation harness comparing legacy vs new indexer search output.
- ADR-020 compliance gates proving `cmd/hermes-indexer` has no DB DSN and persists execution results only through server-owned APIs.
- Release-artifact rollback procedure and staging drill before legacy code deletion.
- Removal of legacy indexer code, configuration, and binary.
- Updated operator docs.

Out of scope:

- LLM summaries and embeddings as v1.0 blockers; they are v1.x enrichment unless promoted by a separate release decision.
- Additional pipeline rulesets beyond `search_index`, `llm_summary`, `embeddings` (deferred to v1.x).
- Multi-tenant LLM key management beyond per-instance config (v1.x).

## Dependencies

- T1 (T2 already uses its own outbox via RFC-014; no hard dep, but coordinate relay interface).
- Redpanda/Kafka deployment — already running in testing and production.

## Phases & exit criteria

### Phase 0 — Search-index cutover minimum

- Define the canonical indexed-document projection used by both legacy and event-driven indexers.
- Define parity normalization rules for timing, ranking, provider-generated metadata, and enrichment fields.
- Confirm `search_index` writes only through `search.Provider` and does not introduce a vector/search side channel outside backend-owned search APIs.
- Confirm execution-result persistence is server-owned or API-mediated; the worker must not write `document_revision_pipeline_executions` directly.
- Add an ADR-020 compliance gate: no database DSN or database driver dependency in `cmd/hermes-indexer` runtime config.

**Exit when:**

- A fixed-corpus parity fixture exists with canonical JSON output for the legacy search projection.
- A worker test proves the indexer starts and processes `search_index` without DB credentials in its environment.
- API-mediated execution-result recording is covered if execution status must persist.
- The plan for release-artifact rollback is documented and ready for a staging drill.

### Phase 1 — Search pipeline + integration

- `search_index` step reproduces the canonical projection through the configured `search.Provider`.
- Pipeline executor submits step start/result/failure state through the central Hermes API when persistence is required.
- Test coverage uses deterministic fake providers or local services; live external provider tests are opt-in smoke runs, not required CI.

**Exit when:**

- testcontainers integration test: create document -> publish to Redpanda -> consume -> `search_index` projection appears in Meilisearch/Algolia-compatible provider, all within 30 s.
- Fixed-corpus parity test compares canonical JSON projection and ignores only fields listed in the projection spec.
- The worker binary/test environment has no DB DSN; attempts to configure one fail validation.

### Phase 2 — Parallel run & parity validation

- Run legacy and new indexers concurrently in a staging environment.
- Parity job samples N documents/hour and compares the canonical search projection; differences are logged with a structured reason code.
- Burn down all parity differences that are not explained by projection spec, provider timing, or documented upstream workspace changes.
- Rehearse restoring the previous production indexing path from release artifacts and config, not source history.

**Exit when:**

- 7-day window with zero unexplained parity differences over ≥10,000 sampled documents.
- Load test: 1,000 docs/hour sustained; consumer lag < 100 messages at peak.
- Staging rollback drill restores the previous indexing binary/config from release artifacts and proves search catches up after rollback.

### Phase 3 — Cutover & legacy removal

- Cut production traffic to the new indexer.
- After a 7-day soak with rollback artifacts retained, **delete** the legacy indexer code, binary target, configuration, and docs.
- Mark the search-index cutover slice of RFC-014 implemented; keep RFC-014 open or split follow-up work if LLM/embedding enrichment remains.

**Exit when:**

- `cmd/hermes-indexer-legacy` (or equivalent) is removed; `git log` shows the deletion.
- Roadmap Implementation Tracker updated to reflect v1.0 indexer cutover complete.
- The `legacy_indexer.*` config keys are removed from all `testing/*.hcl` and example configs.

### Phase 4 — Optional LLM and embedding enrichment (v1.x)

- Finish OpenAI, Ollama, and Bedrock clients with token usage tracking, rate limiting, and circuit breakers.
- `llm_summary` step generates summaries end-to-end using fake/Ollama in required tests; live OpenAI/Bedrock tests are opt-in smoke runs with explicit credentials.
- `embeddings` step writes vectors only through provider-specific abstractions or feature flags that do not bypass `search.Provider`/backend search boundaries.

**Exit when:**

- All enabled clients implement a shared `llm.Client` interface with deterministic tests.
- Token budgets, per-provider circuit breakers, and metrics are enabled before production-size runs.
- Provider-specific embedding semantics are documented before enabling semantic search in production.

## Reviews / gates

Each phase exit-only. Update [Roadmap Implementation Tracker](roadmap-tracker.md) progress on every phase boundary. Promote or split RFC-014 only when the implemented search cutover and remaining enrichment scope are accurately represented.

## Risks

- **Scope creep from LLM/embedding enrichment.** Mitigation: enrichment is Phase 4/v1.x and cannot block v1.0 legacy indexer deletion unless a later release decision explicitly promotes it.
- **Parity drift caused by upstream Google API changes.** Mitigation: parity reason codes distinguish "upstream change" from "indexer bug", and canonical projection ignores only documented non-semantic fields.
- **Cutover rollback complexity.** Mitigation: retain previous release artifacts and config for 30 days post-cutover; rehearse artifact restore in staging before deleting legacy code from `main`.
- **Stateless-indexer boundary regression.** Mitigation: ADR-020 gate forbids DB DSNs and database driver dependencies in the indexer worker; execution persistence goes through the central API.
- **LLM cost overruns in enrichment.** Mitigation: hard token-budget cap per executor with circuit breaker before any production-size enrichment run; default to fake/Ollama in required tests.

## References

- [RFC-014: Event-Driven Indexer with Pipeline Rulesets](../rfc/rfc-014-event-driven-indexer.md)
- [ADR-016: Backend-Mediated Search and Runtime Auth Header Selection](../adr/adr-016-search-and-auth-refactoring.md)
- [ADR-020: Core+Deltas Migrations and Stateless Indexer](../adr/adr-020-dual-database-support-stateless-indexer.md)
- [RFC-022: Database Deltas and Stateless Indexer](../rfc/rfc-022-database-deltas-and-stateless-indexer.md)
- [MEMO-053](../memo/memo-053-event-driven-indexer-summary.md), [MEMO-054](../memo/memo-054-event-driven-indexer-production-deployment.md), [MEMO-058](../memo/memo-058-event-driven-indexer-testing-status.md)
- [Roadmap Implementation Tracker](roadmap-tracker.md)
