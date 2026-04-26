# Hermes Executive Demo Materials

This folder contains all materials for the Hermes Local-First Document Management demonstration.

## 📂 Contents

- **`demo-narrative.md`** - Complete presentation script and talking points
- **`DEMO-SCRIPT.sh`** - Automated demo execution script
- **`DEMO-readme.md`** - Quick start guide for demo execution
- **`demo1/`** - Example configurations and scripts

## 🚀 Quick Start

### Run the Automated Demo

**Full Demo** (10 minutes):
```bash
# From Hermes root directory
./docs-demo/DEMO-SCRIPT.sh
```

**Fast Version** (no pauses):
```bash
DEMO_PAUSE=0 ./docs-demo/DEMO-SCRIPT.sh
```

**Skip Setup** (if services already running):
```bash
SKIP_SETUP=true ./docs-demo/DEMO-SCRIPT.sh
```

**Individual Demos**:
```bash
# Run specific demo (1-4)
./docs-demo/DEMO-SCRIPT.sh --demo 1  # One-command local setup
./docs-demo/DEMO-SCRIPT.sh --demo 2  # Multi-provider configuration
./docs-demo/DEMO-SCRIPT.sh --demo 3  # Provider abstraction architecture
./docs-demo/DEMO-SCRIPT.sh --demo 4  # Local testing excellence
```

### Manual Demo Setup

If you prefer to run commands manually:

```bash
# 1. Start services
cd testing && docker compose up -d

# 2. Verify services
docker compose ps

# 3. Open browser
open http://localhost:4201

# Login: test@hermes.local / password
```

## 📋 Presentation Structure

The demo is organized into four sections:

1. **One-Command Local Setup** (2 minutes)
   - Complete environment in Docker
   - Zero cloud dependencies
   - Instant productivity

2. **Multi-Provider Configuration** (3 minutes)
   - Local workspace (Markdown files)
   - Configuration-driven backends
   - Production flexibility

3. **Provider Abstraction Architecture** (2 minutes)
   - Design decisions (ADR-073)
   - Migration pipeline (RFC-080)
   - Document identity and versioning

4. **Local Testing Excellence** (3 minutes)
   - Playwright E2E tests
   - Zero cloud API calls
   - Cost and velocity improvements

**Total**: ~10 minutes demo + ~10 minutes Q&A

## 🎯 Key Messages

### Technical Achievements
- ✅ **42,000+ lines** of production Go code
- ✅ **50,000+ lines** of TypeScript/JavaScript frontend
- ✅ **70 design documents** (16 ADRs, 19 RFCs, 35 MEMOs)
- ✅ **3 auth providers**, 2 workspace providers, 2 search providers
- ✅ **One-command setup**: `docker compose up -d`
- ✅ **Modern stack**: Ember 6.8, Go 1.25, HashiCorp Design System 4.24

### Value Proposition
1. **Local-First Development**: Zero cloud dependencies for testing
2. **Provider Flexibility**: Swap auth/workspace/search via configuration
3. **Migration Support**: Move documents between providers without data loss
4. **Cost Efficiency**: Eliminate cloud API costs in dev/test environments
5. **Production Options**: Support Google, Office365 (Q1), self-hosted

### Competitive Advantages

**vs. Google Drive / Microsoft 365**:
- Not locked to single cloud provider
- Local-first development and testing
- Open source, self-hostable
- Custom approval workflows

**vs. Confluence / Notion**:
- Full data ownership (no SaaS lock-in)
- Pluggable search (Meilisearch with vector search)
- Multi-provider architecture
- Developer-friendly (Git-like workflow)

**vs. Custom Solutions**:
- Production-ready (42K+ lines, 70+ design docs)
- Multi-tenant support
- Comprehensive auth (OAuth, OIDC, enterprise SSO)
- Battle-tested provider abstractions

## 📊 Concrete Metrics

**Development Velocity**:
- Setup time: **5 minutes** (down from hours)
- Test execution: **~30 seconds** (no cloud latency)
- Cost per developer: **$0/month** (vs. $50+ for cloud dev accounts)

**Architecture**:
- Backend: **42,000+ lines** of Go 1.25 code, 782 files
- Frontend: **50,000+ lines** TypeScript/JavaScript (Ember 6.8)
- UI Framework: HashiCorp Design System 4.24
- Documentation: **70 design documents**
- Providers: **7 total** (3 auth, 2 workspace, 2 search)

**Testing**:
- E2E suite: Playwright covering full document lifecycle
- Local environment: 5 Docker services
- Cloud dependencies: **Zero** for dev/test

## 🗓️ Roadmap

**Q1 2025** (Near-Term):
- Office365 workspace provider (2-3 weeks)
- Enhanced LLM integration (auto-summaries, semantic search)
- Multi-organization support

**Q2 2025** (Medium-Term):
- Distributed projects (cross-org collaboration)
- Enhanced review workflows (inline comments, notifications)
- Advanced search (vector embeddings, recommendations)

