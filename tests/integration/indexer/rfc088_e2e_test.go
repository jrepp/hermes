package indexer

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp-forge/hermes/pkg/indexer/pipeline/steps"
	"github.com/hashicorp-forge/hermes/pkg/indexer/ruleset"
	"github.com/hashicorp-forge/hermes/pkg/models"
	"github.com/hashicorp-forge/hermes/pkg/search"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// TestRFC088_FullPipeline tests the complete RFC-088 indexer pipeline:
// Document → LLM Summary → Embeddings → Search
func TestRFC088_FullPipeline(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	// Setup test database
	db := setupTestDB(t)

	// Setup OpenAI client (mock for testing)
	mockOpenAI := &MockOpenAIClient{}
	mockWorkspace := &MockWorkspaceProvider{
		Content: make(map[string]string),
	}

	// Create test document
	testDoc := createTestDocument(t, db, "RFC-100: Test Feature",
		"This is a test RFC document about implementing a new authentication system using OAuth 2.0.")
	mockWorkspace.Content[testDoc.DocumentID] = testDoc.Title + ". " + "This is a test RFC document about implementing a new authentication system using OAuth 2.0."

	// Setup LLM summary step
	llmStep := steps.NewLLMSummaryStep(db, mockOpenAI, mockWorkspace, hclog.NewNullLogger())

	// Setup embeddings step
	embeddingsStep := steps.NewEmbeddingsStep(db, mockOpenAI, mockWorkspace, hclog.NewNullLogger())

	// Configure mock responses
	mockOpenAI.On("GenerateSummary",
		mock.Anything,
		mock.AnythingOfType("string"),
		mock.AnythingOfType("steps.SummaryOptions"),
	).Return(&steps.Summary{
		ExecutiveSummary: "This RFC proposes implementing OAuth 2.0 authentication",
		KeyPoints:        []string{"OAuth 2.0 implementation", "Security improvements", "User experience"},
		Topics:           []string{"Authentication", "Security", "OAuth"},
		Tags:             []string{"auth", "security", "rfc"},
		Confidence:       0.85,
		TokensUsed:       150,
		GenerationTimeMs: 1200,
	}, nil)

	// Create test embedding vector
	testEmbedding := make([]float64, 1536)
	for i := range testEmbedding {
		testEmbedding[i] = float64(i) * 0.001
	}

	mockOpenAI.On("GenerateEmbeddings",
		mock.Anything,
		mock.AnythingOfType("string"),
		"text-embedding-3-small",
		1536,
	).Return(testEmbedding, nil)

	// Execute pipeline
	ctx := context.Background()

	// Step 1: Generate LLM summary
	t.Log("Step 1: Generating LLM summary...")
	err := llmStep.Execute(ctx, testDoc, map[string]interface{}{
		"model":      "gpt-4o-mini",
		"max_tokens": 500,
		"style":      "executive",
	})
	require.NoError(t, err, "LLM summary generation failed")

	// Verify summary was created
	var summary models.DocumentSummary
	err = db.Where("document_id = ?", testDoc.DocumentID).First(&summary).Error
	require.NoError(t, err, "Summary not found")
	assert.Equal(t, "This RFC proposes implementing OAuth 2.0 authentication", summary.ExecutiveSummary)
	assert.Equal(t, 3, len(summary.KeyPoints))
	assert.Equal(t, 3, len(summary.Topics))
	t.Logf("✓ Summary created: %s", summary.ExecutiveSummary)

	// Step 2: Generate embeddings
	t.Log("Step 2: Generating embeddings...")
	err = embeddingsStep.Execute(ctx, testDoc, map[string]interface{}{
		"model":      "text-embedding-3-small",
		"dimensions": 1536,
		"provider":   "openai",
	})
	require.NoError(t, err, "Embeddings generation failed")

	// Verify embeddings were created
	var embedding models.DocumentEmbedding
	err = db.Where("document_id = ?", testDoc.DocumentID).First(&embedding).Error
	require.NoError(t, err, "Embedding not found")
	assert.Equal(t, 1536, len(embedding.Embedding))
	assert.Equal(t, "text-embedding-3-small", embedding.Model)
	t.Logf("✓ Embedding created: %d dimensions", len(embedding.Embedding))

	// Step 3: Test semantic search
	t.Log("Step 3: Testing semantic search...")
	semanticSearch, err := search.NewSemanticSearch(search.SemanticSearchConfig{
		DB:         db,
		EmbedGen:   mockOpenAI,
		Model:      "text-embedding-3-small",
		Dimensions: 1536,
		Logger:     hclog.NewNullLogger(),
	})
	require.NoError(t, err)

	// Note: Semantic search requires pgvector which isn't available in SQLite
	// This test verifies the structure works, full integration requires PostgreSQL
	_, err = semanticSearch.GetDocumentEmbedding(ctx, testDoc.DocumentID)
	require.NoError(t, err, "Failed to retrieve embedding for search")
	t.Log("✓ Semantic search infrastructure validated")

	// Step 4: Test idempotency
	t.Log("Step 4: Testing idempotency...")

	// Re-run LLM summary (should skip)
	err = llmStep.Execute(ctx, testDoc, map[string]interface{}{
		"model":      "gpt-4o-mini",
		"max_tokens": 500,
	})
	require.NoError(t, err)

	// Verify only one summary exists
	var summaryCount int64
	db.Model(&models.DocumentSummary{}).Where("document_id = ?", testDoc.DocumentID).Count(&summaryCount)
	assert.Equal(t, int64(1), summaryCount, "Idempotency failed: duplicate summary created")

	// Re-run embeddings (should skip)
	err = embeddingsStep.Execute(ctx, testDoc, map[string]interface{}{
		"model":      "text-embedding-3-small",
		"dimensions": 1536,
	})
	require.NoError(t, err)

	// Verify only one embedding exists
	var embeddingCount int64
	db.Model(&models.DocumentEmbedding{}).Where("document_id = ?", testDoc.DocumentID).Count(&embeddingCount)
	assert.Equal(t, int64(1), embeddingCount, "Idempotency failed: duplicate embedding created")
	t.Log("✓ Idempotency verified")

	t.Log("✅ Full pipeline test completed successfully")
}

