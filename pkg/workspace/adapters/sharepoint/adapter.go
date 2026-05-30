// Package sharepoint adapts Microsoft SharePoint to the workspace provider interfaces.
package sharepoint

//revive:disable:exported Methods document interface behavior in pkg/workspace.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp-forge/hermes/pkg/docid"
	"github.com/hashicorp-forge/hermes/pkg/sharepointhelper"
	"github.com/hashicorp-forge/hermes/pkg/workspace"
)

const providerType = "sharepoint"

type providerParts struct {
	siteID  string
	driveID string
	itemID  string
}

// Adapter implements workspace.WorkspaceProvider for Microsoft SharePoint.
type Adapter struct {
	service *sharepointhelper.Service
}

var (
	_ workspace.WorkspaceProvider        = (*Adapter)(nil)
	_ workspace.DocumentProvider         = (*Adapter)(nil)
	_ workspace.ContentProvider          = (*Adapter)(nil)
	_ workspace.RevisionTrackingProvider = (*Adapter)(nil)
	_ workspace.PermissionProvider       = (*Adapter)(nil)
	_ workspace.PeopleProvider           = (*Adapter)(nil)
	_ workspace.TeamProvider             = (*Adapter)(nil)
	_ workspace.NotificationProvider     = (*Adapter)(nil)
)

// NewAdapter wraps an existing SharePoint service in the workspace provider interface.
func NewAdapter(service *sharepointhelper.Service) workspace.WorkspaceProvider {
	return &Adapter{service: service}
}

// GetService returns the underlying SharePoint service for compatibility code.
func (a *Adapter) GetService() *sharepointhelper.Service {
	return a.service
}

func scopedProviderID(siteID, driveID, itemID string) string {
	return fmt.Sprintf("%s:site:%s:drive:%s:item:%s", providerType, siteID, driveID, itemID)
}

func parseProviderID(id, configuredSiteID, configuredDriveID string) (providerParts, error) {
	if id == "" {
		return providerParts{}, workspace.InvalidInputError("providerID", "cannot be empty")
	}
	legacyPrefix := providerType + ":"
	if !strings.HasPrefix(id, legacyPrefix) {
		return providerParts{siteID: configuredSiteID, driveID: configuredDriveID, itemID: id}, nil
	}

	parts := strings.Split(id, ":")
	if len(parts) == 2 {
		return providerParts{siteID: configuredSiteID, driveID: configuredDriveID, itemID: parts[1]}, nil
	}
	if len(parts) != 7 || parts[1] != "site" || parts[3] != "drive" || parts[5] != "item" {
		return providerParts{}, workspace.InvalidInputError("providerID", "expected sharepoint:site:<site-id>:drive:<drive-id>:item:<item-id>")
	}
	parsed := providerParts{siteID: parts[2], driveID: parts[4], itemID: parts[6]}
	if parsed.siteID != configuredSiteID || parsed.driveID != configuredDriveID {
		return providerParts{}, fmt.Errorf("%w: provider ID targets site %q drive %q, configured provider is site %q drive %q",
			workspace.ErrInvalidInput, parsed.siteID, parsed.driveID, configuredSiteID, configuredDriveID)
	}
	return parsed, nil
}

func parseTime(value string) time.Time {
	if value == "" {
		return time.Time{}
	}
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t
	}
	return time.Time{}
}

func (a *Adapter) providerID(itemID string) string {
	return scopedProviderID(a.service.SiteID, a.service.DriveID, itemID)
}

func (a *Adapter) itemID(id string) (string, error) {
	parts, err := parseProviderID(id, a.service.SiteID, a.service.DriveID)
	if err != nil {
		return "", err
	}
	return parts.itemID, nil
}

func (a *Adapter) ensureHermesUUID(itemID string) (docid.UUID, error) {
	uuid, ok, err := a.service.GetHermesUUID(itemID)
	if err != nil {
		return docid.UUID{}, err
	}
	if ok {
		return uuid, nil
	}

	uuid = docid.NewUUID()
	if err := a.service.SetHermesUUID(itemID, uuid); err != nil {
		return docid.UUID{}, err
	}
	return uuid, nil
}

func (a *Adapter) metadataFromDocument(doc *sharepointhelper.Document, uuid docid.UUID) *workspace.DocumentMetadata {
	meta := &workspace.DocumentMetadata{
		ProviderType: providerType,
		ProviderID:   a.providerID(doc.ID),
		Name:         doc.Name,
		MimeType:     "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		ModifiedTime: parseTime(doc.LastModifiedTime),
		SyncStatus:   "canonical",
		UUID:         uuid,
		ExtendedMetadata: map[string]any{
			"site_id":        a.service.SiteID,
			"drive_id":       a.service.DriveID,
			"item_id":        doc.ID,
			"web_url":        doc.WebURL,
			"file_extension": doc.FileExtension,
			"size":           doc.Size,
		},
	}
	return meta
}

