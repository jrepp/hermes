package migration

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp-forge/hermes/pkg/docid"
	"github.com/hashicorp/go-hclog"
)

// Manager orchestrates document migration between storage providers
type Manager struct {
	db     *sql.DB
	logger hclog.Logger
}

// NewManager creates a new migration manager
func NewManager(db *sql.DB, logger hclog.Logger) *Manager {
	if logger == nil {
		logger = hclog.NewNullLogger()
	}
	return &Manager{
		db:     db,
		logger: logger.Named("migration-manager"),
	}
}

// CreateJob creates a new migration job
func (m *Manager) CreateJob(ctx context.Context, req *CreateJobRequest) (*Job, error) {
	// Validate request
	if req.JobName == "" {
		return nil, fmt.Errorf("job name is required")
	}
	if req.SourceProvider == "" || req.DestProvider == "" {
		return nil, fmt.Errorf("source and destination providers are required")
	}
	if req.Strategy == "" {
		req.Strategy = StrategyCopy // Default to copy
	}

	// Set defaults
	if req.Concurrency == 0 {
		req.Concurrency = 5
	}
	if req.BatchSize == 0 {
		req.BatchSize = 100
	}

	// Lookup provider IDs
	sourceID, err := m.getProviderID(ctx, req.SourceProvider)
	if err != nil {
		return nil, fmt.Errorf("failed to lookup source provider: %w", err)
	}
	destID, err := m.getProviderID(ctx, req.DestProvider)
	if err != nil {
		return nil, fmt.Errorf("failed to lookup dest provider: %w", err)
	}

	// Generate job UUID
	jobUUID := uuid.New().String()

	// Serialize filter criteria
	var filterCriteriaJSON []byte
	if req.FilterCriteria != nil {
		filterCriteriaJSON, err = json.Marshal(req.FilterCriteria)
		if err != nil {
			return nil, fmt.Errorf("failed to serialize filter criteria: %w", err)
		}
	} else {
		// Use empty JSON object instead of nil to avoid PostgreSQL parsing errors
		filterCriteriaJSON = []byte("{}")
	}

	// Insert job
	query := `
		INSERT INTO migration_jobs (
			job_uuid, job_name, source_provider_id, dest_provider_id,
			filter_criteria, strategy, status, concurrency, batch_size,
			dry_run, validate_after_migration, rollback_enabled, created_by
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		RETURNING id, created_at, updated_at
	`

	var job Job
	err = m.db.QueryRowContext(ctx, query,
		jobUUID, req.JobName, sourceID, destID,
		filterCriteriaJSON, req.Strategy, JobStatusPending,
		req.Concurrency, req.BatchSize, req.DryRun, req.Validate,
		true, req.CreatedBy,
	).Scan(&job.ID, &job.CreatedAt, &job.UpdatedAt)

	if err != nil {
		return nil, fmt.Errorf("failed to create migration job: %w", err)
	}

	// Populate job fields
	job.JobUUID = jobUUID
	job.JobName = req.JobName
	job.SourceID = sourceID
	job.DestID = destID
	job.Strategy = req.Strategy
	job.Status = JobStatusPending
	job.DryRun = req.DryRun
	job.Concurrency = req.Concurrency
	job.BatchSize = req.BatchSize
	job.CreatedBy = req.CreatedBy
	job.ValidateAfter = req.Validate
	job.RollbackEnabled = true

	m.logger.Info("migration job created",
		"job_id", job.ID,
		"job_uuid", jobUUID,
		"source", req.SourceProvider,
		"dest", req.DestProvider,
		"strategy", req.Strategy)

	return &job, nil
}

