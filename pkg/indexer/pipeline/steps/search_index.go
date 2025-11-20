package steps

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp-forge/hermes/pkg/models"
	"github.com/hashicorp-forge/hermes/pkg/search"
)

// SearchIndexStep updates the search index (Meilisearch/Algolia) for a document revision.
type SearchIndexStep struct {
	searchProvider search.Provider
	logger         hclog.Logger
}

// NewSearchIndexStep creates a new search index step.
func NewSearchIndexStep(searchProvider search.Provider, logger hclog.Logger) *SearchIndexStep {
	if logger == nil {
		logger = hclog.NewNullLogger()
	}

	return &SearchIndexStep{
		searchProvider: searchProvider,
		logger:         logger.Named("search-index-step"),
	}
}

// Name returns the step name.
func (s *SearchIndexStep) Name() string {
	return "search_index"
}

// Execute updates the search index for the given revision.
func (s *SearchIndexStep) Execute(ctx context.Context, revision *models.DocumentRevision, config map[string]interface{}) error {
	s.logger.Debug("executing search index step",
		"document_uuid", revision.DocumentUUID,
		"revision_id", revision.ID,
		"provider", s.searchProvider.Name(),
	)

	// Convert revision to search document
	doc, err := s.revisionToSearchDocument(revision)
	if err != nil {
		return fmt.Errorf("failed to convert revision to search document: %w", err)
	}

	// Determine which index to use based on status
	var indexer interface {
		Index(ctx context.Context, doc *search.Document) error
	}

	// Check if this is a draft or published document
	if s.isDraft(revision) {
		indexer = s.searchProvider.DraftIndex()
	} else {
		indexer = s.searchProvider.DocumentIndex()
	}

	// Index the document
	if err := indexer.Index(ctx, doc); err != nil {
		return fmt.Errorf("failed to index document: %w", err)
	}

	s.logger.Info("indexed document in search",
		"document_uuid", revision.DocumentUUID,
		"revision_id", revision.ID,
		"object_id", doc.ObjectID,
	)

	return nil
}

// IsRetryable determines if an error should trigger a retry.
func (s *SearchIndexStep) IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	// Check for retryable errors
	errMsg := strings.ToLower(err.Error())

	// Network errors are retryable
	if strings.Contains(errMsg, "timeout") ||
		strings.Contains(errMsg, "connection refused") ||
		strings.Contains(errMsg, "temporary") {
		return true
	}

	// Rate limiting is retryable
	if strings.Contains(errMsg, "rate limit") ||
		strings.Contains(errMsg, "too many requests") {
		return true
	}

	// Backend unavailable is retryable
	if strings.Contains(errMsg, "unavailable") ||
		strings.Contains(errMsg, "service unavailable") {
		return true
	}

	// Other errors are not retryable (e.g., validation errors)
	return false
}

// revisionToSearchDocument converts a DocumentRevision to a search.Document.
func (s *SearchIndexStep) revisionToSearchDocument(revision *models.DocumentRevision) (*search.Document, error) {
	// Build search document from revision
	doc := &search.Document{
		ObjectID:     revision.DocumentID,
		Title:        revision.Title,
		DocType:      s.extractDocType(revision),
		Status:       revision.Status,
		Product:      s.extractProduct(revision),
		ModifiedTime: revision.ModifiedTime.Unix(),
		CreatedTime:  revision.CreatedAt.Unix(),
	}

	// Note: This is a basic implementation. In a real system, you would:
	// 1. Fetch the full document content from the workspace provider
	// 2. Extract additional metadata (owners, contributors, summary, etc.)
	// 3. Parse and format the content for search
	//
	// For now, we're creating a minimal document for the search index.
	// The full implementation would look like:
	//
	// content, err := s.fetchDocumentContent(revision)
	// if err != nil {
	//     return nil, err
	// }
	// doc.Content = content
	//
	// metadata, err := s.fetchDocumentMetadata(revision)
	// if err != nil {
	//     return nil, err
	// }
	// doc.Owners = metadata.Owners
	// doc.Contributors = metadata.Contributors
	// ...

	return doc, nil
}

// isDraft determines if the revision represents a draft document.
func (s *SearchIndexStep) isDraft(revision *models.DocumentRevision) bool {
	// Draft detection logic:
	// 1. Status is "draft" or "in-progress"
	// 2. Provider folder indicates drafts
	// 3. Document type indicates draft
	status := strings.ToLower(revision.Status)
	return status == "draft" || status == "in-progress" || status == "wip"
}

// extractDocType extracts the document type from the revision.
func (s *SearchIndexStep) extractDocType(revision *models.DocumentRevision) string {
	// This is a placeholder. In a real implementation, you would:
	// 1. Parse the document title (e.g., "RFC-088" -> "RFC")
	// 2. Check metadata or document properties
	// 3. Use a document type registry

	title := revision.Title
	if strings.HasPrefix(title, "RFC-") {
		return "RFC"
	}
	if strings.HasPrefix(title, "PRD-") {
		return "PRD"
	}

	return "Document"
}

// extractProduct extracts the product from the revision.
func (s *SearchIndexStep) extractProduct(revision *models.DocumentRevision) string {
	// This is a placeholder. In a real implementation, you would:
	// 1. Look up the project associated with the revision
	// 2. Get the product from the project
	// 3. Use metadata from the document

	if revision.ProjectUUID != nil {
		// In a real implementation: fetch project and return project.Product
		// For now, return empty string
		return ""
	}

	return ""
}

// TODO: Add these helper methods in a full implementation:
//
// func (s *SearchIndexStep) fetchDocumentContent(revision *models.DocumentRevision) (string, error) {
//     // Fetch content from workspace provider based on provider_type
//     // This would use the workspace abstraction to get content
// }
//
// func (s *SearchIndexStep) fetchDocumentMetadata(revision *models.DocumentRevision) (*DocumentMetadata, error) {
//     // Fetch metadata from database or workspace provider
// }
