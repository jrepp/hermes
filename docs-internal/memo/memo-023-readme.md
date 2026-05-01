---
id: memo-023
title: "Memo Index"
status: Reference
created: 2026-04-24
author: Hermes Team
project_id: hermes
doc_uuid: 1f80098b-75c3-4ef1-b0be-8c4f459d40eb
type: Memo
subtype: Reference
tags: [memo, index, hub]
---

# Memo Index

> Investigations, post-mortems, milestone reports, demoted decisions, and other dated narrative notes from Hermes development. **Evergreen step-by-step content lives in [`docs-internal/guides/`](../guides/readme.md), not here.**

> **Authoring a new memo?** Start from [`docs-internal/templates/readme.md`](../templates/readme.md) and use [`memo-template.md`](../templates/memo-template.md). For evergreen setup/reference content, use [`guide-template.md`](../templates/guide-template.md) instead.

## Index by category

### Demoted ADRs (now memos)

These memos used to be ADRs; the binding rules they once carried have been retired or never existed.

- **memo-060** — Fix Ember `locationType` configuration. (formerly ADR-036)
- **memo-061** — `ember-animated` stub components. (formerly ADR-006)
- **memo-062** — Multi-provider auth diagrams. (formerly ADR-084 — diagrams aren't a decision)
- **memo-063** — SQLite driver conflict investigation. (split from ADR-083)
- **memo-064** — `ember-concurrency` / `ember-power-select` compatibility. (formerly ADR-029 — pinning belongs in `package.json`)

### Investigations & debug logs

- **memo-040** — GTS template compilation debug log.
- **memo-041** — People API architecture clarification.
- **memo-052** — Indexer architecture refactoring notes.
- **memo-057** — Query optimization analysis.
- **memo-058** — Event-driven indexer testing status.

### Implementation summaries & milestones

- **memo-001** — RFC-014 milestone log (week 1-2).
- **memo-002** — RFC-014 milestone log (week 2-3).
- **memo-003** — *Retired.* Promoted out of the memo namespace to [`docs-internal/plans/roadmap-tracker.md`](../plans/roadmap-tracker.md) on 2026-04-27 to reflect its role as the living index of v1.0 trajectories. The MEMO-003 number is not reused.
- **memo-038** — UUID integration summary.
- **memo-039** — UUID migration summary.
- **memo-053** — Event-driven indexer implementation summary.
- **memo-054** — Event-driven indexer production deployment.
- **memo-055** — Semantic search release notes.
- **memo-056** — Semantic search performance benchmarks.
- **memo-059** — S3 storage implementation summary.

### Analysis & metrics

- **memo-004** — Agent usage analysis.
- **memo-005** — AI agent capabilities and limitations.
- **memo-006** — Human enablement patterns.
- **memo-008** — AI agent session playbook.
- **memo-016** — Development velocity analysis.
- **memo-065** — Search outbox NFR trial.

### Feature reference (RFC supplements)

#### Simplified Mode (RFC-009)
- **memo-026** — Simplified local mode demo.
- **memo-042** — Architecture diagram.
- **memo-043** — Implementation checklist.
- **memo-044** — Summary.

#### API Provider (RFC-011)
- **memo-045** — Permissions appendix.

#### Notification System (RFC-013)
- **memo-046** — Backend addendum.
- **memo-047** — Backends implementation.
- **memo-048** — Docker Compose setup.
- **memo-049** — Message schema.
- **memo-050** — Template scheme.
- **memo-051** — Implementation status.

### E2E testing

- **memo-032** — E2E testing summary.

### Project config (legacy)

- **memo-035** — Project config API usage notes.
- **memo-036** — Project config package implementation summary.

### Hubs

- **memo-023** — This file (memo index).
- **memo-031** — Docs-internal hub (cross-cutting navigation).

## See also

- [`../guides/readme.md`](../guides/readme.md) — evergreen setup/reference guides (formerly memos 007, 009, 010–015, 017–022, 024–025, 027–030, 033–034, 037).
- [`../adr/adr-002-readme.md`](../adr/adr-002-readme.md) — ADR index (binding architectural decisions).
- [`../rfc/rfc-002-readme.md`](../rfc/rfc-002-readme.md) — RFC index (proposals).
- [`../templates/readme.md`](../templates/readme.md) — authoring guide and templates.
