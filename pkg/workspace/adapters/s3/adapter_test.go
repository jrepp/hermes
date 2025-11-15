package s3

import (
	"context"
	"os"
	"testing"

	"github.com/hashicorp-forge/hermes/pkg/docid"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestS3AdapterIntegration tests the S3 adapter against MinIO
// Run with: go test -v -tags=integration ./pkg/workspace/adapters/s3
// Requires: MinIO running on localhost:9000 (docker compose up -d minio)
func TestS3AdapterIntegration(t *testing.T) {
	if os.Getenv("INTEGRATION_TEST") == "" {
		t.Skip("Skipping integration test. Set INTEGRATION_TEST=1 to run")
	}

	// Configure S3 adapter to connect to MinIO
	cfg := &Config{
		Endpoint:          "http://localhost:9000",
		Region:            "us-east-1",
		Bucket:            "hermes-documents",
		Prefix:            "test",
		AccessKey:         "minioadmin",
		SecretKey:         "minioadmin",
		VersioningEnabled: true,
		MetadataStore:     "manifest", // Use manifest store for integration test
		UseSSL:            false,
		DefaultMimeType:   "text/markdown",
	}

	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "s3-test",
		Level: hclog.Debug,
	})

	adapter, err := NewAdapter(cfg, logger)
	require.NoError(t, err, "Failed to create S3 adapter")
	require.NotNil(t, adapter, "Adapter should not be nil")

	ctx := context.Background()

	t.Run("CreateDocument", func(t *testing.T) {
		// Create a new document
		doc, err := adapter.CreateDocument(ctx, "", "", "Test Document RFC-089")
		require.NoError(t, err, "Failed to create document")
		require.NotNil(t, doc, "Document should not be nil")

		assert.NotEmpty(t, doc.UUID, "Document UUID should not be empty")
		assert.Equal(t, "Test Document RFC-089", doc.Name)
		assert.Equal(t, "s3", doc.ProviderType)
		assert.Contains(t, doc.ProviderID, "s3:")
		assert.Equal(t, "canonical", doc.SyncStatus)

		t.Logf("Created document: UUID=%s, ProviderID=%s", doc.UUID, doc.ProviderID)

		// Store UUID for later tests
		t.Cleanup(func() {
			// Clean up test document
			_ = adapter.DeleteDocument(context.Background(), doc.ProviderID)
		})
	})

	t.Run("CreateAndReadDocument", func(t *testing.T) {
		// Create document
		doc, err := adapter.CreateDocument(ctx, "", "", "Test Read Document")
		require.NoError(t, err)

		// Read document back
		readDoc, err := adapter.GetDocument(ctx, doc.ProviderID)
		require.NoError(t, err, "Failed to read document")
		assert.Equal(t, doc.UUID, readDoc.UUID)
		assert.Equal(t, doc.Name, readDoc.Name)

		t.Cleanup(func() {
			_ = adapter.DeleteDocument(context.Background(), doc.ProviderID)
		})
	})

	t.Run("UpdateContent", func(t *testing.T) {
		// Create document
		doc, err := adapter.CreateDocument(ctx, "", "", "Test Update Document")
		require.NoError(t, err)

		// Update content
		newContent := "# Updated Content\n\nThis is the updated content for RFC-089 testing."
		updatedDoc, err := adapter.UpdateContent(ctx, doc.ProviderID, newContent)
		require.NoError(t, err, "Failed to update content")
		assert.Equal(t, newContent, updatedDoc.Body)
		assert.NotEmpty(t, updatedDoc.ContentHash)

		// Verify content was updated
		content, err := adapter.GetContent(ctx, doc.ProviderID)
		require.NoError(t, err)
		assert.Equal(t, newContent, content.Body)

		t.Cleanup(func() {
			_ = adapter.DeleteDocument(context.Background(), doc.ProviderID)
		})
	})

	t.Run("RevisionHistory", func(t *testing.T) {
		if !cfg.VersioningEnabled {
			t.Skip("Versioning not enabled")
		}

		// Create document
		doc, err := adapter.CreateDocument(ctx, "", "", "Test Revision Document")
		require.NoError(t, err)

		// Update content multiple times to create revisions
		for i := 1; i <= 3; i++ {
			content := "# Revision " + string(rune('0'+i)) + "\n\nContent version " + string(rune('0'+i))
			_, err = adapter.UpdateContent(ctx, doc.ProviderID, content)
			require.NoError(t, err, "Failed to update content for revision %d", i)
		}

		// Get revision history
		revisions, err := adapter.GetRevisionHistory(ctx, doc.ProviderID, 10)
		require.NoError(t, err, "Failed to get revision history")
		assert.GreaterOrEqual(t, len(revisions), 3, "Should have at least 3 revisions")

		t.Logf("Found %d revisions", len(revisions))
		for i, rev := range revisions {
			t.Logf("Revision %d: ID=%s, ModifiedTime=%s", i, rev.RevisionID, rev.ModifiedTime)
		}

		t.Cleanup(func() {
			_ = adapter.DeleteDocument(context.Background(), doc.ProviderID)
		})
	})

	t.Run("GetDocumentByUUID", func(t *testing.T) {
		// Create document
		doc, err := adapter.CreateDocument(ctx, "", "", "Test UUID Lookup")
		require.NoError(t, err)

		// Find by UUID
		foundDoc, err := adapter.GetDocumentByUUID(ctx, doc.UUID)
		require.NoError(t, err, "Failed to find document by UUID")
		assert.Equal(t, doc.UUID, foundDoc.UUID)
		assert.Equal(t, doc.ProviderID, foundDoc.ProviderID)

		t.Cleanup(func() {
			_ = adapter.DeleteDocument(context.Background(), doc.ProviderID)
		})
	})

	t.Run("CopyDocument", func(t *testing.T) {
		// Create source document
		sourceDoc, err := adapter.CreateDocument(ctx, "", "", "Source Document")
		require.NoError(t, err)

		// Add content to source
		sourceContent := "# Source Content\n\nThis is the original content."
		_, err = adapter.UpdateContent(ctx, sourceDoc.ProviderID, sourceContent)
		require.NoError(t, err)

		// Copy document
		copiedDoc, err := adapter.CopyDocument(ctx, sourceDoc.ProviderID, "", "Copied Document")
		require.NoError(t, err, "Failed to copy document")
		assert.NotEqual(t, sourceDoc.UUID, copiedDoc.UUID, "Copied document should have different UUID")
		assert.Equal(t, "Copied Document", copiedDoc.Name)

		// Verify copied content
		copiedContent, err := adapter.GetContent(ctx, copiedDoc.ProviderID)
		require.NoError(t, err)
		assert.Equal(t, sourceContent, copiedContent.Body)

		t.Cleanup(func() {
			_ = adapter.DeleteDocument(context.Background(), sourceDoc.ProviderID)
			_ = adapter.DeleteDocument(context.Background(), copiedDoc.ProviderID)
		})
	})

	t.Run("RenameDocument", func(t *testing.T) {
		// Create document
		doc, err := adapter.CreateDocument(ctx, "", "", "Original Name")
		require.NoError(t, err)

		// Rename document
		err = adapter.RenameDocument(ctx, doc.ProviderID, "New Name")
		require.NoError(t, err, "Failed to rename document")

		// Verify new name
		renamedDoc, err := adapter.GetDocument(ctx, doc.ProviderID)
		require.NoError(t, err)
		assert.Equal(t, "New Name", renamedDoc.Name)

		t.Cleanup(func() {
			_ = adapter.DeleteDocument(context.Background(), doc.ProviderID)
		})
	})

	t.Run("DeleteDocument", func(t *testing.T) {
		// Create document
		doc, err := adapter.CreateDocument(ctx, "", "", "Document To Delete")
		require.NoError(t, err)

		// Delete document
		err = adapter.DeleteDocument(ctx, doc.ProviderID)
		require.NoError(t, err, "Failed to delete document")

		// Verify document is gone
		_, err = adapter.GetDocument(ctx, doc.ProviderID)
		assert.Error(t, err, "Document should not exist after deletion")
	})

	t.Run("CompareContent", func(t *testing.T) {
		// Create two documents with same content
		doc1, err := adapter.CreateDocument(ctx, "", "", "Doc 1")
		require.NoError(t, err)
		doc2, err := adapter.CreateDocument(ctx, "", "", "Doc 2")
		require.NoError(t, err)

		content := "# Same Content\n\nBoth documents have this content."
		_, err = adapter.UpdateContent(ctx, doc1.ProviderID, content)
		require.NoError(t, err)
		_, err = adapter.UpdateContent(ctx, doc2.ProviderID, content)
		require.NoError(t, err)

		// Compare content
		comparison, err := adapter.CompareContent(ctx, doc1.ProviderID, doc2.ProviderID)
		require.NoError(t, err, "Failed to compare content")
		assert.True(t, comparison.ContentMatch, "Content should match")
		assert.Equal(t, "same", comparison.HashDifference)

		t.Cleanup(func() {
			_ = adapter.DeleteDocument(context.Background(), doc1.ProviderID)
			_ = adapter.DeleteDocument(context.Background(), doc2.ProviderID)
		})
	})
}