// TestRFC088_RulesetMatching tests the ruleset matching system
func TestRFC088_RulesetMatching(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	db := setupTestDB(t)

	// Create test rulesets
	rulesets := []ruleset.Ruleset{
		{
			Name: "published-rfcs",
			Conditions: map[string]string{
				"document_type": "RFC",
				"status":        "Approved",
			},
			Pipeline: []string{"search_index", "llm_summary", "embeddings"},
		},
		{
			Name:       "all-documents",
			Conditions: map[string]string{},
			Pipeline:   []string{"search_index"},
		},
	}

	matcher := ruleset.NewMatcher(rulesets)

	// Test Case 1: RFC document (should match published-rfcs)
	rfcDoc := createTestDocument(t, db, "RFC-101: Test", "Test RFC")
	rfcDoc.DocumentID = "rfc-101"
	rfcDoc.Status = "Approved" // Update status to match ruleset condition
	require.NoError(t, db.Save(rfcDoc).Error)

	metadata := map[string]interface{}{
		"document_type": "RFC",
	}

	matched := matcher.Match(rfcDoc, metadata)
	require.NotEmpty(t, matched)
	assert.Equal(t, "published-rfcs", matched[0].Name)
	assert.Equal(t, 3, len(matched[0].Pipeline))
	t.Logf("✓ RFC document matched: %s", matched[0].Name)

	// Test Case 2: Meeting notes (should match all-documents)
	notesDoc := createTestDocument(t, db, "Meeting Notes", "Weekly sync")

	metadata2 := map[string]interface{}{
		"document_type": "Meeting Notes",
	}

	matched2 := matcher.Match(notesDoc, metadata2)
	require.NotEmpty(t, matched2)
	assert.Equal(t, "all-documents", matched2[0].Name)
	assert.Equal(t, 1, len(matched2[0].Pipeline))
	t.Logf("✓ Meeting notes matched: %s", matched2[0].Name)

	t.Log("✅ Ruleset matching test completed successfully")
}

