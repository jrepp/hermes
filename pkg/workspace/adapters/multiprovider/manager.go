package multiprovider

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/hashicorp-forge/hermes/pkg/docid"
	"github.com/hashicorp-forge/hermes/pkg/workspace"
)

// Manager coordinates multiple workspace providers with intelligent routing.
//
// Architecture:
// - Primary provider: Local workspace for document authoring and content
// - Secondary provider: API provider for directory, permissions, notifications
//
// Routing Strategy:
// - Document operations → Primary (local authoring)
// - Content operations → Primary (local editing)
// - Revision operations → Primary (local Git)
// - Directory operations → Secondary (central directory)
// - Permission operations → Secondary (central access control)
// - Team operations → Secondary (central groups)
// - Notification operations → Secondary (central email)
//
// Sync Strategy:
// - Immediate: Sync metadata on every document operation
// - Batch: Buffer operations and sync periodically
// - Manual: Only sync when explicitly requested
type Manager struct {
	config   *Config
	strategy *RoutingStrategy

	// Sync management
	syncQueue chan *SyncOperation
	syncMutex sync.Mutex
	stopChan  chan struct{}
	wg        sync.WaitGroup
}

// SyncOperation represents a pending sync operation
type SyncOperation struct {
	Type         string // "register", "update", "delete"
	Document     *workspace.DocumentMetadata
	AttemptCount int
	LastError    error
}

// Compile-time interface checks - ensures Manager implements all RFC-084 interfaces
var (
	_ workspace.WorkspaceProvider        = (*Manager)(nil)
	_ workspace.DocumentProvider         = (*Manager)(nil)
	_ workspace.ContentProvider          = (*Manager)(nil)
	_ workspace.RevisionTrackingProvider = (*Manager)(nil)
	_ workspace.PermissionProvider       = (*Manager)(nil)
	_ workspace.PeopleProvider           = (*Manager)(nil)
	_ workspace.TeamProvider             = (*Manager)(nil)
	_ workspace.NotificationProvider     = (*Manager)(nil)
)

// NewManager creates a new multi-provider manager
func NewManager(cfg *Config) (*Manager, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	// Use default routing strategy if not provided
	strategy := DefaultRoutingStrategy()

	// If no secondary provider, use local-only strategy
	if cfg.Secondary == nil {
		strategy = LocalOnlyStrategy()
	}

	m := &Manager{
		config:    cfg,
		strategy:  strategy,
		syncQueue: make(chan *SyncOperation, 100),
		stopChan:  make(chan struct{}),
	}

	// Start sync worker if batch mode is enabled
	if cfg.Sync.Enabled && cfg.Sync.Mode == SyncModeBatch {
		m.wg.Add(1)
		go m.syncWorker()
	}

	return m, nil
}

// Close stops the manager and flushes pending sync operations
func (m *Manager) Close() error {
	close(m.stopChan)
	m.wg.Wait()
	return nil
}

// syncWorker processes sync operations in batch mode
func (m *Manager) syncWorker() {
	defer m.wg.Done()

	ticker := time.NewTicker(m.config.Sync.BatchInterval)
	defer ticker.Stop()

	var batch []*SyncOperation

	for {
		select {
		case <-m.stopChan:
			// Flush remaining operations
			if len(batch) > 0 {
				m.processBatch(batch)
			}
			return

		case op := <-m.syncQueue:
			batch = append(batch, op)

		case <-ticker.C:
			if len(batch) > 0 {
				m.processBatch(batch)
				batch = nil
			}
		}
	}
}

// processBatch processes a batch of sync operations
func (m *Manager) processBatch(batch []*SyncOperation) {
	for _, op := range batch {
		if err := m.executeSyncOperation(op); err != nil {
			log.Printf("[multiprovider] sync failed: %v", err)

			// Retry if attempts remaining
			if op.AttemptCount < m.config.Sync.RetryAttempts {
				op.AttemptCount++
				op.LastError = err

				// Re-queue with delay
				go func(operation *SyncOperation) {
					time.Sleep(m.config.Sync.RetryDelay)
					m.syncQueue <- operation
				}(op)
			} else {
				log.Printf("[multiprovider] sync failed after %d attempts: %v",
					op.AttemptCount, err)
			}
		}
	}
}

