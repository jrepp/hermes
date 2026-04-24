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

// ShareDocument is a stub that returns an error for the S3 adapter.
func (a *Adapter) ShareDocument(_ context.Context, _, _, _ string) error {
	return fmt.Errorf("S3 adapter does not support permissions natively - delegate to API provider")
}

// ShareDocumentWithDomain is a stub that returns an error for the S3 adapter.
func (a *Adapter) ShareDocumentWithDomain(_ context.Context, _, _, _ string) error {
	return fmt.Errorf("S3 adapter does not support permissions natively - delegate to API provider")
}

// ListPermissions is a stub that returns an error for the S3 adapter.
func (a *Adapter) ListPermissions(_ context.Context, _ string) ([]*workspace.FilePermission, error) {
	return nil, fmt.Errorf("S3 adapter does not support permissions natively - delegate to API provider")
}

// RemovePermission is a stub that returns an error for the S3 adapter.
func (a *Adapter) RemovePermission(_ context.Context, _, _ string) error {
	return fmt.Errorf("S3 adapter does not support permissions natively - delegate to API provider")
}

// UpdatePermission is a stub that returns an error for the S3 adapter.
func (a *Adapter) UpdatePermission(_ context.Context, _, _, _ string) error {
	return fmt.Errorf("S3 adapter does not support permissions natively - delegate to API provider")
}

// =========================================================================
// PeopleProvider stub implementation
// =========================================================================

// SearchPeople is a stub that returns an error for the S3 adapter.
func (a *Adapter) SearchPeople(_ context.Context, _ string) ([]*workspace.UserIdentity, error) {
	return nil, fmt.Errorf("S3 adapter does not support people directory - delegate to API provider")
}

// GetPerson is a stub that returns an error for the S3 adapter.
func (a *Adapter) GetPerson(_ context.Context, _ string) (*workspace.UserIdentity, error) {
	return nil, fmt.Errorf("S3 adapter does not support people directory - delegate to API provider")
}

// GetPersonByUnifiedID is a stub that returns an error for the S3 adapter.
func (a *Adapter) GetPersonByUnifiedID(_ context.Context, _ string) (*workspace.UserIdentity, error) {
	return nil, fmt.Errorf("S3 adapter does not support people directory - delegate to API provider")
}

// ResolveIdentity is a stub that returns an error for the S3 adapter.
func (a *Adapter) ResolveIdentity(_ context.Context, _ string) (*workspace.UserIdentity, error) {
	return nil, fmt.Errorf("S3 adapter does not support identity resolution - delegate to API provider")
}

// =========================================================================
// TeamProvider stub implementation
// =========================================================================

// ListTeams is a stub that returns an error for the S3 adapter.
func (a *Adapter) ListTeams(_ context.Context, _, _ string, _ int64) ([]*workspace.Team, error) {
	return nil, fmt.Errorf("S3 adapter does not support teams - delegate to API provider")
}

// GetTeam is a stub that returns an error for the S3 adapter.
func (a *Adapter) GetTeam(_ context.Context, _ string) (*workspace.Team, error) {
	return nil, fmt.Errorf("S3 adapter does not support teams - delegate to API provider")
}

// GetUserTeams is a stub that returns an error for the S3 adapter.
func (a *Adapter) GetUserTeams(_ context.Context, _ string) ([]*workspace.Team, error) {
	return nil, fmt.Errorf("S3 adapter does not support teams - delegate to API provider")
}

// GetTeamMembers is a stub that returns an error for the S3 adapter.
func (a *Adapter) GetTeamMembers(_ context.Context, _ string) ([]*workspace.UserIdentity, error) {
	return nil, fmt.Errorf("S3 adapter does not support teams - delegate to API provider")
}

// =========================================================================
// NotificationProvider stub implementation
// =========================================================================

// SendEmail is a stub that returns an error for the S3 adapter.
func (a *Adapter) SendEmail(_ context.Context, _ []string, _, _, _ string) error {
	return fmt.Errorf("S3 adapter does not support email sending - delegate to API provider or SMTP")
}

// SendEmailWithTemplate is a stub that returns an error for the S3 adapter.
func (a *Adapter) SendEmailWithTemplate(_ context.Context, _ []string, _ string, _ map[string]any) error {
	return fmt.Errorf("S3 adapter does not support email sending - delegate to API provider or SMTP")
}
