package search

import (
	"context"
	"fmt"
	"math/rand"
	"testing"

	"github.com/hashicorp-forge/hermes/pkg/models"
	"github.com/hashicorp/go-hclog"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// BenchmarkEmbeddingsGenerator is a mock for benchmarking without external API calls
type BenchmarkEmbeddingsGenerator struct {
	dimensions int
}

func (m *BenchmarkEmbeddingsGenerator) GenerateEmbeddings(ctx context.Context, text string, model string, dimensions int) ([]float64, error) {
	// Generate deterministic embeddings based on text length
	// This allows consistent benchmarking without external API calls
	embedding := make([]float64, dimensions)
	seed := int64(len(text))
	rng := rand.New(rand.NewSource(seed))

	for i := range embedding {
		embedding[i] = rng.Float64()*2 - 1 // Range: -1 to 1
	}

	// Normalize the vector
	var magnitude float64
	for _, v := range embedding {
		magnitude += v * v
	}
	magnitude = 1.0 // Skip actual sqrt for benchmark speed

	for i := range embedding {
		embedding[i] /= magnitude
	}

	return embedding, nil
}

// setupBenchmarkDB creates an in-memory SQLite database with test data
func setupBenchmarkDB(b *testing.B, docCount int) (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: nil, // Disable logging for benchmarks
	})
	if err != nil {
		return nil, err
	}

	// Auto-migrate schema
	if err := db.AutoMigrate(&models.Document{}, &models.DocumentEmbedding{}); err != nil {
		return nil, err
	}

	// Insert test documents
	rng := rand.New(rand.NewSource(42)) // Deterministic seed
	for i := 0; i < docCount; i++ {
		doc := models.Document{
			GoogleFileID: fmt.Sprintf("doc-%d", i),
			Title:        fmt.Sprintf("Test Document %d", i),
		}
		if err := db.Create(&doc).Error; err != nil {
			return nil, err
		}

		// Create embeddings for each document
		embedding := models.DocumentEmbedding{
			DocumentID:  fmt.Sprintf("doc-%d", i),
			RevisionID:  nil,
			ChunkIndex:  intPtr(0),
			ChunkText:   fmt.Sprintf("This is test document number %d with some content.", i),
			Model:       "test-model",
			Provider:    "test",
			Dimensions:  1536,
			ContentHash: fmt.Sprintf("hash-%d", i),
		}

		// Generate random embedding vector
		vec := make([]float64, 1536)
		for j := range vec {
			vec[j] = rng.Float64()*2 - 1
		}
		embedding.Embedding = vec

		if err := db.Create(&embedding).Error; err != nil {
			return nil, err
		}
	}

	return db, nil
}

// intPtr is a helper to create int pointers
func intPtr(i int) *int {
	return &i
}

// BenchmarkEmbeddingGeneration benchmarks the embedding generation step
func BenchmarkEmbeddingGeneration(b *testing.B) {
	mockGen := &BenchmarkEmbeddingsGenerator{dimensions: 1536}
	ctx := context.Background()

	queries := []string{
		"short query",
		"This is a medium length query with more words to process",
		"This is a longer query that contains significantly more text and should take longer to process into embeddings because it has many more tokens that need to be encoded and transformed into vector representations",
	}

	for _, query := range queries {
		b.Run(fmt.Sprintf("length-%d", len(query)), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := mockGen.GenerateEmbeddings(ctx, query, "test-model", 1536)
				if err != nil {
					b.Fatal(err)
				}
			}
			b.ReportMetric(float64(len(query)), "query_length")
		})
	}
}

// BenchmarkSemanticSearch_VaryingCorpusSize benchmarks search with different document counts
func BenchmarkSemanticSearch_VaryingCorpusSize(b *testing.B) {
	corpusSizes := []int{100, 1000, 5000}
	mockGen := &BenchmarkEmbeddingsGenerator{dimensions: 1536}
	ctx := context.Background()
	logger := hclog.NewNullLogger()

	for _, size := range corpusSizes {
		b.Run(fmt.Sprintf("docs-%d", size), func(b *testing.B) {
			// Setup database with test data
			db, err := setupBenchmarkDB(b, size)
			if err != nil {
				b.Fatal(err)
			}

			search, err := NewSemanticSearch(SemanticSearchConfig{
				DB:         db,
				EmbedGen:   mockGen,
				Model:      "test-model",
				Dimensions: 1536,
				Logger:     logger,
			})
			if err != nil {
				b.Fatal(err)
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := search.Search(ctx, "test query", 10)
				if err != nil {
					b.Fatal(err)
				}
			}
			b.ReportMetric(float64(size), "corpus_size")
		})
	}
}

