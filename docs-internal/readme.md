# Hermes Internal Documentation

**Purpose**: Internal reference for architecture, implementation notes, and planning artifacts.

## Directory Map

```
docs-internal/
├── adr/                      # Architectural Decision Records (binding decisions)
├── rfc/                      # RFCs (design proposals and architecture plans)
├── memo/                     # Memos (investigations, post-mortems, milestones, demoted ADRs)
├── guides/                   # Evergreen step-by-step guides (setup, integration, dev workflow)
├── plans/                    # Plans (non-durable work items and checklists)
├── archive/                  # Archived (completed/superseded documents)
├── ember-development-guide/  # Ember + frontend developer workflow guide (legacy location)
├── templates/                # ADR / RFC / Memo / Guide templates and authoring guide
└── readme.md                 # This file
```

## Quick Start

- **New contributors** → [`guides/dev/env-setup.md`](guides/dev/env-setup.md)
- **Development quick reference** → [`guides/dev/quickref.md`](guides/dev/quickref.md)
- **All setup & how-to guides** → [`guides/readme.md`](guides/readme.md)
- **RFC index** → [`rfc/rfc-002-readme.md`](rfc/rfc-002-readme.md)
- **Current planning status** → [`plans/roadmap-tracker.md`](plans/roadmap-tracker.md)

## Core Indexes

- [ADR index](adr/adr-002-readme.md) — finalized architectural decisions (binding)
- [RFC index](rfc/rfc-002-readme.md) — active design proposals
- [Memo index](memo/memo-023-readme.md) — investigations, milestones, demoted ADRs
- [Guides index](guides/readme.md) — evergreen step-by-step references
- [Plans index](plans/readme.md) — non-durable work items
- [Templates & authoring guide](templates/readme.md) — start here when writing any new doc
- [Archive index](archive/readme.md) — completed and superseded documents
- [Docs-internal cross-cutting hub](memo/memo-031-docs-internal-hub.md)

## Document Categories

| Category | Purpose | Durability |
|----------|---------|------------|
| **ADR** | Binding architectural decisions with context, alternatives, consequences | Durable |
| **RFC** | Design proposals and architecture plans (pre-coding) | Durable |
| **Guide** | Evergreen step-by-step setup, integration, and workflow references | Durable |
| **Memo** | Dated narrative notes — investigations, post-mortems, milestones, demoted ADRs | Durable |
| **Plan** | Work items, checklists, progress tracking | Non-durable |
| **Archive** | Completed/superseded items kept for historical reference | Read-only |

## Source-of-Truth Paths

- **Decision records** → `docs-internal/adr/`
- **RFCs** → `docs-internal/rfc/`
- **Setup & how-to guides** → `docs-internal/guides/`
- **Implementation notes & investigations** → `docs-internal/memo/`
- **Work items / plans** → `docs-internal/plans/`

## Maintenance Tips

- Prefer keeping topic-specific indexes up-to-date in each folder's readme.
- Use **RFC** for design proposals before coding; promote to **ADR** when finalized.
- Use **ADR** when a decision is binding and has long-term force.
- Use **Guide** for evergreen step-by-step content (broad audience, no dates).
- Use **Memo** for dated narrative notes (incidents, milestones, post-mortems, demoted ADRs).
- Use **Plan** for non-durable work items. When completed, fold knowledge into durable docs and archive.
- **Archive** completed plans and superseded documents — don't delete them.
- See [`templates/readme.md`](templates/readme.md) for the full doc-type matrix and authoring rules.
