package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp-forge/hermes/pkg/indexer/pipeline/steps"
)

func TestOpenAIClient_GenerateSummary(t *testing.T) {
	// Create a mock HTTP server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/chat/completions", r.URL.Path)
		assert.Contains(t, r.Header.Get("Authorization"), "Bearer test-api-key")

		// Parse request body
		var reqBody OpenAIChatRequest
		err := json.NewDecoder(r.Body).Decode(&reqBody)
		require.NoError(t, err)

		assert.Equal(t, "gpt-4o-mini", reqBody.Model)
		assert.Equal(t, 500, reqBody.MaxTokens)
		assert.Len(t, reqBody.Messages, 2)

		// Return mock response
		resp := OpenAIChatResponse{
			ID:      "chatcmpl-123",
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   "gpt-4o-mini",
			Choices: []OpenAIChatChoice{
				{
					Index: 0,
					Message: OpenAIChatMessage{
						Role:    "assistant",
						Content: "EXECUTIVE SUMMARY:\nThis is a comprehensive test document that covers important topics related to software architecture and best practices.\n\nKEY POINTS:\n- The document emphasizes scalable design patterns\n- Performance optimization is a key consideration\n- Security measures are thoroughly discussed\n\nTOPICS:\nsoftware architecture, scalability, performance, security\n\nTAGS:\narchitecture, best-practices, engineering",
					},
					FinishReason: "stop",
				},
			},
			Usage: OpenAIUsage{
				PromptTokens:     100,
				CompletionTokens: 150,
				TotalTokens:      250,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	// Create client
	client, err := NewOpenAIClient(OpenAIConfig{
		APIKey:  "test-api-key",
		BaseURL: mockServer.URL,
		Timeout: 10 * time.Second,
		Logger:  hclog.NewNullLogger(),
	})
	require.NoError(t, err)

	// Test summary generation
	ctx := context.Background()
	summary, err := client.GenerateSummary(ctx, "This is a test document content", steps.SummaryOptions{
		Model:     "gpt-4o-mini",
		MaxTokens: 500,
		Language:  "en",
		Style:     "executive",
	})

	if err != nil {
		t.Logf("Response content: %q", "EXECUTIVE SUMMARY:\nThis is a comprehensive test document that covers important topics related to software architecture and best practices.\n\nKEY POINTS:\n- The document emphasizes scalable design patterns\n- Performance optimization is a key consideration\n- Security measures are thoroughly discussed\n\nTOPICS:\nsoftware architecture, scalability, performance, security\n\nTAGS:\narchitecture, best-practices, engineering")
	}
	require.NoError(t, err)
	require.NotNil(t, summary)

	// Verify summary content
	assert.Contains(t, summary.ExecutiveSummary, "comprehensive test document")
	assert.Len(t, summary.KeyPoints, 3)
	assert.Contains(t, summary.KeyPoints[0], "scalable design patterns")
	assert.Len(t, summary.Topics, 4)
	assert.Contains(t, summary.Topics, "software architecture")
	assert.Len(t, summary.Tags, 3)
	assert.Contains(t, summary.Tags, "architecture")
	assert.Equal(t, 250, summary.TokensUsed)
	assert.GreaterOrEqual(t, summary.GenerationTimeMs, 0)
}

func TestOpenAIClient_GenerateSummary_APIError(t *testing.T) {
	// Create a mock HTTP server that returns an error
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		json.NewEncoder(w).Encode(OpenAIErrorResponse{
			Error: struct {
				Message string `json:"message"`
				Type    string `json:"type"`
				Code    string `json:"code"`
			}{
				Message: "Rate limit exceeded",
				Type:    "rate_limit_error",
				Code:    "rate_limit_exceeded",
			},
		})
	}))
	defer mockServer.Close()

	// Create client
	client, err := NewOpenAIClient(OpenAIConfig{
		APIKey:  "test-api-key",
		BaseURL: mockServer.URL,
		Timeout: 10 * time.Second,
		Logger:  hclog.NewNullLogger(),
	})
	require.NoError(t, err)

	// Test summary generation
	ctx := context.Background()
	summary, err := client.GenerateSummary(ctx, "Test content", steps.SummaryOptions{
		Model:     "gpt-4o-mini",
		MaxTokens: 500,
	})

	require.Error(t, err)
	assert.Nil(t, summary)
	assert.Contains(t, err.Error(), "Rate limit exceeded")
}

func TestOpenAIClient_GenerateSummary_Timeout(t *testing.T) {
	// Create a mock HTTP server that delays response
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer mockServer.Close()

	// Create client with short timeout
	client, err := NewOpenAIClient(OpenAIConfig{
		APIKey:  "test-api-key",
		BaseURL: mockServer.URL,
		Timeout: 100 * time.Millisecond,
		Logger:  hclog.NewNullLogger(),
	})
	require.NoError(t, err)

	// Test summary generation
	ctx := context.Background()
	summary, err := client.GenerateSummary(ctx, "Test content", steps.SummaryOptions{
		Model:     "gpt-4o-mini",
		MaxTokens: 500,
	})

	require.Error(t, err)
	assert.Nil(t, summary)
	assert.Contains(t, err.Error(), "context deadline exceeded")
}

