package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp-forge/hermes/pkg/indexer/ruleset"
)

func TestLoadRulesetsFromFile(t *testing.T) {
	t.Run("valid configuration", func(t *testing.T) {
		// Create temporary config file
		configContent := `
# Test indexer configuration

ruleset "published-rfcs" {
  conditions = {
    document_type = "RFC"
    status = "Approved"
  }
  pipeline = ["search_index", "llm_summary", "embeddings"]
}

ruleset "all-documents" {
  conditions = {}
  pipeline = ["search_index"]
}
`
		tmpfile := createTempFile(t, "indexer-*.hcl", configContent)
		defer os.Remove(tmpfile)

		rulesets, err := LoadRulesetsFromFile(tmpfile)
		require.NoError(t, err)
		require.Len(t, rulesets, 2)

		// Check first ruleset
		assert.Equal(t, "published-rfcs", rulesets[0].Name)
		assert.Equal(t, "RFC", rulesets[0].Conditions["document_type"])
		assert.Equal(t, "Approved", rulesets[0].Conditions["status"])
		assert.Equal(t, []string{"search_index", "llm_summary", "embeddings"}, rulesets[0].Pipeline)

		// Check second ruleset
		assert.Equal(t, "all-documents", rulesets[1].Name)
		assert.Empty(t, rulesets[1].Conditions)
		assert.Equal(t, []string{"search_index"}, rulesets[1].Pipeline)
	})

	t.Run("file not found", func(t *testing.T) {
		_, err := LoadRulesetsFromFile("/nonexistent/config.hcl")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "configuration file not found")
	})

	t.Run("empty filename", func(t *testing.T) {
		_, err := LoadRulesetsFromFile("")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "configuration file path is required")
	})

	t.Run("invalid HCL syntax", func(t *testing.T) {
		configContent := `
ruleset "invalid" {
  this is not valid HCL
}
`
		tmpfile := createTempFile(t, "invalid-*.hcl", configContent)
		defer os.Remove(tmpfile)

		_, err := LoadRulesetsFromFile(tmpfile)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse")
	})
}

func TestLoadIndexerConfig(t *testing.T) {
	t.Run("complete configuration", func(t *testing.T) {
		configContent := `
llm {
  openai_api_key = "sk-test-123"
  ollama_url = "http://localhost:11434"
  default_model = "gpt-4o-mini"
}

embeddings {
  model = "text-embedding-3-small"
  dimensions = 1536
  provider = "openai"
  chunk_size = 8000
}

kafka {
  brokers = ["localhost:9092", "localhost:9093"]
  topic = "document-revisions"
  consumer_group = "indexer-worker"
  enable_tls = true
  sasl_username = "user"
  sasl_password = "pass"
  sasl_mechanism = "PLAIN"
}

ruleset "test-ruleset" {
  conditions = {
    document_type = "RFC"
  }
  pipeline = ["search_index", "llm_summary"]
}
`
		tmpfile := createTempFile(t, "indexer-full-*.hcl", configContent)
		defer os.Remove(tmpfile)

		config, err := LoadIndexerConfig(tmpfile)
		require.NoError(t, err)
		require.NotNil(t, config)

		// Check LLM config
		require.NotNil(t, config.LLM)
		assert.Equal(t, "sk-test-123", config.LLM.OpenAIAPIKey)
		assert.Equal(t, "http://localhost:11434", config.LLM.OllamaURL)
		assert.Equal(t, "gpt-4o-mini", config.LLM.DefaultModel)

		// Check embeddings config
		require.NotNil(t, config.Embeddings)
		assert.Equal(t, "text-embedding-3-small", config.Embeddings.Model)
		assert.Equal(t, 1536, config.Embeddings.Dimensions)
		assert.Equal(t, "openai", config.Embeddings.Provider)
		assert.Equal(t, 8000, config.Embeddings.ChunkSize)

		// Check Kafka config
		require.NotNil(t, config.Kafka)
		assert.Equal(t, []string{"localhost:9092", "localhost:9093"}, config.Kafka.Brokers)
		assert.Equal(t, "document-revisions", config.Kafka.Topic)
		assert.Equal(t, "indexer-worker", config.Kafka.ConsumerGroup)
		assert.True(t, config.Kafka.EnableTLS)
		assert.Equal(t, "user", config.Kafka.SASLUsername)

		// Check rulesets
		require.Len(t, config.Rulesets, 1)
		assert.Equal(t, "test-ruleset", config.Rulesets[0].Name)
	})

	t.Run("minimal configuration with defaults", func(t *testing.T) {
		configContent := `
llm {
  openai_api_key = "sk-test"
}

embeddings {}

ruleset "minimal" {
  conditions = {}
  pipeline = ["search_index"]
}
`
		tmpfile := createTempFile(t, "indexer-minimal-*.hcl", configContent)
		defer os.Remove(tmpfile)

		config, err := LoadIndexerConfig(tmpfile)
		require.NoError(t, err)

		// Check defaults were applied
		assert.Equal(t, "gpt-4o-mini", config.LLM.DefaultModel)
		assert.Equal(t, "text-embedding-3-small", config.Embeddings.Model)
		assert.Equal(t, 1536, config.Embeddings.Dimensions)
		assert.Equal(t, "openai", config.Embeddings.Provider)
		assert.Equal(t, 8000, config.Embeddings.ChunkSize)
	})
}

