# Indexer Refactor TODO

Complete task list for refactoring the Hermes indexer to use command pattern and pipeline architecture with provider abstraction.

## ✅ Phase 1: Core Abstractions (COMPLETE)

### Core Interfaces
- [x] Create `pkg/indexer/command.go` - Command interface definitions
  - [x] `Command` interface with `Execute()` and `Name()`
  - [x] `BatchCommand` interface for batch operations
  - [x] `DiscoverCommand` interface for document discovery

- [x] Create `pkg/indexer/context.go` - Document context and filters
  - [x] `DocumentContext` struct to hold document state
  - [x] `LoadMetadata()` method to populate from database
  - [x] `DocumentFilter` function type
  - [x] `RecentlyModifiedFilter()` helper
  - [x] `DocumentTypeFilter()` helper
  - [x] `StatusFilter()` helper
  - [x] `CombineFilters()` helper

- [x] Create `pkg/indexer/pipeline.go` - Pipeline execution engine
  - [x] `Pipeline` struct with Commands, Filter, MaxParallel
  - [x] `Execute()` method with filtering and parallel processing
  - [x] `ParallelProcess()` generic helper function
  - [x] Support for DiscoverCommand handling
  - [x] Support for BatchCommand optimization

- [x] Create `pkg/indexer/orchestrator.go` - Main orchestrator
  - [x] `Orchestrator` struct
  - [x] `NewOrchestrator()` with functional options
  - [x] `RegisterPipeline()` method
  - [x] `ExecutePipeline()` method
  - [x] `Run()` continuous execution
  - [x] `RunOnce()` single execution
  - [x] `runCycle()` internal cycle logic
  - [x] Functional options: WithDatabase, WithLogger, WithWorkspaceProvider, etc.

- [x] Create `pkg/indexer/config.go` - Configuration structures
  - [x] `Config` struct

### Documentation
- [x] Create `pkg/indexer/README.md` - Package documentation
  - [x] Overview and key concepts
  - [x] Usage examples
  - [x] Custom command examples
  - [x] Testing examples

## ✅ Phase 2: Basic Commands (COMPLETE)

### Command Implementations
- [x] Create `pkg/indexer/commands/discover.go`
  - [x] `DiscoverCommand` struct
  - [x] `Discover()` method to query workspace provider
  - [x] Support for Since/Until time filtering
  - [x] Support for custom DocumentFilter

- [x] Create `pkg/indexer/commands/extract.go`
  - [x] `ExtractContentCommand` struct
  - [x] `Execute()` to get document content
  - [x] Support for MaxSize trimming
  - [x] `ExecuteBatch()` for parallel extraction

- [x] Create `pkg/indexer/commands/load_metadata.go`
  - [x] `LoadMetadataCommand` struct
  - [x] `Execute()` to load from database
  - [x] `ExecuteBatch()` for parallel loading

- [x] Create `pkg/indexer/commands/transform.go`
  - [x] `TransformCommand` struct
  - [x] `Execute()` to convert to search document
  - [x] Support for DocumentTypes configuration
  - [x] `ExecuteBatch()` for parallel transformation

- [x] Create `pkg/indexer/commands/index.go`
  - [x] `IndexCommand` struct
  - [x] `IndexType` enum (published, drafts)
  - [x] `Execute()` to index single document
  - [x] `ExecuteBatch()` for bulk indexing
  - [x] `toSearchDocument()` helper

- [x] Create `pkg/indexer/commands/track.go`
  - [x] `TrackCommand` struct
  - [x] `Execute()` to update document tracking
  - [x] `ExecuteBatch()` to update folder tracking
  - [x] Support for UpdateDocumentTime option

### Testing
- [x] Create `pkg/indexer/commands/commands_test.go`
  - [x] `MockDocumentStorage` implementation
  - [x] Test for ExtractContentCommand
  - [x] Test for DiscoverCommand
  - [x] Test for time-based filtering

- [x] Create `pkg/indexer/orchestrator_test.go`
  - [x] Example usage test
  - [x] Full pipeline setup example

## 🔄 Phase 3: Additional Commands (IN PROGRESS)

