// Package server provides server command routing.
//
//nolint:unused // OpenAPI annotation stubs are discovered by documentation generators.
package server

// openAPIRouteAnnotations documents mounted infrastructure routes for generators
// that read swag-style comments. Runtime routing stays in server.go.
func openAPIRouteAnnotations() {}

// @Summary Health check
// @Tags system
// @x-rbac {"resource":"health","action":"read","authenticated":false}
// @Router /health [get]
func openAPIHealthRoute() {}

// @Summary Get OpenAPI document
// @Tags openapi
// @x-rbac {"resource":"openapi","action":"read","authenticated":false}
// @Router /openapi.json [get]
func openAPIOpenAPIDocumentRoute() {}

// @Summary Public document access
// @Tags public
// @x-rbac {"resource":"public-documents","action":"read","authenticated":false}
// @Router /pub/{path} [get]
func openAPIPublicDocumentRoute() {}

// @Summary Short-link redirect
// @Tags links
// @x-rbac {"resource":"links.redirect","action":"read","authenticated":false}
// @Router /l/{id} [get]
func openAPIShortLinkRoute() {}