func TestValidateRulesets(t *testing.T) {
	t.Run("valid rulesets", func(t *testing.T) {
		rulesets := []ruleset.Ruleset{
			{
				Name:       "published-rfcs",
				Conditions: map[string]string{"document_type": "RFC"},
				Pipeline:   []string{"search_index", "llm_summary"},
			},
			{
				Name:       "all-documents",
				Conditions: map[string]string{},
				Pipeline:   []string{"search_index"},
			},
		}

		err := ValidateRulesets(rulesets)
		assert.NoError(t, err)
	})

	t.Run("empty rulesets", func(t *testing.T) {
		err := ValidateRulesets([]ruleset.Ruleset{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "at least one ruleset must be defined")
	})

	t.Run("duplicate names", func(t *testing.T) {
		rulesets := []ruleset.Ruleset{
			{
				Name:     "test",
				Pipeline: []string{"search_index"},
			},
			{
				Name:     "test",
				Pipeline: []string{"llm_summary"},
			},
		}

		err := ValidateRulesets(rulesets)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "duplicate ruleset name")
	})

	t.Run("empty pipeline", func(t *testing.T) {
		rulesets := []ruleset.Ruleset{
			{
				Name:     "empty-pipeline",
				Pipeline: []string{},
			},
		}

		err := ValidateRulesets(rulesets)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "pipeline cannot be empty")
	})

	t.Run("invalid pipeline step", func(t *testing.T) {
		rulesets := []ruleset.Ruleset{
			{
				Name:     "invalid-step",
				Pipeline: []string{"search_index", "invalid_step"},
			},
		}

		err := ValidateRulesets(rulesets)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid pipeline step 'invalid_step'")
	})
}

func TestRulesetConfigConversion(t *testing.T) {
	t.Run("converts to ruleset.Ruleset correctly", func(t *testing.T) {
		configContent := `
ruleset "test-conversion" {
  conditions = {
    document_type = "RFC"
    status = "Approved"
  }

  pipeline = ["search_index", "llm_summary", "embeddings"]
}
`
		tmpfile := createTempFile(t, "conversion-*.hcl", configContent)
		defer os.Remove(tmpfile)

		rulesets, err := LoadRulesetsFromFile(tmpfile)
		require.NoError(t, err)
		require.Len(t, rulesets, 1)

		rs := rulesets[0]
		assert.Equal(t, "test-conversion", rs.Name)
		assert.Equal(t, "RFC", rs.Conditions["document_type"])
		assert.Equal(t, "Approved", rs.Conditions["status"])
		assert.Equal(t, 3, len(rs.Pipeline))
	})
}

// Helper function to create temporary config files for testing
func createTempFile(t *testing.T, pattern, content string) string {
	tmpfile, err := os.CreateTemp("", pattern)
	require.NoError(t, err)

	_, err = tmpfile.WriteString(content)
	require.NoError(t, err)

	err = tmpfile.Close()
	require.NoError(t, err)

	return tmpfile.Name()
}