**Future**:
- Real-time collaboration (Yjs/Automerge)
- Mobile applications
- Plugin/extension system

## 🛠️ Prerequisites

**Required**:
- Docker Desktop OR Podman
- Git

**Optional** (for code exploration):
- Go 1.25+
- Node.js 20+
- Yarn 4.10+

**Hardware**: Any modern dev machine (Mac/Linux/Windows)

## 📝 Customization

### Modify the Narrative

Edit `demo-narrative.md` to:
- Add organization-specific context
- Highlight particular features
- Adjust timing for different audiences

### Customize the Script

Edit `DEMO-SCRIPT.sh` to:
- Change demo order
- Add/remove sections
- Adjust pause durations

Environment variables:
```bash
DEMO_PAUSE=N       # Seconds between sections (default: 3)
SKIP_SETUP=true    # Skip Docker setup if already running
HERMES_ROOT=path   # Path to Hermes repo (default: ..)
```

## 🎨 Presentation Tips

### Before the Meeting

1. ✅ **Test run**: Execute `./DEMO-SCRIPT.sh` once to verify
2. ✅ **Pre-start services**: Run `cd testing && docker compose up -d`
3. ✅ **Prepare browser**: Open http://localhost:4201 in advance
4. ✅ **Large fonts**: Use 16-18pt terminal font for visibility
5. ✅ **Review narrative**: Read demo-narrative.md for talking points

### During Presentation

1. **Show, don't tell**: Live browser interaction, real files, actual tests
2. **Highlight numbers**: 42K lines, 70 docs, 5 min setup, $0 cost
3. **Pause for impact**: Allow questions after each section
4. **Use real examples**: Show actual Markdown files, test output
5. **Stay on message**: Local-first, provider flexibility, cost efficiency

### For Different Audiences

**Executives** (5 minutes):
- Focus on metrics and business value
- Skip technical details
- Emphasize cost savings and flexibility

**Engineering Leadership** (20 minutes):
- Full demo with technical depth
- Show architecture decisions (ADRs)
- Discuss roadmap and trade-offs

**Developers** (30 minutes):
- Deep dive into code
- Live debugging and exploration
- Provider implementation details

## 🐛 Troubleshooting

### Services Won't Start
```bash
# Check logs
cd testing && docker compose logs

# Restart
docker compose down && docker compose up -d
```

### Port Conflicts
```bash
# Check what's using ports
lsof -i :8001  # Backend
lsof -i :4201  # Frontend
lsof -i :5432  # PostgreSQL
lsof -i :7700  # Meilisearch
lsof -i :5556  # Dex

# Kill conflicting processes or change ports in docker-compose.yml
```

### Frontend Not Loading
```bash
# Rebuild frontend container
cd testing
docker compose build hermes-frontend
docker compose up -d hermes-frontend

# Check logs
docker compose logs hermes-frontend
```

### Database Issues
```bash
# Reset database
cd testing
docker compose down -v  # Removes volumes
docker compose up -d
```

## 📚 Additional Resources

### Documentation
- [Testing Environment Guide](../testing/readme.md)
- [Configuration Documentation](../docs-internal/CONFIG_HCL_DOCUMENTATION.md)
- [ADR-071: Local File Workspace](../docs-internal/adr/adr-071-local-file-workspace-system.md)
- [ADR-073: Provider Abstraction](../docs-internal/adr/adr-073-provider-abstraction-architecture.md)
- [RFC-080: Document Sync](../docs-internal/rfc/rfc-080-outbox-pattern-document-sync.md)

### Related Guides
- [Dex Authentication Setup](../docs-internal/memo/README-dex.md)
- [Local Workspace Setup](../docs-internal/memo/README-local-workspace.md)
- [Meilisearch Configuration](../docs-internal/memo/README-meilisearch.md)
- [Playwright E2E Testing](../docs-internal/PLAYWRIGHT_E2E_AGENT_GUIDE.md)

## 🤝 Post-Demo Follow-Up

### Share with Stakeholders

1. **demo-narrative.md** - Complete context and talking points
2. **ADR-073** - Provider abstraction architecture
3. **ADR-071** - Local file workspace system
4. **RFC-080** - Document migration design

### Discussion Points

**Immediate Decisions**:
- Office365 provider priority? (Q1 2025 target)
- LLM integration scope? (summarization, semantic search)
- Multi-org support timing? (Q2 2025)

**Deployment Planning**:
- Self-hosted vs cloud-managed infrastructure?
- Pilot use cases for local workspace?
- Migration from existing systems?

## 📧 Contact

For questions about the demo:
- Technical details: See ADRs in `docs-internal/adr/`
- Architecture decisions: See RFCs in `docs-internal/rfc/`
- Setup issues: See `testing/readme.md`

## 📜 License

Follow project license for code artifacts. Demo materials are for internal use.

---

**Quick Links**:
- [Run Demo](./DEMO-SCRIPT.sh)
- [Narrative](./demo-narrative.md)
- [Setup Guide](./DEMO-readme.md)
- [Main README](../readme.md)
