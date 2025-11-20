package router

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp-forge/hermes/pkg/docid"
	"github.com/hashicorp-forge/hermes/pkg/workspace"
)

// mockProvider is a simple mock provider for testing
type mockProvider struct {
	name       string
	documents  map[string]*workspace.DocumentMetadata
	shouldFail bool
}

func newMockProvider(name string) *mockProvider {
	return &mockProvider{
		name:      name,
		documents: make(map[string]*workspace.DocumentMetadata),
	}
}

func (m *mockProvider) GetDocumentByUUID(ctx context.Context, uuid docid.UUID) (*workspace.DocumentMetadata, error) {
	if m.shouldFail {
		return nil, fmt.Errorf("mock provider %s: intentional failure", m.name)
	}

	doc, exists := m.documents[uuid.String()]
	if !exists {
		return nil, fmt.Errorf("resource not found: document with id \"%s\"", uuid.String())
	}

	return doc, nil
}

func (m *mockProvider) AddDocument(uuid docid.UUID, doc *workspace.DocumentMetadata) {
	m.documents[uuid.String()] = doc
}

// Stub implementations for other required interfaces
func (m *mockProvider) GetDocument(ctx context.Context, providerID string) (*workspace.DocumentMetadata, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockProvider) CreateDocument(ctx context.Context, templateID, destFolderID, name string) (*workspace.DocumentMetadata, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockProvider) CreateDocumentWithUUID(ctx context.Context, uuid docid.UUID, templateID, destFolderID, name string) (*workspace.DocumentMetadata, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockProvider) RegisterDocument(ctx context.Context, doc *workspace.DocumentMetadata) (*workspace.DocumentMetadata, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockProvider) CopyDocument(ctx context.Context, srcProviderID, destFolderID, name string) (*workspace.DocumentMetadata, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockProvider) MoveDocument(ctx context.Context, providerID, destFolderID string) (*workspace.DocumentMetadata, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockProvider) RenameDocument(ctx context.Context, providerID, newName string) error {
	return fmt.Errorf("not implemented")
}

func (m *mockProvider) DeleteDocument(ctx context.Context, providerID string) error {
	return fmt.Errorf("not implemented")
}

func (m *mockProvider) CreateFolder(ctx context.Context, name, parentID string) (*workspace.DocumentMetadata, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockProvider) GetSubfolder(ctx context.Context, parentID, name string) (string, error) {
	return "", fmt.Errorf("not implemented")
}

func (m *mockProvider) GetContent(ctx context.Context, providerID string) (*workspace.DocumentContent, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockProvider) GetContentByUUID(ctx context.Context, uuid docid.UUID) (*workspace.DocumentContent, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockProvider) UpdateContent(ctx context.Context, providerID string, content string) (*workspace.DocumentContent, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockProvider) GetContentBatch(ctx context.Context, providerIDs []string) ([]*workspace.DocumentContent, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockProvider) CompareContent(ctx context.Context, providerID1, providerID2 string) (*workspace.ContentComparison, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockProvider) GetRevisionHistory(ctx context.Context, providerID string, limit int) ([]*workspace.BackendRevision, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockProvider) GetRevision(ctx context.Context, providerID, revisionID string) (*workspace.BackendRevision, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockProvider) GetRevisionContent(ctx context.Context, providerID, revisionID string) (*workspace.DocumentContent, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockProvider) KeepRevisionForever(ctx context.Context, providerID, revisionID string) error {
	return fmt.Errorf("not implemented")
}

func (m *mockProvider) GetAllDocumentRevisions(ctx context.Context, uuid docid.UUID) ([]*workspace.RevisionInfo, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockProvider) ShareDocument(ctx context.Context, providerID, email, role string) error {
	return fmt.Errorf("not implemented")
}

func (m *mockProvider) ShareDocumentWithDomain(ctx context.Context, providerID, domain, role string) error {
	return fmt.Errorf("not implemented")
}

func (m *mockProvider) ListPermissions(ctx context.Context, providerID string) ([]*workspace.FilePermission, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockProvider) RemovePermission(ctx context.Context, providerID, permissionID string) error {
	return fmt.Errorf("not implemented")
}

func (m *mockProvider) UpdatePermission(ctx context.Context, providerID, permissionID, newRole string) error {
	return fmt.Errorf("not implemented")
}

func (m *mockProvider) SearchPeople(ctx context.Context, query string) ([]*workspace.UserIdentity, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockProvider) GetPerson(ctx context.Context, email string) (*workspace.UserIdentity, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockProvider) GetPersonByUnifiedID(ctx context.Context, unifiedID string) (*workspace.UserIdentity, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockProvider) ResolveIdentity(ctx context.Context, email string) (*workspace.UserIdentity, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockProvider) ListTeams(ctx context.Context, domain, query string, maxResults int64) ([]*workspace.Team, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockProvider) GetTeam(ctx context.Context, teamID string) (*workspace.Team, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockProvider) GetUserTeams(ctx context.Context, userEmail string) ([]*workspace.Team, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockProvider) GetTeamMembers(ctx context.Context, teamID string) ([]*workspace.UserIdentity, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockProvider) SendEmail(ctx context.Context, to []string, from, subject, body string) error {
	return fmt.Errorf("not implemented")
}

