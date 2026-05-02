package qdrant

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp-forge/hermes/pkg/search"
)

func TestNewRequiresBaseURL(t *testing.T) {
	_, err := New(Config{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestName(t *testing.T) {
	adapter, err := New(Config{BaseURL: "http://127.0.0.1:6333"})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	if adapter.Name() != string(search.ProviderTypeQdrant) {
		t.Fatalf("Name = %q", adapter.Name())
	}
}

func TestEnsureCollection(t *testing.T) {
	var got map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/collections/test" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		_, _ = w.Write([]byte(`{"result":true}`))
	}))
	defer server.Close()

	adapter := newTestAdapter(t, server.URL)
	if err := adapter.EnsureCollection(t.Context(), 3); err != nil {
		t.Fatalf("EnsureCollection returned error: %v", err)
	}
	vectors := got["vectors"].(map[string]any)
	if vectors["distance"] != defaultDistance || int(vectors["size"].(float64)) != 3 {
		t.Fatalf("unexpected vectors config: %#v", vectors)
	}
}

func TestIndexEmbeddingBatch(t *testing.T) {
	var got map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/collections/test/points" || r.URL.Query().Get("wait") != "true" {
			t.Fatalf("unexpected request %s %s?%s", r.Method, r.URL.Path, r.URL.RawQuery)
		}
		if r.Header.Get("api-key") != "test-key" {
			t.Fatalf("missing api-key header")
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		_, _ = w.Write([]byte(`{"result":true}`))
	}))
	defer server.Close()

	adapter := newTestAdapter(t, server.URL)
	doc := &search.VectorDocument{
		DocID:            "doc-1",
		ObjectID:         "object-1",
		Title:            "Doc One",
		ContentEmbedding: []float32{0.1, 0.2, 0.3},
		Dimensions:       3,
		Model:            "test-model",
		EmbeddedAt:       time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
	}
	if err := adapter.IndexEmbeddingBatch(t.Context(), []*search.VectorDocument{doc}); err != nil {
		t.Fatalf("IndexEmbeddingBatch returned error: %v", err)
	}
	points := got["points"].([]any)
	if len(points) != 1 {
		t.Fatalf("expected one point, got %#v", points)
	}
	payload := points[0].(map[string]any)["payload"].(map[string]any)
	if payload["doc_id"] != "doc-1" || payload["title"] != "Doc One" {
		t.Fatalf("unexpected payload: %#v", payload)
	}
}

func TestSearchSimilar(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/collections/test/points/search" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		var got map[string]any
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if int(got["limit"].(float64)) != 2 || got["filter"] == nil {
			t.Fatalf("unexpected search request: %#v", got)
		}
		_, _ = w.Write([]byte(`{
        "result": [{
          "id": "point-1",
          "score": 0.91,
          "vector": [0.1, 0.2, 0.3],
          "payload": {"doc_id":"doc-1","object_id":"object-1","title":"Doc One","model":"test-model","dimensions":3,"topics":["edge"]}
        }]
      }`))
	}))
	defer server.Close()

	adapter := newTestAdapter(t, server.URL)
	result, err := adapter.SearchSimilar(t.Context(), &search.VectorSearchQuery{QueryEmbedding: []float32{0.1, 0.2, 0.3}, Limit: 2, Filters: map[string]any{"project": "docs"}})
	if err != nil {
		t.Fatalf("SearchSimilar returned error: %v", err)
	}
	if result.Total != 1 || result.Hits[0].Score != 0.91 || result.Hits[0].Document.DocID != "doc-1" {
		t.Fatalf("unexpected result: %#v", result)
	}
	if len(result.Hits[0].Document.Topics) != 1 || result.Hits[0].Document.Topics[0] != "edge" {
		t.Fatalf("unexpected topics: %#v", result.Hits[0].Document.Topics)
	}
}

func TestGetEmbeddingNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"result":[]}`))
	}))
	defer server.Close()

	adapter := newTestAdapter(t, server.URL)
	_, err := adapter.GetEmbedding(t.Context(), "missing")
	if !errors.Is(err, search.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestDeleteBatchAndClear(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.Method+" "+r.URL.String())
		_, _ = w.Write([]byte(`{"result":true}`))
	}))
	defer server.Close()

	adapter := newTestAdapter(t, server.URL)
	if err := adapter.DeleteBatch(t.Context(), []string{"doc-1", "doc-2"}); err != nil {
		t.Fatalf("DeleteBatch returned error: %v", err)
	}
	if err := adapter.Clear(t.Context()); err != nil {
		t.Fatalf("Clear returned error: %v", err)
	}
	joined := strings.Join(paths, "\n")
	if !strings.Contains(joined, "POST /collections/test/points/delete?wait=true") || !strings.Contains(joined, "DELETE /collections/test") {
		t.Fatalf("unexpected requests:\n%s", joined)
	}
}

func TestDimensionValidation(t *testing.T) {
	adapter, err := New(Config{BaseURL: "http://127.0.0.1:6333", Dimensions: 3})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	err = adapter.IndexEmbedding(t.Context(), &search.VectorDocument{DocID: "doc-1", ContentEmbedding: []float32{1, 2}})
	if !errors.Is(err, search.ErrInvalidQuery) {
		t.Fatalf("expected ErrInvalidQuery, got %v", err)
	}
}

func newTestAdapter(t *testing.T, baseURL string) *Adapter {
	t.Helper()
	adapter, err := New(Config{BaseURL: baseURL, Collection: "test", APIKey: "test-key", Dimensions: 3})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	return adapter
}
