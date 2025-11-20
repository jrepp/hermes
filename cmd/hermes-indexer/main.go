package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/hcl/v2/hclsimple"

	"github.com/hashicorp-forge/hermes/internal/config"
	"github.com/hashicorp-forge/hermes/pkg/indexer/consumer"
	"github.com/hashicorp-forge/hermes/pkg/indexer/pipeline"
	"github.com/hashicorp-forge/hermes/pkg/indexer/pipeline/steps"
	"github.com/hashicorp-forge/hermes/pkg/indexer/ruleset"
	"github.com/hashicorp-forge/hermes/pkg/kafka"
	"github.com/hashicorp-forge/hermes/pkg/search"
	algoliaadapter "github.com/hashicorp-forge/hermes/pkg/search/adapters/algolia"
	bleveadapter "github.com/hashicorp-forge/hermes/pkg/search/adapters/bleve"
	meilisearchadapter "github.com/hashicorp-forge/hermes/pkg/search/adapters/meilisearch"
)

func main() {
	// Parse command-line flags
	configPath := flag.String("config", "config.hcl", "Path to configuration file")
	flag.Parse()

	// Create logger
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "hermes-indexer",
		Level: hclog.Info,
	})

	logger.Info("starting hermes-indexer", "config", *configPath)

	// Load configuration
	cfg, err := loadConfig(*configPath)
	if err != nil {
		logger.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle signals for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		logger.Info("received signal, shutting down", "signal", sig)
		cancel()
	}()

	// Run consumer mode
	if err := runConsumer(ctx, cfg, logger); err != nil {
		logger.Error("consumer failed", "error", err)
		cancel() // Ensure context is canceled before exit
		os.Exit(1)
	}

	logger.Info("hermes-indexer stopped gracefully")
}

// runConsumer runs the indexer consumer (database-independent).
func runConsumer(ctx context.Context, cfg *config.Config, logger hclog.Logger) error {
	logger.Info("starting indexer consumer")

	// Initialize search provider
	searchProvider, err := initializeSearchProvider(cfg, logger)
	if err != nil {
		return fmt.Errorf("failed to initialize search provider: %w", err)
	}

	// Create pipeline steps
	pipelineSteps := []pipeline.Step{
		steps.NewSearchIndexStep(searchProvider, logger),
		// Add more steps as they're implemented:
		// steps.NewLLMSummaryStep(hermesAPIClient, llmClient, logger),
		// steps.NewEmbeddingsStep(hermesAPIClient, embeddingClient, logger),
	}

	// Create pipeline executor (no database - stateless)
	executor, err := pipeline.NewExecutor(pipeline.ExecutorConfig{
		DB:     nil, // No database - indexer is stateless
		Steps:  pipelineSteps,
		Logger: logger,
	})
	if err != nil {
		return fmt.Errorf("failed to create pipeline executor: %w", err)
	}

	// Get Redpanda configuration
	brokers := kafka.GetBrokers(cfg)
	topic := kafka.GetDocumentRevisionTopic(cfg)
	consumerGroup := kafka.GetConsumerGroup(cfg)

	// Convert config rulesets to indexer rulesets
	rulesets := convertRulesets(cfg.Indexer.Rulesets)

	// Create consumer (no database - gets all data from event payload)
	indexerConsumer, err := consumer.New(consumer.Config{
		DB:            nil, // No database - indexer is stateless
		Brokers:       brokers,
		Topic:         topic,
		ConsumerGroup: consumerGroup,
		Rulesets:      rulesets,
		Executor:      executor,
		Logger:        logger,
	})
	if err != nil {
		return fmt.Errorf("failed to create consumer: %w", err)
	}

	// Start consumer
	return indexerConsumer.Start(ctx)
}

// convertRulesets converts config rulesets to indexer rulesets.
func convertRulesets(cfgRulesets []config.IndexerRuleset) []ruleset.Ruleset {
	rulesets := make([]ruleset.Ruleset, len(cfgRulesets))
	for i, cfgRs := range cfgRulesets {
		rulesets[i] = ruleset.Ruleset{
			Name:       cfgRs.Name,
			Conditions: cfgRs.Conditions,
			Pipeline:   cfgRs.Pipeline,
			Config:     cfgRs.Config,
		}
	}
	return rulesets
}

// loadConfig loads the configuration from an HCL file.
func loadConfig(path string) (*config.Config, error) {
	var cfg config.Config
	err := hclsimple.DecodeFile(path, nil, &cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	return &cfg, nil
}

// initializeSearchProvider creates the search provider based on config.
func initializeSearchProvider(cfg *config.Config, logger hclog.Logger) (search.Provider, error) {
	if cfg.Providers == nil {
		return nil, fmt.Errorf("providers configuration is missing")
	}

	providerName := cfg.Providers.Search

	switch providerName {
	case "algolia":
		if cfg.Algolia == nil {
			return nil, fmt.Errorf("algolia configuration is missing")
		}

		searchAdapterCfg := &algoliaadapter.Config{
			AppID:           cfg.Algolia.AppID,
			WriteAPIKey:     cfg.Algolia.WriteAPIKey,
			DocsIndexName:   cfg.Algolia.DocsIndexName,
			DraftsIndexName: cfg.Algolia.DraftsIndexName,
		}

		provider, err := algoliaadapter.NewAdapter(searchAdapterCfg)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize algolia adapter: %w", err)
		}

		logger.Info("initialized search provider", "provider", "algolia")
		return provider, nil

	case "meilisearch":
		if cfg.Meilisearch == nil {
			return nil, fmt.Errorf("meilisearch configuration is missing")
		}

		meilisearchCfg := cfg.Meilisearch.ToMeilisearchAdapterConfig()
		provider, err := meilisearchadapter.NewAdapter(meilisearchCfg)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize meilisearch adapter: %w", err)
		}

		logger.Info("initialized search provider", "provider", "meilisearch")
		return provider, nil

	case "bleve":
		if cfg.Bleve == nil {
			return nil, fmt.Errorf("bleve configuration is missing")
		}

		bleveCfg := &bleveadapter.Config{
			IndexPath: cfg.Bleve.IndexPath,
		}

		provider, err := bleveadapter.NewAdapter(bleveCfg)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize bleve adapter: %w", err)
		}

		logger.Info("initialized search provider", "provider", "bleve")
		return provider, nil

	default:
		return nil, fmt.Errorf("unsupported search provider: %s (supported: algolia, meilisearch, bleve)", providerName)
	}
}
