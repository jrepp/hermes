package api

import (
	"context"
	"fmt"
	"net/url"

	"github.com/hashicorp-forge/hermes/pkg/docid"
	"github.com/hashicorp-forge/hermes/pkg/workspace"
)

// ===================================================================
// ContentProvider Implementation
// ===================================================================
// All methods delegate to remote Hermes /api/v2/documents/*/content endpoints

// GetContent retrieves document content from remote Hermes
func (p *Provider) GetContent(ctx context.Context, providerID string) (*workspace.DocumentContent, error) {
	if err := p.checkCapability("content"); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/api/v2/documents/%s/content", url.PathEscape(providerID))

	var content workspace.DocumentContent
	if err := p.doRequest(ctx, "GET", path, nil, &content); err != nil {
		return nil, fmt.Errorf("failed to get content: %w", err)
	}

	return &content, nil
}

// GetContentByUUID retrieves content using UUID from remote Hermes
func (p *Provider) GetContentByUUID(ctx context.Context, uuid docid.UUID) (*workspace.DocumentContent, error) {
	if err := p.checkCapability("content"); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/api/v2/documents/uuid/%s/content", uuid.String())

	var content workspace.DocumentContent
	if err := p.doRequest(ctx, "GET", path, nil, &content); err != nil {
		return nil, fmt.Errorf("failed to get content by UUID: %w", err)
	}

	return &content, nil
}

// UpdateContent updates document content on remote Hermes
func (p *Provider) UpdateContent(ctx context.Context, providerID string, content string) (*workspace.DocumentContent, error) {
	if err := p.checkCapability("content"); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/api/v2/documents/%s/content", url.PathEscape(providerID))

	requestBody := map[string]string{
		"content": content,
	}

	var updatedContent workspace.DocumentContent
	if err := p.doRequest(ctx, "PUT", path, requestBody, &updatedContent); err != nil {
		return nil, fmt.Errorf("failed to update content: %w", err)
	}

	return &updatedContent, nil
}

// GetContentBatch retrieves multiple documents' content from remote Hermes (efficient for migration)
func (p *Provider) GetContentBatch(ctx context.Context, providerIDs []string) ([]*workspace.DocumentContent, error) {
	if err := p.checkCapability("content"); err != nil {
		return nil, err
	}

	path := "/api/v2/documents/batch/content"

	requestBody := map[string][]string{
		"providerIDs": providerIDs,
	}

	var contents []*workspace.DocumentContent
	if err := p.doRequest(ctx, "POST", path, requestBody, &contents); err != nil {
		return nil, fmt.Errorf("failed to get content batch: %w", err)
	}

	return contents, nil
}

// CompareContent compares content between two revisions on remote Hermes
func (p *Provider) CompareContent(ctx context.Context, providerID1, providerID2 string) (*workspace.ContentComparison, error) {
	if err := p.checkCapability("content"); err != nil {
		return nil, err
	}

	path := "/api/v2/documents/compare"

	requestBody := map[string]string{
		"providerID1": providerID1,
		"providerID2": providerID2,
	}

	var comparison workspace.ContentComparison
	if err := p.doRequest(ctx, "POST", path, requestBody, &comparison); err != nil {
		return nil, fmt.Errorf("failed to compare content: %w", err)
	}

	return &comparison, nil
}
