package pipeline

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/hashicorp-forge/hermes/pkg/indexer/ruleset"
	"github.com/hashicorp-forge/hermes/pkg/models"
)

// MockStep is a test implementation of the Step interface
type MockStep struct {
	name         string
	executed     bool
	shouldFail   bool
	failError    error
	isRetryable  bool
	execDuration time.Duration
}

func (m *MockStep) Name() string {
	return m.name
}

func (m *MockStep) Execute(ctx context.Context, revision *models.DocumentRevision, config map[string]interface{}) error {
	if m.execDuration > 0 {
		time.Sleep(m.execDuration)
	}
	m.executed = true
	if m.shouldFail {
		return m.failError
	}
	return nil
}

func (m *MockStep) IsRetryable(err error) bool {
	return m.isRetryable
}

// setupTestDB creates an in-memory SQLite database for testing
func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// Auto-migrate tables
	err = db.AutoMigrate(
		&models.DocumentRevision{},
		&models.DocumentRevisionPipelineExecution{},
	)
	require.NoError(t, err)

	return db
}

// createTestRevision creates a test document revision
func createTestRevision(t *testing.T, db *gorm.DB) *models.DocumentRevision {
	revision := &models.DocumentRevision{
		DocumentUUID: uuid.New(),
		DocumentID:   "test-doc-1",
		ProviderType: "google",
		Title:        "Test Document",
		ContentHash:  "hash123",
		Status:       "active",
		ModifiedTime: time.Now(),
	}
	require.NoError(t, db.Create(revision).Error)
	return revision
}

func TestNewExecutor_Success(t *testing.T) {
	db := setupTestDB(t)

	step1 := &MockStep{name: "step1"}
	step2 := &MockStep{name: "step2"}

	executor, err := NewExecutor(ExecutorConfig{
		DB:     db,
		Steps:  []Step{step1, step2},
		Logger: hclog.NewNullLogger(),
	})

	require.NoError(t, err)
	require.NotNil(t, executor)
	assert.Len(t, executor.steps, 2)
	assert.NotNil(t, executor.steps["step1"])
	assert.NotNil(t, executor.steps["step2"])
}

func TestNewExecutor_MissingDB(t *testing.T) {
	step := &MockStep{name: "step1"}

	executor, err := NewExecutor(ExecutorConfig{
		DB:    nil,
		Steps: []Step{step},
	})

	require.Error(t, err)
	assert.Nil(t, executor)
	assert.Contains(t, err.Error(), "database is required")
}

func TestNewExecutor_NoLogger(t *testing.T) {
	db := setupTestDB(t)

	executor, err := NewExecutor(ExecutorConfig{
		DB:     db,
		Steps:  []Step{},
		Logger: nil, // Should use null logger
	})

	require.NoError(t, err)
	assert.NotNil(t, executor)
	assert.NotNil(t, executor.logger)
}

func TestExecutor_Execute_Success(t *testing.T) {
	db := setupTestDB(t)
	revision := createTestRevision(t, db)

	step1 := &MockStep{name: "step1"}
	step2 := &MockStep{name: "step2"}

	executor, err := NewExecutor(ExecutorConfig{
		DB:     db,
		Steps:  []Step{step1, step2},
		Logger: hclog.NewNullLogger(),
	})
	require.NoError(t, err)

	rs := &ruleset.Ruleset{
		Name:     "test-ruleset",
		Pipeline: []string{"step1", "step2"},
	}

	ctx := context.Background()
	err = executor.Execute(ctx, revision, 1, rs)

	require.NoError(t, err)
	assert.True(t, step1.executed, "step1 should have been executed")
	assert.True(t, step2.executed, "step2 should have been executed")

	// Verify pipeline execution was recorded
	var executions []models.DocumentRevisionPipelineExecution
	err = db.Where("revision_id = ?", revision.ID).Find(&executions).Error
	require.NoError(t, err)
	require.Len(t, executions, 1)
	assert.Equal(t, "test-ruleset", executions[0].RulesetName)
	assert.Equal(t, models.PipelineStatusCompleted, executions[0].Status)
}

