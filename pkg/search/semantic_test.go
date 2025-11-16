package search

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/hashicorp-forge/hermes/pkg/models"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// MockEmbeddingsGenerator mocks the EmbeddingsGenerator interface.
type MockEmbeddingsGenerator struct {
	mock.Mock
}

func (m *MockEmbeddingsGenerator) GenerateEmbeddings(ctx context.Context, text string, model string, dimensions int) ([]float64, error) {
	args := m.Called(ctx, text, model, dimensions)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]float64), args.Error(1)
}

func setupSemanticSearchTest(t *testing.T) (*gorm.DB, *MockEmbeddingsGenerator) {
	// Setup in-memory SQLite database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// Auto-migrate the schema
	err = db.AutoMigrate(&models.DocumentEmbedding{})
	require.NoError(t, err)

	mockGen := new(MockEmbeddingsGenerator)
	return db, mockGen
}

func TestNewSemanticSearch(t *testing.T) {
	t.Run("valid configuration", func(t *testing.T) {
		db, mockGen := setupSemanticSearchTest(t)

		search, err := NewSemanticSearch(SemanticSearchConfig{
			DB:         db,
			EmbedGen:   mockGen,
			Model:      "text-embedding-3-small",
			Dimensions: 1536,
			Logger:     hclog.NewNullLogger(),
		})

		require.NoError(t, err)
		require.NotNil(t, search)
		assert.Equal(t, "text-embedding-3-small", search.model)
		assert.Equal(t, 1536, search.dimensions)
	})

	t.Run("missing database", func(t *testing.T) {
		_, mockGen := setupSemanticSearchTest(t)

		search, err := NewSemanticSearch(SemanticSearchConfig{
			EmbedGen: mockGen,
		})

		require.Error(t, err)
		assert.Nil(t, search)
		assert.Contains(t, err.Error(), "database connection is required")
	})

	t.Run("missing embeddings generator", func(t *testing.T) {
		db, _ := setupSemanticSearchTest(t)

		search, err := NewSemanticSearch(SemanticSearchConfig{
			DB: db,
		})

		require.Error(t, err)
		assert.Nil(t, search)
		assert.Contains(t, err.Error(), "embeddings generator is required")
	})

	t.Run("default values", func(t *testing.T) {
		db, mockGen := setupSemanticSearchTest(t)

		search, err := NewSemanticSearch(SemanticSearchConfig{
			DB:       db,
			EmbedGen: mockGen,
		})

		require.NoError(t, err)
		assert.Equal(t, "text-embedding-3-small", search.model)
		assert.Equal(t, 1536, search.dimensions)
	})
}

