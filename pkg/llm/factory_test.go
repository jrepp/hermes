package llm

import (
	"context"
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClientFactory_DetectProvider(t *testing.T) {
	factory := NewClientFactory(ClientFactoryConfig{
		Logger: hclog.NewNullLogger(),
	})

	tests := []struct {
		model            string
		expectedProvider string
	}{
		// OpenAI models
		{"gpt-4o", "openai"},
		{"gpt-4o-mini", "openai"},
		{"gpt-4-turbo", "openai"},
		{"gpt-3.5-turbo", "openai"},
		{"o1-preview", "openai"},
		{"o1-mini", "openai"},

		// Bedrock models (Claude)
		{"claude-3-opus", "bedrock"},
		{"claude-3-sonnet", "bedrock"},
		{"us.anthropic.claude-3-7-sonnet-20250219-v1:0", "bedrock"},
		{"anthropic.claude-3-opus-20240229-v1:0", "bedrock"},

		// Bedrock models (Amazon)
		{"amazon.titan-text-express-v1", "bedrock"},
		{"amazon.titan-text-lite-v1", "bedrock"},

		// Ollama models
		{"llama2", "ollama"},
		{"llama3", "ollama"},
		{"llama3:70b", "ollama"},
		{"mistral", "ollama"},
		{"mistral-large", "ollama"},
		{"codellama", "ollama"},
		{"phi", "ollama"},
		{"qwen2", "ollama"},
		{"gemma2", "ollama"},

		// Unknown model (defaults to OpenAI)
		{"unknown-model-xyz", "openai"},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			provider := factory.detectProvider(tt.model)
			assert.Equal(t, tt.expectedProvider, provider,
				"Model %s should be detected as %s", tt.model, tt.expectedProvider)
		})
	}
}

func TestClientFactory_GetOpenAIClient(t *testing.T) {
	t.Run("with API key", func(t *testing.T) {
		factory := NewClientFactory(ClientFactoryConfig{
			OpenAIAPIKey: "test-api-key",
			Logger:       hclog.NewNullLogger(),
		})

		client, err := factory.GetOpenAIClient()
		require.NoError(t, err)
		require.NotNil(t, client)
		assert.Equal(t, "test-api-key", client.apiKey)
	})

	t.Run("without API key", func(t *testing.T) {
		factory := NewClientFactory(ClientFactoryConfig{
			Logger: hclog.NewNullLogger(),
		})

		client, err := factory.GetOpenAIClient()
		require.Error(t, err)
		assert.Nil(t, client)
		assert.Contains(t, err.Error(), "API key not configured")
	})
}

func TestClientFactory_GetOllamaClient(t *testing.T) {
	t.Run("with default URL", func(t *testing.T) {
		factory := NewClientFactory(ClientFactoryConfig{
			Logger: hclog.NewNullLogger(),
		})

		client, err := factory.GetOllamaClient()
		require.NoError(t, err)
		require.NotNil(t, client)
		assert.Equal(t, "http://localhost:11434", client.baseURL)
	})

	t.Run("with custom URL", func(t *testing.T) {
		factory := NewClientFactory(ClientFactoryConfig{
			OllamaURL: "http://custom-ollama:8080",
			Logger:    hclog.NewNullLogger(),
		})

		client, err := factory.GetOllamaClient()
		require.NoError(t, err)
		require.NotNil(t, client)
		assert.Equal(t, "http://custom-ollama:8080", client.baseURL)
	})
}

func TestClientFactory_GetBedrockClient(t *testing.T) {
	// Skip this test in CI environments without AWS credentials
	if testing.Short() {
		t.Skip("Skipping Bedrock test in short mode")
	}

	t.Run("with default region", func(t *testing.T) {
		factory := NewClientFactory(ClientFactoryConfig{
			Logger: hclog.NewNullLogger(),
		})

		ctx := context.Background()
		client, err := factory.GetBedrockClient(ctx)

		// Note: This may fail without proper AWS credentials
		// That's expected in test environments
		if err != nil {
			t.Logf("Expected error without AWS credentials: %v", err)
			return
		}

		require.NotNil(t, client)
	})

	t.Run("with custom region", func(t *testing.T) {
		factory := NewClientFactory(ClientFactoryConfig{
			BedrockRegion: "eu-west-1",
			Logger:        hclog.NewNullLogger(),
		})

		ctx := context.Background()
		client, err := factory.GetBedrockClient(ctx)

		// Note: This may fail without proper AWS credentials
		if err != nil {
			t.Logf("Expected error without AWS credentials: %v", err)
			return
		}

		require.NotNil(t, client)
	})
}

