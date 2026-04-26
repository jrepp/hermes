---
id: adr-029
title: "Ember Concurrency Version Pinning Policy"
status: Accepted
decision_type: Configuration Choice
created: 2025-10-08
deciders: Hermes Team
author: Hermes Team
project_id: hermes
doc_uuid: daffd0f1-bd26-4fea-b25f-766b4ebd95e2
date: 2025-10-08
type: ADR
subtype: Dependency Decision
tags: [ember, dependencies, ember-concurrency, ember-power-select]
related:
  - ADR-001
  - MEMO-125
---

# ADR-029: Ember Concurrency Version Pinning Policy

> Use `ember-power-select` 8.x with `ember-concurrency` 2.x. This is an intentional version mismatch — do not "fix" it by upgrading `ember-concurrency` to 3.x.

## Context

`ember-power-select` 8.x declares a peer dependency on `ember-concurrency` 3.x and its precompiled dist files import from a private path:

```javascript
import { buildTask } from 'ember-concurrency/async-arrow-runtime';
```

`ember-concurrency` 3.x does not export `async-arrow-runtime` at the root — it lives at `addon/-private/async-arrow-runtime.js`. Ember's module resolution does not honor webpack aliases, manual `node_modules` shims, or `ember-cli-build.js` aliases for addon resolution. Downgrading `ember-power-select` to 7.x produces SASS import errors with Ember 6.x.

Hermes uses only basic dropdown features; the missing `async-arrow-runtime` import is required only for advanced features Hermes does not use.

## Decision

Pin `ember-power-select` to `^8.11.0` and `ember-concurrency` to `^2.3.7`:

```bash
yarn up ember-power-select@^8.11.0 ember-concurrency@^2.3.7
```

This produces a peer-dependency warning in the console; it is expected and intentional. Do not "fix" the warning by upgrading `ember-concurrency` to 3.x — it will break dropdown rendering at runtime.

## Consequences

### Positive
- Dropdowns render and function correctly under Ember 6.x.
- No build errors; document creation, project selection, and product/area selection all work.

### Negative
- A peer-dependency mismatch warning appears in the console.
- Advanced `ember-power-select` features that depend on `async-arrow-runtime` are unavailable (Hermes does not use them).
- Future `ember-power-select` releases may break the workaround.

## Alternatives Considered

- **Upgrade `ember-concurrency` to 3.x** — fails at runtime: `async-arrow-runtime` is not exported.
- **Manual `node_modules` shim** — Ember addon resolution ignores it.
- **Webpack alias in `ember-cli-build.js`** — Ember addon resolution ignores it.
- **Patch `ember-power-select` dist file in place** — lost on `yarn install`; would need `patch-package` automation.
- **Downgrade to `ember-power-select` 7.x** — SASS import errors with Ember 6.x.
- **Switch to a different dropdown library** — large UI rewrite for marginal benefit.

## Long-Term Path

Re-evaluate when `ember-power-select` upstream fixes the export path, or when a Hermes feature requires `ember-concurrency` 3.x (in which case `patch-package` or a fork becomes the path forward).