func TestSemanticSearch_FormatVectorForPostgres(t *testing.T) {
	db, mockGen := setupSemanticSearchTest(t)
	search, _ := NewSemanticSearch(SemanticSearchConfig{
		DB:       db,
		EmbedGen: mockGen,
	})

	tests := []struct {
		name     string
		vector   []float64
		expected string
	}{
		{
			name:     "empty vector",
			vector:   []float64{},
			expected: "[]",
		},
		{
			name:     "single value",
			vector:   []float64{0.5},
			expected: "[0.500000]",
		},
		{
			name:     "multiple values",
			vector:   []float64{0.1, 0.2, 0.3},
			expected: "[0.100000,0.200000,0.300000]",
		},
		{
			name:     "negative values",
			vector:   []float64{-0.5, 0.5},
			expected: "[-0.500000,0.500000]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := search.formatVectorForPostgres(tt.vector)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSemanticSearch_GetDocumentEmbedding(t *testing.T) {
	db, mockGen := setupSemanticSearchTest(t)
	search, _ := NewSemanticSearch(SemanticSearchConfig{
		DB:       db,
		EmbedGen: mockGen,
		Model:    "text-embedding-3-small",
	})

	ctx := context.Background()
	docUUID := uuid.New()

	// Create test embedding
	testEmbedding := &models.DocumentEmbedding{
		DocumentID:   "test-doc-123",
		DocumentUUID: &docUUID,
		Embedding:    []float64{0.1, 0.2, 0.3},
		Dimensions:   3,
		Model:        "text-embedding-3-small",
		Provider:     "openai",
	}
	require.NoError(t, db.Create(testEmbedding).Error)

	t.Run("found", func(t *testing.T) {
		embedding, err := search.GetDocumentEmbedding(ctx, "test-doc-123")

		require.NoError(t, err)
		require.NotNil(t, embedding)
		assert.Equal(t, "test-doc-123", embedding.DocumentID)
		assert.Equal(t, []float64{0.1, 0.2, 0.3}, []float64(embedding.Embedding))
	})

	t.Run("not found", func(t *testing.T) {
		embedding, err := search.GetDocumentEmbedding(ctx, "nonexistent")

		require.Error(t, err)
		assert.Nil(t, embedding)
	})
}

func TestSemanticSearch_SearchByEmbedding_Validation(t *testing.T) {
	db, mockGen := setupSemanticSearchTest(t)
	search, _ := NewSemanticSearch(SemanticSearchConfig{
		DB:         db,
		EmbedGen:   mockGen,
		Dimensions: 1536,
	})

	ctx := context.Background()

	t.Run("wrong dimensions", func(t *testing.T) {
		wrongDimEmbedding := make([]float64, 512) // Wrong size

		results, err := search.SearchByEmbedding(ctx, wrongDimEmbedding, 10)

		require.Error(t, err)
		assert.Nil(t, results)
		assert.Contains(t, err.Error(), "embedding dimensions mismatch")
	})

	t.Run("zero limit", func(t *testing.T) {
		embedding := make([]float64, 1536)

		// Should use default limit of 10
		_, err := search.SearchByEmbedding(ctx, embedding, 0)
		// May error on empty DB, but shouldn't error on limit validation
		if err != nil {
			assert.NotContains(t, err.Error(), "limit")
		}
	})
}

func TestSemanticSearch_Search_EmptyQuery(t *testing.T) {
	db, mockGen := setupSemanticSearchTest(t)
	search, _ := NewSemanticSearch(SemanticSearchConfig{
		DB:       db,
		EmbedGen: mockGen,
	})

	ctx := context.Background()

	results, err := search.Search(ctx, "", 10)

	require.Error(t, err)
	assert.Nil(t, results)
	assert.Contains(t, err.Error(), "query cannot be empty")
}

func TestSemanticSearch_SearchWithFilters_Validation(t *testing.T) {
	db, mockGen := setupSemanticSearchTest(t)
	search, _ := NewSemanticSearch(SemanticSearchConfig{
		DB:       db,
		EmbedGen: mockGen,
	})

	ctx := context.Background()

	t.Run("empty query", func(t *testing.T) {
		filter := SearchFilter{
			MinSimilarity: 0.5,
		}

		results, err := search.SearchWithFilters(ctx, "", 10, filter)

		require.Error(t, err)
		assert.Nil(t, results)
		assert.Contains(t, err.Error(), "query cannot be empty")
	})
}

// Note: Full integration tests with actual pgvector queries require PostgreSQL
// with pgvector extension. These tests focus on validation and structure.
// See tests/integration/ for full database integration tests.

func TestSemanticSearchResult_Structure(t *testing.T) {
	result := SemanticSearchResult{
		DocumentID: "doc-123",
		Similarity: 0.95,
	}

	assert.Equal(t, "doc-123", result.DocumentID)
	assert.Equal(t, 0.95, result.Similarity)
}

func TestSearchFilter_Structure(t *testing.T) {
	filter := SearchFilter{
		DocumentTypes: []string{"RFC", "PRD"},
		DocumentIDs:   []string{"doc-1", "doc-2"},
		MinSimilarity: 0.8,
	}

	assert.Equal(t, 2, len(filter.DocumentTypes))
	assert.Equal(t, 0.8, filter.MinSimilarity)
}
