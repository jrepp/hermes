//go:build integration
// +build integration

package edgesync

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/go-hclog"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	apiv2 "github.com/hashicorp-forge/hermes/internal/api/v2"
	"github.com/hashicorp-forge/hermes/internal/server"
	"github.com/hashicorp-forge/hermes/tests/integration"
)

// TestEdgeSyncAuthenticationMiddleware tests the Bearer token authentication middleware
// for edge-to-central communication as specified in RFC-086.
func TestEdgeSyncAuthenticationMiddleware(t *testing.T) {
	fixture := integration.GetFixture()
	ctx := context.Background()

	// Open database connection
	db, err := sql.Open("pgx", fixture.PostgresURL)
	require.NoError(t, err, "Should connect to PostgreSQL")
	defer db.Close()

	// Apply migrations (ensure service_tokens and edge_document_registry tables exist)
	err = applyMigrations(ctx, db)
	require.NoError(t, err, "Should apply migrations")

	// Convert sql.DB to gorm.DB
	gormDB, err := gorm.Open(postgres.New(postgres.Config{Conn: db}), &gorm.Config{})
	require.NoError(t, err)

	// Create server struct
	srv := server.Server{
		DB:     gormDB,
		Logger: hclog.NewNullLogger(),
	}

	t.Run("RejectMissingAuthorizationHeader", func(t *testing.T) {
		handler := apiv2.EdgeSyncAuthMiddleware(srv, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("authenticated"))
		}))

		req := httptest.NewRequest("GET", "/api/v2/edge/documents/sync-status", http.NoBody)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "Missing authorization header")
	})

	t.Run("RejectInvalidAuthorizationFormat", func(t *testing.T) {
		handler := apiv2.EdgeSyncAuthMiddleware(srv, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/api/v2/edge/documents/sync-status", http.NoBody)
		req.Header.Set("Authorization", "InvalidFormat token123")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "Invalid authorization header format")
	})

	t.Run("RejectEmptyBearerToken", func(t *testing.T) {
		handler := apiv2.EdgeSyncAuthMiddleware(srv, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/api/v2/edge/documents/sync-status", http.NoBody)
		req.Header.Set("Authorization", "Bearer ")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "Empty bearer token")
	})

	t.Run("RejectNonExistentToken", func(t *testing.T) {
		handler := apiv2.EdgeSyncAuthMiddleware(srv, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		fakeToken := generateTestToken("edge")
		req := httptest.NewRequest("GET", "/api/v2/edge/documents/sync-status", http.NoBody)
		req.Header.Set("Authorization", "Bearer "+fakeToken)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "Invalid or expired token")
	})

	t.Run("RejectRevokedToken", func(t *testing.T) {
		// Create revoked token
		token := generateTestToken("edge")
		tokenID := insertToken(ctx, t, db, token, "edge", true, nil)
		defer deleteToken(ctx, t, db, tokenID)

		handler := apiv2.EdgeSyncAuthMiddleware(srv, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/api/v2/edge/documents/sync-status", http.NoBody)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "expired or been revoked")
	})

	t.Run("RejectExpiredToken", func(t *testing.T) {
		// Create expired token
		token := generateTestToken("edge")
		expiredTime := time.Now().Add(-24 * time.Hour)
		tokenID := insertToken(ctx, t, db, token, "edge", false, &expiredTime)
		defer deleteToken(ctx, t, db, tokenID)

		handler := apiv2.EdgeSyncAuthMiddleware(srv, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/api/v2/edge/documents/sync-status", http.NoBody)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "expired or been revoked")
	})

	t.Run("RejectWrongTokenType", func(t *testing.T) {
		// Create token with wrong type
		token := generateTestToken("registration")
		tokenID := insertToken(ctx, t, db, token, "registration", false, nil)
		defer deleteToken(ctx, t, db, tokenID)

		handler := apiv2.EdgeSyncAuthMiddleware(srv, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/api/v2/edge/documents/sync-status", http.NoBody)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Contains(t, w.Body.String(), "Invalid token type")
	})

	t.Run("AcceptValidEdgeToken", func(t *testing.T) {
		// Create valid edge token
		token := generateTestToken("edge")
		tokenID := insertToken(ctx, t, db, token, "edge", false, nil)
		defer deleteToken(ctx, t, db, tokenID)

		called := false
		handler := apiv2.EdgeSyncAuthMiddleware(srv, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			called = true
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("success"))
		}))

		req := httptest.NewRequest("GET", "/api/v2/edge/documents/sync-status", http.NoBody)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.True(t, called, "Handler should be called for valid token")
		assert.Equal(t, "success", w.Body.String())
	})

	t.Run("AcceptValidAPIToken", func(t *testing.T) {
		// Create valid api token (also accepted for edge sync)
		token := generateTestToken("api")
		tokenID := insertToken(ctx, t, db, token, "api", false, nil)
		defer deleteToken(ctx, t, db, tokenID)

		called := false
		handler := apiv2.EdgeSyncAuthMiddleware(srv, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			called = true
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/api/v2/edge/documents/sync-status", http.NoBody)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.True(t, called, "Handler should be called for valid API token")
	})

	t.Run("AcceptTokenWithFutureExpiration", func(t *testing.T) {
		// Create token that expires in the future
		token := generateTestToken("edge")
		futureTime := time.Now().Add(24 * time.Hour)
		tokenID := insertToken(ctx, t, db, token, "edge", false, &futureTime)
		defer deleteToken(ctx, t, db, tokenID)

		called := false
		handler := apiv2.EdgeSyncAuthMiddleware(srv, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			called = true
			w.WriteHeader(http.StatusOK)
		}))

		req := httptest.NewRequest("GET", "/api/v2/edge/documents/sync-status", http.NoBody)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.True(t, called, "Handler should be called for token with future expiration")
	})
}

