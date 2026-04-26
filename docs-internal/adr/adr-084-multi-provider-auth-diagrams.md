---
id: adr-084
deciders: Hermes Team
created: 2026-04-24
author: Hermes Team
project_id: hermes
doc_uuid: 45308585-fccc-494b-95c7-8901f13e942b
status: Accepted
title: "Multi-Provider Auth Architecture (Current State)"
---
# Multi-Provider Auth Architecture (Current State)

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

## Authentication Flows (Detailed)

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
   │                         │         │               │                    │
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
   │                         │         │               │                    │
   │                         │◄────────┘               │                    │
   │                         │                         │                    │
   │                         │ API Call with header:   │                    │
   │                         │ Hermes-Google-Access-   │                    │
   │                         │ Token: <token>          │                    │
   │                         ├────────────────────────►│                    │
   │                         │                         │                    │
   │                         │◄────────────────────────┤                    │
   │                         │    Protected Resource   │                    │
   │                         │                         │                    │
```

**Key Points**:
- ✅ Frontend manages OAuth flow directly
- ✅ Uses ember-simple-auth (NOT Torii library)
- ✅ Custom header: `Hermes-Google-Access-Token`
- ✅ Token stored in browser session

### OIDC Flow (Okta/Dex) - Backend-Centric

```text
┌──────┐              ┌──────────┐                ┌─────────┐           ┌──────────┐
│ User │              │ Frontend │                │ Backend │           │ OIDC     │
│      │              │          │                │         │           │ Provider │
└──┬───┘              └────┬─────┘                └────┬────┘           └────┬─────┘
   │                       │                           │                     │
   │  Click "Sign in"      │                           │                     │
   ├──────────────────────►│                           │                     │
   │                       │                           │                     │
   │                       │ Redirect to:              │                     │
   │                       │ /api/v2/auth/{provider}/  │                     │
   │                       │ login                     │                     │
   │                       ├──────────────────────────►│                     │
   │                       │                           │                     │
   │                       │                           │ Generate state,     │
   │                       │                           │ Redirect to OIDC    │
   │                       │                           ├────────────────────►│
   │                       │                           │                     │
   │  ◄──────── Browser redirected to OIDC Provider ──────────────────────►  │
   │                       │                           │                     │
   │  ◄──── Auth Dialog ──►│                           │                     │
   │                       │                           │                     │
   │                       │                           │◄────────────────────┤
   │                       │                           │  Auth Code          │
   │                       │                           │                     │
   │                       │                           │ Exchange for tokens │
   │                       │                           ├────────────────────►│
   │                       │                           │                     │
   │                       │                           │◄────────────────────┤
   │                       │                           │  ID Token, Access   │
   │                       │                           │  Token              │
   │                       │                           │                     │
   │                       │                           │ Create session      │
   │                       │                           ├─────────┐           │
   │                       │                           │         │           │
   │                       │                           │◄────────┘           │
   │                       │                           │                     │
   │                       │◄──────────────────────────┤                     │
   │                       │  Redirect to app          │                     │
   │                       │  (with session cookie)    │                     │
   │                       │                           │                     │
   │                       │  API Call with header:    │                     │
   │                       │  Authorization: Bearer    │                     │
   │                       │  <jwt>                    │                     │
   │                       ├──────────────────────────►│                     │
   │                       │                           │                     │
   │                       │◄──────────────────────────┤                     │
   │                       │   Protected Resource      │                     │
   │                       │                           │                     │

```

**Key Points**:
- ✅ Backend handles OIDC protocol completely
- ✅ Frontend only does redirect (no OAuth library needed)
- ✅ Standard header: `Authorization: Bearer <jwt>`
- ✅ Session cookie maintained by backend

## Provider Detection Pattern

```typescript
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
               │
               │ Used by:
               ├──► SessionService (auth flow selection)
               ├──► FetchService (header selection)
               ├──► SearchService (header selection)
               └──► AuthenticateController (UI/button selection)
```

## Header Selection Logic

```typescript
// Pattern used across services (fetch, search, etc.)

function getAuthHeaders(authProvider: string, token: string) {
  if (authProvider === "google") {
    return {
      "Hermes-Google-Access-Token": token
    };
  } else if (authProvider === "dex" || authProvider === "okta") {
    return {
      "Authorization": `Bearer ${token}`
    };
  }
  return {};
}

// Applied automatically to all backend API calls

