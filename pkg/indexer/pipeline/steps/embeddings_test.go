package steps

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/hashicorp-forge/hermes/pkg/models"
)

// MockEmbeddingsClient mocks the EmbeddingsClient interface.
type MockEmbeddingsClient struct {
	mock.Mock
}

func (m *MockEmbeddingsClient) GenerateEmbeddings(ctx context.Context, text string, model string, dimensions int) ([]float64, error) {
	args := m.Called(ctx, text, model, dimensions)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]float64), args.Error(1)
}

func (m *MockEmbeddingsClient) GenerateEmbeddingsBatch(ctx context.Context, texts []string, model string, dimensions int) ([][]float64, error) {
	args := m.Called(ctx, texts, model, dimensions)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	// Support both direct return and function return
	result := args.Get(0)
	if fn, ok := result.(func(context.Context, []string, string, int) [][]float64); ok {
		return fn(ctx, texts, model, dimensions), args.Error(1)
	}
	return result.([][]float64), args.Error(1)
}

func setupEmbeddingsTest(t *testing.T) (*gorm.DB, *MockEmbeddingsClient, *MockWorkspaceProvider) {
	// Setup in-memory SQLite database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// Auto-migrate the schema
	err = db.AutoMigrate(&models.DocumentEmbedding{})
	require.NoError(t, err)

	mockClient := new(MockEmbeddingsClient)
	mockProvider := &MockWorkspaceProvider{
		Content: make(map[string]string),
	}

	return db, mockClient, mockProvider
}

func TestEmbeddingsStep_Name(t *testing.T) {
	db, mockClient, mockProvider := setupEmbeddingsTest(t)
	step := NewEmbeddingsStep(db, mockClient, mockProvider, hclog.NewNullLogger())

	assert.Equal(t, "embeddings", step.Name())
}

func TestEmbeddingsStep_Execute_Success(t *testing.T) {
	db, mockClient, mockProvider := setupEmbeddingsTest(t)

	docUUID := uuid.New()
	revision := &models.DocumentRevision{
		ID:           1,
		DocumentID:   "test-doc-123",
		DocumentUUID: docUUID,
		ContentHash:  "abc123",
	}

	config := map[string]interface{}{
		"model":      "text-embedding-3-small",
		"dimensions": 1536,
		"provider":   "openai",
	}

	testContent := "This is a test document for embeddings generation."
	testEmbedding := make([]float64, 1536)
	for i := range testEmbedding {
		testEmbedding[i] = float64(i) * 0.001
	}

	// Setup mocks
	mockProvider.Content["test-doc-123"] = testContent
	mockClient.On("GenerateEmbeddings",
		mock.Anything,
		mock.AnythingOfType("string"),
		"text-embedding-3-small",
		1536,
	).Return(testEmbedding, nil)

	// Execute step
	step := NewEmbeddingsStep(db, mockClient, mockProvider, hclog.NewNullLogger())
	err := step.Execute(context.Background(), revision, config)

	require.NoError(t, err)
	mockClient.AssertExpectations(t)

	// Verify embedding was saved
	var embedding models.DocumentEmbedding
	err = db.Where("document_id = ?", "test-doc-123").First(&embedding).Error
	require.NoError(t, err)

	assert.Equal(t, "test-doc-123", embedding.DocumentID)
	assert.NotNil(t, embedding.DocumentUUID)
	assert.Equal(t, docUUID, *embedding.DocumentUUID)
	assert.Equal(t, 1536, embedding.Dimensions)
	assert.Equal(t, "text-embedding-3-small", embedding.Model)
	assert.Equal(t, "openai", embedding.Provider)
	assert.Equal(t, "abc123", embedding.ContentHash)
	assert.Equal(t, 1536, len(embedding.Embedding))
}

func TestEmbeddingsStep_Execute_Idempotency(t *testing.T) {
	db, mockClient, mockProvider := setupEmbeddingsTest(t)

	docUUID := uuid.New()
	revision := &models.DocumentRevision{
		ID:           1,
		DocumentID:   "test-doc-123",
		DocumentUUID: docUUID,
		ContentHash:  "abc123",
	}

	config := map[string]interface{}{
		"model":      "text-embedding-3-small",
		"dimensions": 1536,
	}

	// Create existing embedding with same content hash
	existingEmbedding := &models.DocumentEmbedding{
		DocumentID:   "test-doc-123",
		DocumentUUID: &docUUID,
		Embedding:    make([]float64, 1536),
		Dimensions:   1536,
		Model:        "text-embedding-3-small",
		Provider:     "openai",
		ContentHash:  "abc123", // Same content hash
		GeneratedAt:  time.Now(),
	}
	require.NoError(t, db.Create(existingEmbedding).Error)

	// Execute step (should skip generation)
	step := NewEmbeddingsStep(db, mockClient, mockProvider, hclog.NewNullLogger())
	err := step.Execute(context.Background(), revision, config)

	require.NoError(t, err)

	// Verify no new embeddings were created
	var count int64
	db.Model(&models.DocumentEmbedding{}).Where("document_id = ?", "test-doc-123").Count(&count)
	assert.Equal(t, int64(1), count)

	// Verify mock client was NOT called (provider would be called to check but we skip early)
	mockClient.AssertNotCalled(t, "GenerateEmbeddings")
}