### Header Refresh Command
- [ ] Create `pkg/indexer/commands/update_header.go`
  - [ ] `UpdateHeaderCommand` struct
  - [ ] `Execute()` to update document headers in workspace
  - [ ] Integration with `pkg/hashicorpdocs` for header replacement
  - [ ] Support for Enabled flag (from config)
  - [ ] `ExecuteBatch()` for parallel header updates

### Migration Command
- [ ] Create `pkg/indexer/commands/migrate.go`
  - [ ] `MigrateCommand` struct with Source and Target providers
  - [ ] `Execute()` to copy document between providers
  - [ ] Support for SkipExisting option
  - [ ] Support for DryRun mode
  - [ ] Support for PreserveMetadata option
  - [ ] `ExecuteBatch()` for parallel migration

### Validation Commands
- [ ] Create `pkg/indexer/commands/validate.go`
  - [ ] `ValidateCommand` struct
  - [ ] Check for required fields
  - [ ] Check for content issues
  - [ ] Add errors to DocumentContext without failing

## 🔄 Phase 4: Plan Configuration (IN PROGRESS)

### Plan Data Structures
- [ ] Create `pkg/indexer/plan.go`
  - [ ] `Plan` struct with Name, Description, Providers, Folders, Pipelines
  - [ ] `FolderConfig` struct
  - [ ] `PipelineConfig` struct
  - [ ] `CommandConfig` struct
  - [ ] `FilterConfig` struct

### Plan Loading
- [ ] Create `pkg/indexer/plan_loader.go`
  - [ ] `LoadPlan()` function to load YAML plans
  - [ ] `ListPlans()` function to discover plans
  - [ ] `BuildOrchestrator()` to create orchestrator from plan
  - [ ] `buildPipeline()` to create pipeline from config
  - [ ] `buildCommand()` to create command from config
  - [ ] `buildFilter()` to create filter from config
  - [ ] Template variable substitution (e.g., `{{ .GoogleWorkspace.DocsFolder }}`)

### Example Plans
- [ ] Create `testing/indexer/plans/local-integration-test.yaml`
  - [ ] Local workspace provider configuration
  - [ ] Meilisearch search provider
  - [ ] Index published and drafts pipelines
  - [ ] Fast intervals for testing (10s)

- [ ] Create `config/indexer/plans/production.yaml`
  - [ ] Google workspace provider configuration
  - [ ] Algolia search provider
  - [ ] Index and refresh headers pipelines
  - [ ] Production intervals (60s)
  - [ ] Skip recently modified filter (30m)

- [ ] Create `config/indexer/plans/migrate-google-to-local.yaml`
  - [ ] Google source, Local target
  - [ ] Migration pipeline with all documents
  - [ ] Re-index in target search provider
  - [ ] One-time execution (run_interval: 0s)

### Plan Validation
- [ ] Create `pkg/indexer/plan_validator.go`
  - [ ] `ValidatePlan()` function
  - [ ] Check for required fields
  - [ ] Check for valid command types
  - [ ] Check for valid provider names
  - [ ] Check for valid filter configurations

## 🔜 Phase 5: CLI Integration

### Update Indexer Command
- [ ] Update `internal/cmd/commands/indexer/indexer.go`
  - [ ] Add `-plan` flag for plan file path
  - [ ] Add `-list-plans` flag to list available plans
  - [ ] Add `-validate` flag to validate plan without running
  - [ ] Add `-dry-run` flag override
  - [ ] Load plan and build orchestrator
  - [ ] Support for legacy indexer fallback
  - [ ] Execute orchestrator based on mode

### Command Registry
- [ ] Create `pkg/indexer/registry.go`
  - [ ] `CommandFactory` function type
  - [ ] `RegisterCommand()` function
  - [ ] `GetCommand()` function
  - [ ] Default command registration

### Help and Documentation
- [ ] Update CLI help text
  - [ ] Document plan file format
  - [ ] Document available command types
  - [ ] Document filter options
  - [ ] Provide usage examples

## 🔜 Phase 6: Local Testing Infrastructure

### Test Data
- [ ] Create `testing/indexer/fixtures.go`
  - [ ] `SetupLocalWorkspace()` helper
  - [ ] `CreateTestDocuments()` helper
  - [ ] `SetupSearchProvider()` helper
  - [ ] `SetupDatabase()` helper
  - [ ] Sample RFC, PRD, FRD documents

