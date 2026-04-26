---
id: adr-008
title: Dex OIDC for Local Development
type: ADR
subtype: Authentication
decision_type: Authentication
status: Accepted
tags: [authentication, development, dex, oidc]
related: [ADR-006, ADR-012, ADR-014]
created: 2026-04-24
deciders: Hermes Team
project_id: hermes
doc_uuid: b1c6dc1c-a68b-4b12-86cf-be97dcd38823
---

# Dex OIDC for Local Development

> Local development and acceptance testing use **Dex `v2.41.1`** (`ghcr.io/dexidp/dex`) with the **static-password connector** as the OIDC provider. It runs in `testing/docker-compose.yml`, is configured via `testing/dex-config.yaml`, and is selected by `providers.auth = "dex"`. Dex is **not** a production auth option.

## Context

Hermes' production auth stack is OIDC-based (Google, Okta). Local development needed a real OIDC provider — not a mock — so that the same code paths run locally and in production, but without external accounts, network access, or rate limits. The provider had to start in seconds, support multiple test users, and be safe to commit secrets for.

## Decision

Run Dex as a containerized OIDC provider in the testing stack with a static-password connector. Hermes' Dex auth adapter (`pkg/auth/adapters/dex/`) implements the same `auth.Provider` interface (ADR-009, ADR-012) as the Google and Okta adapters; switching is purely a configuration change.

**Pinned version:** `v2.41.1`. **Issuer:** `http://dex:5557/dex` inside the Docker network. **Test users:** `test@hermes.local` and `admin@hermes.local`, password `password` (bcrypt hashes in `dex-config.yaml`).

## Consequences

### Positive
- Real OIDC flow exercised locally; no mock divergence from production.
- Container starts in ~2 s; no external service, no quota, works offline and in CI.
- Adding a test user is a YAML edit plus `docker compose restart dex`.

### Negative
- Static passwords and HTTP-only — explicitly **not** for production.
- No MFA, password reset, or directory integration; intentional for dev simplicity.

## Alternatives Considered

- **Mock OAuth (no real OIDC):** Simplest, but defeats the purpose of testing the real flow.
- **Keycloak:** Full-featured IAM, but ~1 GB RAM and ~30 s startup — too heavy for a dev loop.
- **Auth0 / Okta free tier:** Real OIDC, but reintroduces external dependency, accounts, and rate limits.
- **Hand-rolled JWT minter:** Trivial to write but not OIDC-compliant; would mask real integration bugs.

## References

- `testing/dex-config.yaml`, `pkg/auth/adapters/dex/`
- ADR-012 (multi-provider auth), ADR-014 (Dex implementation), ADR-006 (testing stack)