package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp-forge/hermes/internal/server"
)

// ProvidersHandler handles storage provider management endpoints
// Routes:
//
//	GET    /api/v2/providers              - List all providers
//	POST   /api/v2/providers              - Register new provider
//	GET    /api/v2/providers/:id          - Get provider details
//	PATCH  /api/v2/providers/:id          - Update provider
//	DELETE /api/v2/providers/:id          - Remove provider
//	GET    /api/v2/providers/:id/health   - Get provider health status
func ProvidersHandler(srv server.Server) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract path after /api/v2/providers/
		path := strings.TrimPrefix(r.URL.Path, "/api/v2/providers/")
		path = strings.TrimPrefix(path, "/api/v2/providers")
		path = strings.TrimPrefix(path, "/")

		if path == "" {
			// /api/v2/providers
			if r.Method == http.MethodGet {
				listProviders(w, r, srv)
			} else if r.Method == http.MethodPost {
				registerProvider(w, r, srv)
			} else {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			}
			return
		}

		parts := strings.Split(path, "/")
		if len(parts) == 0 || parts[0] == "" {
			http.Error(w, "Provider ID required", http.StatusBadRequest)
			return
		}

		providerID, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			http.Error(w, "Invalid provider ID", http.StatusBadRequest)
			return
		}

		if len(parts) == 1 {
			// /providers/:id
			switch r.Method {
			case http.MethodGet:
				getProvider(w, r, srv, providerID)
			case http.MethodPatch:
				updateProvider(w, r, srv, providerID)
			case http.MethodDelete:
				removeProvider(w, r, srv, providerID)
			default:
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			}
		} else if len(parts) == 2 && parts[1] == "health" {
			// /providers/:id/health
			if r.Method == http.MethodGet {
				getProviderHealth(w, r, srv, providerID)
			} else {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			}
		} else {
			http.Error(w, "Invalid path", http.StatusBadRequest)
		}
	})
}

// RegisterProviderRequest represents a request to register a new provider
type RegisterProviderRequest struct {
	ProviderName string                 `json:"providerName"`
	ProviderType string                 `json:"providerType"` // "local", "s3", "google", "azure"
	Config       map[string]interface{} `json:"config"`
	Capabilities map[string]interface{} `json:"capabilities"`
	IsPrimary    bool                   `json:"isPrimary"`
	IsWritable   bool                   `json:"isWritable"`
	Status       string                 `json:"status"` // "active", "readonly", "disabled"
}

// UpdateProviderRequest represents a request to update a provider
type UpdateProviderRequest struct {
	Config       map[string]interface{} `json:"config,omitempty"`
	Capabilities map[string]interface{} `json:"capabilities,omitempty"`
	IsPrimary    *bool                  `json:"isPrimary,omitempty"`
	IsWritable   *bool                  `json:"isWritable,omitempty"`
	Status       string                 `json:"status,omitempty"`
}

