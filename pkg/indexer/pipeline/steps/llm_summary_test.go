package steps

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/hashicorp-forge/hermes/pkg/models"
)

// setupTestDB creates an in-memory SQLite database for testing
func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// Auto-migrate tables
	err = db.AutoMigrate(
		&models.DocumentRevision{},
		&models.DocumentSummary{},
	)
	require.NoError(t, err)

	return db
}

// createTestRevision creates a test document revision
func createTestRevision(t *testing.T, db *gorm.DB) *models.DocumentRevision {
	revision := &models.DocumentRevision{
		DocumentUUID: uuid.New(),
		DocumentID:   "test-doc-1",
		ProviderType: "google",
		Title:        "Test Document",
		ContentHash:  "hash123",
		Status:       "active",
		ModifiedTime: time.Now(),
	}
	require.NoError(t, db.Create(revision).Error)
	return revision
}

func TestLLMSummaryStep_FetchDocumentContent_Success(t *testing.T) {
	db := setupTestDB(t)
	revision := createTestRevision(t, db)

	// Create mock workspace provider with test content
	mockWorkspace := &MockWorkspaceProvider{
		Content: map[string]string{
			"test-doc-1": "This is the actual content from the workspace provider that should be fetched and processed by the LLM.",
		},
	}

	step := NewLLMSummaryStep(db, &MockLLMClient{}, mockWorkspace, hclog.NewNullLogger())

	// Fetch content
	content, err := step.fetchDocumentContent(revision)

	require.NoError(t, err)
	assert.Equal(t, "This is the actual content from the workspace provider that should be fetched and processed by the LLM.", content)
}

func TestLLMSummaryStep_FetchDocumentContent_ProviderError(t *testing.T) {
	db := setupTestDB(t)
	revision := createTestRevision(t, db)

	// Create mock workspace provider that returns an error
	mockWorkspace := &MockWorkspaceProvider{
		Error: errors.New("workspace provider connection failed"),
	}

	step := NewLLMSummaryStep(db, &MockLLMClient{}, mockWorkspace, hclog.NewNullLogger())

	// Attempt to fetch content
	_, err := step.fetchDocumentContent(revision)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to fetch content from workspace provider")
	assert.Contains(t, err.Error(), "workspace provider connection failed")
}

func TestLLMSummaryStep_FetchDocumentContent_NoProvider(t *testing.T) {
	db := setupTestDB(t)
	revision := createTestRevision(t, db)

	// Create step without workspace provider
	step := NewLLMSummaryStep(db, &MockLLMClient{}, nil, hclog.NewNullLogger())

	// Attempt to fetch content
	_, err := step.fetchDocumentContent(revision)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "workspace provider not configured")
}

func TestLLMSummaryStep_CleanContent(t *testing.T) {
	step := &LLMSummaryStep{
		logger: hclog.NewNullLogger(),
	}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "remove leading/trailing whitespace",
			input:    "   content with spaces   ",
			expected: "content with spaces",
		},
		{
			name:     "normalize line endings",
			input:    "line1\r\nline2\r\nline3",
			expected: "line1\nline2\nline3",
		},
		{
			name:     "remove excessive blank lines",
			input:    "paragraph1\n\n\n\n\nparagraph2",
			expected: "paragraph1\n\nparagraph2",
		},
		{
			name:     "combined cleaning",
			input:    "  \n\nContent\r\n\r\n\r\n\r\nMore content\n\n\n\n  ",
			expected: "Content\n\nMore content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := step.cleanContent(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestLLMSummaryStep_Execute_WithContentFetching(t *testing.T) {
	db := setupTestDB(t)
	revision := createTestRevision(t, db)

	// Create mock workspace provider with sufficient content
	mockWorkspace := &MockWorkspaceProvider{
		Content: map[string]string{
			"test-doc-1": "This is a substantial test document with enough content to warrant summary generation. " +
				"It contains multiple sentences and paragraphs that provide meaningful information about the topic. " +
				"The document discusses important concepts and provides detailed explanations that are valuable for readers.",
		},
	}

	step := NewLLMSummaryStep(db, &MockLLMClient{}, mockWorkspace, hclog.NewNullLogger())

	// Execute the step
	ctx := context.Background()
	err := step.Execute(ctx, revision, nil)

	require.NoError(t, err)

	// Verify summary was created
	var summaries []models.DocumentSummary
	err = db.Where("document_id = ?", revision.DocumentID).Find(&summaries).Error
	require.NoError(t, err)
	require.Len(t, summaries, 1)

	// Verify summary content
	summary := summaries[0]
	assert.Equal(t, "test-doc-1", summary.DocumentID)
	assert.Equal(t, revision.DocumentUUID, *summary.DocumentUUID)
	assert.NotEmpty(t, summary.ExecutiveSummary)
	assert.NotEmpty(t, summary.KeyPoints)
	assert.NotEmpty(t, summary.Topics)
	assert.NotEmpty(t, summary.Tags)
}