func TestEmbeddingsConfigDefaults(t *testing.T) {
	t.Run("applies default values", func(t *testing.T) {
		configContent := `
embeddings {}

ruleset "test" {
  conditions = {}
  pipeline = ["search_index"]
}
`
		tmpfile := createTempFile(t, "defaults-*.hcl", configContent)
		defer os.Remove(tmpfile)

		config, err := LoadIndexerConfig(tmpfile)
		require.NoError(t, err)

		assert.Equal(t, "text-embedding-3-small", config.Embeddings.Model)
		assert.Equal(t, 1536, config.Embeddings.Dimensions)
		assert.Equal(t, "openai", config.Embeddings.Provider)
		assert.Equal(t, 8000, config.Embeddings.ChunkSize)
	})

	t.Run("preserves custom values", func(t *testing.T) {
		configContent := `
embeddings {
  model = "text-embedding-3-large"
  dimensions = 3072
  provider = "openai"
  chunk_size = 10000
}

ruleset "test" {
  conditions = {}
  pipeline = ["search_index"]
}
`
		tmpfile := createTempFile(t, "custom-*.hcl", configContent)
		defer os.Remove(tmpfile)

		config, err := LoadIndexerConfig(tmpfile)
		require.NoError(t, err)

		assert.Equal(t, "text-embedding-3-large", config.Embeddings.Model)
		assert.Equal(t, 3072, config.Embeddings.Dimensions)
		assert.Equal(t, "openai", config.Embeddings.Provider)
		assert.Equal(t, 10000, config.Embeddings.ChunkSize)
	})
}

func TestKafkaConfiguration(t *testing.T) {
	t.Run("loads Kafka config correctly", func(t *testing.T) {
		configContent := `
kafka {
  brokers = ["broker1:9092", "broker2:9092", "broker3:9092"]
  topic = "test-topic"
  consumer_group = "test-group"
  enable_tls = true
  sasl_username = "testuser"
  sasl_password = "testpass"
  sasl_mechanism = "SCRAM-SHA-256"
  security_protocol = "SASL_SSL"
}

ruleset "test" {
  conditions = {}
  pipeline = ["search_index"]
}
`
		tmpfile := createTempFile(t, "kafka-*.hcl", configContent)
		defer os.Remove(tmpfile)

		config, err := LoadIndexerConfig(tmpfile)
		require.NoError(t, err)
		require.NotNil(t, config.Kafka)

		assert.Equal(t, 3, len(config.Kafka.Brokers))
		assert.Equal(t, "broker1:9092", config.Kafka.Brokers[0])
		assert.Equal(t, "test-topic", config.Kafka.Topic)
		assert.Equal(t, "test-group", config.Kafka.ConsumerGroup)
		assert.True(t, config.Kafka.EnableTLS)
		assert.Equal(t, "testuser", config.Kafka.SASLUsername)
		assert.Equal(t, "testpass", config.Kafka.SASLPassword)
		assert.Equal(t, "SCRAM-SHA-256", config.Kafka.SASLMechanism)
		assert.Equal(t, "SASL_SSL", config.Kafka.SecurityProtocol)
	})
}

func TestComplexRulesetConfig(t *testing.T) {
	t.Run("handles complex ruleset", func(t *testing.T) {
		configContent := `
ruleset "complex" {
  conditions = {
    document_type = "RFC"
    status = "Approved"
    priority = "High"
  }

  pipeline = ["search_index", "llm_summary", "embeddings"]
}
`
		tmpfile := createTempFile(t, "complex-*.hcl", configContent)
		defer os.Remove(tmpfile)

		rulesets, err := LoadRulesetsFromFile(tmpfile)
		require.NoError(t, err)
		require.Len(t, rulesets, 1)

		rs := rulesets[0]
		assert.Equal(t, "complex", rs.Name)
		assert.Equal(t, 3, len(rs.Conditions))
		assert.Equal(t, "RFC", rs.Conditions["document_type"])
		assert.Equal(t, "Approved", rs.Conditions["status"])
		assert.Equal(t, "High", rs.Conditions["priority"])
		assert.Equal(t, 3, len(rs.Pipeline))
	})
}
