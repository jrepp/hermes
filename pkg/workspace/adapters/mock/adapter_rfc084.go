// Package mock provides RFC-084 compliant fake implementations for testing.
package mock

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/hashicorp-forge/hermes/pkg/docid"
	"github.com/hashicorp-forge/hermes/pkg/workspace"
)

// FakeAdapter is an RFC-084 compliant fake workspace provider for testing.
// It stores all data in memory and implements all 7 required interfaces.
// Suitable for docker compose and local testing scenarios.
type FakeAdapter struct {
	mu sync.RWMutex

	// Documents stores document metadata by providerID
	Documents map[string]*workspace.DocumentMetadata

	// DocumentsByUUID provides UUID-based lookup
	DocumentsByUUID map[docid.UUID]*workspace.DocumentMetadata

	// Contents stores document content by providerID
	Contents map[string]*workspace.DocumentContent

	// Revisions stores revision history by providerID
	Revisions map[string][]*workspace.BackendRevision

	// Permissions stores permissions by providerID
	Permissions map[string][]*workspace.FilePermission

	// People stores user identities by email
	People map[string]*workspace.UserIdentity

	// Teams stores teams by ID
	Teams map[string]*workspace.Team

	// UserTeams maps user emails to their team memberships
	UserTeams map[string][]string // email -> []teamID

	// TeamMembers maps team IDs to member emails
	TeamMembers map[string][]string // teamID -> []email

	// EmailsSent tracks sent emails for testing verification
	EmailsSent []EmailRecord

	// Folders stores subfolder mappings
	Folders map[string]map[string]string // parentID -> name -> folderID

	// nextID is used for generating unique IDs
	nextID int
}

// EmailRecord tracks emails sent through the fake adapter.
type EmailRecord struct {
	To       []string
	From     string
	Subject  string
	Body     string
	Template string
	Data     map[string]any
	SentAt   time.Time
}

// Compile-time interface checks - ensures FakeAdapter implements all RFC-084 interfaces
var (
	_ workspace.WorkspaceProvider        = (*FakeAdapter)(nil)
	_ workspace.DocumentProvider         = (*FakeAdapter)(nil)
	_ workspace.ContentProvider          = (*FakeAdapter)(nil)
	_ workspace.RevisionTrackingProvider = (*FakeAdapter)(nil)
	_ workspace.PermissionProvider       = (*FakeAdapter)(nil)
	_ workspace.PeopleProvider           = (*FakeAdapter)(nil)
	_ workspace.TeamProvider             = (*FakeAdapter)(nil)
	_ workspace.NotificationProvider     = (*FakeAdapter)(nil)
)

// NewFakeAdapter creates a new RFC-084 compliant fake adapter.
func NewFakeAdapter() *FakeAdapter {
	return &FakeAdapter{
		Documents:       make(map[string]*workspace.DocumentMetadata),
		DocumentsByUUID: make(map[docid.UUID]*workspace.DocumentMetadata),
		Contents:        make(map[string]*workspace.DocumentContent),
		Revisions:       make(map[string][]*workspace.BackendRevision),
		Permissions:     make(map[string][]*workspace.FilePermission),
		People:          make(map[string]*workspace.UserIdentity),
		Teams:           make(map[string]*workspace.Team),
		UserTeams:       make(map[string][]string),
		TeamMembers:     make(map[string][]string),
		EmailsSent:      make([]EmailRecord, 0),
		Folders:         make(map[string]map[string]string),
		nextID:          1,
	}
}

// generateID generates a unique ID for documents/folders
func (f *FakeAdapter) generateID() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	id := fmt.Sprintf("fake-%d", f.nextID)
	f.nextID++
	return id
}

// ===================================================================
// DocumentProvider Implementation
// ===================================================================

// GetDocument retrieves document metadata by backend-specific ID.
func (f *FakeAdapter) GetDocument(ctx context.Context, providerID string) (*workspace.DocumentMetadata, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	doc, ok := f.Documents[providerID]
	if !ok {
		return nil, fmt.Errorf("document not found: %s", providerID)
	}
	return doc, nil
}