// TestS3AdapterWithUUID tests creating a document with a specific UUID (for migration)
func TestS3AdapterWithUUID(t *testing.T) {
	if os.Getenv("INTEGRATION_TEST") == "" {
		t.Skip("Skipping integration test. Set INTEGRATION_TEST=1 to run")
	}

	cfg := &Config{
		Endpoint:          "http://localhost:9000",
		Region:            "us-east-1",
		Bucket:            "hermes-documents",
		Prefix:            "test-uuid",
		AccessKey:         "minioadmin",
		SecretKey:         "minioadmin",
		VersioningEnabled: true,
		MetadataStore:     "manifest",
		UseSSL:            false,
	}

	logger := hclog.NewNullLogger()
	adapter, err := NewAdapter(cfg, logger)
	require.NoError(t, err)

	ctx := context.Background()

	// Create a specific UUID
	uuid := docid.NewUUID()

	// Create document with this UUID
	doc, err := adapter.CreateDocumentWithUUID(ctx, uuid, "", "", "Document with UUID")
	require.NoError(t, err, "Failed to create document with UUID")
	assert.Equal(t, uuid, doc.UUID, "Document should have the specified UUID")

	// Verify we can find it by UUID
	foundDoc, err := adapter.GetDocumentByUUID(ctx, uuid)
	require.NoError(t, err)
	assert.Equal(t, doc.ProviderID, foundDoc.ProviderID)

	t.Cleanup(func() {
		_ = adapter.DeleteDocument(context.Background(), doc.ProviderID)
	})
}
