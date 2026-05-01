package qdrant

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestQdrantHealthyLive(t *testing.T) {
	url := os.Getenv("HERMES_TEST_QDRANT_URL")
	if url == "" {
		t.Skip("set HERMES_TEST_QDRANT_URL to run Qdrant integration test")
	}
	adapter, err := New(Config{BaseURL: url, APIKey: os.Getenv("HERMES_TEST_QDRANT_API_KEY"), Timeout: 2 * time.Second})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := adapter.Healthy(ctx); err != nil {
		t.Fatalf("Healthy returned error: %v", err)
	}
}