func (a *Adapter) metadataFromCopyResponse(doc *sharepointhelper.CopyFileResponse, uuid docid.UUID) *workspace.DocumentMetadata {
	meta := &workspace.DocumentMetadata{
		ProviderType: providerType,
		ProviderID:   a.providerID(doc.ID),
		Name:         doc.Name,
		ModifiedTime: parseTime(doc.LastModified),
		CreatedTime:  parseTime(doc.CreatedAt),
		SyncStatus:   "canonical",
		UUID:         uuid,
		ExtendedMetadata: map[string]any{
			"site_id":        a.service.SiteID,
			"drive_id":       a.service.DriveID,
			"item_id":        doc.ID,
			"web_url":        doc.WebURL,
			"file_extension": doc.FileExtension,
		},
	}
	return meta
}

func (a *Adapter) GetDocument(_ context.Context, id string) (*workspace.DocumentMetadata, error) {
	itemID, err := a.itemID(id)
	if err != nil {
		return nil, err
	}
	doc, err := a.service.GetFile(itemID)
	if err != nil {
		return nil, fmt.Errorf("failed to get SharePoint file: %w", err)
	}
	uuid, err := a.ensureHermesUUID(itemID)
	if err != nil {
		return nil, fmt.Errorf("failed to ensure SharePoint Hermes UUID: %w", err)
	}
	return a.metadataFromDocument(doc, uuid), nil
}

func (a *Adapter) GetDocumentByUUID(_ context.Context, uuid docid.UUID) (*workspace.DocumentMetadata, error) {
	doc, err := a.service.FindFileByHermesUUID(uuid)
	if err != nil {
		return nil, err
	}
	return a.metadataFromDocument(doc, uuid), nil
}

func (a *Adapter) CreateDocument(ctx context.Context, templateID, destFolderID, name string) (*workspace.DocumentMetadata, error) {
	return a.CreateDocumentWithUUID(ctx, docid.NewUUID(), templateID, destFolderID, name)
}

func (a *Adapter) CreateDocumentWithUUID(_ context.Context, uuid docid.UUID, templateID, destFolderID, name string) (*workspace.DocumentMetadata, error) {
	templateItemID, err := a.itemID(templateID)
	if err != nil {
		return nil, err
	}
	doc, err := a.service.CopyFile(templateItemID, name, destFolderID)
	if err != nil {
		return nil, fmt.Errorf("failed to create SharePoint document from template: %w", err)
	}
	if err := a.service.SetHermesUUID(doc.ID, uuid); err != nil {
		return nil, fmt.Errorf("failed to persist SharePoint Hermes UUID: %w", err)
	}
	return a.metadataFromCopyResponse(doc, uuid), nil
}

func (a *Adapter) RegisterDocument(ctx context.Context, doc *workspace.DocumentMetadata) (*workspace.DocumentMetadata, error) {
	itemID, err := a.itemID(doc.ProviderID)
	if err != nil {
		return nil, err
	}
	uuid := doc.UUID
	if uuid.IsZero() {
		uuid = docid.NewUUID()
	}
	if err := a.service.SetHermesUUID(itemID, uuid); err != nil {
		return nil, fmt.Errorf("failed to persist SharePoint Hermes UUID: %w", err)
	}
	return a.GetDocument(ctx, itemID)
}

func (a *Adapter) CopyDocument(_ context.Context, srcProviderID, destFolderID, name string) (*workspace.DocumentMetadata, error) {
	srcItemID, err := a.itemID(srcProviderID)
	if err != nil {
		return nil, err
	}
	doc, err := a.service.CopyFile(srcItemID, name, destFolderID)
	if err != nil {
		return nil, fmt.Errorf("failed to copy SharePoint document: %w", err)
	}
	uuid := docid.NewUUID()
	if err := a.service.SetHermesUUID(doc.ID, uuid); err != nil {
		return nil, fmt.Errorf("failed to persist SharePoint Hermes UUID: %w", err)
	}
	return a.metadataFromCopyResponse(doc, uuid), nil
}

func (a *Adapter) MoveDocument(_ context.Context, id, destFolderID string) (*workspace.DocumentMetadata, error) {
	itemID, err := a.itemID(id)
	if err != nil {
		return nil, err
	}
	doc, err := a.service.MoveFile(itemID, destFolderID)
	if err != nil {
		return nil, fmt.Errorf("failed to move SharePoint document: %w", err)
	}
	uuid, err := a.ensureHermesUUID(doc.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to ensure SharePoint Hermes UUID: %w", err)
	}
	return a.metadataFromCopyResponse(doc, uuid), nil
}