func TestExecutor_Execute_StepFailure_NonRetryable(t *testing.T) {
	db := setupTestDB(t)
	revision := createTestRevision(t, db)

	step1 := &MockStep{name: "step1"}
	step2 := &MockStep{
		name:        "step2",
		shouldFail:  true,
		failError:   errors.New("permanent failure"),
		isRetryable: false, // Non-retryable error
	}
	step3 := &MockStep{name: "step3"}

	executor, err := NewExecutor(ExecutorConfig{
		DB:     db,
		Steps:  []Step{step1, step2, step3},
		Logger: hclog.NewNullLogger(),
	})
	require.NoError(t, err)

	rs := &ruleset.Ruleset{
		Name:     "test-ruleset",
		Pipeline: []string{"step1", "step2", "step3"},
	}

	ctx := context.Background()
	err = executor.Execute(ctx, revision, 1, rs)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "permanent failure")
	assert.True(t, step1.executed, "step1 should have been executed")
	assert.True(t, step2.executed, "step2 should have been executed")
	assert.False(t, step3.executed, "step3 should NOT have been executed (fail-fast)")

	// Verify pipeline execution was marked as failed
	var executions []models.DocumentRevisionPipelineExecution
	err = db.Where("revision_id = ?", revision.ID).Find(&executions).Error
	require.NoError(t, err)
	require.Len(t, executions, 1, "should have exactly one execution record")

	assert.Equal(t, models.PipelineStatusFailed, executions[0].Status)
}

func TestExecutor_Execute_StepFailure_Retryable(t *testing.T) {
	db := setupTestDB(t)
	revision := createTestRevision(t, db)

	step1 := &MockStep{name: "step1"}
	step2 := &MockStep{
		name:        "step2",
		shouldFail:  true,
		failError:   errors.New("retryable failure"),
		isRetryable: true, // Retryable error
	}
	step3 := &MockStep{name: "step3"}

	executor, err := NewExecutor(ExecutorConfig{
		DB:     db,
		Steps:  []Step{step1, step2, step3},
		Logger: hclog.NewNullLogger(),
	})
	require.NoError(t, err)

	rs := &ruleset.Ruleset{
		Name:     "test-ruleset",
		Pipeline: []string{"step1", "step2", "step3"},
	}

	ctx := context.Background()
	err = executor.Execute(ctx, revision, 1, rs)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "retryable failure")
	assert.True(t, step1.executed, "step1 should have been executed")
	assert.True(t, step2.executed, "step2 should have been executed")
	assert.True(t, step3.executed, "step3 SHOULD have been executed (continue on retryable error)")

	// Verify pipeline execution was marked as partial (some succeeded, some failed)
	var executions []models.DocumentRevisionPipelineExecution
	err = db.Where("revision_id = ?", revision.ID).Find(&executions).Error
	require.NoError(t, err)
	require.Len(t, executions, 1)
	assert.Equal(t, models.PipelineStatusPartial, executions[0].Status)
}

func TestExecutor_Execute_UnknownStep(t *testing.T) {
	db := setupTestDB(t)
	revision := createTestRevision(t, db)

	step1 := &MockStep{name: "step1"}

	executor, err := NewExecutor(ExecutorConfig{
		DB:     db,
		Steps:  []Step{step1},
		Logger: hclog.NewNullLogger(),
	})
	require.NoError(t, err)

	rs := &ruleset.Ruleset{
		Name:     "test-ruleset",
		Pipeline: []string{"step1", "unknown_step"}, // unknown_step not registered
	}

	ctx := context.Background()
	err = executor.Execute(ctx, revision, 1, rs)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown pipeline step")
	assert.True(t, step1.executed, "step1 should have been executed before error")
}

// ConfigCapturingStep is a test step that captures the config it receives
type ConfigCapturingStep struct {
	name           string
	receivedConfig map[string]interface{}
}

func (c *ConfigCapturingStep) Name() string {
	return c.name
}

func (c *ConfigCapturingStep) Execute(ctx context.Context, revision *models.DocumentRevision, config map[string]interface{}) error {
	c.receivedConfig = config
	return nil
}

func (c *ConfigCapturingStep) IsRetryable(err error) bool {
	return false
}

func TestExecutor_Execute_WithStepConfig(t *testing.T) {
	db := setupTestDB(t)
	revision := createTestRevision(t, db)

	step1 := &ConfigCapturingStep{name: "step1"}

	executor, err := NewExecutor(ExecutorConfig{
		DB:     db,
		Steps:  []Step{step1},
		Logger: hclog.NewNullLogger(),
	})
	require.NoError(t, err)

	rs := &ruleset.Ruleset{
		Name:     "test-ruleset",
		Pipeline: []string{"step1"},
		Config: map[string]interface{}{
			"step1": map[string]interface{}{
				"model":      "gpt-4o-mini",
				"max_tokens": 500,
			},
		},
	}

	ctx := context.Background()
	err = executor.Execute(ctx, revision, 1, rs)

	require.NoError(t, err)
	require.NotNil(t, step1.receivedConfig)
	assert.Equal(t, "gpt-4o-mini", step1.receivedConfig["model"])
	assert.Equal(t, 500, step1.receivedConfig["max_tokens"])
}

