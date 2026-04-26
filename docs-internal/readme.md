# Hermes Internal Documentation

**Purpose**: Internal reference for architecture, implementation notes, and planning artifacts.

## Directory Map

```
docs-internal/
├── adr/                      # Architectural Decision Records (finalized decisions)
├── rfc/                      # RFCs (design proposals and architecture plans)
├── memo/                     # Memos (implementation notes, guides, summaries)
├── plans/                    # Plans (non-durable work items and checklists)
├── archive/                  # Archived (completed/superseded documents)
├── ember-development-guide/  # Ember + frontend developer workflow guide
├── templates/                # ADR / RFC / Memo templates and authoring guide
└── readme.md                 # This file
```

## Quick Start

- **New contributors** → `memo/memo-035-env-setup.md`
- **Development quick reference** → `memo/memo-017-dev-quickref.md`
- **RFC index** → `rfc/rfc-002-readme.md`
- **Current planning status** → `memo/memo-003-2025-11-15-rfc-implementation-tracker.md`

## Core Indexes

- [ADR index](adr/adr-003-readme.md) — 24 finalized architectural decisions
- [RFC index](rfc/rfc-002-readme.md) — 18 active design proposals
- [Memo index](memo/memo-026-readme.md) — Implementation notes and guides
- [Plans index](plans/readme.md) — Non-durable work items
- [Templates & authoring guide](templates/readme.md) — Start here when writing a new ADR, RFC, or memo
- [Archive index](archive/readme.md) — Completed and superseded documents
- [Documentation hub](memo/memo-073-docs-internal-hub.md)

## Document Categories

| Category | Purpose | Durability |
|----------|---------|------------|
| **ADR** | Finalized decisions with context, alternatives, consequences | Durable |
| **RFC** | Design proposals and architecture plans (pre-coding) | Durable |
| **Memo** | Implementation notes, guides, session summaries, reference material | Durable |
| **Plan** | Work items, checklists, progress tracking | Non-durable |
| **Archive** | Completed/superseded items kept for historical reference | Read-only |

## Source-of-Truth Paths

- **RFCs** → `docs-internal/rfc/`
- **Decision records** → `docs-internal/adr/`
- **Implementation notes / guides** → `docs-internal/memo/`
- **Work items / plans** → `docs-internal/plans/`

## Maintenance Tips

- Prefer keeping topic-specific indexes up-to-date in each folder's readme.
- Use **RFC** for design proposals before coding. Promote to **ADR** when finalized.
- Use **ADR** when a decision is finalized with long-term relevance.
- Use **Memo** for implementation notes, guides, and reference material.
- Use **Plan** for non-durable work items. When completed, fold knowledge into durable docs and archive.
- **Archive** completed plans and superseded documents — don't delete them.
