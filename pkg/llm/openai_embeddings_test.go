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
)

func TestOpenAIClient_GenerateEmbeddings(t *testing.T) {
	t.Run("successful embeddings generation", func(t *testing.T) {
		// Mock OpenAI API server
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify request
			assert.Equal(t, "/embeddings", r.URL.Path)
			assert.Equal(t, "Bearer test-api-key", r.Header.Get("Authorization"))

			// Parse request body
			var req OpenAIEmbeddingsRequest
			err := json.NewDecoder(r.Body).Decode(&req)
			require.NoError(t, err)

			assert.Equal(t, "text-embedding-3-small", req.Model)
			assert.Equal(t, "This is a test document", req.Input)
			assert.Equal(t, 1536, req.Dimensions)

			// Send response
			resp := OpenAIEmbeddingsResponse{
				Object: "list",
				Data: []OpenAIEmbeddingData{
					{
						Object:    "embedding",
						Index:     0,
						Embedding: make([]float64, 1536),
					},
				},
				Model: "text-embedding-3-small",
				Usage: OpenAIEmbeddingsUsage{
					PromptTokens: 5,
					TotalTokens:  5,
				},
			}

			// Fill embedding with test values
			for i := range resp.Data[0].Embedding {
				resp.Data[0].Embedding[i] = float64(i) * 0.001
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		// Create client
		client, err := NewOpenAIClient(OpenAIConfig{
			APIKey:  "test-api-key",
			BaseURL: server.URL,
			Timeout: 10 * time.Second,
			Logger:  hclog.NewNullLogger(),
		})
		require.NoError(t, err)

		// Generate embeddings
		ctx := context.Background()
		embeddings, err := client.GenerateEmbeddings(ctx, "This is a test document", "text-embedding-3-small", 1536)

		require.NoError(t, err)
		require.NotNil(t, embeddings)
		assert.Equal(t, 1536, len(embeddings))
		assert.Equal(t, 0.0, embeddings[0])
		assert.Equal(t, 0.001, embeddings[1])
	})

	t.Run("text-embedding-3-large with 3072 dimensions", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req OpenAIEmbeddingsRequest
			json.NewDecoder(r.Body).Decode(&req)

			assert.Equal(t, "text-embedding-3-large", req.Model)
			assert.Equal(t, 3072, req.Dimensions)

			resp := OpenAIEmbeddingsResponse{
				Object: "list",
				Data: []OpenAIEmbeddingData{
					{
						Object:    "embedding",
						Index:     0,
						Embedding: make([]float64, 3072),
					},
				},
				Model: "text-embedding-3-large",
				Usage: OpenAIEmbeddingsUsage{
					PromptTokens: 5,
					TotalTokens:  5,
				},
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		client, err := NewOpenAIClient(OpenAIConfig{
			APIKey:  "test-api-key",
			BaseURL: server.URL,
			Logger:  hclog.NewNullLogger(),
		})
		require.NoError(t, err)

		ctx := context.Background()
		embeddings, err := client.GenerateEmbeddings(ctx, "Test", "text-embedding-3-large", 3072)

		require.NoError(t, err)
		assert.Equal(t, 3072, len(embeddings))
	})

	t.Run("API error handling", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
		defer server.Close()

		client, err := NewOpenAIClient(OpenAIConfig{
			APIKey:  "test-api-key",
			BaseURL: server.URL,
			Logger:  hclog.NewNullLogger(),
		})
		require.NoError(t, err)

		ctx := context.Background()
		_, err = client.GenerateEmbeddings(ctx, "Test", "text-embedding-3-small", 1536)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "Rate limit exceeded")
	})

	t.Run("empty response handling", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := OpenAIEmbeddingsResponse{
				Object: "list",
				Data:   []OpenAIEmbeddingData{},
				Model:  "text-embedding-3-small",
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		client, err := NewOpenAIClient(OpenAIConfig{
			APIKey:  "test-api-key",
			BaseURL: server.URL,
			Logger:  hclog.NewNullLogger(),
		})
		require.NoError(t, err)

		ctx := context.Background()
		_, err = client.GenerateEmbeddings(ctx, "Test", "text-embedding-3-small", 1536)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "no embeddings in response")
	})

	t.Run("context timeout", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(2 * time.Second) // Delay longer than client timeout
		}))
		defer server.Close()

		client, err := NewOpenAIClient(OpenAIConfig{
			APIKey:  "test-api-key",
			BaseURL: server.URL,
			Timeout: 100 * time.Millisecond, // Short timeout
			Logger:  hclog.NewNullLogger(),
		})
		require.NoError(t, err)

		ctx := context.Background()
		_, err = client.GenerateEmbeddings(ctx, "Test", "text-embedding-3-small", 1536)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "context deadline exceeded")
	})
}

