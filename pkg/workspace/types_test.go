package workspace

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/hashicorp-forge/hermes/pkg/docid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDocumentMetadata_JSON(t *testing.T) {
	uuid := docid.MustParseUUID("550e8400-e29b-41d4-a716-446655440000")
	now := time.Now().UTC()

	doc := &DocumentMetadata{
		UUID:         uuid,
		ProviderType: "local",
		ProviderID:   "local:docs/rfc-084.md",
		Name:         "RFC-084: Provider Interface Refactoring",
		MimeType:     "text/markdown",
		CreatedTime:  now,
		ModifiedTime: now,
		Owner: &UserIdentity{
			Email:         "jacob.repp@hashicorp.com",
			DisplayName:   "Jacob Repp",
			UnifiedUserID: "user-12345",
			AlternateEmails: []AlternateIdentity{
				{
					Email:          "jrepp@ibm.com",
					Provider:       "ibm-verify",
					ProviderUserID: "ibm-67890",
				},
			},
		},
		OwningTeam:     "Platform Team",
		Contributors:   []UserIdentity{},
		Parents:        []string{"docs", "rfc"},
		Project:        "platform-engineering",
		Tags:           []string{"rfc", "architecture", "providers"},
		SyncStatus:     "canonical",
		WorkflowStatus: "Draft",
		ContentHash:    "sha256:abc123def456",
		ExtendedMetadata: map[string]any{
			"id":               "rfc-084",
			"sidebar_position": 84,
			"document_type":    "rfc",
		},
	}

	// Test JSON marshaling
	data, err := json.Marshal(doc)
	require.NoError(t, err)
	assert.Contains(t, string(data), "550e8400-e29b-41d4-a716-446655440000")
	assert.Contains(t, string(data), "Platform Team")
	assert.Contains(t, string(data), "jrepp@ibm.com")

	// Test JSON unmarshaling
	var decoded DocumentMetadata
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, uuid, decoded.UUID)
	assert.Equal(t, "local", decoded.ProviderType)
	assert.Equal(t, "RFC-084: Provider Interface Refactoring", decoded.Name)
	assert.Equal(t, "Platform Team", decoded.OwningTeam)
	assert.Equal(t, []string{"rfc", "architecture", "providers"}, decoded.Tags)
	assert.Equal(t, "canonical", decoded.SyncStatus)
	assert.Equal(t, "Draft", decoded.WorkflowStatus)

	// Verify owner identity
	require.NotNil(t, decoded.Owner)
	assert.Equal(t, "jacob.repp@hashicorp.com", decoded.Owner.Email)
	assert.Equal(t, "user-12345", decoded.Owner.UnifiedUserID)
	require.Len(t, decoded.Owner.AlternateEmails, 1)
	assert.Equal(t, "jrepp@ibm.com", decoded.Owner.AlternateEmails[0].Email)

	// Verify extended metadata
	require.NotNil(t, decoded.ExtendedMetadata)
	assert.Equal(t, "rfc-084", decoded.ExtendedMetadata["id"])
	assert.Equal(t, float64(84), decoded.ExtendedMetadata["sidebar_position"]) // JSON numbers are float64
}

func TestDocumentContent_WithBackendRevision(t *testing.T) {
	uuid := docid.MustParseUUID("550e8400-e29b-41d4-a716-446655440000")
	now := time.Now().UTC()

	content := &DocumentContent{
		UUID:       uuid,
		ProviderID: "google:1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs",
		Title:      "RFC-084: Provider Interface Refactoring",
		Body:       "# RFC-084\n\n## Summary\n\nThis RFC proposes...",
		Format:     "markdown",
		BackendRevision: &BackendRevision{
			ProviderType: "google",
			RevisionID:   "123",
			ModifiedTime: now,
			ModifiedBy: &UserIdentity{
				Email:       "jacob.repp@hashicorp.com",
				DisplayName: "Jacob Repp",
			},
			Comment:     "Updated architecture diagram",
			KeepForever: true,
			Metadata: map[string]any{
				"published": true,
				"size":      12345,
			},
		},
		ContentHash:  "sha256:abc123def456",
		LastModified: now,
	}

	// Test JSON marshaling
	data, err := json.Marshal(content)
	require.NoError(t, err)
	assert.Contains(t, string(data), "backendRevision")
	assert.Contains(t, string(data), `"revisionID":"123"`)
	assert.Contains(t, string(data), "Updated architecture diagram")

	// Test JSON unmarshaling
	var decoded DocumentContent
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, uuid, decoded.UUID)
	assert.Equal(t, "RFC-084: Provider Interface Refactoring", decoded.Title)
	assert.Equal(t, "markdown", decoded.Format)

	// Verify backend revision
	require.NotNil(t, decoded.BackendRevision)
	assert.Equal(t, "google", decoded.BackendRevision.ProviderType)
	assert.Equal(t, "123", decoded.BackendRevision.RevisionID)
	assert.Equal(t, "Updated architecture diagram", decoded.BackendRevision.Comment)
	assert.True(t, decoded.BackendRevision.KeepForever)

	// Verify revision metadata
	require.NotNil(t, decoded.BackendRevision.Metadata)
	assert.Equal(t, true, decoded.BackendRevision.Metadata["published"])
	assert.Equal(t, float64(12345), decoded.BackendRevision.Metadata["size"])
}

