// Package migration provides document migration orchestration between storage providers.
// Implements RFC-089: S3-Compatible Storage Backend and Document Migration System
package migration

import (
	"time"

	"github.com/hashicorp-forge/hermes/pkg/docid"
)

// JobStatus represents the state of a migration job
type JobStatus string

const (
	// JobStatusPending indicates the job is waiting to start.
	JobStatusPending JobStatus = "pending"
	// JobStatusRunning indicates the job is actively running.
	JobStatusRunning JobStatus = "running"
	// JobStatusPaused indicates the job has been paused.
	JobStatusPaused JobStatus = "paused"
	// JobStatusCompleted indicates the job finished successfully.
	JobStatusCompleted JobStatus = "completed"
	// JobStatusFailed indicates the job failed.
	JobStatusFailed JobStatus = "failed"
	// JobStatusCancelled indicates the job was canceled.
	JobStatusCancelled JobStatus = "canceled"
)

// ItemStatus represents the state of a migration item (individual document)
type ItemStatus string

const (
	// ItemStatusPending indicates the item is waiting.
	ItemStatusPending ItemStatus = "pending"
	// ItemStatusInProgress indicates the item is being processed.
	ItemStatusInProgress ItemStatus = "in_progress"
	// ItemStatusCompleted indicates the item completed successfully.
	ItemStatusCompleted ItemStatus = "completed"
	// ItemStatusFailed indicates the item failed.
	ItemStatusFailed ItemStatus = "failed"
	// ItemStatusSkipped indicates the item was skipped.
	ItemStatusSkipped ItemStatus = "skipped"
)

// Strategy defines how documents are migrated
type Strategy string

const (
	// StrategyMove removes documents from source after migration.
	StrategyMove Strategy = "move"
	// StrategyCopy keeps documents in source after migration.
	StrategyCopy Strategy = "copy"
	// StrategyMirror keeps both source and destination in sync.
	StrategyMirror Strategy = "mirror"
)

// Job represents a migration job
type Job struct {
	CreatedAt         time.Time  `json:"createdAt" db:"created_at"`
	UpdatedAt         time.Time  `json:"updatedAt" db:"updated_at"`
	ValidationStatus  *string    `json:"validationStatus,omitempty" db:"validation_status"`
	CompletedAt       *time.Time `json:"completedAt,omitempty" db:"completed_at"`
	StartedAt         *time.Time `json:"startedAt,omitempty" db:"started_at"`
	CreatedBy         string     `json:"createdBy" db:"created_by"`
	Strategy          Strategy   `json:"strategy" db:"strategy"`
	JobUUID           string     `json:"jobUuid" db:"job_uuid"`
	JobName           string     `json:"jobName" db:"job_name"`
	Status            JobStatus  `json:"status" db:"status"`
	ID                int64      `json:"id" db:"id"`
	BatchSize         int        `json:"batchSize" db:"batch_size"`
	DestID            int64      `json:"destProviderId" db:"dest_provider_id"`
	SourceID          int64      `json:"sourceProviderId" db:"source_provider_id"`
	Concurrency       int        `json:"concurrency" db:"concurrency"`
	TotalDocuments    int        `json:"totalDocuments" db:"total_documents"`
	MigratedDocuments int        `json:"migratedDocuments" db:"migrated_documents"`
	FailedDocuments   int        `json:"failedDocuments" db:"failed_documents"`
	SkippedDocuments  int        `json:"skippedDocuments" db:"skipped_documents"`
	ValidateAfter     bool       `json:"validateAfter" db:"validate_after_migration"`
	DryRun            bool       `json:"dryRun" db:"dry_run"`
	RollbackEnabled   bool       `json:"rollbackEnabled" db:"rollback_enabled"`
}

