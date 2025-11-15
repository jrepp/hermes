package models

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

// DocumentRevisionPipelineExecution tracks pipeline processing for document revisions.
// Records which rulesets matched and what steps were executed, with per-step results.
type DocumentRevisionPipelineExecution struct {
	ID uint `gorm:"primaryKey" json:"id"`

	// Links to revision and outbox
	RevisionID uint `gorm:"not null;index:idx_pipeline_exec_revision_id" json:"revisionId"`
	OutboxID   uint `gorm:"not null;index:idx_pipeline_exec_outbox_id" json:"outboxId"`

	// Execution metadata
	RulesetName   string   `gorm:"type:varchar(100);not null;index:idx_pipeline_exec_ruleset" json:"rulesetName"`
	PipelineSteps []string `gorm:"serializer:json;type:jsonb;not null" json:"pipelineSteps"` // ['search_index', 'embeddings', 'llm_summary']

	// Execution state
	Status      string     `gorm:"type:varchar(20);not null;default:'pending';index:idx_pipeline_exec_status" json:"status"` // 'pending', 'running', 'completed', 'failed', 'partial'
	StartedAt   *time.Time `json:"startedAt,omitempty"`
	CompletedAt *time.Time `json:"completedAt,omitempty"`

	// Results per step
	// Example: {"search_index": {"status": "success", "duration_ms": 234}, "embeddings": {"status": "failed", "error": "..."}}
	StepResults map[string]interface{} `gorm:"serializer:json;type:jsonb" json:"stepResults,omitempty"`

	// Error details for debugging
	ErrorDetails map[string]interface{} `gorm:"serializer:json;type:jsonb" json:"errorDetails,omitempty"`

	// Retry tracking
	AttemptNumber int `gorm:"not null;default:1" json:"attemptNumber"`
	MaxAttempts   int `gorm:"not null;default:3" json:"maxAttempts"`

	// Timestamps
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`

	// Associations
	Revision *DocumentRevision       `gorm:"foreignKey:RevisionID" json:"-"`
	Outbox   *DocumentRevisionOutbox `gorm:"foreignKey:OutboxID" json:"-"`
}

// TableName specifies the table name.
func (DocumentRevisionPipelineExecution) TableName() string {
	return "document_revision_pipeline_executions"
}

// PipelineStatus constants
const (
	PipelineStatusPending   = "pending"   // Not yet started
	PipelineStatusRunning   = "running"   // Currently executing
	PipelineStatusCompleted = "completed" // All steps succeeded
	PipelineStatusFailed    = "failed"    // At least one step failed
	PipelineStatusPartial   = "partial"   // Some steps succeeded, some failed
)

// StepStatus constants for individual step results
const (
	StepStatusSuccess = "success"
	StepStatusFailed  = "failed"
	StepStatusSkipped = "skipped"
)

// BeforeCreate hook to ensure required fields.
func (e *DocumentRevisionPipelineExecution) BeforeCreate(tx *gorm.DB) error {
	if e.RevisionID == 0 {
		return fmt.Errorf("revision_id is required")
	}
	if e.OutboxID == 0 {
		return fmt.Errorf("outbox_id is required")
	}
	if e.RulesetName == "" {
		return fmt.Errorf("ruleset_name is required")
	}
	if len(e.PipelineSteps) == 0 {
		return fmt.Errorf("pipeline_steps is required")
	}

	// Set default status
	if e.Status == "" {
		e.Status = PipelineStatusPending
	}

	// Initialize step results if not set
	if e.StepResults == nil {
		e.StepResults = make(map[string]interface{})
	}

	return nil
}

// NewPipelineExecution creates a new pipeline execution record.
func NewPipelineExecution(revisionID, outboxID uint, rulesetName string, steps []string) *DocumentRevisionPipelineExecution {
	return &DocumentRevisionPipelineExecution{
		RevisionID:    revisionID,
		OutboxID:      outboxID,
		RulesetName:   rulesetName,
		PipelineSteps: steps,
		Status:        PipelineStatusPending,
		AttemptNumber: 1,
		MaxAttempts:   3,
		StepResults:   make(map[string]interface{}),
	}
}

// Start marks the execution as running.
func (e *DocumentRevisionPipelineExecution) Start(db *gorm.DB) error {
	now := time.Now()
	e.Status = PipelineStatusRunning
	e.StartedAt = &now
	e.UpdatedAt = now

	return db.Save(e).Error
}

// RecordStepResult records the result of a pipeline step.
func (e *DocumentRevisionPipelineExecution) RecordStepResult(db *gorm.DB, stepName, status string, details map[string]interface{}) error {
	if e.StepResults == nil {
		e.StepResults = make(map[string]interface{})
	}

	result := map[string]interface{}{
		"status":       status,
		"completed_at": time.Now(),
	}

	// Merge additional details
	for k, v := range details {
		result[k] = v
	}

	e.StepResults[stepName] = result
	e.UpdatedAt = time.Now()

	return db.Save(e).Error
}

// MarkAsCompleted marks the execution as completed successfully.
func (e *DocumentRevisionPipelineExecution) MarkAsCompleted(db *gorm.DB) error {
	now := time.Now()
	e.Status = PipelineStatusCompleted
	e.CompletedAt = &now
	e.UpdatedAt = now

	return db.Save(e).Error
}

// MarkAsFailed marks the execution as failed with error details.
func (e *DocumentRevisionPipelineExecution) MarkAsFailed(db *gorm.DB, stepName string, err error) error {
	now := time.Now()
	e.Status = PipelineStatusFailed
	e.CompletedAt = &now
	e.UpdatedAt = now

	if e.ErrorDetails == nil {
		e.ErrorDetails = make(map[string]interface{})
	}

	e.ErrorDetails["failed_step"] = stepName
	e.ErrorDetails["error"] = err.Error()
	e.ErrorDetails["failed_at"] = now

	return db.Save(e).Error
}

// MarkAsPartial marks the execution as partially completed (some steps failed).
func (e *DocumentRevisionPipelineExecution) MarkAsPartial(db *gorm.DB) error {
	now := time.Now()
	e.Status = PipelineStatusPartial
	e.CompletedAt = &now
	e.UpdatedAt = now

	return db.Save(e).Error
}

// ShouldRetry determines if the execution should be retried.
func (e *DocumentRevisionPipelineExecution) ShouldRetry() bool {
	return e.Status == PipelineStatusFailed && e.AttemptNumber < e.MaxAttempts
}

// Retry increments the attempt counter and resets status for retry.
func (e *DocumentRevisionPipelineExecution) Retry(db *gorm.DB) error {
	e.AttemptNumber++
	e.Status = PipelineStatusPending
	e.StartedAt = nil
	e.CompletedAt = nil
	e.UpdatedAt = time.Now()

	return db.Save(e).Error
}

// GetExecutionsByRevision retrieves all pipeline executions for a revision.
func GetExecutionsByRevision(db *gorm.DB, revisionID uint) ([]DocumentRevisionPipelineExecution, error) {
	var executions []DocumentRevisionPipelineExecution
	err := db.Where("revision_id = ?", revisionID).
		Order("created_at DESC").
		Find(&executions).Error

	return executions, err
}

// GetExecutionsByOutbox retrieves all pipeline executions for an outbox entry.
func GetExecutionsByOutbox(db *gorm.DB, outboxID uint) ([]DocumentRevisionPipelineExecution, error) {
	var executions []DocumentRevisionPipelineExecution
	err := db.Where("outbox_id = ?", outboxID).
		Order("created_at DESC").
		Find(&executions).Error

	return executions, err
}

// GetFailedExecutionsForRetry retrieves failed executions that should be retried.
func GetFailedExecutionsForRetry(db *gorm.DB, limit int) ([]DocumentRevisionPipelineExecution, error) {
	var executions []DocumentRevisionPipelineExecution
	err := db.Where("status = ? AND attempt_number < max_attempts", PipelineStatusFailed).
		Order("updated_at ASC").
		Limit(limit).
		Find(&executions).Error

	return executions, err
}

// GetExecutionStats returns statistics about pipeline executions.
func GetExecutionStats(db *gorm.DB) (map[string]int64, error) {
	stats := make(map[string]int64)

	statuses := []string{
		PipelineStatusPending,
		PipelineStatusRunning,
		PipelineStatusCompleted,
		PipelineStatusFailed,
		PipelineStatusPartial,
	}

	for _, status := range statuses {
		var count int64
		err := db.Model(&DocumentRevisionPipelineExecution{}).
			Where("status = ?", status).
			Count(&count).Error

		if err != nil {
			return nil, err
		}

		stats[status] = count
	}

	return stats, nil
}

// GetAverageDuration calculates average execution duration for completed pipelines.
func GetAverageDuration(db *gorm.DB, rulesetName string) (*time.Duration, error) {
	var result struct {
		AvgDuration float64
	}

	query := db.Model(&DocumentRevisionPipelineExecution{}).
		Select("AVG(EXTRACT(EPOCH FROM (completed_at - started_at))) as avg_duration").
		Where("status IN (?, ?) AND started_at IS NOT NULL AND completed_at IS NOT NULL",
			PipelineStatusCompleted, PipelineStatusPartial)

	if rulesetName != "" {
		query = query.Where("ruleset_name = ?", rulesetName)
	}

	err := query.Scan(&result).Error
	if err != nil {
		return nil, err
	}

	duration := time.Duration(result.AvgDuration * float64(time.Second))
	return &duration, nil
}

// DeleteOldCompletedExecutions removes completed executions older than the specified duration.
func DeleteOldCompletedExecutions(db *gorm.DB, olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan)
	result := db.
		Where("status IN (?, ?) AND completed_at < ?",
			PipelineStatusCompleted, PipelineStatusPartial, cutoff).
		Delete(&DocumentRevisionPipelineExecution{})

	return result.RowsAffected, result.Error
}
