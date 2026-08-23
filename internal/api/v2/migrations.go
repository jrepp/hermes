package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/hashicorp-forge/hermes/internal/server"
	"github.com/hashicorp-forge/hermes/pkg/docid"
	"github.com/hashicorp-forge/hermes/pkg/migration"
)

// getSQLDB gets the underlying *sql.DB from the GORM DB
func getSQLDB(srv server.Server) (*sql.DB, error) {
	return srv.DB.DB()
}

// MigrationsHandler handles migration job management endpoints
// Routes:
//
//	GET    /api/v2/migrations/jobs                - List migration jobs
//	POST   /api/v2/migrations/jobs                - Create new migration job
//	GET    /api/v2/migrations/jobs/:id            - Get job details
//	POST   /api/v2/migrations/jobs/:id/start      - Start a job
//	POST   /api/v2/migrations/jobs/:id/pause      - Pause a job
//	POST   /api/v2/migrations/jobs/:id/cancel     - Cancel a job
//	GET    /api/v2/migrations/jobs/:id/progress   - Get job progress
//	GET    /api/v2/migrations/jobs/:id/items      - List migration items
//
//nolint:gocognit,gocyclo // Route-style handler intentionally keeps path dispatch in one place.
func MigrationsHandler(srv server.Server) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Bind to the site serving this request before touching the
		// database; srv as constructed holds the process-wide default.
		srv, ok := srv.ForRequest(w, r)
		if !ok {
			return
		}

		// Extract path after /api/v2/migrations/
		path := strings.TrimPrefix(r.URL.Path, "/api/v2/migrations/")

		switch {
		case path == "jobs" || path == "jobs/":
			switch r.Method {
			case http.MethodGet:
				listMigrationJobs(w, r, srv)
			case http.MethodPost:
				createMigrationJob(w, r, srv)
			default:
				WriteMethodNotAllowed(w, r, http.MethodGet, http.MethodPost)
			}

		case strings.HasPrefix(path, "jobs/"):
			jobPath := strings.TrimPrefix(path, "jobs/")
			parts := strings.Split(jobPath, "/")

			if len(parts) == 0 || parts[0] == "" {
				http.Error(w, "Job ID required", http.StatusBadRequest)
				return
			}

			jobID, err := strconv.ParseInt(parts[0], 10, 64)
			if err != nil {
				http.Error(w, "Invalid job ID", http.StatusBadRequest)
				return
			}

			switch len(parts) {
			case 1:
				// /jobs/:id
				switch r.Method {
				case http.MethodGet:
					getMigrationJob(w, r, srv, jobID)
				case http.MethodDelete:
					cancelMigrationJob(w, r, srv, jobID)
				default:
					WriteMethodNotAllowed(w, r, http.MethodGet, http.MethodDelete)
				}
			case 2:
				// /jobs/:id/:action
				action := parts[1]
				switch action {
				case "start":
					if r.Method == http.MethodPost {
						startMigrationJob(w, r, srv, jobID)
					} else {
						WriteMethodNotAllowed(w, r, http.MethodPost)
					}
				case "pause":
					if r.Method == http.MethodPost {
						pauseMigrationJob(w, r, srv, jobID)
					} else {
						WriteMethodNotAllowed(w, r, http.MethodPost)
					}
				case "cancel":
					if r.Method == http.MethodPost {
						cancelMigrationJob(w, r, srv, jobID)
					} else {
						WriteMethodNotAllowed(w, r, http.MethodPost)
					}
				case "progress":
					if r.Method == http.MethodGet {
						getMigrationProgress(w, r, srv, jobID)
					} else {
						WriteMethodNotAllowed(w, r, http.MethodGet)
					}
				case "items":
					if r.Method == http.MethodGet {
						listMigrationItems(w, r, srv, jobID)
					} else {
						WriteMethodNotAllowed(w, r, http.MethodGet)
					}
				default:
					WriteError(w, r, http.StatusNotFound, "unknown action; use start, pause, cancel, progress, or items")
				}
			default:
				http.Error(w, "Invalid path", http.StatusBadRequest)
			}

		default:
			http.Error(w, "Not found", http.StatusNotFound)
		}
	})
}

// CreateMigrationJobRequest represents a request to create a new migration job
type CreateMigrationJobRequest struct {
	FilterCriteria map[string]any `json:"filterCriteria"`
	JobName        string         `json:"jobName"`
	SourceProvider string         `json:"sourceProvider"`
	DestProvider   string         `json:"destProvider"`
	Strategy       string         `json:"strategy"`
	DocumentUUIDs  []string       `json:"documentUuids"`
	Concurrency    int            `json:"concurrency"`
	BatchSize      int            `json:"batchSize"`
	DryRun         bool           `json:"dryRun"`
	Validate       bool           `json:"validate"`
}