func TestBackendRevision_GitFormat(t *testing.T) {
	now := time.Now().UTC()

	rev := &BackendRevision{
		ProviderType: "git",
		RevisionID:   "a1b2c3d4e5f67890abcdef1234567890abcdef12", // Full Git SHA
		ModifiedTime: now,
		ModifiedBy: &UserIdentity{
			Email:       "jacob.repp@hashicorp.com",
			DisplayName: "Jacob Repp",
		},
		Comment: "feat: add multi-provider support",
		Metadata: map[string]any{
			"tree":   "abc123",
			"parent": "def456",
			"author": "Jacob Repp <jacob.repp@hashicorp.com>",
		},
	}

	// Verify Git SHA format (40 hex characters)
	assert.Len(t, rev.RevisionID, 40)
	assert.Regexp(t, "^[0-9a-f]{40}$", rev.RevisionID)

	// Test JSON marshaling
	data, err := json.Marshal(rev)
	require.NoError(t, err)
	assert.Contains(t, string(data), "a1b2c3d4e5f67890abcdef1234567890abcdef12")
	assert.Contains(t, string(data), "feat: add multi-provider support")
}

func TestBackendRevision_Office365Format(t *testing.T) {
	now := time.Now().UTC()

	rev := &BackendRevision{
		ProviderType: "office365",
		RevisionID:   "2.0", // Semantic version
		ModifiedTime: now,
		ModifiedBy: &UserIdentity{
			Email:       "jacob.repp@hashicorp.com",
			DisplayName: "Jacob Repp",
		},
		Comment: "Major version update",
		Metadata: map[string]any{
			"versionLabel": "Major",
			"size":         12345,
			"comment":      "Updated content",
		},
	}

	data, err := json.Marshal(rev)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"revisionID":"2.0"`)
	assert.Contains(t, string(data), "versionLabel")
}

func TestUserIdentity_UnifiedIdentity(t *testing.T) {
	identity := &UserIdentity{
		Email:         "jacob.repp@hashicorp.com",
		DisplayName:   "Jacob Repp",
		PhotoURL:      "https://example.com/photo.jpg",
		UnifiedUserID: "user-12345",
		AlternateEmails: []AlternateIdentity{
			{
				Email:          "jrepp@ibm.com",
				Provider:       "ibm-verify",
				ProviderUserID: "ibm-67890",
			},
			{
				Email:          "jacob-repp",
				Provider:       "github",
				ProviderUserID: "87654321",
			},
		},
	}

	// Test JSON marshaling
	data, err := json.Marshal(identity)
	require.NoError(t, err)
	assert.Contains(t, string(data), "jacob.repp@hashicorp.com")
	assert.Contains(t, string(data), "jrepp@ibm.com")
	assert.Contains(t, string(data), "github")

	// Test JSON unmarshaling
	var decoded UserIdentity
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, "jacob.repp@hashicorp.com", decoded.Email)
	assert.Equal(t, "user-12345", decoded.UnifiedUserID)
	require.Len(t, decoded.AlternateEmails, 2)

	// Verify alternate identities
	assert.Equal(t, "jrepp@ibm.com", decoded.AlternateEmails[0].Email)
	assert.Equal(t, "ibm-verify", decoded.AlternateEmails[0].Provider)
	assert.Equal(t, "jacob-repp", decoded.AlternateEmails[1].Email)
	assert.Equal(t, "github", decoded.AlternateEmails[1].Provider)
}

func TestFilePermission(t *testing.T) {
	perm := &FilePermission{
		ID:    "perm-123",
		Email: "user@example.com",
		Role:  "writer",
		Type:  "user",
		User: &UserIdentity{
			Email:         "user@example.com",
			DisplayName:   "Test User",
			UnifiedUserID: "user-456",
		},
	}

	data, err := json.Marshal(perm)
	require.NoError(t, err)
	assert.Contains(t, string(data), "writer")
	assert.Contains(t, string(data), "user@example.com")

	var decoded FilePermission
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, "writer", decoded.Role)
	assert.Equal(t, "user", decoded.Type)
	require.NotNil(t, decoded.User)
	assert.Equal(t, "Test User", decoded.User.DisplayName)
}

func TestTeam(t *testing.T) {
	team := &Team{
		ID:           "team-123",
		Email:        "platform-team@company.com",
		Name:         "Platform Team",
		Description:  "Platform engineering team",
		MemberCount:  15,
		ProviderType: "google",
		ProviderID:   "google:group-456",
	}

	data, err := json.Marshal(team)
	require.NoError(t, err)
	assert.Contains(t, string(data), "Platform Team")
	assert.Contains(t, string(data), `"memberCount":15`)

	var decoded Team
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, "Platform Team", decoded.Name)
	assert.Equal(t, 15, decoded.MemberCount)
	assert.Equal(t, "google", decoded.ProviderType)
}

func TestRevisionInfo(t *testing.T) {
	uuid := docid.MustParseUUID("550e8400-e29b-41d4-a716-446655440000")
	now := time.Now().UTC()

	revInfo := &RevisionInfo{
		UUID:         uuid,
		ProviderType: "google",
		ProviderID:   "google:1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs",
		BackendRevision: &BackendRevision{
			ProviderType: "google",
			RevisionID:   "123",
			ModifiedTime: now,
		},
		ContentHash: "sha256:abc123",
		SyncStatus:  "canonical",
	}

	data, err := json.Marshal(revInfo)
	require.NoError(t, err)
	assert.Contains(t, string(data), "canonical")

	var decoded RevisionInfo
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, uuid, decoded.UUID)
	assert.Equal(t, "canonical", decoded.SyncStatus)
	require.NotNil(t, decoded.BackendRevision)
	assert.Equal(t, "123", decoded.BackendRevision.RevisionID)
}

func TestMergeRequest(t *testing.T) {
	sourceUUID := docid.MustParseUUID("550e8400-e29b-41d4-a716-446655440000")
	targetUUID := docid.MustParseUUID("7e8f4a2c-9d5b-4c1e-a8f7-3b2d1e6c9a4f")

	req := &MergeRequest{
		SourceUUID:     sourceUUID,
		TargetUUID:     targetUUID,
		MergeRevisions: true,
		MergeStrategy:  "merge-all",
		InitiatedBy:    "jacob.repp@hashicorp.com",
	}

	data, err := json.Marshal(req)
	require.NoError(t, err)
	assert.Contains(t, string(data), "550e8400")
	assert.Contains(t, string(data), "7e8f4a2c")
	assert.Contains(t, string(data), "merge-all")

	var decoded MergeRequest
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, sourceUUID, decoded.SourceUUID)
	assert.Equal(t, targetUUID, decoded.TargetUUID)
	assert.True(t, decoded.MergeRevisions)
	assert.Equal(t, "merge-all", decoded.MergeStrategy)
}

func TestSyncStatus(t *testing.T) {
	uuid := docid.MustParseUUID("550e8400-e29b-41d4-a716-446655440000")
	now := time.Now().UTC()

	status := &SyncStatus{
		UUID:         uuid,
		LastSyncTime: now,
		SyncState:    "synced",
	}

	data, err := json.Marshal(status)
	require.NoError(t, err)
	assert.Contains(t, string(data), "synced")

	var decoded SyncStatus
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, uuid, decoded.UUID)
	assert.Equal(t, "synced", decoded.SyncState)
	assert.Empty(t, decoded.ErrorMessage)
}

func TestOAuthFlow(t *testing.T) {
	flow := &OAuthFlow{
		AuthURL:  "https://github.com/login/oauth/authorize?client_id=abc",
		State:    "random-state-token",
		Provider: "github",
	}

	data, err := json.Marshal(flow)
	require.NoError(t, err)
	assert.Contains(t, string(data), "github.com")
	assert.Contains(t, string(data), "random-state-token")

	var decoded OAuthFlow
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, "github", decoded.Provider)
	assert.Contains(t, decoded.AuthURL, "github.com")
	assert.Equal(t, "random-state-token", decoded.State)
}

func TestJoinIdentityRequest(t *testing.T) {
	req := &JoinIdentityRequest{
		Provider: "github",
		Code:     "oauth-code-12345",
		State:    "random-state-token",
	}

	data, err := json.Marshal(req)
	require.NoError(t, err)
	assert.Contains(t, string(data), "github")
	assert.Contains(t, string(data), "oauth-code-12345")

	var decoded JoinIdentityRequest
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, "github", decoded.Provider)
	assert.Equal(t, "oauth-code-12345", decoded.Code)
	assert.Equal(t, "random-state-token", decoded.State)
}
