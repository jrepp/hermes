//go:build integration
// +build integration

package api

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp-forge/hermes/pkg/docid"
	"github.com/hashicorp-forge/hermes/pkg/models"
	"github.com/hashicorp-forge/hermes/tests/api/fixtures"
)

// TestDocuments_GetByUUID tests the GET /api/v2/documents/:uuid endpoint.
// This validates that documents can be retrieved using their UUID instead of GoogleFileID.
func TestDocuments_GetByUUID(t *testing.T) {
	suite := NewIntegrationSuite(t)
	defer suite.Cleanup()

	t.Run("Get document by UUID", func(t *testing.T) {
		// Create test document with UUID
		uuid := docid.NewUUID()
		doc := fixtures.NewDocument().
			WithGoogleFileID("test-uuid-file-123").
			WithTitle("Test RFC with UUID").
			WithDocType("RFC").
			WithStatus(models.WIPDocumentStatus).
			WithSummary("This document has a UUID").
			WithDocNumber(456).
			Create(t, suite.DB)

		// Set the UUID on the document
		doc.SetDocumentUUID(uuid)
		err := suite.DB.Save(&doc).Error
		require.NoError(t, err, "Failed to save document with UUID")

		// Make GET request using UUID (bare format)
		resp := suite.Client.Get(fmt.Sprintf("/api/v2/documents/%s", uuid.String()))

		// Assert response
		resp.AssertStatusOK()

		// Decode and validate response body
		var result map[string]interface{}
		resp.DecodeJSON(&result)

		// Validate that we got the right document
		assert.Equal(t, doc.GoogleFileID, result["googleFileID"], "googleFileID should match")
		assert.Equal(t, doc.Title, result["title"], "title should match")
		assert.Equal(t, "WIP", result["status"], "status should match")
	})

	t.Run("Get document by UUID with uuid/ prefix", func(t *testing.T) {
		// Create test document with UUID
		uuid := docid.NewUUID()
		doc := fixtures.NewDocument().
			WithGoogleFileID("test-uuid-file-456").
			WithTitle("Test PRD with UUID Prefix").
			WithDocType("PRD").
			WithStatus(models.ApprovedDocumentStatus).
			Create(t, suite.DB)

		// Set the UUID on the document
		doc.SetDocumentUUID(uuid)
		err := suite.DB.Save(&doc).Error
		require.NoError(t, err, "Failed to save document with UUID")

		// Make GET request using UUID with uuid/ prefix
		resp := suite.Client.Get(fmt.Sprintf("/api/v2/documents/uuid/%s", uuid.String()))

		// Assert response
		resp.AssertStatusOK()

		// Decode and validate response body
		var result map[string]interface{}
		resp.DecodeJSON(&result)

		// Validate that we got the right document
		assert.Equal(t, doc.GoogleFileID, result["googleFileID"], "googleFileID should match")
		assert.Equal(t, doc.Title, result["title"], "title should match")
		assert.Equal(t, "Approved", result["status"], "status should match")
	})

	t.Run("Get document by GoogleFileID still works", func(t *testing.T) {
		// Create document with GoogleFileID but no UUID
		doc := fixtures.NewDocument().
			WithGoogleFileID("test-legacy-file-789").
			WithTitle("Legacy Document without UUID").
			WithDocType("RFC").
			WithStatus(models.InReviewDocumentStatus).
			Create(t, suite.DB)

		// Make GET request using GoogleFileID
		resp := suite.Client.Get(fmt.Sprintf("/api/v2/documents/%s", doc.GoogleFileID))

		// Assert response
		resp.AssertStatusOK()

		// Decode and validate response body
		var result map[string]interface{}
		resp.DecodeJSON(&result)

		// Validate that we got the right document
		assert.Equal(t, doc.GoogleFileID, result["googleFileID"], "googleFileID should match")
		assert.Equal(t, doc.Title, result["title"], "title should match")
		assert.Equal(t, "In-Review", result["status"], "status should match")
	})

	t.Run("Non-existent UUID returns 404", func(t *testing.T) {
		// Create a UUID that doesn't exist in database
		nonExistentUUID := docid.NewUUID()

		// Make GET request
		resp := suite.Client.Get(fmt.Sprintf("/api/v2/documents/%s", nonExistentUUID.String()))

		// Should return 404
		resp.AssertStatusNotFound()
	})

	t.Run("Invalid UUID format falls back to GoogleFileID lookup", func(t *testing.T) {
		// Try to get document with invalid UUID format (should try GoogleFileID lookup)
		resp := suite.Client.Get("/api/v2/documents/not-a-valid-uuid-or-file-id")

		// Should return 404 (not found as GoogleFileID either)
		resp.AssertStatusNotFound()
	})

	t.Run("Document with both UUID and GoogleFileID accessible by either", func(t *testing.T) {
		// Create test document with both identifiers
		uuid := docid.NewUUID()
		doc := fixtures.NewDocument().
			WithGoogleFileID("test-dual-id-123").
			WithTitle("Document with Dual IDs").
			WithDocType("FRD").
			WithStatus(models.ApprovedDocumentStatus).
			Create(t, suite.DB)

		// Set the UUID on the document
		doc.SetDocumentUUID(uuid)
		providerType := "google"
		projectID := "rfc-archive"
		doc.ProviderType = &providerType
		doc.ProjectID = &projectID
		err := suite.DB.Save(&doc).Error
		require.NoError(t, err, "Failed to save document with UUID")

		// Test 1: Access by UUID
		resp1 := suite.Client.Get(fmt.Sprintf("/api/v2/documents/%s", uuid.String()))
		resp1.AssertStatusOK()

		var result1 map[string]interface{}
		resp1.DecodeJSON(&result1)
		assert.Equal(t, doc.GoogleFileID, result1["googleFileID"], "Should get same document by UUID")
		assert.Equal(t, doc.Title, result1["title"], "Title should match")

		// Test 2: Access by GoogleFileID
		resp2 := suite.Client.Get(fmt.Sprintf("/api/v2/documents/%s", doc.GoogleFileID))
		resp2.AssertStatusOK()

		var result2 map[string]interface{}
		resp2.DecodeJSON(&result2)
		assert.Equal(t, doc.GoogleFileID, result2["googleFileID"], "Should get same document by GoogleFileID")
		assert.Equal(t, doc.Title, result2["title"], "Title should match")

		// Both methods should return the same document
		assert.Equal(t, result1["googleFileID"], result2["googleFileID"], "Both lookups should return same document")
	})
}