// createMigrationJob creates a new migration job
//
//nolint:gocognit,gocyclo // Validation and job construction are kept inline to preserve behavior.
func createMigrationJob(w http.ResponseWriter, r *http.Request, srv server.Server) {
	var req CreateMigrationJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		srv.Logger.Error("failed to decode request", "error", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.JobName == "" {
		http.Error(w, "jobName is required", http.StatusBadRequest)
		return
	}
	if req.SourceProvider == "" {
		http.Error(w, "sourceProvider is required", http.StatusBadRequest)
		return
	}
	if req.DestProvider == "" {
		http.Error(w, "destProvider is required", http.StatusBadRequest)
		return
	}

	// Get user email from context (set by auth middleware)
	userEmail := r.Context().Value("user_email")
	var createdBy string
	if userEmail == nil {
		createdBy = "system"
	} else {
		emailStr, ok := userEmail.(string)
		if !ok {
			srv.Logger.Error("user_email context value is not a string", "type", fmt.Sprintf("%T", userEmail))
			createdBy = "system"
		} else {
			createdBy = emailStr
		}
	}

	// Get underlying SQL DB
	sqlDB, err := getSQLDB(srv)
	if err != nil {
		srv.Logger.Error("failed to get SQL DB", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Create migration manager
	manager := migration.NewManager(sqlDB, srv.Logger)

	// Create job
	jobReq := &migration.CreateJobRequest{
		JobName:        req.JobName,
		SourceProvider: req.SourceProvider,
		DestProvider:   req.DestProvider,
		Strategy:       migration.Strategy(req.Strategy),
		FilterCriteria: req.FilterCriteria,
		Concurrency:    req.Concurrency,
		BatchSize:      req.BatchSize,
		DryRun:         req.DryRun,
		Validate:       req.Validate,
		CreatedBy:      createdBy,
	}

	job, err := manager.CreateJob(r.Context(), jobReq)
	if err != nil {
		srv.Logger.Error("failed to create migration job", "error", err)
		http.Error(w, "Failed to create migration job", http.StatusInternalServerError)
		return
	}

	// If specific documents provided, queue them
	if len(req.DocumentUUIDs) > 0 {
		var uuids []docid.UUID
		var providerIDs []string
		for _, uuidStr := range req.DocumentUUIDs {
			uuid, err := docid.ParseUUID(uuidStr)
			if err != nil {
				srv.Logger.Error("invalid document UUID", "uuid", uuidStr, "error", err)
				continue
			}
			uuids = append(uuids, uuid)
			providerIDs = append(providerIDs, uuidStr) // Use UUID as provider ID for now
		}

		if len(uuids) > 0 {
			if err := manager.QueueDocuments(r.Context(), job.ID, uuids, providerIDs); err != nil {
				srv.Logger.Error("failed to queue documents", "error", err)
				http.Error(w, "Failed to queue documents", http.StatusInternalServerError)
				return
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(job); err != nil {
		srv.Logger.Error("error encoding response", "error", err)
	}
}

// listMigrationJobs lists all migration jobs
func listMigrationJobs(w http.ResponseWriter, r *http.Request, srv server.Server) {
	// Parse query parameters
	status := r.URL.Query().Get("status")
	limit := 50 // Default limit

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	// Query jobs from database
	query := `
		SELECT id, job_uuid, job_name, source_provider_id, dest_provider_id,
			   status, strategy, total_documents, migrated_documents, failed_documents,
			   created_at, started_at, completed_at
		FROM migration_jobs
	`
	args := []interface{}{}

	if status != "" {
		query += " WHERE status = $1"
		args = append(args, status)
	}

	//nolint:gosec // G202: query is built from integer limit, not user input
	query += " ORDER BY created_at DESC LIMIT $" + strconv.Itoa(len(args)+1)
	args = append(args, limit)

	sqlDB, err := getSQLDB(srv)
	if err != nil {
		srv.Logger.Error("failed to get SQL DB", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	//nolint:gosec // G701: query uses parameterized args, not string interpolation
	rows, err := sqlDB.QueryContext(r.Context(), query, args...)
	if err != nil {
		srv.Logger.Error("failed to query jobs", "error", err)
		http.Error(w, "Failed to list jobs", http.StatusInternalServerError)
		return
	}
	defer func() { _ = rows.Close() }()

	var jobs []map[string]interface{}
	for rows.Next() {
		var (
			id, sourceProviderID, destProviderID int64
			jobUUID, jobName, status, strategy   string
			totalDocs, migratedDocs, failedDocs  int
			createdAt                            string
			startedAt, completedAt               sql.NullString
		)

		if err := rows.Scan(&id, &jobUUID, &jobName, &sourceProviderID, &destProviderID,
			&status, &strategy, &totalDocs, &migratedDocs, &failedDocs,
			&createdAt, &startedAt, &completedAt); err != nil {
			srv.Logger.Error("failed to scan job", "error", err)
			continue
		}

		job := map[string]interface{}{
			"id":                id,
			"jobUuid":           jobUUID,
			"jobName":           jobName,
			"sourceProviderId":  sourceProviderID,
			"destProviderId":    destProviderID,
			"status":            status,
			"strategy":          strategy,
			"totalDocuments":    totalDocs,
			"migratedDocuments": migratedDocs,
			"failedDocuments":   failedDocs,
			"createdAt":         createdAt,
		}

		if startedAt.Valid {
			job["startedAt"] = startedAt.String
		}
		if completedAt.Valid {
			job["completedAt"] = completedAt.String
		}

		jobs = append(jobs, job)
	}

	response := map[string]interface{}{
		"jobs":  jobs,
		"count": len(jobs),
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		srv.Logger.Error("error encoding response", "error", err)
	}
}

// getMigrationJob gets a specific migration job
func getMigrationJob(w http.ResponseWriter, r *http.Request, srv server.Server, jobID int64) {
	sqlDB, err := getSQLDB(srv)
	if err != nil {
		srv.Logger.Error("failed to get SQL DB", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	manager := migration.NewManager(sqlDB, srv.Logger)

	job, err := manager.GetJob(r.Context(), jobID)
	if err != nil {
		if err.Error() == "migration job "+strconv.FormatInt(jobID, 10)+" not found" {
			http.Error(w, "Job not found", http.StatusNotFound)
		} else {
			srv.Logger.Error("failed to get job", "error", err)
			http.Error(w, "Failed to get job", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(job); err != nil {
		srv.Logger.Error("error encoding response", "error", err)
	}
}

// startMigrationJob starts a migration job
func startMigrationJob(w http.ResponseWriter, r *http.Request, srv server.Server, jobID int64) {
	sqlDB, err := getSQLDB(srv)
	if err != nil {
		srv.Logger.Error("failed to get SQL DB", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	manager := migration.NewManager(sqlDB, srv.Logger)

	if err := manager.StartJob(r.Context(), jobID); err != nil {
		srv.Logger.Error("failed to start job", "jobID", jobID, "error", err)
		http.Error(w, "Failed to start job", http.StatusInternalServerError)
		return
	}

	// Return updated job
	job, err := manager.GetJob(r.Context(), jobID)
	if err != nil {
		srv.Logger.Error("failed to get job after start", "error", err)
		http.Error(w, "Failed to get job", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(job); err != nil {
		srv.Logger.Error("error encoding response", "error", err)
	}
}

// pauseMigrationJob pauses a migration job
func pauseMigrationJob(w http.ResponseWriter, r *http.Request, srv server.Server, jobID int64) {
	sqlDB, err := getSQLDB(srv)
	if err != nil {
		srv.Logger.Error("failed to get SQL DB", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Update job status to paused
	result, err := sqlDB.ExecContext(r.Context(), `
		UPDATE migration_jobs
		SET status = $1, updated_at = NOW()
		WHERE id = $2 AND status = $3
	`, "paused", jobID, "running")

	if err != nil {
		srv.Logger.Error("failed to pause job", "jobID", jobID, "error", err)
		http.Error(w, "Failed to pause job", http.StatusInternalServerError)
		return
	}

	rows, err := result.RowsAffected()
	if err != nil {
		http.Error(w, "Failed to check result", http.StatusInternalServerError)
		return
	}
	if rows == 0 {
		http.Error(w, "Job not found or not running", http.StatusBadRequest)
		return
	}

	// Return updated job
	sqlDB, err = getSQLDB(srv)
	if err != nil {
		srv.Logger.Error("failed to get SQL DB", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	manager := migration.NewManager(sqlDB, srv.Logger)
	job, err := manager.GetJob(r.Context(), jobID)
	if err != nil {
		srv.Logger.Error("failed to get job after pause", "error", err)
		http.Error(w, "Failed to get job", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(job); err != nil {
		srv.Logger.Error("error encoding response", "error", err)
	}
}

// cancelMigrationJob cancels a migration job
func cancelMigrationJob(w http.ResponseWriter, r *http.Request, srv server.Server, jobID int64) {
	sqlDB, err := getSQLDB(srv)
	if err != nil {
		srv.Logger.Error("failed to get SQL DB", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Update job status to canceled
	result, err := sqlDB.ExecContext(r.Context(), `
		UPDATE migration_jobs
		SET status = $1, completed_at = NOW(), updated_at = NOW()
		WHERE id = $2 AND status IN ($3, $4)
	`, "canceled", jobID, "pending", "running")

	if err != nil {
		srv.Logger.Error("failed to cancel job", "jobID", jobID, "error", err)
		http.Error(w, "Failed to cancel job", http.StatusInternalServerError)
		return
	}

	rows, err := result.RowsAffected()
	if err != nil {
		http.Error(w, "Failed to check result", http.StatusInternalServerError)
		return
	}
	if rows == 0 {
		http.Error(w, "Job not found or already completed", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Job canceled successfully",
		"jobId":   jobID,
	}); err != nil {
		srv.Logger.Error("error encoding response", "error", err)
	}
}

// getMigrationProgress gets migration progress for a job
func getMigrationProgress(w http.ResponseWriter, r *http.Request, srv server.Server, jobID int64) {
	sqlDB, err := getSQLDB(srv)
	if err != nil {
		srv.Logger.Error("failed to get SQL DB", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	manager := migration.NewManager(sqlDB, srv.Logger)

	progress, err := manager.GetProgress(r.Context(), jobID)
	if err != nil {
		srv.Logger.Error("failed to get progress", "jobID", jobID, "error", err)
		http.Error(w, "Failed to get progress", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(progress); err != nil {
		srv.Logger.Error("error encoding response", "error", err)
	}
}

// listMigrationItems lists migration items for a job
//
//nolint:gocognit,gocyclo // Response assembly is explicit to keep migration item filtering readable.
func listMigrationItems(w http.ResponseWriter, r *http.Request, srv server.Server, jobID int64) {
	// Parse query parameters
	status := r.URL.Query().Get("status")
	limit := 100 // Default limit

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 1000 {
			limit = l
		}
	}

	// Query items from database
	query := `
		SELECT id, document_uuid, source_provider_id, dest_provider_id,
			   status, attempt_count, error_message, duration_ms, content_match,
			   created_at, started_at, completed_at
		FROM migration_items
		WHERE migration_job_id = $1
	`
	args := []interface{}{jobID}

	if status != "" {
		query += " AND status = $2"
		args = append(args, status)
	}

	//nolint:gosec // G202: query is built from integer limit, not user input
	query += " ORDER BY created_at ASC LIMIT $" + strconv.Itoa(len(args)+1)
	args = append(args, limit)

	sqlDB, err := getSQLDB(srv)
	if err != nil {
		srv.Logger.Error("failed to get SQL DB", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	//nolint:gosec // G701: query uses parameterized args, not string interpolation
	rows, err := sqlDB.QueryContext(r.Context(), query, args...)
	if err != nil {
		srv.Logger.Error("failed to query items", "error", err)
		http.Error(w, "Failed to list items", http.StatusInternalServerError)
		return
	}
	defer func() { _ = rows.Close() }()

	var items []map[string]interface{}
	for rows.Next() {
		var (
			id, attemptCount                       int64
			documentUUID, sourceProviderID, status string
			destProviderID, errorMessage           sql.NullString
			durationMs                             sql.NullInt64
			contentMatch                           sql.NullBool
			createdAt                              string
			startedAt, completedAt                 sql.NullString
		)

		if err := rows.Scan(&id, &documentUUID, &sourceProviderID, &destProviderID,
			&status, &attemptCount, &errorMessage, &durationMs, &contentMatch,
			&createdAt, &startedAt, &completedAt); err != nil {
			srv.Logger.Error("failed to scan item", "error", err)
			continue
		}

		item := map[string]interface{}{
			"id":               id,
			"documentUuid":     documentUUID,
			"sourceProviderId": sourceProviderID,
			"status":           status,
			"attemptCount":     attemptCount,
			"createdAt":        createdAt,
		}

		if destProviderID.Valid {
			item["destProviderId"] = destProviderID.String
		}
		if errorMessage.Valid {
			item["errorMessage"] = errorMessage.String
		}
		if durationMs.Valid {
			item["durationMs"] = durationMs.Int64
		}
		if contentMatch.Valid {
			item["contentMatch"] = contentMatch.Bool
		}
		if startedAt.Valid {
			item["startedAt"] = startedAt.String
		}
		if completedAt.Valid {
			item["completedAt"] = completedAt.String
		}

		items = append(items, item)
	}

	response := map[string]interface{}{
		"items": items,
		"count": len(items),
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		srv.Logger.Error("error encoding response", "error", err)
	}
}
