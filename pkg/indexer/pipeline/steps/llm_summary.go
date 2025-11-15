package steps

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp-forge/hermes/pkg/models"
	"github.com/hashicorp/go-hclog"
	"gorm.io/gorm"
)

// LLMSummaryStep generates AI summaries for document revisions.
// Summaries are stored in the document_summaries table.
type LLMSummaryStep struct {
	db                *gorm.DB
	llmClient         LLMClient
	workspaceProvider WorkspaceContentProvider
	logger            hclog.Logger
}

// WorkspaceContentProvider defines the interface for fetching document content.
type WorkspaceContentProvider interface {
	// GetDocumentContent fetches content from the workspace provider.
	GetDocumentContent(fileID string) (string, error)
}

// LLMClient is the interface for LLM API clients.
type LLMClient interface {
	// GenerateSummary generates a summary for the given content.
	GenerateSummary(ctx context.Context, content string, options SummaryOptions) (*Summary, error)
}

// SummaryOptions holds options for summary generation.
type SummaryOptions struct {
	Model     string // e.g., "gpt-4o-mini", "claude-3-haiku"
	MaxTokens int    // Maximum tokens for the summary
	Language  string // Target language (default: "en")
	Style     string // Summary style (e.g., "executive", "technical", "bullet-points")
}

// Summary represents an LLM-generated summary.
type Summary struct {
	ExecutiveSummary string   // High-level overview
	KeyPoints        []string // Main takeaways
	Topics           []string // Topics covered
	Tags             []string // Suggested tags
	Confidence       float64  // Confidence score (0-1)
	TokensUsed       int      // Tokens consumed
	GenerationTimeMs int      // Time taken in milliseconds
}

// NewLLMSummaryStep creates a new LLM summary step.
func NewLLMSummaryStep(db *gorm.DB, llmClient LLMClient, workspaceProvider WorkspaceContentProvider, logger hclog.Logger) *LLMSummaryStep {
	if logger == nil {
		logger = hclog.NewNullLogger()
	}

	return &LLMSummaryStep{
		db:                db,
		llmClient:         llmClient,
		workspaceProvider: workspaceProvider,
		logger:            logger.Named("llm-summary-step"),
	}
}

// Name returns the step name.
func (s *LLMSummaryStep) Name() string {
	return "llm_summary"
}

// Execute generates an AI summary for the given revision.
func (s *LLMSummaryStep) Execute(ctx context.Context, revision *models.DocumentRevision, config map[string]interface{}) error {
	s.logger.Debug("executing LLM summary step",
		"document_uuid", revision.DocumentUUID,
		"revision_id", revision.ID,
		"content_hash", revision.ContentHash,
	)

	// Check if summary already exists for this content hash
	existing, err := models.GetSummaryByDocumentIDAndModel(s.db, revision.DocumentID, s.getModel(config))
	if err != nil && err != gorm.ErrRecordNotFound {
		return fmt.Errorf("failed to check for existing summary: %w", err)
	}

	if existing != nil && existing.MatchesContentHash(revision.ContentHash) {
		s.logger.Debug("summary already exists for this content hash, skipping",
			"document_uuid", revision.DocumentUUID,
			"content_hash", revision.ContentHash,
		)
		return nil
	}

	// Fetch document content
	content, err := s.fetchDocumentContent(revision)
	if err != nil {
		return fmt.Errorf("failed to fetch document content: %w", err)
	}

	// Check content length
	if len(content) < 100 {
		s.logger.Debug("document too short for summary, skipping",
			"document_uuid", revision.DocumentUUID,
			"content_length", len(content),
		)
		return nil
	}

	// Build summary options from config
	options := SummaryOptions{
		Model:     s.getModel(config),
		MaxTokens: s.getMaxTokens(config),
		Language:  s.getLanguage(config),
		Style:     s.getStyle(config),
	}

	// Generate summary using LLM
	summary, err := s.llmClient.GenerateSummary(ctx, content, options)
	if err != nil {
		return fmt.Errorf("failed to generate summary: %w", err)
	}

	// Save summary to database
	dbSummary := &models.DocumentSummary{
		DocumentID:       revision.DocumentID,
		DocumentUUID:     &revision.DocumentUUID,
		ExecutiveSummary: summary.ExecutiveSummary,
		KeyPoints:        summary.KeyPoints,
		Topics:           summary.Topics,
		Tags:             summary.Tags,
		SuggestedStatus:  "", // Could be populated by LLM analysis
		Confidence:       &summary.Confidence,
		Model:            options.Model,
		Provider:         s.extractProvider(options.Model),
		TokensUsed:       &summary.TokensUsed,
		GenerationTimeMs: &summary.GenerationTimeMs,
		DocumentTitle:    revision.Title,
		DocumentType:     s.extractDocType(revision),
		ContentHash:      revision.ContentHash,
		ContentLength:    ptrInt(len(content)),
	}

	if err := s.db.Create(dbSummary).Error; err != nil {
		return fmt.Errorf("failed to save summary: %w", err)
	}

	s.logger.Info("generated and saved LLM summary",
		"document_uuid", revision.DocumentUUID,
		"revision_id", revision.ID,
		"summary_id", dbSummary.ID,
		"model", options.Model,
		"tokens_used", summary.TokensUsed,
	)

	return nil
}