func TestLLMSummaryStep_Execute_ContentTooShort(t *testing.T) {
	db := setupTestDB(t)
	revision := createTestRevision(t, db)

	// Create mock workspace provider with very short content
	mockWorkspace := &MockWorkspaceProvider{
		Content: map[string]string{
			"test-doc-1": "Short content.",
		},
	}

	step := NewLLMSummaryStep(db, &MockLLMClient{}, mockWorkspace, hclog.NewNullLogger())

	// Execute the step
	ctx := context.Background()
	err := step.Execute(ctx, revision, nil)

	require.NoError(t, err) // Should succeed but skip summary generation

	// Verify no summary was created
	var summaries []models.DocumentSummary
	err = db.Where("document_id = ?", revision.DocumentID).Find(&summaries).Error
	require.NoError(t, err)
	assert.Len(t, summaries, 0) // No summary should be created for short content
}

func TestLLMSummaryStep_Execute_IdempotentSummary(t *testing.T) {
	db := setupTestDB(t)
	revision := createTestRevision(t, db)

	mockWorkspace := &MockWorkspaceProvider{
		Content: map[string]string{
			"test-doc-1": "This is a substantial test document with enough content to warrant summary generation. " +
				"It contains multiple sentences and paragraphs that provide meaningful information about the topic.",
		},
	}

	step := NewLLMSummaryStep(db, &MockLLMClient{}, mockWorkspace, hclog.NewNullLogger())

	// Execute the step twice
	ctx := context.Background()
	err := step.Execute(ctx, revision, nil)
	require.NoError(t, err)

	err = step.Execute(ctx, revision, nil)
	require.NoError(t, err)

	// Verify only one summary was created (idempotent)
	var summaries []models.DocumentSummary
	err = db.Where("document_id = ?", revision.DocumentID).Find(&summaries).Error
	require.NoError(t, err)
	assert.Len(t, summaries, 1) // Should still be only 1 summary
}

func TestLLMSummaryStep_Execute_FetchContentError(t *testing.T) {
	db := setupTestDB(t)
	revision := createTestRevision(t, db)

	// Create mock workspace provider that returns an error
	mockWorkspace := &MockWorkspaceProvider{
		Error: errors.New("network timeout"),
	}

	step := NewLLMSummaryStep(db, &MockLLMClient{}, mockWorkspace, hclog.NewNullLogger())

	// Execute the step
	ctx := context.Background()
	err := step.Execute(ctx, revision, nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to fetch document content")
}

func TestMockWorkspaceProvider_DefaultContent(t *testing.T) {
	mockWorkspace := &MockWorkspaceProvider{}

	// Test default content when no specific content is set
	content, err := mockWorkspace.GetDocumentContent("any-doc-id")

	require.NoError(t, err)
	assert.Equal(t, "This is a test document with some sample content for processing.", content)
}

func TestMockWorkspaceProvider_SpecificContent(t *testing.T) {
	mockWorkspace := &MockWorkspaceProvider{
		Content: map[string]string{
			"doc1": "Content for document 1",
			"doc2": "Content for document 2",
		},
	}

	// Test specific content
	content1, err := mockWorkspace.GetDocumentContent("doc1")
	require.NoError(t, err)
	assert.Equal(t, "Content for document 1", content1)

	content2, err := mockWorkspace.GetDocumentContent("doc2")
	require.NoError(t, err)
	assert.Equal(t, "Content for document 2", content2)

	// Test fallback to default for unknown doc
	content3, err := mockWorkspace.GetDocumentContent("doc3")
	require.NoError(t, err)
	assert.Equal(t, "This is a test document with some sample content for processing.", content3)
}
