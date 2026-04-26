---
id: adr-013
title: Auth Provider CLI and Environment Override
type: ADR
subtype: Configuration
decision_type: Configuration
status: Accepted
tags: [authentication, cli, configuration, dex, google, okta]
related: [ADR-009, ADR-012, ADR-021]
created: 2025-10-06
deciders: Hermes Team
project_id: hermes
doc_uuid: 4dddc960-f32b-436f-aaa7-6a9b9e7dcdb4
---

# Auth Provider CLI and Environment Override

> The auth provider can be selected explicitly via the **`-auth-provider`** CLI flag or **`HERMES_AUTH_PROVIDER`** environment variable, taking precedence over the HCL `providers.auth` setting. Valid values: `google`, `okta`, `dex`. Precedence is **flag > env > config**.

## Context

By default Hermes auto-selects an auth provider from HCL config (ADR-021) using the priority Dex → Okta → Google. That works for static deployments but is awkward for testing matrices, CI pipelines that switch providers per stage, and local debugging where you want to force a specific provider without editing config files.

## Decision

Add a single explicit override that short-circuits the config-driven selection and takes effect before provider initialization.

- **CLI flag:** `hermes server -auth-provider=dex`
- **Env var:** `HERMES_AUTH_PROVIDER=dex` (read when the flag is empty)
- **Precedence:** CLI flag > environment variable > HCL config auto-selection.
- **Behavior:** Selecting a provider sets `disabled = false` for it and `disabled = true` for the others; an invalid value is a startup error, and the selection source (flag vs env) is logged.

## Consequences

### Positive
- CI stages and Docker Compose files can pin a provider with one variable, no config rewrite.
- Local debugging can switch providers per invocation without touching version-controlled config.
- Explicit and observable — the chosen provider and its source are logged at startup.

### Negative
- A third precedence layer to remember (flag, env, config); contributors must know the order.
- Easy to leave `HERMES_AUTH_PROVIDER` set in a shell and be confused later; surfaced only via startup logs.

## Alternatives Considered

- **Config edits only:** Honest but painful for short-lived overrides and CI matrices.
- **Per-provider toggle env vars (`HERMES_DEX_DISABLED=false`, etc.):** Combinatorially messy and easy to misconfigure (two providers enabled at once).

## References

- `internal/cmd/commands/server/server.go` — `flagAuthProvider`, `applyAuthProviderSelection()`
- ADR-009 (provider abstraction), ADR-012 (multi-provider auth), ADR-021 (HCL config)