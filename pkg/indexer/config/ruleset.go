package config

import (
	"fmt"
	"os"

	"github.com/hashicorp/hcl/v2/hclsimple"

	"github.com/hashicorp-forge/hermes/pkg/indexer/ruleset"
)

// IndexerConfig represents the indexer configuration from HCL.
type IndexerConfig struct {
	// Rulesets define document processing rules
	Rulesets []RulesetConfig `hcl:"ruleset,block"`

	// LLM configuration for summary generation
	LLM *LLMConfig `hcl:"llm,block"`

	// Embeddings configuration
	Embeddings *EmbeddingsConfig `hcl:"embeddings,block"`

	// Kafka/Redpanda configuration
	Kafka *KafkaConfig `hcl:"kafka,block"`
}

// RulesetConfig represents a single ruleset configuration.
type RulesetConfig struct {
	Name       string            `hcl:"name,label"`
	Conditions map[string]string `hcl:"conditions,optional"`
	Pipeline   []string          `hcl:"pipeline"`
}

// LLMConfig represents LLM provider configuration.
type LLMConfig struct {
	OpenAIAPIKey  string `hcl:"openai_api_key,optional"`
	OllamaURL     string `hcl:"ollama_url,optional"`
	BedrockRegion string `hcl:"bedrock_region,optional"`
	DefaultModel  string `hcl:"default_model,optional"`
}

// EmbeddingsConfig represents embeddings generation configuration.
type EmbeddingsConfig struct {
	Model      string `hcl:"model,optional"`      // e.g., "text-embedding-3-small"
	Dimensions int    `hcl:"dimensions,optional"` // e.g., 1536
	Provider   string `hcl:"provider,optional"`   // e.g., "openai"
	ChunkSize  int    `hcl:"chunk_size,optional"` // e.g., 8000
}

// KafkaConfig represents Kafka/Redpanda configuration.
type KafkaConfig struct {
	Brokers          []string `hcl:"brokers"`
	Topic            string   `hcl:"topic,optional"`
	ConsumerGroup    string   `hcl:"consumer_group,optional"`
	EnableTLS        bool     `hcl:"enable_tls,optional"`
	SASLUsername     string   `hcl:"sasl_username,optional"`
	SASLPassword     string   `hcl:"sasl_password,optional"`
	SASLMechanism    string   `hcl:"sasl_mechanism,optional"`
	SecurityProtocol string   `hcl:"security_protocol,optional"`
}

// LoadRulesetsFromFile loads indexer rulesets from an HCL configuration file.
func LoadRulesetsFromFile(filename string) ([]ruleset.Ruleset, error) {
	if filename == "" {
		return nil, fmt.Errorf("configuration file path is required")
	}

	// Check if file exists
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return nil, fmt.Errorf("configuration file not found: %s", filename)
	}

	var config IndexerConfig
	err := hclsimple.DecodeFile(filename, nil, &config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse configuration file: %w", err)
	}

	// Convert to ruleset.Ruleset format
	rulesets := make([]ruleset.Ruleset, len(config.Rulesets))
	for i, rc := range config.Rulesets {
		rulesets[i] = ruleset.Ruleset{
			Name:       rc.Name,
			Conditions: rc.Conditions,
			Pipeline:   rc.Pipeline,
			Config:     make(map[string]interface{}), // Empty config for now
		}
	}

	return rulesets, nil
}

// LoadIndexerConfig loads the complete indexer configuration from HCL file.
func LoadIndexerConfig(filename string) (*IndexerConfig, error) {
	if filename == "" {
		return nil, fmt.Errorf("configuration file path is required")
	}

	// Check if file exists
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return nil, fmt.Errorf("configuration file not found: %s", filename)
	}

	var config IndexerConfig
	err := hclsimple.DecodeFile(filename, nil, &config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse configuration file: %w", err)
	}

	// Set defaults
	if config.Embeddings != nil {
		if config.Embeddings.Model == "" {
			config.Embeddings.Model = "text-embedding-3-small"
		}
		if config.Embeddings.Dimensions == 0 {
			config.Embeddings.Dimensions = 1536
		}
		if config.Embeddings.Provider == "" {
			config.Embeddings.Provider = "openai"
		}
		if config.Embeddings.ChunkSize == 0 {
			config.Embeddings.ChunkSize = 8000
		}
	}

	if config.LLM != nil && config.LLM.DefaultModel == "" {
		config.LLM.DefaultModel = "gpt-4o-mini"
	}

	return &config, nil
}

// ValidateRulesets validates ruleset configuration.
func ValidateRulesets(rulesets []ruleset.Ruleset) error {
	if len(rulesets) == 0 {
		return fmt.Errorf("at least one ruleset must be defined")
	}

	seen := make(map[string]bool)
	for i, rs := range rulesets {
		// Check for duplicate names
		if seen[rs.Name] {
			return fmt.Errorf("duplicate ruleset name: %s", rs.Name)
		}
		seen[rs.Name] = true

		// Validate pipeline steps
		if len(rs.Pipeline) == 0 {
			return fmt.Errorf("ruleset %d (%s): pipeline cannot be empty", i, rs.Name)
		}

		// Validate pipeline step names
		validSteps := map[string]bool{
			"search_index": true,
			"llm_summary":  true,
			"embeddings":   true,
		}

		for _, step := range rs.Pipeline {
			if !validSteps[step] {
				return fmt.Errorf("ruleset %s: invalid pipeline step '%s' (valid: search_index, llm_summary, embeddings)", rs.Name, step)
			}
		}
	}

	return nil
}

// Example configuration file format:
//
// # Indexer Worker Configuration
//
// # LLM Configuration
// llm {
//   openai_api_key = "sk-..."
//   ollama_url = "http://localhost:11434"
//   bedrock_region = "us-east-1"
//   default_model = "gpt-4o-mini"
// }
//
// # Embeddings Configuration
// embeddings {
//   model = "text-embedding-3-small"
//   dimensions = 1536
//   provider = "openai"
//   chunk_size = 8000
// }
//
// # Kafka/Redpanda Configuration
// kafka {
//   brokers = ["localhost:9092"]
//   topic = "document-revisions"
//   consumer_group = "indexer-worker"
//   enable_tls = false
// }
//
// # Ruleset for published RFCs
// ruleset "published-rfcs" {
//   conditions = {
//     document_type = "RFC"
//     status = "Approved"
//   }
//
//   pipeline = ["search_index", "llm_summary", "embeddings"]
//
//   config = {
//     llm_summary = {
//       model = "gpt-4o-mini"
//       max_tokens = 500
//       style = "executive"
//     }
//     embeddings = {
//       model = "text-embedding-3-small"
//       dimensions = 1536
//       provider = "openai"
//     }
//   }
// }
//
// # Ruleset for all documents (fallback)
// ruleset "all-documents" {
//   conditions = {}
//   pipeline = ["search_index"]
// }