- [ ] Create `testing/indexer/test-data/` directory
  - [ ] `testing/indexer/test-data/docs/RFC-001.md`
  - [ ] `testing/indexer/test-data/docs/PRD-002.md`
  - [ ] `testing/indexer/test-data/docs/FRD-003.md`
  - [ ] `testing/indexer/test-data/drafts/DRAFT-001.md`

### Integration Tests
- [ ] Create `testing/indexer/integration_test.go`
  - [ ] Test: Index documents from local workspace
  - [ ] Test: Incremental updates (modify, re-index)
  - [ ] Test: Header refresh
  - [ ] Test: Document migration
  - [ ] Test: Error handling and recovery
  - [ ] Test: Concurrent processing
  - [ ] Test: Filter application
  - [ ] Test: Database tracking verification

### Docker Setup
- [ ] Update `testing/docker-compose.yml`
  - [ ] Add indexer-test service
  - [ ] Mount local workspace data
  - [ ] Configure environment variables
  - [ ] Add health checks

### Makefile Targets
- [ ] Update root `Makefile`
  - [ ] `indexer/test/unit` - Run unit tests
  - [ ] `indexer/test/integration` - Run integration tests with plan
  - [ ] `indexer/plans/list` - List available plans
  - [ ] `indexer/plans/validate` - Validate all plans

## 🔜 Phase 7: Extended Workspace Provider Interface

Some operations may need to be added to workspace provider interface:

- [ ] Review `pkg/workspace/workspace.go`
  - [ ] Check if `GetUpdatedDocsBetween()` equivalent exists
  - [ ] Add batch document operations if needed
  - [ ] Add document metadata update operations

- [ ] Update workspace providers
  - [ ] Google adapter: Implement any missing operations
  - [ ] Local adapter: Implement any missing operations
  - [ ] Mock adapter: Implement any missing operations

## 🔜 Phase 8: Migration & Backward Compatibility

### Feature Flag
- [ ] Add indexer feature flag to config
  - [ ] `use_new_indexer` boolean flag
  - [ ] Default to false initially
  - [ ] Switch logic in CLI command

### Parallel Execution
- [ ] Run both indexers in parallel
  - [ ] Execute legacy indexer
  - [ ] Execute new indexer
  - [ ] Compare results
  - [ ] Log discrepancies

### Metrics & Monitoring
- [ ] Add indexer metrics
  - [ ] Documents processed count
  - [ ] Errors count
  - [ ] Processing duration
  - [ ] Pipeline execution times
  - [ ] Command execution times

### Performance Benchmarks
- [ ] Create benchmark tests
  - [ ] Compare new vs legacy performance
  - [ ] Memory usage comparison
  - [ ] API call efficiency

## 🔜 Phase 9: Production Deployment

### Rollout Plan
- [ ] Week 1: Deploy with feature flag off
  - [ ] Verify no impact
  - [ ] Monitor logs

- [ ] Week 2: Enable for test environment
  - [ ] Run new indexer in test
  - [ ] Validate search results
  - [ ] Monitor errors

- [ ] Week 3: Parallel execution in staging
  - [ ] Run both indexers
  - [ ] Compare results
  - [ ] Fix any discrepancies

- [ ] Week 4: Gradual rollout to production
  - [ ] 10% traffic to new indexer
  - [ ] 50% traffic to new indexer
  - [ ] 100% traffic to new indexer

### Deprecation
- [ ] Mark legacy indexer as deprecated
  - [ ] Add deprecation warnings
  - [ ] Update documentation

- [ ] Remove legacy indexer
  - [ ] Delete `internal/indexer/` (except tracking models)
  - [ ] Remove legacy CLI flags
  - [ ] Update all documentation

## 📚 Documentation Updates

### Implementation Guides
- [x] Create `docs-internal/INDEXER_REFACTOR_PLAN.md`
- [x] Create `docs-internal/INDEXER_IMPLEMENTATION_GUIDE.md`
- [x] Create `docs-internal/INDEXER_PHASE1_COMPLETE.md`
- [x] Create `docs-internal/README-indexer.md`

### Code Documentation
- [x] Create `pkg/indexer/README.md`
- [ ] Add godoc comments to all exported types
- [ ] Add examples in godoc

### User Documentation
- [ ] Update main README.md
  - [ ] Add indexer plan examples
  - [ ] Document CLI usage