// GetDocumentByUUID retrieves document metadata by UUID.
func (f *FakeAdapter) GetDocumentByUUID(ctx context.Context, uuid docid.UUID) (*workspace.DocumentMetadata, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	doc, ok := f.DocumentsByUUID[uuid]
	if !ok {
		return nil, fmt.Errorf("document with UUID %s not found", uuid.String())
	}
	return doc, nil
}

// CreateDocument creates a new document from template.
func (f *FakeAdapter) CreateDocument(ctx context.Context, templateID, destFolderID, name string) (*workspace.DocumentMetadata, error) {
	uuid := docid.NewUUID()
	return f.CreateDocumentWithUUID(ctx, uuid, templateID, destFolderID, name)
}

// CreateDocumentWithUUID creates document with explicit UUID (for migration).
func (f *FakeAdapter) CreateDocumentWithUUID(ctx context.Context, uuid docid.UUID, templateID, destFolderID, name string) (*workspace.DocumentMetadata, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Generate provider ID
	id := f.generateIDUnsafe()
	providerID := fmt.Sprintf("fake:%s", id)

	// Get template content if exists
	var content string
	if templateID != "" {
		if templateContent, ok := f.Contents[templateID]; ok {
			content = templateContent.Body
		}
	}

	// Create document metadata
	now := time.Now()
	doc := &workspace.DocumentMetadata{
		UUID:         uuid,
		ProviderType: "fake",
		ProviderID:   providerID,
		Name:         name,
		CreatedTime:  now,
		ModifiedTime: now,
		SyncStatus:   "canonical",
		ExtendedMetadata: map[string]any{
			"parent_folder": destFolderID,
		},
	}

	// Store document
	f.Documents[providerID] = doc
	f.DocumentsByUUID[uuid] = doc

	// Create initial revision
	revision := &workspace.BackendRevision{
		ProviderType: "fake",
		RevisionID:   "1",
		ModifiedTime: now,
		KeepForever:  false,
	}
	f.Revisions[providerID] = []*workspace.BackendRevision{revision}

	// Create initial content
	docContent := &workspace.DocumentContent{
		UUID:            uuid,
		ProviderID:      providerID,
		Body:            content,
		Format:          "markdown",
		BackendRevision: revision,
		ContentHash:     fmt.Sprintf("hash-%d", time.Now().UnixNano()),
		LastModified:    now,
	}
	f.Contents[providerID] = docContent

	return doc, nil
}

// generateIDUnsafe generates ID without locking (caller must hold lock)
func (f *FakeAdapter) generateIDUnsafe() string {
	id := fmt.Sprintf("fake-%d", f.nextID)
	f.nextID++
	return id
}

// RegisterDocument registers document metadata with provider.
func (f *FakeAdapter) RegisterDocument(ctx context.Context, doc *workspace.DocumentMetadata) (*workspace.DocumentMetadata, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Store document
	f.Documents[doc.ProviderID] = doc
	f.DocumentsByUUID[doc.UUID] = doc

	return doc, nil
}

