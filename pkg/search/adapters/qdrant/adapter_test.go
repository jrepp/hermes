package qdrant

import (
	"testing"

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

func TestSearchNotImplemented(t *testing.T) {
	adapter, err := New(Config{BaseURL: "http://127.0.0.1:6333"})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	results, err := adapter.Search(t.Context(), "docs", []float32{1, 2, 3}, 10)
	if err == nil {
		t.Fatal("expected error")
	}
	if results != nil {
		t.Fatalf("expected nil results, got %#v", results)
	}
}
