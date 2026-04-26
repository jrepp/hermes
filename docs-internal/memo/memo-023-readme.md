---
id: memo-023
created: 2026-04-24
author: Hermes Team
project_id: hermes
doc_uuid: 1f80098b-75c3-4ef1-b0be-8c4f459d40eb
status: Draft
title: Memo Documents
tags: []
---

# Memo Documents

Implementation notes, quick reference guides, session summaries, and completion reports from Hermes development.

> **Authoring a new memo?** Start from [`docs-internal/templates/readme.md`](../templates/readme.md) and use [`memo-template.md`](../templates/memo-template.md). The guide pins memo `status` and `type` conventions and the demotion checklist for converting an ADR into a memo.

## Index

### Quick Reference Guides

- **memo-007** - Auth provider quick reference
- **memo-009** - Ember dev server, proxy config, upgrade strategy
- **memo-014** - Development workflow quick reference (build, test, deploy)
- **memo-020** - Dex OIDC quick start guide
- **memo-027** - Complete environment setup (10-minute quick start)
- **memo-028** - Outbox pattern quick reference with SQL queries
- **memo-029** - Playwright agent E2E testing guide
- **memo-030** - Authentication providers guide (Google, Okta, Dex)
- **memo-031** - Documentation hub and navigation guide
- **memo-033** - Environment variables setup guide
- **memo-034** - Makefile quick start targets
- **memo-035** - Project config API usage guide
- **memo-036** - Project config package implementation summary

### Component & Setup Guides

- **memo-010** - Algolia search provider setup
- **memo-011** - Auth provider testing guide
- **memo-012** - Dex OIDC local auth guide
- **memo-013** - Google Workspace integration guide
- **memo-015** - Hermes indexer readme
- **memo-017** - Jira integration guide
- **memo-018** - Local workspace provider setup
- **memo-019** - Meilisearch setup guide
- **memo-021** - Ollama AI provider setup
- **memo-022** - PostgreSQL database setup
- **memo-024** - Setup wizard guide
- **memo-025** - Setup wizard Ollama integration
- **memo-026** - Simplified local mode demo

### Analysis & Metrics

- **memo-004** - AI agent tool usage patterns and effectiveness
- **memo-005** - AI agent capabilities and limitations
- **memo-006** - Human enablement patterns for AI agents
- **memo-008** - AI session playbook (16 sessions analyzed)
- **memo-016** - Development velocity metrics (10-15x speedup)

### Implementation Reports

- **memo-037** - AI prompt templates reference
- **memo-038** - UUID integration summary
- **memo-039** - UUID migration guide
- **memo-040** - GTS template compilation debug log
- **memo-041** - People API architecture clarification

### Simplified Mode (RFC-009) Reference

- **memo-042** - Simplified mode architecture diagram
- **memo-043** - Simplified mode implementation checklist
- **memo-044** - Simplified mode summary

### API Provider (RFC-011) Reference

- **memo-045** - API provider permissions appendix

### Notification System (RFC-013) Reference

- **memo-046** - Notification backend addendum
- **memo-047** - Notification backends implementation
- **memo-048** - Notification Docker Compose setup
- **memo-049** - Notification message schema
- **memo-050** - Notification template scheme
- **memo-051** - Notification implementation status

### Event-Driven Indexer (RFC-014) Reference

- **memo-052** - Indexer architecture refactoring notes
- **memo-053** - Event-driven indexer implementation summary
- **memo-054** - Event-driven indexer production deployment
- **memo-055** - Semantic search release notes
- **memo-056** - Semantic search performance benchmarks
- **memo-057** - Query optimization analysis
- **memo-058** - Event-driven indexer testing status

### S3 Storage (RFC-015) Reference

- **memo-059** - S3 storage implementation summary

### Planning Trackers

- **memo-001** - RFC-014 milestone log (week 1-2)
- **memo-002** - RFC-014 milestone log (week 2-3)
- **memo-003** - Active RFC priority tracker and roadmap

### E2E Testing

- **memo-032** - E2E testing summary

## Document Organization

Memos are organized by type:
- ***-quickref.md / *-quickstart.md**: Quick reference guides
- ***-debug.md**: Debugging and investigation logs
- ***-summary.md**: Implementation and feature summaries
- ***-analysis.md**: Analysis and metrics documents
- ***-guide.md**: Setup and usage guides

## Memo Format

```markdown
---
id: memo-NNN
title: Descriptive Title
date: YYYY-MM-DD
type: Investigation | Implementation | Guide | Analysis | Session
status: Draft | Final | Archived
tags: [tag1, tag2, tag3]
related:
  - RFC-YYY
  - ADR-ZZZ
---

# Title

## Summary
## Content
## Outcomes/Next Steps
## References
```

## Contributing

1. Assign next sequential ID (NNN format)
2. Use descriptive kebab-case filename
3. Include frontmatter with date and type
4. Link related RFCs, ADRs, and memos
5. Update this README index