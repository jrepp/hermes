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
	JobStatusPending   JobStatus = "pending"
	JobStatusRunning   JobStatus = "running"
	JobStatusPaused    JobStatus = "paused"
	JobStatusCompleted JobStatus = "completed"
	JobStatusFailed    JobStatus = "failed"
	JobStatusCancelled JobStatus = "cancelled"
)

// ItemStatus represents the state of a migration item (individual document)
type ItemStatus string

const (
	ItemStatusPending    ItemStatus = "pending"
	ItemStatusInProgress ItemStatus = "in_progress"
	ItemStatusCompleted  ItemStatus = "completed"
	ItemStatusFailed     ItemStatus = "failed"
	ItemStatusSkipped    ItemStatus = "skipped"
)

// Strategy defines how documents are migrated
type Strategy string

const (
	StrategyMove   Strategy = "move"   // Move documents (delete from source)
	StrategyCopy   Strategy = "copy"   // Copy documents (keep in source)
	StrategyMirror Strategy = "mirror" // Keep both in sync
)

// Job represents a migration job
type Job struct {
	ID          int64      `json:"id" db:"id"`
	JobUUID     string     `json:"jobUuid" db:"job_uuid"`
	JobName     string     `json:"jobName" db:"job_name"`
	SourceID    int64      `json:"sourceProviderId" db:"source_provider_id"`
	DestID      int64      `json:"destProviderId" db:"dest_provider_id"`
	Status      JobStatus  `json:"status" db:"status"`
	Strategy    Strategy   `json:"strategy" db:"strategy"`
	DryRun      bool       `json:"dryRun" db:"dry_run"`
	Concurrency int        `json:"concurrency" db:"concurrency"`
	BatchSize   int        `json:"batchSize" db:"batch_size"`
	CreatedBy   string     `json:"createdBy" db:"created_by"`
	CreatedAt   time.Time  `json:"createdAt" db:"created_at"`
	UpdatedAt   time.Time  `json:"updatedAt" db:"updated_at"`
	StartedAt   *time.Time `json:"startedAt,omitempty" db:"started_at"`
	CompletedAt *time.Time `json:"completedAt,omitempty" db:"completed_at"`

	// Progress tracking
	TotalDocuments    int `json:"totalDocuments" db:"total_documents"`
	MigratedDocuments int `json:"migratedDocuments" db:"migrated_documents"`
	FailedDocuments   int `json:"failedDocuments" db:"failed_documents"`
	SkippedDocuments  int `json:"skippedDocuments" db:"skipped_documents"`

	// Validation
	ValidateAfter    bool    `json:"validateAfter" db:"validate_after_migration"`
	ValidationStatus *string `json:"validationStatus,omitempty" db:"validation_status"`

	// Rollback
	RollbackEnabled bool `json:"rollbackEnabled" db:"rollback_enabled"`
}

// Item represents a single document migration item
type Item struct {
	ID                int64      `json:"id" db:"id"`
	MigrationJobID    int64      `json:"migrationJobId" db:"migration_job_id"`
	DocumentUUID      docid.UUID `json:"documentUuid" db:"document_uuid"`
	SourceProviderID  string     `json:"sourceProviderId" db:"source_provider_id"`
	DestProviderID    *string    `json:"destProviderId,omitempty" db:"dest_provider_id"`
	Status            ItemStatus `json:"status" db:"status"`
	AttemptCount      int        `json:"attemptCount" db:"attempt_count"`
	MaxAttempts       int        `json:"maxAttempts" db:"max_attempts"`
	SourceContentHash string     `json:"sourceContentHash,omitempty" db:"source_content_hash"`
	DestContentHash   *string    `json:"destContentHash,omitempty" db:"dest_content_hash"`
	ContentMatch      *bool      `json:"contentMatch,omitempty" db:"content_match"`
	StartedAt         *time.Time `json:"startedAt,omitempty" db:"started_at"`
	CompletedAt       *time.Time `json:"completedAt,omitempty" db:"completed_at"`
	DurationMS        *int       `json:"durationMs,omitempty" db:"duration_ms"`
	ErrorMessage      *string    `json:"errorMessage,omitempty" db:"error_message"`
	IsRetryable       bool       `json:"isRetryable" db:"is_retryable"`
	CreatedAt         time.Time  `json:"createdAt" db:"created_at"`
	UpdatedAt         time.Time  `json:"updatedAt" db:"updated_at"`
}

// OutboxEvent represents a migration task event in the outbox
type OutboxEvent struct {
	ID              int64      `json:"id" db:"id"`
	MigrationJobID  int64      `json:"migrationJobId" db:"migration_job_id"`
	MigrationItemID int64      `json:"migrationItemId" db:"migration_item_id"`
	DocumentUUID    docid.UUID `json:"documentUuid" db:"document_uuid"`
	DocumentID      string     `json:"documentId" db:"document_id"`
	IdempotentKey   string     `json:"idempotentKey" db:"idempotent_key"`
	EventType       string     `json:"eventType" db:"event_type"`
	ProviderSource  string     `json:"providerSource" db:"provider_source"`
	ProviderDest    string     `json:"providerDest" db:"provider_dest"`
	Payload         string     `json:"payload" db:"payload"` // JSON
	Status          string     `json:"status" db:"status"`
	PublishedAt     *time.Time `json:"publishedAt,omitempty" db:"published_at"`
	PublishAttempts int        `json:"publishAttempts" db:"publish_attempts"`
	LastError       *string    `json:"lastError,omitempty" db:"last_error"`
	CreatedAt       time.Time  `json:"createdAt" db:"created_at"`
	UpdatedAt       time.Time  `json:"updatedAt" db:"updated_at"`
}

// CreateJobRequest represents a request to create a migration job
type CreateJobRequest struct {
	JobName        string         `json:"jobName"`
	SourceProvider string         `json:"sourceProvider"`
	DestProvider   string         `json:"destProvider"`
	Strategy       Strategy       `json:"strategy"`
	FilterCriteria map[string]any `json:"filterCriteria,omitempty"`
	Concurrency    int            `json:"concurrency,omitempty"`
	BatchSize      int            `json:"batchSize,omitempty"`
	DryRun         bool           `json:"dryRun"`
	Validate       bool           `json:"validate"`
	CreatedBy      string         `json:"createdBy"`
}

// TaskPayload represents the payload for a migration task event
type TaskPayload struct {
	JobID            int64      `json:"jobId"`
	ItemID           int64      `json:"itemId"`
	DocumentUUID     docid.UUID `json:"documentUuid"`
	SourceProvider   string     `json:"sourceProvider"`
	SourceProviderID string     `json:"sourceProviderId"`
	DestProvider     string     `json:"destProvider"`
	Strategy         Strategy   `json:"strategy"`
	DryRun           bool       `json:"dryRun"`
	Validate         bool       `json:"validate"`
	AttemptCount     int        `json:"attemptCount"`
	MaxAttempts      int        `json:"maxAttempts"`
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
	Match          bool   `json:"match"`
	SourceHash     string `json:"sourceHash"`
	DestHash       string `json:"destHash"`
	BytesDiff      int    `json:"bytesDiff"`
	ValidationTime int64  `json:"validationTimeMs"`
}
