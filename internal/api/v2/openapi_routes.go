// Package api contains V2 HTTP API handlers.
//
//nolint:unused // OpenAPI annotation stubs are discovered by documentation generators.
package api

// openAPIRouteAnnotations documents mounted V2 routes for generators that read
// swag-style comments. Runtime routing still lives in internal/cmd/commands/server.
func openAPIRouteAnnotations() {}

// @Summary Update document approval state
// @Tags approvals
// @Security UserAuth
// @x-rbac {"resource":"approvals","action":"update","authenticated":true}
// @Router /api/v2/approvals/{id} [post]
func openAPIApprovalsRoute() {}

// @Summary List document types
// @Tags document-types
// @Security UserAuth
// @x-rbac {"resource":"document-types","action":"read","authenticated":true}
// @Router /api/v2/document-types [get]
func openAPIDocumentTypesRoute() {}

// @Summary Read or mutate a document
// @Tags documents
// @Security UserAuth
// @x-rbac {"resource":"documents","action":"read-write","authenticated":true}
// @Router /api/v2/documents/{id} [get]
// @Router /api/v2/documents/{id} [patch]
// @Router /api/v2/documents/{id} [delete]
func openAPIDocumentRoute() {}

// @Summary Read document content
// @Tags documents
// @Security UserAuth
// @x-rbac {"resource":"documents.content","action":"read","authenticated":true}
// @Router /api/v2/documents/{id}/content [get]
func openAPIDocumentContentRoute() {}

// @Summary Find similar documents
// @Tags search
// @Security UserAuth
// @x-rbac {"resource":"search.similar-documents","action":"read","authenticated":true}
// @Router /api/v2/documents/{id}/similar [get]
func openAPISimilarDocumentsRoute() {}

// @Summary Create or list drafts
// @Tags drafts
// @Security UserAuth
// @x-rbac {"resource":"drafts","action":"read-write","authenticated":true}
// @Router /api/v2/drafts [get]
// @Router /api/v2/drafts [post]
func openAPIDraftsRoute() {}

// @Summary Read or mutate a draft
// @Tags drafts
// @Security UserAuth
// @x-rbac {"resource":"drafts","action":"read-write","authenticated":true}
// @Router /api/v2/drafts/{id} [get]
// @Router /api/v2/drafts/{id} [patch]
// @Router /api/v2/drafts/{id} [delete]
func openAPIDraftRoute() {}

// @Summary Search groups
// @Tags groups
// @Security UserAuth
// @x-rbac {"resource":"groups","action":"read","authenticated":true}
// @Router /api/v2/groups [post]
func openAPIGroupsRoute() {}

// @Summary Get Jira issue details
// @Tags jira
// @Security UserAuth
// @x-rbac {"resource":"jira.issues","action":"read","authenticated":true}
// @Router /api/v2/jira/issues/{key} [get]
func openAPIJiraIssueRoute() {}

// @Summary Search Jira issues
// @Tags jira
// @Security UserAuth
// @x-rbac {"resource":"jira.issues","action":"read","authenticated":true}
// @Router /api/v2/jira/issue/picker [get]
func openAPIJiraIssuePickerRoute() {}

// @Summary Get current user profile
// @Tags me
// @Security UserAuth
// @x-rbac {"resource":"me","action":"read","authenticated":true}
// @Router /api/v2/me [get]
func openAPIMeRoute() {}

// @Summary Get recently viewed documents
// @Tags me
// @Security UserAuth
// @x-rbac {"resource":"me.recently-viewed-docs","action":"read","authenticated":true}
// @Router /api/v2/me/recently-viewed-docs [get]
func openAPIMeRecentlyViewedDocsRoute() {}

// @Summary Get recently viewed projects
// @Tags me
// @Security UserAuth
// @x-rbac {"resource":"me.recently-viewed-projects","action":"read","authenticated":true}
// @Router /api/v2/me/recently-viewed-projects [get]
func openAPIMeRecentlyViewedProjectsRoute() {}

// @Summary Get reviews for current user
// @Tags me
// @Security UserAuth
// @x-rbac {"resource":"me.reviews","action":"read","authenticated":true}
// @Router /api/v2/me/reviews [get]
func openAPIMeReviewsRoute() {}

// @Summary Manage current user subscriptions
// @Tags me
// @Security UserAuth
// @x-rbac {"resource":"me.subscriptions","action":"read-write","authenticated":true}
// @Router /api/v2/me/subscriptions [get]
// @Router /api/v2/me/subscriptions [put]
func openAPIMeSubscriptionsRoute() {}

// @Summary Read migration state
// @Tags migrations
// @Security UserAuth
// @x-rbac {"resource":"migrations","action":"read","authenticated":true}
// @Router /api/v2/migrations/{id} [get]
func openAPIMigrationsRoute() {}

