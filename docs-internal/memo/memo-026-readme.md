---
id: memo-026
created: 2026-04-24
author: Hermes Team
project_id: hermes
doc_uuid: 1f80098b-75c3-4ef1-b0be-8c4f459d40eb
status: Draft
title: "Memo Documents"
---
# Memo Documents

Implementation notes, quick reference guides, session summaries, and completion reports from Hermes development.

## Index

### Quick Reference Guides

- **memo-008** - Auth provider quick reference
- **memo-010** - Ember dev server, proxy config, upgrade strategy
- **memo-017** - Development workflow quick reference (build, test, deploy)
- **memo-023** - Dex OIDC quick start guide
- **memo-035** - Complete environment setup (10-minute quick start)
- **memo-052** - Outbox pattern quick reference with SQL queries
- **memo-058** - Playwright agent E2E testing guide
- **memo-071** - Authentication providers guide (Google, Okta, Dex)
- **memo-073** - Documentation hub and navigation guide
- **memo-093** - Environment variables setup guide
- **memo-096** - Makefile quick start targets
- **memo-097** - Project config API usage guide
- **memo-099** - Project config package implementation summary

### Component & Setup Guides

- **memo-012** - Algolia search provider setup
- **memo-013** - Auth provider testing guide
- **memo-014** - Dex OIDC local auth guide
- **memo-015** - Google Workspace integration guide
- **memo-018** - Hermes indexer readme
- **memo-020** - Jira integration guide
- **memo-021** - Local workspace provider setup
- **memo-022** - Meilisearch setup guide
- **memo-024** - Ollama AI provider setup
- **memo-025** - PostgreSQL database setup
- **memo-027** - Setup wizard guide
- **memo-028** - Setup wizard Ollama integration
- **memo-029** - Simplified local mode demo

### Analysis & Metrics

- **memo-004** - AI agent tool usage patterns and effectiveness
- **memo-005** - AI agent capabilities and limitations
- **memo-007** - Human enablement patterns for AI agents
- **memo-009** - AI session playbook (16 sessions analyzed)
- **memo-019** - Development velocity metrics (10-15x speedup)

### Implementation Reports

- **memo-100** - AI prompt templates reference
- **memo-101** - UUID integration summary
- **memo-102** - UUID migration guide
- **memo-104** - GTS template compilation debug log
- **memo-105** - People API architecture clarification

### Simplified Mode (RFC-083) Reference

- **memo-106** - Simplified mode architecture diagram
- **memo-107** - Simplified mode implementation checklist
- **memo-108** - Simplified mode summary

### API Provider (RFC-085) Reference

- **memo-109** - API provider permissions appendix

### Notification System (RFC-087) Reference

- **memo-110** - Notification backend addendum
- **memo-111** - Notification backends implementation
- **memo-112** - Notification Docker Compose setup
- **memo-113** - Notification message schema
- **memo-114** - Notification template scheme
- **memo-115** - Notification implementation status

### Event-Driven Indexer (RFC-088) Reference

- **memo-116** - Indexer architecture refactoring notes
- **memo-117** - Event-driven indexer implementation summary
- **memo-118** - Event-driven indexer production deployment
- **memo-119** - Semantic search release notes
- **memo-120** - Semantic search performance benchmarks
- **memo-121** - Query optimization analysis
- **memo-122** - Event-driven indexer testing status

### S3 Storage (RFC-089) Reference

- **memo-123** - S3 storage implementation summary

### Planning Trackers

- **memo-001** - RFC-088 milestone log (week 1-2)
- **memo-002** - RFC-088 milestone log (week 2-3)
- **memo-003** - Active RFC priority tracker and roadmap

### E2E Testing

- **memo-092** - E2E testing summary

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