func (a *Adapter) DeleteDocument(_ context.Context, id string) error {
	itemID, err := a.itemID(id)
	if err != nil {
		return err
	}
	return a.service.DeleteFile(itemID)
}

func (a *Adapter) RenameDocument(_ context.Context, id, newName string) error {
	itemID, err := a.itemID(id)
	if err != nil {
		return err
	}
	return a.service.RenameFile(itemID, newName)
}

func (a *Adapter) CreateFolder(_ context.Context, name, parentID string) (*workspace.DocumentMetadata, error) {
	id, err := a.service.CreateFolder(name, parentID)
	if err != nil {
		return nil, fmt.Errorf("failed to create SharePoint folder: %w", err)
	}
	return &workspace.DocumentMetadata{
		ProviderType: providerType,
		ProviderID:   a.providerID(id),
		Name:         name,
		MimeType:     "folder",
		SyncStatus:   "canonical",
		UUID:         docid.NewUUID(),
	}, nil
}

func (a *Adapter) GetSubfolder(_ context.Context, parentID, name string) (string, error) {
	path := strings.TrimSuffix(parentID, "/") + "/" + strings.TrimPrefix(name, "/")
	return a.service.ResolveFolderPath(path)
}

func (a *Adapter) GetContent(_ context.Context, id string) (*workspace.DocumentContent, error) {
	fileID, err := a.itemID(id)
	if err != nil {
		return nil, err
	}
	body, err := a.service.DownloadContent(fileID)
	if err != nil {
		return nil, fmt.Errorf("failed to download SharePoint content: %w", err)
	}
	hash := sha256.Sum256([]byte(body))
	content := &workspace.DocumentContent{
		ProviderID:  a.providerID(fileID),
		Body:        body,
		Format:      "docx-text",
		ContentHash: "sha256:" + hex.EncodeToString(hash[:]),
	}
	if meta, err := a.GetDocument(context.Background(), fileID); err == nil {
		content.UUID = meta.UUID
		content.Title = meta.Name
		content.LastModified = meta.ModifiedTime
		content.BackendRevision = &workspace.BackendRevision{
			ProviderType: providerType,
			RevisionID:   meta.ModifiedTime.Format(time.RFC3339Nano),
			ModifiedTime: meta.ModifiedTime,
		}
	}
	return content, nil
}

func (a *Adapter) GetContentByUUID(ctx context.Context, uuid docid.UUID) (*workspace.DocumentContent, error) {
	meta, err := a.GetDocumentByUUID(ctx, uuid)
	if err != nil {
		return nil, err
	}
	return a.GetContent(ctx, meta.ProviderID)
}

func (a *Adapter) UpdateContent(context.Context, string, string) (*workspace.DocumentContent, error) {
	return nil, workspace.ErrNotImplemented
}

func (a *Adapter) GetContentBatch(ctx context.Context, providerIDs []string) ([]*workspace.DocumentContent, error) {
	contents := make([]*workspace.DocumentContent, 0, len(providerIDs))
	for _, id := range providerIDs {
		content, err := a.GetContent(ctx, id)
		if err != nil {
			return nil, err
		}
		contents = append(contents, content)
	}
	return contents, nil
}

func (a *Adapter) CompareContent(ctx context.Context, providerID1, providerID2 string) (*workspace.ContentComparison, error) {
	content1, err := a.GetContent(ctx, providerID1)
	if err != nil {
		return nil, err
	}
	content2, err := a.GetContent(ctx, providerID2)
	if err != nil {
		return nil, err
	}
	return &workspace.ContentComparison{
		Revision1:    content1.BackendRevision,
		Revision2:    content2.BackendRevision,
		ContentMatch: content1.ContentHash == content2.ContentHash,
	}, nil
}

func (a *Adapter) GetRevisionHistory(context.Context, string, int) ([]*workspace.BackendRevision, error) {
	return nil, workspace.ErrNotImplemented
}

func (a *Adapter) GetRevision(context.Context, string, string) (*workspace.BackendRevision, error) {
	return nil, workspace.ErrNotImplemented
}

func (a *Adapter) GetRevisionContent(context.Context, string, string) (*workspace.DocumentContent, error) {
	return nil, workspace.ErrNotImplemented
}

func (a *Adapter) KeepRevisionForever(context.Context, string, string) error {
	return workspace.ErrNotImplemented
}

func (a *Adapter) GetAllDocumentRevisions(context.Context, docid.UUID) ([]*workspace.RevisionInfo, error) {
	return nil, workspace.ErrNotImplemented
}