// @Summary Search people
// @Tags people
// @Security UserAuth
// @x-rbac {"resource":"people","action":"read","authenticated":true}
// @Router /api/v2/people [get]
// @Router /api/v2/people [post]
func openAPIPeopleRoute() {}

// @Summary List products
// @Tags products
// @Security UserAuth
// @x-rbac {"resource":"products","action":"read","authenticated":true}
// @Router /api/v2/products [get]
func openAPIProductsRoute() {}

// @Summary List or create projects
// @Tags projects
// @Security UserAuth
// @x-rbac {"resource":"projects","action":"read-write","authenticated":true}
// @Router /api/v2/projects [get]
// @Router /api/v2/projects [post]
func openAPIProjectsRoute() {}

// @Summary Read or update a project
// @Tags projects
// @Security UserAuth
// @x-rbac {"resource":"projects","action":"read-write","authenticated":true}
// @Router /api/v2/projects/{id} [get]
// @Router /api/v2/projects/{id} [patch]
func openAPIProjectRoute() {}

// @Summary List configured providers
// @Tags providers
// @Security UserAuth
// @x-rbac {"resource":"providers","action":"read","authenticated":true}
// @Router /api/v2/providers [get]
// @Router /api/v2/providers/{type} [get]
func openAPIProvidersRoute() {}

// @Summary Create review request
// @Tags reviews
// @Security UserAuth
// @x-rbac {"resource":"reviews","action":"create","authenticated":true}
// @Router /api/v2/reviews/{id} [post]
func openAPIReviewsRoute() {}

// @Summary Search documents and drafts
// @Tags search
// @Security UserAuth
// @x-rbac {"resource":"search","action":"read","authenticated":true}
// @Router /api/v2/search/{index} [get]
// @Router /api/v2/search/{index} [post]
func openAPISearchRoute() {}

// @Summary Run semantic search
// @Tags search
// @Security UserAuth
// @x-rbac {"resource":"search.semantic","action":"read","authenticated":true}
// @Router /api/v2/search/semantic [post]
func openAPISemanticSearchRoute() {}

// @Summary Run hybrid search
// @Tags search
// @Security UserAuth
// @x-rbac {"resource":"search.hybrid","action":"read","authenticated":true}
// @Router /api/v2/search/hybrid [post]
func openAPIHybridSearchRoute() {}

// @Summary List workspace projects
// @Tags workspace-projects
// @Security UserAuth
// @x-rbac {"resource":"workspace-projects","action":"read","authenticated":true}
// @Router /api/v2/workspace-projects [get]
func openAPIWorkspaceProjectsRoute() {}

// @Summary Read workspace project
// @Tags workspace-projects
// @Security UserAuth
// @x-rbac {"resource":"workspace-projects","action":"read","authenticated":true}
// @Router /api/v2/workspace-projects/{id} [get]
func openAPIWorkspaceProjectRoute() {}

// @Summary Algolia-compatible backend search proxy
// @Tags search
// @Security UserAuth
// @x-rbac {"resource":"search.proxy","action":"read","authenticated":true}
// @Router /1/indexes/{index} [get]
// @Router /1/indexes/{index} [post]
func openAPIAlgoliaProxyRoute() {}

// @Summary Indexer ingestion API
// @Tags indexer
// @Security IndexerToken
// @x-rbac {"resource":"indexer","action":"write","authenticated":"token"}
// @Router /api/v2/indexer/{path} [post]
func openAPIIndexerRoute() {}

// @Summary Edge sync API
// @Tags edge
// @Security EdgeSyncToken
// @x-rbac {"resource":"edge-sync","action":"write","authenticated":"token"}
// @Router /api/v2/edge/{path} [get]
// @Router /api/v2/edge/{path} [post]
// @Router /api/v2/edge/{path} [patch]
// @Router /api/v2/edge/{path} [delete]
func openAPIEdgeRoute() {}

// @Summary Get setup status
// @Tags setup
// @x-rbac {"resource":"setup.status","action":"read","authenticated":false}
// @Router /api/v2/setup/status [get]
func openAPISetupStatusRoute() {}

// @Summary Configure setup
// @Tags setup
// @x-rbac {"resource":"setup.configure","action":"write","authenticated":false}
// @Router /api/v2/setup/configure [post]
func openAPISetupConfigureRoute() {}

// @Summary Validate Ollama setup
// @Tags setup
// @x-rbac {"resource":"setup.ollama","action":"read","authenticated":false}
// @Router /api/v2/setup/validate-ollama [post]
func openAPIOllamaValidateRoute() {}
