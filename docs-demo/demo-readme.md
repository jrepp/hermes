# Hermes Executive-Level Demo Guide

**Created**: 2025-11-10
**Duration**: ~20 minutes (10 min demo + 10 min discussion)
**Audience**: Technical stakeholders and engineering leadership

## Overview

This demo showcases Hermes' evolution from cloud-dependent to fully local-verifiable document management system, highlighting the multi-provider architecture that eliminates vendor lock-in.

## What You'll Demonstrate

1. **One-Command Local Setup**: Complete environment in Docker with zero cloud dependencies
2. **Multi-Provider Architecture**: Configuration-driven backend selection (auth, workspace, search)
3. **Document Migration Pipeline**: Move documents between providers without data loss
4. **Local Testing Excellence**: E2E tests with no cloud API calls or credentials
5. **Production Flexibility**: Same code runs with local or cloud backends

## Prerequisites

**Required Software**:
- Docker Desktop OR Podman
- Go 1.25+
- Node.js 20+
- Yarn 4.10+
- Git

**Time to Setup**: 5 minutes

**Hardware**: Any modern dev machine (Mac/Linux/Windows)

## Quick Start

```bash
# Navigate to Hermes repo
cd /Users/jrepp/hc/hermes

# Start complete testing environment
cd testing && docker compose up -d

# Verify services are running
docker compose ps

# Open browser to http://localhost:4201
# Login: test@hermes.local / password
```

## Demo Structure

### Part 1: The Narrative (3 minutes)

Use `DEMO-NARRATIVE.md` to tell the story:

1. **Business Problem** (30 seconds)
   - Cloud dependency for testing
   - Vendor lock-in to Google Workspace
   - Auth complexity for local development
   - Testing costs from API calls

2. **Hermes Solution** (1 minute)
   - Local-first architecture
   - Provider abstraction pattern
   - Configuration-driven backends
   - Zero cloud dependencies for dev/test

3. **Current Status** (1.5 minutes)
   - 42K+ lines of Go code
   - 50K+ lines of TypeScript frontend
   - 3 auth providers, 2 workspace providers, 2 search providers
   - 70 design documents (16 ADRs, 19 RFCs, 35 MEMOs)
   - One-command local setup

### Part 2: Live Demos (10 minutes)

**Demo 1: One-Command Local Setup** (2 minutes)
- Start: `cd testing && docker compose up -d`
- Show services: PostgreSQL, Meilisearch, Dex, backend, frontend
- Browser demo: Create document, search, approval workflow
- Key metric: Zero cloud credentials needed

**Demo 2: Multi-Provider Configuration** (3 minutes)
- Show local workspace files: `ls testing/workspace_data/`
- Show config.hcl with local providers
- Compare to production config with Google providers
- Key insight: Same code, different backends via config

**Demo 3: Document Migration Pipeline** (2 minutes)
- Show migration command: `./hermes migrate --help`
- Explain UUID-based document identity
- Highlight metadata preservation across providers
- Note LLM integration for auto-summaries

**Demo 4: Local Testing** (3 minutes)
- Show Playwright test structure: `ls tests/e2e-playwright/tests/`
- Run tests: `cd tests/e2e-playwright && npx playwright test`
- Highlight: ~30 seconds, zero cloud calls, $0 cost
- Compare to previous: hours of OAuth setup, API costs

### Part 3: Q&A (10 minutes)

Common questions addressed in `DEMO-NARRATIVE.md`:
- Why multiple workspace providers?
- Performance overhead from abstraction?
- Maturity of local workspace?
- Migration from existing Google Docs?
- Total cost of ownership?
- Production readiness?

## Demo Execution Tips

### Before the Demo

1. **Test Run**: Execute demo commands once to verify everything works
2. **Pre-Build Images**: `cd testing && docker compose pull` to cache images
3. **Terminal Setup**: Use large font (16-18pt) for visibility
4. **Browser Setup**: Open localhost:4201 in advance, keep login page ready
5. **Documentation Ready**: Have DEMO-NARRATIVE.md open for reference

### During the Demo

1. **Start Services Early**: Run `docker compose up -d` before presenting
2. **Show, Don't Tell**: Live browser interaction, real file system, actual tests
3. **Highlight Numbers**: 42K lines, 70 docs, 5 min setup, $0 cost, 30 sec tests
4. **Pause for Impact**: After each demo section, allow questions
5. **Use Real Examples**: Show actual Markdown files, real test output

### Handling Questions

**Q: Is this production-ready?**
- A: Local workspace is production-ready for development. Google Workspace is production-proven. Office365 coming Q1 2025.

**Q: What's the migration path from our current system?**
- A: Migration pipeline supports Google Workspace → Hermes (local or cloud). Preserves all metadata and versions.

**Q: How does local workspace compare to Google Docs?**
- A: Local is faster, cheaper, no cloud dependency. Trade-off: no real-time collaboration (yet). Best for development/testing or air-gapped deployments.

