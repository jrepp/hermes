package edge

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp-forge/hermes/pkg/llm"
	"github.com/hashicorp-forge/hermes/pkg/search"
	"github.com/hashicorp-forge/hermes/pkg/search/adapters/qdrant"
)

const defaultVectorLimit = 10

// EmbeddingsGenerator generates embeddings for vector indexing and querying.
type EmbeddingsGenerator interface {
	GenerateEmbeddings(ctx context.Context, text string, model string, dimensions int) ([]float64, error)
}

// VectorIndex is the vector search provider boundary used by edge search.
type VectorIndex interface {
	search.VectorIndex
}

type collectionEnsurer interface {
	EnsureCollection(ctx context.Context, dimensions int) error
}

// SearchVector builds an opt-in local vector index and searches it.
func SearchVector(ctx context.Context, opts Options, query string, limit int) (SearchResult, error) {
	if query == "" {
		return SearchResult{}, fmt.Errorf("search query is required")
	}
	if limit <= 0 {
		limit = defaultVectorLimit
	}
	vectorIndex, embedder, err := resolveVectorDependencies(opts)
	if err != nil {
		return SearchResult{}, err
	}
	return searchVectorWithDependencies(ctx, opts, vectorIndex, embedder, query, limit)
}

// SearchHybrid combines BM25 and opt-in vector results when vector dependencies are configured.
func SearchHybrid(ctx context.Context, opts Options, query string, limit int, debug bool) (SearchResult, error) {
	bm25Result, err := SearchLocal(opts, query, limit, debug)
	if err != nil {
		return SearchResult{}, err
	}
	bm25Result.Mode = "hybrid"
	vectorIndex, embedder, err := resolveVectorDependencies(opts)
	if err != nil {
		bm25Result.Warnings = append(bm25Result.Warnings, "vector search unavailable: "+err.Error())
		return bm25Result, nil
	}
	vectorResult, err := searchVectorWithDependencies(ctx, opts, vectorIndex, embedder, query, limit)
	if err != nil {
		bm25Result.Warnings = append(bm25Result.Warnings, "vector search unavailable: "+err.Error())
		return bm25Result, nil
	}
	return fuseHybrid(query, bm25Result, vectorResult, limit, debug), nil
}

// BuildVectorIndex indexes discovered local documents into the vector provider.
func BuildVectorIndex(ctx context.Context, opts Options, vectorIndex VectorIndex, embedder EmbeddingsGenerator) (IndexResult, error) {
	if vectorIndex == nil {
		return IndexResult{}, fmt.Errorf("vector index is required")
	}
	if embedder == nil {
		return IndexResult{}, fmt.Errorf("embedding generator is required")
	}
	config, docs, err := load(opts)
	if err != nil {
		return IndexResult{}, err
	}
	dimensions := vectorDimensions(opts)
	if ensurer, ok := vectorIndex.(collectionEnsurer); ok {
		if err := ensurer.EnsureCollection(ctx, dimensions); err != nil {
			return IndexResult{}, err
		}
	}

	result := IndexResult{IndexedAt: time.Now().UTC()}
	for _, doc := range docs {
		indexed, searchDoc, err := buildIndexDocument(config, doc)
		if err != nil {
			result.Skipped = append(result.Skipped, IndexSkip{Path: doc.RelPath, Reason: err.Error()})
			continue
		}
		embedding, err := embedder.GenerateEmbeddings(ctx, searchDoc.Title+"\n"+searchDoc.Content, vectorModel(opts), dimensions)
		if err != nil {
			result.Skipped = append(result.Skipped, IndexSkip{Path: doc.RelPath, Reason: err.Error()})
			continue
		}
		vectorDoc := &search.VectorDocument{DocID: indexed.ID, ObjectID: indexed.Path, Title: searchDoc.Title, DocType: doc.Schema, Model: vectorModel(opts), Dimensions: dimensions, EmbeddedAt: result.IndexedAt, ContentEmbedding: float64ToFloat32(embedding)}
		if err := vectorIndex.IndexEmbedding(ctx, vectorDoc); err != nil {
			result.Skipped = append(result.Skipped, IndexSkip{Path: doc.RelPath, Reason: err.Error()})
			continue
		}
		result.Documents = append(result.Documents, indexed)
	}
	return result, nil
}

func searchVectorWithDependencies(ctx context.Context, opts Options, vectorIndex VectorIndex, embedder EmbeddingsGenerator, query string, limit int) (SearchResult, error) {
	indexResult, err := BuildVectorIndex(ctx, opts, vectorIndex, embedder)
	if err != nil {
		return SearchResult{}, err
	}
	queryEmbedding, err := embedder.GenerateEmbeddings(ctx, query, vectorModel(opts), vectorDimensions(opts))
	if err != nil {
		return SearchResult{}, fmt.Errorf("generate query embedding: %w", err)
	}
	result, err := vectorIndex.SearchSimilar(ctx, &search.VectorSearchQuery{QueryEmbedding: float64ToFloat32(queryEmbedding), Limit: limit})
	if err != nil {
		return SearchResult{}, err
	}
	hits := make([]SearchHit, 0, len(result.Hits))
	for _, hit := range result.Hits {
		doc := hit.Document
		if doc == nil {
			continue
		}
		hits = append(hits, SearchHit{Path: doc.ObjectID, Title: doc.Title, Project: opts.Project, Score: hit.Score, Debug: map[string]any{"score_components": "vector", "document_id": doc.DocID}})
	}
	return SearchResult{Query: query, Mode: "vector", Hits: hits, Indexed: len(indexResult.Documents)}, nil
}