// Item represents a single document migration item
type Item struct {
	CreatedAt         time.Time  `json:"createdAt" db:"created_at"`
	UpdatedAt         time.Time  `json:"updatedAt" db:"updated_at"`
	CompletedAt       *time.Time `json:"completedAt,omitempty" db:"completed_at"`
	DurationMS        *int       `json:"durationMs,omitempty" db:"duration_ms"`
	DestProviderID    *string    `json:"destProviderId,omitempty" db:"dest_provider_id"`
	ErrorMessage      *string    `json:"errorMessage,omitempty" db:"error_message"`
	DestContentHash   *string    `json:"destContentHash,omitempty" db:"dest_content_hash"`
	ContentMatch      *bool      `json:"contentMatch,omitempty" db:"content_match"`
	StartedAt         *time.Time `json:"startedAt,omitempty" db:"started_at"`
	Status            ItemStatus `json:"status" db:"status"`
	SourceProviderID  string     `json:"sourceProviderId" db:"source_provider_id"`
	SourceContentHash string     `json:"sourceContentHash,omitempty" db:"source_content_hash"`
	AttemptCount      int        `json:"attemptCount" db:"attempt_count"`
	ID                int64      `json:"id" db:"id"`
	MaxAttempts       int        `json:"maxAttempts" db:"max_attempts"`
	MigrationJobID    int64      `json:"migrationJobId" db:"migration_job_id"`
	DocumentUUID      docid.UUID `json:"documentUuid" db:"document_uuid"`
	IsRetryable       bool       `json:"isRetryable" db:"is_retryable"`
}

// OutboxEvent represents a migration task event in the outbox
type OutboxEvent struct {
	UpdatedAt       time.Time  `json:"updatedAt" db:"updated_at"`
	CreatedAt       time.Time  `json:"createdAt" db:"created_at"`
	PublishedAt     *time.Time `json:"publishedAt,omitempty" db:"published_at"`
	LastError       *string    `json:"lastError,omitempty" db:"last_error"`
	ProviderDest    string     `json:"providerDest" db:"provider_dest"`
	IdempotentKey   string     `json:"idempotentKey" db:"idempotent_key"`
	EventType       string     `json:"eventType" db:"event_type"`
	ProviderSource  string     `json:"providerSource" db:"provider_source"`
	Payload         string     `json:"payload" db:"payload"`
	Status          string     `json:"status" db:"status"`
	DocumentID      string     `json:"documentId" db:"document_id"`
	ID              int64      `json:"id" db:"id"`
	PublishAttempts int        `json:"publishAttempts" db:"publish_attempts"`
	MigrationItemID int64      `json:"migrationItemId" db:"migration_item_id"`
	MigrationJobID  int64      `json:"migrationJobId" db:"migration_job_id"`
	DocumentUUID    docid.UUID `json:"documentUuid" db:"document_uuid"`
}

// CreateJobRequest represents a request to create a migration job
type CreateJobRequest struct {
	FilterCriteria map[string]any `json:"filterCriteria,omitempty"`
	JobName        string         `json:"jobName"`
	SourceProvider string         `json:"sourceProvider"`
	DestProvider   string         `json:"destProvider"`
	Strategy       Strategy       `json:"strategy"`
	CreatedBy      string         `json:"createdBy"`
	Concurrency    int            `json:"concurrency,omitempty"`
	BatchSize      int            `json:"batchSize,omitempty"`
	DryRun         bool           `json:"dryRun"`
	Validate       bool           `json:"validate"`
}

// TaskPayload represents the payload for a migration task event
type TaskPayload struct {
	SourceProvider   string     `json:"sourceProvider"`
	SourceProviderID string     `json:"sourceProviderId"`
	DestProvider     string     `json:"destProvider"`
	Strategy         Strategy   `json:"strategy"`
	JobID            int64      `json:"jobId"`
	ItemID           int64      `json:"itemId"`
	AttemptCount     int        `json:"attemptCount"`
	MaxAttempts      int        `json:"maxAttempts"`
	DocumentUUID     docid.UUID `json:"documentUuid"`
	DryRun           bool       `json:"dryRun"`
	Validate         bool       `json:"validate"`
}

// Progress represents migration progress statistics
type Progress struct {
	Total      int     `json:"total"`
	Migrated   int     `json:"migrated"`
	Failed     int     `json:"failed"`
	Skipped    int     `json:"skipped"`
	Pending    int     `json:"pending"`
	Percent    float64 `json:"percent"`
	Rate       float64 `json:"rate"` // docs/second
	ETASeconds int     `json:"etaSeconds"`
}

// ValidationResult represents content validation results
type ValidationResult struct {
	SourceHash     string `json:"sourceHash"`
	DestHash       string `json:"destHash"`
	BytesDiff      int    `json:"bytesDiff"`
	ValidationTime int64  `json:"validationTimeMs"`
	Match          bool   `json:"match"`
}
