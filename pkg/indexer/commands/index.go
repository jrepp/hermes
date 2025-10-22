package commands

import (
	"context"
	"fmt"

	"github.com/hashicorp-forge/hermes/pkg/document"
	"github.com/hashicorp-forge/hermes/pkg/indexer"
	"github.com/hashicorp-forge/hermes/pkg/search"
)

// IndexType specifies which search index to use.
type IndexType string

const (
	IndexTypePublished IndexType = "published"
	IndexTypeDrafts    IndexType = "drafts"
)

// IndexCommand indexes a document in the search provider.
// It can index into either the published or drafts index.
type IndexCommand struct {
	SearchProvider search.Provider
	IndexType      IndexType
	BatchSize      int // For batch operations
}

// Name returns the command name.
func (c *IndexCommand) Name() string {
	return fmt.Sprintf("index-%s", c.IndexType)
}

// Execute indexes a single document.
func (c *IndexCommand) Execute(ctx context.Context, doc *indexer.DocumentContext) error {
	if doc.Transformed == nil {
		return fmt.Errorf("document not transformed, run transform command first")
	}

	// Convert to search document format
	searchDoc, err := c.toSearchDocument(doc.Transformed)
	if err != nil {
		return fmt.Errorf("failed to convert to search document: %w", err)
	}

	// Index in appropriate index
	var idx search.DocumentIndex
	switch c.IndexType {
	case IndexTypePublished:
		idx = c.SearchProvider.DocumentIndex()
	case IndexTypeDrafts:
		idx = c.SearchProvider.DraftIndex()
	default:
		return fmt.Errorf("unknown index type: %s", c.IndexType)
	}

	if err := idx.Index(ctx, searchDoc); err != nil {
		return fmt.Errorf("failed to index document: %w", err)
	}

	return nil
}

// ExecuteBatch implements BatchCommand for batch indexing.
func (c *IndexCommand) ExecuteBatch(ctx context.Context, docs []*indexer.DocumentContext) error {
	searchDocs := make([]*search.Document, 0, len(docs))

	for _, doc := range docs {
		if doc.Transformed == nil {
			// Skip documents that weren't transformed
			continue
		}

		searchDoc, err := c.toSearchDocument(doc.Transformed)
		if err != nil {
			doc.AddError(fmt.Errorf("failed to convert to search document: %w", err))
			continue
		}
		searchDocs = append(searchDocs, searchDoc)
	}

	if len(searchDocs) == 0 {
		return nil
	}

	var idx search.DocumentIndex
	switch c.IndexType {
	case IndexTypePublished:
		idx = c.SearchProvider.DocumentIndex()
	case IndexTypeDrafts:
		idx = c.SearchProvider.DraftIndex()
	default:
		return fmt.Errorf("unknown index type: %s", c.IndexType)
	}

	return idx.IndexBatch(ctx, searchDocs)
}

// toSearchDocument converts a document.Document to search.Document
func (c *IndexCommand) toSearchDocument(doc *document.Document) (*search.Document, error) {
	// Create search document from the Hermes document
	searchDoc := &search.Document{
		ObjectID:     doc.ObjectID,
		DocID:        doc.ObjectID,
		Title:        doc.Title,
		DocNumber:    doc.DocNumber,
		DocType:      doc.DocType,
		Product:      doc.Product,
		Status:       doc.Status,
		Owners:       doc.Owners,
		Contributors: doc.Contributors,
		Approvers:    doc.Approvers,
		Summary:      doc.Summary,
		Content:      doc.Content,
		CreatedTime:  doc.CreatedTime,
		ModifiedTime: doc.ModifiedTime,
		CustomFields: make(map[string]interface{}),
	}

	// Add custom fields
	for _, cf := range doc.CustomFields {
		searchDoc.CustomFields[cf.Name] = cf.Value
	}

	return searchDoc, nil
}
