---
id: adr-009
title: Provider Abstraction Architecture
type: ADR
subtype: System Architecture
decision_type: Architectural Pattern
status: Accepted
tags: [architecture, authentication, providers, search, workspace]
related: [ADR-012, ADR-013, ADR-017]
created: 2026-04-24
deciders: Hermes Team
project_id: hermes
doc_uuid: 7840bd07-7728-4eac-97e9-d7b4302c79b4
---

# Provider Abstraction Architecture

> Every external integration in Hermes is reached through a **provider interface**: `auth.Provider`, `workspace.Provider`, `search.Provider`. Concrete adapters (Google, Okta, Dex; Google, Local; Algolia, Meilisearch) implement those interfaces; handlers depend only on the interface and receive the provider via dependency injection. Selection is configuration-driven via the HCL `providers { auth, workspace, search }` block.

## Context

Hermes originally embedded Google Drive/Docs and Algolia client calls directly in HTTP handlers and middleware. That coupling made every handler hard to test, blocked offline development, and meant adding a second auth provider or a local search engine touched dozens of files. Configuration was scattered, mocks were brittle, and there was no single place to enforce error or context conventions.

## Decision

Introduce three provider interfaces and require all backend code to depend on them — never on a concrete vendor SDK.

**Interfaces (one per integration domain):**

- `pkg/auth/provider.go` → `auth.Provider` (Google, Okta, Dex)
- `pkg/workspace/provider.go` → `workspace.Provider` (Google, Local)
- `pkg/search/provider.go` → `search.Provider` (Algolia, Meilisearch)

**Patterns:** Strategy (per-domain interface) + Adapter (wrap vendor SDKs) + Factory (`New*Provider(cfg)` switches on config) + Dependency Injection (handlers receive providers via constructor / `server.Server`, see ADR-017).

**Conventions every provider follows:**
- First parameter is `context.Context`.
- Errors wrap typed sentinels (e.g. `workspace.ErrNotFound`, `search.ErrNotFound`) and are matched with `errors.Is`.
- Methods are idempotent where the underlying operation allows.
- Interfaces stay minimal — add a method only when a real handler needs it.

**Selection** is HCL-driven (ADR-021):

```hcl
providers {
  auth      = "dex"           # google | okta | dex
  workspace = "local"         # google | local
  search    = "meilisearch"   # algolia | meilisearch
}
```

CLI flags and environment variables (e.g. `-auth-provider`, `HERMES_AUTH_PROVIDER`) override the config (ADR-013).

## Consequences

### Positive
- Handlers are testable with trivial mocks; no vendor SDK in the unit-test path.
- New environments (local dev, CI, hybrid staging) are configuration changes, not code changes.
- Adding a provider (e.g. a new auth IdP) is an isolated package; no handler edits.
- Typed sentinel errors give callers a uniform contract across vendors.

### Negative
- One extra layer of indirection between handlers and vendor calls.
- Interfaces are a lowest-common-denominator: vendor-specific features must be either generalized, exposed via capability checks, or deliberately excluded.
- More surface area to document; agents and contributors must learn the provider model before touching handlers.

## Alternatives Considered

- **Go plugins for runtime-loaded providers:** Fragile build/version story; not worth it for a known set of providers.
- **Microservices per provider:** Network overhead and operational cost with no payoff at current scale.
- **Single concrete implementation:** Already tried — produced the testability and lock-in problems this ADR fixes.
- **Code generation from OpenAPI/gRPC:** Heavy build pipeline for interfaces small enough to write by hand.

## References

- `pkg/auth/`, `pkg/workspace/`, `pkg/search/`
- ADR-012 (multi-provider auth), ADR-013 (auth provider selection), ADR-017 (DI via `server.Server`), ADR-021 (HCL config)