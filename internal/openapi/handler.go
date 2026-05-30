package openapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler serves the generated OpenAPI document.
//
// @Summary Get OpenAPI document
// @Tags openapi
// @Produce json
// @Success 200 {object} map[string]any
// @x-rbac {"resource":"openapi","action":"read","authenticated":false}
// @Router /openapi.json [get]
func Handler() http.Handler {
	r := gin.New()
	r.GET("/openapi.json", func(c *gin.Context) {
		c.JSON(http.StatusOK, Document())
	})
	r.NoRoute(func(c *gin.Context) {
		if c.Request.Method != http.MethodGet {
			c.Status(http.StatusMethodNotAllowed)
			return
		}
		c.JSON(http.StatusOK, Document())
	})
	return r
}

// Document returns a minimal OpenAPI document generated from route metadata.
func Document() map[string]any {
	paths := map[string]any{}
	routes := Routes()
	for i := range routes {
		route := routes[i]
		pathItem, ok := paths[route.Path].(map[string]any)
		if !ok {
			pathItem = map[string]any{}
			paths[route.Path] = pathItem
		}

		operation := map[string]any{
			"summary": route.Summary,
			"tags":    []string{route.Tag},
			"responses": map[string]any{
				"200": map[string]any{"description": "OK"},
			},
			"x-rbac": route.RBAC,
		}
		if route.Description != "" {
			operation["description"] = route.Description
		}
		if route.RBAC.Authenticated == AuthModeUser {
			operation["security"] = []map[string][]string{{"UserAuth": {}}}
		}
		if route.RBAC.Authenticated == AuthModeToken {
			operation["security"] = []map[string][]string{{"BearerToken": {}}}
		}

		pathItem[methodKey(route.Method)] = operation
	}

	return map[string]any{
		"openapi": "3.1.0",
		"info": map[string]any{
			"title":   "Hermes API",
			"version": "0.0.0",
		},
		"paths": paths,
		"components": map[string]any{
			"securitySchemes": map[string]any{
				"UserAuth": map[string]any{
					"type":        "apiKey",
					"in":          "header",
					"name":        "Hermes-Google-Access-Token",
					"description": "Runtime-selected user auth; OIDC deployments may use session cookies instead.",
				},
				"BearerToken": map[string]any{
					"type":         "http",
					"scheme":       "bearer",
					"bearerFormat": "opaque",
				},
			},
		},
	}
}

func methodKey(method string) string {
	switch method {
	case http.MethodGet:
		return "get"
	case http.MethodPost:
		return "post"
	case http.MethodPut:
		return "put"
	case http.MethodPatch:
		return "patch"
	case http.MethodDelete:
		return "delete"
	default:
		return method
	}
}