func TestClientFactory_ValidateConfig(t *testing.T) {
	tests := []struct {
		name      string
		config    ClientFactoryConfig
		model     string
		wantError bool
	}{
		{
			name: "OpenAI with API key",
			config: ClientFactoryConfig{
				OpenAIAPIKey: "test-key",
				Logger:       hclog.NewNullLogger(),
			},
			model:     "gpt-4o-mini",
			wantError: false,
		},
		{
			name: "OpenAI without API key",
			config: ClientFactoryConfig{
				Logger: hclog.NewNullLogger(),
			},
			model:     "gpt-4o-mini",
			wantError: true,
		},
		{
			name: "Ollama (no credentials needed)",
			config: ClientFactoryConfig{
				Logger: hclog.NewNullLogger(),
			},
			model:     "llama3",
			wantError: false,
		},
		{
			name: "Bedrock (uses AWS credentials)",
			config: ClientFactoryConfig{
				Logger: hclog.NewNullLogger(),
			},
			model:     "claude-3-opus",
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := NewClientFactory(tt.config)
			err := factory.ValidateConfig(tt.model)

			if tt.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestClientFactory_SupportedModels(t *testing.T) {
	factory := NewClientFactory(ClientFactoryConfig{
		Logger: hclog.NewNullLogger(),
	})

	models := factory.SupportedModels()

	// Verify all three providers are present
	require.Contains(t, models, "openai")
	require.Contains(t, models, "bedrock")
	require.Contains(t, models, "ollama")

	// Verify each has models
	assert.NotEmpty(t, models["openai"])
	assert.NotEmpty(t, models["bedrock"])
	assert.NotEmpty(t, models["ollama"])

	// Verify specific models
	assert.Contains(t, models["openai"], "gpt-4o-mini")
	assert.Contains(t, models["bedrock"], "us.anthropic.claude-3-7-sonnet-20250219-v1:0")
	assert.Contains(t, models["ollama"], "llama3")
}

func TestClientFactory_GetClient(t *testing.T) {
	factory := NewClientFactory(ClientFactoryConfig{
		OpenAIAPIKey: "test-api-key",
		Logger:       hclog.NewNullLogger(),
	})

	ctx := context.Background()

	t.Run("OpenAI model", func(t *testing.T) {
		client, err := factory.GetClient(ctx, "gpt-4o-mini")
		require.NoError(t, err)
		require.NotNil(t, client)

		// Verify it's an OpenAI client
		_, ok := client.(*OpenAIClient)
		assert.True(t, ok, "Expected OpenAIClient")
	})

	t.Run("Ollama model", func(t *testing.T) {
		client, err := factory.GetClient(ctx, "llama3")
		require.NoError(t, err)
		require.NotNil(t, client)

		// Verify it's an Ollama client
		_, ok := client.(*OllamaClient)
		assert.True(t, ok, "Expected OllamaClient")
	})

	t.Run("Bedrock model", func(t *testing.T) {
		// Skip if not in a proper AWS environment
		client, err := factory.GetClient(ctx, "claude-3-opus")

		// We expect this to potentially fail without AWS credentials
		if err != nil {
			t.Logf("Expected error without AWS credentials: %v", err)
			return
		}

		require.NotNil(t, client)

		// Verify it's a Bedrock client
		_, ok := client.(*BedrockClient)
		assert.True(t, ok, "Expected BedrockClient")
	})

	t.Run("Unknown model", func(t *testing.T) {
		// Unknown models default to OpenAI
		client, err := factory.GetClient(ctx, "unknown-model")
		require.NoError(t, err)
		require.NotNil(t, client)

		// Should default to OpenAI
		_, ok := client.(*OpenAIClient)
		assert.True(t, ok, "Expected OpenAIClient as default")
	})
}

func TestClientFactory_CaseInsensitiveDetection(t *testing.T) {
	factory := NewClientFactory(ClientFactoryConfig{
		Logger: hclog.NewNullLogger(),
	})

	tests := []struct {
		model            string
		expectedProvider string
	}{
		{"GPT-4O", "openai"},
		{"Gpt-4o-Mini", "openai"},
		{"LLAMA3", "ollama"},
		{"Mistral", "ollama"},
		{"CLAUDE-3-OPUS", "bedrock"},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			provider := factory.detectProvider(tt.model)
			assert.Equal(t, tt.expectedProvider, provider)
		})
	}
}
