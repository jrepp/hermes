package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp-forge/hermes/internal/server"
	"github.com/hashicorp-forge/hermes/internal/services"
	"github.com/hashicorp-forge/hermes/pkg/docid"
	"github.com/hashicorp-forge/hermes/pkg/workspace"
)

// RegisterDocumentRequest represents a document registration request from edge
type RegisterDocumentRequest struct {
	UUID         string         `json:"uuid"`
	Title        string         `json:"title"`
	DocumentType string         `json:"document_type"`
	Status       string         `json:"status"`
	Owners       []string       `json:"owners"`
	EdgeInstance string         `json:"edge_instance"`
	ProviderID   string         `json:"provider_id"`
	Product      string         `json:"product"`
	Tags         []string       `json:"tags"`
	Parents      []string       `json:"parents"`
	Metadata     map[string]any `json:"metadata"`
	ContentHash  string         `json:"content_hash"`
	CreatedAt    string         `json:"created_at"`
	UpdatedAt    string         `json:"updated_at"`
}

// SyncMetadataRequest represents a metadata update request from edge
type SyncMetadataRequest struct {
	Title       string `json:"title,omitempty"`
	Status      string `json:"status,omitempty"`
	Summary     string `json:"summary,omitempty"`
	Product     string `json:"product,omitempty"`
	ContentHash string `json:"content_hash,omitempty"`
}

// SyncStatusResponse represents sync status for documents
type SyncStatusResponse struct {
	EdgeInstance string                         `json:"edge_instance"`
	Documents    []*services.EdgeDocumentRecord `json:"documents"`
	Stats        map[string]any                 `json:"stats,omitempty"`
}

// EdgeSyncHandler handles edge-to-central document synchronization endpoints
//
// POST   /api/v2/edge/documents/register          - Register document from edge
// PUT    /api/v2/edge/documents/:uuid/sync        - Sync metadata updates
// GET    /api/v2/edge/documents/sync-status       - Get sync status
// GET    /api/v2/edge/documents/:uuid             - Get document by UUID
// GET    /api/v2/edge/documents/search            - Search documents
// DELETE /api/v2/edge/documents/:uuid             - Delete document
// GET    /api/v2/edge/stats                       - Get edge instance stats
func EdgeSyncHandler(srv server.Server) http.Handler {
	syncService := services.NewDocumentSyncService(srv.DB)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Parse the path to determine which endpoint was called
		path := strings.TrimPrefix(r.URL.Path, "/api/v2/edge/")

		switch {
		case r.Method == "POST" && path == "documents/register":
			handleRegisterDocument(w, r, syncService, srv)

		case r.Method == "GET" && path == "documents/sync-status":
			handleGetSyncStatus(w, r, syncService, srv)

		case r.Method == "GET" && path == "documents/search":
			handleSearchDocuments(w, r, syncService, srv)

		case r.Method == "GET" && path == "stats":
			handleGetEdgeInstanceStats(w, r, syncService, srv)

		case strings.HasPrefix(path, "documents/"):
			// Extract UUID from path
			parts := strings.Split(path, "/")
			if len(parts) < 2 {
				http.Error(w, "Invalid path", http.StatusBadRequest)
				return
			}
			uuid := parts[1]

			// Check for /sync suffix
			if len(parts) == 3 && parts[2] == "sync" {
				if r.Method == "PUT" {
					handleSyncMetadata(w, r, uuid, syncService, srv)
					return
				}
			} else if len(parts) == 2 {
				// No suffix - document CRUD operations
				switch r.Method {
				case "GET":
					handleGetDocumentByUUID(w, r, uuid, syncService, srv)
				case "DELETE":
					handleDeleteDocument(w, r, uuid, syncService, srv)
				default:
					http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				}
				return
			}

			http.Error(w, "Not found", http.StatusNotFound)

		default:
			http.Error(w, "Not found", http.StatusNotFound)
		}
	})
}

