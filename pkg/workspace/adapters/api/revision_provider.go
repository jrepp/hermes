package api

import (
	"context"
	"fmt"
	"net/url"

	"github.com/hashicorp-forge/hermes/pkg/docid"
	"github.com/hashicorp-forge/hermes/pkg/workspace"
)

// ===================================================================
// RevisionTrackingProvider Implementation
// ===================================================================
// All methods delegate to remote Hermes /api/v2/documents/*/revisions endpoints

// GetRevisionHistory lists all revisions for a document from remote Hermes
func (p *Provider) GetRevisionHistory(ctx context.Context, providerID string, limit int) ([]*workspace.BackendRevision, error) {
	if err := p.checkCapability("revisions"); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/api/v2/documents/%s/revisions", url.PathEscape(providerID))
	if limit > 0 {
		path = fmt.Sprintf("%s?limit=%d", path, limit)
	}

	var revisions []*workspace.BackendRevision
	if err := p.doRequest(ctx, "GET", path, nil, &revisions); err != nil {
		return nil, fmt.Errorf("failed to get revision history: %w", err)
	}

	return revisions, nil
}

// GetRevision retrieves a specific revision from remote Hermes
func (p *Provider) GetRevision(ctx context.Context, providerID, revisionID string) (*workspace.BackendRevision, error) {
	if err := p.checkCapability("revisions"); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/api/v2/documents/%s/revisions/%s",
		url.PathEscape(providerID),
		url.PathEscape(revisionID))

	var revision workspace.BackendRevision
	if err := p.doRequest(ctx, "GET", path, nil, &revision); err != nil {
		return nil, fmt.Errorf("failed to get revision: %w", err)
	}

	return &revision, nil
}

// GetRevisionContent retrieves content at a specific revision from remote Hermes
func (p *Provider) GetRevisionContent(ctx context.Context, providerID, revisionID string) (*workspace.DocumentContent, error) {
	if err := p.checkCapability("revisions"); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/api/v2/documents/%s/revisions/%s/content",
		url.PathEscape(providerID),
		url.PathEscape(revisionID))

	var content workspace.DocumentContent
	if err := p.doRequest(ctx, "GET", path, nil, &content); err != nil {
		return nil, fmt.Errorf("failed to get revision content: %w", err)
	}

	return &content, nil
}

// KeepRevisionForever marks a revision as permanent on remote Hermes (if supported)
func (p *Provider) KeepRevisionForever(ctx context.Context, providerID, revisionID string) error {
	if err := p.checkCapability("revisions"); err != nil {
		return err
	}

	path := fmt.Sprintf("/api/v2/documents/%s/revisions/%s/keep",
		url.PathEscape(providerID),
		url.PathEscape(revisionID))

	if err := p.doRequest(ctx, "POST", path, nil, nil); err != nil {
		return fmt.Errorf("failed to keep revision forever: %w", err)
	}

	return nil
}

// GetAllDocumentRevisions returns all revisions across all backends for a UUID from remote Hermes
func (p *Provider) GetAllDocumentRevisions(ctx context.Context, uuid docid.UUID) ([]*workspace.RevisionInfo, error) {
	if err := p.checkCapability("revisions"); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/api/v2/documents/uuid/%s/revisions/all", uuid.String())

	var revisions []*workspace.RevisionInfo
	if err := p.doRequest(ctx, "GET", path, nil, &revisions); err != nil {
		return nil, fmt.Errorf("failed to get all document revisions: %w", err)
	}

	return revisions, nil
}
