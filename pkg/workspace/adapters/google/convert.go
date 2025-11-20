package google

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"google.golang.org/api/docs/v1"
	"google.golang.org/api/drive/v3"

	"github.com/hashicorp-forge/hermes/pkg/docid"
	"github.com/hashicorp-forge/hermes/pkg/workspace"
)

// ConvertToDocumentMetadata converts Google Drive file to RFC-084 DocumentMetadata.
//
// Mapping:
//   - UUID: Extracted from customProperties.hermesUuid (if present)
//   - ProviderID: google:{fileId}
//   - Name: file.Name
//   - Tags: Extracted from description or properties
//   - Owner: First owner from file.Owners
//   - CreatedTime/ModifiedTime: From Drive API
func ConvertToDocumentMetadata(file *drive.File) (*workspace.DocumentMetadata, error) {
	if file == nil {
		return nil, fmt.Errorf("file cannot be nil")
	}

	meta := &workspace.DocumentMetadata{
		ProviderType:     "google",
		ProviderID:       fmt.Sprintf("google:%s", file.Id),
		Name:             file.Name,
		MimeType:         file.MimeType,
		ExtendedMetadata: make(map[string]any),
	}

	// Extract UUID from custom properties
	if file.Properties != nil {
		if uuidStr, ok := file.Properties["hermesUuid"]; ok {
			uuid, err := docid.ParseUUID(uuidStr)
			if err == nil {
				meta.UUID = uuid
			}
		}
	}

	// Generate UUID if not present
	if meta.UUID.IsZero() {
		meta.UUID = docid.NewUUID()
	}

	// Parse timestamps
	if file.CreatedTime != "" {
		if t, err := time.Parse(time.RFC3339, file.CreatedTime); err == nil {
			meta.CreatedTime = t
		}
	}
	if file.ModifiedTime != "" {
		if t, err := time.Parse(time.RFC3339, file.ModifiedTime); err == nil {
			meta.ModifiedTime = t
		}
	}

	// Extract owner
	if len(file.Owners) > 0 {
		owner := file.Owners[0]
		meta.Owner = &workspace.UserIdentity{
			Email:       owner.EmailAddress,
			DisplayName: owner.DisplayName,
			PhotoURL:    owner.PhotoLink,
		}
	}

	// Parent folders
	if len(file.Parents) > 0 {
		meta.Parents = file.Parents
	}

	// Default sync status for Google documents
	meta.SyncStatus = "canonical" // Google is source of truth

	// Store Google-specific metadata in ExtendedMetadata
	if file.WebViewLink != "" {
		meta.ExtendedMetadata["web_view_link"] = file.WebViewLink
	}
	if file.ThumbnailLink != "" {
		meta.ExtendedMetadata["thumbnail_link"] = file.ThumbnailLink
	}
	if file.IconLink != "" {
		meta.ExtendedMetadata["icon_link"] = file.IconLink
	}
	if file.Version > 0 {
		meta.ExtendedMetadata["drive_version"] = file.Version
	}

	// Store all custom properties in ExtendedMetadata
	if file.Properties != nil {
		for key, value := range file.Properties {
			if key != "hermesUuid" { // Skip UUID as it's in core field
				meta.ExtendedMetadata[key] = value
			}
		}
	}

	return meta, nil
}

// ConvertToDocumentContent converts Google Doc content to RFC-084 DocumentContent.
//
// Mapping:
//   - Body: Extracted plain text from Google Doc
//   - Format: "richtext" (Google Docs native format)
//   - BackendRevision: Current revision info from Drive
func ConvertToDocumentContent(doc *docs.Document, file *drive.File) (*workspace.DocumentContent, error) {
	if doc == nil {
		return nil, fmt.Errorf("document cannot be nil")
	}

	content := &workspace.DocumentContent{
		ProviderID: fmt.Sprintf("google:%s", doc.DocumentId),
		Title:      doc.Title,
		Format:     "richtext", // Google Docs native format
	}

	// Extract UUID from document properties or generate
	uuid := docid.NewUUID()
	if file != nil && file.Properties != nil {
		if uuidStr, ok := file.Properties["hermesUuid"]; ok {
			if parsed, err := docid.ParseUUID(uuidStr); err == nil {
				uuid = parsed
			}
		}
	}
	content.UUID = uuid

	// Extract body text from Google Doc
	content.Body = ExtractDocText(doc)

	// Calculate content hash
	hash := sha256.Sum256([]byte(content.Body))
	content.ContentHash = "sha256:" + hex.EncodeToString(hash[:])

	// Extract backend revision info
	if file != nil {
		content.BackendRevision = ConvertToBackendRevision(file)
		if file.ModifiedTime != "" {
			if t, err := time.Parse(time.RFC3339, file.ModifiedTime); err == nil {
				content.LastModified = t
			}
		}
	}

	return content, nil
}

