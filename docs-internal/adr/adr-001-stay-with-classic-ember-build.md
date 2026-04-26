---
id: adr-001
title: Stay with Classic Ember CLI Build System
status: Accepted
decision_type: Build System
created: 2025-10-10
deciders: Hermes Team
author: Hermes Team
project_id: hermes
doc_uuid: ab8079d7-5eb8-4aab-8faf-4ca65a24ccb9
type: ADR
tags: [broccoli, build, ember, embroider]
related: [MEMO-064]
---

# ADR-001: Stay with Classic Ember CLI Build System

> The Hermes web app stays on classic Ember CLI / Broccoli. Do not migrate to Embroider (Webpack or Vite) until `@embroider/vite` ships v2.0+ stable, build time becomes a blocking development issue, or a major Ember version upgrade requires it.

## Context

Hermes uses classic Ember CLI with Broccoli. Embroider promises 30–70% faster builds and modern tooling (Vite/Webpack 5, code splitting, tree shaking, ES modules). An aggressive migration attempt to Embroider + Vite was made and reverted.

What blocked the migration:

- `@embroider/vite@1.3.2` is marked experimental; build fails with `Cannot read properties of undefined (reading 'code')` and produces unclear errors.
- Migration requires async `module.exports`, new dependency patterns, and a separate `vite.config.js`; addon compatibility is uncertain.
- The classic build is stable, integrated with all tooling (tests, coverage, linting), and has no production-blocking performance issues.

Build times are slow but acceptable (~45–60 s full build). Build performance does not affect runtime users.

## Decision

Stay on classic Ember CLI / Broccoli.

Re-evaluate when **any** of the following holds:

1. `@embroider/vite` reaches v2.0 or is officially marked stable.
2. Build times become a blocking development issue (measure first).
3. A major Ember version upgrade requires Embroider.
4. Three or more comparable production Ember apps publish successful migration case studies.

## Consequences

### Positive
- Zero disruption; no risk to existing workflows or addons.
- Team focuses on features, not build-system migration.
- Known performance characteristics; existing knowledge stays valid.

### Negative
- Continued ~45–60 s full builds and ~5–10 s incremental rebuilds.
- Build system ages relative to the Ember ecosystem; future migration gap grows.
- No Embroider improvements (code splitting, tree shaking, instant HMR).

## Alternatives Considered

- **Embroider + Webpack** — more stable than Vite, 30–50% faster, easier rollback. Rejected: still ~1–2 weeks effort with addon-compatibility risk; current build times are tolerable.
- **Incremental Embroider adoption** (staticHelpers, staticModifiers, …) — rejected: ongoing maintenance burden of a hybrid state for partial benefit.
- **Embroider + Vite** — rejected (the attempted path): tooling not production-ready.

## References

- Embroider: https://github.com/embroider-build/embroider
- Ember CLI build pipeline: https://cli.emberjs.com/release/advanced-use/build-pipeline/