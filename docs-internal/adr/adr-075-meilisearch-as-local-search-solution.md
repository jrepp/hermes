---
id: adr-075
title: Meilisearch as Local Search Provider
date: 2025-10-09
type: ADR
subtype: Infrastructure
decision_type: Infrastructure
status: Accepted
tags: ['infrastructure', 'search', 'meilisearch', 'algolia']
related: ['ADR-070', 'ADR-073', 'ADR-080']
created: 2026-04-24
deciders: Hermes Team
project_id: hermes
doc_uuid: 68c866e8-bcce-4a0d-9b10-7fbb58379b90
---
# Meilisearch as Local Search Provider

> Local development, CI, and acceptance testing use **Meilisearch `v1.11`** (`getmeili/meilisearch`) as the search backend. Production uses **Algolia**. Both sit behind `search.Provider` (ADR-073), and selection is `providers.search = "meilisearch" | "algolia"` in HCL. The frontend never talks to either directly — all search goes through the backend `/1/indexes/*` API (ADR-080).

## Context

Search is a core feature of Hermes, but using Algolia for development meant every developer and every CI job needed API keys, paid for requests, polluted a shared index, and incurred 100–300 ms of network latency per query. We needed a local search engine that supported full-text search, typo tolerance, faceted filtering, and highlighting — the features Hermes actually uses — without changing handler code or shipping Algolia credentials to the frontend.

## Decision

Adopt Meilisearch as the local/CI search provider behind the existing `search.Provider` interface, keep Algolia as the production provider, and pin a specific Meilisearch version so behavior is reproducible.

- **Pinned version:** `v1.11`.
- **Deployment:** containerized in `testing/docker-compose.yml` on port `7701` (ADR-070).
- **Adapter:** `pkg/search/meilisearch/` implementing `search.Provider`; the `SearchResult` shape is normalized so handlers cannot tell which provider answered.
- **Production parity:** the features Hermes relies on — search, typo tolerance, facets, highlighting, synonyms, stop words — are supported by both providers. Vendor-specific extras (Algolia Analytics, A/B testing, Personalization) are deliberately **not** part of the abstraction.

## Consequences

### Positive
- Zero credentials, zero network, zero cost for local and CI search.
- Per-developer / per-CI-job index isolation eliminates shared-state flakiness.
- Adapter abstraction means production-vs-dev provider differences cannot leak into handlers.
- Frontend stays credential-free because it only talks to the backend (ADR-080).

### Negative
- Two search backends to keep in sync as Hermes grows; vendor-specific features are out-of-bounds for the core product.
- Subtle relevance/ranking differences between Meilisearch and Algolia are possible; relevance regressions must be caught in staging, not local dev.
- One more service in the local stack (~40 MB RAM, negligible CPU).

## Alternatives Considered

- **Elasticsearch:** Powerful but ~1 GB RAM and JVM-heavy; far more than Hermes needs locally.
- **Typesense:** Comparable, but Meilisearch had more momentum and better docs at decision time.
- **Bleve (embedded Go):** Avoids a service but is materially slower and weaker on relevance; would not match production behavior.
- **PostgreSQL full-text search:** No extra service, but no real typo tolerance and weaker ranking; poor match for the search UX.
- **Algolia everywhere:** Perfect parity, but reintroduces every pain point this ADR exists to fix.

## References

- `pkg/search/meilisearch/`, `pkg/search/algolia/`
- ADR-070 (testing stack), ADR-073 (provider abstraction), ADR-080 (backend-mediated search)