```

## Component Architecture

```text
┌─────────────────────────────────────────────────────────────┐
│                     Authenticate Route                      │
│                                                              │
│  ┌────────────────────────────────────────────────────┐    │
│  │           AuthenticateController                   │    │
│  │                                                     │    │
│  │  authProvider = config.auth_provider               │    │
│  │                                                     │    │
│  │  ┌───────────────────┐   ┌──────────────────────┐ │    │
│  │  │ authenticate()    │   │ authenticateOIDC()   │ │    │
│  │  │ (Google OAuth)    │   │ (Okta/Dex OIDC)      │ │    │
│  │  │                   │   │                      │ │    │
│  │  │ Uses:             │   │ Uses:                │ │    │
│  │  │ ember-simple-auth │   │ window.location.href │ │    │
│  │  └───────────────────┘   └──────────────────────┘ │    │
│  └────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────┐
│                      SessionService                         │
│                                                              │
│  isUsingOIDC = (provider === "okta" || provider === "dex")  │
│                                                              │
│  ┌────────────────────────────────────────────────────┐    │
│  │  pollForExpiredAuth()                              │    │
│  │  • Checks session validity                         │    │
│  │  • Shows re-auth prompts                           │    │
│  │  • Different logic for OIDC vs Google              │    │
│  └────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────┐
│                       FetchService                          │
│                                                              │
│  fetch(url, options) {                                      │
│    if (authProvider === "google") {                         │
│      headers["Hermes-Google-Access-Token"] = token;         │
│    } else if (authProvider === "dex" || authProvider ===    │
│                "okta") {                                     │
│      headers["Authorization"] = `Bearer ${token}`;          │
│    }                                                         │
│  }                                                           │
└─────────────────────────────────────────────────────────────┘
```

## Why No Torii Needed

```text
┌─────────────────────────────────────────────────────────┐
│              What Torii Was Designed For                │
├─────────────────────────────────────────────────────────┤
│                                                          │
│  • Popup-based OAuth flows (Google, Facebook, GitHub)  │
│  • Multiple OAuth providers with similar flows          │
│  • Client-side token management                         │
│  • Provider abstraction layer                           │
│                                                          │
└─────────────────────────────────────────────────────────┘
                           │
                           ▼
         ┌────────────────────────────────┐
         │   Does Hermes Need This?       │
         └────────┬───────────────────────┘
                  │
      ┌───────────┴───────────┐
      │                       │
      ▼                       ▼
┌──────────────┐      ┌──────────────────┐
│ Google OAuth │      │  OIDC (Okta/Dex) │
└──────┬───────┘      └────────┬─────────┘
       │                       │
       │ Popup flow            │ Server redirect
       │ ✅ Works with         │ ❌ Doesn't use
       │    ember-simple-auth  │    popups at all!
       │    directly           │
       │                       │ Backend handles
       │ ❌ Torii adds no      │ entire flow
       │    value              │
       │                       │ ❌ Torii NOT
       │                       │    applicable
       └───────────────────────┘

```

## Comparison: With vs Without Torii

### ❌ With Torii (Unnecessarily Complex)

```typescript
// Would need:
- Torii library (dependency)
- Torii initializer (configuration)
- Torii providers for each OAuth method
- Torii authenticator wrapper
- Maintenance overhead

// But provides:
- Abstraction we don't need (only 1 OAuth provider)
- Complexity for OIDC that doesn't use it
```

### ✅ Without Torii (Current - Simpler)

```typescript
// What we have:
- ember-simple-auth (already required)
- Simple if/else provider detection
- Backend handles OIDC
- Clear, maintainable code

// Benefits:
- One less dependency
- Simpler mental model
- Backend-centric OIDC (correct pattern)
- Easy to test

```

## Decision Matrix

| Criteria | With Torii | Without Torii (Current) |
|----------|-----------|------------------------|
| **Dependencies** | +1 (Torii) | 0 |
| **Complexity** | High | Low |
| **OIDC Support** | Not designed for it | ✅ Native pattern |
| **Google OAuth** | Works | ✅ Works (ember-simple-auth) |
| **Maintenance** | More code | Less code |
| **Ember 6.x Compat** | ❌ Broken | ✅ Working |
| **Testing** | Mock Torii + providers | Mock auth provider config |
| **Provider Addition** | Add Torii provider | Add if/else case |

## Recommendation Summary

### ✅ Current Pattern is Optimal

**Keep using**: Runtime provider detection with direct authentication flows

**Reasons**:
1. Simpler architecture
2. Fewer dependencies
3. Backend-centric OIDC (industry standard)
4. Already working and tested
5. Torii provides no benefit for current use case

### 🧹 Cleanup Tasks

1. Remove Torii initializer stub
2. Remove Torii provider stubs
3. Rename authenticator (torii → oauth or custom-auth)
4. Update documentation
5. Add tests for each provider flow

---

**Created**: October 6, 2025
**Related Docs**:
- `TORII_AUTH_ANALYSIS.md` - Full analysis
- `AUTH_PROVIDER_SELECTION.md` - Provider selection logic
- `DEX_AUTHENTICATION.md` - Dex OIDC setup

