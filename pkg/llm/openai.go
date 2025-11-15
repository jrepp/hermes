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

// OpenAIClient implements the LLMClient interface for OpenAI's API.
type OpenAIClient struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
	logger     hclog.Logger
}

// OpenAIConfig holds configuration for the OpenAI client.
type OpenAIConfig struct {
	APIKey  string        // OpenAI API key
	BaseURL string        // Base URL (default: https://api.openai.com/v1)
	Timeout time.Duration // HTTP timeout (default: 60s)
	Logger  hclog.Logger  // Logger (optional)
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
	defer resp.Body.Close()

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
	// Truncate content if too long (roughly 10k tokens max for input)
	maxContentChars := 40000 // Roughly 10k tokens
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
func (c *OpenAIClient) getSystemPrompt(options steps.SummaryOptions) string {
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
func (c *OpenAIClient) parseSummaryResponse(content string) (*steps.Summary, error) {
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

		// Detect section headers (check if line starts with the header, allowing for colons)
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

// OpenAI API types

type OpenAIChatRequest struct {
	Model       string              `json:"model"`
	Messages    []OpenAIChatMessage `json:"messages"`
	MaxTokens   int                 `json:"max_tokens,omitempty"`
	Temperature float64             `json:"temperature,omitempty"`
	TopP        float64             `json:"top_p,omitempty"`
}

type OpenAIChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type OpenAIChatResponse struct {
	ID      string             `json:"id"`
	Object  string             `json:"object"`
	Created int64              `json:"created"`
	Model   string             `json:"model"`
	Choices []OpenAIChatChoice `json:"choices"`
	Usage   OpenAIUsage        `json:"usage"`
}

type OpenAIChatChoice struct {
	Index        int               `json:"index"`
	Message      OpenAIChatMessage `json:"message"`
	FinishReason string            `json:"finish_reason"`
}

type OpenAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type OpenAIErrorResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}