// listProviders lists all registered storage providers
func listProviders(w http.ResponseWriter, r *http.Request, srv server.Server) {
	// Parse query parameters
	providerType := r.URL.Query().Get("type")
	status := r.URL.Query().Get("status")

	query := `
		SELECT id, provider_name, provider_type, config, capabilities,
			   status, is_primary, is_writable, document_count, total_size_bytes,
			   health_status, last_health_check, created_at, updated_at
		FROM provider_storage
		WHERE 1=1
	`
	args := []interface{}{}

	if providerType != "" {
		query += " AND provider_type = $" + strconv.Itoa(len(args)+1)
		args = append(args, providerType)
	}

	if status != "" {
		query += " AND status = $" + strconv.Itoa(len(args)+1)
		args = append(args, status)
	}

	query += " ORDER BY is_primary DESC, provider_name ASC"

	sqlDB, err := getSQLDB(srv)
	if err != nil {
		srv.Logger.Error("failed to get SQL DB", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	rows, err := sqlDB.QueryContext(r.Context(), query, args...)
	if err != nil {
		srv.Logger.Error("failed to query providers", "error", err)
		http.Error(w, "Failed to list providers", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var providers []map[string]interface{}
	for rows.Next() {
		var (
			id, documentCount, totalSizeBytes  int64
			providerName, providerType, status string
			config, capabilities               string
			isPrimary, isWritable              bool
			healthStatus                       sql.NullString
			lastHealthCheck                    sql.NullTime
			createdAt, updatedAt               time.Time
		)

		if err := rows.Scan(&id, &providerName, &providerType, &config, &capabilities,
			&status, &isPrimary, &isWritable, &documentCount, &totalSizeBytes,
			&healthStatus, &lastHealthCheck, &createdAt, &updatedAt); err != nil {
			srv.Logger.Error("failed to scan provider", "error", err)
			continue
		}

		provider := map[string]interface{}{
			"id":             id,
			"providerName":   providerName,
			"providerType":   providerType,
			"status":         status,
			"isPrimary":      isPrimary,
			"isWritable":     isWritable,
			"documentCount":  documentCount,
			"totalSizeBytes": totalSizeBytes,
			"createdAt":      createdAt,
			"updatedAt":      updatedAt,
		}

		// Parse JSON fields
		var configMap map[string]interface{}
		if err := json.Unmarshal([]byte(config), &configMap); err == nil {
			// Remove sensitive fields
			delete(configMap, "access_key")
			delete(configMap, "secret_key")
			delete(configMap, "credentials")
			provider["config"] = configMap
		}

		var capabilitiesMap map[string]interface{}
		if err := json.Unmarshal([]byte(capabilities), &capabilitiesMap); err == nil {
			provider["capabilities"] = capabilitiesMap
		}

		if healthStatus.Valid {
			provider["healthStatus"] = healthStatus.String
		}
		if lastHealthCheck.Valid {
			provider["lastHealthCheck"] = lastHealthCheck.Time
		}

		providers = append(providers, provider)
	}

	response := map[string]interface{}{
		"providers": providers,
		"count":     len(providers),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// registerProvider registers a new storage provider
func registerProvider(w http.ResponseWriter, r *http.Request, srv server.Server) {
	var req RegisterProviderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		srv.Logger.Error("failed to decode request", "error", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.ProviderName == "" {
		http.Error(w, "providerName is required", http.StatusBadRequest)
		return
	}
	if req.ProviderType == "" {
		http.Error(w, "providerType is required", http.StatusBadRequest)
		return
	}

	// Default status
	if req.Status == "" {
		req.Status = "active"
	}

	// Serialize JSON fields
	configJSON, err := json.Marshal(req.Config)
	if err != nil {
		srv.Logger.Error("failed to marshal config", "error", err)
		http.Error(w, "Invalid config", http.StatusBadRequest)
		return
	}

	capabilitiesJSON, err := json.Marshal(req.Capabilities)
	if err != nil {
		srv.Logger.Error("failed to marshal capabilities", "error", err)
		http.Error(w, "Invalid capabilities", http.StatusBadRequest)
		return
	}

	// Get underlying SQL DB
	sqlDB, err := getSQLDB(srv)
	if err != nil {
		srv.Logger.Error("failed to get SQL DB", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Insert provider
	var providerID int64
	err = sqlDB.QueryRowContext(r.Context(), `
		INSERT INTO provider_storage (
			provider_name, provider_type, config, capabilities,
			status, is_primary, is_writable
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id
	`, req.ProviderName, req.ProviderType, configJSON, capabilitiesJSON,
		req.Status, req.IsPrimary, req.IsWritable).Scan(&providerID)

	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			http.Error(w, "Provider name already exists", http.StatusConflict)
		} else {
			srv.Logger.Error("failed to insert provider", "error", err)
			http.Error(w, "Failed to register provider", http.StatusInternalServerError)
		}
		return
	}

	srv.Logger.Info("provider registered",
		"id", providerID,
		"name", req.ProviderName,
		"type", req.ProviderType)

	// Return the created provider
	getProvider(w, r, srv, providerID)
}

// getProvider gets a specific provider
func getProvider(w http.ResponseWriter, r *http.Request, srv server.Server, providerID int64) {
	sqlDB, err := getSQLDB(srv)
	if err != nil {
		srv.Logger.Error("failed to get SQL DB", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	var (
		id, documentCount, totalSizeBytes  int64
		providerName, providerType, status string
		config, capabilities               string
		isPrimary, isWritable              bool
		healthStatus                       sql.NullString
		lastHealthCheck                    sql.NullTime
		createdAt, updatedAt               time.Time
	)

	err = sqlDB.QueryRowContext(r.Context(), `
		SELECT id, provider_name, provider_type, config, capabilities,
			   status, is_primary, is_writable, document_count, total_size_bytes,
			   health_status, last_health_check, created_at, updated_at
		FROM provider_storage
		WHERE id = $1
	`, providerID).Scan(&id, &providerName, &providerType, &config, &capabilities,
		&status, &isPrimary, &isWritable, &documentCount, &totalSizeBytes,
		&healthStatus, &lastHealthCheck, &createdAt, &updatedAt)

	if err == sql.ErrNoRows {
		http.Error(w, "Provider not found", http.StatusNotFound)
		return
	} else if err != nil {
		srv.Logger.Error("failed to query provider", "error", err)
		http.Error(w, "Failed to get provider", http.StatusInternalServerError)
		return
	}

	provider := map[string]interface{}{
		"id":             id,
		"providerName":   providerName,
		"providerType":   providerType,
		"status":         status,
		"isPrimary":      isPrimary,
		"isWritable":     isWritable,
		"documentCount":  documentCount,
		"totalSizeBytes": totalSizeBytes,
		"createdAt":      createdAt,
		"updatedAt":      updatedAt,
	}

	// Parse JSON fields
	var configMap map[string]interface{}
	if err := json.Unmarshal([]byte(config), &configMap); err == nil {
		// Remove sensitive fields
		delete(configMap, "access_key")
		delete(configMap, "secret_key")
		delete(configMap, "credentials")
		provider["config"] = configMap
	}

	var capabilitiesMap map[string]interface{}
	if err := json.Unmarshal([]byte(capabilities), &capabilitiesMap); err == nil {
		provider["capabilities"] = capabilitiesMap
	}

	if healthStatus.Valid {
		provider["healthStatus"] = healthStatus.String
	}
	if lastHealthCheck.Valid {
		provider["lastHealthCheck"] = lastHealthCheck.Time
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(provider)
}

// updateProvider updates a provider
func updateProvider(w http.ResponseWriter, r *http.Request, srv server.Server, providerID int64) {
	var req UpdateProviderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		srv.Logger.Error("failed to decode request", "error", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Build dynamic update query
	updates := []string{}
	args := []interface{}{}
	argNum := 1

	if req.Config != nil {
		configJSON, err := json.Marshal(req.Config)
		if err != nil {
			http.Error(w, "Invalid config", http.StatusBadRequest)
			return
		}
		updates = append(updates, "config = $"+strconv.Itoa(argNum))
		args = append(args, configJSON)
		argNum++
	}

	if req.Capabilities != nil {
		capabilitiesJSON, err := json.Marshal(req.Capabilities)
		if err != nil {
			http.Error(w, "Invalid capabilities", http.StatusBadRequest)
			return
		}
		updates = append(updates, "capabilities = $"+strconv.Itoa(argNum))
		args = append(args, capabilitiesJSON)
		argNum++
	}

	if req.IsPrimary != nil {
		updates = append(updates, "is_primary = $"+strconv.Itoa(argNum))
		args = append(args, *req.IsPrimary)
		argNum++
	}

	if req.IsWritable != nil {
		updates = append(updates, "is_writable = $"+strconv.Itoa(argNum))
		args = append(args, *req.IsWritable)
		argNum++
	}

	if req.Status != "" {
		updates = append(updates, "status = $"+strconv.Itoa(argNum))
		args = append(args, req.Status)
		argNum++
	}

	if len(updates) == 0 {
		http.Error(w, "No fields to update", http.StatusBadRequest)
		return
	}

	// Always update updated_at
	updates = append(updates, "updated_at = NOW()")

	// Add provider ID to args
	args = append(args, providerID)

	query := "UPDATE provider_storage SET " + strings.Join(updates, ", ") +
		" WHERE id = $" + strconv.Itoa(argNum)

	sqlDB, err := getSQLDB(srv)
	if err != nil {
		srv.Logger.Error("failed to get SQL DB", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	result, err := sqlDB.ExecContext(r.Context(), query, args...)
	if err != nil {
		srv.Logger.Error("failed to update provider", "error", err)
		http.Error(w, "Failed to update provider", http.StatusInternalServerError)
		return
	}

	rows, err := result.RowsAffected()
	if err != nil {
		http.Error(w, "Failed to check result", http.StatusInternalServerError)
		return
	}
	if rows == 0 {
		http.Error(w, "Provider not found", http.StatusNotFound)
		return
	}

	srv.Logger.Info("provider updated", "id", providerID)

	// Return updated provider
	getProvider(w, r, srv, providerID)
}

// removeProvider removes a provider
func removeProvider(w http.ResponseWriter, r *http.Request, srv server.Server, providerID int64) {
	sqlDB, err := getSQLDB(srv)
	if err != nil {
		srv.Logger.Error("failed to get SQL DB", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Check if provider has any migration jobs
	var jobCount int
	err = sqlDB.QueryRowContext(r.Context(), `
		SELECT COUNT(*)
		FROM migration_jobs
		WHERE source_provider_id = $1 OR dest_provider_id = $1
	`, providerID).Scan(&jobCount)

	if err != nil {
		srv.Logger.Error("failed to check migration jobs", "error", err)
		http.Error(w, "Failed to remove provider", http.StatusInternalServerError)
		return
	}

	if jobCount > 0 {
		http.Error(w, "Cannot remove provider with existing migration jobs", http.StatusConflict)
		return
	}

	// Delete provider
	result, err := sqlDB.ExecContext(r.Context(), `
		DELETE FROM provider_storage WHERE id = $1
	`, providerID)

	if err != nil {
		srv.Logger.Error("failed to delete provider", "error", err)
		http.Error(w, "Failed to remove provider", http.StatusInternalServerError)
		return
	}

	rows, err := result.RowsAffected()
	if err != nil {
		http.Error(w, "Failed to check result", http.StatusInternalServerError)
		return
	}
	if rows == 0 {
		http.Error(w, "Provider not found", http.StatusNotFound)
		return
	}

	srv.Logger.Info("provider removed", "id", providerID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":    "Provider removed successfully",
		"providerId": providerID,
	})
}

// getProviderHealth gets health status for a provider
func getProviderHealth(w http.ResponseWriter, r *http.Request, srv server.Server, providerID int64) {
	sqlDB, err := getSQLDB(srv)
	if err != nil {
		srv.Logger.Error("failed to get SQL DB", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	var (
		providerName, healthStatus    string
		lastHealthCheck               sql.NullTime
		documentCount, totalSizeBytes int64
	)

	err = sqlDB.QueryRowContext(r.Context(), `
		SELECT provider_name, health_status, last_health_check, document_count, total_size_bytes
		FROM provider_storage
		WHERE id = $1
	`, providerID).Scan(&providerName, &healthStatus, &lastHealthCheck, &documentCount, &totalSizeBytes)

	if err == sql.ErrNoRows {
		http.Error(w, "Provider not found", http.StatusNotFound)
		return
	} else if err != nil {
		srv.Logger.Error("failed to query provider health", "error", err)
		http.Error(w, "Failed to get health", http.StatusInternalServerError)
		return
	}

	health := map[string]interface{}{
		"providerId":     providerID,
		"providerName":   providerName,
		"healthStatus":   healthStatus,
		"documentCount":  documentCount,
		"totalSizeBytes": totalSizeBytes,
	}

	if lastHealthCheck.Valid {
		health["lastHealthCheck"] = lastHealthCheck.Time
		health["lastHealthCheckAge"] = time.Since(lastHealthCheck.Time).String()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}