func fuseHybrid(query string, bm25Result, vectorResult SearchResult, limit int, debug bool) SearchResult {
	if limit <= 0 {
		limit = defaultVectorLimit
	}
	type scored struct {
		hit         SearchHit
		bm25Score   float64
		vectorScore float64
	}
	merged := make(map[string]*scored)
	for _, hit := range bm25Result.Hits {
		key := hit.Path
		merged[key] = &scored{hit: hit, bm25Score: hit.Score}
	}
	for _, hit := range vectorResult.Hits {
		key := hit.Path
		entry := merged[key]
		if entry == nil {
			entry = &scored{hit: hit}
			merged[key] = entry
		}
		entry.vectorScore = hit.Score
		if entry.hit.Title == "" {
			entry.hit.Title = hit.Title
		}
	}
	hits := make([]SearchHit, 0, len(merged))
	for _, entry := range merged {
		entry.hit.Score = entry.bm25Score + entry.vectorScore
		if debug {
			entry.hit.Debug = map[string]any{"bm25_score": entry.bm25Score, "vector_score": entry.vectorScore, "score_components": "bm25+vector"}
		}
		hits = append(hits, entry.hit)
	}
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].Score != hits[j].Score {
			return hits[i].Score > hits[j].Score
		}
		return hits[i].Path < hits[j].Path
	})
	if len(hits) > limit {
		hits = hits[:limit]
	}
	return SearchResult{Query: query, Mode: "hybrid", Hits: hits, Indexed: bm25Result.Indexed, Warnings: append(bm25Result.Warnings, vectorResult.Warnings...)}
}

func resolveVectorDependencies(opts Options) (VectorIndex, EmbeddingsGenerator, error) {
	vectorIndex := opts.VectorIndex
	if vectorIndex == nil {
		adapter, err := newQdrantIndex(opts)
		if err != nil {
			return nil, nil, err
		}
		vectorIndex = adapter
	}
	embedder := opts.EmbeddingsGenerator
	if embedder == nil {
		client, err := newEmbeddingGenerator(opts)
		if err != nil {
			return nil, nil, err
		}
		embedder = client
	}
	return vectorIndex, embedder, nil
}

func newQdrantIndex(opts Options) (*qdrant.Adapter, error) {
	url := firstNonEmpty(opts.QdrantURL, os.Getenv("HERMES_QDRANT_URL"), os.Getenv("HERMES_TEST_QDRANT_URL"))
	if url == "" {
		return nil, fmt.Errorf("qdrant URL is required for vector search")
	}
	return qdrant.New(qdrant.Config{BaseURL: url, APIKey: firstNonEmpty(opts.QdrantAPIKey, os.Getenv("HERMES_QDRANT_API_KEY"), os.Getenv("HERMES_TEST_QDRANT_API_KEY")), Collection: firstNonEmpty(opts.QdrantCollection, os.Getenv("HERMES_QDRANT_COLLECTION")), Dimensions: vectorDimensions(opts), Timeout: opts.Timeout})
}

func newEmbeddingGenerator(opts Options) (EmbeddingsGenerator, error) {
	model := vectorModel(opts)
	if model == "" {
		return nil, fmt.Errorf("embedding model is required for vector search")
	}
	factory := llm.NewClientFactory(llm.ClientFactoryConfig{Logger: hclog.NewNullLogger(), OpenAIAPIKey: firstNonEmpty(opts.OpenAIAPIKey, os.Getenv("OPENAI_API_KEY")), OllamaURL: firstNonEmpty(opts.OllamaURL, os.Getenv("OLLAMA_URL"), os.Getenv("HERMES_OLLAMA_URL"))})
	client, err := factory.GetClient(context.Background(), model)
	if err != nil {
		return nil, err
	}
	embedder, ok := client.(EmbeddingsGenerator)
	if !ok {
		return nil, fmt.Errorf("model %q provider does not support embeddings", model)
	}
	return embedder, nil
}

func vectorModel(opts Options) string {
	return firstNonEmpty(opts.EmbeddingModel, os.Getenv("HERMES_EMBEDDING_MODEL"))
}

func vectorDimensions(opts Options) int {
	if opts.EmbeddingDimensions > 0 {
		return opts.EmbeddingDimensions
	}
	if env := os.Getenv("HERMES_EMBEDDING_DIMENSIONS"); env != "" {
		var dimensions int
		_, _ = fmt.Sscanf(env, "%d", &dimensions)
		if dimensions > 0 {
			return dimensions
		}
	}
	return 768
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func float64ToFloat32(values []float64) []float32 {
	converted := make([]float32, len(values))
	for i, value := range values {
		converted[i] = float32(value)
	}
	return converted
}
