package pipeline

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp-forge/hermes/pkg/indexer/ruleset"
	"github.com/hashicorp-forge/hermes/pkg/models"
	"github.com/hashicorp/go-hclog"
	"gorm.io/gorm"
)

// Executor executes pipeline steps for document revisions.
type Executor struct {
	steps  map[string]Step
	db     *gorm.DB
	logger hclog.Logger
}

// Step represents a single pipeline step.
type Step interface {
	// Name returns the step name (e.g., "search_index", "embeddings").
	Name() string

	// Execute runs the step for the given revision.
	Execute(ctx context.Context, revision *models.DocumentRevision, config map[string]interface{}) error

	// IsRetryable determines if an error should trigger a retry.
	IsRetryable(err error) bool
}

// ExecutorConfig holds configuration for the executor.
type ExecutorConfig struct {
	DB     *gorm.DB
	Steps  []Step
	Logger hclog.Logger
}

// NewExecutor creates a new pipeline executor.
func NewExecutor(cfg ExecutorConfig) (*Executor, error) {
	// Note: DB is optional. If not provided, execution tracking is skipped (stateless mode).
	if cfg.Logger == nil {
		cfg.Logger = hclog.NewNullLogger()
	}

	// Build steps map
	steps := make(map[string]Step)
	for _, step := range cfg.Steps {
		steps[step.Name()] = step
	}

	return &Executor{
		steps:  steps,
		db:     cfg.DB,
		logger: cfg.Logger.Named("pipeline-executor"),
	}, nil
}

// Execute executes a pipeline for a document revision based on the matched ruleset.
func (e *Executor) Execute(ctx context.Context, revision *models.DocumentRevision, outboxID uint, rs *ruleset.Ruleset) error {
	e.logger.Info("executing pipeline",
		"ruleset", rs.Name,
		"document_uuid", revision.DocumentUUID,
		"revision_id", revision.ID,
		"steps", rs.Pipeline,
	)

	// Create pipeline execution record (only if database is available)
	var execution *models.DocumentRevisionPipelineExecution
	if e.db != nil {
		execution = models.NewPipelineExecution(revision.ID, outboxID, rs.Name, rs.Pipeline)
		if err := e.db.Create(execution).Error; err != nil {
			return fmt.Errorf("failed to create pipeline execution: %w", err)
		}

		// Mark as running
		if err := execution.Start(e.db); err != nil {
			return fmt.Errorf("failed to mark execution as running: %w", err)
		}
	}

	// Execute each step in order
	allSucceeded := true
	var firstError error

	for _, stepName := range rs.Pipeline {
		step, ok := e.steps[stepName]
		if !ok {
			err := fmt.Errorf("unknown pipeline step: %s", stepName)
			if e.db != nil && execution != nil {
				execution.MarkAsFailed(e.db, stepName, err)
			}
			return err
		}

		// Get step-specific config from ruleset
		stepConfig := rs.GetStepConfig(stepName)

		// Execute the step
		stepStart := time.Now()
		err := step.Execute(ctx, revision, stepConfig)
		stepDuration := time.Since(stepStart)

		if err != nil {
			e.logger.Error("pipeline step failed",
				"step", stepName,
				"ruleset", rs.Name,
				"document_uuid", revision.DocumentUUID,
				"error", err,
			)

			// Record step failure (only if database is available)
			if e.db != nil && execution != nil {
				execution.RecordStepResult(e.db, stepName, models.StepStatusFailed, map[string]interface{}{
					"error":       err.Error(),
					"duration_ms": stepDuration.Milliseconds(),
				})
			}

			allSucceeded = false
			if firstError == nil {
				firstError = err
			}

			// Check if we should continue or fail fast
			if !step.IsRetryable(err) {
				// Permanent failure, stop pipeline
				if e.db != nil && execution != nil {
					execution.MarkAsFailed(e.db, stepName, err)
				}
				return fmt.Errorf("pipeline failed at step %s: %w", stepName, err)
			}

			// Continue to next step for retryable errors
			continue
		}

		// Record step success
		e.logger.Debug("pipeline step succeeded",
			"step", stepName,
			"ruleset", rs.Name,
			"document_uuid", revision.DocumentUUID,
			"duration_ms", stepDuration.Milliseconds(),
		)

		if e.db != nil && execution != nil {
			execution.RecordStepResult(e.db, stepName, models.StepStatusSuccess, map[string]interface{}{
				"duration_ms": stepDuration.Milliseconds(),
			})
		}
	}

	// Mark execution as completed or partial (only if database is available)
	if allSucceeded {
		if e.db != nil && execution != nil {
			if err := execution.MarkAsCompleted(e.db); err != nil {
				return fmt.Errorf("failed to mark execution as completed: %w", err)
			}
		}

		e.logger.Info("pipeline completed successfully",
			"ruleset", rs.Name,
			"document_uuid", revision.DocumentUUID,
			"steps", len(rs.Pipeline),
		)

		return nil
	}

	// Some steps failed but we continued (partial success)
	if e.db != nil && execution != nil {
		if err := execution.MarkAsPartial(e.db); err != nil {
			return fmt.Errorf("failed to mark execution as partial: %w", err)
		}
	}

	e.logger.Warn("pipeline completed with failures",
		"ruleset", rs.Name,
		"document_uuid", revision.DocumentUUID,
		"error", firstError,
	)

	return firstError
}

