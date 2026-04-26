---
id: adr-021
title: HCL for Per-Project Configuration
type: ADR
subtype: Configuration
decision_type: Configuration
status: Accepted
tags: [configuration, hcl, projects]
related: [ADR-009, ADR-013]
created: 2026-04-24
deciders: Hermes Team
project_id: hermes
doc_uuid: c0755685-ba8d-446f-b087-08492ce933bb
---

# HCL for Per-Project Configuration

> Per-project configuration uses **HCL**, not JSON. Each project lives in its own file under `testing/projects/*.hcl` and is composed via `import` from `testing/projects.hcl`. Files prefixed with **`_template-`** are templates and **are not loaded**. Each project block declares a **short name** (e.g. `RFC`) used for document IDs (`RFC-001`, `RFC 042`). The same HCL parser (`github.com/hashicorp/hcl/v2`) is used for `config.hcl`, so projects and server config share syntax, types, and the `env()` function.

## Context

Hermes' main `config.hcl` is HCL. Adding a parallel JSON file for projects would mean two parsers, two type systems, two comment conventions, and two ways to reference environment variables. Project configs also need to be edited by separate teams without merge conflicts on a single monolithic file, and need a clean way to ship templates that are version-controlled but not loaded as live projects.

## Decision

- **Format:** HCL for all project configuration.
- **Layout:** one file per project under `testing/projects/` (e.g. `engineering-rfcs.hcl`); `testing/projects.hcl` composes them with `import` statements.
- **Template convention:** any file whose basename starts with `_template-` is treated as a template and is **not** loaded by the projects loader.
- **Project identity:** each `project "<key>" { ... }` block declares a `short_name` used as the document ID prefix (`RFC-001`, `TEST-042`); this is stable even if the human-readable title changes.
- **Loader:** uses `github.com/hashicorp/hcl/v2` with strongly-typed Go structs; invalid configs fail at parse time with structured errors.
- **Env vars:** referenced via HCL's `env()` function; no custom string templating.

## Consequences

### Positive
- One configuration language across the codebase; same parser, same diagnostics, same ergonomics as `config.hcl`.
- One-file-per-project enables CODEOWNERS-style team ownership and parallel edits without merge-conflict churn.
- Templates can ship in the repo without polluting the live project list — naming alone controls loading.
- Short names give stable, terse document identifiers decoupled from project titles.

### Negative
- Loader must explicitly skip `_template-*` files and document the convention; a contributor could otherwise expect their template to load.
- HCL's tooling outside the HashiCorp ecosystem is thinner than JSON's; some external consumers may need a converter.

## Alternatives Considered

- **JSON:** No comments, no native env-var support, monolithic by convention; adds a second config language alongside `config.hcl`.
- **YAML:** Comment-friendly but indentation-sensitive and historically error-prone for Hermes-style nested blocks; no `env()` equivalent without templating.
- **Single big HCL file with all projects:** Loses team ownership and multiplies merge conflicts; offers no win over per-project files.

## References

- `testing/projects.hcl`, `testing/projects/*.hcl`, `testing/projects/readme.md`
- ADR-009 (provider abstraction — projects declare providers), ADR-013 (CLI/env override interacts with config selection)