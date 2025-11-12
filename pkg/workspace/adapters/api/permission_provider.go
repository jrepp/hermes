package api

import (
	"context"
	"fmt"
	"net/url"

	"github.com/hashicorp-forge/hermes/pkg/workspace"
)

// ===================================================================
// PermissionProvider Implementation
// ===================================================================
// All methods delegate to remote Hermes /api/v2/documents/*/permissions endpoints

// ShareDocument grants access to a user/group on remote Hermes
func (p *Provider) ShareDocument(ctx context.Context, providerID, email, role string) error {
	if err := p.checkCapability("permissions"); err != nil {
		return err
	}

	path := fmt.Sprintf("/api/v2/documents/%s/permissions", url.PathEscape(providerID))

	requestBody := map[string]string{
		"email": email,
		"role":  role,
	}

	if err := p.doRequest(ctx, "POST", path, requestBody, nil); err != nil {
		return fmt.Errorf("failed to share document: %w", err)
	}

	return nil
}

// ShareDocumentWithDomain grants access to entire domain on remote Hermes
func (p *Provider) ShareDocumentWithDomain(ctx context.Context, providerID, domain, role string) error {
	if err := p.checkCapability("permissions"); err != nil {
		return err
	}

	path := fmt.Sprintf("/api/v2/documents/%s/permissions/domain", url.PathEscape(providerID))

	requestBody := map[string]string{
		"domain": domain,
		"role":   role,
	}

	if err := p.doRequest(ctx, "POST", path, requestBody, nil); err != nil {
		return fmt.Errorf("failed to share document with domain: %w", err)
	}

	return nil
}

// ListPermissions lists all permissions for a document from remote Hermes
func (p *Provider) ListPermissions(ctx context.Context, providerID string) ([]*workspace.FilePermission, error) {
	if err := p.checkCapability("permissions"); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/api/v2/documents/%s/permissions", url.PathEscape(providerID))

	var permissions []*workspace.FilePermission
	if err := p.doRequest(ctx, "GET", path, nil, &permissions); err != nil {
		return nil, fmt.Errorf("failed to list permissions: %w", err)
	}

	return permissions, nil
}

// RemovePermission revokes access on remote Hermes
func (p *Provider) RemovePermission(ctx context.Context, providerID, permissionID string) error {
	if err := p.checkCapability("permissions"); err != nil {
		return err
	}

	path := fmt.Sprintf("/api/v2/documents/%s/permissions/%s",
		url.PathEscape(providerID),
		url.PathEscape(permissionID))

	if err := p.doRequest(ctx, "DELETE", path, nil, nil); err != nil {
		return fmt.Errorf("failed to remove permission: %w", err)
	}

	return nil
}

// UpdatePermission changes permission role on remote Hermes
func (p *Provider) UpdatePermission(ctx context.Context, providerID, permissionID, newRole string) error {
	if err := p.checkCapability("permissions"); err != nil {
		return err
	}

	path := fmt.Sprintf("/api/v2/documents/%s/permissions/%s",
		url.PathEscape(providerID),
		url.PathEscape(permissionID))

	requestBody := map[string]string{
		"role": newRole,
	}

	if err := p.doRequest(ctx, "PATCH", path, requestBody, nil); err != nil {
		return fmt.Errorf("failed to update permission: %w", err)
	}

	return nil
}