// IsRetryable determines if an error should trigger a retry.
func (s *LLMSummaryStep) IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	errMsg := strings.ToLower(err.Error())

	// Network errors are retryable
	if strings.Contains(errMsg, "timeout") ||
		strings.Contains(errMsg, "connection") {
		return true
	}

	// Rate limiting is retryable
	if strings.Contains(errMsg, "rate limit") ||
		strings.Contains(errMsg, "quota exceeded") ||
		strings.Contains(errMsg, "too many requests") {
		return true
	}

	// Service errors are retryable
	if strings.Contains(errMsg, "service unavailable") ||
		strings.Contains(errMsg, "internal server error") {
		return true
	}

	// Content/validation errors are not retryable
	if strings.Contains(errMsg, "content policy") ||
		strings.Contains(errMsg, "invalid") {
		return false
	}

	return false
}

// fetchDocumentContent fetches the full document content for the revision.
func (s *LLMSummaryStep) fetchDocumentContent(revision *models.DocumentRevision) (string, error) {
	if s.workspaceProvider == nil {
		return "", fmt.Errorf("workspace provider not configured")
	}

	// Fetch content using workspace provider
	content, err := s.workspaceProvider.GetDocumentContent(revision.DocumentID)
	if err != nil {
		return "", fmt.Errorf("failed to fetch content from workspace provider: %w", err)
	}

	// Clean and normalize the content for LLM processing
	cleanedContent := s.cleanContent(content)

	s.logger.Debug("fetched document content",
		"document_uuid", revision.DocumentUUID,
		"document_id", revision.DocumentID,
		"provider_type", revision.ProviderType,
		"content_length", len(cleanedContent),
	)

	return cleanedContent, nil
}

// cleanContent cleans and normalizes content for LLM processing.
func (s *LLMSummaryStep) cleanContent(content string) string {
	// Remove excessive whitespace
	content = strings.TrimSpace(content)

	// Normalize line endings
	content = strings.ReplaceAll(content, "\r\n", "\n")

	// Remove multiple consecutive blank lines (keep max 2 newlines)
	for strings.Contains(content, "\n\n\n") {
		content = strings.ReplaceAll(content, "\n\n\n", "\n\n")
	}

	return content
}

// extractDocType extracts document type from revision.
func (s *LLMSummaryStep) extractDocType(revision *models.DocumentRevision) string {
	title := revision.Title
	if strings.HasPrefix(title, "RFC-") {
		return "RFC"
	}
	if strings.HasPrefix(title, "PRD-") {
		return "PRD"
	}
	return "Document"
}

// extractProvider extracts the LLM provider from the model name.
func (s *LLMSummaryStep) extractProvider(model string) string {
	if strings.Contains(model, "gpt") {
		return "openai"
	}
	if strings.Contains(model, "claude") {
		return "anthropic"
	}
	if strings.Contains(model, "gemini") {
		return "google"
	}
	if strings.Contains(model, "llama") {
		return "meta"
	}
	return "unknown"
}

// Configuration helpers

func (s *LLMSummaryStep) getModel(config map[string]interface{}) string {
	if config != nil {
		if model, ok := config["model"].(string); ok {
			return model
		}
	}
	return "gpt-4o-mini" // Default model
}

func (s *LLMSummaryStep) getMaxTokens(config map[string]interface{}) int {
	if config != nil {
		if maxTokens, ok := config["max_tokens"].(int); ok {
			return maxTokens
		}
		if maxTokens, ok := config["max_tokens"].(float64); ok {
			return int(maxTokens)
		}
	}
	return 500 // Default max tokens
}

func (s *LLMSummaryStep) getLanguage(config map[string]interface{}) string {
	if config != nil {
		if language, ok := config["language"].(string); ok {
			return language
		}
	}
	return "en" // Default language
}

func (s *LLMSummaryStep) getStyle(config map[string]interface{}) string {
	if config != nil {
		if style, ok := config["style"].(string); ok {
			return style
		}
	}
	return "executive" // Default style
}

func ptrInt(i int) *int {
	return &i
}

// MockLLMClient is a mock implementation for testing.
type MockLLMClient struct{}

func (m *MockLLMClient) GenerateSummary(ctx context.Context, content string, options SummaryOptions) (*Summary, error) {
	// Return a mock summary
	return &Summary{
		ExecutiveSummary: "This is a mock summary generated for testing purposes.",
		KeyPoints:        []string{"Point 1", "Point 2", "Point 3"},
		Topics:           []string{"Topic A", "Topic B"},
		Tags:             []string{"test", "mock", "summary"},
		Confidence:       0.85,
		TokensUsed:       150,
		GenerationTimeMs: 1200,
	}, nil
}

// MockWorkspaceProvider is a mock implementation for testing.
type MockWorkspaceProvider struct {
	Content map[string]string // Map of document ID to content
	Error   error             // Error to return (if set)
}

func (m *MockWorkspaceProvider) GetDocumentContent(fileID string) (string, error) {
	if m.Error != nil {
		return "", m.Error
	}

	if m.Content != nil {
		if content, ok := m.Content[fileID]; ok {
			return content, nil
		}
	}

	// Default content for testing
	return "This is a test document with some sample content for processing.", nil
}
