package migration

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hashicorp-forge/hermes/pkg/workspace"
	"github.com/hashicorp/go-hclog"
)

// Worker processes migration tasks
type Worker struct {
	db             *sql.DB
	providerMap    map[string]workspace.WorkspaceProvider
	manager        *Manager
	logger         hclog.Logger
	pollInterval   time.Duration
	maxConcurrency int
}

// WorkerConfig contains worker configuration
type WorkerConfig struct {
	PollInterval   time.Duration
	MaxConcurrency int
}

// NewWorker creates a new migration worker
func NewWorker(db *sql.DB, providerMap map[string]workspace.WorkspaceProvider, logger hclog.Logger, cfg *WorkerConfig) *Worker {
	if logger == nil {
		logger = hclog.NewNullLogger()
	}
	if cfg == nil {
		cfg = &WorkerConfig{
			PollInterval:   5 * time.Second,
			MaxConcurrency: 5,
		}
	}

	return &Worker{
		db:             db,
		providerMap:    providerMap,
		manager:        NewManager(db, logger),
		logger:         logger.Named("migration-worker"),
		pollInterval:   cfg.PollInterval,
		maxConcurrency: cfg.MaxConcurrency,
	}
}

// Start starts the worker
func (w *Worker) Start(ctx context.Context) error {
	w.logger.Info("migration worker started",
		"poll_interval", w.pollInterval,
		"max_concurrency", w.maxConcurrency)

	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("migration worker stopped")
			return ctx.Err()
		case <-ticker.C:
			if err := w.processPendingTasks(ctx); err != nil {
				w.logger.Error("failed to process pending tasks", "error", err)
			}
		}
	}
}

// processPendingTasks processes pending migration tasks from the outbox
func (w *Worker) processPendingTasks(ctx context.Context) error {
	// Fetch pending tasks (limit to max concurrency)
	rows, err := w.db.QueryContext(ctx, `
		SELECT id, migration_job_id, migration_item_id, payload
		FROM migration_outbox
		WHERE status = 'pending'
		ORDER BY created_at ASC
		LIMIT $1
		FOR UPDATE SKIP LOCKED
	`, w.maxConcurrency)
	if err != nil {
		return fmt.Errorf("failed to fetch pending tasks: %w", err)
	}
	defer rows.Close()

	var tasks []struct {
		outboxID int64
		jobID    int64
		itemID   int64
		payload  string
	}

	for rows.Next() {
		var task struct {
			outboxID int64
			jobID    int64
			itemID   int64
			payload  string
		}
		if err := rows.Scan(&task.outboxID, &task.jobID, &task.itemID, &task.payload); err != nil {
			w.logger.Error("failed to scan task", "error", err)
			continue
		}
		tasks = append(tasks, task)
	}

	if len(tasks) == 0 {
		return nil // No pending tasks
	}

	w.logger.Debug("processing migration tasks", "count", len(tasks))

	// Process each task
	for _, task := range tasks {
		// Mark as published (being processed)
		_, err := w.db.ExecContext(ctx, `
			UPDATE migration_outbox
			SET status = 'published', published_at = NOW(), updated_at = NOW()
			WHERE id = $1
		`, task.outboxID)
		if err != nil {
			w.logger.Error("failed to mark task as published", "outbox_id", task.outboxID, "error", err)
			continue
		}

		// Process the task
		if err := w.processTask(ctx, task.itemID, task.payload); err != nil {
			w.logger.Error("failed to process task",
				"item_id", task.itemID,
				"error", err)

			// Mark outbox event as failed
			_, _ = w.db.ExecContext(ctx, `
				UPDATE migration_outbox
				SET status = 'failed', last_error = $1, updated_at = NOW()
				WHERE id = $2
			`, err.Error(), task.outboxID)
		} else {
			// Task completed successfully - outbox event stays as published
			w.logger.Info("migration task completed", "item_id", task.itemID)
		}
	}

	return nil
}

