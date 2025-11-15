package s3

import (
	"context"
	"fmt"

	"github.com/hashicorp-forge/hermes/pkg/workspace"
)

// Stub implementations for required interfaces that S3 doesn't natively support
// These should be delegated to another provider in a real deployment

// =========================================================================
// PermissionProvider stub implementation
// =========================================================================

func (a *Adapter) ShareDocument(ctx context.Context, providerID, email, role string) error {
	return fmt.Errorf("S3 adapter does not support permissions natively - delegate to API provider")
}

func (a *Adapter) ShareDocumentWithDomain(ctx context.Context, providerID, domain, role string) error {
	return fmt.Errorf("S3 adapter does not support permissions natively - delegate to API provider")
}

func (a *Adapter) ListPermissions(ctx context.Context, providerID string) ([]*workspace.FilePermission, error) {
	return nil, fmt.Errorf("S3 adapter does not support permissions natively - delegate to API provider")
}

func (a *Adapter) RemovePermission(ctx context.Context, providerID, permissionID string) error {
	return fmt.Errorf("S3 adapter does not support permissions natively - delegate to API provider")
}

func (a *Adapter) UpdatePermission(ctx context.Context, providerID, permissionID, newRole string) error {
	return fmt.Errorf("S3 adapter does not support permissions natively - delegate to API provider")
}

// =========================================================================
// PeopleProvider stub implementation
// =========================================================================

func (a *Adapter) SearchPeople(ctx context.Context, query string) ([]*workspace.UserIdentity, error) {
	return nil, fmt.Errorf("S3 adapter does not support people directory - delegate to API provider")
}

func (a *Adapter) GetPerson(ctx context.Context, email string) (*workspace.UserIdentity, error) {
	return nil, fmt.Errorf("S3 adapter does not support people directory - delegate to API provider")
}

func (a *Adapter) GetPersonByUnifiedID(ctx context.Context, unifiedID string) (*workspace.UserIdentity, error) {
	return nil, fmt.Errorf("S3 adapter does not support people directory - delegate to API provider")
}

func (a *Adapter) ResolveIdentity(ctx context.Context, email string) (*workspace.UserIdentity, error) {
	return nil, fmt.Errorf("S3 adapter does not support identity resolution - delegate to API provider")
}

// =========================================================================
// TeamProvider stub implementation
// =========================================================================

func (a *Adapter) ListTeams(ctx context.Context, domain, query string, maxResults int64) ([]*workspace.Team, error) {
	return nil, fmt.Errorf("S3 adapter does not support teams - delegate to API provider")
}

func (a *Adapter) GetTeam(ctx context.Context, teamID string) (*workspace.Team, error) {
	return nil, fmt.Errorf("S3 adapter does not support teams - delegate to API provider")
}

func (a *Adapter) GetUserTeams(ctx context.Context, userEmail string) ([]*workspace.Team, error) {
	return nil, fmt.Errorf("S3 adapter does not support teams - delegate to API provider")
}

func (a *Adapter) GetTeamMembers(ctx context.Context, teamID string) ([]*workspace.UserIdentity, error) {
	return nil, fmt.Errorf("S3 adapter does not support teams - delegate to API provider")
}

// =========================================================================
// NotificationProvider stub implementation
// =========================================================================

func (a *Adapter) SendEmail(ctx context.Context, to []string, from, subject, body string) error {
	return fmt.Errorf("S3 adapter does not support email sending - delegate to API provider or SMTP")
}

func (a *Adapter) SendEmailWithTemplate(ctx context.Context, to []string, template string, data map[string]any) error {
	return fmt.Errorf("S3 adapter does not support email sending - delegate to API provider or SMTP")
}
