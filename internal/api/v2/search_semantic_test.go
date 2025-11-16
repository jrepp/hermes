package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp-forge/hermes/internal/server"
	pkgauth "github.com/hashicorp-forge/hermes/pkg/auth"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
)

// TODO: To properly test the happy paths with mocked search services, we need to:
// 1. Create interfaces for SemanticSearch and HybridSearch
// 2. Update server.Server to use those interfaces
// 3. Then create mock implementations
//
// For now, these tests focus on error cases and request validation which don't
// require actual search service implementations.

func TestSemanticSearchHandler(t *testing.T) {
	logger := hclog.NewNullLogger()

	// Note: Query validation tests would require a working SemanticSearch service
	// or interface-based mocking. The handler checks service availability first,
	// which is the correct behavior (fail fast if service is down).

	t.Run("no authentication returns unauthorized", func(t *testing.T) {
		srv := server.Server{
			SemanticSearch: nil,
			Logger:         logger,
		}

		reqBody := SemanticSearchRequest{
			Query: "test",
			Limit: 10,
		}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/api/v2/search/semantic", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()

		handler := SemanticSearchHandler(srv)
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("semantic search not configured returns service unavailable", func(t *testing.T) {
		srv := server.Server{
			SemanticSearch: nil, // Not configured
			Logger:         logger,
		}

		reqBody := SemanticSearchRequest{
			Query: "test",
			Limit: 10,
		}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/api/v2/search/semantic", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		ctx := context.WithValue(req.Context(), pkgauth.UserEmailKey, "test@example.com")
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()

		handler := SemanticSearchHandler(srv)
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	})

	t.Run("invalid HTTP method returns method not allowed", func(t *testing.T) {
		srv := server.Server{
			SemanticSearch: nil,
			Logger:         logger,
		}

		req := httptest.NewRequest(http.MethodGet, "/api/v2/search/semantic", nil)
		w := httptest.NewRecorder()

		handler := SemanticSearchHandler(srv)
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
	})

}

func TestHybridSearchHandler(t *testing.T) {
	logger := hclog.NewNullLogger()

	t.Run("hybrid search not configured returns service unavailable", func(t *testing.T) {
		srv := server.Server{
			HybridSearch: nil,
			Logger:       logger,
		}

		reqBody := HybridSearchRequest{
			Query: "test",
			Limit: 10,
		}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/api/v2/search/hybrid", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		ctx := context.WithValue(req.Context(), pkgauth.UserEmailKey, "test@example.com")
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()

		handler := HybridSearchHandler(srv)
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	})

	// Note: Query validation tests would require a working HybridSearch service.
	// The handler checks service availability first (correct behavior).

	t.Run("no authentication returns unauthorized", func(t *testing.T) {
		srv := server.Server{
			HybridSearch: nil,
			Logger:       logger,
		}

		reqBody := HybridSearchRequest{
			Query: "test",
			Limit: 10,
		}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/api/v2/search/hybrid", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()

		handler := HybridSearchHandler(srv)
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("invalid HTTP method returns method not allowed", func(t *testing.T) {
		srv := server.Server{
			HybridSearch: nil,
			Logger:       logger,
		}

		req := httptest.NewRequest(http.MethodGet, "/api/v2/search/hybrid", nil)
		w := httptest.NewRecorder()

		handler := HybridSearchHandler(srv)
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
	})
}

func TestSimilarDocumentsHandler(t *testing.T) {
	logger := hclog.NewNullLogger()

	// Note: Path validation tests would require a working SemanticSearch service.
	// The handler checks service availability first (correct behavior).

	t.Run("invalid HTTP method returns method not allowed", func(t *testing.T) {
		srv := server.Server{
			SemanticSearch: nil,
			Logger:         logger,
		}

		req := httptest.NewRequest(http.MethodPost, "/api/v2/documents/doc1/similar", nil)
		w := httptest.NewRecorder()

		handler := SimilarDocumentsHandler(srv)
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
	})

	t.Run("semantic search not configured returns service unavailable", func(t *testing.T) {
		srv := server.Server{
			SemanticSearch: nil,
			Logger:         logger,
		}

		req := httptest.NewRequest(http.MethodGet, "/api/v2/documents/doc1/similar", nil)
		ctx := context.WithValue(req.Context(), pkgauth.UserEmailKey, "test@example.com")
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()

		handler := SimilarDocumentsHandler(srv)
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	})

	t.Run("no authentication returns unauthorized", func(t *testing.T) {
		srv := server.Server{
			SemanticSearch: nil,
			Logger:         logger,
		}

		req := httptest.NewRequest(http.MethodGet, "/api/v2/documents/doc1/similar", nil)
		w := httptest.NewRecorder()

		handler := SimilarDocumentsHandler(srv)
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}