// TestDocuments_PatchByUUID tests the PATCH /api/v2/documents/:uuid endpoint.
// This validates that documents can be updated using their UUID.
func TestDocuments_PatchByUUID(t *testing.T) {
	suite := NewIntegrationSuite(t)
	defer suite.Cleanup()

	t.Run("Patch document by UUID", func(t *testing.T) {
		// Create test document with UUID
		uuid := docid.NewUUID()
		doc := fixtures.NewDocument().
			WithGoogleFileID("test-patch-uuid-123").
			WithTitle("Original Title").
			WithDocType("RFC").
			WithStatus(models.WIPDocumentStatus).
			Create(t, suite.DB)

		// Set the UUID on the document
		doc.SetDocumentUUID(uuid)
		err := suite.DB.Save(&doc).Error
		require.NoError(t, err, "Failed to save document with UUID")

		// Patch the document using UUID
		patchData := map[string]interface{}{
			"title": "Updated Title via UUID",
		}
		resp := suite.Client.Patch(fmt.Sprintf("/api/v2/documents/%s", uuid.String()), patchData)

		// Assert response
		resp.AssertStatusOK()

		// Verify the document was updated
		var updated models.Document
		err = updated.GetByUUID(suite.DB, uuid)
		require.NoError(t, err, "Failed to retrieve updated document")
		assert.Equal(t, "Updated Title via UUID", updated.Title, "Title should be updated")
	})
}

// TestDocuments_DeleteByUUID tests the DELETE /api/v2/documents/:uuid endpoint.
// This validates that documents can be deleted using their UUID.
func TestDocuments_DeleteByUUID(t *testing.T) {
	suite := NewIntegrationSuite(t)
	defer suite.Cleanup()

	t.Run("Delete document by UUID", func(t *testing.T) {
		// Create test document with UUID
		uuid := docid.NewUUID()
		doc := fixtures.NewDocument().
			WithGoogleFileID("test-delete-uuid-123").
			WithTitle("Document to Delete").
			WithDocType("RFC").
			WithStatus(models.WIPDocumentStatus).
			Create(t, suite.DB)

		// Set the UUID on the document
		doc.SetDocumentUUID(uuid)
		err := suite.DB.Save(&doc).Error
		require.NoError(t, err, "Failed to save document with UUID")

		// Delete the document using UUID
		resp := suite.Client.Delete(fmt.Sprintf("/api/v2/documents/%s", uuid.String()))

		// Assert response (status may vary, typically 200 or 204)
		assert.True(t, resp.StatusCode >= 200 && resp.StatusCode < 300, "Delete should succeed")

		// Verify the document was deleted
		var deleted models.Document
		err = deleted.GetByUUID(suite.DB, uuid)
		assert.Error(t, err, "Document should not exist after deletion")
	})
}
