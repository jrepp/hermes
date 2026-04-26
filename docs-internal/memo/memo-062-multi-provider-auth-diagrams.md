---
id: memo-062
title: Multi-Provider Auth Architecture (Diagrams)
status: Reference
created: 2026-04-24
author: Hermes Team
project_id: hermes
doc_uuid: 45308585-fccc-494b-95c7-8901f13e942b
type: MEMO
tags: [auth, oidc, oauth, diagrams, reference]
supersedes: MEMO-062
related: [ADR-012, ADR-013, ADR-014]
---

# MEMO-062: Multi-Provider Auth Architecture (Diagrams)

> Demoted from MEMO-062. This memo holds the reference diagrams for the multi-provider auth flows; the binding architectural decisions live in [ADR-012](../adr/adr-012-multi-provider-auth-architecture.md), [ADR-013](../adr/adr-013-auth-provider-selection.md), and [ADR-014](../adr/adr-014-dex-authentication-implementation.md). Diagrams alone are not a decision.

## System Overview

```text
┌─────────────────────────────────────────────────────────────────────┐
│                         Hermes Frontend                             │
│                         (Ember.js App)                              │
└─────────────────────────────────────────────────────────────────────┘
                                 │
                    Runtime Config Detection
                                 │
                    ┌────────────┴────────────┐
                    │   ConfigService         │
                    │   auth_provider: string │
                    └────────────┬────────────┘
                                 │
                ┌────────────────┼────────────────┐
                │                │                │
         auth_provider     auth_provider    auth_provider
            = "google"        = "okta"         = "dex"
                │                │                │
                ▼                ▼                ▼
    ┌─────────────────┐  ┌─────────────┐  ┌─────────────┐
    │ Google OAuth    │  │ OIDC (Okta) │  │ OIDC (Dex)  │
    │ Flow            │  │ Flow        │  │ Flow        │
    └─────────────────┘  └─────────────┘  └─────────────┘
```

## Authentication Flows

### Google OAuth Flow (No Torii Needed)

```text
┌──────┐                ┌──────────┐              ┌─────────┐           ┌────────┐
│ User │                │ Frontend │              │ Backend │           │ Google │
└──┬───┘                └────┬─────┘              └────┬────┘           └───┬────┘
   │                         │                         │                    │
   │  Click "Sign in"        │                         │                    │
   ├────────────────────────►│                         │                    │
   │                         │                         │                    │
   │                         │ session.authenticate()  │                    │
   │                         │ (ember-simple-auth)     │                    │
   │                         ├─────────┐               │                    │
   │                         │◄────────┘               │                    │
   │                         │                         │                    │
   │                         │ Open OAuth popup        │                    │
   │                         ├─────────────────────────┼───────────────────►│
   │                         │                         │                    │
   │  ◄──── Auth Dialog ────►│                         │                    │
   │                         │                         │                    │
   │                         │◄────────────────────────┼────────────────────┤
   │                         │      Access Token       │                    │
   │                         │                         │                    │
   │                         │ Store in session        │                    │
   │                         ├─────────┐               │                    │
   │                         │◄────────┘               │                    │
   │                         │                         │                    │
   │                         │ API Call with header:   │                    │
   │                         │ Hermes-Google-Access-   │                    │
   │                         │ Token: <token>          │                    │
   │                         ├────────────────────────►│                    │
   │                         │◄────────────────────────┤                    │
   │                         │    Protected Resource   │                    │
```

Key points:
- Frontend manages OAuth flow directly.
- Uses ember-simple-auth (NOT Torii library).
- Custom header: `Hermes-Google-Access-Token`.
- Token stored in browser session.

### OIDC Flow (Okta/Dex) — Backend-Centric

```text
┌──────┐              ┌──────────┐                ┌─────────┐           ┌──────────┐
│ User │              │ Frontend │                │ Backend │           │ OIDC     │
└──┬───┘              └────┬─────┘                └────┬────┘           └────┬─────┘
   │                       │                           │                     │
   │  Click "Sign in"      │                           │                     │
   ├──────────────────────►│                           │                     │
   │                       │                           │                     │
   │                       │ Redirect to:              │                     │
   │                       │ /api/v2/auth/{provider}/  │                     │
   │                       │ login                     │                     │
   │                       ├──────────────────────────►│                     │
   │                       │                           │ Generate state,     │
   │                       │                           │ Redirect to OIDC    │
   │                       │                           ├────────────────────►│
   │  ◄──────── Browser redirected to OIDC Provider ──────────────────────►  │
   │  ◄──── Auth Dialog ──►│                           │                     │
   │                       │                           │◄────────────────────┤
   │                       │                           │  Auth Code          │
   │                       │                           │ Exchange for tokens │
   │                       │                           ├────────────────────►│
   │                       │                           │◄────────────────────┤
   │                       │                           │  ID Token, Access   │
   │                       │                           │  Token              │
   │                       │                           │ Create session      │
   │                       │                           ├─────────┐           │
   │                       │                           │◄────────┘           │
   │                       │◄──────────────────────────┤                     │
   │                       │  Redirect to app          │                     │
   │                       │  (with session cookie)    │                     │
   │                       │  API Call with header:    │                     │
   │                       │  Authorization: Bearer    │                     │
   │                       │  <jwt>                    │                     │
   │                       ├──────────────────────────►│                     │
   │                       │◄──────────────────────────┤                     │
   │                       │   Protected Resource      │                     │
```

