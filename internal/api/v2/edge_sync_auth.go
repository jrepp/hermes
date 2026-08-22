package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/hashicorp-forge/hermes/internal/server"
	"github.com/hashicorp-forge/hermes/pkg/models"
)

// EdgeSyncAuthMiddleware validates API tokens for edge-to-central communication.
// Uses Bearer token authentication with the service_tokens table.
//
// Token validation:
//   - Checks Authorization: Bearer <token> header
//   - Validates token exists and is not expired/revoked
//   - Verifies token type is "edge" or "api"
//
// Usage:
//
//	handler := EdgeSyncAuthMiddleware(srv, EdgeSyncHandler(srv))
func EdgeSyncAuthMiddleware(srv server.Server, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Bind to the site serving this request before touching the
		// database; srv as constructed holds the process-wide default.
		srv, ok := srv.ForRequest(w, r)
		if !ok {
			return
		}

		// Extract Bearer token from Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			srv.Logger.Warn("edge sync: missing authorization header",
				"path", r.URL.Path,
				"method", r.Method,
			)
			http.Error(w, "Missing authorization header", http.StatusUnauthorized)
			return
		}

		if !strings.HasPrefix(authHeader, "Bearer ") {
			srv.Logger.Warn("edge sync: invalid authorization header format",
				"path", r.URL.Path,
				"method", r.Method,
			)
			http.Error(w, "Invalid authorization header format", http.StatusUnauthorized)
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token == "" {
			srv.Logger.Warn("edge sync: empty bearer token",
				"path", r.URL.Path,
				"method", r.Method,
			)
			http.Error(w, "Empty bearer token", http.StatusUnauthorized)
			return
		}

		// Validate token against database
		var indexerToken models.IndexerToken
		if err := indexerToken.GetByToken(srv.DB, token); err != nil {
			srv.Logger.Warn("edge sync: invalid API token",
				"error", err,
				"path", r.URL.Path,
				"method", r.Method,
			)
			http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
			return
		}

		// Check if token is valid (not expired or revoked)
		if !indexerToken.IsValid() {
			srv.Logger.Warn("edge sync: token expired or revoked",
				"token_id", indexerToken.ID,
				"path", r.URL.Path,
				"method", r.Method,
			)
			http.Error(w, "Token has expired or been revoked", http.StatusUnauthorized)
			return
		}

		// Verify token type is appropriate for edge sync
		// Accept both "edge" (edge-specific tokens) and "api" (general API tokens)
		if indexerToken.TokenType != "edge" && indexerToken.TokenType != "api" {
			srv.Logger.Warn("edge sync: invalid token type",
				"token_type", indexerToken.TokenType,
				"token_id", indexerToken.ID,
				"path", r.URL.Path,
				"method", r.Method,
			)
			http.Error(w, "Invalid token type for edge sync", http.StatusForbidden)
			return
		}

		// Token is valid, proceed to handler
		srv.Logger.Debug("edge sync: authenticated request",
			"token_id", indexerToken.ID,
			"token_type", indexerToken.TokenType,
			"path", r.URL.Path,
			"method", r.Method,
		)

		next.ServeHTTP(w, r)
	})
}

// CreateEdgeSyncToken creates a new API token for edge-to-central authentication.
// This is a helper function for generating tokens programmatically.
//
// Token characteristics:
//   - Type: "edge"
//   - No expiration (nil ExpiresAt)
//   - Revocable: true (can be revoked via RevokedAt)
//   - Stored as SHA-256 hash
//
// Returns the plaintext token (only time it's available) and error.
//
// The context selects the site the token belongs to. A token is stored in one
// site's schema and is rejected by every other, so writing it to whichever
// database happened to be on the Server would produce a token that
// authenticates against a site the caller did not mean -- or, more likely, a
// token the edge instance is then told is invalid. Taking the context makes
// that choice explicit rather than incidental.
func CreateEdgeSyncToken(
	ctx context.Context, srv server.Server, edgeInstance string,
) (string, error) {
	srv, err := srv.ForDomain(ctx)
	if err != nil {
		return "", err
	}

	plaintext, err := models.GenerateToken("edge")
	if err != nil {
		return "", err
	}

	token := models.IndexerToken{
		TokenType: "edge",
		ExpiresAt: nil, // No expiration
	}

	if err := token.Create(srv.DB, plaintext); err != nil {
		return "", err
	}

	srv.Logger.Info("created edge sync token",
		"token_id", token.ID,
		"edge_instance", edgeInstance,
	)

	return plaintext, nil
}
