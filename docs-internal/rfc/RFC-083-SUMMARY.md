# RFC-083 Summary: Simplified Local Mode

## What This Enables

Transform Hermes from enterprise-only to **universal document CMS**:

```bash
# Download binary
curl -L https://releases.hermes/latest/hermes-macos > hermes
chmod +x hermes

# Run in any directory
./hermes

# Browser opens automatically → Start writing documents
```

**No configuration. No external services. No complexity.**

## Key Changes

### User Experience
- **Before**: 30-60 min setup (PostgreSQL, config files, auth setup)
- **After**: 5 minutes (download → first document created)

### Technical Architecture

| Component | Enterprise Mode | Simplified Mode |
|-----------|----------------|-----------------|
| **Database** | PostgreSQL (external) | SQLite (embedded) |
| **Search** | Algolia/Meilisearch (external) | Bleve (embedded) |
| **Auth** | Google/Okta/Dex (OAuth/OIDC) | Trust-based local |
| **Workspace** | Google Drive | Local filesystem |
| **Config** | `config.hcl` (200+ lines) | `config.yaml` (auto-generated) |
| **Binary** | ~40MB | ~50MB |

### Well-Known Directory

```
./docs-cms/
├── config.yaml              # Optional overrides
├── data/
│   ├── hermes.db           # SQLite database
│   └── fts.index           # Bleve search index
├── documents/              # Published markdown docs
├── drafts/                 # Draft documents
├── attachments/            # Images, PDFs
└── templates/              # RFC, PRD, FRD templates
```

## Implementation Phases

**Phase 1** (Week 1-2): Foundation
- Zero-config command detection
- SQLite integration
- Auto-initialize `./docs-cms/`

**Phase 2** (Week 3-4): Embedded Search
- Bleve full-text search adapter
- Document indexing pipeline

**Phase 3** (Week 5): UX Polish
- Auto-open browser on startup
- "Getting Started" guide in UI
- File browser integration

**Phase 4** (Week 6): Distribution
- Homebrew formula
- Docker image
- Documentation updates

## Use Cases Unlocked

1. **Solo Developer**: Personal RFC/design doc repository
2. **Small Teams**: 2-5 people, shared network folder
3. **Education**: Students learning document-driven development
4. **Evaluation**: Try Hermes before enterprise deployment
5. **Offline Work**: No internet required

## Migration Path

**Simplified → Enterprise** (as team grows):
```bash
# Export SQLite to PostgreSQL
./hermes operator migrate-local-to-postgresql

# Generate enterprise config
./hermes operator generate-config --from-local
```

## Risks & Mitigations

| Risk | Mitigation |
|------|------------|
| Binary size (+10MB) | Acceptable for single binary (50MB total) |
| SQLite scale limits | Document limit (10K docs), provide migration path |
| Search quality vs. Algolia | Tune Bleve, extensive testing |
| User expectations | Clear messaging about modes |

## Success Metrics (3 months)

- ✅ **1,000+ downloads** of standalone binary
- ✅ **500+ new users** (via opt-in telemetry)
- ✅ **< 5 minutes** time to first document
- ✅ **10% conversion** to enterprise mode

## Open Questions for Review

1. Default port: 8000 (current) or 3000 (common for dev tools)?
2. Opt-in telemetry for adoption tracking?
3. Auto-update mechanism or package managers only?
4. Ship with full templates or minimal + download?
5. Windows support in Phase 1 or defer to Phase 2?

## Related Documents

- **RFC-083**: [Full RFC](RFC-083-simplified-local-mode.md) (detailed architecture)
- **README-local-workspace.md**: Existing local workspace provider
- **README-indexer.md**: Document indexing patterns

## Next Steps

1. ✅ **Team review** of RFC (1 week)
2. ⏭️ **Technical spikes**:
   - Bleve integration (2 days)
   - SQLite GORM migration (1 day)
3. ⏭️ **Phase 1 implementation** (2 weeks)

---

**Vision**: Make Hermes as easy to use as Obsidian, as powerful as Notion, and as open as Markdown.
