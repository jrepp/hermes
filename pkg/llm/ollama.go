package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/hashicorp-forge/hermes/pkg/indexer/pipeline/steps"
	"github.com/hashicorp/go-hclog"
)

// OllamaClient implements the LLMClient interface for Ollama's local API.
type OllamaClient struct {
	baseURL    string
	httpClient *http.Client
	logger     hclog.Logger
}

// OllamaConfig holds configuration for the Ollama client.
type OllamaConfig struct {
	BaseURL string        // Base URL (default: http://localhost:11434)
	Timeout time.Duration // HTTP timeout (default: 300s for local generation)
	Logger  hclog.Logger  // Logger (optional)
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
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check for errors
	if resp.StatusCode != http.StatusOK {
		var errResp OllamaErrorResponse
		if err := json.Unmarshal(respBody, &errResp); err == nil && errResp.Error != "" {
			return nil, fmt.Errorf("Ollama API error (%d): %s", resp.StatusCode, errResp.Error)
		}
		return nil, fmt.Errorf("Ollama API error (%d): %s", resp.StatusCode, string(respBody))
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

// buildPrompt builds the prompt for summary generation.
func (c *OllamaClient) buildPrompt(content string, options steps.SummaryOptions) string {
	// Truncate content if too long
	maxContentChars := 40000
	if len(content) > maxContentChars {
		content = content[:maxContentChars] + "\n\n[Content truncated...]"
	}

	styleInstruction := ""
	switch options.Style {
	case "executive":
		styleInstruction = "Provide an executive summary suitable for leadership."
	case "technical":
		styleInstruction = "Provide a technical summary with implementation details."
	case "bullet-points":
		styleInstruction = "Focus on concise bullet points of key information."
	default:
		styleInstruction = "Provide a clear and comprehensive summary."
	}

	return fmt.Sprintf(`%s

Please analyze the following document and provide a summary:

%s`, styleInstruction, content)
}

// getSystemPrompt returns the system prompt for the LLM.
func (c *OllamaClient) getSystemPrompt(options steps.SummaryOptions) string {
	return `You are an expert document analyst. Your task is to provide accurate, well-structured summaries of documents.

For each document, provide:
1. EXECUTIVE SUMMARY: A concise 2-3 sentence overview
2. KEY POINTS: The 3-5 most important takeaways (one per line, prefixed with "- ")
3. TOPICS: Main topics covered (comma-separated)
4. TAGS: Relevant tags for categorization (comma-separated)

Format your response as follows:
EXECUTIVE SUMMARY:
[Your executive summary here]

KEY POINTS:
- [First key point]
- [Second key point]
- [Third key point]

TOPICS:
[topic1, topic2, topic3]

TAGS:
[tag1, tag2, tag3]`
}

// parseSummaryResponse parses the LLM response into a structured Summary.
// Uses the same parser as OpenAI client for consistency.
func (c *OllamaClient) parseSummaryResponse(content string) (*steps.Summary, error) {
	summary := &steps.Summary{
		KeyPoints:  []string{},
		Topics:     []string{},
		Tags:       []string{},
		Confidence: 0.8, // Default confidence
	}

	lines := strings.Split(content, "\n")
	currentSection := ""

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		lineUpper := strings.ToUpper(line)

		// Detect section headers (check if line starts with the header)
		if strings.HasPrefix(lineUpper, "EXECUTIVE SUMMARY") {
			currentSection = "executive"
			continue
		}
		if strings.HasPrefix(lineUpper, "KEY POINTS") {
			currentSection = "keypoints"
			continue
		}
		if strings.HasPrefix(lineUpper, "TOPICS") {
			currentSection = "topics"
			continue
		}
		if strings.HasPrefix(lineUpper, "TAGS") {
			currentSection = "tags"
			continue
		}

		// Parse content based on current section
		switch currentSection {
		case "executive":
			if summary.ExecutiveSummary == "" {
				summary.ExecutiveSummary = line
			} else {
				summary.ExecutiveSummary += " " + line
			}

		case "keypoints":
			// Remove bullet prefixes
			point := strings.TrimPrefix(line, "- ")
			point = strings.TrimPrefix(point, "* ")
			point = strings.TrimPrefix(point, "• ")
			if point != line { // Only add if it had a bullet prefix
				summary.KeyPoints = append(summary.KeyPoints, point)
			}

		case "topics":
			// Split by commas
			topics := strings.Split(line, ",")
			for _, topic := range topics {
				topic = strings.TrimSpace(topic)
				if topic != "" {
					summary.Topics = append(summary.Topics, topic)
				}
			}

		case "tags":
			// Split by commas
			tags := strings.Split(line, ",")
			for _, tag := range tags {
				tag = strings.TrimSpace(tag)
				if tag != "" {
					summary.Tags = append(summary.Tags, tag)
				}
			}
		}
	}

	// Validate we got the essential parts
	if summary.ExecutiveSummary == "" {
		return nil, fmt.Errorf("failed to extract executive summary from response")
	}

	return summary, nil
}

// Ollama API types

type OllamaChatRequest struct {
	Model    string              `json:"model"`
	Messages []OllamaChatMessage `json:"messages"`
	Stream   bool                `json:"stream"`
	Options  *OllamaOptions      `json:"options,omitempty"`
}

type OllamaChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type OllamaOptions struct {
	Temperature float64 `json:"temperature,omitempty"`
	NumPredict  int     `json:"num_predict,omitempty"` // Max tokens to generate
}

type OllamaChatResponse struct {
	Model     string            `json:"model"`
	CreatedAt string            `json:"created_at"`
	Message   OllamaChatMessage `json:"message"`
	Done      bool              `json:"done"`
}

type OllamaErrorResponse struct {
	Error string `json:"error"`
}
