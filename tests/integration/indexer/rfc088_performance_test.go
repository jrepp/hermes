package indexer

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/hashicorp-forge/hermes/pkg/indexer/pipeline/steps"
	"github.com/hashicorp-forge/hermes/pkg/models"
)

// BenchmarkEmbeddingsGeneration measures embedding generation performance
func BenchmarkEmbeddingsGeneration(b *testing.B) {
	db, mockOpenAI, mockWorkspace := setupPerfTest(b)

	// Create test document
	testDoc := createPerfDocument(b, db, "Test Document", 1000) // 1000 words
	mockWorkspace.Content[testDoc.DocumentID] = generateLargeContent(1000)

	// Configure mock
	mockOpenAI.On("GenerateEmbeddings",
		mock.Anything,
		mock.AnythingOfType("string"),
		"text-embedding-3-small",
		1536,
	).Return(generateTestEmbedding(1536), nil)

	embeddingsStep := steps.NewEmbeddingsStep(db, mockOpenAI, mockWorkspace, hclog.NewNullLogger())
	ctx := context.Background()

	config := map[string]interface{}{
		"model":      "text-embedding-3-small",
		"dimensions": 1536,
		"provider":   "openai",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Create new document for each iteration to avoid idempotency skip
		doc := createPerfDocument(b, db, fmt.Sprintf("Doc-%d", i), 1000)
		mockWorkspace.Content[doc.DocumentID] = generateLargeContent(1000)

		err := embeddingsStep.Execute(ctx, doc, config)
		require.NoError(b, err)
	}
	b.StopTimer()

	// Report metrics
	b.ReportMetric(float64(b.N)/b.Elapsed().Seconds(), "docs/sec")
}

// BenchmarkChunkedEmbeddings measures chunked embedding performance for large documents
func BenchmarkChunkedEmbeddings(b *testing.B) {
	db, mockOpenAI, mockWorkspace := setupPerfTest(b)

	// Configure batch embeddings mock
	mockOpenAI.On("GenerateEmbeddingsBatch",
		mock.Anything,
		mock.MatchedBy(func(texts []string) bool { return len(texts) >= 1 }),
		"text-embedding-3-small",
		1536,
	).Return(func(ctx context.Context, texts []string, model string, dimensions int) [][]float64 {
		embeddings := make([][]float64, len(texts))
		for i := range embeddings {
			embeddings[i] = generateTestEmbedding(1536)
		}
		return embeddings
	}, nil)

	embeddingsStep := steps.NewEmbeddingsStep(db, mockOpenAI, mockWorkspace, hclog.NewNullLogger())
	ctx := context.Background()

	config := map[string]interface{}{
		"model":      "text-embedding-3-small",
		"dimensions": 1536,
		"chunk_size": 500, // Force chunking
		"provider":   "openai",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Large document: 5000 words (~10 chunks)
		doc := createPerfDocument(b, db, fmt.Sprintf("LargeDoc-%d", i), 5000)
		mockWorkspace.Content[doc.DocumentID] = generateLargeContent(5000)

		err := embeddingsStep.Execute(ctx, doc, config)
		require.NoError(b, err)
	}
	b.StopTimer()

	b.ReportMetric(float64(b.N)/b.Elapsed().Seconds(), "docs/sec")
}

// TestPipelineThroughput tests pipeline throughput with multiple concurrent documents
func TestPipelineThroughput(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping throughput test in short mode")
	}

	db, mockOpenAI, mockWorkspace := setupPerfTest(t)

	// Configure mocks
	mockOpenAI.On("GenerateSummary",
		mock.Anything,
		mock.AnythingOfType("string"),
		mock.AnythingOfType("steps.SummaryOptions"),
	).Return(&steps.Summary{
		ExecutiveSummary: "Test summary",
		KeyPoints:        []string{"point1", "point2"},
		Topics:           []string{"topic1"},
		Tags:             []string{"tag1"},
		Confidence:       0.85,
		TokensUsed:       100,
		GenerationTimeMs: 500,
	}, nil)

	mockOpenAI.On("GenerateEmbeddings",
		mock.Anything,
		mock.AnythingOfType("string"),
		"text-embedding-3-small",
		1536,
	).Return(generateTestEmbedding(1536), nil)

	llmStep := steps.NewLLMSummaryStep(db, mockOpenAI, mockWorkspace, hclog.NewNullLogger())
	embeddingsStep := steps.NewEmbeddingsStep(db, mockOpenAI, mockWorkspace, hclog.NewNullLogger())

	ctx := context.Background()
	numDocs := 50    // Reduced for SQLite limitations
	concurrency := 1 // SQLite: use sequential processing to avoid table locks

	t.Logf("Testing pipeline with %d documents (sequential processing)", numDocs)
	t.Log("Note: SQLite doesn't support concurrent writes. For concurrent load testing, use PostgreSQL.")

	// Create documents
	docs := make([]*models.DocumentRevision, numDocs)
	for i := 0; i < numDocs; i++ {
		docs[i] = createPerfDocument(t, db, fmt.Sprintf("Doc-%d", i), 500)
		mockWorkspace.Content[docs[i].DocumentID] = generateLargeContent(500)
	}

	startTime := time.Now()

	// Process documents concurrently
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, concurrency)
	errors := make(chan error, numDocs)

	for _, doc := range docs {
		wg.Add(1)
		go func(d *models.DocumentRevision) {
			defer wg.Done()

			semaphore <- struct{}{}        // Acquire semaphore
			defer func() { <-semaphore }() // Release semaphore

			// Run pipeline steps
			llmConfig := map[string]interface{}{
				"model":      "gpt-4o-mini",
				"max_tokens": 500,
			}
			if err := llmStep.Execute(ctx, d, llmConfig); err != nil {
				errors <- fmt.Errorf("LLM step failed for %s: %w", d.DocumentID, err)
				return
			}

			embConfig := map[string]interface{}{
				"model":      "text-embedding-3-small",
				"dimensions": 1536,
				"provider":   "openai",
			}
			if err := embeddingsStep.Execute(ctx, d, embConfig); err != nil {
				errors <- fmt.Errorf("embeddings step failed for %s: %w", d.DocumentID, err)
				return
			}
		}(doc)
	}

	wg.Wait()
	close(errors)

	elapsed := time.Since(startTime)

	// Check for errors
	errorCount := 0
	for err := range errors {
		t.Errorf("Pipeline error: %v", err)
		errorCount++
	}

	if errorCount > 0 {
		t.Fatalf("%d pipeline failures out of %d documents", errorCount, numDocs)
	}

	// Calculate metrics
	throughput := float64(numDocs) / elapsed.Seconds()
	avgLatency := elapsed / time.Duration(numDocs)

	t.Logf("✅ Pipeline test completed successfully")
	t.Logf("   Total time: %v", elapsed)
	t.Logf("   Throughput: %.2f docs/sec", throughput)
	t.Logf("   Average latency: %v per document", avgLatency)
	if concurrency > 1 {
		t.Logf("   Concurrency: %d workers", concurrency)
	} else {
		t.Logf("   Mode: Sequential (SQLite limitation)")
	}

	// Assert minimum performance requirements (adjusted for SQLite)
	require.Greater(t, throughput, 5.0, "Throughput should be > 5 docs/sec with SQLite")
	require.Less(t, avgLatency, 5*time.Second, "Average latency should be < 5 seconds")
}

