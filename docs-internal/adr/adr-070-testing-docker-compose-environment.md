---
id: adr-070
title: Testing Docker Compose Environment
date: 2025-10-09
type: ADR
subtype: Infrastructure
decision_type: Infrastructure
status: Accepted
tags: ['infrastructure', 'docker', 'testing', 'environment']
related: ['ADR-072', 'ADR-075']
created: 2026-04-24
deciders: Hermes Team
project_id: hermes
doc_uuid: 735b0ba9-9507-4a9d-a1ee-6dd1d34d4728
---
# Testing Docker Compose Environment

> The `testing/` Docker Compose stack runs the full Hermes integration environment on **+1-offset ports** so it never collides with native development or integration-test fixtures: hermes `8001`, postgres `5433`, meilisearch `7701`, dex `5557/5559`, web `4201`.

## Context

Hermes needs a reproducible local stack — backend, database, search, OIDC, and optionally the frontend — that runs without Google Workspace, Algolia, or any external SaaS. Native dev and integration tests already occupy the canonical ports (`8000`, `5432`, `7700`, `5556/5558`, `4200`), so the testing stack must coexist with them.

## Decision

Provide `testing/docker-compose.yml` with a fixed, documented +1 port offset and an HCL config (`testing/config.hcl`) wiring the local workspace, Meilisearch, and Dex providers. The stack is the canonical target for E2E tests and agent-driven exploration.

**Port allocation:**

| Service     | Native | Integration | Testing   | Production |
|-------------|--------|-------------|-----------|------------|
| Backend     | 8000   | —           | **8001**  | 8080       |
| Frontend    | 4200   | —           | **4201**  | —          |
| Postgres    | 5432   | 5432        | **5433**  | 5432       |
| Meilisearch | 7700   | —           | **7701**  | 7700       |
| Dex         | —      | 5556/5558   | **5557/5559** | —      |

The stack supports three modes: full Docker, hybrid (native frontend → Docker backend on `8001`), and infra-only (native backend, containerized Postgres/Meilisearch/Dex).

## Consequences

### Positive
- No conflicts with native dev or integration tests; both stacks can run concurrently.
- Single `docker compose up` produces a complete environment with no external credentials.
- Numbers are memorable (`+1` from canonical), so the offset is easy to teach and recall.

### Negative
- Two sets of ports to remember; documentation must stay current.
- Backend changes require a rebuild (no in-container hot reload).

## Alternatives Considered

- **Kubernetes (kind/minikube):** Production-like but slow startup and overkill for a dev loop.
- **Dynamic port allocation:** Avoids conflicts but breaks bookmarks, scripts, and muscle memory.
- **Shared DB with integration tests:** Causes data pollution and flakiness; isolation is non-negotiable.

## References

- `testing/docker-compose.yml`, `testing/config.hcl`, `testing/readme.md`
- ADR-072 (Dex), ADR-075 (Meilisearch)