func TestOpenAIClient_GenerateSummary_EmptyResponse(t *testing.T) {
	// Create a mock HTTP server that returns empty choices
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := OpenAIChatResponse{
			ID:      "chatcmpl-123",
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   "gpt-4o-mini",
			Choices: []OpenAIChatChoice{}, // Empty choices
			Usage: OpenAIUsage{
				TotalTokens: 0,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	// Create client
	client, err := NewOpenAIClient(OpenAIConfig{
		APIKey:  "test-api-key",
		BaseURL: mockServer.URL,
		Timeout: 10 * time.Second,
		Logger:  hclog.NewNullLogger(),
	})
	require.NoError(t, err)

	// Test summary generation
	ctx := context.Background()
	summary, err := client.GenerateSummary(ctx, "Test content", steps.SummaryOptions{
		Model:     "gpt-4o-mini",
		MaxTokens: 500,
	})

	require.Error(t, err)
	assert.Nil(t, summary)
	assert.Contains(t, err.Error(), "no choices in response")
}

func TestOpenAIClient_ParseSummaryResponse(t *testing.T) {
	client := &OpenAIClient{
		logger: hclog.NewNullLogger(),
	}

	tests := []struct {
		name     string
		content  string
		wantErr  bool
		validate func(t *testing.T, summary *steps.Summary)
	}{
		{
			name: "valid response",
			content: `EXECUTIVE SUMMARY:
This document provides guidance on API design principles and best practices.

KEY POINTS:
- RESTful API design patterns
- Authentication and authorization strategies
- Rate limiting and throttling

TOPICS:
API design, REST, security, performance

TAGS:
api, rest, security, design`,
			wantErr: false,
			validate: func(t *testing.T, summary *steps.Summary) {
				assert.Contains(t, summary.ExecutiveSummary, "API design principles")
				assert.Len(t, summary.KeyPoints, 3)
				assert.Equal(t, "RESTful API design patterns", summary.KeyPoints[0])
				assert.Len(t, summary.Topics, 4)
				assert.Len(t, summary.Tags, 4)
			},
		},
		{
			name: "missing executive summary",
			content: `KEY POINTS:
- Point one
- Point two

TOPICS:
topic1, topic2`,
			wantErr: true,
		},
		{
			name:    "exact mock response content",
			content: "EXECUTIVE SUMMARY:\nThis is a comprehensive test document that covers important topics related to software architecture and best practices.\n\nKEY POINTS:\n- The document emphasizes scalable design patterns\n- Performance optimization is a key consideration\n- Security measures are thoroughly discussed\n\nTOPICS:\nsoftware architecture, scalability, performance, security\n\nTAGS:\narchitecture, best-practices, engineering",
			wantErr: false,
			validate: func(t *testing.T, summary *steps.Summary) {
				assert.Contains(t, summary.ExecutiveSummary, "comprehensive test document")
				assert.Len(t, summary.KeyPoints, 3)
				assert.Len(t, summary.Topics, 4)
				assert.Len(t, summary.Tags, 3)
			},
		},
		{
			name: "alternative bullet formats",
			content: `EXECUTIVE SUMMARY:
Test summary here.

KEY POINTS:
* First point with asterisk
• Second point with bullet
- Third point with dash

TOPICS:
topic1, topic2

TAGS:
tag1, tag2`,
			wantErr: false,
			validate: func(t *testing.T, summary *steps.Summary) {
				assert.Len(t, summary.KeyPoints, 3)
				assert.Contains(t, summary.KeyPoints[0], "First point")
				assert.Contains(t, summary.KeyPoints[1], "Second point")
				assert.Contains(t, summary.KeyPoints[2], "Third point")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summary, err := client.parseSummaryResponse(tt.content)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, summary)

			if tt.validate != nil {
				tt.validate(t, summary)
			}
		})
	}
}

func TestNewOpenAIClient_Validation(t *testing.T) {
	tests := []struct {
		name    string
		config  OpenAIConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			config: OpenAIConfig{
				APIKey: "test-key",
			},
			wantErr: false,
		},
		{
			name: "missing API key",
			config: OpenAIConfig{
				APIKey: "",
			},
			wantErr: true,
			errMsg:  "API key is required",
		},
		{
			name: "custom base URL",
			config: OpenAIConfig{
				APIKey:  "test-key",
				BaseURL: "https://custom-api.example.com",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewOpenAIClient(tt.config)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
				return
			}

			require.NoError(t, err)
			require.NotNil(t, client)
			assert.NotNil(t, client.httpClient)
			assert.NotNil(t, client.logger)
		})
	}
}

func TestOpenAIClient_ContentTruncation(t *testing.T) {
	client := &OpenAIClient{
		logger: hclog.NewNullLogger(),
	}

	// Create very long content
	longContent := string(make([]byte, 50000))

	options := steps.SummaryOptions{
		Model:     "gpt-4o-mini",
		MaxTokens: 500,
		Style:     "executive",
	}

	prompt := client.buildPrompt(longContent, options)

	// Verify content was truncated
	assert.Contains(t, prompt, "[Content truncated...]")
	assert.Less(t, len(prompt), 50000)
}
