package edge

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp-forge/hermes/pkg/search"
)

type fakeEmbedder struct{}

func (fakeEmbedder) GenerateEmbeddings(_ context.Context, text, _ string, dimensions int) ([]float64, error) {
	if dimensions <= 0 {
		dimensions = 3
	}
	values := make([]float64, dimensions)
	values[0] = float64(len(text) % 10)
	return values, nil
}

type fakeVectorIndex struct {
	docs       []*search.VectorDocument
	ensuredDim int
}

func (f *fakeVectorIndex) EnsureCollection(_ context.Context, dimensions int) error {
	f.ensuredDim = dimensions
	return nil
}

func (f *fakeVectorIndex) IndexEmbedding(_ context.Context, doc *search.VectorDocument) error {
	f.docs = append(f.docs, doc)
	return nil
}

func (f *fakeVectorIndex) IndexEmbeddingBatch(ctx context.Context, docs []*search.VectorDocument) error {
	for _, doc := range docs {
		if err := f.IndexEmbedding(ctx, doc); err != nil {
			return err
		}
	}
	return nil
}

func (f *fakeVectorIndex) SearchSimilar(_ context.Context, _ *search.VectorSearchQuery) (*search.VectorSearchResult, error) {
	hits := make([]search.VectorHit, 0, len(f.docs))
	for i, doc := range f.docs {
		hits = append(hits, search.VectorHit{Document: doc, Score: 1 - float64(i)*0.1})
	}
	return &search.VectorSearchResult{Hits: hits, Total: len(hits)}, nil
}

func (f *fakeVectorIndex) SearchHybrid(_ context.Context, _ *search.HybridSearchQuery) (*search.SearchResult, error) {
	return nil, fmt.Errorf("not implemented")
}

func (f *fakeVectorIndex) Delete(_ context.Context, _ string) error { return nil }

func (f *fakeVectorIndex) DeleteBatch(_ context.Context, _ []string) error { return nil }

func (f *fakeVectorIndex) GetEmbedding(_ context.Context, _ string) (*search.VectorDocument, error) {
	return nil, search.ErrNotFound
}

func (f *fakeVectorIndex) Clear(_ context.Context) error { return nil }

func TestSearchVectorWithInjectedDependencies(t *testing.T) {
	idx := &fakeVectorIndex{}
	opts := testOptions(false)
	opts.VectorIndex = idx
	opts.EmbeddingsGenerator = fakeEmbedder{}
	opts.EmbeddingModel = "test-model"
	opts.EmbeddingDimensions = 3

	result, err := SearchVector(t.Context(), opts, "clean", 2)
	if err != nil {
		t.Fatalf("SearchVector returned error: %v", err)
	}
	if result.Mode != "vector" || result.Indexed != 3 || len(result.Hits) != 3 {
		t.Fatalf("unexpected vector result: %#v", result)
	}
	if idx.ensuredDim != 3 {
		t.Fatalf("expected collection dimensions 3, got %d", idx.ensuredDim)
	}
}

func TestSearchVectorRequiresConfiguration(t *testing.T) {
	_, err := SearchVector(t.Context(), testOptions(false), "clean", 10)
	if err == nil {
		t.Fatal("expected configuration error")
	}
}

func TestSearchHybridFallsBackToBM25(t *testing.T) {
	result, err := SearchHybrid(t.Context(), testOptions(false), "clean", 10, true)
	if err != nil {
		t.Fatalf("SearchHybrid returned error: %v", err)
	}
	if result.Mode != "hybrid" || len(result.Hits) == 0 || len(result.Warnings) == 0 {
		t.Fatalf("expected BM25 fallback with warning, got %#v", result)
	}
	if result.Hits[0].Path != "docs/clean.md" {
		t.Fatalf("expected clean BM25 hit, got %#v", result.Hits)
	}
}

func TestSearchHybridFusesVectorResults(t *testing.T) {
	idx := &fakeVectorIndex{}
	opts := testOptions(false)
	opts.VectorIndex = idx
	opts.EmbeddingsGenerator = fakeEmbedder{}
	opts.EmbeddingModel = "test-model"
	opts.EmbeddingDimensions = 3

	result, err := SearchHybrid(t.Context(), opts, "clean", 10, true)
	if err != nil {
		t.Fatalf("SearchHybrid returned error: %v", err)
	}
	if result.Mode != "hybrid" || len(result.Warnings) != 0 || len(result.Hits) == 0 {
		t.Fatalf("unexpected hybrid result: %#v", result)
	}
	if result.Hits[0].Debug == nil || result.Hits[0].Debug["score_components"] != "bm25+vector" {
		t.Fatalf("expected fused debug output, got %#v", result.Hits[0].Debug)
	}
}