Key points:
- Backend handles OIDC protocol completely.
- Frontend only does redirect (no OAuth library needed).
- Standard header: `Authorization: Bearer <jwt>`.
- Session cookie maintained by backend.

## Provider Detection Pattern

```text
┌────────────────────────────────────────────────────────────┐
│                    Application Startup                     │
└────────────────┬───────────────────────────────────────────┘
                 │
                 ▼
    ┌────────────────────────┐
    │ GET /api/v2/web/config │
    └────────────┬─────────────┘
                 │
                 ▼
    ┌──────────────────────────┐
    │ {                        │
    │   auth_provider: "dex",  │   ◄── Backend determines provider
    │   api_version: "v2",     │
    │   feature_flags: {...}   │
    │ }                        │
    └────────────┬─────────────┘
                 │
                 ▼
    ┌────────────────────────────────────────┐
    │  ConfigService.config.auth_provider    │
    └──────────┬─────────────────────────────┘
               │ Used by:
               ├──► SessionService (auth flow selection)
               ├──► FetchService (header selection)
               ├──► SearchService (header selection)
               └──► AuthenticateController (UI/button selection)
```

## Header Selection Logic

```typescript
function getAuthHeaders(authProvider: string, token: string) {
  if (authProvider === "google") {
    return { "Hermes-Google-Access-Token": token };
  } else if (authProvider === "dex" || authProvider === "okta") {
    return { "Authorization": `Bearer ${token}` };
  }
  return {};
}
```

## Component Architecture

```text
┌─────────────────────────────────────────────────────────────┐
│                     Authenticate Route                      │
│                                                              │
│  ┌────────────────────────────────────────────────────┐    │
│  │           AuthenticateController                   │    │
│  │  authProvider = config.auth_provider               │    │
│  │                                                     │    │
│  │  ┌───────────────────┐   ┌──────────────────────┐ │    │
│  │  │ authenticate()    │   │ authenticateOIDC()   │ │    │
│  │  │ (Google OAuth)    │   │ (Okta/Dex OIDC)      │ │    │
│  │  │ Uses:             │   │ Uses:                │ │    │
│  │  │ ember-simple-auth │   │ window.location.href │ │    │
│  │  └───────────────────┘   └──────────────────────┘ │    │
│  └────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────┐
│                      SessionService                         │
│  isUsingOIDC = (provider === "okta" || provider === "dex")  │
│  pollForExpiredAuth() — checks session validity, shows      │
│  re-auth prompts; OIDC vs Google have different logic.      │
└─────────────────────────────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────┐
│                       FetchService                          │
│  Adds Hermes-Google-Access-Token or Authorization: Bearer   │
│  header based on configured auth_provider.                  │
└─────────────────────────────────────────────────────────────┘
```

## Why No Torii Needed

Torii was designed for popup-based OAuth flows with multiple OAuth providers and client-side token management.

- **Google OAuth** in Hermes uses popup flow but works with `ember-simple-auth` directly; Torii adds no value.
- **OIDC (Okta/Dex)** is server-redirect based; Torii doesn't apply at all.

| Criteria | With Torii | Without Torii (Current) |
|----------|-----------|------------------------|
| Dependencies | +1 (Torii) | 0 |
| Complexity | High | Low |
| OIDC Support | Not designed for it | Native pattern |
| Google OAuth | Works | Works (ember-simple-auth) |
| Maintenance | More code | Less code |
| Ember 6.x Compat | Broken | Working |
| Testing | Mock Torii + providers | Mock auth provider config |
| Provider Addition | Add Torii provider | Add if/else case |

Recommendation: keep runtime provider detection with direct authentication flows. Cleanup tasks: remove Torii initializer/provider stubs; rename authenticator (torii → oauth or custom-auth); update docs; add per-provider tests.