// handleRegisterDocument registers a document from edge instance
func handleRegisterDocument(w http.ResponseWriter, r *http.Request, syncService *services.DocumentSyncService, srv server.Server) {
	var req RegisterDocumentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		srv.Logger.Error("failed to decode register request", "error", err)
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.UUID == "" {
		http.Error(w, "uuid is required", http.StatusBadRequest)
		return
	}
	if req.Title == "" {
		http.Error(w, "title is required", http.StatusBadRequest)
		return
	}
	if req.EdgeInstance == "" {
		http.Error(w, "edge_instance is required", http.StatusBadRequest)
		return
	}

	// Parse UUID
	uuid, err := docid.ParseUUID(req.UUID)
	if err != nil {
		srv.Logger.Error("invalid uuid format", "error", err, "uuid", req.UUID)
		http.Error(w, "invalid uuid format", http.StatusBadRequest)
		return
	}

	// Parse timestamps
	createdAt, err := parseTimestamp(req.CreatedAt)
	if err != nil {
		srv.Logger.Error("invalid created_at timestamp", "error", err)
		http.Error(w, "invalid created_at timestamp", http.StatusBadRequest)
		return
	}

	updatedAt, err := parseTimestamp(req.UpdatedAt)
	if err != nil {
		srv.Logger.Error("invalid updated_at timestamp", "error", err)
		http.Error(w, "invalid updated_at timestamp", http.StatusBadRequest)
		return
	}

	// Convert to workspace.DocumentMetadata
	doc := &workspace.DocumentMetadata{
		UUID:             uuid,
		Name:             req.Title,
		ProviderType:     req.DocumentType,
		ProviderID:       req.ProviderID,
		Project:          req.Product,
		Tags:             req.Tags,
		Parents:          req.Parents,
		WorkflowStatus:   req.Status,
		ContentHash:      req.ContentHash,
		ExtendedMetadata: req.Metadata,
		CreatedTime:      createdAt,
		ModifiedTime:     updatedAt,
	}

	// Register document
	record, err := syncService.RegisterDocument(r.Context(), doc, req.EdgeInstance)
	if err != nil {
		srv.Logger.Error("failed to register document", "error", err, "uuid", uuid)
		http.Error(w, "failed to register document: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(record)
}

// handleSyncMetadata updates document metadata from edge
func handleSyncMetadata(w http.ResponseWriter, r *http.Request, uuidStr string, syncService *services.DocumentSyncService, srv server.Server) {
	uuid, err := docid.ParseUUID(uuidStr)
	if err != nil {
		srv.Logger.Error("invalid uuid format", "error", err, "uuid", uuidStr)
		http.Error(w, "invalid uuid format", http.StatusBadRequest)
		return
	}

	var req SyncMetadataRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		srv.Logger.Error("failed to decode sync metadata request", "error", err)
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Build updates map
	updates := make(map[string]any)
	if req.Title != "" {
		updates["title"] = req.Title
	}
	if req.Status != "" {
		updates["status"] = req.Status
	}
	if req.Summary != "" {
		updates["summary"] = req.Summary
	}
	if req.Product != "" {
		updates["product"] = req.Product
	}
	if req.ContentHash != "" {
		updates["content_hash"] = req.ContentHash
	}

	// Update metadata
	record, err := syncService.UpdateDocumentMetadata(r.Context(), uuid, updates)
	if err != nil {
		srv.Logger.Error("failed to update metadata", "error", err, "uuid", uuid)
		http.Error(w, "failed to update metadata: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(record)
}

// handleGetSyncStatus returns synchronization status for edge instance
func handleGetSyncStatus(w http.ResponseWriter, r *http.Request, syncService *services.DocumentSyncService, srv server.Server) {
	edgeInstance := r.URL.Query().Get("edge_instance")
	if edgeInstance == "" {
		http.Error(w, "edge_instance query parameter is required", http.StatusBadRequest)
		return
	}

	limit := parseIntQueryParam(r, "limit", 100)

	// Get documents
	documents, err := syncService.GetSyncStatus(r.Context(), edgeInstance, limit)
	if err != nil {
		srv.Logger.Error("failed to get sync status", "error", err, "edge_instance", edgeInstance)
		http.Error(w, "failed to get sync status: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Get stats
	stats, err := syncService.GetEdgeInstanceStats(r.Context(), edgeInstance)
	if err != nil {
		// Log error but don't fail the request
		srv.Logger.Warn("failed to get stats", "error", err, "edge_instance", edgeInstance)
		stats = nil
	}

	response := &SyncStatusResponse{
		EdgeInstance: edgeInstance,
		Documents:    documents,
		Stats:        stats,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleGetDocumentByUUID retrieves a synced document by UUID
func handleGetDocumentByUUID(w http.ResponseWriter, r *http.Request, uuidStr string, syncService *services.DocumentSyncService, srv server.Server) {
	uuid, err := docid.ParseUUID(uuidStr)
	if err != nil {
		srv.Logger.Error("invalid uuid format", "error", err, "uuid", uuidStr)
		http.Error(w, "invalid uuid format", http.StatusBadRequest)
		return
	}

	record, err := syncService.GetDocumentByUUID(r.Context(), uuid)
	if err != nil {
		srv.Logger.Error("document not found", "error", err, "uuid", uuid)
		http.Error(w, "document not found: "+err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(record)
}

// handleSearchDocuments searches edge documents
func handleSearchDocuments(w http.ResponseWriter, r *http.Request, syncService *services.DocumentSyncService, srv server.Server) {
	query := r.URL.Query().Get("q")
	limit := parseIntQueryParam(r, "limit", 50)

	filters := map[string]any{}
	if docType := r.URL.Query().Get("document_type"); docType != "" {
		filters["document_type"] = docType
	}
	if status := r.URL.Query().Get("status"); status != "" {
		filters["status"] = status
	}
	if product := r.URL.Query().Get("product"); product != "" {
		filters["product"] = product
	}
	if edgeInstance := r.URL.Query().Get("edge_instance"); edgeInstance != "" {
		filters["edge_instance"] = edgeInstance
	}

	documents, err := syncService.SearchDocuments(r.Context(), query, filters, limit)
	if err != nil {
		srv.Logger.Error("search failed", "error", err, "query", query)
		http.Error(w, "search failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]any{
		"documents": documents,
		"count":     len(documents),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleDeleteDocument deletes a synced document
func handleDeleteDocument(w http.ResponseWriter, r *http.Request, uuidStr string, syncService *services.DocumentSyncService, srv server.Server) {
	uuid, err := docid.ParseUUID(uuidStr)
	if err != nil {
		srv.Logger.Error("invalid uuid format", "error", err, "uuid", uuidStr)
		http.Error(w, "invalid uuid format", http.StatusBadRequest)
		return
	}

	if err := syncService.DeleteDocument(r.Context(), uuid); err != nil {
		srv.Logger.Error("failed to delete document", "error", err, "uuid", uuid)
		http.Error(w, "failed to delete document: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleGetEdgeInstanceStats returns statistics for an edge instance
func handleGetEdgeInstanceStats(w http.ResponseWriter, r *http.Request, syncService *services.DocumentSyncService, srv server.Server) {
	edgeInstance := r.URL.Query().Get("edge_instance")
	if edgeInstance == "" {
		http.Error(w, "edge_instance query parameter is required", http.StatusBadRequest)
		return
	}

	stats, err := syncService.GetEdgeInstanceStats(r.Context(), edgeInstance)
	if err != nil {
		srv.Logger.Error("failed to get stats", "error", err, "edge_instance", edgeInstance)
		http.Error(w, "failed to get stats: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// parseTimestamp parses a timestamp string in RFC3339 format
func parseTimestamp(ts string) (time.Time, error) {
	if ts == "" {
		return time.Now(), nil
	}
	return time.Parse(time.RFC3339, ts)
}

// parseIntQueryParam parses an integer query parameter with a default value
func parseIntQueryParam(r *http.Request, param string, defaultValue int) int {
	valueStr := r.URL.Query().Get(param)
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}
