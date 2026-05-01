package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp-forge/hermes/pkg/indexer/pipeline/steps"
)

// OllamaClient implements the LLMClient interface for Ollama's local API.
type OllamaClient struct {
	logger     hclog.Logger
	httpClient *http.Client
	baseURL    string
}

// OllamaConfig holds configuration for the Ollama client.
type OllamaConfig struct {
	Logger  hclog.Logger
	BaseURL string
	Timeout time.Duration
}

// NewOllamaClient creates a new Ollama client.
func NewOllamaClient(config OllamaConfig) (*OllamaClient, error) {
	if config.BaseURL == "" {
		config.BaseURL = "http://localhost:11434"
	}

	if config.Timeout == 0 {
		config.Timeout = 300 * time.Second // Local LLM can be slower
	}

	if config.Logger == nil {
		config.Logger = hclog.NewNullLogger()
	}

	return &OllamaClient{
		baseURL: config.BaseURL,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
		logger: config.Logger.Named("ollama-client"),
	}, nil
}

// GenerateSummary generates a summary using Ollama's local API.
func (c *OllamaClient) GenerateSummary(ctx context.Context, content string, options steps.SummaryOptions) (*steps.Summary, error) {
	startTime := time.Now()

	// Build the prompt
	prompt := c.buildPrompt(content, options)

	// Prepare the request - use chat format for consistency with OpenAI
	reqBody := OllamaChatRequest{
		Model: options.Model,
		Messages: []OllamaChatMessage{
			{
				Role:    "system",
				Content: c.getSystemPrompt(options),
			},
			{
				Role:    "user",
				Content: prompt,
			},
		},
		Stream: false,
		Options: &OllamaOptions{
			Temperature: 0.3, // Lower temperature for more consistent summaries
			NumPredict:  options.MaxTokens,
		},
	}

	reqJSON, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/chat", bytes.NewReader(reqJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	c.logger.Debug("sending request to Ollama",
		"model", options.Model,
		"max_tokens", options.MaxTokens,
		"content_length", len(content),
	)

	// Send request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check for errors
	if resp.StatusCode != http.StatusOK {
		var errResp OllamaErrorResponse
		if err := json.Unmarshal(respBody, &errResp); err == nil && errResp.Error != "" {
			return nil, fmt.Errorf("ollama API error (%d): %s", resp.StatusCode, errResp.Error)
		}
		return nil, fmt.Errorf("ollama API error (%d): %s", resp.StatusCode, string(respBody))
	}

	// Parse response
	var chatResp OllamaChatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if chatResp.Message.Content == "" {
		return nil, fmt.Errorf("empty response from Ollama")
	}

	generationTime := int(time.Since(startTime).Milliseconds())

	// Parse the LLM response into structured summary
	summary, err := c.parseSummaryResponse(chatResp.Message.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse summary: %w", err)
	}

	// Add metadata - Ollama doesn't provide token counts in the same way
	// We can estimate or leave as 0
	summary.TokensUsed = 0 // Ollama doesn't return token counts
	summary.GenerationTimeMs = generationTime

	c.logger.Info("generated summary via Ollama",
		"model", options.Model,
		"generation_time_ms", generationTime,
	)

	return summary, nil
}

// GenerateEmbeddings generates embeddings for the given text using Ollama's local API.
func (c *OllamaClient) GenerateEmbeddings(ctx context.Context, text, model string, dimensions int) ([]float64, error) {
	startTime := time.Now()
	embedding, err := c.generateEmbedding(ctx, text, model)
	if err != nil {
		return nil, err
	}
	if dimensions > 0 && len(embedding) != dimensions {
		return nil, fmt.Errorf("ollama embedding dimensions mismatch: got %d, want %d", len(embedding), dimensions)
	}

	c.logger.Info("generated embeddings via Ollama",
		"model", model,
		"dimensions", len(embedding),
		"generation_time_ms", time.Since(startTime).Milliseconds(),
	)

	return embedding, nil
}