// CopyDocument copies a document (preserves UUID if in frontmatter/metadata).
func (f *FakeAdapter) CopyDocument(ctx context.Context, srcProviderID, destFolderID, name string) (*workspace.DocumentMetadata, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Get source document
	srcDoc, ok := f.Documents[srcProviderID]
	if !ok {
		return nil, fmt.Errorf("source document not found: %s", srcProviderID)
	}

	// Generate new provider ID and UUID
	id := f.generateIDUnsafe()
	providerID := fmt.Sprintf("fake:%s", id)
	uuid := docid.NewUUID()

	// Copy document metadata
	now := time.Now()
	doc := &workspace.DocumentMetadata{
		UUID:         uuid,
		ProviderType: "fake",
		ProviderID:   providerID,
		Name:         name,
		CreatedTime:  now,
		ModifiedTime: now,
		Owner:        srcDoc.Owner,
		Tags:         append([]string{}, srcDoc.Tags...),
		SyncStatus:   "canonical",
		ExtendedMetadata: map[string]any{
			"parent_folder": destFolderID,
			"copied_from":   srcProviderID,
		},
	}

	// Store document
	f.Documents[providerID] = doc
	f.DocumentsByUUID[uuid] = doc

	// Copy content if exists
	if srcContent, ok := f.Contents[srcProviderID]; ok {
		// Create initial revision for the copy
		revision := &workspace.BackendRevision{
			ProviderType: "fake",
			RevisionID:   "1",
			ModifiedTime: now,
			KeepForever:  false,
		}
		f.Revisions[providerID] = []*workspace.BackendRevision{revision}

		docContent := &workspace.DocumentContent{
			UUID:            uuid,
			ProviderID:      providerID,
			Body:            srcContent.Body,
			Format:          srcContent.Format,
			BackendRevision: revision,
			ContentHash:     fmt.Sprintf("hash-%d", time.Now().UnixNano()),
			LastModified:    now,
		}
		f.Contents[providerID] = docContent
	}

	return doc, nil
}

// MoveDocument moves a document to different folder.
func (f *FakeAdapter) MoveDocument(ctx context.Context, providerID, destFolderID string) (*workspace.DocumentMetadata, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	doc, ok := f.Documents[providerID]
	if !ok {
		return nil, fmt.Errorf("document not found: %s", providerID)
	}

	// Update parent folder in extended metadata
	if doc.ExtendedMetadata == nil {
		doc.ExtendedMetadata = make(map[string]any)
	}
	doc.ExtendedMetadata["parent_folder"] = destFolderID
	doc.ModifiedTime = time.Now()

	return doc, nil
}

// DeleteDocument deletes a document.
func (f *FakeAdapter) DeleteDocument(ctx context.Context, providerID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	doc, ok := f.Documents[providerID]
	if !ok {
		return fmt.Errorf("document not found: %s", providerID)
	}

	// Remove from all storage maps
	delete(f.Documents, providerID)
	delete(f.DocumentsByUUID, doc.UUID)
	delete(f.Contents, providerID)
	delete(f.Revisions, providerID)
	delete(f.Permissions, providerID)

	return nil
}

// RenameDocument renames a document.
func (f *FakeAdapter) RenameDocument(ctx context.Context, providerID, newName string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	doc, ok := f.Documents[providerID]
	if !ok {
		return fmt.Errorf("document not found: %s", providerID)
	}

	doc.Name = newName
	doc.ModifiedTime = time.Now()

	return nil
}

// CreateFolder creates a folder/directory.
func (f *FakeAdapter) CreateFolder(ctx context.Context, name, parentID string) (*workspace.DocumentMetadata, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Generate folder ID
	id := f.generateIDUnsafe()
	providerID := fmt.Sprintf("fake:%s", id)
	uuid := docid.NewUUID()

	// Create folder metadata
	now := time.Now()
	folder := &workspace.DocumentMetadata{
		UUID:         uuid,
		ProviderType: "fake",
		ProviderID:   providerID,
		Name:         name,
		CreatedTime:  now,
		ModifiedTime: now,
		SyncStatus:   "canonical",
		ExtendedMetadata: map[string]any{
			"is_folder": true,
			"parent_id": parentID,
		},
	}

	// Store folder
	f.Documents[providerID] = folder
	f.DocumentsByUUID[uuid] = folder

	// Add to folders map
	if f.Folders[parentID] == nil {
		f.Folders[parentID] = make(map[string]string)
	}
	f.Folders[parentID][name] = providerID

	return folder, nil
}

// GetSubfolder finds a subfolder by name.
func (f *FakeAdapter) GetSubfolder(ctx context.Context, parentID, name string) (string, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	subfolders, ok := f.Folders[parentID]
	if !ok {
		return "", fmt.Errorf("parent folder not found: %s", parentID)
	}

	folderID, ok := subfolders[name]
	if !ok {
		return "", fmt.Errorf("subfolder %s not found in parent %s", name, parentID)
	}

	return folderID, nil
}

