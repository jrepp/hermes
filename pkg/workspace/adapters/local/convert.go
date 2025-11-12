package local

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/hashicorp-forge/hermes/pkg/docid"
	"github.com/hashicorp-forge/hermes/pkg/workspace"
)

// ConvertToDocumentMetadata converts local Document to workspace.DocumentMetadata.
//
// Mapping:
//   - UUID: Extracted from CompositeID or metadata, or generated
//   - ProviderID: local:{document-id}
//   - Name: Document.Name
//   - Owner: Document.Owner
//   - Tags: Extracted from metadata
//   - CreatedTime/ModifiedTime: From Document
func ConvertToDocumentMetadata(doc *workspace.Document) (*workspace.DocumentMetadata, error) {
	if doc == nil {
		return nil, fmt.Errorf("document cannot be nil")
	}

	meta := &workspace.DocumentMetadata{
		ProviderType:     "local",
		ProviderID:       fmt.Sprintf("local:%s", doc.ID),
		Name:             doc.Name,
		MimeType:         doc.MimeType,
		CreatedTime:      doc.CreatedTime,
		ModifiedTime:     doc.ModifiedTime,
		ExtendedMetadata: make(map[string]any),
	}

	// Extract UUID from CompositeID if available
	if doc.CompositeID != nil && !doc.CompositeID.UUID().IsZero() {
		meta.UUID = doc.CompositeID.UUID()
	} else {
		// Try to extract from metadata
		if uuidStr, ok := doc.Metadata["hermes_uuid"].(string); ok {
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

	// Extract owner
	if doc.Owner != "" {
		meta.Owner = &workspace.UserIdentity{
			Email:       doc.Owner,
			DisplayName: doc.Owner,
		}
	}

	// Parent folders
	if doc.ParentFolderID != "" {
		meta.Parents = []string{doc.ParentFolderID}
	}

	// Default sync status for local documents
	meta.SyncStatus = "canonical" // Local is source of truth

	// Extract tags and extended metadata
	if doc.Metadata != nil {
		// Try to extract tags
		if tags, ok := doc.Metadata["tags"].([]string); ok {
			meta.Tags = tags
		} else if tagsIface, ok := doc.Metadata["tags"].([]interface{}); ok {
			// Handle case where tags come as []interface{}
			tags := make([]string, 0, len(tagsIface))
			for _, t := range tagsIface {
				if tagStr, ok := t.(string); ok {
					tags = append(tags, tagStr)
				}
			}
			meta.Tags = tags
		}

		// Extract project
		if project, ok := doc.Metadata["project"].(string); ok {
			meta.Project = project
		}

		// Extract owning team
		if team, ok := doc.Metadata["owning_team"].(string); ok {
			meta.OwningTeam = team
		}

		// Extract workflow status
		if status, ok := doc.Metadata["workflow_status"].(string); ok {
			meta.WorkflowStatus = status
		}

		// Store all other metadata in ExtendedMetadata
		for key, value := range doc.Metadata {
			switch key {
			case "hermes_uuid", "tags", "project", "owning_team", "workflow_status":
				// Skip fields already extracted
				continue
			default:
				meta.ExtendedMetadata[key] = value
			}
		}
	}

	return meta, nil
}

// ConvertToDocumentContent converts local Document with content to workspace.DocumentContent.
//
// Mapping:
//   - Body: Document.Content
//   - Format: "markdown" (local documents are markdown)
//   - BackendRevision: Git commit info if available
func ConvertToDocumentContent(doc *workspace.Document) (*workspace.DocumentContent, error) {
	if doc == nil {
		return nil, fmt.Errorf("document cannot be nil")
	}

	content := &workspace.DocumentContent{
		ProviderID: fmt.Sprintf("local:%s", doc.ID),
		Title:      doc.Name,
		Body:       doc.Content,
		Format:     "markdown", // Local documents are markdown
	}

	// Extract UUID
	uuid := docid.NewUUID()
	if doc.CompositeID != nil && !doc.CompositeID.UUID().IsZero() {
		uuid = doc.CompositeID.UUID()
	} else if uuidStr, ok := doc.Metadata["hermes_uuid"].(string); ok {
		if parsed, err := docid.ParseUUID(uuidStr); err == nil {
			uuid = parsed
		}
	}
	content.UUID = uuid

	// Calculate content hash
	hash := sha256.Sum256([]byte(doc.Content))
	content.ContentHash = "sha256:" + hex.EncodeToString(hash[:])

	// Set last modified
	content.LastModified = doc.ModifiedTime

	// Create backend revision info
	if doc.Metadata != nil {
		content.BackendRevision = &workspace.BackendRevision{
			ProviderType: "local",
			ModifiedTime: doc.ModifiedTime,
			Metadata:     make(map[string]any),
		}

		// Extract Git commit info if available
		if commitSHA, ok := doc.Metadata["git_commit"].(string); ok {
			content.BackendRevision.RevisionID = commitSHA
		}

		// Extract revision metadata
		if revisionNum, ok := doc.Metadata["revision_number"].(int); ok {
			content.BackendRevision.Metadata["revision_number"] = revisionNum
		}
	}

	return content, nil
}

// ConvertToUserIdentity converts local User to workspace.UserIdentity.
func ConvertToUserIdentity(user *workspace.User) *workspace.UserIdentity {
	if user == nil {
		return nil
	}

	return &workspace.UserIdentity{
		Email:       user.Email,
		DisplayName: user.Name, // User has Name, not DisplayName
		PhotoURL:    user.PhotoURL,
	}
}

// ConvertToFilePermission converts local Permission to workspace.FilePermission.
func ConvertToFilePermission(perm workspace.Permission) *workspace.FilePermission {
	return &workspace.FilePermission{
		ID:    perm.Email, // Use email as ID since Permission doesn't have ID field
		Email: perm.Email,
		Role:  perm.Role,
		Type:  perm.Type,
		User: &workspace.UserIdentity{
			Email:       perm.Email,
			DisplayName: perm.Email, // Permission doesn't have DisplayName, use email
		},
	}
}

// ConvertToBackendRevision converts local Revision to workspace.BackendRevision.
func ConvertToBackendRevision(rev *workspace.Revision) *workspace.BackendRevision {
	if rev == nil {
		return nil
	}

	backendRev := &workspace.BackendRevision{
		ProviderType: "local",
		RevisionID:   rev.ID,
		ModifiedTime: rev.ModifiedTime,
		Metadata:     make(map[string]any),
	}

	// Store revision metadata
	if rev.ModifiedBy != "" {
		backendRev.ModifiedBy = &workspace.UserIdentity{
			Email:       rev.ModifiedBy,
			DisplayName: rev.ModifiedBy,
		}
	}
	if rev.Name != "" {
		backendRev.Metadata["name"] = rev.Name
	}
	if rev.Content != "" {
		backendRev.Comment = "Revision: " + rev.Name
	}

	return backendRev
}

// ConvertFromDocumentMetadata converts workspace.DocumentMetadata to local Document format.
// This is useful for creating/updating documents.
func ConvertFromDocumentMetadata(meta *workspace.DocumentMetadata) *workspace.Document {
	doc := &workspace.Document{
		Name:         meta.Name,
		MimeType:     meta.MimeType,
		CreatedTime:  meta.CreatedTime,
		ModifiedTime: meta.ModifiedTime,
		Metadata:     make(map[string]any),
	}

	// Extract ID from providerID first
	var localID string
	if meta.ProviderID != "" {
		const prefix = "local:"
		if len(meta.ProviderID) > len(prefix) {
			localID = meta.ProviderID[len(prefix):]
			doc.ID = localID
		}
	}

	// Set CompositeID if UUID is available
	if !meta.UUID.IsZero() {
		// Create ProviderID if we have a local ID
		var compositeID docid.CompositeID
		if localID != "" {
			providerID, err := docid.NewProviderID(docid.ProviderTypeLocal, localID)
			if err == nil {
				compositeID = docid.NewCompositeID(meta.UUID, providerID, "")
			} else {
				// Fallback to UUID-only if ProviderID creation fails
				compositeID = docid.NewCompositeIDFromUUID(meta.UUID)
			}
		} else {
			compositeID = docid.NewCompositeIDFromUUID(meta.UUID)
		}
		doc.CompositeID = &compositeID
		doc.Metadata["hermes_uuid"] = meta.UUID.String()
	}

	// Set owner
	if meta.Owner != nil {
		doc.Owner = meta.Owner.Email
	}

	// Set parent folder
	if len(meta.Parents) > 0 {
		doc.ParentFolderID = meta.Parents[0]
	}

	// Copy core attributes to metadata
	if len(meta.Tags) > 0 {
		doc.Metadata["tags"] = meta.Tags
	}
	if meta.Project != "" {
		doc.Metadata["project"] = meta.Project
	}
	if meta.OwningTeam != "" {
		doc.Metadata["owning_team"] = meta.OwningTeam
	}
	if meta.WorkflowStatus != "" {
		doc.Metadata["workflow_status"] = meta.WorkflowStatus
	}

	// Copy extended metadata
	for key, value := range meta.ExtendedMetadata {
		doc.Metadata[key] = value
	}

	return doc
}
