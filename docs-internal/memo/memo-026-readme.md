# Memo Documents

Status updates, investigation findings, session summaries, quick reference guides, and completion reports from Hermes development.

## Index

### Quick Reference Guides ✅

- **memo-008-auth-provider-quickref.md** - Quick reference for authentication provider configuration and selection
- **memo-010-ember-dev-server.md** - Ember development server, proxy configuration, and upgrade strategy
- **memo-017-dev-quickref.md** - Development workflow quick reference (build, test, deploy)
- **memo-023-dex-quickstart.md** - Dex OIDC authentication quick start guide
- **memo-035-env-setup.md** - **NEW!** Complete environment setup guide for new developers (10-minute quick start)
- **memo-052-outbox-pattern-quickref.md** - Outbox pattern implementation quick reference with SQL queries and patterns
- **memo-058-playwright-agent-guide.md** - Comprehensive guide for AI agents using Playwright for E2E testing
- **memo-071-auth-providers-guide.md** - **NEW!** Complete authentication providers guide (Google, Okta, Dex)
- **memo-073-docs-internal-hub.md** - **NEW!** Documentation hub and navigation guide

### Analysis & Metrics ✅

- **memo-004-agent-usage-analysis.md** - Analysis of AI agent tool usage patterns and effectiveness
- **memo-016-deliverables-summary.md** - Project deliverables and milestone summary
- **memo-019-dev-velocity-analysis.md** - Development velocity metrics and analysis

### Implementation Reports ✅

- **memo-009-ai-session-playbook.md** - AI agent session playbook with success patterns and anti-patterns
- **memo-011-config-cleanup.md** - Configuration file cleanup - removing duplicates (config-example.hcl, dex-config.yaml)
- **memo-086-memo-organization-2025-10-09.md** - Documentation reorganization into structured memos

### Current Planning Trackers

- **2025-11-15-rfc-implementation-tracker.md** - Active RFC priority tracker and roadmapping notes
- **2025-11-15-rfc-088-week1-2-complete.md** - RFC-088 milestone log (week 1-2)
- **2025-11-15-rfc-088-week2-3-complete.md** - RFC-088 milestone log (week 2-3)

### Investigation & Root Cause Analysis 🔜

- **MEMO-001-admin-login-hang-rootcause.md** - Root cause analysis for admin login dashboard loading spinner hang in Promise.all()
- **MEMO-075-rootcause-fetchpeople-hang.md** - Root cause analysis for maybeFetchPeople hang causing admin login spinner issue

### Implementation & Completion Reports 🔜

- **MEMO-025-doc-content-integration-complete.md** - Document content API integration test completion summary
- **MEMO-045-local-workspace-complete.md** - Local workspace provider implementation completion report
- **MEMO-084-testing-env-complete.md** - Testing environment setup and configuration completion

### README Files

- ✅ **memo-071-auth-providers-guide.md** - Authentication providers guide (Google, Okta, Dex)
- 🔜 **MEMO-072-local-workspace-readme.md** - Local workspace provider setup and usage documentation
- ✅ **memo-073-docs-internal-hub.md** - Main documentation hub for docs-internal directory structure

### Legacy Provider & Setup READMEs

- ✅ **README-algolia.md** - Algolia search provider setup and operations
- ✅ **README-auth-providers.md** - Auth provider overview and selection
- ✅ **README-dex.md** - Dex OIDC local auth setup
- ✅ **README-google-workspace.md** - Google Workspace integration guide
- ✅ **README-indexer.md** - Indexer architecture and provider mapping
- ✅ **README-jira.md** - Jira integration guide
- ✅ **README-meilisearch.md** - Meilisearch setup and tuning
- ✅ **README-ollama.md** - Ollama provider and embedding setup
- ✅ **README-postgresql.md** - PostgreSQL database setup
- ✅ **README-local-workspace.md** - Local workspace provider setup
- ✅ **SETUP_WIZARD_GUIDE.md** - Setup wizard implementation and usage
- ✅ **SETUP_WIZARD_OLLAMA.md** - Ollama setup wizard extension notes
- ✅ **SIMPLIFIED_MODE_DEMO.md** - Simplified mode demo instructions
- ✅ **SQLITE_DRIVER_CONFLICT.md** - SQLite conflict rationale and migration notes
- ✅ **VALIDATION_AUTO_MIGRATION.md** - Validation workflow and automatic migration notes

## Document Organization

Memos are organized by type:

- **MEMO-NNN-*-rootcause.md**: Root cause analysis documents
- **MEMO-NNN-*-complete.md**: Implementation completion reports  
- **MEMO-NNN-*-quickref.md** / **NNN-*-quickstart.md**: Quick reference guides
- **MEMO-NNN-*-readme.md**: README documentation
- **MEMO-NNN-*-analysis.md**: Analysis and metrics documents
- **MEMO-NNN-*-summary.md**: Session and feature summaries
- **MEMO-NNN-*-validation.md**: Validation and test results

## Memo Format

All memos MUST include YAML frontmatter for metadata tracking and searchability:

```markdown
---
id: MEMO-NNN
title: Descriptive Title
date: YYYY-MM-DD
type: Investigation | Implementation | Guide | Analysis | Validation | Session
status: Draft | Final | Archived
tags: [tag1, tag2, tag3]
related:
  - MEMO-XXX
  - RFC-YYY
  - ADR-ZZZ
---

# Title

## Summary
Brief overview of the memo's purpose

## Content
Detailed information, findings, or instructions

## Outcomes/Next Steps
Results, action items, or follow-up work

## References
Related documents and resources
```

**Frontmatter Fields**:
- `id`: Sequential memo identifier (MEMO-001, MEMO-002, etc.)
- `title`: Full descriptive title
- `date`: Creation or last update date (YYYY-MM-DD)
- `type`: Document category (see types below)
- `status`: Draft (WIP), Final (complete), Archived (historical)
- `tags`: Searchable keywords (e.g., authentication, testing, playwright)
- `related`: Links to related documents (optional)

## Contributing

When adding new memos:
1. Assign next sequential ID (NNN format)
2. Use descriptive kebab-case filename  
3. Include type indicator in filename (-complete, -quickref, -analysis, etc.)
4. Add front matter with date and type
5. Link related RFCs, ADRs, and memos
6. Update this README index in appropriate section