func (m *mockProvider) SendEmailWithTemplate(ctx context.Context, to []string, template string, data map[string]any) error {
	return fmt.Errorf("not implemented")
}

// Unit tests (no database required)

func TestRouterBasics(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "router-test",
		Level: hclog.Error,
	})

	router := NewRouter(nil, logger, nil)

	t.Run("RegisterProvider", func(t *testing.T) {
		mockPrimary := newMockProvider("primary")
		config := &ProviderConfig{
			ID:           1,
			Name:         "primary",
			Type:         "mock",
			IsPrimary:    true,
			IsWritable:   true,
			Status:       "active",
			HealthStatus: "healthy",
		}

		err := router.RegisterProvider("primary", mockPrimary, config)
		require.NoError(t, err)

		// Try to register again - should fail
		err = router.RegisterProvider("primary", mockPrimary, config)
		assert.Error(t, err)
	})

	t.Run("GetProvider", func(t *testing.T) {
		provider, err := router.GetProvider("primary")
		require.NoError(t, err)
		assert.NotNil(t, provider)

		// Non-existent provider
		_, err = router.GetProvider("nonexistent")
		assert.Error(t, err)
	})

	t.Run("GetPrimaryProvider", func(t *testing.T) {
		provider, config, err := router.GetPrimaryProvider()
		require.NoError(t, err)
		assert.NotNil(t, provider)
		assert.Equal(t, "primary", config.Name)
		assert.True(t, config.IsPrimary)
	})

	t.Run("ListProviders", func(t *testing.T) {
		providers := router.ListProviders()
		assert.Len(t, providers, 1)
		assert.Contains(t, providers, "primary")
	})

	t.Run("UnregisterProvider", func(t *testing.T) {
		err := router.UnregisterProvider("primary")
		require.NoError(t, err)

		_, err = router.GetProvider("primary")
		assert.Error(t, err)
	})
}

func TestRouteReadStrategies(t *testing.T) {
	ctx := context.Background()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "router-test",
		Level: hclog.Error,
	})

	t.Run("PrimaryOnly", func(t *testing.T) {
		router := NewRouter(nil, logger, &RouterConfig{
			ReadStrategy: ReadStrategyPrimaryOnly,
		})

		// Setup primary provider
		mockPrimary := newMockProvider("primary")
		testUUID := docid.NewUUID()
		mockPrimary.AddDocument(testUUID, &workspace.DocumentMetadata{
			UUID: testUUID,
			Name: "Test Doc",
		})

		err := router.RegisterProvider("primary", mockPrimary, &ProviderConfig{
			ID:           1,
			Name:         "primary",
			Type:         "mock",
			IsPrimary:    true,
			IsWritable:   true,
			Status:       "active",
			HealthStatus: "healthy",
		})
		require.NoError(t, err)

		// Should read from primary
		doc, err := router.RouteRead(ctx, testUUID)
		require.NoError(t, err)
		assert.Equal(t, testUUID, doc.UUID)
		assert.Equal(t, "Test Doc", doc.Name)

		// Document not found
		_, err = router.RouteRead(ctx, docid.NewUUID())
		assert.Error(t, err)
	})

	t.Run("PrimaryWithFallback", func(t *testing.T) {
		router := NewRouter(nil, logger, &RouterConfig{
			ReadStrategy: ReadStrategyPrimaryThenFallback,
		})

		// Setup primary provider (will fail)
		mockPrimary := newMockProvider("primary")
		mockPrimary.shouldFail = true

		err := router.RegisterProvider("primary", mockPrimary, &ProviderConfig{
			ID:           1,
			Name:         "primary",
			Type:         "mock",
			IsPrimary:    true,
			IsWritable:   true,
			Status:       "active",
			HealthStatus: "healthy",
		})
		require.NoError(t, err)

		// Setup secondary provider (has the document)
		mockSecondary := newMockProvider("secondary")
		testUUID := docid.NewUUID()
		mockSecondary.AddDocument(testUUID, &workspace.DocumentMetadata{
			UUID: testUUID,
			Name: "Test Doc Fallback",
		})

		err = router.RegisterProvider("secondary", mockSecondary, &ProviderConfig{
			ID:           2,
			Name:         "secondary",
			Type:         "mock",
			IsPrimary:    false,
			IsWritable:   true,
			Status:       "active",
			HealthStatus: "healthy",
		})
		require.NoError(t, err)

		// Should fallback to secondary
		doc, err := router.RouteRead(ctx, testUUID)
		require.NoError(t, err)
		assert.Equal(t, testUUID, doc.UUID)
		assert.Equal(t, "Test Doc Fallback", doc.Name)
	})
}

