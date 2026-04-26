---
id: adr-078
title: Dex OIDC Backend Implementation
date: 2025-10-01
type: ADR
subtype: Authentication
decision_type: Authentication
status: Accepted
tags: ['authentication', 'dex', 'oidc']
related: ['ADR-072', 'ADR-073', 'ADR-076']
created: 2025-10-01
deciders: Hermes Team
project_id: hermes
doc_uuid: e6956bd2-999d-433b-9021-0779a9f5b4ce
---
# Dex OIDC Backend Implementation

> The Dex auth path implements `auth.Provider` in **`pkg/auth/adapters/dex/`** and is wired into the standard backend OIDC endpoints — `GET /auth/login`, `GET /auth/callback`, `GET /auth/logout`. Sessions are issued as **HttpOnly + Secure + SameSite=Lax** cookies by `DexSessionProvider` in `internal/auth/auth.go`. Two stack profiles exist: integration tests (port `5556`, client `hermes-integration`) and the testing Compose stack (port `5557`, client `hermes-acceptance`).

## Context

ADR-072 chose Dex as the local OIDC provider. This ADR records how the Hermes side of that integration is built: which packages own the adapter and the session, which HTTP endpoints participate in the flow, and how the integration vs acceptance configurations stay isolated from each other (per the +1-port convention in ADR-070).

## Decision

- **Adapter:** `pkg/auth/adapters/dex/` — implements `auth.Provider` (ADR-073), validates OIDC ID tokens, and extracts the user email from claims.
- **HTTP endpoints (`internal/api/auth.go`):**
  - `GET /auth/login` — start OIDC authorization flow.
  - `GET /auth/callback` — exchange code, issue session cookie.
  - `GET /auth/logout` — clear session.
- **Session:** `DexSessionProvider` in `internal/auth/auth.go` — cookie-based, HttpOnly + Secure + SameSite=Lax (consistent with ADR-076).
- **Profiles:**
  - **Integration** (`docker-compose.yml`): Dex on `5556` HTTP / `5558` telemetry, client `hermes-integration`, issuer `http://localhost:5556/dex`.
  - **Acceptance** (`testing/docker-compose.yml`): Dex on `5557` / `5559`, client `hermes-acceptance`, issuer `http://dex:5557/dex` (in-network).
- **Frontend:** `web/app/routes/authenticated.ts` performs a `HEAD /api/v2/me` check and redirects to `/auth/login?redirect=<url>` if unauthenticated; the dev proxy (`web/server/index.js`) forwards `/auth/*` and `/api/*` to the backend so the Ember router does not intercept them.
- **Test users:** `test@hermes.local` / `password`, `admin@hermes.local` / `password` (defined in Dex static-password connector, ADR-072).

## Consequences

### Positive
- Dex slots into the same `auth.Provider` and session-cookie machinery as Okta; no special-case handler code.
- Integration and acceptance profiles cannot collide (different ports, different client IDs/secrets).
- Frontend auth gating and dev proxying are uniform across providers.

### Negative
- Two Dex configurations to keep in sync as flows evolve.
- Dev proxy must explicitly forward `/auth/*` — easy to forget when adding new auth endpoints.

## Alternatives Considered

- **Single Dex profile shared by integration and acceptance:** Causes port conflicts and cross-test interference; the +1 offset (ADR-070) exists exactly to prevent this.
- **Token-in-`localStorage` instead of session cookies:** Inconsistent with the rest of the OIDC story (ADR-076) and reopens XSS exfiltration risk.

## References

- `pkg/auth/adapters/dex/`, `internal/api/auth.go`, `internal/auth/auth.go`
- `testing/dex-config.yaml`, `testing/docker-compose.yml`
- ADR-072 (Dex selection), ADR-073 (provider abstraction), ADR-076 (multi-provider auth)
