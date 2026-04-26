---
id: memo-128
title: ember-concurrency / ember-power-select Compatibility Notes
status: Final
created: 2026-04-26
date: 2026-04-26
deciders: Hermes Team
author: Hermes Team
project_id: hermes
doc_uuid: daffd0f1-bd26-4fea-b25f-766b4ebd95e2
type: Memo
subtype: Frontend Compatibility Note
tags: [ember, dependencies, ember-concurrency, ember-power-select]
related:
  - ADR-001
  - MEMO-125
supersedes: ADR-029
---
# ember-concurrency / ember-power-select Compatibility Notes

> Demoted from ADR-029. The earlier ADR mandated specific package versions (`ember-power-select ^8.11.0` + `ember-concurrency ^2.3.7`); pinning specific dependency versions is `package.json`'s job, not an architectural decision. The current versions in `web/package.json` may differ from what this memo describes — treat the live `web/package.json` as ground truth.

## Background

`ember-power-select` 8.x ships precompiled `dist/` files that import a private path from `ember-concurrency` 3.x:

```javascript
import { buildTask } from 'ember-concurrency/async-arrow-runtime';
```

`ember-concurrency` 3.x does not export `async-arrow-runtime` at the root — it lives at `addon/-private/async-arrow-runtime.js`. Ember's module resolution does **not** honor webpack aliases, manual `node_modules` shims, or `ember-cli-build.js` aliases for addon resolution. So the import fails at runtime with `Could not find module 'ember-concurrency/async-arrow-runtime'`, and dropdowns built on `ember-power-select` stop rendering.

The features Hermes actually uses do not require `async-arrow-runtime`; only advanced `ember-power-select` features do.

## Workarounds That Were Tried

When the project was on `ember-power-select` 8.x + `ember-concurrency` 3.x:

- **Manual `node_modules` shim** — ignored by Ember addon resolution.
- **Webpack alias in `ember-cli-build.js`** — ignored by Ember addon resolution.
- **In-place edit of the `ember-power-select` dist file** — lost on `yarn install`; only viable under `patch-package`.
- **Downgrade `ember-power-select` to 7.x** — produced SASS import errors with Ember 6.x at the time.
- **Pin `ember-concurrency` to 2.x and accept a peer-dependency warning** — worked; this is what ADR-029 mandated.

## Current State

`web/package.json` is the source of truth for which versions are installed. If the team is running a combination that does not exhibit the bug above (for example, `ember-power-select` 7.x with `ember-concurrency` 4.x), no workaround is needed.

If a future upgrade re-introduces the `ember-power-select` 8.x + `ember-concurrency` 3.x pairing, this memo records what worked and what did not.

## Related

- ADR-001 — staying with the classic Ember build (Embroider/Vite migration deferred).
- MEMO-125 — `ember-animated` passthrough stub components (sibling Ember-ecosystem migration note).

## History

- 2025-10-08: Originally written as ADR-029 mandating `ember-power-select ^8.11.0` + `ember-concurrency ^2.3.7`.
- 2026-04-26: Demoted to MEMO-128. ADRs should not pin specific package versions; that contract belongs in `package.json`. The dependency situation in `web/package.json` may have moved on; treat the live manifest as ground truth.
