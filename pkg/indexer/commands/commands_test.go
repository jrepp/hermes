package commands_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp-forge/hermes/pkg/indexer"
	"github.com/hashicorp-forge/hermes/pkg/indexer/commands"
	"github.com/hashicorp-forge/hermes/pkg/workspace"
)

// MockDocumentStorage is a simple mock for testing
type MockDocumentStorage struct {
	documents map[string]*workspace.Document
	content   map[string]string
}

func NewMockDocumentStorage() *MockDocumentStorage {
	return &MockDocumentStorage{
		documents: make(map[string]*workspace.Document),
		content:   make(map[string]string),
	}
}

func (m *MockDocumentStorage) GetDocument(ctx context.Context, id string) (*workspace.Document, error) {
	if doc, ok := m.documents[id]; ok {
		return doc, nil
	}
	return nil, nil
}

func (m *MockDocumentStorage) CreateDocument(ctx context.Context, doc *workspace.DocumentCreate) (*workspace.Document, error) {
	return nil, nil
}

func (m *MockDocumentStorage) UpdateDocument(ctx context.Context, id string, updates *workspace.DocumentUpdate) (*workspace.Document, error) {
	return nil, nil
}

func (m *MockDocumentStorage) DeleteDocument(ctx context.Context, id string) error {
	delete(m.documents, id)
	return nil
}

func (m *MockDocumentStorage) GetDocumentContent(ctx context.Context, id string) (string, error) {
	if content, ok := m.content[id]; ok {
		return content, nil
	}
	return "", nil
}

func (m *MockDocumentStorage) UpdateDocumentContent(ctx context.Context, id string, content string) error {
	m.content[id] = content
	return nil
}

func (m *MockDocumentStorage) ReplaceTextInDocument(ctx context.Context, id string, replacements map[string]string) error {
	return nil
}

func (m *MockDocumentStorage) CopyDocument(ctx context.Context, sourceID, destFolderID, name string) (*workspace.Document, error) {
	return nil, nil
}

func (m *MockDocumentStorage) MoveDocument(ctx context.Context, docID, destFolderID string) error {
	return nil
}

func (m *MockDocumentStorage) CreateFolder(ctx context.Context, name, parentID string) (*workspace.Folder, error) {
	return nil, nil
}

func (m *MockDocumentStorage) GetFolder(ctx context.Context, id string) (*workspace.Folder, error) {
	return nil, nil
}

func (m *MockDocumentStorage) ListFolders(ctx context.Context, parentID string) ([]*workspace.Folder, error) {
	return nil, nil
}

func (m *MockDocumentStorage) GetSubfolder(ctx context.Context, parentID, name string) (*workspace.Folder, error) {
	return nil, nil
}

func (m *MockDocumentStorage) ListRevisions(ctx context.Context, docID string) ([]*workspace.Revision, error) {
	return nil, nil
}

func (m *MockDocumentStorage) GetRevision(ctx context.Context, docID, revisionID string) (*workspace.Revision, error) {
	return nil, nil
}

func (m *MockDocumentStorage) GetLatestRevision(ctx context.Context, docID string) (*workspace.Revision, error) {
	return nil, nil
}

func (m *MockDocumentStorage) ListDocuments(ctx context.Context, folderID string, opts *workspace.ListOptions) ([]*workspace.Document, error) {
	docs := make([]*workspace.Document, 0)
	for _, doc := range m.documents {
		if doc.ParentFolderID == folderID {
			// Apply filters
			if opts.ModifiedAfter != nil && doc.ModifiedTime.Before(*opts.ModifiedAfter) {
				continue
			}
			docs = append(docs, doc)
		}
	}
	return docs, nil
}

func TestExtractContentCommand(t *testing.T) {
	t.Run("extracts content successfully", func(t *testing.T) {
		// Setup
		mockProvider := NewMockDocumentStorage()
		mockProvider.content["doc-123"] = "Document content here"

		cmd := &commands.ExtractContentCommand{
			Provider: mockProvider,
			MaxSize:  0, // No limit
		}

		doc := &indexer.DocumentContext{
			Document: &workspace.Document{
				ID:   "doc-123",
				Name: "Test Doc",
			},
		}

		// Execute
		err := cmd.Execute(context.Background(), doc)
		require.NoError(t, err)
		assert.Equal(t, "Document content here", doc.Content)
	})

	t.Run("trims content when exceeds max size", func(t *testing.T) {
		mockProvider := NewMockDocumentStorage()
		mockProvider.content["doc-123"] = "This is a very long document content"

		cmd := &commands.ExtractContentCommand{
			Provider: mockProvider,
			MaxSize:  10, // Trim to 10 bytes
		}

		doc := &indexer.DocumentContext{
			Document: &workspace.Document{ID: "doc-123"},
		}

		err := cmd.Execute(context.Background(), doc)
		require.NoError(t, err)
		assert.Equal(t, "This is a ", doc.Content)
		assert.Len(t, doc.Content, 10)
	})
}

func TestDiscoverCommand(t *testing.T) {
	t.Run("discovers documents in folder", func(t *testing.T) {
		// Setup
		mockProvider := NewMockDocumentStorage()
		now := time.Now()

		mockProvider.documents["doc-1"] = &workspace.Document{
			ID:             "doc-1",
			Name:           "Doc 1",
			ParentFolderID: "docs",
			ModifiedTime:   now.Add(-1 * time.Hour),
		}
		mockProvider.documents["doc-2"] = &workspace.Document{
			ID:             "doc-2",
			Name:           "Doc 2",
			ParentFolderID: "docs",
			ModifiedTime:   now.Add(-30 * time.Minute),
		}
		mockProvider.documents["doc-3"] = &workspace.Document{
			ID:             "doc-3",
			Name:           "Doc 3",
			ParentFolderID: "drafts", // Different folder
			ModifiedTime:   now.Add(-1 * time.Hour),
		}

		cmd := &commands.DiscoverCommand{
			Provider: mockProvider,
			FolderID: "docs",
		}

		// Execute
		discovered, err := cmd.Discover(context.Background())
		require.NoError(t, err)
		assert.Len(t, discovered, 2)

		// Verify doc-3 was not discovered (different folder)
		for _, doc := range discovered {
			assert.NotEqual(t, "doc-3", doc.Document.ID)
		}
	})

	t.Run("filters by modified time", func(t *testing.T) {
		mockProvider := NewMockDocumentStorage()
		now := time.Now()
		since := now.Add(-45 * time.Minute)

		mockProvider.documents["doc-1"] = &workspace.Document{
			ID:             "doc-1",
			Name:           "Doc 1",
			ParentFolderID: "docs",
			ModifiedTime:   now.Add(-1 * time.Hour), // Too old
		}
		mockProvider.documents["doc-2"] = &workspace.Document{
			ID:             "doc-2",
			Name:           "Doc 2",
			ParentFolderID: "docs",
			ModifiedTime:   now.Add(-30 * time.Minute), // Should be included
		}

		cmd := &commands.DiscoverCommand{
			Provider: mockProvider,
			FolderID: "docs",
			Since:    &since,
		}

		discovered, err := cmd.Discover(context.Background())
		require.NoError(t, err)
		assert.Len(t, discovered, 1)
		assert.Equal(t, "doc-2", discovered[0].Document.ID)
	})
}