func TestRouteWriteStrategies(t *testing.T) {
	ctx := context.Background()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "router-test",
		Level: hclog.Error,
	})

	t.Run("PrimaryOnly", func(t *testing.T) {
		router := NewRouter(nil, logger, &RouterConfig{
			WriteStrategy: WriteStrategyPrimaryOnly,
		})

		mockPrimary := newMockProvider("primary")
		err := router.RegisterProvider("primary", mockPrimary, &ProviderConfig{
			ID:           1,
			Name:         "primary",
			Type:         "mock",
			IsPrimary:    true,
			IsWritable:   true,
			Status:       "active",
			HealthStatus: "healthy",
		})
		require.NoError(t, err)

		testUUID := docid.NewUUID()
		writeCount := 0

		// Write operation
		err = router.RouteWrite(ctx, func(provider workspace.WorkspaceProvider) error {
			writeCount++
			mp := provider.(*mockProvider)
			mp.AddDocument(testUUID, &workspace.DocumentMetadata{
				UUID: testUUID,
				Name: "Written Doc",
			})
			return nil
		})

		require.NoError(t, err)
		assert.Equal(t, 1, writeCount, "Should write to primary only")
		assert.Len(t, mockPrimary.documents, 1)
	})

	t.Run("AllWritable", func(t *testing.T) {
		router := NewRouter(nil, logger, &RouterConfig{
			WriteStrategy: WriteStrategyAllWritable,
		})

		mockPrimary := newMockProvider("primary")
		mockSecondary := newMockProvider("secondary")

		err := router.RegisterProvider("primary", mockPrimary, &ProviderConfig{
			ID:           1,
			Name:         "primary",
			Type:         "mock",
			IsPrimary:    true,
			IsWritable:   true,
			Status:       "active",
			HealthStatus: "healthy",
		})
		require.NoError(t, err)

		err = router.RegisterProvider("secondary", mockSecondary, &ProviderConfig{
			ID:           2,
			Name:         "secondary",
			Type:         "mock",
			IsPrimary:    false,
			IsWritable:   true,
			Status:       "active",
			HealthStatus: "healthy",
		})
		require.NoError(t, err)

		testUUID := docid.NewUUID()

		// Write operation
		err = router.RouteWrite(ctx, func(provider workspace.WorkspaceProvider) error {
			mp := provider.(*mockProvider)
			mp.AddDocument(testUUID, &workspace.DocumentMetadata{
				UUID: testUUID,
				Name: "Written Doc",
			})
			return nil
		})

		require.NoError(t, err)
		assert.Len(t, mockPrimary.documents, 1, "Should write to primary")
		assert.Len(t, mockSecondary.documents, 1, "Should write to secondary")
	})
}

func TestGetProviderFiltering(t *testing.T) {
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "router-test",
		Level: hclog.Error,
	})

	router := NewRouter(nil, logger, nil)

	// Register multiple providers
	mockPrimary := newMockProvider("primary")
	mockSecondary := newMockProvider("secondary")
	mockReadonly := newMockProvider("readonly")
	mockDisabled := newMockProvider("disabled")

	router.RegisterProvider("primary", mockPrimary, &ProviderConfig{
		ID:           1,
		Name:         "primary",
		Type:         "mock",
		IsPrimary:    true,
		IsWritable:   true,
		Status:       "active",
		HealthStatus: "healthy",
	})

	router.RegisterProvider("secondary", mockSecondary, &ProviderConfig{
		ID:           2,
		Name:         "secondary",
		Type:         "mock",
		IsPrimary:    false,
		IsWritable:   true,
		Status:       "active",
		HealthStatus: "healthy",
	})

	router.RegisterProvider("readonly", mockReadonly, &ProviderConfig{
		ID:           3,
		Name:         "readonly",
		Type:         "mock",
		IsPrimary:    false,
		IsWritable:   false,
		Status:       "active",
		HealthStatus: "healthy",
	})

	router.RegisterProvider("disabled", mockDisabled, &ProviderConfig{
		ID:           4,
		Name:         "disabled",
		Type:         "mock",
		IsPrimary:    false,
		IsWritable:   true,
		Status:       "disabled",
		HealthStatus: "unhealthy",
	})

	t.Run("GetWritableProviders", func(t *testing.T) {
		writable := router.GetWritableProviders()
		assert.Len(t, writable, 2, "Should have 2 writable providers (primary, secondary)")
	})

	t.Run("GetHealthyProviders", func(t *testing.T) {
		healthy := router.GetHealthyProviders()
		assert.Len(t, healthy, 3, "Should have 3 healthy providers (primary, secondary, readonly)")
	})
}