**Q: Can we self-host everything?**
- A: Yes. PostgreSQL + Meilisearch + Dex = fully self-hosted stack. No external dependencies beyond OIDC (if using Google/Okta for production auth).

## Demo Variants

### Fast Track (5 minutes)
- Demo 1: One-command setup (2 min)
- Demo 4: Local testing (2 min)
- Summary (1 min)

### Deep Dive (30 minutes)
- Add architecture walkthrough (show ADR-073)
- Live code tour: provider interfaces
- Database schema exploration
- Performance profiling demonstration
- Migration pipeline deep dive

### Executive Summary (2 minutes)
- Show running environment (http://localhost:4201)
- Show test results (30 seconds, all passing)
- Show codebase stats (42K+ Go, 50K+ frontend, 70 docs)
- Timeline: Office365 Q1, enhancements Q2

## Success Metrics

After the demo, stakeholders should understand:

1. ✅ **Local-First Development**: Zero cloud dependencies for testing
2. ✅ **Provider Flexibility**: Swap auth/workspace/search via config
3. ✅ **Migration Support**: Move documents between providers safely
4. ✅ **Cost Efficiency**: Eliminate cloud API costs in dev/test
5. ✅ **Production Options**: Support Google, future Office365, self-hosted

## Troubleshooting

### Docker Connection Errors
```bash
# Verify Docker is running
docker ps

# For Podman on macOS
export DOCKER_HOST="unix://$(podman machine inspect --format '{{.ConnectionInfo.PodmanSocket.Path}}')"
```

### Services Won't Start
```bash
# Check logs
cd testing && docker compose logs

# Restart services
docker compose down && docker compose up -d

# Verify ports are available (8000, 8001, 4201, 5432, 7700, 5556)
lsof -i :8001
```

### Frontend Build Fails
```bash
# Rebuild frontend container
cd testing
docker compose build hermes-frontend
docker compose up -d hermes-frontend
```

### Database Connection Issues
```bash
# Verify PostgreSQL is healthy
docker compose exec postgres psql -U postgres -d hermes -c "SELECT COUNT(*) FROM documents;"

# Reset database if needed
docker compose down -v
docker compose up -d
```

## Post-Demo Follow-Up

### Share with Stakeholders

1. **Narrative Document**: Email `DEMO-NARRATIVE.md` (comprehensive context)
2. **ADR-073**: Provider abstraction architecture
3. **ADR-071**: Local file workspace system
4. **RFC-080**: Outbox pattern for document sync
5. **Testing README**: `testing/README.md` for setup details

### Next Steps Discussion

**Immediate Actions** (if approved):
1. Schedule engineering sync to discuss Office365 provider
2. Review roadmap priorities (Q1 vs Q2 features)
3. Discuss deployment targets (self-hosted vs cloud-managed)
4. Identify pilot use cases for local workspace

**Decisions Needed**:
1. Office365 provider priority (Q1 2025 target realistic?)
2. LLM integration scope (summarization, semantic search, recommendations?)
3. Multi-org support timing (Q2 2025?)
4. Real-time collaboration requirements (future phase?)

## Files Reference

- `DEMO-NARRATIVE.md` - Complete presentation script and talking points
- `DEMO-SCRIPT.sh` - Automated demo execution script
- `docs-internal/adr/ADR-071-local-file-workspace-system.md` - Local workspace design
- `docs-internal/adr/ADR-073-provider-abstraction-architecture.md` - Provider pattern
- `docs-internal/adr/ADR-075-meilisearch-as-local-search-solution.md` - Search provider
- `docs-internal/rfc/RFC-080-outbox-pattern-document-sync.md` - Migration design
- `testing/README.md` - Complete testing environment documentation
- `tests/e2e-playwright/` - E2E test suite

## Demo Script Commands

### Setup
```bash
cd /Users/jrepp/hc/hermes/testing
docker compose up -d
docker compose ps
```

### Demo 1: Local Environment
```bash
# Open browser
open http://localhost:4201

# Login: test@hermes.local / password
# Create document, demonstrate search
```

### Demo 2: Configuration
```bash
# Show local config
cat ../config.hcl | grep -A 5 "providers"

# Show workspace files
ls -la workspace_data/drafts/
cat workspace_data/drafts/RFC-001-example.md
```

### Demo 3: Architecture
```bash
# Show provider abstraction
cat ../docs-internal/adr/ADR-073-provider-abstraction-architecture.md | head -50

# Show migration RFC
cat ../docs-internal/rfc/RFC-080-outbox-pattern-document-sync.md | head -50
```

### Demo 4: Testing
```bash
cd ../tests/e2e-playwright
ls -la tests/
npx playwright test --reporter=line
```

### Cleanup
```bash
cd ../testing
docker compose down
# Keep volumes: docker compose down (without -v)
# Remove volumes: docker compose down -v
```

## Contact

For questions about the demo:
- Technical details: See ADRs in `docs-internal/adr/`
- Architecture decisions: See RFCs in `docs-internal/rfc/`
- Setup issues: See `testing/README.md`

## License

Internal demo materials. Follow project license for code artifacts.
