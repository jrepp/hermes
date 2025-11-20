package models

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp-forge/hermes/pkg/docid"
)

func TestDocument_GetDocumentUUID(t *testing.T) {
	t.Run("returns existing UUID", func(t *testing.T) {
		uuid := docid.NewUUID()
		doc := &Document{
			DocumentUUID: &uuid,
		}
		result := doc.GetDocumentUUID()
		assert.True(t, uuid.Equal(result))
	})

	t.Run("generates new UUID when not set", func(t *testing.T) {
		doc := &Document{}
		result := doc.GetDocumentUUID()
		assert.False(t, result.IsZero())
	})

	t.Run("generates new UUID when zero", func(t *testing.T) {
		zeroUUID := docid.UUID{}
		doc := &Document{
			DocumentUUID: &zeroUUID,
		}
		result := doc.GetDocumentUUID()
		assert.False(t, result.IsZero())
	})
}

func TestDocument_SetDocumentUUID(t *testing.T) {
	t.Run("sets UUID", func(t *testing.T) {
		uuid := docid.NewUUID()
		doc := &Document{}
		doc.SetDocumentUUID(uuid)
		require.NotNil(t, doc.DocumentUUID)
		assert.True(t, uuid.Equal(*doc.DocumentUUID))
	})
}

func TestDocument_HasUUID(t *testing.T) {
	t.Run("returns true when UUID is set", func(t *testing.T) {
		uuid := docid.NewUUID()
		doc := &Document{
			DocumentUUID: &uuid,
		}
		assert.True(t, doc.HasUUID())
	})

	t.Run("returns false when UUID is nil", func(t *testing.T) {
		doc := &Document{}
		assert.False(t, doc.HasUUID())
	})

	t.Run("returns false when UUID is zero", func(t *testing.T) {
		zeroUUID := docid.UUID{}
		doc := &Document{
			DocumentUUID: &zeroUUID,
		}
		assert.False(t, doc.HasUUID())
	})
}

func TestDocument_GetByUUID(t *testing.T) {
	dsn := os.Getenv("HERMES_TEST_POSTGRESQL_DSN")
	if dsn == "" {
		t.Skip("HERMES_TEST_POSTGRESQL_DSN environment variable isn't set")
	}

	db, tearDownTest := setupTest(t, dsn)
	defer tearDownTest(t)

	t.Run("retrieves document by UUID", func(t *testing.T) {
		// Create a document with UUID
		uuid := docid.NewUUID()
		original := &Document{
			GoogleFileID: "test-file-id-uuid-lookup",
			DocumentUUID: &uuid,
			Title:        "Test Document",
		}
		require.NoError(t, original.Create(db))

		// Retrieve by UUID
		retrieved := &Document{}
		err := retrieved.GetByUUID(db, uuid)
		require.NoError(t, err)
		assert.Equal(t, original.ID, retrieved.ID)
		assert.Equal(t, original.GoogleFileID, retrieved.GoogleFileID)
		assert.Equal(t, original.Title, retrieved.Title)
		assert.True(t, uuid.Equal(*retrieved.DocumentUUID))
	})

	t.Run("returns error when UUID not found", func(t *testing.T) {
		nonExistentUUID := docid.NewUUID()
		doc := &Document{}
		err := doc.GetByUUID(db, nonExistentUUID)
		assert.Error(t, err)
	})
}

