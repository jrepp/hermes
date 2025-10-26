# Distributed Testing Quick Reference

**TL;DR**: Automated test document generation and distributed indexing scenarios for Hermes.

## Quick Start

```bash
cd testing
make up                  # Start Hermes environment
make seed                # Generate 10 test documents
make scenario-basic      # Run E2E test
make open                # View in browser
```

## Common Commands

### Seed Workspaces
```bash
make seed                # 10 basic documents
make seed-clean          # Clean + regenerate
make seed-migration      # Migration scenario (same UUID, different workspaces)
make seed-conflict       # Conflict scenario (modified content)
make seed-multi-author   # Multi-author scenario (realistic timeline)
```

### Run Scenarios
```bash
make scenario-basic      # Basic indexing test (E2E)
make test-distributed    # Full test suite
```

### Cleanup
```bash
make workspace-clean     # Clean workspace data
make clean               # Stop containers + remove volumes
```

## Script Usage

### Direct Script Execution
```bash
# Seed workspaces
./scripts/seed-workspaces.sh --scenario basic --count 10 --clean

# Run basic scenario
./scripts/scenario-basic.sh

# Use generator library
source ./scripts/lib/document-generator.sh
uuid=$(generate_uuid)
generate_rfc 1 "$uuid" "My RFC" > workspace/rfcs/RFC-001.md
```

## Scenarios Explained

| Scenario | Purpose | Documents | Use Case |
|----------|---------|-----------|----------|
| **basic** | General testing | RFC, PRD, Meeting | Basic indexing validation |
| **migration** | Same UUID in 2 workspaces | 5 pairs | Migration testing, conflict detection |
| **conflict** | Modified content | 5 with edits | Concurrent edit simulation |
| **multi-author** | Realistic timeline | 10 varied | Metadata extraction, filtering |

## Document Types

- **RFC**: Request for Comments (technical proposals)
- **PRD**: Product Requirements Documents
- **MEETING**: Meeting notes
- **DOCUMENTATION**: Public documentation pages

## File Locations

```
testing/
├── scripts/
│   ├── lib/document-generator.sh     # Generator library
│   ├── seed-workspaces.sh            # Seeding tool
│   └── scenario-basic.sh             # E2E test
├── fixtures/
│   ├── rfcs/RFC-TEMPLATE.md          # RFC template
│   └── prds/PRD-TEMPLATE.md          # PRD template
└── workspaces/
    ├── testing/                       # TEST project workspace
    │   ├── rfcs/
    │   ├── prds/
    │   └── meetings/
    └── docs/                          # DOCS project workspace
        └── docs/
```

## Verification

### Check Documents Created
```bash
ls -lh workspaces/testing/rfcs/
cat workspaces/testing/rfcs/RFC-001-test-rfc.md
```

### Check Indexing
```bash
# Wait for indexer (5-minute interval)
docker compose logs -f hermes-indexer

# Query API
curl http://localhost:8001/api/v2/documents | jq '.total'
curl http://localhost:8001/api/v2/documents?type=RFC | jq
```

### Check Search
```bash
curl http://localhost:8001/api/v2/search?q=test | jq '.hits | length'
```

## Troubleshooting

### Documents Not Indexed
```bash
# Check indexer running
docker compose ps hermes-indexer

# View logs
docker compose logs hermes-indexer | tail -20

# Verify workspace mounts
docker compose exec hermes ls -la /app/workspaces/testing
```

### Permission Errors
```bash
# Fix ownership
sudo chown -R $(whoami) ./workspaces
```

### Seed Script Fails
```bash
# Make executable
chmod +x scripts/*.sh scripts/lib/*.sh

# Check bash version
bash --version  # Need 3.2+
```

## Example Workflow

### Test Basic Indexing
```bash
# 1. Start clean
cd testing
make clean
make up

# 2. Generate test docs
make seed

# 3. Wait for indexing (or check logs)
make logs-hermes | grep "indexed"

# 4. Verify via API
curl http://localhost:8001/api/v2/documents | jq '.total'

# 5. Test search
curl http://localhost:8001/api/v2/search?q=RFC | jq

# 6. Open UI
make open
```

### Test Migration Scenario
```bash
# 1. Generate migration documents (same UUID in 2 workspaces)
make seed-migration

# 2. Wait for indexing

# 3. Check for conflicts
curl http://localhost:8001/api/v2/documents?filter=conflicts | jq

# 4. Verify UUID resolution
curl http://localhost:8001/api/v2/documents/{uuid} | jq
```

## Next Steps

- **Phase 2**: Multi-indexer setup (docker-compose profiles)
- **Phase 3**: Migration scenario automation
- **Phase 4**: Performance testing, monitoring dashboard

## Documentation

- **Full Design**: `DISTRIBUTED_TESTING_ENHANCEMENTS.md`
- **Implementation**: `IMPLEMENTATION_SUMMARY.md`
- **Scripts Guide**: `scripts/README.md`
- **Templates**: `fixtures/README.md`
- **Architecture**: `../docs-internal/DISTRIBUTED_PROJECTS_ARCHITECTURE.md`

---

**Quick Help**: Run `make help` to see all available targets