// ExecuteMultiple executes pipelines for multiple matched rulesets.
// Each ruleset is executed independently.
func (e *Executor) ExecuteMultiple(ctx context.Context, revision *models.DocumentRevision, outboxID uint, rulesets []ruleset.Ruleset) []error {
	var errors []error

	for _, rs := range rulesets {
		if err := e.Execute(ctx, revision, outboxID, &rs); err != nil {
			errors = append(errors, fmt.Errorf("ruleset %s: %w", rs.Name, err))
		}
	}

	return errors
}

// RegisterStep registers a new pipeline step.
func (e *Executor) RegisterStep(step Step) {
	e.steps[step.Name()] = step
	e.logger.Debug("registered pipeline step", "step", step.Name())
}

// UnregisterStep removes a pipeline step.
func (e *Executor) UnregisterStep(stepName string) {
	delete(e.steps, stepName)
	e.logger.Debug("unregistered pipeline step", "step", stepName)
}

// GetRegisteredSteps returns the names of all registered steps.
func (e *Executor) GetRegisteredSteps() []string {
	names := make([]string, 0, len(e.steps))
	for name := range e.steps {
		names = append(names, name)
	}
	return names
}

// StepContext provides context information for step execution.
type StepContext struct {
	Revision  *models.DocumentRevision
	Config    map[string]interface{}
	Logger    hclog.Logger
	DB        *gorm.DB
	StartTime time.Time
}

// NewStepContext creates a new step context.
func NewStepContext(revision *models.DocumentRevision, config map[string]interface{}, db *gorm.DB, logger hclog.Logger) *StepContext {
	return &StepContext{
		Revision:  revision,
		Config:    config,
		DB:        db,
		Logger:    logger,
		StartTime: time.Now(),
	}
}

// Elapsed returns the elapsed time since step start.
func (sc *StepContext) Elapsed() time.Duration {
	return time.Since(sc.StartTime)
}

// GetConfigString retrieves a string configuration value.
func (sc *StepContext) GetConfigString(key, defaultVal string) string {
	if sc.Config == nil {
		return defaultVal
	}

	if val, ok := sc.Config[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}

	return defaultVal
}

// GetConfigInt retrieves an integer configuration value.
func (sc *StepContext) GetConfigInt(key string, defaultVal int) int {
	if sc.Config == nil {
		return defaultVal
	}

	if val, ok := sc.Config[key]; ok {
		switch v := val.(type) {
		case int:
			return v
		case float64:
			return int(v)
		}
	}

	return defaultVal
}

// GetConfigBool retrieves a boolean configuration value.
func (sc *StepContext) GetConfigBool(key string, defaultVal bool) bool {
	if sc.Config == nil {
		return defaultVal
	}

	if val, ok := sc.Config[key]; ok {
		if b, ok := val.(bool); ok {
			return b
		}
	}

	return defaultVal
}

// GetConfigMap retrieves a map configuration value.
func (sc *StepContext) GetConfigMap(key string) map[string]interface{} {
	if sc.Config == nil {
		return nil
	}

	if val, ok := sc.Config[key]; ok {
		if m, ok := val.(map[string]interface{}); ok {
			return m
		}
	}

	return nil
}
