package indexer

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/hashicorp/go-hclog"
	"gorm.io/gorm"

	"github.com/hashicorp-forge/hermes/pkg/models"
	"github.com/hashicorp-forge/hermes/pkg/search"
	"github.com/hashicorp-forge/hermes/pkg/workspace"
)

// Orchestrator manages the execution of indexer pipelines.
// It coordinates between workspace providers, search providers,
// and the database to keep everything in sync.
type Orchestrator struct {
	db                *gorm.DB
	logger            hclog.Logger
	workspaceProvider workspace.StorageProvider
	searchProvider    search.Provider
	pipelines         map[string]*Pipeline
	maxParallelDocs   int
	dryRun            bool
}

// Option is a functional option for creating an Orchestrator.
type Option func(*Orchestrator)

// WithDatabase sets the database connection.
func WithDatabase(db *gorm.DB) Option {
	return func(o *Orchestrator) {
		o.db = db
	}
}

// WithLogger sets the logger.
func WithLogger(logger hclog.Logger) Option {
	return func(o *Orchestrator) {
		o.logger = logger
	}
}

// WithWorkspaceProvider sets the workspace provider.
func WithWorkspaceProvider(provider workspace.StorageProvider) Option {
	return func(o *Orchestrator) {
		o.workspaceProvider = provider
	}
}

// WithSearchProvider sets the search provider.
func WithSearchProvider(provider search.Provider) Option {
	return func(o *Orchestrator) {
		o.searchProvider = provider
	}
}

// WithMaxParallelDocs sets the maximum parallel documents.
func WithMaxParallelDocs(max int) Option {
	return func(o *Orchestrator) {
		o.maxParallelDocs = max
	}
}

// WithDryRun enables or disables dry-run mode.
func WithDryRun(dryRun bool) Option {
	return func(o *Orchestrator) {
		o.dryRun = dryRun
	}
}

// NewOrchestrator creates a new indexer orchestrator.
func NewOrchestrator(opts ...Option) (*Orchestrator, error) {
	o := &Orchestrator{
		pipelines:       make(map[string]*Pipeline),
		maxParallelDocs: 5, // Default
		logger: hclog.New(&hclog.LoggerOptions{
			Name: "indexer",
		}),
	}

	// Apply options
	for _, opt := range opts {
		opt(o)
	}

	// Validate required fields
	if o.db == nil {
		return nil, fmt.Errorf("database is required")
	}
	if o.workspaceProvider == nil {
		return nil, fmt.Errorf("workspace provider is required")
	}
	if o.searchProvider == nil {
		return nil, fmt.Errorf("search provider is required")
	}

	return o, nil
}

// RegisterPipeline adds a pipeline to the orchestrator.
func (o *Orchestrator) RegisterPipeline(name string, pipeline *Pipeline) {
	if pipeline.Logger == nil {
		pipeline.Logger = o.logger.Named("pipeline").Named(name)
	}
	if pipeline.MaxParallel == 0 {
		pipeline.MaxParallel = o.maxParallelDocs
	}
	o.pipelines[name] = pipeline
}

// ExecutePipeline runs a specific pipeline by name.
func (o *Orchestrator) ExecutePipeline(ctx context.Context, name string) error {
	pipeline, ok := o.pipelines[name]
	if !ok {
		return fmt.Errorf("pipeline %s not found", name)
	}

	o.logger.Info("executing pipeline", "name", name)

	// Pipeline will discover its own documents if it has a DiscoverCommand
	// Otherwise, pass empty slice
	if err := pipeline.Execute(ctx, nil); err != nil {
		o.logger.Error("pipeline failed", "name", name, "error", err)
		return err
	}

	return nil
}

// Run executes all registered pipelines continuously.
// It runs in a loop with a configurable interval.
func (o *Orchestrator) Run(ctx context.Context, interval time.Duration) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Run once immediately
	if err := o.runCycle(ctx); err != nil {
		o.logger.Error("initial indexer cycle failed", "error", err)
		// Continue anyway
	}

	for {
		select {
		case <-ctx.Done():
			o.logger.Info("indexer stopped")
			return ctx.Err()
		case <-ticker.C:
			if err := o.runCycle(ctx); err != nil {
				o.logger.Error("indexer cycle failed", "error", err)
				// Continue to next cycle
			}
		}
	}
}

// RunOnce executes all pipelines once and returns.
func (o *Orchestrator) RunOnce(ctx context.Context) error {
	return o.runCycle(ctx)
}

// runCycle executes one complete indexing cycle.
func (o *Orchestrator) runCycle(ctx context.Context) error {
	o.logger.Info("starting indexer cycle", "pipelines", len(o.pipelines))

	startTime := time.Now()

	// Get indexer metadata
	md := models.IndexerMetadata{}
	if err := md.Get(o.db); err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("failed to get indexer metadata: %w", err)
		}
		// First run, set last full index to epoch
		md.LastFullIndexAt = time.Unix(0, 0).UTC()
	}

	// Execute each registered pipeline
	for name := range o.pipelines {
		if err := o.ExecutePipeline(ctx, name); err != nil {
			o.logger.Error("pipeline failed", "name", name, "error", err)
			// Continue with other pipelines
		}
	}

	// Update last full index time
	md.LastFullIndexAt = startTime.UTC()
	if err := md.Upsert(o.db); err != nil {
		o.logger.Error("failed to update indexer metadata", "error", err)
	}

	duration := time.Since(startTime)
	o.logger.Info("indexer cycle completed", "duration", duration)

	return nil
}

// ListPipelines returns the names of all registered pipelines.
func (o *Orchestrator) ListPipelines() []string {
	names := make([]string, 0, len(o.pipelines))
	for name := range o.pipelines {
		names = append(names, name)
	}
	return names
}
