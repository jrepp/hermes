package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOllamaClient_GenerateEmbeddings(t *testing.T) {
	t.Run("successful embeddings generation", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method)
			assert.Equal(t, "/api/embeddings", r.URL.Path)

			var req OllamaEmbeddingsRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			assert.Equal(t, "nomic-embed-text", req.Model)
			assert.Equal(t, "This is a test document", req.Prompt)

			w.Header().Set("Content-Type", "application/json")
			require.NoError(t, json.NewEncoder(w).Encode(OllamaEmbeddingsResponse{
				Embedding: []float64{0.1, 0.2, 0.3},
			}))
		}))
		defer server.Close()

		client, err := NewOllamaClient(OllamaConfig{
			BaseURL: server.URL,
			Timeout: 10 * time.Second,
			Logger:  hclog.NewNullLogger(),
		})
		require.NoError(t, err)

		embedding, err := client.GenerateEmbeddings(context.Background(), "This is a test document", "nomic-embed-text", 3)

		require.NoError(t, err)
		assert.Equal(t, []float64{0.1, 0.2, 0.3}, embedding)
	})

	t.Run("dimension mismatch", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			require.NoError(t, json.NewEncoder(w).Encode(OllamaEmbeddingsResponse{
				Embedding: []float64{0.1, 0.2, 0.3},
			}))
		}))
		defer server.Close()

		client, err := NewOllamaClient(OllamaConfig{BaseURL: server.URL, Logger: hclog.NewNullLogger()})
		require.NoError(t, err)

		_, err = client.GenerateEmbeddings(context.Background(), "Test", "nomic-embed-text", 768)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "dimensions mismatch")
	})

	t.Run("API error handling", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			require.NoError(t, json.NewEncoder(w).Encode(OllamaErrorResponse{Error: "model not found"}))
		}))
		defer server.Close()

		client, err := NewOllamaClient(OllamaConfig{BaseURL: server.URL, Logger: hclog.NewNullLogger()})
		require.NoError(t, err)

		_, err = client.GenerateEmbeddings(context.Background(), "Test", "nomic-embed-text", 768)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "model not found")
	})

	t.Run("empty response handling", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			require.NoError(t, json.NewEncoder(w).Encode(OllamaEmbeddingsResponse{}))
		}))
		defer server.Close()

		client, err := NewOllamaClient(OllamaConfig{BaseURL: server.URL, Logger: hclog.NewNullLogger()})
		require.NoError(t, err)

		_, err = client.GenerateEmbeddings(context.Background(), "Test", "nomic-embed-text", 768)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "no embeddings in response")
	})
}

func TestOllamaClient_GenerateEmbeddingsBatch(t *testing.T) {
	t.Run("successful batch embeddings", func(t *testing.T) {
		var prompts []string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/api/embeddings", r.URL.Path)

			var req OllamaEmbeddingsRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			prompts = append(prompts, req.Prompt)

			w.Header().Set("Content-Type", "application/json")
			require.NoError(t, json.NewEncoder(w).Encode(OllamaEmbeddingsResponse{
				Embedding: []float64{float64(len(prompts)), 0.2},
			}))
		}))
		defer server.Close()

		client, err := NewOllamaClient(OllamaConfig{BaseURL: server.URL, Logger: hclog.NewNullLogger()})
		require.NoError(t, err)

		embeddings, err := client.GenerateEmbeddingsBatch(context.Background(), []string{"First doc", "Second doc"}, "nomic-embed-text", 2)

		require.NoError(t, err)
		assert.Equal(t, []string{"First doc", "Second doc"}, prompts)
		require.Len(t, embeddings, 2)
		assert.Equal(t, []float64{1, 0.2}, embeddings[0])
		assert.Equal(t, []float64{2, 0.2}, embeddings[1])
	})

	t.Run("batch error includes item index", func(t *testing.T) {
		calls := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			calls++
			if calls == 2 {
				w.WriteHeader(http.StatusInternalServerError)
				require.NoError(t, json.NewEncoder(w).Encode(OllamaErrorResponse{Error: "ollama unavailable"}))
				return
			}

			w.Header().Set("Content-Type", "application/json")
			require.NoError(t, json.NewEncoder(w).Encode(OllamaEmbeddingsResponse{
				Embedding: []float64{0.1, 0.2},
			}))
		}))
		defer server.Close()

		client, err := NewOllamaClient(OllamaConfig{BaseURL: server.URL, Logger: hclog.NewNullLogger()})
		require.NoError(t, err)

		_, err = client.GenerateEmbeddingsBatch(context.Background(), []string{"First doc", "Second doc"}, "nomic-embed-text", 2)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "generate embedding 1")
		assert.Contains(t, err.Error(), "ollama unavailable")
	})
}

func TestOllamaClient_GenerateEmbeddings_Live(t *testing.T) {
	model := os.Getenv("HERMES_TEST_OLLAMA_EMBED_MODEL")
	if model == "" {
		t.Skip("set HERMES_TEST_OLLAMA_EMBED_MODEL to run live Ollama embedding test")
	}

	baseURL := os.Getenv("HERMES_TEST_OLLAMA_URL")
	client, err := NewOllamaClient(OllamaConfig{
		BaseURL: baseURL,
		Timeout: 30 * time.Second,
		Logger:  hclog.NewNullLogger(),
	})
	require.NoError(t, err)

	embedding, err := client.GenerateEmbeddings(context.Background(), "Hermes live embedding smoke test", model, 768)

	require.NoError(t, err)
	assert.Len(t, embedding, 768)
	assert.NotZero(t, embedding[0])
}