// ConvertToBackendRevision converts Google Drive file to RFC-084 BackendRevision.
//
// Google Drive Revision Format:
//   - RevisionID: Numeric string (e.g., "123", "456")
//   - Increments with each revision
//   - Can be marked keepForever
func ConvertToBackendRevision(file *drive.File) *workspace.BackendRevision {
	if file == nil {
		return nil
	}

	rev := &workspace.BackendRevision{
		ProviderType: "google",
		// Note: Google Drive's file.Version is the current revision number
		RevisionID: fmt.Sprintf("%d", file.Version),
		Metadata:   make(map[string]any),
	}

	// Parse modified time
	if file.ModifiedTime != "" {
		if t, err := time.Parse(time.RFC3339, file.ModifiedTime); err == nil {
			rev.ModifiedTime = t
		}
	}

	// Extract modifier
	if file.LastModifyingUser != nil {
		rev.ModifiedBy = &workspace.UserIdentity{
			Email:       file.LastModifyingUser.EmailAddress,
			DisplayName: file.LastModifyingUser.DisplayName,
			PhotoURL:    file.LastModifyingUser.PhotoLink,
		}
	}

	// Store Google-specific metadata
	if file.Size > 0 {
		rev.Metadata["size"] = file.Size
	}
	if file.Md5Checksum != "" {
		rev.Metadata["md5_checksum"] = file.Md5Checksum
	}

	return rev
}

// ConvertDriveRevisionToBackendRevision converts a Google Drive revision to RFC-084 BackendRevision.
func ConvertDriveRevisionToBackendRevision(driveRev *drive.Revision) *workspace.BackendRevision {
	if driveRev == nil {
		return nil
	}

	rev := &workspace.BackendRevision{
		ProviderType: "google",
		RevisionID:   driveRev.Id,
		KeepForever:  driveRev.KeepForever,
		Metadata:     make(map[string]any),
	}

	// Parse modified time
	if driveRev.ModifiedTime != "" {
		if t, err := time.Parse(time.RFC3339, driveRev.ModifiedTime); err == nil {
			rev.ModifiedTime = t
		}
	}

	// Extract modifier
	if driveRev.LastModifyingUser != nil {
		rev.ModifiedBy = &workspace.UserIdentity{
			Email:       driveRev.LastModifyingUser.EmailAddress,
			DisplayName: driveRev.LastModifyingUser.DisplayName,
			PhotoURL:    driveRev.LastModifyingUser.PhotoLink,
		}
	}

	// Store Google-specific metadata
	if driveRev.Size > 0 {
		rev.Metadata["size"] = driveRev.Size
	}
	if driveRev.Md5Checksum != "" {
		rev.Metadata["md5_checksum"] = driveRev.Md5Checksum
	}
	if driveRev.MimeType != "" {
		rev.Metadata["mime_type"] = driveRev.MimeType
	}
	if driveRev.OriginalFilename != "" {
		rev.Metadata["original_filename"] = driveRev.OriginalFilename
	}

	return rev
}

// ConvertToUserIdentity converts Google user to RFC-084 UserIdentity.
func ConvertToUserIdentity(user *drive.User) *workspace.UserIdentity {
	if user == nil {
		return nil
	}

	return &workspace.UserIdentity{
		Email:       user.EmailAddress,
		DisplayName: user.DisplayName,
		PhotoURL:    user.PhotoLink,
		// Note: UnifiedUserID would be populated by identity service
	}
}

// ConvertToFilePermission converts Google Drive permission to RFC-084 FilePermission.
func ConvertToFilePermission(perm *drive.Permission) *workspace.FilePermission {
	if perm == nil {
		return nil
	}

	fp := &workspace.FilePermission{
		ID:    perm.Id,
		Email: perm.EmailAddress,
		Role:  perm.Role,
		Type:  perm.Type,
	}

	// Convert user info if available
	if perm.EmailAddress != "" {
		fp.User = &workspace.UserIdentity{
			Email:       perm.EmailAddress,
			DisplayName: perm.DisplayName,
			PhotoURL:    perm.PhotoLink,
		}
	}

	return fp
}

// ExtractDocText extracts plain text from a Google Doc.
// Walks through the document structure and concatenates text content.
func ExtractDocText(doc *docs.Document) string {
	if doc == nil || doc.Body == nil {
		return ""
	}

	var text string
	for _, element := range doc.Body.Content {
		if element.Paragraph != nil {
			for _, elem := range element.Paragraph.Elements {
				if elem.TextRun != nil {
					text += elem.TextRun.Content
				}
			}
		}
		if element.Table != nil {
			for _, row := range element.Table.TableRows {
				for _, cell := range row.TableCells {
					for _, cellElement := range cell.Content {
						if cellElement.Paragraph != nil {
							for _, elem := range cellElement.Paragraph.Elements {
								if elem.TextRun != nil {
									text += elem.TextRun.Content
								}
							}
						}
					}
				}
			}
		}
	}

	return text
}

// UpdateFileWithUUID updates a Google Drive file with Hermes UUID in custom properties.
func UpdateFileWithUUID(service *drive.Service, fileID string, uuid docid.UUID) error {
	file := &drive.File{
		Properties: map[string]string{
			"hermesUuid": uuid.String(),
		},
	}

	_, err := service.Files.Update(fileID, file).
		Fields("id,properties").
		Do()

	return err
}