func TestExecutor_ExecuteMultiple_Success(t *testing.T) {
	db := setupTestDB(t)
	revision := createTestRevision(t, db)

	step1 := &MockStep{name: "step1"}
	step2 := &MockStep{name: "step2"}

	executor, err := NewExecutor(ExecutorConfig{
		DB:     db,
		Steps:  []Step{step1, step2},
		Logger: hclog.NewNullLogger(),
	})
	require.NoError(t, err)

	rulesets := []ruleset.Ruleset{
		{
			Name:     "ruleset1",
			Pipeline: []string{"step1"},
		},
		{
			Name:     "ruleset2",
			Pipeline: []string{"step2"},
		},
	}

	ctx := context.Background()
	errs := executor.ExecuteMultiple(ctx, revision, 1, rulesets)

	assert.Len(t, errs, 0, "no errors should be returned")

	// Verify both executions were recorded
	var executions []models.DocumentRevisionPipelineExecution
	err = db.Where("revision_id = ?", revision.ID).Find(&executions).Error
	require.NoError(t, err)
	assert.Len(t, executions, 2)
}

func TestExecutor_ExecuteMultiple_WithErrors(t *testing.T) {
	db := setupTestDB(t)
	revision := createTestRevision(t, db)

	step1 := &MockStep{name: "step1"}
	step2 := &MockStep{
		name:        "step2",
		shouldFail:  true,
		failError:   errors.New("step2 error"),
		isRetryable: false,
	}

	executor, err := NewExecutor(ExecutorConfig{
		DB:     db,
		Steps:  []Step{step1, step2},
		Logger: hclog.NewNullLogger(),
	})
	require.NoError(t, err)

	rulesets := []ruleset.Ruleset{
		{
			Name:     "ruleset1",
			Pipeline: []string{"step1"}, // Should succeed
		},
		{
			Name:     "ruleset2",
			Pipeline: []string{"step2"}, // Should fail
		},
	}

	ctx := context.Background()
	errs := executor.ExecuteMultiple(ctx, revision, 1, rulesets)

	require.Len(t, errs, 1, "one error should be returned")
	assert.Contains(t, errs[0].Error(), "ruleset2")
	assert.Contains(t, errs[0].Error(), "step2 error")
}

func TestExecutor_RegisterStep(t *testing.T) {
	db := setupTestDB(t)

	executor, err := NewExecutor(ExecutorConfig{
		DB:     db,
		Steps:  []Step{},
		Logger: hclog.NewNullLogger(),
	})
	require.NoError(t, err)

	assert.Len(t, executor.steps, 0)

	step := &MockStep{name: "new_step"}
	executor.RegisterStep(step)

	assert.Len(t, executor.steps, 1)
	assert.Equal(t, step, executor.steps["new_step"])
}

func TestExecutor_UnregisterStep(t *testing.T) {
	db := setupTestDB(t)

	step1 := &MockStep{name: "step1"}
	step2 := &MockStep{name: "step2"}

	executor, err := NewExecutor(ExecutorConfig{
		DB:     db,
		Steps:  []Step{step1, step2},
		Logger: hclog.NewNullLogger(),
	})
	require.NoError(t, err)

	assert.Len(t, executor.steps, 2)

	executor.UnregisterStep("step1")

	assert.Len(t, executor.steps, 1)
	assert.Nil(t, executor.steps["step1"])
	assert.NotNil(t, executor.steps["step2"])
}

func TestExecutor_GetRegisteredSteps(t *testing.T) {
	db := setupTestDB(t)

	step1 := &MockStep{name: "step1"}
	step2 := &MockStep{name: "step2"}
	step3 := &MockStep{name: "step3"}

	executor, err := NewExecutor(ExecutorConfig{
		DB:     db,
		Steps:  []Step{step1, step2, step3},
		Logger: hclog.NewNullLogger(),
	})
	require.NoError(t, err)

	steps := executor.GetRegisteredSteps()

	assert.Len(t, steps, 3)
	assert.Contains(t, steps, "step1")
	assert.Contains(t, steps, "step2")
	assert.Contains(t, steps, "step3")
}

func TestStepContext_GetConfigString(t *testing.T) {
	config := map[string]interface{}{
		"string_val": "test-value",
		"int_val":    123,
	}

	ctx := NewStepContext(nil, config, nil, hclog.NewNullLogger())

	// Existing string value
	val := ctx.GetConfigString("string_val", "default")
	assert.Equal(t, "test-value", val)

	// Non-existent key
	val = ctx.GetConfigString("missing_key", "default")
	assert.Equal(t, "default", val)

	// Wrong type (int instead of string)
	val = ctx.GetConfigString("int_val", "default")
	assert.Equal(t, "default", val)

	// Nil config
	ctxNil := NewStepContext(nil, nil, nil, hclog.NewNullLogger())
	val = ctxNil.GetConfigString("any_key", "default")
	assert.Equal(t, "default", val)
}