func TestOpenAIClient_GenerateEmbeddingsBatch(t *testing.T) {
	t.Run("successful batch embeddings", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req OpenAIEmbeddingsRequest
			err := json.NewDecoder(r.Body).Decode(&req)
			require.NoError(t, err)

			// Input should be an array
			texts, ok := req.Input.([]interface{})
			require.True(t, ok)
			assert.Equal(t, 3, len(texts))

			resp := OpenAIEmbeddingsResponse{
				Object: "list",
				Data: []OpenAIEmbeddingData{
					{
						Object:    "embedding",
						Index:     0,
						Embedding: make([]float64, 1536),
					},
					{
						Object:    "embedding",
						Index:     1,
						Embedding: make([]float64, 1536),
					},
					{
						Object:    "embedding",
						Index:     2,
						Embedding: make([]float64, 1536),
					},
				},
				Model: "text-embedding-3-small",
				Usage: OpenAIEmbeddingsUsage{
					PromptTokens: 15,
					TotalTokens:  15,
				},
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		client, err := NewOpenAIClient(OpenAIConfig{
			APIKey:  "test-api-key",
			BaseURL: server.URL,
			Logger:  hclog.NewNullLogger(),
		})
		require.NoError(t, err)

		ctx := context.Background()
		texts := []string{"First doc", "Second doc", "Third doc"}
		embeddings, err := client.GenerateEmbeddingsBatch(ctx, texts, "text-embedding-3-small", 1536)

		require.NoError(t, err)
		require.NotNil(t, embeddings)
		assert.Equal(t, 3, len(embeddings))
		for _, emb := range embeddings {
			assert.Equal(t, 1536, len(emb))
		}
	})

	t.Run("batch with single item", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := OpenAIEmbeddingsResponse{
				Object: "list",
				Data: []OpenAIEmbeddingData{
					{
						Object:    "embedding",
						Index:     0,
						Embedding: make([]float64, 1536),
					},
				},
				Model: "text-embedding-3-small",
				Usage: OpenAIEmbeddingsUsage{
					PromptTokens: 5,
					TotalTokens:  5,
				},
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		client, err := NewOpenAIClient(OpenAIConfig{
			APIKey:  "test-api-key",
			BaseURL: server.URL,
			Logger:  hclog.NewNullLogger(),
		})
		require.NoError(t, err)

		ctx := context.Background()
		embeddings, err := client.GenerateEmbeddingsBatch(ctx, []string{"Single doc"}, "text-embedding-3-small", 1536)

		require.NoError(t, err)
		assert.Equal(t, 1, len(embeddings))
		assert.Equal(t, 1536, len(embeddings[0]))
	})

	t.Run("batch error handling", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(OpenAIErrorResponse{
				Error: struct {
					Message string `json:"message"`
					Type    string `json:"type"`
					Code    string `json:"code"`
				}{
					Message: "Invalid request",
					Type:    "invalid_request_error",
				},
			})
		}))
		defer server.Close()

		client, err := NewOpenAIClient(OpenAIConfig{
			APIKey:  "test-api-key",
			BaseURL: server.URL,
			Logger:  hclog.NewNullLogger(),
		})
		require.NoError(t, err)

		ctx := context.Background()
		_, err = client.GenerateEmbeddingsBatch(ctx, []string{"Test"}, "text-embedding-3-small", 1536)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "Invalid request")
	})

	t.Run("batch ordering preserved", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Return embeddings in non-sequential order to test sorting
			resp := OpenAIEmbeddingsResponse{
				Object: "list",
				Data: []OpenAIEmbeddingData{
					{
						Object:    "embedding",
						Index:     2, // Out of order
						Embedding: []float64{0.2},
					},
					{
						Object:    "embedding",
						Index:     0,
						Embedding: []float64{0.0},
					},
					{
						Object:    "embedding",
						Index:     1,
						Embedding: []float64{0.1},
					},
				},
				Model: "text-embedding-3-small",
				Usage: OpenAIEmbeddingsUsage{
					PromptTokens: 10,
					TotalTokens:  10,
				},
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		client, err := NewOpenAIClient(OpenAIConfig{
			APIKey:  "test-api-key",
			BaseURL: server.URL,
			Logger:  hclog.NewNullLogger(),
		})
		require.NoError(t, err)

		ctx := context.Background()
		embeddings, err := client.GenerateEmbeddingsBatch(ctx, []string{"A", "B", "C"}, "text-embedding-3-small", 1)

		require.NoError(t, err)
		assert.Equal(t, 3, len(embeddings))
		// Verify ordering is preserved
		assert.Equal(t, 0.0, embeddings[0][0])
		assert.Equal(t, 0.1, embeddings[1][0])
		assert.Equal(t, 0.2, embeddings[2][0])
	})
}
