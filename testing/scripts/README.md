# Testing Scripts

Automation scripts for distributed document authoring and indexing scenarios.

## Directory Structure

```
scripts/
├── lib/
│   └── document-generator.sh    # Reusable document generation functions
├── seed-workspaces.sh            # Populate workspaces with test documents
└── scenario-basic.sh             # Basic distributed indexing test
```

## Scripts

### seed-workspaces.sh

Populate workspaces with test documents.

**Usage**:
```bash
./scripts/seed-workspaces.sh [options]

Options:
  --scenario <name>   Scenario to generate (basic|migration|conflict|multi-author)
  --count <n>         Number of documents (default: 10)
  --clean             Remove existing documents first
  --workspace <name>  Target workspace (testing|docs|all) (default: all)
```

**Examples**:
```bash
# Generate 10 basic test documents
./scripts/seed-workspaces.sh --scenario basic --count 10

# Clean and regenerate
./scripts/seed-workspaces.sh --scenario basic --count 10 --clean

# Migration scenario (creates duplicates with same UUID)
./scripts/seed-workspaces.sh --scenario migration --count 5

# Conflict scenario (modified content)
./scripts/seed-workspaces.sh --scenario conflict --count 5

# Multi-author scenario (different authors, timestamps, statuses)
./scripts/seed-workspaces.sh --scenario multi-author --count 10
```

### scenario-basic.sh

Run end-to-end basic distributed indexing scenario.

**What it does**:
1. Verifies Hermes is running
2. Seeds test documents
3. Waits for indexing
4. Verifies documents via API
5. Tests search functionality

**Usage**:
```bash
./scripts/scenario-basic.sh
```

**Prerequisites**:
- Hermes running: `make up`
- jq installed: `brew install jq` (macOS) or `apt install jq` (Linux)

## Library Functions

### document-generator.sh

Reusable functions for generating test documents.

**Functions**:
- `generate_uuid()` - Generate a UUID (cross-platform)
- `generate_timestamp()` - Generate ISO 8601 timestamp
- `generate_rfc(number, uuid, title, status, author, created)` - Generate RFC document
- `generate_prd(number, uuid, title, status, author, created)` - Generate PRD document
- `generate_meeting_notes(number, uuid, title, attendees, date, created)` - Generate meeting notes
- `generate_doc_page(title, uuid, category, author, created)` - Generate documentation page
- `ensure_dir(path)` - Create directory if it doesn't exist
- `write_document(filepath, content)` - Write document to file

**Usage**:
```bash
source ./scripts/lib/document-generator.sh

# Generate an RFC
uuid=$(generate_uuid)
content=$(generate_rfc 1 "$uuid" "My RFC" "draft")
write_document "./workspace/rfcs/RFC-001.md" "$content"
```

## Makefile Integration

Scripts are integrated into the testing Makefile:

```bash
# Seed workspaces
make seed              # Basic scenario, 10 documents
make seed-clean        # Clean and re-seed
make seed-migration    # Migration scenario
make seed-conflict     # Conflict scenario
make seed-multi-author # Multi-author scenario

# Run scenarios
make scenario-basic    # Basic indexing test

# Full test suite
make test-distributed  # Start environment, seed, and run scenarios

# Cleanup
make workspace-clean   # Clean workspace data
```

## Scenarios

### Basic
- Generates variety of document types (RFC, PRD, Meeting Notes)
- Distributed across testing workspace
- Simple indexing validation

### Migration
- Creates documents with same UUID in multiple workspaces
- Simulates migration from one provider to another
- Tests conflict detection when content differs

### Conflict
- Creates documents with deliberate content modifications
- Simulates concurrent edits during migration
- Validates conflict marker detection

### Multi-Author
- Documents from different authors
- Staggered timestamps (realistic timeline)
- Various statuses (draft, review, approved, published)
- Tests metadata extraction and filtering

## Adding New Scenarios

1. **Add scenario function** to `seed-workspaces.sh`:
   ```bash
   scenario_my_scenario() {
       echo -e "${BLUE}Generating my scenario...${NC}"
       # ... implementation
   }
   ```

2. **Add to case statement** in `seed-workspaces.sh`:
   ```bash
   case $SCENARIO in
       # ... existing scenarios
       my-scenario)
           scenario_my_scenario
           ;;
   esac
   ```

3. **Add Makefile target** in `Makefile`:
   ```makefile
   .PHONY: seed-my-scenario
   seed-my-scenario: ## Seed my custom scenario
       ./scripts/seed-workspaces.sh --scenario my-scenario --count 10
   ```

4. **Create scenario test script** (optional):
   ```bash
   cp scripts/scenario-basic.sh scripts/scenario-my-scenario.sh
   # Customize for your scenario
   ```

## Best Practices

### Test Data
- ✅ Use `example.com` domains for email addresses
- ✅ Generate random UUIDs (not real document IDs)
- ✅ Use fictional names and content
- ✅ Keep content generic and public-safe

### Performance
- ⚡ Generate in batches for large document sets
- ⚡ Use `--clean` sparingly (preserves existing data otherwise)
- ⚡ Wait for indexer scans (5-minute interval by default)

### Debugging
- 🔍 Check indexer logs: `make logs-hermes`
- 🔍 Verify file permissions in workspaces
- 🔍 Use `jq` to parse API responses
- 🔍 Check workspace mount points in docker-compose.yml

## Troubleshooting

### Documents not indexed
```bash
# Check indexer is running
docker compose ps hermes-indexer

# Check indexer logs
docker compose logs hermes-indexer

# Verify workspace mounts
docker compose exec hermes ls -la /app/workspaces/testing

# Check indexer configuration
docker compose exec hermes cat /app/projects.hcl
```

### Permission errors
```bash
# Fix workspace ownership (if needed)
sudo chown -R $(whoami) ./workspaces

# Or from container
docker compose exec hermes chown -R hermes:hermes /app/workspaces
```

### API not responding
```bash
# Check Hermes health
curl http://localhost:8001/health

# Check all services
make status

# Restart services
make restart
```

## Contributing

When adding new scripts:
1. Add descriptive header comment with usage
2. Use `set -e` for fail-fast behavior
3. Source `lib/document-generator.sh` for helpers
4. Add color output using `$BLUE`, `$GREEN`, `$YELLOW`, `$NC`
5. Include error handling and validation
6. Add to Makefile with `##` description
7. Document in this README

## References

- **Architecture**: `../DISTRIBUTED_TESTING_ENHANCEMENTS.md`
- **Project Configs**: `../projects/README.md`
- **Docker Compose**: `../docker-compose.yml`
- **Main README**: `../README.md`
