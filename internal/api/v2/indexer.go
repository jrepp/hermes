package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/hashicorp-forge/hermes/internal/server"
	"github.com/hashicorp-forge/hermes/pkg/models"
)

// IndexerRegisterRequest is the request body for indexer registration.
type IndexerRegisterRequest struct {
	Token         string                 `json:"token"`
	IndexerType   string                 `json:"indexer_type"`
	WorkspacePath string                 `json:"workspace_path,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// IndexerRegisterResponse is the response for successful registration.
type IndexerRegisterResponse struct {
	IndexerID uuid.UUID `json:"indexer_id"`
	APIToken  string    `json:"api_token"`
	ExpiresAt time.Time `json:"expires_at"`
	Config    struct {
		HeartbeatInterval string `json:"heartbeat_interval"`
		BatchSize         int    `json:"batch_size"`
	} `json:"config"`
}

// IndexerHeartbeatRequest is the request body for heartbeat updates.
type IndexerHeartbeatRequest struct {
	IndexerID     uuid.UUID              `json:"indexer_id"`
	Status        string                 `json:"status"`
	DocumentCount int                    `json:"document_count"`
	LastScanAt    *time.Time             `json:"last_scan_at,omitempty"`
	Metrics       map[string]interface{} `json:"metrics,omitempty"`
}

// IndexerHeartbeatResponse is the response for heartbeat.
type IndexerHeartbeatResponse struct {
	Acknowledged bool      `json:"acknowledged"`
	ServerTime   time.Time `json:"server_time"`
}

// IndexerHandler handles indexer-related API endpoints.
func IndexerHandler(srv server.Server) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Route based on method and path
		path := strings.TrimPrefix(r.URL.Path, "/api/v2/indexer")

		switch {
		case path == "/register" && r.Method == http.MethodPost:
			handleIndexerRegister(srv, w, r)
		case path == "/heartbeat" && r.Method == http.MethodPost:
			handleIndexerHeartbeat(srv, w, r)
		case path == "/documents" && r.Method == http.MethodPost:
			handleIndexerDocuments(srv, w, r)
		default:
			http.Error(w, "Not Found", http.StatusNotFound)
		}
	})
}

// handleIndexerRegister processes indexer registration requests.
func handleIndexerRegister(srv server.Server, w http.ResponseWriter, r *http.Request) {
	// Parse request body
	var req IndexerRegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		srv.Logger.Error("error decoding indexer registration request", "error", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.Token == "" {
		http.Error(w, "token is required", http.StatusBadRequest)
		return
	}
	if req.IndexerType == "" {
		http.Error(w, "indexer_type is required", http.StatusBadRequest)
		return
	}

	// Validate token (for now, just check it's not empty - in production, verify against stored registration tokens)
	// TODO: Implement proper token validation against service_tokens table
	tokenHash := models.HashToken(req.Token)
	var token models.IndexerToken
	if err := token.GetByHash(srv.DB, tokenHash); err != nil {
		srv.Logger.Warn("invalid registration token", "error", err)
		http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
		return
	}

	// Check if token is valid
	if !token.IsValid() {
		http.Error(w, "Token has expired or been revoked", http.StatusUnauthorized)
		return
	}

	// Create indexer record
	indexer := models.Indexer{
		IndexerType:   req.IndexerType,
		WorkspacePath: req.WorkspacePath,
		Status:        "active",
	}

	// Add metadata from request
	if req.Metadata != nil {
		metadataJSON, err := json.Marshal(req.Metadata)
		if err != nil {
			srv.Logger.Error("error marshaling metadata", "error", err)
		} else {
			indexer.Metadata = string(metadataJSON)
		}

		// Extract hostname and version if provided
		if hostname, ok := req.Metadata["hostname"].(string); ok {
			indexer.Hostname = hostname
		}
		if version, ok := req.Metadata["version"].(string); ok {
			indexer.Version = version
		}
	}

	if err := indexer.Create(srv.DB); err != nil {
		srv.Logger.Error("error creating indexer", "error", err)
		http.Error(w, "Failed to register indexer", http.StatusInternalServerError)
		return
	}

	// Generate API token for the indexer
	apiToken, err := models.GenerateToken("api")
	if err != nil {
		srv.Logger.Error("error generating API token", "error", err)
		http.Error(w, "Failed to generate API token", http.StatusInternalServerError)
		return
	}

	// Store API token
	expiresAt := time.Now().Add(24 * time.Hour * 365) // 1 year expiration
	indexerToken := models.IndexerToken{
		TokenType: "api",
		ExpiresAt: &expiresAt,
		IndexerID: &indexer.ID,
	}

	if err := indexerToken.Create(srv.DB, apiToken); err != nil {
		srv.Logger.Error("error creating API token", "error", err)
		http.Error(w, "Failed to store API token", http.StatusInternalServerError)
		return
	}

	// Mark registration token as used by associating it with the indexer
	token.IndexerID = &indexer.ID
	if err := srv.DB.Save(&token).Error; err != nil {
		srv.Logger.Warn("error updating registration token", "error", err)
	}

	// Build response
	resp := IndexerRegisterResponse{
		IndexerID: indexer.ID,
		APIToken:  apiToken,
		ExpiresAt: expiresAt,
	}
	resp.Config.HeartbeatInterval = "5m"
	resp.Config.BatchSize = 100

	srv.Logger.Info("indexer registered successfully",
		"indexer_id", indexer.ID,
		"indexer_type", indexer.IndexerType,
		"workspace_path", indexer.WorkspacePath)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		srv.Logger.Error("error encoding response", "error", err)
	}
}

// handleIndexerHeartbeat processes heartbeat updates from indexers.
func handleIndexerHeartbeat(srv server.Server, w http.ResponseWriter, r *http.Request) {
	// Extract and validate bearer token
	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		http.Error(w, "Missing or invalid authorization header", http.StatusUnauthorized)
		return
	}
	token := strings.TrimPrefix(authHeader, "Bearer ")

	// Validate token
	var indexerToken models.IndexerToken
	if err := indexerToken.GetByToken(srv.DB, token); err != nil {
		srv.Logger.Warn("invalid API token", "error", err)
		http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
		return
	}

	if !indexerToken.IsValid() {
		http.Error(w, "Token has expired or been revoked", http.StatusUnauthorized)
		return
	}

	if indexerToken.IndexerID == nil {
		http.Error(w, "Token not associated with an indexer", http.StatusBadRequest)
		return
	}

	// Parse request body
	var req IndexerHeartbeatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		srv.Logger.Error("error decoding heartbeat request", "error", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Verify indexer ID matches token
	if req.IndexerID != *indexerToken.IndexerID {
		http.Error(w, "Indexer ID mismatch", http.StatusForbidden)
		return
	}

	// Load indexer
	var indexer models.Indexer
	indexer.ID = req.IndexerID
	if err := indexer.Get(srv.DB); err != nil {
		srv.Logger.Error("error loading indexer", "error", err)
		http.Error(w, "Indexer not found", http.StatusNotFound)
		return
	}

	// Update heartbeat
	if err := indexer.UpdateHeartbeat(srv.DB, req.DocumentCount); err != nil {
		srv.Logger.Error("error updating heartbeat", "error", err)
		http.Error(w, "Failed to update heartbeat", http.StatusInternalServerError)
		return
	}

	// Update status if provided
	if req.Status != "" {
		indexer.Status = req.Status
		if err := indexer.Update(srv.DB); err != nil {
			srv.Logger.Warn("error updating indexer status", "error", err)
		}
	}

	srv.Logger.Debug("heartbeat received",
		"indexer_id", indexer.ID,
		"status", req.Status,
		"document_count", req.DocumentCount)

	// Build response
	resp := IndexerHeartbeatResponse{
		Acknowledged: true,
		ServerTime:   time.Now(),
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		srv.Logger.Error("error encoding response", "error", err)
	}
}

// handleIndexerDocuments processes document submissions from indexers.
func handleIndexerDocuments(srv server.Server, w http.ResponseWriter, r *http.Request) {
	// Extract and validate bearer token
	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		http.Error(w, "Missing or invalid authorization header", http.StatusUnauthorized)
		return
	}
	token := strings.TrimPrefix(authHeader, "Bearer ")

	// Validate token
	var indexerToken models.IndexerToken
	if err := indexerToken.GetByToken(srv.DB, token); err != nil {
		srv.Logger.Warn("invalid API token", "error", err)
		http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
		return
	}

	if !indexerToken.IsValid() {
		http.Error(w, "Token has expired or been revoked", http.StatusUnauthorized)
		return
	}

	if indexerToken.IndexerID == nil {
		http.Error(w, "Token not associated with an indexer", http.StatusBadRequest)
		return
	}

	// For now, just acknowledge receipt
	// TODO: Implement full document ingestion pipeline
	srv.Logger.Info("document submission received",
		"indexer_id", indexerToken.IndexerID)

	resp := map[string]interface{}{
		"accepted": 0,
		"rejected": 0,
		"message":  "Document ingestion not yet implemented",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		srv.Logger.Error("error encoding response", "error", err)
	}
}
