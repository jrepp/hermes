package llm

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/go-hclog"
)

// ClientFactory creates LLM clients based on provider name or model.
type ClientFactory struct {
	logger        hclog.Logger
	openaiAPIKey  string
	ollamaURL     string
	bedrockRegion string
}

// ClientFactoryConfig holds configuration for the client factory.
type ClientFactoryConfig struct {
	Logger        hclog.Logger
	OpenAIAPIKey  string
	OllamaURL     string
	BedrockRegion string
}

// NewClientFactory creates a new LLM client factory.
func NewClientFactory(config ClientFactoryConfig) *ClientFactory {
	if config.Logger == nil {
		config.Logger = hclog.NewNullLogger()
	}

	return &ClientFactory{
		openaiAPIKey:  config.OpenAIAPIKey,
		ollamaURL:     config.OllamaURL,
		bedrockRegion: config.BedrockRegion,
		logger:        config.Logger.Named("llm-factory"),
	}
}

// GetClient returns an LLM client based on the model name.
// Automatically detects provider from model name:
// - "gpt-*" → OpenAI
// - "claude-*" → AWS Bedrock
// - "llama*", "mistral", "codellama", "phi" → Ollama
func (f *ClientFactory) GetClient(ctx context.Context, model string) (interface{}, error) {
	provider := f.detectProvider(model)

	f.logger.Debug("selecting LLM client",
		"model", model,
		"provider", provider,
	)

	switch provider {
	case providerOpenAI:
		return f.GetOpenAIClient()
	case providerBedrock:
		return f.GetBedrockClient(ctx)
	case providerOllama:
		return f.GetOllamaClient()
	default:
		return nil, fmt.Errorf("unsupported model: %s (unknown provider)", model)
	}
}

// GetOpenAIClient creates an OpenAI client.
func (f *ClientFactory) GetOpenAIClient() (*OpenAIClient, error) {
	if f.openaiAPIKey == "" {
		return nil, fmt.Errorf("OpenAI API key not configured")
	}

	return NewOpenAIClient(OpenAIConfig{
		APIKey: f.openaiAPIKey,
		Logger: f.logger,
	})
}

// GetOllamaClient creates an Ollama client.
func (f *ClientFactory) GetOllamaClient() (*OllamaClient, error) {
	config := OllamaConfig{
		Logger: f.logger,
	}

	if f.ollamaURL != "" {
		config.BaseURL = f.ollamaURL
	}

	return NewOllamaClient(config)
}

// GetBedrockClient creates an AWS Bedrock client.
func (f *ClientFactory) GetBedrockClient(ctx context.Context) (*BedrockClient, error) {
	config := BedrockConfig{
		Logger: f.logger,
	}

	if f.bedrockRegion != "" {
		config.Region = f.bedrockRegion
	}

	return NewBedrockClient(ctx, config)
}

// detectProvider detects the LLM provider from the model name.
func (f *ClientFactory) detectProvider(model string) string {
	modelLower := strings.ToLower(model)

	for _, prefix := range []string{"gpt-", "o1-", "o3-"} {
		if strings.HasPrefix(modelLower, prefix) {
			return providerOpenAI
		}
	}

	for _, needle := range []string{"claude", "titan", "us.", "anthropic."} {
		if strings.Contains(modelLower, needle) {
			return providerBedrock
		}
	}

	for _, prefix := range []string{"llama", "mistral", "codellama", "phi", "qwen", "gemma", "embeddinggemma", "nomic-embed", "mxbai-embed"} {
		if strings.HasPrefix(modelLower, prefix) {
			return providerOllama
		}
	}
	if strings.Contains(modelLower, "embeddinggemma") {
		return providerOllama
	}

	// Default to OpenAI for unknown models
	f.logger.Warn("unknown model, defaulting to OpenAI",
		"model", model,
	)
	return providerOpenAI
}

// SupportedModels returns a list of example supported models.
func (f *ClientFactory) SupportedModels() map[string][]string {
	return map[string][]string{
		"openai": {
			"gpt-4o",
			"gpt-4o-mini",
			"gpt-4-turbo",
			"gpt-3.5-turbo",
			"o1-preview",
			"o1-mini",
		},
		"bedrock": {
			"us.anthropic.claude-3-7-sonnet-20250219-v1:0",
			"us.anthropic.claude-3-5-sonnet-20241022-v2:0",
			"anthropic.claude-3-opus-20240229-v1:0",
			"anthropic.claude-3-sonnet-20240229-v1:0",
			"anthropic.claude-3-haiku-20240307-v1:0",
			"amazon.titan-text-express-v1",
			"amazon.titan-text-lite-v1",
		},
		"ollama": {
			"llama2",
			"llama3",
			"llama3:70b",
			"mistral",
			"mistral-large",
			"codellama",
			"phi",
			"qwen2",
			"gemma2",
			"nomic-embed-text",
			"mxbai-embed-large",
			"embeddinggemma-300m",
			"unsloth/embeddinggemma-300m",
			"hf.co/unsloth/embeddinggemma-300m-GGUF:Q4_0",
		},
	}
}

// ValidateConfig checks if the factory is properly configured for a given model.
func (f *ClientFactory) ValidateConfig(model string) error {
	provider := f.detectProvider(model)

	switch provider {
	case "openai":
		if f.openaiAPIKey == "" {
			return fmt.Errorf("OpenAI API key required for model %s", model)
		}
	case "bedrock":
		// Bedrock uses AWS credentials from environment/IAM
		// No explicit validation needed here
	case "ollama":
		// Ollama uses local server, no credentials needed
		// Could ping the server to validate connectivity
	}

	return nil
}
