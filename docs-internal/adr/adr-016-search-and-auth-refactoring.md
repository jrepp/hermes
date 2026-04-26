---
id: adr-016
title: Backend-Mediated Search and Runtime Auth Header Selection
type: ADR
subtype: Frontend Architecture
decision_type: Frontend Architecture
status: Accepted
tags: [algolia, authentication, frontend, meilisearch, search]
related: [ADR-009, ADR-011, ADR-012]
created: 2025-10-06
deciders: Hermes Team
project_id: hermes
doc_uuid: 890ca48f-ff1b-4290-a371-0622eda66de7
---

# Backend-Mediated Search and Runtime Auth Header Selection

> The frontend **never holds search-provider credentials**. All search traffic goes through the backend at **`/1/indexes/*`** in every environment, regardless of whether the backend is fronting Algolia or Meilisearch. The frontend selects its auth header at runtime from `auth_provider` on `GET /api/v2/web/config`: **`Hermes-Google-Access-Token`** for Google, **`Authorization: Bearer <jwt>`** for Okta and Dex.

## Context

Two prior compromises were causing real problems:

1. **Search:** The frontend talked directly to Algolia in development and proxied through the backend in production. Dev builds therefore required `ALGOLIA_APP_ID` / `ALGOLIA_SEARCH_API_KEY`, behavior diverged between environments, and Docker setups failed without real Algolia credentials.
2. **Auth headers:** Frontend code assumed Google or Okta and hard-coded which header to use. There was no path for Dex (ADR-008, ADR-014) and no runtime flexibility.

Both problems pushed environment-specific logic and secrets into the frontend bundle, which is the wrong place for either.

## Decision

**1. Backend-mediated search in all environments.**
- The frontend always calls `/1/indexes/*` on the backend (`web/app/services/algolia.ts`); the per-environment branch and the build-time Algolia env vars in `web/config/environment.js` are removed.
- The backend's `search.Provider` (ADR-009) decides whether to delegate to Algolia or Meilisearch (ADR-011). The frontend cannot tell the difference.
- Mirage mocks the backend proxy endpoints, not Algolia hosts.

**2. Runtime auth header selection.**
- Backend publishes the active provider as `auth_provider` (and any needed provider-specific fields, e.g. `dex_issuer_url`, `dex_client_id`) on `GET /api/v2/web/config`.
- `web/app/services/fetch.ts` chooses the header at request time:
  - `google` → `Hermes-Google-Access-Token: <token>`
  - `okta` | `dex` → `Authorization: Bearer <jwt>`
- One bundle ships for every provider; no rebuild required to switch.

## Consequences

### Positive
- No search credentials in the browser; revocation, rate-limiting, and provider swaps are all server-side concerns.
- Identical search behavior across dev, CI, and production (modulo Algolia↔Meilisearch differences, which are bounded by `search.Provider`).
- Adding an OIDC provider is "publish a new `auth_provider` value and add a backend adapter" — no frontend conditional explosion.
- Docker Compose works without real Algolia or Google OAuth env vars.

### Negative
- Backend becomes the single point of failure / latency for search; the proxy must be performant.
- The frontend depends on `/api/v2/web/config` early in startup; auth-aware services must wait for it.

## Alternatives Considered

- **Keep direct-to-Algolia in dev:** Fast for dev but reproduces the credential-leak and dev/prod divergence the team just fixed.
- **Build-time provider selection:** Requires a separate bundle per environment; cuts off the runtime override (ADR-013) and per-tenant flexibility.

## References

- `web/app/services/algolia.ts`, `web/app/services/fetch.ts`, `web/config/environment.js`
- `web/web.go` (config response), `web/mirage/algolia/hosts.ts`
- ADR-009 (provider abstraction), ADR-011 (Meilisearch), ADR-012 (multi-provider auth)