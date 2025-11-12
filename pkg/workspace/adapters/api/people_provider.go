package api

import (
	"context"
	"fmt"
	"net/url"

	"github.com/hashicorp-forge/hermes/pkg/workspace"
)

// ===================================================================
// PeopleProvider Implementation
// ===================================================================
// All methods delegate to remote Hermes /api/v2/people/* endpoints

// SearchPeople searches for users in the directory on remote Hermes
func (p *Provider) SearchPeople(ctx context.Context, query string) ([]*workspace.UserIdentity, error) {
	if err := p.checkCapability("directory"); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/api/v2/people/search?q=%s", url.QueryEscape(query))

	var people []*workspace.UserIdentity
	if err := p.doRequest(ctx, "GET", path, nil, &people); err != nil {
		return nil, fmt.Errorf("failed to search people: %w", err)
	}

	return people, nil
}

// GetPerson retrieves a user by email from remote Hermes
func (p *Provider) GetPerson(ctx context.Context, email string) (*workspace.UserIdentity, error) {
	if err := p.checkCapability("directory"); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/api/v2/people/%s", url.PathEscape(email))

	var person workspace.UserIdentity
	if err := p.doRequest(ctx, "GET", path, nil, &person); err != nil {
		return nil, fmt.Errorf("failed to get person: %w", err)
	}

	return &person, nil
}

// GetPersonByUnifiedID retrieves user by unified ID from remote Hermes (cross-provider lookup)
func (p *Provider) GetPersonByUnifiedID(ctx context.Context, unifiedID string) (*workspace.UserIdentity, error) {
	if err := p.checkCapability("directory"); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/api/v2/people/unified/%s", url.PathEscape(unifiedID))

	var person workspace.UserIdentity
	if err := p.doRequest(ctx, "GET", path, nil, &person); err != nil {
		return nil, fmt.Errorf("failed to get person by unified ID: %w", err)
	}

	return &person, nil
}

// ResolveIdentity resolves alternate identities for a user on remote Hermes
func (p *Provider) ResolveIdentity(ctx context.Context, email string) (*workspace.UserIdentity, error) {
	if err := p.checkCapability("directory"); err != nil {
		return nil, err
	}

	path := "/api/v2/people/resolve"

	requestBody := map[string]string{
		"email": email,
	}

	var identity workspace.UserIdentity
	if err := p.doRequest(ctx, "POST", path, requestBody, &identity); err != nil {
		return nil, fmt.Errorf("failed to resolve identity: %w", err)
	}

	return &identity, nil
}
