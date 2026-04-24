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

// OpenAIClient implements the LLMClient interface for OpenAI's API.
type OpenAIClient struct {
	logger     hclog.Logger
	httpClient *http.Client
	apiKey     string
	baseURL    string
}

// OpenAIConfig holds configuration for the OpenAI client.
type OpenAIConfig struct {
	Logger  hclog.Logger
	APIKey  string
	BaseURL string
	Timeout time.Duration
}

// NewOpenAIClient creates a new OpenAI client.
func NewOpenAIClient(config OpenAIConfig) (*OpenAIClient, error) {
	if config.APIKey == "" {
		return nil, fmt.Errorf("OpenAI API key is required")
	}

	if config.BaseURL == "" {
		config.BaseURL = "https://api.openai.com/v1"
	}

	if config.Timeout == 0 {
		config.Timeout = 60 * time.Second
	}

	if config.Logger == nil {
		config.Logger = hclog.NewNullLogger()
	}

	return &OpenAIClient{
		apiKey:  config.APIKey,
		baseURL: config.BaseURL,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
		logger: config.Logger.Named("openai-client"),
	}, nil
}

// GenerateSummary generates a summary using OpenAI's API.
func (c *OpenAIClient) GenerateSummary(ctx context.Context, content string, options steps.SummaryOptions) (*steps.Summary, error) {
	startTime := time.Now()

	// Build the prompt
	prompt := c.buildPrompt(content, options)

	// Prepare the request
	reqBody := OpenAIChatRequest{
		Model: options.Model,
		Messages: []OpenAIChatMessage{
			{
				Role:    "system",
				Content: c.getSystemPrompt(options),
			},
			{
				Role:    "user",
				Content: prompt,
			},
		},
		MaxTokens:   options.MaxTokens,
		Temperature: 0.3, // Lower temperature for more consistent summaries
		TopP:        1.0,
	}

	reqJSON, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/chat/completions", bytes.NewReader(reqJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	c.logger.Debug("sending request to OpenAI",
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
		var errResp OpenAIErrorResponse
		if err := json.Unmarshal(respBody, &errResp); err == nil {
			return nil, fmt.Errorf("OpenAI API error (%d): %s", resp.StatusCode, errResp.Error.Message)
		}
		return nil, fmt.Errorf("OpenAI API error (%d): %s", resp.StatusCode, string(respBody))
	}

	// Parse response
	var chatResp OpenAIChatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	generationTime := int(time.Since(startTime).Milliseconds())

	// Parse the LLM response into structured summary
	summary, err := c.parseSummaryResponse(chatResp.Choices[0].Message.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse summary: %w", err)
	}

	// Add metadata
	summary.TokensUsed = chatResp.Usage.TotalTokens
	summary.GenerationTimeMs = generationTime

	c.logger.Info("generated summary via OpenAI",
		"model", options.Model,
		"tokens_used", summary.TokensUsed,
		"generation_time_ms", generationTime,
	)

	return summary, nil
}

// buildPrompt builds the prompt for summary generation.
func (c *OpenAIClient) buildPrompt(content string, options steps.SummaryOptions) string {
	return buildSummaryPrompt(content, options)
}

// getSystemPrompt returns the system prompt for the LLM.
func (c *OpenAIClient) getSystemPrompt(_ steps.SummaryOptions) string {
	return summarySystemPrompt
}

// parseSummaryResponse parses the LLM response into a structured Summary.
func (c *OpenAIClient) parseSummaryResponse(content string) (*steps.Summary, error) {
	return parseSummaryResponse(content)
}

// OpenAI API types

// OpenAIChatRequest represents an OpenAI chat API request.
type OpenAIChatRequest struct {
	Model       string              `json:"model"`
	Messages    []OpenAIChatMessage `json:"messages"`
	MaxTokens   int                 `json:"max_tokens,omitempty"`
	Temperature float64             `json:"temperature,omitempty"`
	TopP        float64             `json:"top_p,omitempty"`
}

// OpenAIChatMessage represents a message in an OpenAI chat.
type OpenAIChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// OpenAIChatResponse represents an OpenAI chat API response.
type OpenAIChatResponse struct {
	ID      string             `json:"id"`
	Object  string             `json:"object"`
	Model   string             `json:"model"`
	Choices []OpenAIChatChoice `json:"choices"`
	Usage   OpenAIUsage        `json:"usage"`
	Created int64              `json:"created"`
}

// OpenAIChatChoice represents a choice in an OpenAI chat response.
type OpenAIChatChoice struct {
	Message      OpenAIChatMessage `json:"message"`
	FinishReason string            `json:"finish_reason"`
	Index        int               `json:"index"`
}