func TestEmbeddingsStep_Execute_WithChunking(t *testing.T) {
	db, mockClient, mockProvider := setupEmbeddingsTest(t)

	docUUID := uuid.New()
	revision := &models.DocumentRevision{
		ID:           1,
		DocumentID:   "test-doc-123",
		DocumentUUID: docUUID,
		ContentHash:  "abc123",
	}

	config := map[string]interface{}{
		"model":      "text-embedding-3-small",
		"dimensions": 1536,
		"chunk_size": 100, // Force chunking
		"provider":   "openai",
	}

	// Create content that will be chunked (make it longer to ensure multiple chunks)
	testContent := "This is paragraph 1 with some content.\n\n" +
		"This is paragraph 2 with different content that is also quite long.\n\n" +
		"This is paragraph 3 which is even longer and should definitely cause chunking to happen because it exceeds the chunk size limit we set for this test."

	// Setup mocks
	mockProvider.Content["test-doc-123"] = testContent

	// Use mock.MatchedBy to dynamically create embeddings based on the actual number of texts
	mockClient.On("GenerateEmbeddingsBatch",
		mock.Anything,
		mock.MatchedBy(func(texts []string) bool {
			// Accept any slice of strings
			return len(texts) >= 1
		}),
		"text-embedding-3-small",
		1536,
	).Run(func(args mock.Arguments) {
		// This runs during the call, not when setting up the mock
	}).Return(func(ctx context.Context, texts []string, model string, dimensions int) [][]float64 {
		// Dynamically create embeddings based on actual texts
		embeddings := make([][]float64, len(texts))
		for i := range embeddings {
			embeddings[i] = make([]float64, 1536)
		}
		return embeddings
	}, nil).Once()

	// Execute step
	step := NewEmbeddingsStep(db, mockClient, mockProvider, hclog.NewNullLogger())
	err := step.Execute(context.Background(), revision, config)

	require.NoError(t, err)
	mockClient.AssertExpectations(t)

	// Verify multiple chunk embeddings were saved
	var embeddings []models.DocumentEmbedding
	err = db.Where("document_id = ?", "test-doc-123").Order("chunk_index ASC").Find(&embeddings).Error
	require.NoError(t, err)

	assert.GreaterOrEqual(t, len(embeddings), 2, "Should have at least 2 chunks")
	for i, emb := range embeddings {
		assert.Equal(t, i, *emb.ChunkIndex)
		assert.NotEmpty(t, emb.ChunkText)
	}
}

func TestEmbeddingsStep_CleanContent(t *testing.T) {
	db, mockClient, mockProvider := setupEmbeddingsTest(t)
	step := NewEmbeddingsStep(db, mockClient, mockProvider, hclog.NewNullLogger())

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "normalize CRLF",
			input:    "line1\r\nline2\r\nline3",
			expected: "line1\nline2\nline3",
		},
		{
			name:     "normalize CR",
			input:    "line1\rline2\rline3",
			expected: "line1\nline2\nline3",
		},
		{
			name:     "trim whitespace",
			input:    "  \n  content  \n  ",
			expected: "content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := step.cleanContent(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEmbeddingsStep_ChunkContent(t *testing.T) {
	db, mockClient, mockProvider := setupEmbeddingsTest(t)
	step := NewEmbeddingsStep(db, mockClient, mockProvider, hclog.NewNullLogger())

	t.Run("chunk by paragraphs", func(t *testing.T) {
		content := "Para 1.\n\nPara 2.\n\nPara 3."
		chunks := step.chunkContent(content, 20)

		assert.GreaterOrEqual(t, len(chunks), 2)
		for _, chunk := range chunks {
			assert.LessOrEqual(t, len(chunk), 30) // Allow some overhead
		}
	})

	t.Run("single paragraph force split", func(t *testing.T) {
		content := "This is a very long single paragraph without any breaks that needs to be forcibly split into chunks."
		chunks := step.chunkContent(content, 30)

		assert.GreaterOrEqual(t, len(chunks), 2)
	})

	t.Run("no chunking needed", func(t *testing.T) {
		content := "Short content"
		chunks := step.chunkContent(content, 100)

		assert.Equal(t, 1, len(chunks))
		assert.Equal(t, content, chunks[0])
	})
}

func TestEmbeddingsStep_ParseOptions(t *testing.T) {
	db, mockClient, mockProvider := setupEmbeddingsTest(t)
	step := NewEmbeddingsStep(db, mockClient, mockProvider, hclog.NewNullLogger())

	t.Run("default options", func(t *testing.T) {
		config := map[string]interface{}{}
		opts := step.parseOptions(config)

		assert.Equal(t, "text-embedding-3-small", opts.Model)
		assert.Equal(t, 1536, opts.Dimensions)
		assert.Equal(t, 0, opts.ChunkSize)
		assert.Equal(t, "openai", opts.Provider)
	})

	t.Run("custom options", func(t *testing.T) {
		config := map[string]interface{}{
			"model":      "text-embedding-3-large",
			"dimensions": 3072,
			"chunk_size": 1000,
			"provider":   "bedrock",
		}
		opts := step.parseOptions(config)

		assert.Equal(t, "text-embedding-3-large", opts.Model)
		assert.Equal(t, 3072, opts.Dimensions)
		assert.Equal(t, 1000, opts.ChunkSize)
		assert.Equal(t, "bedrock", opts.Provider)
	})

	t.Run("float64 conversion", func(t *testing.T) {
		config := map[string]interface{}{
			"dimensions": 3072.0,
			"chunk_size": 1000.0,
		}
		opts := step.parseOptions(config)

		assert.Equal(t, 3072, opts.Dimensions)
		assert.Equal(t, 1000, opts.ChunkSize)
	})
}