- [ ] Create migration guide
  - [ ] How to migrate from legacy indexer
  - [ ] How to create custom commands
  - [ ] How to create custom plans

## 🧪 Testing Strategy

### Unit Tests
- [x] Command tests with mocks
- [ ] Pipeline tests
- [ ] Orchestrator tests
- [ ] Filter tests
- [ ] Plan loader tests
- [ ] Target: >80% coverage

### Integration Tests
- [ ] Local workspace indexing
- [ ] Document migration
- [ ] Error scenarios
- [ ] Concurrent processing
- [ ] Database tracking

### E2E Tests
- [ ] Full indexing cycle with Docker
- [ ] Verify search results
- [ ] Verify database state
- [ ] Verify workspace state

### Performance Tests
- [ ] Benchmark indexing speed
- [ ] Benchmark with different MaxParallel settings
- [ ] Memory usage profiling
- [ ] API call efficiency

## 🐛 Known Issues & TODOs

### Current Limitations
- [ ] Migration command not implemented
- [ ] Update header command not implemented
- [ ] Plan loading not implemented
- [ ] No CLI integration yet
- [ ] No validation command

### Future Enhancements
- [ ] Support for webhooks/notifications on completion
- [ ] Support for scheduling (cron-like)
- [ ] Support for incremental backfill
- [ ] Support for document transformations
- [ ] Support for custom metadata enrichment
- [ ] Metrics dashboard
- [ ] Admin UI for pipeline management

## 🔗 Related Files

### Core Implementation
- `pkg/indexer/command.go`
- `pkg/indexer/context.go`
- `pkg/indexer/pipeline.go`
- `pkg/indexer/orchestrator.go`
- `pkg/indexer/config.go`

### Commands
- `pkg/indexer/commands/discover.go`
- `pkg/indexer/commands/extract.go`
- `pkg/indexer/commands/load_metadata.go`
- `pkg/indexer/commands/transform.go`
- `pkg/indexer/commands/index.go`
- `pkg/indexer/commands/track.go`

### Legacy Code (to be replaced)
- `internal/indexer/indexer.go`
- `internal/indexer/refresh_headers.go`
- `internal/indexer/refresh_docs_headers.go`
- `internal/indexer/refresh_drafts_headers.go`
- `internal/cmd/commands/indexer/indexer.go`

### Related Interfaces
- `pkg/workspace/workspace.go` - Workspace provider interface
- `pkg/search/search.go` - Search provider interface
- `pkg/document/document.go` - Document model
- `pkg/models/document.go` - Database models

## 📊 Progress Tracking

### Overall Progress: ~30% Complete

- ✅ Phase 1: Core Abstractions (100%)
- ✅ Phase 2: Basic Commands (100%)
- 🔄 Phase 3: Additional Commands (0%)
- 🔄 Phase 4: Plan Configuration (0%)
- 🔜 Phase 5: CLI Integration (0%)
- 🔜 Phase 6: Local Testing (0%)
- 🔜 Phase 7: Provider Extensions (0%)
- 🔜 Phase 8: Migration (0%)
- 🔜 Phase 9: Production (0%)

### Estimated Timeline
- Phase 1-2: ✅ Complete (Week 1)
- Phase 3: Week 2
- Phase 4: Week 2-3
- Phase 5: Week 3
- Phase 6: Week 4
- Phase 7: Week 4-5
- Phase 8: Week 5-6
- Phase 9: Week 7-8

Total: ~8 weeks for complete implementation and rollout

## 🎯 Success Criteria

### Phase 1-2 (Complete)
- [x] All command interfaces defined
- [x] All basic commands implemented
- [x] Pipeline execution working
- [x] Orchestrator functional
- [x] Unit tests passing

### Phase 3-6 (In Progress)
- [ ] All commands implemented and tested
- [ ] Plan loading working
- [ ] CLI integration complete
- [ ] Local integration tests passing

### Phase 7-9 (Future)
- [ ] Performance equal or better than legacy
- [ ] All integration tests passing
- [ ] Feature flag deployed
- [ ] Running in production successfully
- [ ] Legacy indexer removed

---

**Last Updated**: October 22, 2025
**Current Status**: Phase 1 & 2 Complete, Phase 3 Starting
**Next Milestone**: Complete Phase 3 commands and Phase 4 plan configuration