// QueueDocuments queues documents for migration
func (m *Manager) QueueDocuments(ctx context.Context, jobID int64, documentUUIDs []docid.UUID, sourceProviderIDs []string) error {
	if len(documentUUIDs) != len(sourceProviderIDs) {
		return fmt.Errorf("documentUUIDs and sourceProviderIDs length mismatch")
	}

	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Insert migration items
	itemStmt, err := tx.PrepareContext(ctx, `
		INSERT INTO migration_items (
			migration_job_id, document_uuid, source_provider_id,
			status, attempt_count, max_attempts
		) VALUES ($1, $2, $3, $4, 0, 3)
		RETURNING id
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare item insert: %w", err)
	}
	defer itemStmt.Close()

	// Get job details for outbox events
	var sourceProvider, destProvider string
	var strategy Strategy
	var dryRun, validate bool
	err = tx.QueryRowContext(ctx, `
		SELECT
			sp.provider_name, dp.provider_name, mj.strategy, mj.dry_run, mj.validate_after_migration
		FROM migration_jobs mj
		JOIN provider_storage sp ON mj.source_provider_id = sp.id
		JOIN provider_storage dp ON mj.dest_provider_id = dp.id
		WHERE mj.id = $1
	`, jobID).Scan(&sourceProvider, &destProvider, &strategy, &dryRun, &validate)
	if err != nil {
		return fmt.Errorf("failed to get job details: %w", err)
	}

	outboxStmt, err := tx.PrepareContext(ctx, `
		INSERT INTO migration_outbox (
			migration_job_id, migration_item_id, document_uuid, document_id,
			idempotent_key, event_type, provider_source, provider_dest, payload, status
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare outbox insert: %w", err)
	}
	defer outboxStmt.Close()

	// Queue each document
	for i, docUUID := range documentUUIDs {
		// Insert migration item
		var itemID int64
		err = itemStmt.QueryRowContext(ctx, jobID, docUUID.String(), sourceProviderIDs[i], ItemStatusPending).Scan(&itemID)
		if err != nil {
			return fmt.Errorf("failed to insert migration item: %w", err)
		}

		// Create task payload
		payload := TaskPayload{
			JobID:            jobID,
			ItemID:           itemID,
			DocumentUUID:     docUUID,
			SourceProvider:   sourceProvider,
			SourceProviderID: sourceProviderIDs[i],
			DestProvider:     destProvider,
			Strategy:         strategy,
			DryRun:           dryRun,
			Validate:         validate,
			AttemptCount:     0,
			MaxAttempts:      3,
		}
		payloadJSON, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("failed to marshal payload: %w", err)
		}

		// Insert outbox event
		idempotentKey := fmt.Sprintf("%d:%s", jobID, docUUID.String())
		_, err = outboxStmt.ExecContext(ctx,
			jobID, itemID, docUUID.String(), sourceProviderIDs[i],
			idempotentKey, "migration.task.created",
			sourceProvider, destProvider, string(payloadJSON), "pending",
		)
		if err != nil {
			return fmt.Errorf("failed to insert outbox event: %w", err)
		}
	}

	// Update job total count
	_, err = tx.ExecContext(ctx, `
		UPDATE migration_jobs
		SET total_documents = $1, updated_at = NOW()
		WHERE id = $2
	`, len(documentUUIDs), jobID)
	if err != nil {
		return fmt.Errorf("failed to update job total: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	m.logger.Info("documents queued for migration",
		"job_id", jobID,
		"count", len(documentUUIDs))

	return nil
}

// StartJob starts a migration job
func (m *Manager) StartJob(ctx context.Context, jobID int64) error {
	result, err := m.db.ExecContext(ctx, `
		UPDATE migration_jobs
		SET status = $1, started_at = NOW(), updated_at = NOW()
		WHERE id = $2 AND status = $3
	`, JobStatusRunning, jobID, JobStatusPending)

	if err != nil {
		return fmt.Errorf("failed to start job: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("job %d not found or not in pending state", jobID)
	}

	m.logger.Info("migration job started", "job_id", jobID)
	return nil
}

// GetJob retrieves a migration job
func (m *Manager) GetJob(ctx context.Context, jobID int64) (*Job, error) {
	var job Job
	query := `
		SELECT
			id, job_uuid, job_name, source_provider_id, dest_provider_id,
			status, strategy, concurrency, batch_size, dry_run,
			validate_after_migration, validation_status, rollback_enabled,
			total_documents, migrated_documents, failed_documents, skipped_documents,
			created_by, created_at, updated_at, started_at, completed_at
		FROM migration_jobs
		WHERE id = $1
	`

	err := m.db.QueryRowContext(ctx, query, jobID).Scan(
		&job.ID, &job.JobUUID, &job.JobName, &job.SourceID, &job.DestID,
		&job.Status, &job.Strategy, &job.Concurrency, &job.BatchSize, &job.DryRun,
		&job.ValidateAfter, &job.ValidationStatus, &job.RollbackEnabled,
		&job.TotalDocuments, &job.MigratedDocuments, &job.FailedDocuments, &job.SkippedDocuments,
		&job.CreatedBy, &job.CreatedAt, &job.UpdatedAt, &job.StartedAt, &job.CompletedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("migration job %d not found", jobID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get migration job: %w", err)
	}

	return &job, nil
}

// GetProgress calculates migration progress
func (m *Manager) GetProgress(ctx context.Context, jobID int64) (*Progress, error) {
	job, err := m.GetJob(ctx, jobID)
	if err != nil {
		return nil, err
	}

	progress := &Progress{
		Total:    job.TotalDocuments,
		Migrated: job.MigratedDocuments,
		Failed:   job.FailedDocuments,
		Skipped:  job.SkippedDocuments,
		Pending:  job.TotalDocuments - job.MigratedDocuments - job.FailedDocuments - job.SkippedDocuments,
	}

	if job.TotalDocuments > 0 {
		completed := job.MigratedDocuments + job.FailedDocuments + job.SkippedDocuments
		progress.Percent = float64(completed) / float64(job.TotalDocuments) * 100

		// Calculate rate and ETA if job is running
		if job.Status == JobStatusRunning && job.StartedAt != nil {
			elapsed := time.Since(*job.StartedAt).Seconds()
			if elapsed > 0 {
				progress.Rate = float64(completed) / elapsed
				if progress.Rate > 0 {
					progress.ETASeconds = int(float64(progress.Pending) / progress.Rate)
				}
			}
		}
	}

	return progress, nil
}

// UpdateItemStatus updates the status of a migration item
func (m *Manager) UpdateItemStatus(ctx context.Context, itemID int64, status ItemStatus, destProviderID *string, contentMatch *bool, errorMsg *string) error {
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	now := time.Now()

	// Update item
	_, err = tx.ExecContext(ctx, `
		UPDATE migration_items
		SET status = $1, dest_provider_id = $2, content_match = $3,
			error_message = $4, completed_at = $5, updated_at = NOW()
		WHERE id = $6
	`, status, destProviderID, contentMatch, errorMsg, now, itemID)
	if err != nil {
		return fmt.Errorf("failed to update item: %w", err)
	}

	// Update job counters
	var counterField string
	switch status {
	case ItemStatusCompleted:
		counterField = "migrated_documents"
	case ItemStatusFailed:
		counterField = "failed_documents"
	case ItemStatusSkipped:
		counterField = "skipped_documents"
	default:
		// No counter update for other statuses
		return tx.Commit()
	}

	_, err = tx.ExecContext(ctx, fmt.Sprintf(`
		UPDATE migration_jobs
		SET %s = %s + 1, updated_at = NOW()
		WHERE id = (SELECT migration_job_id FROM migration_items WHERE id = $1)
	`, counterField, counterField), itemID)
	if err != nil {
		return fmt.Errorf("failed to update job counters: %w", err)
	}

	return tx.Commit()
}

// Helper: getProviderID looks up provider ID by name
func (m *Manager) getProviderID(ctx context.Context, providerName string) (int64, error) {
	var id int64
	err := m.db.QueryRowContext(ctx,
		"SELECT id FROM provider_storage WHERE provider_name = $1",
		providerName,
	).Scan(&id)

	if err == sql.ErrNoRows {
		return 0, fmt.Errorf("provider %s not found", providerName)
	}
	return id, err
}