// processTask processes a single migration task
func (w *Worker) processTask(ctx context.Context, itemID int64, payloadJSON string) error {
	// Parse payload
	var payload TaskPayload
	if err := json.Unmarshal([]byte(payloadJSON), &payload); err != nil {
		return fmt.Errorf("failed to parse payload: %w", err)
	}

	w.logger.Info("processing migration task",
		"item_id", itemID,
		"document_uuid", payload.DocumentUUID,
		"source", payload.SourceProvider,
		"dest", payload.DestProvider,
		"strategy", payload.Strategy)

	// Get providers
	sourceProvider, ok := w.providerMap[payload.SourceProvider]
	if !ok {
		return fmt.Errorf("source provider %s not found", payload.SourceProvider)
	}

	destProvider, ok := w.providerMap[payload.DestProvider]
	if !ok {
		return fmt.Errorf("dest provider %s not found", payload.DestProvider)
	}

	// Update item status to in_progress
	_, err := w.db.ExecContext(ctx, `
		UPDATE migration_items
		SET status = $1, started_at = NOW(), attempt_count = attempt_count + 1, updated_at = NOW()
		WHERE id = $2
	`, ItemStatusInProgress, itemID)
	if err != nil {
		return fmt.Errorf("failed to update item status: %w", err)
	}

	startTime := time.Now()

	// Execute migration
	var destProviderID string
	var validationResult *ValidationResult

	if payload.DryRun {
		// Dry run - just validate source exists
		_, err := sourceProvider.GetContent(ctx, payload.SourceProviderID)
		if err != nil {
			return w.failItem(ctx, itemID, fmt.Sprintf("source document not found: %v", err))
		}
		w.logger.Info("dry run - skipping actual migration", "item_id", itemID)
		destProviderID = "dry-run:skipped"
	} else {
		// Actual migration
		destProviderID, validationResult, err = w.migrateDocument(ctx, sourceProvider, destProvider, &payload)
		if err != nil {
			return w.failItem(ctx, itemID, err.Error())
		}
	}

	duration := time.Since(startTime).Milliseconds()

	// Update item as completed
	var contentMatch *bool
	if validationResult != nil {
		contentMatch = &validationResult.Match
	}

	err = w.manager.UpdateItemStatus(ctx, itemID, ItemStatusCompleted, &destProviderID, contentMatch, nil)
	if err != nil {
		w.logger.Error("failed to update item status", "error", err)
		return err
	}

	// Update duration
	_, _ = w.db.ExecContext(ctx, `
		UPDATE migration_items SET duration_ms = $1 WHERE id = $2
	`, duration, itemID)

	w.logger.Info("document migrated successfully",
		"item_id", itemID,
		"document_uuid", payload.DocumentUUID,
		"dest_provider_id", destProviderID,
		"duration_ms", duration)

	return nil
}

// migrateDocument performs the actual document migration
func (w *Worker) migrateDocument(ctx context.Context, source, dest workspace.WorkspaceProvider, payload *TaskPayload) (string, *ValidationResult, error) {
	// Get source content
	sourceContent, err := source.GetContent(ctx, payload.SourceProviderID)
	if err != nil {
		return "", nil, fmt.Errorf("failed to get source content: %w", err)
	}

	// Get source metadata
	sourceDoc, err := source.GetDocument(ctx, payload.SourceProviderID)
	if err != nil {
		return "", nil, fmt.Errorf("failed to get source metadata: %w", err)
	}

	// Create document in destination with same UUID
	destDoc, err := dest.CreateDocumentWithUUID(ctx, payload.DocumentUUID, "", "", sourceDoc.Name)
	if err != nil {
		return "", nil, fmt.Errorf("failed to create dest document: %w", err)
	}

	// Write content to destination
	_, err = dest.UpdateContent(ctx, destDoc.ProviderID, sourceContent.Body)
	if err != nil {
		// Try to clean up
		_ = dest.DeleteDocument(ctx, destDoc.ProviderID)
		return "", nil, fmt.Errorf("failed to write dest content: %w", err)
	}

	var validationResult *ValidationResult

	// Validate if requested
	if payload.Validate {
		destContent, err := dest.GetContent(ctx, destDoc.ProviderID)
		if err != nil {
			w.logger.Warn("validation failed - could not read dest content", "error", err)
		} else {
			validationStart := time.Now()

			// Normalize hashes by stripping "sha256:" prefix if present
			sourceHash := sourceContent.ContentHash
			destHash := destContent.ContentHash
			if len(sourceHash) > 7 && sourceHash[:7] == "sha256:" {
				sourceHash = sourceHash[7:]
			}
			if len(destHash) > 7 && destHash[:7] == "sha256:" {
				destHash = destHash[7:]
			}

			match := sourceHash == destHash
			bytesDiff := len(sourceContent.Body) - len(destContent.Body)
			if bytesDiff < 0 {
				bytesDiff = -bytesDiff
			}

			validationResult = &ValidationResult{
				Match:          match,
				SourceHash:     sourceContent.ContentHash,
				DestHash:       destContent.ContentHash,
				BytesDiff:      bytesDiff,
				ValidationTime: time.Since(validationStart).Milliseconds(),
			}

			if !match {
				w.logger.Warn("content validation failed - hashes don't match",
					"source_hash", sourceContent.ContentHash,
					"dest_hash", destContent.ContentHash,
					"normalized_source", sourceHash,
					"normalized_dest", destHash)
			}
		}
	}

	// Handle move strategy - delete from source
	if payload.Strategy == StrategyMove {
		if err := source.DeleteDocument(ctx, payload.SourceProviderID); err != nil {
			w.logger.Error("failed to delete source document after move", "error", err)
			// Don't fail the migration - document is already in dest
		}
	}

	return destDoc.ProviderID, validationResult, nil
}

// failItem marks an item as failed
func (w *Worker) failItem(ctx context.Context, itemID int64, errorMsg string) error {
	w.logger.Error("migration item failed", "item_id", itemID, "error", errorMsg)

	return w.manager.UpdateItemStatus(ctx, itemID, ItemStatusFailed, nil, nil, &errorMsg)
}