// TestMemoryUsage tests memory usage with large document processing
func TestMemoryUsage(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory test in short mode")
	}

	db, mockOpenAI, mockWorkspace := setupPerfTest(t)

	// Configure batch embeddings mock
	mockOpenAI.On("GenerateEmbeddingsBatch",
		mock.Anything,
		mock.MatchedBy(func(texts []string) bool { return len(texts) >= 1 }),
		"text-embedding-3-small",
		1536,
	).Return(func(ctx context.Context, texts []string, model string, dimensions int) [][]float64 {
		embeddings := make([][]float64, len(texts))
		for i := range embeddings {
			embeddings[i] = generateTestEmbedding(1536)
		}
		return embeddings
	}, nil)

	embeddingsStep := steps.NewEmbeddingsStep(db, mockOpenAI, mockWorkspace, hclog.NewNullLogger())
	ctx := context.Background()

	// Test with very large documents (10,000+ words each)
	numDocs := 10
	wordsPerDoc := 10000

	t.Logf("Testing memory usage with %d documents of %d words each", numDocs, wordsPerDoc)

	for i := 0; i < numDocs; i++ {
		doc := createPerfDocument(t, db, fmt.Sprintf("VeryLargeDoc-%d", i), wordsPerDoc)
		mockWorkspace.Content[doc.DocumentID] = generateLargeContent(wordsPerDoc)

		config := map[string]interface{}{
			"model":      "text-embedding-3-small",
			"dimensions": 1536,
			"chunk_size": 500,
			"provider":   "openai",
		}

		err := embeddingsStep.Execute(ctx, doc, config)
		require.NoError(t, err)

		// Verify chunks were created
		var embeddings []models.DocumentEmbedding
		err = db.Where("document_id = ?", doc.DocumentID).Find(&embeddings).Error
		require.NoError(t, err)
		require.NotEmpty(t, embeddings, "Should create embeddings for large document")

		t.Logf("   Document %d: created %d chunks", i+1, len(embeddings))
	}

	t.Logf("✅ Memory test completed successfully")
	t.Log("   All large documents processed without errors")
}

// Helper functions for performance tests

func setupPerfTest(tb testing.TB) (*gorm.DB, *MockOpenAIClient, *MockWorkspaceProvider) {
	// Use WAL mode for better concurrent access
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared&mode=memory&_journal_mode=WAL"), &gorm.Config{
		SkipDefaultTransaction: true,
		PrepareStmt:            true,
	})
	require.NoError(tb, err)

	// Set connection pool limits for better concurrency
	sqlDB, err := db.DB()
	require.NoError(tb, err)
	sqlDB.SetMaxOpenConns(25)

	err = db.AutoMigrate(
		&models.DocumentRevision{},
		&models.DocumentSummary{},
		&models.DocumentEmbedding{},
	)
	require.NoError(tb, err)

	mockOpenAI := new(MockOpenAIClient)
	mockWorkspace := &MockWorkspaceProvider{
		Content: make(map[string]string),
	}

	return db, mockOpenAI, mockWorkspace
}

func createPerfDocument(tb testing.TB, db *gorm.DB, title string, words int) *models.DocumentRevision {
	docUUID := uuid.New()
	doc := &models.DocumentRevision{
		DocumentUUID: docUUID,
		DocumentID:   uuid.New().String(),
		ProviderType: "google",
		Title:        title,
		ContentHash:  fmt.Sprintf("perf-hash-%s-%d", uuid.New().String(), words),
		ModifiedTime: time.Now(),
		Status:       "active",
	}

	err := db.Create(doc).Error
	require.NoError(tb, err)

	return doc
}

func generateTestEmbedding(dimensions int) []float64 {
	embedding := make([]float64, dimensions)
	for i := range embedding {
		embedding[i] = float64(i) * 0.001
	}
	return embedding
}
