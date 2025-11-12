package google

import (
	"context"
	"fmt"

	"github.com/hashicorp-forge/hermes/pkg/workspace"
	"google.golang.org/api/drive/v3"
)

// ===================================================================
// PermissionProvider Implementation
// ===================================================================

// ShareDocument grants access to a user/group.
func (a *Adapter) ShareDocument(ctx context.Context, providerID, email, role string) error {
	fileID, err := extractGoogleFileID(providerID)
	if err != nil {
		return err
	}

	return a.service.ShareFile(fileID, email, role)
}

// ShareDocumentWithDomain grants access to entire domain.
func (a *Adapter) ShareDocumentWithDomain(ctx context.Context, providerID, domain, role string) error {
	fileID, err := extractGoogleFileID(providerID)
	if err != nil {
		return err
	}

	return a.service.ShareFileWithDomain(fileID, domain, role)
}

// ListPermissions lists all permissions for a document.
func (a *Adapter) ListPermissions(ctx context.Context, providerID string) ([]*workspace.FilePermission, error) {
	fileID, err := extractGoogleFileID(providerID)
	if err != nil {
		return nil, err
	}

	perms, err := a.service.ListPermissions(fileID)
	if err != nil {
		return nil, fmt.Errorf("failed to list permissions: %w", err)
	}

	// Convert to RFC-084 FilePermission
	results := make([]*workspace.FilePermission, 0, len(perms))
	for _, perm := range perms {
		results = append(results, ConvertToFilePermission(perm))
	}

	return results, nil
}

// RemovePermission revokes access.
func (a *Adapter) RemovePermission(ctx context.Context, providerID, permissionID string) error {
	fileID, err := extractGoogleFileID(providerID)
	if err != nil {
		return err
	}

	return a.service.DeletePermission(fileID, permissionID)
}

// UpdatePermission changes permission role.
func (a *Adapter) UpdatePermission(ctx context.Context, providerID, permissionID, newRole string) error {
	fileID, err := extractGoogleFileID(providerID)
	if err != nil {
		return err
	}

	// Google Drive API requires updating permission
	perm := &drive.Permission{
		Role: newRole,
	}

	_, err = a.service.Drive.Permissions.Update(fileID, permissionID, perm).
		Context(ctx).
		Do()

	return err
}

// ===================================================================
// PeopleProvider Implementation
// ===================================================================

// SearchPeople searches for users in the directory.
func (a *Adapter) SearchPeople(ctx context.Context, query string) ([]*workspace.UserIdentity, error) {
	persons, err := a.service.SearchPeople(query, "emailAddresses,names,photos")
	if err != nil {
		return nil, fmt.Errorf("failed to search people: %w", err)
	}

	// Convert to RFC-084 UserIdentity
	results := make([]*workspace.UserIdentity, 0, len(persons))
	for _, person := range persons {
		// Extract email from person
		var email, displayName, photoURL string
		if len(person.EmailAddresses) > 0 {
			email = person.EmailAddresses[0].Value
		}
		if len(person.Names) > 0 {
			displayName = person.Names[0].DisplayName
		}
		if len(person.Photos) > 0 {
			photoURL = person.Photos[0].Url
		}

		if email != "" {
			results = append(results, &workspace.UserIdentity{
				Email:       email,
				DisplayName: displayName,
				PhotoURL:    photoURL,
			})
		}
	}

	return results, nil
}

// GetPerson retrieves a user by email.
func (a *Adapter) GetPerson(ctx context.Context, email string) (*workspace.UserIdentity, error) {
	persons, err := a.SearchPeople(ctx, email)
	if err != nil {
		return nil, err
	}

	if len(persons) == 0 {
		return nil, fmt.Errorf("user not found: %s", email)
	}

	return persons[0], nil
}

// GetPersonByUnifiedID retrieves user by unified ID (cross-provider lookup).
// Note: Google adapter does not have access to unified ID system.
// This would need to be implemented by a higher-level service.
func (a *Adapter) GetPersonByUnifiedID(ctx context.Context, unifiedID string) (*workspace.UserIdentity, error) {
	return nil, fmt.Errorf("GetPersonByUnifiedID not supported by Google adapter (requires identity service)")
}