// BenchmarkSemanticSearch_VaryingLimits benchmarks search with different result limits
func BenchmarkSemanticSearch_VaryingLimits(b *testing.B) {
	limits := []int{5, 10, 25, 50, 100}
	mockGen := &BenchmarkEmbeddingsGenerator{dimensions: 1536}
	ctx := context.Background()
	logger := hclog.NewNullLogger()

	// Setup database once
	db, err := setupBenchmarkDB(b, 1000)
	if err != nil {
		b.Fatal(err)
	}

	search, err := NewSemanticSearch(SemanticSearchConfig{
		DB:         db,
		EmbedGen:   mockGen,
		Model:      "test-model",
		Dimensions: 1536,
		Logger:     logger,
	})
	if err != nil {
		b.Fatal(err)
	}

	for _, limit := range limits {
		b.Run(fmt.Sprintf("limit-%d", limit), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				results, err := search.Search(ctx, "test query", limit)
				if err != nil {
					b.Fatal(err)
				}
				if len(results) == 0 {
					b.Fatal("expected results")
				}
			}
			b.ReportMetric(float64(limit), "result_limit")
		})
	}
}

// BenchmarkSemanticSearch_WithFilters benchmarks filtered search
func BenchmarkSemanticSearch_WithFilters(b *testing.B) {
	mockGen := &BenchmarkEmbeddingsGenerator{dimensions: 1536}
	ctx := context.Background()
	logger := hclog.NewNullLogger()

	db, err := setupBenchmarkDB(b, 1000)
	if err != nil {
		b.Fatal(err)
	}

	search, err := NewSemanticSearch(SemanticSearchConfig{
		DB:         db,
		EmbedGen:   mockGen,
		Model:      "test-model",
		Dimensions: 1536,
		Logger:     logger,
	})
	if err != nil {
		b.Fatal(err)
	}

	b.Run("no-filters", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := search.Search(ctx, "test query", 10)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("with-doc-id-filter", func(b *testing.B) {
		filter := SearchFilter{
			DocumentIDs: []string{"doc-1", "doc-2", "doc-3", "doc-4", "doc-5"},
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := search.SearchWithFilters(ctx, "test query", 10, filter)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("with-similarity-threshold", func(b *testing.B) {
		filter := SearchFilter{
			MinSimilarity: 0.7,
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := search.SearchWithFilters(ctx, "test query", 10, filter)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkSemanticSearch_FindSimilar benchmarks finding similar documents
func BenchmarkSemanticSearch_FindSimilar(b *testing.B) {
	mockGen := &BenchmarkEmbeddingsGenerator{dimensions: 1536}
	ctx := context.Background()
	logger := hclog.NewNullLogger()

	db, err := setupBenchmarkDB(b, 1000)
	if err != nil {
		b.Fatal(err)
	}

	search, err := NewSemanticSearch(SemanticSearchConfig{
		DB:         db,
		EmbedGen:   mockGen,
		Model:      "test-model",
		Dimensions: 1536,
		Logger:     logger,
	})
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := search.FindSimilarDocuments(ctx, "doc-50", 10)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkSemanticSearch_ConcurrentQueries benchmarks concurrent search requests
func BenchmarkSemanticSearch_ConcurrentQueries(b *testing.B) {
	mockGen := &BenchmarkEmbeddingsGenerator{dimensions: 1536}
	logger := hclog.NewNullLogger()

	db, err := setupBenchmarkDB(b, 1000)
	if err != nil {
		b.Fatal(err)
	}

	search, err := NewSemanticSearch(SemanticSearchConfig{
		DB:         db,
		EmbedGen:   mockGen,
		Model:      "test-model",
		Dimensions: 1536,
		Logger:     logger,
	})
	if err != nil {
		b.Fatal(err)
	}

	queries := []string{
		"machine learning algorithms",
		"database optimization techniques",
		"cloud infrastructure patterns",
		"security best practices",
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		ctx := context.Background()
		i := 0
		for pb.Next() {
			query := queries[i%len(queries)]
			_, err := search.Search(ctx, query, 10)
			if err != nil {
				b.Fatal(err)
			}
			i++
		}
	})
}

// BenchmarkVectorOperations benchmarks low-level vector operations
func BenchmarkVectorOperations(b *testing.B) {
	b.Run("vector-generation-1536d", func(b *testing.B) {
		rng := rand.New(rand.NewSource(42))
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			vec := make([]float64, 1536)
			for j := range vec {
				vec[j] = rng.Float64()*2 - 1
			}
		}
	})

	b.Run("cosine-similarity-1536d", func(b *testing.B) {
		rng := rand.New(rand.NewSource(42))
		vec1 := make([]float64, 1536)
		vec2 := make([]float64, 1536)
		for i := range vec1 {
			vec1[i] = rng.Float64()
			vec2[i] = rng.Float64()
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			var dotProduct, mag1, mag2 float64
			for j := range vec1 {
				dotProduct += vec1[j] * vec2[j]
				mag1 += vec1[j] * vec1[j]
				mag2 += vec2[j] * vec2[j]
			}
			_ = dotProduct / (mag1 * mag2)
		}
	})
}
