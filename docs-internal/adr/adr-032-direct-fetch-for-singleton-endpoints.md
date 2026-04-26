---
id: adr-032
title: "Direct fetch() for Singleton API Endpoints"
status: Accepted
decision_type: Architectural Pattern
created: 2025-10-08
deciders: Hermes Team
author: Hermes Team
project_id: hermes
doc_uuid: a2cf1e6f-ea7a-4f13-8ee6-00c5a130e5fc
date: 2025-10-08
type: ADR
subtype: Frontend Decision
tags: [ember, frontend, ember-data, fetch, authentication]
related:
  - ADR-076
  - ADR-065
---

# ADR-032: Direct fetch() for Singleton API Endpoints

> Singleton API endpoints (`/me`, `/config`, …) are fetched with `fetch(url, { credentials: "include" })` and manually inserted into the Ember Data store via `peekRecord` + `createRecord` / `setProperties`. Do not use `store.findAll()` or `store.queryRecord()` for endpoints that return a single object rather than an array.

## Context

`store.findAll("me")` expects an array response. `/api/v2/me` returns a single object, which crashes the Ember Data adapter:

```text
TypeError: Cannot read properties of undefined (reading 'request')
    at StoreService.request
    at StoreService.findAll
    at AuthenticatedUserService.loadInfo
```

A second issue surfaced at the same time: `ApplicationAdapter.headers` unconditionally accessed `session.data.authenticated.access_token`, which is `undefined` for cookie-based auth (Dex), causing a crash before any request was made.

Both problems trace to the same root: trying to push singleton endpoints through Ember Data's collection-shaped pipeline.

## Decision

1. **Singleton endpoints use `fetch()` directly** with `credentials: "include"` so session cookies (Dex) and bearer tokens (Google) both work:

   ```typescript
   const response = await fetch(`/api/${this.configSvc.config.api_version}/me`, {
     method: "GET",
     credentials: "include",
   });
   const data = await response.json();

   let person = this.store.peekRecord("person", data.email);
   if (!person) {
     person = this.store.createRecord("person", { /* data */ });
   } else {
     person.setProperties({ /* updated data */ });
   }
   ```

2. **`ApplicationAdapter.headers` uses optional chaining** and returns an empty object when no token is present:

   ```typescript
   get headers() {
     const accessToken = this.session.data?.authenticated?.access_token;
     if (!accessToken) return {};
     return { "Hermes-Google-Access-Token": accessToken };
   }
   ```

## Consequences

### Positive
- Works for all auth providers (Google bearer token, Okta/Dex session cookie).
- No crashes when session data is undefined.
- Explicit, debuggable record lifecycle.
- No dependency on Ember Data's RequestManager configuration for single-object responses.

### Negative
- Bypasses Ember Data conventions for these endpoints.
- Manual store-record management duplicates a small amount of Ember Data logic.
- Less declarative than `store.findAll()`.

## Alternatives Considered

- **Change backend to return an array** — wrong semantics; `/me` is a singular resource.
- **`store.findRecord("me", id)`** — requires a backend `/me/:id` endpoint that doesn't exist semantically.
- **Configure RequestManager for single-object responses** — overkill for one endpoint.
- **`store.queryRecord()`** — still expects Ember Data conventions and a `query` shape.
