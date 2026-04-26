---
id: adr-048
title: "Environment-Agnostic OAuth Redirect"
status: Accepted
decision_type: Architectural Pattern
created: 2025-10-08
deciders: Hermes Team
author: Hermes Team
project_id: hermes
doc_uuid: da31eae5-c771-4a30-9541-6c4dbe8d6722
date: 2025-10-08
type: ADR
subtype: Backend Decision
tags: [backend, auth, oauth, redirect, configuration]
related:
  - ADR-076
  - ADR-077
  - ADR-078
---

# ADR-048: Environment-Agnostic OAuth Redirect

> OAuth `redirect_uri` always points to the backend. After the backend completes the token exchange and sets the session cookie, it redirects the browser to the frontend via the configured `base_url`. Frontend URL is never embedded in OAuth provider configuration.

## Context

OAuth/OIDC token exchange must happen server-side: the `redirect_uri` registered with the OAuth provider receives the authorization code, and only the backend can exchange it (the client secret never leaves the server). But the backend port is rarely the user-facing URL — local testing runs the Ember dev server on `4201` while the backend listens on `8001`; production fronts everything behind a single domain. Hard-coding either URL into OAuth provider configuration breaks portability across environments (local dev, testing, production).

A related bug surfaced this requirement: in the testing environment, `/api/v2/me` returned "Guest User" instead of the authenticated user. Root causes were `providers { workspace = "google" }` (should have been `"local"`) and an uninitialized local `ProviderAdapter`. Fixing the adapter exposed that the OAuth callback still landed the user on the backend port instead of the Ember dev server, which led to the redirect rule below.

## Decision

1. **OAuth `redirect_uri` is always a backend URL** (e.g. `http://localhost:8001/auth/callback`, `https://api.hermes.example.com/auth/callback`). Registered once per environment with the OAuth provider; never points at the frontend.

2. **After token exchange, the backend redirects to `base_url + <path>`** (typically `/dashboard`). `base_url` is environment-specific HCL configuration:

   ```hcl
   # testing/config.hcl
   base_url = "http://localhost:4201"   # Ember dev server
   dex { redirect_url = "http://localhost:8001/auth/callback" }

   # native dev
   base_url = "http://localhost:4200"
   dex { redirect_url = "http://localhost:8000/auth/callback" }

   # production
   base_url = "https://hermes.company.com"
   ```

3. **`base_url` must be set explicitly per environment.** No auto-detection from `Referer` or `Host` headers (security risk; redirect-target spoofing).

## Consequences

### Positive
- One OAuth client configuration works across deployment topologies (split frontend/backend ports, single-origin production).
- Frontend never holds the client secret or processes the authorization code (preserves OAuth security model).
- Adding a new environment requires only setting `base_url` and registering one new `redirect_uri` with the OAuth provider.

### Negative
- `base_url` must be configured manually for every environment; no auto-detection.
- The OAuth callback briefly lives on the backend port even in dev, which can confuse operators tailing logs.
- Two URLs to keep in sync (provider `redirect_uri` ↔ `base_url`) when an environment moves.

## Alternatives Considered

- **Auto-detect frontend URL from `Referer`** — rejected: `Referer` after the OAuth provider round-trip is the provider's URL, not the frontend; spoofable.
- **Relative redirects** — rejected: redirect lands on the backend port, not the frontend.
- **Frontend processes the OAuth callback** — rejected: violates OAuth security model; would expose client secret.
- **Single proxy that fronts backend and frontend on one port** — rejected: complex local setup, doesn't match production architecture.

## References

- Code: `internal/cmd/commands/server/server.go` (workspace-provider initialization), `pkg/auth/dex/` (callback handler).
- [ADR-076: Multi-Provider Auth Architecture](adr-076-multi-provider-auth-architecture.md) — broader auth model.
- [ADR-078: Dex Authentication Implementation](adr-078-dex-authentication-implementation.md) — concrete OIDC flow.