// TestEdgeSyncEndpointsIntegration tests actual edge sync API endpoints with authentication.
// This test requires the database to have proper schema (migrations applied).
func TestEdgeSyncEndpointsIntegration(t *testing.T) {
	fixture := integration.GetFixture()
	ctx := context.Background()

	// Open database connection
	db, err := sql.Open("pgx", fixture.PostgresURL)
	require.NoError(t, err, "Should connect to PostgreSQL")
	defer db.Close()

	// Apply migrations
	err = applyMigrations(ctx, db)
	require.NoError(t, err, "Should apply migrations")

	// Create valid token for all endpoint tests
	token := generateTestToken("edge")
	tokenID := insertToken(ctx, t, db, token, "edge", false, nil)
	defer deleteToken(ctx, t, db, tokenID)

	// Convert sql.DB to gorm.DB
	gormDB, err := gorm.Open(postgres.New(postgres.Config{Conn: db}), &gorm.Config{})
	require.NoError(t, err)

	// Create server struct
	srv := server.Server{
		DB:     gormDB,
		Logger: hclog.NewNullLogger(),
	}

	t.Run("GetSyncStatus", func(t *testing.T) {
		handler := apiv2.EdgeSyncAuthMiddleware(srv, apiv2.EdgeSyncHandler(srv))

		req := httptest.NewRequest("GET", "/api/v2/edge/documents/sync-status?edge_instance=test-edge&limit=10", http.NoBody)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, "test-edge", response["edge_instance"])
		assert.NotNil(t, response["stats"])
	})

	t.Run("GetEdgeStats", func(t *testing.T) {
		handler := apiv2.EdgeSyncAuthMiddleware(srv, apiv2.EdgeSyncHandler(srv))

		req := httptest.NewRequest("GET", "/api/v2/edge/stats?edge_instance=test-edge", http.NoBody)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var stats map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &stats)
		require.NoError(t, err)

		assert.Contains(t, stats, "edge_instance")
		assert.Equal(t, "test-edge", stats["edge_instance"])
	})

	t.Run("RegisterDocument", func(t *testing.T) {
		handler := apiv2.EdgeSyncAuthMiddleware(srv, apiv2.EdgeSyncHandler(srv))

		// Create registration request
		regReq := map[string]interface{}{
			"uuid":          uuid.New().String(),
			"title":         "Test RFC-999: Integration Test",
			"document_type": "RFC",
			"status":        "Draft",
			"owners":        []string{"test@example.com"},
			"edge_instance": "test-edge",
			"provider_id":   "local:docs/test-rfc-999.md",
			"product":       "Engineering",
			"content_hash":  "sha256:test123",
			"created_at":    time.Now().Format(time.RFC3339),
			"updated_at":    time.Now().Format(time.RFC3339),
		}

		body, err := json.Marshal(regReq)
		require.NoError(t, err)

		req := httptest.NewRequest("POST", "/api/v2/edge/documents/register", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		// Should be authenticated (not 401/403)
		assert.NotEqual(t, http.StatusUnauthorized, w.Code, "Should be authenticated")
		assert.NotEqual(t, http.StatusForbidden, w.Code, "Should have permission")

		// Clean up if successful
		if w.Code == http.StatusOK || w.Code == http.StatusCreated {
			_, err := db.ExecContext(ctx, "DELETE FROM edge_document_registry WHERE uuid = $1", regReq["uuid"])
			assert.NoError(t, err, "Should clean up test document")
		}
	})
}

