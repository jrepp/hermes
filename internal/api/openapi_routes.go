// Package api contains auth and legacy HTTP handlers.
//
//nolint:unused // OpenAPI annotation stubs are discovered by documentation generators.
package api

// openAPIRouteAnnotations documents mounted auth routes for generators that read
// swag-style comments. Runtime routing still lives in internal/cmd/commands/server.
func openAPIRouteAnnotations() {}

// @Summary Start login
// @Tags auth
// @x-rbac {"resource":"auth.login","action":"create","authenticated":false}
// @Router /auth/login [get]
func openAPILoginRoute() {}

// @Summary Complete auth callback
// @Tags auth
// @x-rbac {"resource":"auth.callback","action":"create","authenticated":false}
// @Router /auth/callback [get]
func openAPICallbackRoute() {}

// @Summary Log out
// @Tags auth
// @x-rbac {"resource":"auth.logout","action":"delete","authenticated":false}
// @Router /auth/logout [get]
func openAPILogoutRoute() {}