func TestStepContext_GetConfigInt(t *testing.T) {
	config := map[string]interface{}{
		"int_val":    123,
		"float_val":  456.0,
		"string_val": "not-a-number",
	}

	ctx := NewStepContext(nil, config, nil, hclog.NewNullLogger())

	// Existing int value
	val := ctx.GetConfigInt("int_val", 999)
	assert.Equal(t, 123, val)

	// Float64 value (should convert)
	val = ctx.GetConfigInt("float_val", 999)
	assert.Equal(t, 456, val)

	// Non-existent key
	val = ctx.GetConfigInt("missing_key", 999)
	assert.Equal(t, 999, val)

	// Wrong type (string)
	val = ctx.GetConfigInt("string_val", 999)
	assert.Equal(t, 999, val)

	// Nil config
	ctxNil := NewStepContext(nil, nil, nil, hclog.NewNullLogger())
	val = ctxNil.GetConfigInt("any_key", 999)
	assert.Equal(t, 999, val)
}

func TestStepContext_GetConfigBool(t *testing.T) {
	config := map[string]interface{}{
		"bool_val":   true,
		"string_val": "not-a-bool",
	}

	ctx := NewStepContext(nil, config, nil, hclog.NewNullLogger())

	// Existing bool value
	val := ctx.GetConfigBool("bool_val", false)
	assert.True(t, val)

	// Non-existent key
	val = ctx.GetConfigBool("missing_key", false)
	assert.False(t, val)

	// Wrong type (string)
	val = ctx.GetConfigBool("string_val", true)
	assert.True(t, val)

	// Nil config
	ctxNil := NewStepContext(nil, nil, nil, hclog.NewNullLogger())
	val = ctxNil.GetConfigBool("any_key", true)
	assert.True(t, val)
}

func TestStepContext_GetConfigMap(t *testing.T) {
	nestedMap := map[string]interface{}{
		"key1": "value1",
		"key2": 123,
	}

	config := map[string]interface{}{
		"map_val":    nestedMap,
		"string_val": "not-a-map",
	}

	ctx := NewStepContext(nil, config, nil, hclog.NewNullLogger())

	// Existing map value
	val := ctx.GetConfigMap("map_val")
	require.NotNil(t, val)
	assert.Equal(t, "value1", val["key1"])
	assert.Equal(t, 123, val["key2"])

	// Non-existent key
	val = ctx.GetConfigMap("missing_key")
	assert.Nil(t, val)

	// Wrong type (string)
	val = ctx.GetConfigMap("string_val")
	assert.Nil(t, val)

	// Nil config
	ctxNil := NewStepContext(nil, nil, nil, hclog.NewNullLogger())
	val = ctxNil.GetConfigMap("any_key")
	assert.Nil(t, val)
}

func TestStepContext_Elapsed(t *testing.T) {
	ctx := NewStepContext(nil, nil, nil, hclog.NewNullLogger())

	// Sleep briefly
	time.Sleep(10 * time.Millisecond)

	elapsed := ctx.Elapsed()
	assert.GreaterOrEqual(t, elapsed.Milliseconds(), int64(10))
}

func TestExecutor_Execute_RecordsStepDuration(t *testing.T) {
	db := setupTestDB(t)
	revision := createTestRevision(t, db)

	step := &MockStep{
		name:         "slow_step",
		execDuration: 50 * time.Millisecond,
	}

	executor, err := NewExecutor(ExecutorConfig{
		DB:     db,
		Steps:  []Step{step},
		Logger: hclog.NewNullLogger(),
	})
	require.NoError(t, err)

	rs := &ruleset.Ruleset{
		Name:     "test-ruleset",
		Pipeline: []string{"slow_step"},
	}

	ctx := context.Background()
	err = executor.Execute(ctx, revision, 1, rs)
	require.NoError(t, err)

	// Verify step result includes duration
	var execution models.DocumentRevisionPipelineExecution
	err = db.Where("revision_id = ?", revision.ID).First(&execution).Error
	require.NoError(t, err)

	stepResult := execution.StepResults["slow_step"]
	require.NotNil(t, stepResult)
	resultMap := stepResult.(map[string]interface{})

	durationMs, ok := resultMap["duration_ms"]
	require.True(t, ok)
	assert.GreaterOrEqual(t, int(durationMs.(float64)), 50)
}
