// Package openapi describes HTTP routes for OpenAPI generation.
package openapi

// AuthMode describes how an endpoint is authorized.
type AuthMode string

const (
	// AuthModePublic marks routes that are intentionally unauthenticated.
	AuthModePublic AuthMode = "public"

	// AuthModeUser marks routes that require a user session or user auth header.
	AuthModeUser AuthMode = "user"

	// AuthModeToken marks routes that require an endpoint-specific bearer token.
	AuthModeToken AuthMode = "token"

	// AuthModeDeployment marks routes whose auth depends on runtime deployment config.
	AuthModeDeployment AuthMode = "deployment-dependent"
)

// RBAC describes the authorization capability attached to a route.
type RBAC struct {
	Resource      string   `json:"resource"`
	Action        string   `json:"action"`
	Authenticated AuthMode `json:"authenticated"`
}

// Route describes one HTTP operation for OpenAPI generation.
type Route struct {
	Method      string `json:"method"`
	Path        string `json:"path"`
	Summary     string `json:"summary"`
	Description string `json:"description,omitempty"`
	Tag         string `json:"tag"`
	RBAC        RBAC   `json:"x-rbac"`
}

// Routes returns the mounted Hermes HTTP routes that should appear in generated
// OpenAPI output. Keep this list aligned with internal/cmd/commands/server.
func Routes() []Route {
	return []Route{
		public("GET", "/openapi.json", "Get OpenAPI document", "openapi", "openapi", "read"),
		public("GET", "/api/v2/otel/health", "OpenTelemetry listener health check", "otel", "otel.health", "read"),
		public("GET", "/api/v2/otel/promql/query", "Run PromQL instant query", "otel", "otel.promql", "read"),
		public("POST", "/api/v2/otel/promql/query", "Run PromQL instant query", "otel", "otel.promql", "read"),
		public("GET", "/api/v2/otel/promql/query_range", "Run PromQL range query", "otel", "otel.promql", "read"),
		public("POST", "/api/v2/otel/promql/query_range", "Run PromQL range query", "otel", "otel.promql", "read"),
		public("GET", "/health", "Health check", "system", "health", "read"),
		public("GET", "/pub/{path}", "Public document access", "public", "public-documents", "read"),
		public("GET", "/auth/login", "Start login", "auth", "auth.login", "create"),
		public("GET", "/auth/callback", "Complete auth callback", "auth", "auth.callback", "create"),
		public("GET", "/auth/logout", "Log out", "auth", "auth.logout", "delete"),
		public("GET", "/api/v2/web/config", "Get web runtime configuration", "web", "web.config", "read"),
		public("GET", "/api/v2/setup/status", "Get setup status", "setup", "setup.status", "read"),
		public("POST", "/api/v2/setup/configure", "Configure setup", "setup", "setup.configure", "write"),
		public("POST", "/api/v2/setup/validate-ollama", "Validate Ollama setup", "setup", "setup.ollama", "read"),
		public("GET", "/l/{id}", "Short-link redirect", "links", "links.redirect", "read"),
		deployment("GET", "/", "Serve web application", "web", "web.app", "read"),

		user("POST", "/api/v2/approvals/{id}", "Update document approval state", "approvals", "approvals", "update"),
		user("GET", "/api/v2/document-types", "List document types", "document-types", "document-types", "read"),
		user("GET", "/api/v2/documents/{id}", "Read document", "documents", "documents", "read"),
		user("PATCH", "/api/v2/documents/{id}", "Update document", "documents", "documents", "update"),
		user("DELETE", "/api/v2/documents/{id}", "Delete document", "documents", "documents", "delete"),
		user("GET", "/api/v2/documents/{id}/content", "Read document content", "documents", "documents.content", "read"),
		user("GET", "/api/v2/documents/{id}/similar", "Find similar documents", "search", "search.similar-documents", "read"),
		user("GET", "/api/v2/drafts", "List drafts", "drafts", "drafts", "read"),
		user("POST", "/api/v2/drafts", "Create draft", "drafts", "drafts", "create"),
		user("GET", "/api/v2/drafts/{id}", "Read draft", "drafts", "drafts", "read"),
		user("PATCH", "/api/v2/drafts/{id}", "Update draft", "drafts", "drafts", "update"),
		user("DELETE", "/api/v2/drafts/{id}", "Delete draft", "drafts", "drafts", "delete"),
		user("GET", "/api/v2/drafts/{id}/related-resources", "List draft related resources", "drafts", "drafts.related-resources", "read"),
		user("PUT", "/api/v2/drafts/{id}/related-resources", "Replace draft related resources", "drafts", "drafts.related-resources", "update"),
		user("GET", "/api/v2/drafts/{id}/shareable", "Get draft shareability", "drafts", "drafts.shareable", "read"),
		user("PUT", "/api/v2/drafts/{id}/shareable", "Set draft shareability", "drafts", "drafts.shareable", "update"),
		user("POST", "/api/v2/groups", "Search groups", "groups", "groups", "read"),
		user("GET", "/api/v2/jira/issues/{key}", "Get Jira issue details", "jira", "jira.issues", "read"),
		user("GET", "/api/v2/jira/issue/picker", "Search Jira issues", "jira", "jira.issues", "read"),
		user("GET", "/api/v2/me", "Get current user profile", "me", "me", "read"),
		user("GET", "/api/v2/me/recently-viewed-docs", "Get recently viewed documents", "me", "me.recently-viewed-docs", "read"),
		user("GET", "/api/v2/me/recently-viewed-projects", "Get recently viewed projects", "me", "me.recently-viewed-projects", "read"),
		user("GET", "/api/v2/me/reviews", "Get reviews for current user", "me", "me.reviews", "read"),
		user("GET", "/api/v2/me/subscriptions", "Get current user subscriptions", "me", "me.subscriptions", "read"),
		user("PUT", "/api/v2/me/subscriptions", "Update current user subscriptions", "me", "me.subscriptions", "update"),
		user("GET", "/api/v2/migrations/{id}", "Read migration state", "migrations", "migrations", "read"),
		user("GET", "/api/v2/people", "Get people by email", "people", "people", "read"),
		user("POST", "/api/v2/people", "Search people", "people", "people", "read"),
		user("GET", "/api/v2/products", "List products", "products", "products", "read"),
		user("GET", "/api/v2/projects", "List projects", "projects", "projects", "read"),
		user("POST", "/api/v2/projects", "Create project", "projects", "projects", "create"),
		user("GET", "/api/v2/projects/{id}", "Read project", "projects", "projects", "read"),
		user("PATCH", "/api/v2/projects/{id}", "Update project", "projects", "projects", "update"),
		user("GET", "/api/v2/projects/{id}/related-resources", "List project related resources", "projects", "projects.related-resources", "read"),
		user("PUT", "/api/v2/projects/{id}/related-resources", "Replace project related resources", "projects", "projects.related-resources", "update"),
		user("GET", "/api/v2/providers", "List configured providers", "providers", "providers", "read"),
		user("GET", "/api/v2/providers/{type}", "Read configured provider", "providers", "providers", "read"),
		user("POST", "/api/v2/reviews/{id}", "Create review request", "reviews", "reviews", "create"),
		user("GET", "/api/v2/search/{index}", "Search documents and drafts", "search", "search", "read"),
		user("POST", "/api/v2/search/{index}", "Search documents and drafts", "search", "search", "read"),
		user("POST", "/api/v2/search/semantic", "Run semantic search", "search", "search.semantic", "read"),
		user("POST", "/api/v2/search/hybrid", "Run hybrid search", "search", "search.hybrid", "read"),
		user("POST", "/api/v2/web/analytics", "Record web analytics event", "web", "web.analytics", "create"),
		user("GET", "/api/v2/workspace-projects", "List workspace projects", "workspace-projects", "workspace-projects", "read"),
		user("GET", "/api/v2/workspace-projects/{id}", "Read workspace project", "workspace-projects", "workspace-projects", "read"),
		user("GET", "/1/indexes/{index}", "Algolia-compatible backend search proxy", "search", "search.proxy", "read"),
		user("POST", "/1/indexes/{index}", "Algolia-compatible backend search proxy", "search", "search.proxy", "read"),

		token("POST", "/api/v2/indexer/{path}", "Indexer ingestion API", "indexer", "indexer", "write"),
		token("GET", "/api/v2/edge/{path}", "Edge sync API", "edge", "edge-sync", "read"),
		token("POST", "/api/v2/edge/{path}", "Edge sync API", "edge", "edge-sync", "write"),
		token("PATCH", "/api/v2/edge/{path}", "Edge sync API", "edge", "edge-sync", "update"),
		token("DELETE", "/api/v2/edge/{path}", "Edge sync API", "edge", "edge-sync", "delete"),
	}
}

func public(method, path, summary, tag, resource, action string) Route {
	return route(method, path, summary, tag, resource, action, AuthModePublic)
}

func user(method, path, summary, tag, resource, action string) Route {
	return route(method, path, summary, tag, resource, action, AuthModeUser)
}

func token(method, path, summary, tag, resource, action string) Route {
	return route(method, path, summary, tag, resource, action, AuthModeToken)
}

func deployment(method, path, summary, tag, resource, action string) Route {
	return route(method, path, summary, tag, resource, action, AuthModeDeployment)
}

func route(method, path, summary, tag, resource, action string, auth AuthMode) Route {
	return Route{
		Method:  method,
		Path:    path,
		Summary: summary,
		Tag:     tag,
		RBAC: RBAC{
			Resource:      resource,
			Action:        action,
			Authenticated: auth,
		},
	}
}
