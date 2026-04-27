---
id: trajectory-002-adversarial-review
title: "Adversarial Review — T2 Event-Driven Indexer Cutover"
status: Final
created: 2026-04-27
date: 2026-04-27
author: Hermes Team
project_id: hermes
doc_uuid: b406891b-db5f-4ec2-b86f-f414ccc95545
type: Memo
subtype: Analysis
tags: [trajectory, adversarial-review, v1.0, indexer, redpanda, llm]
related:
  - ADR-016
  - ADR-020
  - RFC-014
  - RFC-022
---

# Adversarial Review — T2 Event-Driven Indexer Cutover

> This review tried to break [Trajectory T2](trajectory-002-indexer-cutover.md) before execution. The blocking decisions are now folded into the source plan: v1.0 is scoped to the search-index cutover, while LLM summaries, embeddings, and semantic search are optional v1.x enrichment.

## Run-readiness verdict

Phase 0 can start. Do not start production cutover until the source plan's Phase 0-2 gates are complete: canonical search projection, stateless-worker proof, deterministic parity tests, release-artifact rollback drill, and no unexplained staging parity differences.

## Resolution summary

- **Scope resolved:** the source plan now makes `search_index` the only v1.0-critical path; LLM summaries, embeddings, and semantic search moved to optional Phase 4/v1.x enrichment.
- **ADR-020 boundary resolved:** the source plan now requires no DB DSN or database driver dependency in `cmd/hermes-indexer`, and execution persistence must be server-owned or API-mediated.
- **Parity resolved:** the source plan now requires a fixed-corpus fixture and canonical JSON projection that ignores only documented fields.
- **Rollback resolved:** the source plan now requires a staging rollback drill from release artifacts and config, not a git tag.
- **Testing resolved:** required CI uses deterministic fake/local providers; live OpenAI/Bedrock tests are opt-in smoke runs.
- **Cost control resolved for enrichment:** token budgets, metrics, and circuit breakers must exist before production-size enrichment runs.

## Highest-risk failure modes

1. **The plan expands v1.0 scope beyond the stable-release criterion.** Resolved: the source plan now defines the v1.0 slice as `search_index` cutover only and moves LLM/embedding work to optional Phase 4.

2. **The stateless-indexer boundary is easy to violate.** Resolved in plan: [ADR-020](../adr/adr-020-dual-database-support-stateless-indexer.md) compliance is now a Phase 0-1 gate, with no DB DSN in worker config and API-mediated execution-result persistence.

3. **Parallel parity can compare the wrong thing.** Resolved in plan: Phase 0 defines the canonical indexed-document projection and normalization rules before staging parity begins.

4. **Deletion of the legacy indexer is framed as irreversible without a safe operational fallback.** Resolved in plan: Phase 2 requires a staging rollback drill using release artifacts and config restore.

5. **LLM external dependencies make CI and staging nondeterministic.** Resolved in plan: required checks use deterministic fake/local providers; live-provider checks are opt-in smoke runs.

6. **Cost control is too late.** Resolved for enrichment: token budgets, circuit breakers, and metrics are required before production-size LLM/embedding runs.

7. **Search provider constraints are under-specified.** Resolved in plan: `search_index` must go through `search.Provider`, and enrichment vectors require provider-specific abstractions or feature flags that do not bypass [ADR-016](../adr/adr-016-search-and-auth-refactoring.md).

## Missing decisions before execution

- LLM summaries and embeddings are v1.x enrichment unless a later release decision promotes them.
- Canonical indexed-document projection is a Phase 0 exit criterion.
- Pipeline execution persistence is server-owned or API-mediated under ADR-020.
- Release-artifact rollback is a Phase 2 exit criterion.
- Provider-specific embedding semantics are a Phase 4 exit criterion before production semantic search.
- Live LLM tests are opt-in smoke runs; deterministic fake/local tests are required CI.

## Concrete pre-flight checklist

- Source plan now splits v1.0 `search_index` cutover from optional Phase 4 enrichment.
- Phase 0 now requires a fixed-corpus parity fixture and canonical JSON projection.
- Phase 0-1 now require a worker test proving no DB credentials are needed.
- Phase 0 now requires API-mediated execution-result recording coverage if status persists.
- Phase 4 now requires token budgets and circuit breakers before production-size enrichment runs.
- Phase 2 now requires a release-artifact rollback drill in staging before deleting legacy code.

## Suggested plan edits

- Done: Phase 1's integration test is now `search_index` only; `llm_summary` and `embeddings` are optional Phase 4 enrichment gates.
- Done: required tests now use deterministic fake/local providers; live-provider tests are opt-in smoke runs.
- Done: rollback mitigation now uses release artifacts and config restore verified in staging.
- Done: ADR-020 compliance now explicitly forbids DB DSNs and database driver dependencies in `cmd/hermes-indexer`.

## Go/no-go gate

Start Phase 0 now. Start production cutover only when the new indexer can reproduce the legacy search projection from a fixed corpus without DB access, staging has run for 7 days with zero unexplained parity differences, and rollback to the previous production indexing path has been rehearsed from release artifacts.