// ===================================================================
// ContentProvider Implementation
// ===================================================================

// GetContent retrieves document content with backend-specific revision.
func (f *FakeAdapter) GetContent(ctx context.Context, providerID string) (*workspace.DocumentContent, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	content, ok := f.Contents[providerID]
	if !ok {
		return nil, fmt.Errorf("content not found: %s", providerID)
	}
	return content, nil
}

// GetContentByUUID retrieves content using UUID.
func (f *FakeAdapter) GetContentByUUID(ctx context.Context, uuid docid.UUID) (*workspace.DocumentContent, error) {
	// First get document metadata to find providerID
	doc, err := f.GetDocumentByUUID(ctx, uuid)
	if err != nil {
		return nil, err
	}

	return f.GetContent(ctx, doc.ProviderID)
}

// UpdateContent updates document content.
func (f *FakeAdapter) UpdateContent(ctx context.Context, providerID string, content string) (*workspace.DocumentContent, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	doc, ok := f.Documents[providerID]
	if !ok {
		return nil, fmt.Errorf("document not found: %s", providerID)
	}

	// Get current revision number
	revisions := f.Revisions[providerID]
	nextRevNum := len(revisions) + 1
	nextRevID := fmt.Sprintf("%d", nextRevNum)

	// Create new revision
	now := time.Now()
	revision := &workspace.BackendRevision{
		ProviderType: "fake",
		RevisionID:   nextRevID,
		ModifiedTime: now,
		KeepForever:  false,
	}
	f.Revisions[providerID] = append(f.Revisions[providerID], revision)

	// Update content
	docContent := &workspace.DocumentContent{
		UUID:            doc.UUID,
		ProviderID:      providerID,
		Body:            content,
		Format:          "markdown",
		BackendRevision: revision,
		ContentHash:     fmt.Sprintf("hash-%d", time.Now().UnixNano()),
		LastModified:    now,
	}
	f.Contents[providerID] = docContent

	// Update document modified time
	doc.ModifiedTime = now

	return docContent, nil
}

// GetContentBatch retrieves multiple documents (efficient for migration).
func (f *FakeAdapter) GetContentBatch(ctx context.Context, providerIDs []string) ([]*workspace.DocumentContent, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	contents := make([]*workspace.DocumentContent, 0, len(providerIDs))
	for _, providerID := range providerIDs {
		if content, ok := f.Contents[providerID]; ok {
			contents = append(contents, content)
		}
	}

	return contents, nil
}

// CompareContent compares content between two revisions.
func (f *FakeAdapter) CompareContent(ctx context.Context, providerID1, providerID2 string) (*workspace.ContentComparison, error) {
	content1, err := f.GetContent(ctx, providerID1)
	if err != nil {
		return nil, fmt.Errorf("failed to get first content: %w", err)
	}

	content2, err := f.GetContent(ctx, providerID2)
	if err != nil {
		return nil, fmt.Errorf("failed to get second content: %w", err)
	}

	comparison := &workspace.ContentComparison{
		UUID:         content1.UUID,
		Revision1:    content1.BackendRevision,
		Revision2:    content2.BackendRevision,
		ContentMatch: content1.ContentHash == content2.ContentHash,
	}

	if comparison.ContentMatch {
		comparison.HashDifference = "same"
	} else {
		comparison.HashDifference = "major"
	}

	return comparison, nil
}

// ===================================================================
// RevisionTrackingProvider Implementation
// ===================================================================

// GetRevisionHistory lists all revisions for a document in this backend.
func (f *FakeAdapter) GetRevisionHistory(ctx context.Context, providerID string, limit int) ([]*workspace.BackendRevision, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	revisions, ok := f.Revisions[providerID]
	if !ok {
		return []*workspace.BackendRevision{}, nil
	}

	// Return most recent first
	result := make([]*workspace.BackendRevision, len(revisions))
	for i := range revisions {
		result[i] = revisions[len(revisions)-1-i]
	}

	// Apply limit
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}

	return result, nil
}