func (a *Adapter) ShareDocument(_ context.Context, providerID, email, role string) error {
	itemID, err := a.itemID(providerID)
	if err != nil {
		return err
	}
	return a.service.ShareFile(itemID, email, role)
}

func (a *Adapter) ShareDocumentWithDomain(_ context.Context, providerID, _, role string) error {
	itemID, err := a.itemID(providerID)
	if err != nil {
		return err
	}
	_, err = a.service.ShareFileWithDomain(itemID, role)
	return err
}

func (a *Adapter) ListPermissions(_ context.Context, providerID string) ([]*workspace.FilePermission, error) {
	itemID, err := a.itemID(providerID)
	if err != nil {
		return nil, err
	}
	permissions, err := a.service.ListPermissions(itemID)
	if err != nil {
		return nil, err
	}
	result := make([]*workspace.FilePermission, 0, len(permissions))
	for _, permission := range permissions {
		role := ""
		if len(permission.Role) > 0 {
			role = permission.Role[0]
		}
		result = append(result, &workspace.FilePermission{
			ID:    permission.ID,
			Email: permission.GrantedTo.User.Email,
			Role:  role,
			Type:  "user",
		})
	}
	return result, nil
}

func (a *Adapter) RemovePermission(_ context.Context, providerID, permissionID string) error {
	itemID, err := a.itemID(providerID)
	if err != nil {
		return err
	}
	return a.service.DeletePermission(itemID, permissionID)
}

func (a *Adapter) UpdatePermission(context.Context, string, string, string) error {
	return workspace.ErrNotImplemented
}

func (a *Adapter) SearchPeople(_ context.Context, query string) ([]*workspace.UserIdentity, error) {
	people, err := a.service.SearchPeople(query, 10)
	if err != nil {
		return nil, err
	}
	result := make([]*workspace.UserIdentity, 0, len(people))
	for _, person := range people {
		identity := identityFromPeople(person)
		result = append(result, identity)
	}
	return result, nil
}

func (a *Adapter) GetPerson(_ context.Context, email string) (*workspace.UserIdentity, error) {
	person, err := a.service.GetPersonByEmail(email)
	if err != nil {
		return nil, err
	}
	emailAddr := person.Mail
	if emailAddr == "" {
		emailAddr = person.UserPrincipalName
	}
	return &workspace.UserIdentity{
		Email:       emailAddr,
		DisplayName: person.DisplayName,
	}, nil
}

func (a *Adapter) GetPersonByUnifiedID(context.Context, string) (*workspace.UserIdentity, error) {
	return nil, workspace.ErrNotImplemented
}

func (a *Adapter) ResolveIdentity(ctx context.Context, email string) (*workspace.UserIdentity, error) {
	return a.GetPerson(ctx, email)
}

func (a *Adapter) ListTeams(_ context.Context, domain, query string, maxResults int64) ([]*workspace.Team, error) {
	groups, err := a.service.SearchGroup(query, domain, int(maxResults))
	if err != nil {
		return nil, err
	}
	result := make([]*workspace.Team, 0, len(groups))
	for _, group := range groups {
		result = append(result, &workspace.Team{
			ID:           group.ID,
			Email:        group.Mail,
			Name:         group.DisplayName,
			ProviderType: providerType,
			ProviderID:   a.providerID(group.ID),
		})
	}
	return result, nil
}

func (a *Adapter) GetTeam(context.Context, string) (*workspace.Team, error) {
	return nil, workspace.ErrNotImplemented
}

func (a *Adapter) GetUserTeams(context.Context, string) ([]*workspace.Team, error) {
	return nil, workspace.ErrNotImplemented
}

func (a *Adapter) GetTeamMembers(_ context.Context, teamID string) ([]*workspace.UserIdentity, error) {
	emails, err := a.service.GetGroupMemberEmails(teamID)
	if err != nil {
		return nil, err
	}
	result := make([]*workspace.UserIdentity, 0, len(emails))
	for _, email := range emails {
		result = append(result, &workspace.UserIdentity{Email: email})
	}
	return result, nil
}

func (a *Adapter) SendEmail(_ context.Context, to []string, from, subject, body string) error {
	return a.service.SendEmail(to, from, subject, body)
}

func (a *Adapter) SendEmailWithTemplate(context.Context, []string, string, map[string]any) error {
	return workspace.ErrNotImplemented
}

func identityFromPeople(person sharepointhelper.People) *workspace.UserIdentity {
	identity := &workspace.UserIdentity{}
	if len(person.EmailAddresses) > 0 {
		identity.Email = person.EmailAddresses[0].Value
	}
	if len(person.Names) > 0 {
		identity.DisplayName = person.Names[0].DisplayName
	}
	if len(person.Photos) > 0 {
		identity.PhotoURL = person.Photos[0].URL
	}
	return identity
}