// executeSyncOperation executes a single sync operation
func (m *Manager) executeSyncOperation(op *SyncOperation) error {
	if m.config.Secondary == nil {
		return fmt.Errorf("no secondary provider configured for sync")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	docProvider, ok := m.config.Secondary.(workspace.DocumentProvider)
	if !ok {
		return fmt.Errorf("secondary provider does not implement DocumentProvider")
	}

	switch op.Type {
	case "register":
		_, err := docProvider.RegisterDocument(ctx, op.Document)
		return err

	case "update":
		// For updates, we'd call an update endpoint (to be implemented in Phase 2)
		// For now, just re-register
		_, err := docProvider.RegisterDocument(ctx, op.Document)
		return err

	case "delete":
		// For deletes, we'd call a delete endpoint (to be implemented in Phase 2)
		// For now, this is a no-op
		return nil

	default:
		return fmt.Errorf("unknown sync operation type: %s", op.Type)
	}
}

// queueSync adds a sync operation to the queue
func (m *Manager) queueSync(op *SyncOperation) {
	if !m.config.Sync.Enabled {
		return
	}

	switch m.config.Sync.Mode {
	case SyncModeImmediate:
		// Execute immediately in background
		go func() {
			if err := m.executeSyncOperation(op); err != nil {
				log.Printf("[multiprovider] immediate sync failed: %v", err)
			}
		}()

	case SyncModeBatch:
		// Add to queue for batch processing
		select {
		case m.syncQueue <- op:
		default:
			log.Printf("[multiprovider] sync queue full, dropping operation")
		}

	case SyncModeManual:
		// No automatic sync, user must call SyncDocument explicitly
		return
	}
}

// ===================================================================
// WorkspaceProvider Implementation
// ===================================================================

// Name returns the provider name
func (m *Manager) Name() string {
	return "multiprovider"
}

// ProviderType returns the provider type
func (m *Manager) ProviderType() string {
	return "multiprovider"
}

// ===================================================================
// DocumentProvider Implementation - Routes to PRIMARY
// ===================================================================

// GetDocument retrieves document metadata by backend-specific ID
func (m *Manager) GetDocument(ctx context.Context, providerID string) (*workspace.DocumentMetadata, error) {
	docProvider, ok := m.config.Primary.(workspace.DocumentProvider)
	if !ok {
		return nil, fmt.Errorf("primary provider does not implement DocumentProvider")
	}
	return docProvider.GetDocument(ctx, providerID)
}

// GetDocumentByUUID retrieves document metadata by UUID
func (m *Manager) GetDocumentByUUID(ctx context.Context, uuid docid.UUID) (*workspace.DocumentMetadata, error) {
	docProvider, ok := m.config.Primary.(workspace.DocumentProvider)
	if !ok {
		return nil, fmt.Errorf("primary provider does not implement DocumentProvider")
	}
	return docProvider.GetDocumentByUUID(ctx, uuid)
}

// CreateDocument creates a new document from template
func (m *Manager) CreateDocument(ctx context.Context, templateID, destFolderID, name string) (*workspace.DocumentMetadata, error) {
	docProvider, ok := m.config.Primary.(workspace.DocumentProvider)
	if !ok {
		return nil, fmt.Errorf("primary provider does not implement DocumentProvider")
	}

	doc, err := docProvider.CreateDocument(ctx, templateID, destFolderID, name)
	if err != nil {
		return nil, err
	}

	// Queue sync to central
	m.queueSync(&SyncOperation{
		Type:     "register",
		Document: doc,
	})

	return doc, nil
}

// CreateDocumentWithUUID creates document with explicit UUID (for migration)
func (m *Manager) CreateDocumentWithUUID(ctx context.Context, uuid docid.UUID, templateID, destFolderID, name string) (*workspace.DocumentMetadata, error) {
	docProvider, ok := m.config.Primary.(workspace.DocumentProvider)
	if !ok {
		return nil, fmt.Errorf("primary provider does not implement DocumentProvider")
	}

	doc, err := docProvider.CreateDocumentWithUUID(ctx, uuid, templateID, destFolderID, name)
	if err != nil {
		return nil, err
	}

	// Queue sync to central
	m.queueSync(&SyncOperation{
		Type:     "register",
		Document: doc,
	})

	return doc, nil
}

// RegisterDocument registers document metadata with provider
func (m *Manager) RegisterDocument(ctx context.Context, doc *workspace.DocumentMetadata) (*workspace.DocumentMetadata, error) {
	docProvider, ok := m.config.Primary.(workspace.DocumentProvider)
	if !ok {
		return nil, fmt.Errorf("primary provider does not implement DocumentProvider")
	}
	return docProvider.RegisterDocument(ctx, doc)
}

// CopyDocument copies a document
func (m *Manager) CopyDocument(ctx context.Context, srcProviderID, destFolderID, name string) (*workspace.DocumentMetadata, error) {
	docProvider, ok := m.config.Primary.(workspace.DocumentProvider)
	if !ok {
		return nil, fmt.Errorf("primary provider does not implement DocumentProvider")
	}

	doc, err := docProvider.CopyDocument(ctx, srcProviderID, destFolderID, name)
	if err != nil {
		return nil, err
	}

	// Queue sync to central
	m.queueSync(&SyncOperation{
		Type:     "register",
		Document: doc,
	})

	return doc, nil
}

// MoveDocument moves a document to different folder
func (m *Manager) MoveDocument(ctx context.Context, providerID, destFolderID string) (*workspace.DocumentMetadata, error) {
	docProvider, ok := m.config.Primary.(workspace.DocumentProvider)
	if !ok {
		return nil, fmt.Errorf("primary provider does not implement DocumentProvider")
	}

	doc, err := docProvider.MoveDocument(ctx, providerID, destFolderID)
	if err != nil {
		return nil, err
	}

	// Queue sync to central
	m.queueSync(&SyncOperation{
		Type:     "update",
		Document: doc,
	})

	return doc, nil
}

// DeleteDocument deletes a document
func (m *Manager) DeleteDocument(ctx context.Context, providerID string) error {
	docProvider, ok := m.config.Primary.(workspace.DocumentProvider)
	if !ok {
		return fmt.Errorf("primary provider does not implement DocumentProvider")
	}

	// Get document metadata before deletion for sync (best effort - ignore errors)
	doc, _ := docProvider.GetDocument(ctx, providerID)

	err := docProvider.DeleteDocument(ctx, providerID)
	if err != nil {
		return err
	}

	// Queue delete sync to central
	if doc != nil {
		m.queueSync(&SyncOperation{
			Type:     "delete",
			Document: doc,
		})
	}

	return nil
}

// RenameDocument renames a document
func (m *Manager) RenameDocument(ctx context.Context, providerID, newName string) error {
	docProvider, ok := m.config.Primary.(workspace.DocumentProvider)
	if !ok {
		return fmt.Errorf("primary provider does not implement DocumentProvider")
	}

	err := docProvider.RenameDocument(ctx, providerID, newName)
	if err != nil {
		return err
	}

	// Get updated metadata and queue sync (best effort - ignore errors)
	doc, _ := docProvider.GetDocument(ctx, providerID)
	if doc != nil {
		m.queueSync(&SyncOperation{
			Type:     "update",
			Document: doc,
		})
	}

	return nil
}

// CreateFolder creates a folder/directory
func (m *Manager) CreateFolder(ctx context.Context, name, parentID string) (*workspace.DocumentMetadata, error) {
	docProvider, ok := m.config.Primary.(workspace.DocumentProvider)
	if !ok {
		return nil, fmt.Errorf("primary provider does not implement DocumentProvider")
	}
	return docProvider.CreateFolder(ctx, name, parentID)
}

// GetSubfolder finds a subfolder by name
func (m *Manager) GetSubfolder(ctx context.Context, parentID, name string) (string, error) {
	docProvider, ok := m.config.Primary.(workspace.DocumentProvider)
	if !ok {
		return "", fmt.Errorf("primary provider does not implement DocumentProvider")
	}
	return docProvider.GetSubfolder(ctx, parentID, name)
}

// ===================================================================
// ContentProvider Implementation - Routes to PRIMARY
// ===================================================================

// GetContent retrieves document content
func (m *Manager) GetContent(ctx context.Context, providerID string) (*workspace.DocumentContent, error) {
	contentProvider, ok := m.config.Primary.(workspace.ContentProvider)
	if !ok {
		return nil, fmt.Errorf("primary provider does not implement ContentProvider")
	}
	return contentProvider.GetContent(ctx, providerID)
}

// GetContentByUUID retrieves document content by UUID
func (m *Manager) GetContentByUUID(ctx context.Context, uuid docid.UUID) (*workspace.DocumentContent, error) {
	contentProvider, ok := m.config.Primary.(workspace.ContentProvider)
	if !ok {
		return nil, fmt.Errorf("primary provider does not implement ContentProvider")
	}
	return contentProvider.GetContentByUUID(ctx, uuid)
}

// UpdateContent updates document content
func (m *Manager) UpdateContent(ctx context.Context, providerID string, content string) (*workspace.DocumentContent, error) {
	contentProvider, ok := m.config.Primary.(workspace.ContentProvider)
	if !ok {
		return nil, fmt.Errorf("primary provider does not implement ContentProvider")
	}

	updated, err := contentProvider.UpdateContent(ctx, providerID, content)
	if err != nil {
		return nil, err
	}

	// Get document metadata and queue sync (best effort - ignore errors)
	docProvider, ok := m.config.Primary.(workspace.DocumentProvider)
	if ok {
		doc, _ := docProvider.GetDocument(ctx, providerID)
		if doc != nil {
			m.queueSync(&SyncOperation{
				Type:     "update",
				Document: doc,
			})
		}
	}

	return updated, nil
}

// CompareContent compares content between two documents
func (m *Manager) CompareContent(ctx context.Context, providerID1, providerID2 string) (*workspace.ContentComparison, error) {
	contentProvider, ok := m.config.Primary.(workspace.ContentProvider)
	if !ok {
		return nil, fmt.Errorf("primary provider does not implement ContentProvider")
	}
	return contentProvider.CompareContent(ctx, providerID1, providerID2)
}

// GetContentBatch retrieves multiple documents efficiently
func (m *Manager) GetContentBatch(ctx context.Context, providerIDs []string) ([]*workspace.DocumentContent, error) {
	contentProvider, ok := m.config.Primary.(workspace.ContentProvider)
	if !ok {
		return nil, fmt.Errorf("primary provider does not implement ContentProvider")
	}
	return contentProvider.GetContentBatch(ctx, providerIDs)
}

// ===================================================================
// RevisionTrackingProvider Implementation - Routes to PRIMARY
// ===================================================================

// GetRevisionHistory retrieves revision history
func (m *Manager) GetRevisionHistory(ctx context.Context, providerID string, limit int) ([]*workspace.BackendRevision, error) {
	revisionProvider, ok := m.config.Primary.(workspace.RevisionTrackingProvider)
	if !ok {
		return nil, fmt.Errorf("primary provider does not implement RevisionTrackingProvider")
	}
	return revisionProvider.GetRevisionHistory(ctx, providerID, limit)
}

// GetRevision retrieves a specific revision
func (m *Manager) GetRevision(ctx context.Context, providerID, revisionID string) (*workspace.BackendRevision, error) {
	revisionProvider, ok := m.config.Primary.(workspace.RevisionTrackingProvider)
	if !ok {
		return nil, fmt.Errorf("primary provider does not implement RevisionTrackingProvider")
	}
	return revisionProvider.GetRevision(ctx, providerID, revisionID)
}

// GetRevisionContent retrieves content at specific revision
func (m *Manager) GetRevisionContent(ctx context.Context, providerID, revisionID string) (*workspace.DocumentContent, error) {
	revisionProvider, ok := m.config.Primary.(workspace.RevisionTrackingProvider)
	if !ok {
		return nil, fmt.Errorf("primary provider does not implement RevisionTrackingProvider")
	}
	return revisionProvider.GetRevisionContent(ctx, providerID, revisionID)
}

// KeepRevisionForever marks a revision as permanent
func (m *Manager) KeepRevisionForever(ctx context.Context, providerID, revisionID string) error {
	revisionProvider, ok := m.config.Primary.(workspace.RevisionTrackingProvider)
	if !ok {
		return fmt.Errorf("primary provider does not implement RevisionTrackingProvider")
	}
	return revisionProvider.KeepRevisionForever(ctx, providerID, revisionID)
}

// GetAllDocumentRevisions returns all revisions across all backends
func (m *Manager) GetAllDocumentRevisions(ctx context.Context, uuid docid.UUID) ([]*workspace.RevisionInfo, error) {
	revisionProvider, ok := m.config.Primary.(workspace.RevisionTrackingProvider)
	if !ok {
		return nil, fmt.Errorf("primary provider does not implement RevisionTrackingProvider")
	}
	return revisionProvider.GetAllDocumentRevisions(ctx, uuid)
}

// ===================================================================
// PermissionProvider Implementation - Routes to SECONDARY
// ===================================================================

// ShareDocument grants access to a user/group
func (m *Manager) ShareDocument(ctx context.Context, providerID, email, role string) error {
	if !m.strategy.UseSecondaryForPermissions || m.config.Secondary == nil {
		permProvider, ok := m.config.Primary.(workspace.PermissionProvider)
		if !ok {
			return fmt.Errorf("permission provider not available")
		}
		return permProvider.ShareDocument(ctx, providerID, email, role)
	}

	permProvider, ok := m.config.Secondary.(workspace.PermissionProvider)
	if !ok {
		if m.strategy.FallbackToPrimary {
			permProvider, ok := m.config.Primary.(workspace.PermissionProvider)
			if !ok {
				return fmt.Errorf("permission provider not available")
			}
			return permProvider.ShareDocument(ctx, providerID, email, role)
		}
		return fmt.Errorf("secondary provider does not implement PermissionProvider")
	}
	return permProvider.ShareDocument(ctx, providerID, email, role)
}

// ShareDocumentWithDomain grants access to entire domain
func (m *Manager) ShareDocumentWithDomain(ctx context.Context, providerID, domain, role string) error {
	if !m.strategy.UseSecondaryForPermissions || m.config.Secondary == nil {
		permProvider, ok := m.config.Primary.(workspace.PermissionProvider)
		if !ok {
			return fmt.Errorf("permission provider not available")
		}
		return permProvider.ShareDocumentWithDomain(ctx, providerID, domain, role)
	}

	permProvider, ok := m.config.Secondary.(workspace.PermissionProvider)
	if !ok {
		if m.strategy.FallbackToPrimary {
			permProvider, ok := m.config.Primary.(workspace.PermissionProvider)
			if !ok {
				return fmt.Errorf("permission provider not available")
			}
			return permProvider.ShareDocumentWithDomain(ctx, providerID, domain, role)
		}
		return fmt.Errorf("secondary provider does not implement PermissionProvider")
	}
	return permProvider.ShareDocumentWithDomain(ctx, providerID, domain, role)
}

// ListPermissions lists all permissions for a document
func (m *Manager) ListPermissions(ctx context.Context, providerID string) ([]*workspace.FilePermission, error) {
	if !m.strategy.UseSecondaryForPermissions || m.config.Secondary == nil {
		permProvider, ok := m.config.Primary.(workspace.PermissionProvider)
		if !ok {
			return nil, fmt.Errorf("permission provider not available")
		}
		return permProvider.ListPermissions(ctx, providerID)
	}

	permProvider, ok := m.config.Secondary.(workspace.PermissionProvider)
	if !ok {
		if m.strategy.FallbackToPrimary {
			permProvider, ok := m.config.Primary.(workspace.PermissionProvider)
			if !ok {
				return nil, fmt.Errorf("permission provider not available")
			}
			return permProvider.ListPermissions(ctx, providerID)
		}
		return nil, fmt.Errorf("secondary provider does not implement PermissionProvider")
	}
	return permProvider.ListPermissions(ctx, providerID)
}

// RemovePermission revokes access
func (m *Manager) RemovePermission(ctx context.Context, providerID, permissionID string) error {
	if !m.strategy.UseSecondaryForPermissions || m.config.Secondary == nil {
		permProvider, ok := m.config.Primary.(workspace.PermissionProvider)
		if !ok {
			return fmt.Errorf("permission provider not available")
		}
		return permProvider.RemovePermission(ctx, providerID, permissionID)
	}

	permProvider, ok := m.config.Secondary.(workspace.PermissionProvider)
	if !ok {
		if m.strategy.FallbackToPrimary {
			permProvider, ok := m.config.Primary.(workspace.PermissionProvider)
			if !ok {
				return fmt.Errorf("permission provider not available")
			}
			return permProvider.RemovePermission(ctx, providerID, permissionID)
		}
		return fmt.Errorf("secondary provider does not implement PermissionProvider")
	}
	return permProvider.RemovePermission(ctx, providerID, permissionID)
}

// UpdatePermission changes permission role
func (m *Manager) UpdatePermission(ctx context.Context, providerID, permissionID, newRole string) error {
	if !m.strategy.UseSecondaryForPermissions || m.config.Secondary == nil {
		permProvider, ok := m.config.Primary.(workspace.PermissionProvider)
		if !ok {
			return fmt.Errorf("permission provider not available")
		}
		return permProvider.UpdatePermission(ctx, providerID, permissionID, newRole)
	}

	permProvider, ok := m.config.Secondary.(workspace.PermissionProvider)
	if !ok {
		if m.strategy.FallbackToPrimary {
			permProvider, ok := m.config.Primary.(workspace.PermissionProvider)
			if !ok {
				return fmt.Errorf("permission provider not available")
			}
			return permProvider.UpdatePermission(ctx, providerID, permissionID, newRole)
		}
		return fmt.Errorf("secondary provider does not implement PermissionProvider")
	}
	return permProvider.UpdatePermission(ctx, providerID, permissionID, newRole)
}

// ===================================================================
// PeopleProvider Implementation - Routes to SECONDARY
// ===================================================================

// SearchPeople searches for people in directory
func (m *Manager) SearchPeople(ctx context.Context, query string) ([]*workspace.UserIdentity, error) {
	if !m.strategy.UseSecondaryForDirectory || m.config.Secondary == nil {
		peopleProvider, ok := m.config.Primary.(workspace.PeopleProvider)
		if !ok {
			return nil, fmt.Errorf("people provider not available")
		}
		return peopleProvider.SearchPeople(ctx, query)
	}

	peopleProvider, ok := m.config.Secondary.(workspace.PeopleProvider)
	if !ok {
		if m.strategy.FallbackToPrimary {
			peopleProvider, ok := m.config.Primary.(workspace.PeopleProvider)
			if !ok {
				return nil, fmt.Errorf("people provider not available")
			}
			return peopleProvider.SearchPeople(ctx, query)
		}
		return nil, fmt.Errorf("secondary provider does not implement PeopleProvider")
	}
	return peopleProvider.SearchPeople(ctx, query)
}

// GetPerson retrieves a user by email
func (m *Manager) GetPerson(ctx context.Context, email string) (*workspace.UserIdentity, error) {
	if !m.strategy.UseSecondaryForDirectory || m.config.Secondary == nil {
		peopleProvider, ok := m.config.Primary.(workspace.PeopleProvider)
		if !ok {
			return nil, fmt.Errorf("people provider not available")
		}
		return peopleProvider.GetPerson(ctx, email)
	}

	peopleProvider, ok := m.config.Secondary.(workspace.PeopleProvider)
	if !ok {
		if m.strategy.FallbackToPrimary {
			peopleProvider, ok := m.config.Primary.(workspace.PeopleProvider)
			if !ok {
				return nil, fmt.Errorf("people provider not available")
			}
			return peopleProvider.GetPerson(ctx, email)
		}
		return nil, fmt.Errorf("secondary provider does not implement PeopleProvider")
	}
	return peopleProvider.GetPerson(ctx, email)
}

// GetPersonByUnifiedID retrieves user by unified ID
func (m *Manager) GetPersonByUnifiedID(ctx context.Context, unifiedID string) (*workspace.UserIdentity, error) {
	if !m.strategy.UseSecondaryForDirectory || m.config.Secondary == nil {
		peopleProvider, ok := m.config.Primary.(workspace.PeopleProvider)
		if !ok {
			return nil, fmt.Errorf("people provider not available")
		}
		return peopleProvider.GetPersonByUnifiedID(ctx, unifiedID)
	}

	peopleProvider, ok := m.config.Secondary.(workspace.PeopleProvider)
	if !ok {
		if m.strategy.FallbackToPrimary {
			peopleProvider, ok := m.config.Primary.(workspace.PeopleProvider)
			if !ok {
				return nil, fmt.Errorf("people provider not available")
			}
			return peopleProvider.GetPersonByUnifiedID(ctx, unifiedID)
		}
		return nil, fmt.Errorf("secondary provider does not implement PeopleProvider")
	}
	return peopleProvider.GetPersonByUnifiedID(ctx, unifiedID)
}

// ResolveIdentity resolves alternate identities for a user
func (m *Manager) ResolveIdentity(ctx context.Context, email string) (*workspace.UserIdentity, error) {
	if !m.strategy.UseSecondaryForDirectory || m.config.Secondary == nil {
		peopleProvider, ok := m.config.Primary.(workspace.PeopleProvider)
		if !ok {
			return nil, fmt.Errorf("people provider not available")
		}
		return peopleProvider.ResolveIdentity(ctx, email)
	}

	peopleProvider, ok := m.config.Secondary.(workspace.PeopleProvider)
	if !ok {
		if m.strategy.FallbackToPrimary {
			peopleProvider, ok := m.config.Primary.(workspace.PeopleProvider)
			if !ok {
				return nil, fmt.Errorf("people provider not available")
			}
			return peopleProvider.ResolveIdentity(ctx, email)
		}
		return nil, fmt.Errorf("secondary provider does not implement PeopleProvider")
	}
	return peopleProvider.ResolveIdentity(ctx, email)
}

// ===================================================================
// TeamProvider Implementation - Routes to SECONDARY
// ===================================================================

// ListTeams lists teams matching query
func (m *Manager) ListTeams(ctx context.Context, domain, query string, maxResults int64) ([]*workspace.Team, error) {
	if !m.strategy.UseSecondaryForTeams || m.config.Secondary == nil {
		teamProvider, ok := m.config.Primary.(workspace.TeamProvider)
		if !ok {
			return nil, fmt.Errorf("team provider not available")
		}
		return teamProvider.ListTeams(ctx, domain, query, maxResults)
	}

	teamProvider, ok := m.config.Secondary.(workspace.TeamProvider)
	if !ok {
		if m.strategy.FallbackToPrimary {
			teamProvider, ok := m.config.Primary.(workspace.TeamProvider)
			if !ok {
				return nil, fmt.Errorf("team provider not available")
			}
			return teamProvider.ListTeams(ctx, domain, query, maxResults)
		}
		return nil, fmt.Errorf("secondary provider does not implement TeamProvider")
	}
	return teamProvider.ListTeams(ctx, domain, query, maxResults)
}

// GetTeam retrieves team details
func (m *Manager) GetTeam(ctx context.Context, teamID string) (*workspace.Team, error) {
	if !m.strategy.UseSecondaryForTeams || m.config.Secondary == nil {
		teamProvider, ok := m.config.Primary.(workspace.TeamProvider)
		if !ok {
			return nil, fmt.Errorf("team provider not available")
		}
		return teamProvider.GetTeam(ctx, teamID)
	}

	teamProvider, ok := m.config.Secondary.(workspace.TeamProvider)
	if !ok {
		if m.strategy.FallbackToPrimary {
			teamProvider, ok := m.config.Primary.(workspace.TeamProvider)
			if !ok {
				return nil, fmt.Errorf("team provider not available")
			}
			return teamProvider.GetTeam(ctx, teamID)
		}
		return nil, fmt.Errorf("secondary provider does not implement TeamProvider")
	}
	return teamProvider.GetTeam(ctx, teamID)
}

// GetUserTeams lists all teams a user belongs to
func (m *Manager) GetUserTeams(ctx context.Context, userEmail string) ([]*workspace.Team, error) {
	if !m.strategy.UseSecondaryForTeams || m.config.Secondary == nil {
		teamProvider, ok := m.config.Primary.(workspace.TeamProvider)
		if !ok {
			return nil, fmt.Errorf("team provider not available")
		}
		return teamProvider.GetUserTeams(ctx, userEmail)
	}

	teamProvider, ok := m.config.Secondary.(workspace.TeamProvider)
	if !ok {
		if m.strategy.FallbackToPrimary {
			teamProvider, ok := m.config.Primary.(workspace.TeamProvider)
			if !ok {
				return nil, fmt.Errorf("team provider not available")
			}
			return teamProvider.GetUserTeams(ctx, userEmail)
		}
		return nil, fmt.Errorf("secondary provider does not implement TeamProvider")
	}
	return teamProvider.GetUserTeams(ctx, userEmail)
}

// GetTeamMembers lists all members of a team
func (m *Manager) GetTeamMembers(ctx context.Context, teamID string) ([]*workspace.UserIdentity, error) {
	if !m.strategy.UseSecondaryForTeams || m.config.Secondary == nil {
		teamProvider, ok := m.config.Primary.(workspace.TeamProvider)
		if !ok {
			return nil, fmt.Errorf("team provider not available")
		}
		return teamProvider.GetTeamMembers(ctx, teamID)
	}

	teamProvider, ok := m.config.Secondary.(workspace.TeamProvider)
	if !ok {
		if m.strategy.FallbackToPrimary {
			teamProvider, ok := m.config.Primary.(workspace.TeamProvider)
			if !ok {
				return nil, fmt.Errorf("team provider not available")
			}
			return teamProvider.GetTeamMembers(ctx, teamID)
		}
		return nil, fmt.Errorf("secondary provider does not implement TeamProvider")
	}
	return teamProvider.GetTeamMembers(ctx, teamID)
}

// ===================================================================
// NotificationProvider Implementation - Routes to SECONDARY
// ===================================================================

// SendEmail sends an email notification
func (m *Manager) SendEmail(ctx context.Context, to []string, from, subject, body string) error {
	if !m.strategy.UseSecondaryForNotifications || m.config.Secondary == nil {
		notifProvider, ok := m.config.Primary.(workspace.NotificationProvider)
		if !ok {
			return fmt.Errorf("notification provider not available")
		}
		return notifProvider.SendEmail(ctx, to, from, subject, body)
	}

	notifProvider, ok := m.config.Secondary.(workspace.NotificationProvider)
	if !ok {
		if m.strategy.FallbackToPrimary {
			notifProvider, ok := m.config.Primary.(workspace.NotificationProvider)
			if !ok {
				return fmt.Errorf("notification provider not available")
			}
			return notifProvider.SendEmail(ctx, to, from, subject, body)
		}
		return fmt.Errorf("secondary provider does not implement NotificationProvider")
	}
	return notifProvider.SendEmail(ctx, to, from, subject, body)
}

// SendEmailWithTemplate sends email using template
func (m *Manager) SendEmailWithTemplate(ctx context.Context, to []string, template string, data map[string]any) error {
	if !m.strategy.UseSecondaryForNotifications || m.config.Secondary == nil {
		notifProvider, ok := m.config.Primary.(workspace.NotificationProvider)
		if !ok {
			return fmt.Errorf("notification provider not available")
		}
		return notifProvider.SendEmailWithTemplate(ctx, to, template, data)
	}

	notifProvider, ok := m.config.Secondary.(workspace.NotificationProvider)
	if !ok {
		if m.strategy.FallbackToPrimary {
			notifProvider, ok := m.config.Primary.(workspace.NotificationProvider)
			if !ok {
				return fmt.Errorf("notification provider not available")
			}
			return notifProvider.SendEmailWithTemplate(ctx, to, template, data)
		}
		return fmt.Errorf("secondary provider does not implement NotificationProvider")
	}
	return notifProvider.SendEmailWithTemplate(ctx, to, template, data)
}