// GetRevision retrieves a specific revision.
func (f *FakeAdapter) GetRevision(ctx context.Context, providerID, revisionID string) (*workspace.BackendRevision, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	revisions, ok := f.Revisions[providerID]
	if !ok {
		return nil, fmt.Errorf("no revisions found for document: %s", providerID)
	}

	for _, rev := range revisions {
		if rev.RevisionID == revisionID {
			return rev, nil
		}
	}

	return nil, fmt.Errorf("revision %s not found for document %s", revisionID, providerID)
}

// GetRevisionContent retrieves content at a specific revision.
func (f *FakeAdapter) GetRevisionContent(ctx context.Context, providerID, revisionID string) (*workspace.DocumentContent, error) {
	// For fake adapter, just return current content
	// In a real implementation, would retrieve historical content
	return f.GetContent(ctx, providerID)
}

// KeepRevisionForever marks a revision as permanent (if supported).
func (f *FakeAdapter) KeepRevisionForever(ctx context.Context, providerID, revisionID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	revisions, ok := f.Revisions[providerID]
	if !ok {
		return fmt.Errorf("no revisions found for document: %s", providerID)
	}

	for _, rev := range revisions {
		if rev.RevisionID == revisionID {
			rev.KeepForever = true
			return nil
		}
	}

	return fmt.Errorf("revision %s not found for document %s", revisionID, providerID)
}

// GetAllDocumentRevisions returns all revisions across all backends for a UUID.
func (f *FakeAdapter) GetAllDocumentRevisions(ctx context.Context, uuid docid.UUID) ([]*workspace.RevisionInfo, error) {
	// Get document to find providerID
	doc, err := f.GetDocumentByUUID(ctx, uuid)
	if err != nil {
		return nil, err
	}

	// Get revision history
	backendRevisions, err := f.GetRevisionHistory(ctx, doc.ProviderID, 0)
	if err != nil {
		return nil, err
	}

	// Convert to RevisionInfo
	results := make([]*workspace.RevisionInfo, 0, len(backendRevisions))
	for _, backendRev := range backendRevisions {
		revInfo := &workspace.RevisionInfo{
			UUID:            uuid,
			ProviderType:    "fake",
			ProviderID:      doc.ProviderID,
			BackendRevision: backendRev,
			ContentHash:     doc.ContentHash,
			SyncStatus:      "canonical",
		}
		results = append(results, revInfo)
	}

	return results, nil
}

// ===================================================================
// PermissionProvider Implementation
// ===================================================================

// ShareDocument grants access to a user/group.
func (f *FakeAdapter) ShareDocument(ctx context.Context, providerID, email, role string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, ok := f.Documents[providerID]; !ok {
		return fmt.Errorf("document not found: %s", providerID)
	}

	if f.Permissions[providerID] == nil {
		f.Permissions[providerID] = make([]*workspace.FilePermission, 0)
	}

	// Check if permission already exists
	for _, perm := range f.Permissions[providerID] {
		if perm.Email == email {
			// Update existing permission
			perm.Role = role
			return nil
		}
	}

	// Add new permission
	perm := &workspace.FilePermission{
		ID:    fmt.Sprintf("perm-%d", time.Now().UnixNano()),
		Email: email,
		Role:  role,
		Type:  "user",
	}
	f.Permissions[providerID] = append(f.Permissions[providerID], perm)

	return nil
}