func TestDocument_GetByGoogleFileIDOrUUID(t *testing.T) {
	dsn := os.Getenv("HERMES_TEST_POSTGRESQL_DSN")
	if dsn == "" {
		t.Skip("HERMES_TEST_POSTGRESQL_DSN environment variable isn't set")
	}

	db, tearDownTest := setupTest(t, dsn)
	defer tearDownTest(t)

	t.Run("retrieves by UUID string", func(t *testing.T) {
		uuid := docid.NewUUID()
		original := &Document{
			GoogleFileID: "test-file-id-dual-lookup-1",
			DocumentUUID: &uuid,
			Title:        "Test Document UUID",
		}
		require.NoError(t, original.Create(db))

		retrieved := &Document{}
		err := retrieved.GetByGoogleFileIDOrUUID(db, uuid.String())
		require.NoError(t, err)
		assert.Equal(t, original.ID, retrieved.ID)
		assert.Equal(t, original.Title, retrieved.Title)
	})

	t.Run("retrieves by GoogleFileID when UUID lookup fails", func(t *testing.T) {
		original := &Document{
			GoogleFileID: "test-file-id-dual-lookup-2",
			Title:        "Test Document GoogleFileID",
		}
		require.NoError(t, original.Create(db))

		retrieved := &Document{}
		err := retrieved.GetByGoogleFileIDOrUUID(db, "test-file-id-dual-lookup-2")
		require.NoError(t, err)
		assert.Equal(t, original.ID, retrieved.ID)
		assert.Equal(t, original.Title, retrieved.Title)
	})

	t.Run("retrieves by GoogleFileID for non-UUID string", func(t *testing.T) {
		original := &Document{
			GoogleFileID: "not-a-uuid-format",
			Title:        "Test Document Non-UUID",
		}
		require.NoError(t, original.Create(db))

		retrieved := &Document{}
		err := retrieved.GetByGoogleFileIDOrUUID(db, "not-a-uuid-format")
		require.NoError(t, err)
		assert.Equal(t, original.ID, retrieved.ID)
		assert.Equal(t, original.Title, retrieved.Title)
	})

	t.Run("returns error when neither UUID nor GoogleFileID found", func(t *testing.T) {
		retrieved := &Document{}
		err := retrieved.GetByGoogleFileIDOrUUID(db, "nonexistent-id")
		assert.Error(t, err)
	})
}

func TestDocument_UUIDDatabaseIntegration(t *testing.T) {
	dsn := os.Getenv("HERMES_TEST_POSTGRESQL_DSN")
	if dsn == "" {
		t.Skip("HERMES_TEST_POSTGRESQL_DSN environment variable isn't set")
	}

	db, tearDownTest := setupTest(t, dsn)
	defer tearDownTest(t)

	t.Run("UUID persists to database", func(t *testing.T) {
		uuid := docid.NewUUID()
		doc := &Document{
			GoogleFileID: "test-file-id-persist",
			DocumentUUID: &uuid,
			Title:        "Test UUID Persistence",
		}
		require.NoError(t, doc.Create(db))

		// Retrieve and verify UUID persisted
		retrieved := &Document{}
		err := db.First(retrieved, doc.ID).Error
		require.NoError(t, err)
		require.NotNil(t, retrieved.DocumentUUID)
		assert.True(t, uuid.Equal(*retrieved.DocumentUUID))
	})

	t.Run("NULL UUID is handled correctly", func(t *testing.T) {
		doc := &Document{
			GoogleFileID: "test-file-id-null-uuid",
			Title:        "Test NULL UUID",
		}
		require.NoError(t, doc.Create(db))

		// Retrieve and verify NULL UUID
		retrieved := &Document{}
		err := db.First(retrieved, doc.ID).Error
		require.NoError(t, err)
		assert.Nil(t, retrieved.DocumentUUID)
		assert.False(t, retrieved.HasUUID())
	})

	t.Run("ProviderType and ProjectID persist correctly", func(t *testing.T) {
		uuid := docid.NewUUID()
		providerType := "google"
		projectID := "rfc-archive"
		doc := &Document{
			GoogleFileID: "test-file-id-provider-project",
			DocumentUUID: &uuid,
			ProviderType: &providerType,
			ProjectID:    &projectID,
			Title:        "Test Provider and Project",
		}
		require.NoError(t, doc.Create(db))

		// Retrieve and verify
		retrieved := &Document{}
		err := db.First(retrieved, doc.ID).Error
		require.NoError(t, err)
		require.NotNil(t, retrieved.ProviderType)
		require.NotNil(t, retrieved.ProjectID)
		assert.Equal(t, "google", *retrieved.ProviderType)
		assert.Equal(t, "rfc-archive", *retrieved.ProjectID)
	})
}
