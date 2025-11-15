package kafka

import (
	"os"

	"github.com/hashicorp-forge/hermes/internal/config"
)

// GetBrokers returns the Kafka/Redpanda broker addresses.
// It checks environment variables first, then falls back to config, then default.
func GetBrokers(cfg *config.Config) []string {
	// Try environment variable first
	if brokers := os.Getenv("REDPANDA_BROKERS"); brokers != "" {
		return []string{brokers}
	}

	// Fall back to config
	if cfg.Indexer != nil && len(cfg.Indexer.RedpandaBrokers) > 0 {
		return cfg.Indexer.RedpandaBrokers
	}

	// Default
	return []string{"localhost:19092"}
}

// GetDocumentRevisionTopic returns the document revision topic name.
// It checks environment variables first, then falls back to config, then default.
func GetDocumentRevisionTopic(cfg *config.Config) string {
	// Try environment variable first
	if topic := os.Getenv("DOCUMENT_REVISION_TOPIC"); topic != "" {
		return topic
	}

	// Fall back to config
	if cfg.Indexer != nil && cfg.Indexer.Topic != "" {
		return cfg.Indexer.Topic
	}

	// Default
	return "hermes.document-revisions"
}

// GetConsumerGroup returns the consumer group name for indexer workers.
// It checks environment variables first, then falls back to config, then default.
func GetConsumerGroup(cfg *config.Config) string {
	// Try environment variable first
	if group := os.Getenv("CONSUMER_GROUP"); group != "" {
		return group
	}

	// Fall back to config
	if cfg.Indexer != nil && cfg.Indexer.ConsumerGroup != "" {
		return cfg.Indexer.ConsumerGroup
	}

	// Default
	return "hermes-indexer-workers"
}