// TestRFC088_ChunkedEmbeddings tests embeddings for large documents with chunking
func TestRFC088_ChunkedEmbeddings(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	db := setupTestDB(t)

	mockOpenAI := &MockOpenAIClient{}
	mockWorkspace := &MockWorkspaceProvider{
		Content: make(map[string]string),
	}

	// Create large document that requires chunking
	largeDoc := createTestDocument(t, db, "Large RFC", "Large document")
	largeContent := generateLargeContent(250) // 250 words per paragraph, 3 paragraphs = ~750 words
	mockWorkspace.Content[largeDoc.DocumentID] = largeContent

	embeddingsStep := steps.NewEmbeddingsStep(db, mockOpenAI, mockWorkspace, hclog.NewNullLogger())

	// Mock batch embeddings
	mockOpenAI.On("GenerateEmbeddingsBatch",
		mock.Anything,
		mock.MatchedBy(func(texts []string) bool { return len(texts) >= 1 }),
		"text-embedding-3-small",
		1536,
	).Return(func(ctx context.Context, texts []string, model string, dimensions int) [][]float64 {
		embeddings := make([][]float64, len(texts))
		for i := range embeddings {
			embeddings[i] = make([]float64, 1536)
			for j := range embeddings[i] {
				embeddings[i][j] = float64(i*1000+j) * 0.001
			}
		}
		return embeddings
	}, nil)

	// Execute embeddings with chunking enabled
	ctx := context.Background()
	err := embeddingsStep.Execute(ctx, largeDoc, map[string]interface{}{
		"model":      "text-embedding-3-small",
		"dimensions": 1536,
		"chunk_size": 200, // Small chunk size to force chunking
		"provider":   "openai",
	})
	require.NoError(t, err)

	// Verify multiple chunks were created
	var embeddings []models.DocumentEmbedding
	err = db.Where("document_id = ?", largeDoc.DocumentID).
		Order("chunk_index ASC").
		Find(&embeddings).Error
	require.NoError(t, err)

	assert.GreaterOrEqual(t, len(embeddings), 2, "Expected at least 2 chunks for large document")

	// Verify chunk indexes are sequential
	for i, emb := range embeddings {
		require.NotNil(t, emb.ChunkIndex)
		assert.Equal(t, i, *emb.ChunkIndex)
		assert.NotEmpty(t, emb.ChunkText)
	}

	t.Logf("✓ Created %d chunks for large document", len(embeddings))
	t.Log("✅ Chunked embeddings test completed successfully")
}

// Helper functions

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// Auto-migrate schemas
	err = db.AutoMigrate(
		&models.DocumentRevision{},
		&models.DocumentSummary{},
		&models.DocumentEmbedding{},
	)
	require.NoError(t, err)

	return db
}

func createTestDocument(t *testing.T, db *gorm.DB, title, content string) *models.DocumentRevision {
	docUUID := uuid.New()
	doc := &models.DocumentRevision{
		DocumentUUID: docUUID,
		DocumentID:   uuid.New().String(),
		ProviderType: "google",
		Title:        title,
		ContentHash:  "test-hash-" + uuid.New().String(),
		ModifiedTime: time.Now(),
		Status:       "active",
	}

	err := db.Create(doc).Error
	require.NoError(t, err)

	return doc
}

func generateLargeContent(wordsPerPara int) string {
	para1 := generateParagraph(wordsPerPara)
	para2 := generateParagraph(wordsPerPara)
	para3 := generateParagraph(wordsPerPara)

	return para1 + "\n\n" + para2 + "\n\n" + para3
}

func generateParagraph(words int) string {
	content := "This is a test paragraph with repeated content to reach the desired word count. "
	result := ""
	for i := 0; i < words/12; i++ { // Roughly 12 words per sentence
		result += content
	}
	return result
}

// Mock implementations

type MockOpenAIClient struct {
	mock.Mock
}

func (m *MockOpenAIClient) GenerateSummary(ctx context.Context, content string, options steps.SummaryOptions) (*steps.Summary, error) {
	args := m.Called(ctx, content, options)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*steps.Summary), args.Error(1)
}

func (m *MockOpenAIClient) GenerateEmbeddings(ctx context.Context, text string, model string, dimensions int) ([]float64, error) {
	args := m.Called(ctx, text, model, dimensions)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]float64), args.Error(1)
}

func (m *MockOpenAIClient) GenerateEmbeddingsBatch(ctx context.Context, texts []string, model string, dimensions int) ([][]float64, error) {
	args := m.Called(ctx, texts, model, dimensions)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}

	// Support function returns for dynamic batch sizes
	result := args.Get(0)
	if fn, ok := result.(func(context.Context, []string, string, int) [][]float64); ok {
		return fn(ctx, texts, model, dimensions), args.Error(1)
	}
	return result.([][]float64), args.Error(1)
}

type MockWorkspaceProvider struct {
	Content map[string]string
	Error   error
}

func (m *MockWorkspaceProvider) GetDocumentContent(fileID string) (string, error) {
	if m.Error != nil {
		return "", m.Error
	}

	if content, ok := m.Content[fileID]; ok {
		return content, nil
	}

	return "Default test content for document analysis and processing.", nil
}