// ShareDocumentWithDomain grants access to entire domain.
func (f *FakeAdapter) ShareDocumentWithDomain(ctx context.Context, providerID, domain, role string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, ok := f.Documents[providerID]; !ok {
		return fmt.Errorf("document not found: %s", providerID)
	}

	if f.Permissions[providerID] == nil {
		f.Permissions[providerID] = make([]*workspace.FilePermission, 0)
	}

	// Check if domain permission already exists
	domainID := fmt.Sprintf("domain-%s", domain)
	for _, perm := range f.Permissions[providerID] {
		if perm.ID == domainID && perm.Type == "domain" {
			// Update existing permission
			perm.Role = role
			return nil
		}
	}

	// Add new domain permission
	// Note: FilePermission doesn't have Domain field, so we use ID to store domain info
	perm := &workspace.FilePermission{
		ID:    domainID,
		Email: "", // Empty for domain permissions
		Role:  role,
		Type:  "domain",
	}
	f.Permissions[providerID] = append(f.Permissions[providerID], perm)

	return nil
}

// ListPermissions lists all permissions for a document.
func (f *FakeAdapter) ListPermissions(ctx context.Context, providerID string) ([]*workspace.FilePermission, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if _, ok := f.Documents[providerID]; !ok {
		return nil, fmt.Errorf("document not found: %s", providerID)
	}

	perms := f.Permissions[providerID]
	if perms == nil {
		return []*workspace.FilePermission{}, nil
	}

	return perms, nil
}

// RemovePermission revokes access.
func (f *FakeAdapter) RemovePermission(ctx context.Context, providerID, permissionID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, ok := f.Documents[providerID]; !ok {
		return fmt.Errorf("document not found: %s", providerID)
	}

	perms := f.Permissions[providerID]
	for i, perm := range perms {
		if perm.ID == permissionID {
			// Remove permission
			f.Permissions[providerID] = append(perms[:i], perms[i+1:]...)
			return nil
		}
	}

	return fmt.Errorf("permission not found: %s", permissionID)
}

// UpdatePermission changes permission role.
func (f *FakeAdapter) UpdatePermission(ctx context.Context, providerID, permissionID, newRole string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, ok := f.Documents[providerID]; !ok {
		return fmt.Errorf("document not found: %s", providerID)
	}

	perms := f.Permissions[providerID]
	for _, perm := range perms {
		if perm.ID == permissionID {
			perm.Role = newRole
			return nil
		}
	}

	return fmt.Errorf("permission not found: %s", permissionID)
}

// ===================================================================
// PeopleProvider Implementation
// ===================================================================

// SearchPeople searches for users in the directory.
func (f *FakeAdapter) SearchPeople(ctx context.Context, query string) ([]*workspace.UserIdentity, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	results := make([]*workspace.UserIdentity, 0)
	for _, person := range f.People {
		// Simple matching - check email and display name
		if query == "" || person.Email == query || person.DisplayName == query {
			results = append(results, person)
		}
	}

	return results, nil
}

// GetPerson retrieves a user by email.
func (f *FakeAdapter) GetPerson(ctx context.Context, email string) (*workspace.UserIdentity, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	person, ok := f.People[email]
	if !ok {
		return nil, fmt.Errorf("person not found: %s", email)
	}

	return person, nil
}

// GetPersonByUnifiedID retrieves user by unified ID.
func (f *FakeAdapter) GetPersonByUnifiedID(ctx context.Context, unifiedID string) (*workspace.UserIdentity, error) {
	// For fake adapter, unified ID is the same as email
	return f.GetPerson(ctx, unifiedID)
}

// ResolveIdentity resolves alternate identities for a user.
func (f *FakeAdapter) ResolveIdentity(ctx context.Context, email string) (*workspace.UserIdentity, error) {
	// For fake adapter, no alternate identities
	return f.GetPerson(ctx, email)
}

// ===================================================================
// TeamProvider Implementation
// ===================================================================

// ListTeams lists teams matching query.
func (f *FakeAdapter) ListTeams(ctx context.Context, domain, query string, maxResults int64) ([]*workspace.Team, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	results := make([]*workspace.Team, 0)
	for _, team := range f.Teams {
		// Simple matching
		if query == "" || team.Name == query || team.Email == query {
			results = append(results, team)
			if maxResults > 0 && int64(len(results)) >= maxResults {
				break
			}
		}
	}

	return results, nil
}

