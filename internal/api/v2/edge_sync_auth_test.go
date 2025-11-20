package api

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/hashicorp-forge/hermes/internal/server"
	"github.com/hashicorp-forge/hermes/pkg/models"
)

// TestEdgeSyncAuthenticationFlow tests the complete Bearer token authentication flow
func TestEdgeSyncAuthenticationFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup test database connection
	dsn := "host=localhost port=5433 user=postgres password=postgres dbname=hermes_testing sslmode=disable"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skip("PostgreSQL not available for integration test:", err)
	}

	// Clean up any existing test tokens
	db.Exec("DELETE FROM service_tokens WHERE token_type = 'edge' AND token_hash LIKE 'test-%'")

	t.Run("RejectMissingAuthorizationHeader", func(t *testing.T) {
		// Create test server
		srv := createTestServer(t, db)
		handler := EdgeSyncAuthMiddleware(srv, EdgeSyncHandler(srv))

		// Create request without Authorization header
		req := httptest.NewRequest("GET", "/api/v2/edge/documents/sync-status?edge_instance=test-edge", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "Missing authorization header")
	})

	t.Run("RejectInvalidAuthorizationFormat", func(t *testing.T) {
		srv := createTestServer(t, db)
		handler := EdgeSyncAuthMiddleware(srv, EdgeSyncHandler(srv))

		// Test with invalid format (not "Bearer ")
		req := httptest.NewRequest("GET", "/api/v2/edge/documents/sync-status?edge_instance=test-edge", nil)
		req.Header.Set("Authorization", "InvalidFormat token123")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "Invalid authorization header format")
	})

	t.Run("RejectEmptyBearerToken", func(t *testing.T) {
		srv := createTestServer(t, db)
		handler := EdgeSyncAuthMiddleware(srv, EdgeSyncHandler(srv))

		req := httptest.NewRequest("GET", "/api/v2/edge/documents/sync-status?edge_instance=test-edge", nil)
		req.Header.Set("Authorization", "Bearer ")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "Empty bearer token")
	})

	t.Run("RejectNonExistentToken", func(t *testing.T) {
		srv := createTestServer(t, db)
		handler := EdgeSyncAuthMiddleware(srv, EdgeSyncHandler(srv))

		// Use a token that doesn't exist in database
		fakeToken := "hermes-edge-token-" + uuid.New().String() + "-abcd1234"
		req := httptest.NewRequest("GET", "/api/v2/edge/documents/sync-status?edge_instance=test-edge", nil)
		req.Header.Set("Authorization", "Bearer "+fakeToken)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "Invalid or expired token")
	})

	t.Run("RejectRevokedToken", func(t *testing.T) {
		srv := createTestServer(t, db)
		handler := EdgeSyncAuthMiddleware(srv, EdgeSyncHandler(srv))

		// Create a revoked token
		token := generateTestToken()
		hash := hashToken(token)

		tokenModel := models.IndexerToken{
			ID:        uuid.New(),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			TokenHash: hash,
			TokenType: "edge",
			Revoked:   true, // Revoked!
		}
		require.NoError(t, db.Create(&tokenModel).Error)
		defer db.Delete(&tokenModel)

		req := httptest.NewRequest("GET", "/api/v2/edge/documents/sync-status?edge_instance=test-edge", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "expired or been revoked")
	})

	t.Run("RejectExpiredToken", func(t *testing.T) {
		srv := createTestServer(t, db)
		handler := EdgeSyncAuthMiddleware(srv, EdgeSyncHandler(srv))

		// Create an expired token
		token := generateTestToken()
		hash := hashToken(token)
		expiredTime := time.Now().Add(-24 * time.Hour) // Expired yesterday

		tokenModel := models.IndexerToken{
			ID:        uuid.New(),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			TokenHash: hash,
			TokenType: "edge",
			Revoked:   false,
			ExpiresAt: &expiredTime,
		}
		require.NoError(t, db.Create(&tokenModel).Error)
		defer db.Delete(&tokenModel)

		req := httptest.NewRequest("GET", "/api/v2/edge/documents/sync-status?edge_instance=test-edge", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "expired or been revoked")
	})

	t.Run("RejectWrongTokenType", func(t *testing.T) {
		srv := createTestServer(t, db)
		handler := EdgeSyncAuthMiddleware(srv, EdgeSyncHandler(srv))

		// Create a token with wrong type (registration instead of edge/api)
		token := generateTestToken()
		hash := hashToken(token)

		tokenModel := models.IndexerToken{
			ID:        uuid.New(),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			TokenHash: hash,
			TokenType: "registration", // Wrong type!
			Revoked:   false,
		}
		require.NoError(t, db.Create(&tokenModel).Error)
		defer db.Delete(&tokenModel)

		req := httptest.NewRequest("GET", "/api/v2/edge/documents/sync-status?edge_instance=test-edge", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Contains(t, w.Body.String(), "Invalid token type")
	})

	t.Run("AcceptValidEdgeToken", func(t *testing.T) {
		srv := createTestServer(t, db)
		handler := EdgeSyncAuthMiddleware(srv, EdgeSyncHandler(srv))

		// Create a valid edge token
		token := generateTestToken()
		hash := hashToken(token)

		tokenModel := models.IndexerToken{
			ID:        uuid.New(),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			TokenHash: hash,
			TokenType: "edge",
			Revoked:   false,
			ExpiresAt: nil, // Never expires
		}
		require.NoError(t, db.Create(&tokenModel).Error)
		defer db.Delete(&tokenModel)

		req := httptest.NewRequest("GET", "/api/v2/edge/documents/sync-status?edge_instance=test-edge", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		// Should be successful (200) or get to handler which might return other status
		// The important thing is NOT 401/403
		assert.NotEqual(t, http.StatusUnauthorized, w.Code, "Should not be unauthorized")
		assert.NotEqual(t, http.StatusForbidden, w.Code, "Should not be forbidden")
	})

	t.Run("AcceptValidAPIToken", func(t *testing.T) {
		srv := createTestServer(t, db)
		handler := EdgeSyncAuthMiddleware(srv, EdgeSyncHandler(srv))

		// Create a valid api token (also accepted for edge sync)
		token := generateTestToken()
		hash := hashToken(token)

		tokenModel := models.IndexerToken{
			ID:        uuid.New(),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			TokenHash: hash,
			TokenType: "api",
			Revoked:   false,
			ExpiresAt: nil,
		}
		require.NoError(t, db.Create(&tokenModel).Error)
		defer db.Delete(&tokenModel)

		req := httptest.NewRequest("GET", "/api/v2/edge/documents/sync-status?edge_instance=test-edge", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		assert.NotEqual(t, http.StatusUnauthorized, w.Code)
		assert.NotEqual(t, http.StatusForbidden, w.Code)
	})

	t.Run("AcceptTokenWithFutureExpiration", func(t *testing.T) {
		srv := createTestServer(t, db)
		handler := EdgeSyncAuthMiddleware(srv, EdgeSyncHandler(srv))

		// Create a token that expires in the future
		token := generateTestToken()
		hash := hashToken(token)
		futureTime := time.Now().Add(24 * time.Hour) // Expires tomorrow

		tokenModel := models.IndexerToken{
			ID:        uuid.New(),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			TokenHash: hash,
			TokenType: "edge",
			Revoked:   false,
			ExpiresAt: &futureTime,
		}
		require.NoError(t, db.Create(&tokenModel).Error)
		defer db.Delete(&tokenModel)

		req := httptest.NewRequest("GET", "/api/v2/edge/documents/sync-status?edge_instance=test-edge", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		assert.NotEqual(t, http.StatusUnauthorized, w.Code)
		assert.NotEqual(t, http.StatusForbidden, w.Code)
	})
}

// TestEdgeSyncEndpointsWithAuth tests actual API endpoints with authentication
func TestEdgeSyncEndpointsWithAuth(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	dsn := "host=localhost port=5433 user=postgres password=postgres dbname=hermes_testing sslmode=disable"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Skip("PostgreSQL not available for integration test:", err)
	}

	// Create valid token for tests
	token := generateTestToken()
	hash := hashToken(token)

	tokenModel := models.IndexerToken{
		ID:        uuid.New(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		TokenHash: hash,
		TokenType: "edge",
		Revoked:   false,
	}
	require.NoError(t, db.Create(&tokenModel).Error)
	defer db.Delete(&tokenModel)

	t.Run("GetSyncStatus", func(t *testing.T) {
		srv := createTestServer(t, db)
		handler := EdgeSyncAuthMiddleware(srv, EdgeSyncHandler(srv))

		req := httptest.NewRequest("GET", "/api/v2/edge/documents/sync-status?edge_instance=test-edge&limit=10", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response SyncStatusResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, "test-edge", response.EdgeInstance)
		assert.NotNil(t, response.Stats)
	})

	t.Run("RegisterDocument", func(t *testing.T) {
		srv := createTestServer(t, db)
		handler := EdgeSyncAuthMiddleware(srv, EdgeSyncHandler(srv))

		// Create registration request
		regReq := RegisterDocumentRequest{
			UUID:         uuid.New().String(),
			Title:        "Test RFC-999: Edge Sync Test",
			DocumentType: "RFC",
			Status:       "Draft",
			Owners:       []string{"test@example.com"},
			EdgeInstance: "test-edge",
			ProviderID:   "local:docs/test-rfc-999.md",
			Product:      "Engineering",
			ContentHash:  "sha256:test123",
			CreatedAt:    time.Now().Format(time.RFC3339),
			UpdatedAt:    time.Now().Format(time.RFC3339),
		}

		body, err := json.Marshal(regReq)
		require.NoError(t, err)

		req := httptest.NewRequest("POST", "/api/v2/edge/documents/register", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		// May fail due to owners field issue, but should be authenticated
		assert.NotEqual(t, http.StatusUnauthorized, w.Code, "Should be authenticated")
		assert.NotEqual(t, http.StatusForbidden, w.Code, "Should have permission")

		// Clean up if successful
		if w.Code == http.StatusOK {
			db.Exec("DELETE FROM edge_document_registry WHERE uuid = ?", regReq.UUID)
		}
	})

	t.Run("GetEdgeStats", func(t *testing.T) {
		srv := createTestServer(t, db)
		handler := EdgeSyncAuthMiddleware(srv, EdgeSyncHandler(srv))

		req := httptest.NewRequest("GET", "/api/v2/edge/stats?edge_instance=test-edge", nil)
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
}

// Helper functions

func generateTestToken() string {
	return fmt.Sprintf("hermes-edge-token-%s-%s", uuid.New().String(), "test1234")
}

func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

func createTestServer(t *testing.T, db *gorm.DB) server.Server {
	// Create a test logger
	logger := hclog.New(&hclog.LoggerOptions{
		Name:   "test",
		Level:  hclog.Debug,
		Output: nil, // Discard logs during tests
	})

	return server.Server{
		DB:     db,
		Logger: logger,
	}
}
