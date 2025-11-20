package google

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/docs/v1"
	"google.golang.org/api/drive/v3"

	"github.com/hashicorp-forge/hermes/pkg/docid"
)

func TestConvertToDocumentMetadata(t *testing.T) {
	uuid := docid.MustParseUUID("550e8400-e29b-41d4-a716-446655440000")
	now := time.Now().UTC()

	file := &drive.File{
		Id:           "1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs",
		Name:         "RFC-084: Provider Interface Refactoring",
		MimeType:     "application/vnd.google-apps.document",
		CreatedTime:  now.Format(time.RFC3339),
		ModifiedTime: now.Format(time.RFC3339),
		Version:      123,
		Owners: []*drive.User{
			{
				EmailAddress: "jacob.repp@hashicorp.com",
				DisplayName:  "Jacob Repp",
				PhotoLink:    "https://example.com/photo.jpg",
			},
		},
		Parents: []string{"folder-id-123"},
		Properties: map[string]string{
			"hermesUuid": uuid.String(),
			"project":    "platform-engineering",
		},
		WebViewLink:   "https://docs.google.com/document/d/1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs",
		ThumbnailLink: "https://example.com/thumbnail.png",
	}

	meta, err := ConvertToDocumentMetadata(file)
	require.NoError(t, err)

	// Verify UUID
	assert.Equal(t, uuid, meta.UUID)

	// Verify provider info
	assert.Equal(t, "google", meta.ProviderType)
	assert.Equal(t, "google:1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs", meta.ProviderID)

	// Verify core metadata
	assert.Equal(t, "RFC-084: Provider Interface Refactoring", meta.Name)
	assert.Equal(t, "application/vnd.google-apps.document", meta.MimeType)

	// Verify timestamps
	assert.Equal(t, now.Year(), meta.CreatedTime.Year())
	assert.Equal(t, now.Month(), meta.CreatedTime.Month())
	assert.Equal(t, now.Day(), meta.CreatedTime.Day())

	// Verify owner
	require.NotNil(t, meta.Owner)
	assert.Equal(t, "jacob.repp@hashicorp.com", meta.Owner.Email)
	assert.Equal(t, "Jacob Repp", meta.Owner.DisplayName)
	assert.Equal(t, "https://example.com/photo.jpg", meta.Owner.PhotoURL)

	// Verify parents
	assert.Equal(t, []string{"folder-id-123"}, meta.Parents)

	// Verify sync status
	assert.Equal(t, "canonical", meta.SyncStatus)

	// Verify extended metadata
	assert.Equal(t, "https://docs.google.com/document/d/1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs", meta.ExtendedMetadata["web_view_link"])
	assert.Equal(t, "https://example.com/thumbnail.png", meta.ExtendedMetadata["thumbnail_link"])
	assert.Equal(t, int64(123), meta.ExtendedMetadata["drive_version"])
	assert.Equal(t, "platform-engineering", meta.ExtendedMetadata["project"])
}

func TestConvertToDocumentMetadata_NoUUID(t *testing.T) {
	file := &drive.File{
		Id:         "file123",
		Name:       "Test Document",
		MimeType:   "text/markdown",
		Properties: map[string]string{},
	}

	meta, err := ConvertToDocumentMetadata(file)
	require.NoError(t, err)

	// UUID should be auto-generated
	assert.False(t, meta.UUID.IsZero())
}

func TestConvertToDocumentContent(t *testing.T) {
	uuid := docid.MustParseUUID("550e8400-e29b-41d4-a716-446655440000")
	now := time.Now().UTC()

	doc := &docs.Document{
		DocumentId: "1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs",
		Title:      "RFC-084: Provider Interface Refactoring",
		Body: &docs.Body{
			Content: []*docs.StructuralElement{
				{
					Paragraph: &docs.Paragraph{
						Elements: []*docs.ParagraphElement{
							{
								TextRun: &docs.TextRun{
									Content: "# RFC-084\n\n",
								},
							},
							{
								TextRun: &docs.TextRun{
									Content: "## Summary\n\nThis RFC proposes...\n",
								},
							},
						},
					},
				},
			},
		},
	}

	file := &drive.File{
		Id:           "1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs",
		Version:      123,
		ModifiedTime: now.Format(time.RFC3339),
		Properties: map[string]string{
			"hermesUuid": uuid.String(),
		},
		LastModifyingUser: &drive.User{
			EmailAddress: "jacob.repp@hashicorp.com",
			DisplayName:  "Jacob Repp",
		},
	}

	content, err := ConvertToDocumentContent(doc, file)
	require.NoError(t, err)

	// Verify UUID
	assert.Equal(t, uuid, content.UUID)

	// Verify provider ID
	assert.Equal(t, "google:1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs", content.ProviderID)

	// Verify content
	assert.Equal(t, "RFC-084: Provider Interface Refactoring", content.Title)
	assert.Equal(t, "richtext", content.Format)
	assert.Contains(t, content.Body, "# RFC-084")
	assert.Contains(t, content.Body, "## Summary")

	// Verify content hash
	assert.True(t, strings.HasPrefix(content.ContentHash, "sha256:"))

	// Verify backend revision
	require.NotNil(t, content.BackendRevision)
	assert.Equal(t, "google", content.BackendRevision.ProviderType)
	assert.Equal(t, "123", content.BackendRevision.RevisionID)
	require.NotNil(t, content.BackendRevision.ModifiedBy)
	assert.Equal(t, "jacob.repp@hashicorp.com", content.BackendRevision.ModifiedBy.Email)

	// Verify last modified
	assert.Equal(t, now.Year(), content.LastModified.Year())
	assert.Equal(t, now.Month(), content.LastModified.Month())
}

