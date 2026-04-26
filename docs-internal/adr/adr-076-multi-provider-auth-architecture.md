---
id: adr-076
title: Multi-Provider Auth Architecture
date: 2025-10-06
type: ADR
subtype: Authentication
decision_type: Authentication
status: Accepted
tags: ['authentication', 'multi-provider', 'dex', 'okta', 'google']
related: ['ADR-073', 'ADR-077', 'ADR-078', 'MEMO-126']
created: 2025-10-06
deciders: Hermes Team
project_id: hermes
doc_uuid: 1a9504f2-0b24-4a65-bfe4-fc6f117aba60
---
# Multi-Provider Auth Architecture

> Hermes supports multiple authentication providers behind `auth.Provider`: **Google OAuth (client-centric)**, **Okta OIDC (backend-centric)**, and **Dex OIDC (backend-centric, dev/test only)**. The active provider is published to the frontend at **`GET /api/v2/web/config`** (`auth_provider` field), and the frontend selects the auth header at runtime. Tokens are **never** stored in `localStorage`. **Torii is not used.**

## Context

Hermes runs in environments with very different identity stories: hashicorp.com Google Workspace in production, Okta for enterprise customers, and Dex with static passwords for local dev and CI (ADR-072). Auth flows split into two shapes — client-side OAuth (Google popup → access token) and server-side OIDC (redirect → backend code exchange → session cookie) — and the frontend has to handle both without a build-time choice.

## Decision

Make the backend the single source of truth for which provider is active, and have the frontend adapt at runtime.

**Provider classes:**

- **Google OAuth (client-centric):** ember-simple-auth manages the popup; frontend sends the access token via the `Hermes-Google-Access-Token` header.
- **OIDC — Okta and Dex (backend-centric):** Frontend redirects to `/api/v2/auth/{provider}/login`; backend handles the OIDC code exchange and issues an **HttpOnly + Secure + SameSite=Lax session cookie**. Bearer tokens used in subsequent requests travel as `Authorization: Bearer <jwt>`.

**Runtime selection:**

- Backend exposes `auth_provider` (and any provider-specific fields like `dex_issuer_url`) on `GET /api/v2/web/config`.
- Frontend `ConfigService` loads it at startup; `SessionService`, `FetchService`, and `SearchService` consult it.
- Header selection is purely runtime — one bundle ships for all providers.

**No Torii:** Torii targets client-side OAuth popups and does not model backend-driven OIDC redirects, so it would have to be worked around for Okta/Dex. ember-simple-auth covers the Google popup; OIDC is handled entirely by the backend.

## Consequences

### Positive
- One frontend bundle works against any supported provider; switching is configuration plus a backend restart.
- Tokens for OIDC providers live in HttpOnly cookies, not in `localStorage` — significantly reduces XSS exfiltration risk.
- Adding a new OIDC IdP is an `auth.Provider` adapter (ADR-073) plus a config switch; no frontend code change.

### Negative
- Two distinct auth code paths (header-based vs cookie-based) must be maintained and tested.
- Frontend must wait for `/api/v2/web/config` before issuing authenticated requests; auth-aware services have a small startup dependency.

## Alternatives Considered

- **Use Torii for everything:** Forces backend OIDC into a popup model it wasn't designed for; more complexity than skipping it.
- **Pick one provider per build:** Simpler client code but requires per-environment builds and breaks the "same artifact, any IdP" property.
- **Store OIDC tokens in `localStorage`:** Marginally simpler but reopens the XSS risk this ADR is designed to close.

## References

- `internal/api/auth.go`, `internal/auth/auth.go`, `pkg/auth/`
- `web/app/services/session.ts`, `web/app/services/fetch.ts`, `web/app/services/config.ts`
- ADR-073 (provider abstraction), ADR-077 (CLI/env override), ADR-078 (Dex implementation), MEMO-126 (auth flow diagrams)