// OpenAIUsage represents token usage in an OpenAI response.
type OpenAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// OpenAIErrorResponse represents an OpenAI error response.
type OpenAIErrorResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

// Embeddings API

// GenerateEmbeddings generates embeddings for the given text using OpenAI's embeddings API.
func (c *OpenAIClient) GenerateEmbeddings(ctx context.Context, text, model string, dimensions int) ([]float64, error) {
	startTime := time.Now()

	// Prepare the request
	reqBody := OpenAIEmbeddingsRequest{
		Input:      text,
		Model:      model,
		Dimensions: dimensions,
	}

	reqJSON, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/embeddings", bytes.NewReader(reqJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	c.logger.Debug("sending embeddings request to OpenAI",
		"model", model,
		"dimensions", dimensions,
		"text_length", len(text),
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
		var errResp OpenAIErrorResponse
		if err := json.Unmarshal(respBody, &errResp); err == nil {
			return nil, fmt.Errorf("OpenAI API error (%d): %s", resp.StatusCode, errResp.Error.Message)
		}
		return nil, fmt.Errorf("OpenAI API error (%d): %s", resp.StatusCode, string(respBody))
	}

	// Parse response
	var embResp OpenAIEmbeddingsResponse
	if err := json.Unmarshal(respBody, &embResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if len(embResp.Data) == 0 {
		return nil, fmt.Errorf("no embeddings in response")
	}

	generationTime := time.Since(startTime)

	c.logger.Info("generated embeddings via OpenAI",
		"model", model,
		"dimensions", len(embResp.Data[0].Embedding),
		"tokens_used", embResp.Usage.TotalTokens,
		"generation_time_ms", generationTime.Milliseconds(),
	)

	return embResp.Data[0].Embedding, nil
}

// GenerateEmbeddingsBatch generates embeddings for multiple texts in a single API call.
func (c *OpenAIClient) GenerateEmbeddingsBatch(ctx context.Context, texts []string, model string, dimensions int) ([][]float64, error) {
	startTime := time.Now()

	// Prepare the request
	reqBody := OpenAIEmbeddingsRequest{
		Input:      texts,
		Model:      model,
		Dimensions: dimensions,
	}

	reqJSON, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/embeddings", bytes.NewReader(reqJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	c.logger.Debug("sending batch embeddings request to OpenAI",
		"model", model,
		"dimensions", dimensions,
		"num_texts", len(texts),
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
		var errResp OpenAIErrorResponse
		if err := json.Unmarshal(respBody, &errResp); err == nil {
			return nil, fmt.Errorf("OpenAI API error (%d): %s", resp.StatusCode, errResp.Error.Message)
		}
		return nil, fmt.Errorf("OpenAI API error (%d): %s", resp.StatusCode, string(respBody))
	}

	// Parse response
	var embResp OpenAIEmbeddingsResponse
	if err := json.Unmarshal(respBody, &embResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if len(embResp.Data) == 0 {
		return nil, fmt.Errorf("no embeddings in response")
	}

	generationTime := time.Since(startTime)

	c.logger.Info("generated batch embeddings via OpenAI",
		"model", model,
		"num_embeddings", len(embResp.Data),
		"dimensions", dimensions,
		"tokens_used", embResp.Usage.TotalTokens,
		"generation_time_ms", generationTime.Milliseconds(),
	)

	// Extract embeddings in order
	embeddings := make([][]float64, len(embResp.Data))
	for _, data := range embResp.Data {
		embeddings[data.Index] = data.Embedding
	}

	return embeddings, nil
}

// OpenAI Embeddings API types

// OpenAIEmbeddingsRequest represents an OpenAI embeddings API request.
type OpenAIEmbeddingsRequest struct {
	Input      interface{} `json:"input"` // string or []string
	Model      string      `json:"model"`
	Dimensions int         `json:"dimensions,omitempty"`
}

// OpenAIEmbeddingsResponse represents an OpenAI embeddings API response.
type OpenAIEmbeddingsResponse struct {
	Object string                `json:"object"`
	Model  string                `json:"model"`
	Data   []OpenAIEmbeddingData `json:"data"`
	Usage  OpenAIEmbeddingsUsage `json:"usage"`
}

// OpenAIEmbeddingData represents embedding data in an OpenAI response.
type OpenAIEmbeddingData struct {
	Object    string    `json:"object"`
	Embedding []float64 `json:"embedding"`
	Index     int       `json:"index"`
}

// OpenAIEmbeddingsUsage represents token usage in an embeddings response.
type OpenAIEmbeddingsUsage struct {
	PromptTokens int `json:"prompt_tokens"`
	TotalTokens  int `json:"total_tokens"`
}