// GetTeam retrieves team details.
func (f *FakeAdapter) GetTeam(ctx context.Context, teamID string) (*workspace.Team, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	team, ok := f.Teams[teamID]
	if !ok {
		return nil, fmt.Errorf("team not found: %s", teamID)
	}

	return team, nil
}

// GetUserTeams lists all teams a user belongs to.
func (f *FakeAdapter) GetUserTeams(ctx context.Context, userEmail string) ([]*workspace.Team, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	teamIDs, ok := f.UserTeams[userEmail]
	if !ok {
		return []*workspace.Team{}, nil
	}

	results := make([]*workspace.Team, 0, len(teamIDs))
	for _, teamID := range teamIDs {
		if team, ok := f.Teams[teamID]; ok {
			results = append(results, team)
		}
	}

	return results, nil
}

// GetTeamMembers lists all members of a team.
func (f *FakeAdapter) GetTeamMembers(ctx context.Context, teamID string) ([]*workspace.UserIdentity, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	emails, ok := f.TeamMembers[teamID]
	if !ok {
		return []*workspace.UserIdentity{}, nil
	}

	results := make([]*workspace.UserIdentity, 0, len(emails))
	for _, email := range emails {
		if person, ok := f.People[email]; ok {
			results = append(results, person)
		}
	}

	return results, nil
}

// ===================================================================
// NotificationProvider Implementation
// ===================================================================

// SendEmail sends an email notification.
func (f *FakeAdapter) SendEmail(ctx context.Context, to []string, from, subject, body string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	record := EmailRecord{
		To:      to,
		From:    from,
		Subject: subject,
		Body:    body,
		SentAt:  time.Now(),
	}
	f.EmailsSent = append(f.EmailsSent, record)

	return nil
}

// SendEmailWithTemplate sends email using template.
func (f *FakeAdapter) SendEmailWithTemplate(ctx context.Context, to []string, template string, data map[string]any) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	record := EmailRecord{
		To:       to,
		Template: template,
		Data:     data,
		SentAt:   time.Now(),
	}
	f.EmailsSent = append(f.EmailsSent, record)

	return nil
}

// ===================================================================
// Test Helper Methods
// ===================================================================

// WithDocument adds a document to the fake adapter for testing.
func (f *FakeAdapter) WithDocument(doc *workspace.DocumentMetadata) *FakeAdapter {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.Documents[doc.ProviderID] = doc
	f.DocumentsByUUID[doc.UUID] = doc
	return f
}

// WithContent adds content for a document.
func (f *FakeAdapter) WithContent(providerID string, content *workspace.DocumentContent) *FakeAdapter {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.Contents[providerID] = content
	return f
}

// WithPerson adds a person to the directory.
func (f *FakeAdapter) WithPerson(person *workspace.UserIdentity) *FakeAdapter {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.People[person.Email] = person
	return f
}

// WithTeam adds a team to the directory.
func (f *FakeAdapter) WithTeam(team *workspace.Team) *FakeAdapter {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.Teams[team.ID] = team
	return f
}

// WithTeamMember adds a user to a team.
func (f *FakeAdapter) WithTeamMember(teamID, userEmail string) *FakeAdapter {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Add to UserTeams map
	if f.UserTeams[userEmail] == nil {
		f.UserTeams[userEmail] = make([]string, 0)
	}
	f.UserTeams[userEmail] = append(f.UserTeams[userEmail], teamID)

	// Add to TeamMembers map
	if f.TeamMembers[teamID] == nil {
		f.TeamMembers[teamID] = make([]string, 0)
	}
	f.TeamMembers[teamID] = append(f.TeamMembers[teamID], userEmail)

	return f
}

// WithSubfolder adds a subfolder mapping.
func (f *FakeAdapter) WithSubfolder(parentID, name, folderID string) *FakeAdapter {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.Folders[parentID] == nil {
		f.Folders[parentID] = make(map[string]string)
	}
	f.Folders[parentID][name] = folderID

	return f
}
