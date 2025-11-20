package indexer

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/hashicorp-forge/hermes/pkg/document"
	"github.com/hashicorp-forge/hermes/pkg/models"
	"github.com/hashicorp-forge/hermes/pkg/workspace"
)

// DocumentContext holds all information about a document being processed
// through the indexer pipeline. It accumulates state as commands execute.
type DocumentContext struct {
	// Source document from workspace provider
	Document *workspace.Document

	// UUID and Revision Tracking
	DocumentUUID uuid.UUID                // Stable identifier across providers
	ContentHash  string                   // SHA-256 hash for change detection
	Revision     *models.DocumentRevision // Current revision info

	// Database metadata
	Metadata     *models.Document
	Reviews      models.DocumentReviews
	GroupReviews models.DocumentGroupReviews

	// Processing state
	Content     string             // Extracted document content
	Transformed *document.Document // Transformed for search indexing

	// Provider references
	SourceProvider workspace.DocumentStorage
	TargetProvider workspace.DocumentStorage
	TargetFolderID string
	TargetDocument *workspace.Document

	// Migration tracking
	MigrationStatus string        // "none", "source", "target", "conflict", "canonical"
	ConflictInfo    *ConflictInfo // Details about migration conflicts

	// Tracking
	StartTime time.Time
	Errors    []error

	// Custom data that commands can use to pass information
	Custom map[string]any
}

// ConflictInfo tracks migration conflicts between providers.
type ConflictInfo struct {
	DetectedAt    time.Time
	ConflictType  string // "concurrent-edit", "content-divergence", etc.
	SourceHash    string
	TargetHash    string
	SourceModTime time.Time
	TargetModTime time.Time
	Resolution    string // "pending", "source-wins", "target-wins", "manual"
}

// LoadMetadata loads database metadata for the document.
// This populates the Metadata, Reviews, and GroupReviews fields
// from the database.
func (dc *DocumentContext) LoadMetadata(db *gorm.DB) error {
	if dc.Metadata != nil {
		return nil // Already loaded
	}

	// Get document from database
	dbDoc := models.Document{
		GoogleFileID: dc.Document.ID,
	}
	if err := dbDoc.Get(db); err != nil {
		return err
	}
	dc.Metadata = &dbDoc

	// Get reviews
	if err := dc.Reviews.Find(db, models.DocumentReview{
		Document: models.Document{
			GoogleFileID: dc.Document.ID,
		},
	}); err != nil {
		return err
	}

	// Get group reviews
	if err := dc.GroupReviews.Find(db, models.DocumentGroupReview{
		Document: models.Document{
			GoogleFileID: dc.Document.ID,
		},
	}); err != nil {
		return err
	}

	return nil
}

// AddError adds an error to the context without failing immediately.
// This allows the pipeline to collect multiple errors for reporting.
func (dc *DocumentContext) AddError(err error) {
	dc.Errors = append(dc.Errors, err)
}

// HasErrors returns true if any errors occurred during processing.
func (dc *DocumentContext) HasErrors() bool {
	return len(dc.Errors) > 0
}

// SetCustom sets a custom value that can be used by commands to
// pass information down the pipeline.
func (dc *DocumentContext) SetCustom(key string, value any) {
	if dc.Custom == nil {
		dc.Custom = make(map[string]any)
	}
	dc.Custom[key] = value
}

// GetCustom retrieves a custom value from the context.
func (dc *DocumentContext) GetCustom(key string) (any, bool) {
	if dc.Custom == nil {
		return nil, false
	}
	val, ok := dc.Custom[key]
	return val, ok
}

// DocumentFilter is a function that determines whether a document
// should be processed by a pipeline.
type DocumentFilter func(*DocumentContext) bool

// RecentlyModifiedFilter creates a filter that skips documents
// modified within the specified duration.
// This is useful to avoid disrupting users who are actively editing.
func RecentlyModifiedFilter(within time.Duration) DocumentFilter {
	return func(doc *DocumentContext) bool {
		return time.Since(doc.Document.ModifiedTime) > within
	}
}

// DocumentTypeFilter creates a filter that only processes documents
// of the specified types.
func DocumentTypeFilter(docTypes ...string) DocumentFilter {
	typeMap := make(map[string]bool)
	for _, t := range docTypes {
		typeMap[t] = true
	}

	return func(doc *DocumentContext) bool {
		if doc.Metadata == nil || doc.Metadata.DocumentType.Name == "" {
			return false
		}
		return typeMap[doc.Metadata.DocumentType.Name]
	}
}

// StatusFilter creates a filter that only processes documents
// with the specified status values.
func StatusFilter(statuses ...models.DocumentStatus) DocumentFilter {
	statusMap := make(map[models.DocumentStatus]bool)
	for _, s := range statuses {
		statusMap[s] = true
	}

	return func(doc *DocumentContext) bool {
		if doc.Metadata == nil {
			return false
		}
		return statusMap[doc.Metadata.Status]
	}
}

// CombineFilters combines multiple filters with AND logic.
// The document must pass all filters to be processed.
func CombineFilters(filters ...DocumentFilter) DocumentFilter {
	return func(doc *DocumentContext) bool {
		for _, filter := range filters {
			if !filter(doc) {
				return false
			}
		}
		return true
	}
}