// GenerateEmbeddingsBatch generates embeddings for multiple texts using Ollama.
func (c *OllamaClient) GenerateEmbeddingsBatch(ctx context.Context, texts []string, model string, dimensions int) ([][]float64, error) {
	startTime := time.Now()
	embeddings := make([][]float64, 0, len(texts))
	for i, text := range texts {
		embedding, err := c.GenerateEmbeddings(ctx, text, model, dimensions)
		if err != nil {
			return nil, fmt.Errorf("generate embedding %d: %w", i, err)
		}
		embeddings = append(embeddings, embedding)
	}

	c.logger.Info("generated batch embeddings via Ollama",
		"model", model,
		"num_embeddings", len(embeddings),
		"dimensions", dimensions,
		"generation_time_ms", time.Since(startTime).Milliseconds(),
	)

	return embeddings, nil
}

func (c *OllamaClient) generateEmbedding(ctx context.Context, text, model string) ([]float64, error) {
	reqJSON, err := json.Marshal(OllamaEmbeddingsRequest{
		Model:  model,
		Prompt: text,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/embeddings", bytes.NewReader(reqJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	c.logger.Debug("sending embeddings request to Ollama",
		"model", model,
		"text_length", len(text),
	)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp OllamaErrorResponse
		if err := json.Unmarshal(respBody, &errResp); err == nil && errResp.Error != "" {
			return nil, fmt.Errorf("ollama API error (%d): %s", resp.StatusCode, errResp.Error)
		}
		return nil, fmt.Errorf("ollama API error (%d): %s", resp.StatusCode, string(respBody))
	}

	var embResp OllamaEmbeddingsResponse
	if err := json.Unmarshal(respBody, &embResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	if len(embResp.Embedding) == 0 {
		return nil, fmt.Errorf("no embeddings in response")
	}

	return embResp.Embedding, nil
}

// buildPrompt builds the prompt for summary generation.
func (c *OllamaClient) buildPrompt(content string, options steps.SummaryOptions) string {
	return buildSummaryPrompt(content, options)
}

// getSystemPrompt returns the system prompt for the LLM.
func (c *OllamaClient) getSystemPrompt(_ steps.SummaryOptions) string {
	return summarySystemPrompt
}

// parseSummaryResponse parses the LLM response into a structured Summary.
// Uses the same parser as OpenAI client for consistency.
func (c *OllamaClient) parseSummaryResponse(content string) (*steps.Summary, error) {
	return parseSummaryResponse(content)
}

// Ollama API types

// OllamaChatRequest represents an Ollama chat API request.
type OllamaChatRequest struct {
	Options  *OllamaOptions      `json:"options,omitempty"`
	Model    string              `json:"model"`
	Messages []OllamaChatMessage `json:"messages"`
	Stream   bool                `json:"stream"`
}

// OllamaChatMessage represents a message in an Ollama chat.
type OllamaChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// OllamaOptions represents Ollama generation options.
type OllamaOptions struct {
	Temperature float64 `json:"temperature,omitempty"`
	NumPredict  int     `json:"num_predict,omitempty"` // Max tokens to generate
}

// OllamaChatResponse represents an Ollama chat API response.
type OllamaChatResponse struct {
	Model     string            `json:"model"`
	CreatedAt string            `json:"created_at"`
	Message   OllamaChatMessage `json:"message"`
	Done      bool              `json:"done"`
}

// OllamaErrorResponse represents an Ollama error response.
type OllamaErrorResponse struct {
	Error string `json:"error"`
}

// OllamaEmbeddingsRequest represents an Ollama embeddings API request.
type OllamaEmbeddingsRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

// OllamaEmbeddingsResponse represents an Ollama embeddings API response.
type OllamaEmbeddingsResponse struct {
	Embedding []float64 `json:"embedding"`
}