// TestTokenRevocationWorkflow tests the complete token lifecycle including revocation.
func TestTokenRevocationWorkflow(t *testing.T) {
	fixture := integration.GetFixture()
	ctx := context.Background()

	db, err := sql.Open("pgx", fixture.PostgresURL)
	require.NoError(t, err)
	defer db.Close()

	err = applyMigrations(ctx, db)
	require.NoError(t, err)

	// Convert sql.DB to gorm.DB
	gormDB, err := gorm.Open(postgres.New(postgres.Config{Conn: db}), &gorm.Config{})
	require.NoError(t, err)

	srv := server.Server{
		DB:     gormDB,
		Logger: hclog.NewNullLogger(),
	}

	// Create valid token
	token := generateTestToken("edge")
	tokenID := insertToken(ctx, t, db, token, "edge", false, nil)
	defer deleteToken(ctx, t, db, tokenID)

	handler := apiv2.EdgeSyncAuthMiddleware(srv, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))

	t.Run("TokenWorksBeforeRevocation", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v2/edge/documents/sync-status", http.NoBody)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "success", w.Body.String())
	})

	t.Run("RevokeToken", func(t *testing.T) {
		// Revoke the token
		_, err := db.ExecContext(ctx, `
			UPDATE service_tokens
			SET revoked = true, revoked_at = $1, revoked_reason = $2
			WHERE id = $3`,
			time.Now(), "Integration test revocation", tokenID)
		require.NoError(t, err, "Should revoke token")
	})

	t.Run("TokenFailsAfterRevocation", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v2/edge/documents/sync-status", http.NoBody)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "expired or been revoked")
	})
}

// Helper functions

func generateTestToken(tokenType string) string {
	return fmt.Sprintf("hermes-%s-token-%s-%s", tokenType, uuid.New().String(), "test1234abcd5678")
}

func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

func insertToken(ctx context.Context, t *testing.T, db *sql.DB, token, tokenType string, revoked bool, expiresAt *time.Time) uuid.UUID {
	t.Helper()

	tokenHash := hashToken(token)
	tokenID := uuid.New()

	query := `
		INSERT INTO service_tokens (id, created_at, updated_at, token_hash, token_type, revoked, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`

	_, err := db.ExecContext(ctx, query,
		tokenID, time.Now(), time.Now(), tokenHash, tokenType, revoked, expiresAt)
	require.NoError(t, err, "Should insert token")

	return tokenID
}

func deleteToken(ctx context.Context, t *testing.T, db *sql.DB, tokenID uuid.UUID) {
	t.Helper()

	_, err := db.ExecContext(ctx, "DELETE FROM service_tokens WHERE id = $1", tokenID)
	if err != nil {
		t.Logf("Warning: failed to delete test token: %v", err)
	}
}

func applyMigrations(ctx context.Context, db *sql.DB) error {
	// Create service_tokens table if it doesn't exist
	_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS service_tokens (
			id UUID PRIMARY KEY,
			created_at TIMESTAMP NOT NULL,
			updated_at TIMESTAMP NOT NULL,
			deleted_at TIMESTAMP,
			token_hash VARCHAR(256) NOT NULL UNIQUE,
			token_type VARCHAR(50) DEFAULT 'api',
			expires_at TIMESTAMP,
			revoked BOOLEAN DEFAULT FALSE,
			revoked_at TIMESTAMP,
			revoked_reason TEXT,
			indexer_id UUID,
			metadata TEXT
		)`)
	if err != nil {
		return fmt.Errorf("failed to create service_tokens table: %w", err)
	}

	// Create edge_document_registry table if it doesn't exist
	_, err = db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS edge_document_registry (
			uuid UUID PRIMARY KEY,
			title TEXT NOT NULL,
			document_type VARCHAR(50),
			status VARCHAR(50),
			summary TEXT,
			owners TEXT[],
			contributors TEXT[],
			edge_instance VARCHAR(255) NOT NULL,
			edge_provider_id TEXT,
			product VARCHAR(100),
			tags TEXT[],
			parent_folders TEXT[],
			metadata JSONB,
			content_hash VARCHAR(255),
			created_at TIMESTAMP NOT NULL,
			updated_at TIMESTAMP NOT NULL,
			synced_at TIMESTAMP,
			last_sync_status VARCHAR(50),
			sync_error TEXT
		)`)
	if err != nil {
		return fmt.Errorf("failed to create edge_document_registry table: %w", err)
	}

	return nil
}

// Note: We no longer need mockServer and mockLogger since we use server.Server struct directly
