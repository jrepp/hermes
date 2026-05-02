package qdrant

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/hashicorp-forge/hermes/pkg/search"
)

func TestQdrantLiveRoundTrip(t *testing.T) {
	url := os.Getenv("HERMES_TEST_QDRANT_URL")
	if url == "" {
		t.Skip("set HERMES_TEST_QDRANT_URL to run Qdrant integration test")
	}
	collection := fmt.Sprintf("hermes_test_qdrant_%d", time.Now().UnixNano())
	adapter, err := New(Config{BaseURL: url, APIKey: os.Getenv("HERMES_TEST_QDRANT_API_KEY"), Collection: collection, Dimensions: 3, Timeout: 2 * time.Second})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := adapter.Healthy(ctx); err != nil {
		t.Fatalf("Healthy returned error: %v", err)
	}
	if err := adapter.EnsureCollection(ctx, 3); err != nil {
		t.Fatalf("EnsureCollection returned error: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cleanupCancel()
		_ = adapter.Clear(cleanupCtx)
	})

	doc := &search.VectorDocument{
		DocID:            "live-doc-1",
		ObjectID:         "live-object-1",
		Title:            "Live Qdrant smoke document",
		ContentEmbedding: []float32{0.1, 0.2, 0.3},
		Dimensions:       3,
		Model:            "test-model",
		EmbeddedAt:       time.Now().UTC(),
	}
	if err := adapter.IndexEmbedding(ctx, doc); err != nil {
		t.Fatalf("IndexEmbedding returned error: %v", err)
	}
	result, err := adapter.SearchSimilar(ctx, &search.VectorSearchQuery{QueryEmbedding: []float32{0.1, 0.2, 0.3}, Limit: 1})
	if err != nil {
		t.Fatalf("SearchSimilar returned error: %v", err)
	}
	if result.Total == 0 || result.Hits[0].Document.DocID != doc.DocID {
		t.Fatalf("unexpected search result: %#v", result)
	}
	stored, err := adapter.GetEmbedding(ctx, doc.DocID)
	if err != nil {
		t.Fatalf("GetEmbedding returned error: %v", err)
	}
	if stored.DocID != doc.DocID || stored.Title != doc.Title {
		t.Fatalf("unexpected stored document: %#v", stored)
	}
	if err := adapter.Delete(ctx, doc.DocID); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
}