// ResolveIdentity resolves alternate identities for a user.
// Note: Google adapter does not have access to cross-provider identity resolution.
// This would need to be implemented by a higher-level identity service.
func (a *Adapter) ResolveIdentity(ctx context.Context, email string) (*workspace.UserIdentity, error) {
	// For now, just return the Google identity
	return a.GetPerson(ctx, email)
}

// ===================================================================
// TeamProvider Implementation
// ===================================================================

// ListTeams lists teams matching query.
func (a *Adapter) ListTeams(ctx context.Context, domain, query string, maxResults int64) ([]*workspace.Team, error) {
	// Use the admin directory API to list groups
	groupsCall := a.service.AdminDirectory.Groups.List().
		Domain(domain).
		MaxResults(maxResults).
		Context(ctx)

	if query != "" {
		groupsCall = groupsCall.Query(query)
	}

	groups, err := groupsCall.Do()
	if err != nil {
		return nil, fmt.Errorf("failed to list teams: %w", err)
	}

	// Convert to RFC-084 Team
	results := make([]*workspace.Team, 0, len(groups.Groups))
	for _, group := range groups.Groups {
		team := &workspace.Team{
			ID:           group.Id,
			Email:        group.Email,
			Name:         group.Name,
			Description:  group.Description,
			MemberCount:  int(group.DirectMembersCount),
			ProviderType: "google",
			ProviderID:   fmt.Sprintf("google:%s", group.Id),
		}
		results = append(results, team)
	}

	return results, nil
}

// GetTeam retrieves team details.
func (a *Adapter) GetTeam(ctx context.Context, teamID string) (*workspace.Team, error) {
	group, err := a.service.AdminDirectory.Groups.Get(teamID).
		Context(ctx).
		Do()

	if err != nil {
		return nil, fmt.Errorf("failed to get team: %w", err)
	}

	return &workspace.Team{
		ID:           group.Id,
		Email:        group.Email,
		Name:         group.Name,
		Description:  group.Description,
		MemberCount:  int(group.DirectMembersCount),
		ProviderType: "google",
		ProviderID:   fmt.Sprintf("google:%s", group.Id),
	}, nil
}

// GetUserTeams lists all teams a user belongs to.
func (a *Adapter) GetUserTeams(ctx context.Context, userEmail string) ([]*workspace.Team, error) {
	// List groups for user
	groups, err := a.service.AdminDirectory.Groups.List().
		UserKey(userEmail).
		Context(ctx).
		Do()

	if err != nil {
		return nil, fmt.Errorf("failed to get user teams: %w", err)
	}

	// Convert to RFC-084 Team
	results := make([]*workspace.Team, 0, len(groups.Groups))
	for _, group := range groups.Groups {
		team := &workspace.Team{
			ID:           group.Id,
			Email:        group.Email,
			Name:         group.Name,
			Description:  group.Description,
			MemberCount:  int(group.DirectMembersCount),
			ProviderType: "google",
			ProviderID:   fmt.Sprintf("google:%s", group.Id),
		}
		results = append(results, team)
	}

	return results, nil
}

// GetTeamMembers lists all members of a team.
func (a *Adapter) GetTeamMembers(ctx context.Context, teamID string) ([]*workspace.UserIdentity, error) {
	// List members of group
	members, err := a.service.AdminDirectory.Members.List(teamID).
		Context(ctx).
		Do()

	if err != nil {
		return nil, fmt.Errorf("failed to get team members: %w", err)
	}

	// Convert to RFC-084 UserIdentity
	results := make([]*workspace.UserIdentity, 0, len(members.Members))
	for _, member := range members.Members {
		identity := &workspace.UserIdentity{
			Email: member.Email,
			// Note: Google admin API doesn't return display names in members list
			// Would need separate lookup for each member to get full details
		}
		results = append(results, identity)
	}

	return results, nil
}

// ===================================================================
// NotificationProvider Implementation
// ===================================================================

// SendEmail sends an email notification.
func (a *Adapter) SendEmail(ctx context.Context, to []string, from, subject, body string) error {
	// Use Gmail API to send email
	return a.service.SendEmail(to, from, subject, body)
}

// SendEmailWithTemplate sends email using template.
func (a *Adapter) SendEmailWithTemplate(ctx context.Context, to []string, template string, data map[string]any) error {
	// For now, just send plain email
	// Template rendering would be implemented by a higher-level service
	return a.service.SendEmail(to, "", template, fmt.Sprintf("%v", data))
}