func TestConvertToBackendRevision(t *testing.T) {
	now := time.Now().UTC()

	file := &drive.File{
		Id:           "file123",
		Version:      42,
		ModifiedTime: now.Format(time.RFC3339),
		Size:         12345,
		Md5Checksum:  "abc123def456",
		LastModifyingUser: &drive.User{
			EmailAddress: "user@example.com",
			DisplayName:  "Test User",
			PhotoLink:    "https://example.com/photo.jpg",
		},
	}

	rev := ConvertToBackendRevision(file)
	require.NotNil(t, rev)

	// Verify provider type
	assert.Equal(t, "google", rev.ProviderType)

	// Verify revision ID (Google uses numeric version)
	assert.Equal(t, "42", rev.RevisionID)

	// Verify modified time
	assert.Equal(t, now.Year(), rev.ModifiedTime.Year())
	assert.Equal(t, now.Month(), rev.ModifiedTime.Month())

	// Verify modifier
	require.NotNil(t, rev.ModifiedBy)
	assert.Equal(t, "user@example.com", rev.ModifiedBy.Email)
	assert.Equal(t, "Test User", rev.ModifiedBy.DisplayName)

	// Verify metadata
	assert.Equal(t, int64(12345), rev.Metadata["size"])
	assert.Equal(t, "abc123def456", rev.Metadata["md5_checksum"])
}

func TestConvertDriveRevisionToBackendRevision(t *testing.T) {
	now := time.Now().UTC()

	driveRev := &drive.Revision{
		Id:           "123",
		ModifiedTime: now.Format(time.RFC3339),
		KeepForever:  true,
		Size:         54321,
		Md5Checksum:  "xyz789",
		MimeType:     "application/vnd.google-apps.document",
		LastModifyingUser: &drive.User{
			EmailAddress: "modifier@example.com",
			DisplayName:  "Modifier",
		},
	}

	rev := ConvertDriveRevisionToBackendRevision(driveRev)
	require.NotNil(t, rev)

	// Verify revision ID
	assert.Equal(t, "123", rev.RevisionID)

	// Verify keep forever flag
	assert.True(t, rev.KeepForever)

	// Verify metadata
	assert.Equal(t, int64(54321), rev.Metadata["size"])
	assert.Equal(t, "xyz789", rev.Metadata["md5_checksum"])
	assert.Equal(t, "application/vnd.google-apps.document", rev.Metadata["mime_type"])

	// Verify modifier
	require.NotNil(t, rev.ModifiedBy)
	assert.Equal(t, "modifier@example.com", rev.ModifiedBy.Email)
}

func TestConvertToUserIdentity(t *testing.T) {
	user := &drive.User{
		EmailAddress: "user@example.com",
		DisplayName:  "Test User",
		PhotoLink:    "https://example.com/photo.jpg",
	}

	identity := ConvertToUserIdentity(user)
	require.NotNil(t, identity)

	assert.Equal(t, "user@example.com", identity.Email)
	assert.Equal(t, "Test User", identity.DisplayName)
	assert.Equal(t, "https://example.com/photo.jpg", identity.PhotoURL)
}

func TestConvertToFilePermission(t *testing.T) {
	perm := &drive.Permission{
		Id:           "perm123",
		EmailAddress: "user@example.com",
		DisplayName:  "Test User",
		PhotoLink:    "https://example.com/photo.jpg",
		Role:         "writer",
		Type:         "user",
	}

	fp := ConvertToFilePermission(perm)
	require.NotNil(t, fp)

	assert.Equal(t, "perm123", fp.ID)
	assert.Equal(t, "user@example.com", fp.Email)
	assert.Equal(t, "writer", fp.Role)
	assert.Equal(t, "user", fp.Type)

	require.NotNil(t, fp.User)
	assert.Equal(t, "user@example.com", fp.User.Email)
	assert.Equal(t, "Test User", fp.User.DisplayName)
}

func TestExtractDocText(t *testing.T) {
	doc := &docs.Document{
		DocumentId: "doc123",
		Title:      "Test Document",
		Body: &docs.Body{
			Content: []*docs.StructuralElement{
				{
					Paragraph: &docs.Paragraph{
						Elements: []*docs.ParagraphElement{
							{
								TextRun: &docs.TextRun{
									Content: "First paragraph\n",
								},
							},
						},
					},
				},
				{
					Paragraph: &docs.Paragraph{
						Elements: []*docs.ParagraphElement{
							{
								TextRun: &docs.TextRun{
									Content: "Second paragraph\n",
								},
							},
						},
					},
				},
				{
					Table: &docs.Table{
						TableRows: []*docs.TableRow{
							{
								TableCells: []*docs.TableCell{
									{
										Content: []*docs.StructuralElement{
											{
												Paragraph: &docs.Paragraph{
													Elements: []*docs.ParagraphElement{
														{
															TextRun: &docs.TextRun{
																Content: "Cell content\n",
															},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	text := ExtractDocText(doc)

	assert.Contains(t, text, "First paragraph")
	assert.Contains(t, text, "Second paragraph")
	assert.Contains(t, text, "Cell content")
}

func TestExtractDocText_EmptyDoc(t *testing.T) {
	doc := &docs.Document{
		DocumentId: "doc123",
		Title:      "Empty Doc",
	}

	text := ExtractDocText(doc)
	assert.Empty(t, text)
}

func TestConvertToDocumentMetadata_NilFile(t *testing.T) {
	_, err := ConvertToDocumentMetadata(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "file cannot be nil")
}

func TestConvertToDocumentContent_NilDoc(t *testing.T) {
	_, err := ConvertToDocumentContent(nil, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "document cannot be nil")
}
