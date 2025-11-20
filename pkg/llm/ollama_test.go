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

func TestOllamaClient_GenerateSummary(t *testing.T) {
	// Create a mock HTTP server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/api/chat", r.URL.Path)

		// Parse request body
		var reqBody OllamaChatRequest
		err := json.NewDecoder(r.Body).Decode(&reqBody)
		require.NoError(t, err)

		assert.Equal(t, "llama2", reqBody.Model)
		assert.False(t, reqBody.Stream)
		assert.Len(t, reqBody.Messages, 2)
		assert.Equal(t, "system", reqBody.Messages[0].Role)
		assert.Equal(t, "user", reqBody.Messages[1].Role)

		// Return mock response
		resp := OllamaChatResponse{
			Model:     "llama2",
			CreatedAt: time.Now().Format(time.RFC3339),
			Message: OllamaChatMessage{
				Role:    "assistant",
				Content: "EXECUTIVE SUMMARY:\nThis is a comprehensive test document that covers important topics related to software architecture and best practices.\n\nKEY POINTS:\n- The document emphasizes scalable design patterns\n- Performance optimization is a key consideration\n- Security measures are thoroughly discussed\n\nTOPICS:\nsoftware architecture, scalability, performance, security\n\nTAGS:\narchitecture, best-practices, engineering",
			},
			Done: true,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	// Create client
	client, err := NewOllamaClient(OllamaConfig{
		BaseURL: mockServer.URL,
		Timeout: 10 * time.Second,
		Logger:  hclog.NewNullLogger(),
	})
	require.NoError(t, err)

	// Test summary generation
	ctx := context.Background()
	summary, err := client.GenerateSummary(ctx, "This is a test document content", steps.SummaryOptions{
		Model:     "llama2",
		MaxTokens: 500,
		Language:  "en",
		Style:     "executive",
	})

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
	assert.Greater(t, summary.GenerationTimeMs, 0)
}

func TestOllamaClient_GenerateSummary_APIError(t *testing.T) {
	// Create a mock HTTP server that returns an error
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(OllamaErrorResponse{
			Error: "model not found",
		})
	}))
	defer mockServer.Close()

	// Create client
	client, err := NewOllamaClient(OllamaConfig{
		BaseURL: mockServer.URL,
		Timeout: 10 * time.Second,
		Logger:  hclog.NewNullLogger(),
	})
	require.NoError(t, err)

	// Test summary generation
	ctx := context.Background()
	summary, err := client.GenerateSummary(ctx, "Test content", steps.SummaryOptions{
		Model:     "llama2",
		MaxTokens: 500,
	})

	require.Error(t, err)
	assert.Nil(t, summary)
	assert.Contains(t, err.Error(), "model not found")
}

func TestOllamaClient_GenerateSummary_Timeout(t *testing.T) {
	// Create a mock HTTP server that delays response
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer mockServer.Close()

	// Create client with short timeout
	client, err := NewOllamaClient(OllamaConfig{
		BaseURL: mockServer.URL,
		Timeout: 100 * time.Millisecond,
		Logger:  hclog.NewNullLogger(),
	})
	require.NoError(t, err)

	// Test summary generation
	ctx := context.Background()
	summary, err := client.GenerateSummary(ctx, "Test content", steps.SummaryOptions{
		Model:     "llama2",
		MaxTokens: 500,
	})

	require.Error(t, err)
	assert.Nil(t, summary)
	assert.Contains(t, err.Error(), "context deadline exceeded")
}

func TestOllamaClient_GenerateSummary_EmptyResponse(t *testing.T) {
	// Create a mock HTTP server that returns empty content
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := OllamaChatResponse{
			Model:     "llama2",
			CreatedAt: time.Now().Format(time.RFC3339),
			Message: OllamaChatMessage{
				Role:    "assistant",
				Content: "", // Empty content
			},
			Done: true,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	// Create client
	client, err := NewOllamaClient(OllamaConfig{
		BaseURL: mockServer.URL,
		Timeout: 10 * time.Second,
		Logger:  hclog.NewNullLogger(),
	})
	require.NoError(t, err)

	// Test summary generation
	ctx := context.Background()
	summary, err := client.GenerateSummary(ctx, "Test content", steps.SummaryOptions{
		Model:     "llama2",
		MaxTokens: 500,
	})

	require.Error(t, err)
	assert.Nil(t, summary)
	assert.Contains(t, err.Error(), "empty response from Ollama")
}

func TestOllamaClient_ParseSummaryResponse(t *testing.T) {
	client := &OllamaClient{
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

func TestNewOllamaClient_Validation(t *testing.T) {
	tests := []struct {
		name    string
		config  OllamaConfig
		wantErr bool
	}{
		{
			name: "valid config with custom URL",
			config: OllamaConfig{
				BaseURL: "http://custom-ollama:11434",
			},
			wantErr: false,
		},
		{
			name:    "default config",
			config:  OllamaConfig{},
			wantErr: false,
		},
		{
			name: "custom timeout",
			config: OllamaConfig{
				BaseURL: "http://localhost:11434",
				Timeout: 60 * time.Second,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewOllamaClient(tt.config)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, client)
			assert.NotNil(t, client.httpClient)
			assert.NotNil(t, client.logger)

			// Check defaults
			if tt.config.BaseURL == "" {
				assert.Equal(t, "http://localhost:11434", client.baseURL)
			}
			if tt.config.Timeout == 0 {
				assert.Equal(t, 300*time.Second, client.httpClient.Timeout)
			}
		})
	}
}

func TestOllamaClient_ContentTruncation(t *testing.T) {
	client := &OllamaClient{
		logger: hclog.NewNullLogger(),
	}

	// Create very long content
	longContent := string(make([]byte, 50000))

	options := steps.SummaryOptions{
		Model:     "llama2",
		MaxTokens: 500,
		Style:     "executive",
	}

	prompt := client.buildPrompt(longContent, options)

	// Verify content was truncated
	assert.Contains(t, prompt, "[Content truncated...]")
	assert.Less(t, len(prompt), 50000)
}

func TestOllamaClient_DifferentModels(t *testing.T) {
	models := []string{"llama2", "mistral", "codellama", "phi"}

	for _, model := range models {
		t.Run(model, func(t *testing.T) {
			mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var reqBody OllamaChatRequest
				json.NewDecoder(r.Body).Decode(&reqBody)
				assert.Equal(t, model, reqBody.Model)

				resp := OllamaChatResponse{
					Model: model,
					Message: OllamaChatMessage{
						Role:    "assistant",
						Content: "EXECUTIVE SUMMARY:\nTest summary.\n\nKEY POINTS:\n- Point 1\n\nTOPICS:\ntopic1\n\nTAGS:\ntag1",
					},
					Done: true,
				}
				json.NewEncoder(w).Encode(resp)
			}))
			defer mockServer.Close()

			client, _ := NewOllamaClient(OllamaConfig{
				BaseURL: mockServer.URL,
			})

			summary, err := client.GenerateSummary(context.Background(), "test", steps.SummaryOptions{
				Model: model,
			})

			require.NoError(t, err)
			assert.NotNil(t, summary)
		})
	}
}
