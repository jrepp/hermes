package api

import (
	"context"
	"fmt"
	"net/url"

	"github.com/hashicorp-forge/hermes/pkg/workspace"
)

// ===================================================================
// TeamProvider Implementation
// ===================================================================
// All methods delegate to remote Hermes /api/v2/teams/* endpoints

// ListTeams lists teams matching query from remote Hermes
func (p *Provider) ListTeams(ctx context.Context, domain, query string, maxResults int64) ([]*workspace.Team, error) {
	if err := p.checkCapability("groups"); err != nil {
		return nil, err
	}

	params := make(map[string]string)
	if domain != "" {
		params["domain"] = domain
	}
	if query != "" {
		params["q"] = query
	}
	if maxResults > 0 {
		params["maxResults"] = fmt.Sprintf("%d", maxResults)
	}

	// Build query string
	values := url.Values{}
	for k, v := range params {
		values.Set(k, v)
	}
	path := "/api/v2/teams?" + values.Encode()

	var teams []*workspace.Team
	if err := p.doRequest(ctx, "GET", path, nil, &teams); err != nil {
		return nil, fmt.Errorf("failed to list teams: %w", err)
	}

	return teams, nil
}

// GetTeam retrieves team details from remote Hermes
func (p *Provider) GetTeam(ctx context.Context, teamID string) (*workspace.Team, error) {
	if err := p.checkCapability("groups"); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/api/v2/teams/%s", url.PathEscape(teamID))

	var team workspace.Team
	if err := p.doRequest(ctx, "GET", path, nil, &team); err != nil {
		return nil, fmt.Errorf("failed to get team: %w", err)
	}

	return &team, nil
}

// GetUserTeams lists all teams a user belongs to on remote Hermes
func (p *Provider) GetUserTeams(ctx context.Context, userEmail string) ([]*workspace.Team, error) {
	if err := p.checkCapability("groups"); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/api/v2/teams/user/%s", url.PathEscape(userEmail))

	var teams []*workspace.Team
	if err := p.doRequest(ctx, "GET", path, nil, &teams); err != nil {
		return nil, fmt.Errorf("failed to get user teams: %w", err)
	}

	return teams, nil
}

// GetTeamMembers lists all members of a team from remote Hermes
func (p *Provider) GetTeamMembers(ctx context.Context, teamID string) ([]*workspace.UserIdentity, error) {
	if err := p.checkCapability("groups"); err != nil {
		return nil, err
	}

	path := fmt.Sprintf("/api/v2/teams/%s/members", url.PathEscape(teamID))

	var members []*workspace.UserIdentity
	if err := p.doRequest(ctx, "GET", path, nil, &members); err != nil {
		return nil, fmt.Errorf("failed to get team members: %w", err)
	}

	return members, nil
}
