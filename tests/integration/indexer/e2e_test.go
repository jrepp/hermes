package indexer_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/hashicorp-forge/hermes/pkg/indexer/pipeline"
	"github.com/hashicorp-forge/hermes/pkg/indexer/publisher"
	"github.com/hashicorp-forge/hermes/pkg/indexer/ruleset"
	"github.com/hashicorp-forge/hermes/pkg/models"
)

// setupTestDB creates an in-memory SQLite database for testing.
func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// Auto-migrate all tables
	err = db.AutoMigrate(
		&models.DocumentRevision{},
		&models.DocumentRevisionOutbox{},
		&models.DocumentRevisionPipelineExecution{},
		&models.DocumentSummary{},
	)
	require.NoError(t, err)

	return db
}

// MockStep is a mock pipeline step for testing.
type MockStep struct {
	name        string
	executed    bool
	shouldFail  bool
	isRetryable bool
}

func (m *MockStep) Name() string {
	return m.name
}

func (m *MockStep) Execute(ctx context.Context, revision *models.DocumentRevision, config map[string]interface{}) error {
	m.executed = true
	if m.shouldFail {
		return assert.AnError
	}
	return nil
}

func (m *MockStep) IsRetryable(err error) bool {
	return m.isRetryable
}

// TestEndToEnd_PublishAndExecute tests the full flow from publishing to pipeline execution.
func TestEndToEnd_PublishAndExecute(t *testing.T) {
	db := setupTestDB(t)
	logger := hclog.NewNullLogger()
	ctx := context.Background()

	// Step 1: Create publisher
	pub := publisher.New(db, logger)

	// Step 2: Create mock pipeline steps
	searchStep := &MockStep{name: "search_index"}
	llmStep := &MockStep{name: "llm_summary"}

	// Step 3: Create pipeline executor
	exec, err := pipeline.NewExecutor(pipeline.ExecutorConfig{
		DB: db,
		Steps: []pipeline.Step{
			searchStep,
			llmStep,
		},
		Logger: logger,
	})
	require.NoError(t, err)

	// Step 4: Create a document revision and publish event
	docUUID := uuid.New()
	var revision *models.DocumentRevision
	var outboxID uint

	err = db.Transaction(func(tx *gorm.DB) error {
		revision = &models.DocumentRevision{
			DocumentUUID: docUUID,
			DocumentID:   "RFC-088",
			ProviderType: "google",
			Title:        "Event-Driven Indexer",
			ContentHash:  "abc123",
			Status:       "In-Review",
			ModifiedTime: time.Now(),
		}

		if err := tx.Create(revision).Error; err != nil {
			return err
		}

		metadata := map[string]interface{}{
			"document_type": "RFC",
			"product":       "Hermes",
		}

		if err := pub.PublishRevisionCreated(ctx, tx, revision, metadata); err != nil {
			return err
		}

		// Get the outbox entry ID
		var outbox models.DocumentRevisionOutbox
		if err := tx.First(&outbox, "document_uuid = ?", docUUID).Error; err != nil {
			return err
		}
		outboxID = outbox.ID

		return nil
	})
	require.NoError(t, err)

	// Step 5: Verify outbox entry was created
	var outboxEntry models.DocumentRevisionOutbox
	err = db.First(&outboxEntry, outboxID).Error
	require.NoError(t, err)
	assert.Equal(t, models.OutboxStatusPending, outboxEntry.Status)

	// Step 6: Define ruleset that matches our document
	rs := ruleset.Ruleset{
		Name: "rfcs-full-pipeline",
		Conditions: map[string]string{
			"document_type": "RFC",
		},
		Pipeline: []string{"search_index", "llm_summary"},
		Config:   map[string]interface{}{},
	}

	// Step 7: Execute pipeline (simulating consumer)
	err = exec.Execute(ctx, revision, outboxID, &rs)
	require.NoError(t, err)

	// Step 8: Verify pipeline execution was recorded
	var executions []models.DocumentRevisionPipelineExecution
	err = db.Where("revision_id = ?", revision.ID).Find(&executions).Error
	require.NoError(t, err)
	require.Len(t, executions, 1)

	execution := executions[0]
	assert.Equal(t, "rfcs-full-pipeline", execution.RulesetName)
	assert.Equal(t, models.PipelineStatusCompleted, execution.Status)
	assert.NotNil(t, execution.StartedAt)
	assert.NotNil(t, execution.CompletedAt)

	// Step 9: Verify steps were executed
	assert.True(t, searchStep.executed, "search_index step should have executed")
	assert.True(t, llmStep.executed, "llm_summary step should have executed")

	// Step 10: Verify step results were recorded
	assert.NotNil(t, execution.StepResults)
	assert.Contains(t, execution.StepResults, "search_index")
	assert.Contains(t, execution.StepResults, "llm_summary")
}

// TestEndToEnd_RulesetMatching tests that rulesets correctly match documents.
func TestEndToEnd_RulesetMatching(t *testing.T) {
	db := setupTestDB(t)

	// Create document revision
	revision := &models.DocumentRevision{
		DocumentUUID: uuid.New(),
		DocumentID:   "PRD-123",
		ProviderType: "local",
		Title:        "Product Requirements",
		ContentHash:  "xyz789",
		Status:       "draft",
		ModifiedTime: time.Now(),
	}
	require.NoError(t, db.Create(revision).Error)

	// Define multiple rulesets
	rulesets := []ruleset.Ruleset{
		{
			Name:       "rfcs-only",
			Conditions: map[string]string{"document_type": "RFC"},
			Pipeline:   []string{"search_index"},
		},
		{
			Name:       "all-documents",
			Conditions: map[string]string{}, // Matches all
			Pipeline:   []string{"search_index"},
		},
		{
			Name:       "prds-only",
			Conditions: map[string]string{"document_type": "PRD"},
			Pipeline:   []string{"search_index", "llm_summary"},
		},
	}

	// Create matcher
	matcher := ruleset.NewMatcher(rulesets)

	// Match with PRD metadata
	metadata := map[string]interface{}{
		"document_type": "PRD",
	}
	matched := matcher.Match(revision, metadata)

	// Should match "all-documents" and "prds-only"
	require.Len(t, matched, 2)

	matchedNames := make([]string, len(matched))
	for i, rs := range matched {
		matchedNames[i] = rs.Name
	}
	assert.Contains(t, matchedNames, "all-documents")
	assert.Contains(t, matchedNames, "prds-only")
	assert.NotContains(t, matchedNames, "rfcs-only")
}

// TestEndToEnd_PipelineFailure tests handling of pipeline step failures.
func TestEndToEnd_PipelineFailure(t *testing.T) {
	db := setupTestDB(t)
	logger := hclog.NewNullLogger()
	ctx := context.Background()

	// Create failing step
	failingStep := &MockStep{name: "failing_step", shouldFail: true}
	successStep := &MockStep{name: "success_step", shouldFail: false}

	// Create executor
	exec, err := pipeline.NewExecutor(pipeline.ExecutorConfig{
		DB:     db,
		Steps:  []pipeline.Step{failingStep, successStep},
		Logger: logger,
	})
	require.NoError(t, err)

	// Create revision
	revision := &models.DocumentRevision{
		DocumentUUID: uuid.New(),
		DocumentID:   "test-doc",
		ProviderType: "google",
		Title:        "Test",
		ContentHash:  "hash",
		Status:       "active",
		ModifiedTime: time.Now(),
	}
	require.NoError(t, db.Create(revision).Error)

	// Create outbox entry
	outbox := &models.DocumentRevisionOutbox{
		RevisionID:    revision.ID,
		DocumentUUID:  revision.DocumentUUID,
		DocumentID:    revision.DocumentID,
		IdempotentKey: models.GenerateIdempotentKey(revision.DocumentUUID, revision.ContentHash),
		ContentHash:   revision.ContentHash,
		EventType:     models.RevisionEventCreated,
		ProviderType:  revision.ProviderType,
		Payload:       map[string]interface{}{"test": "data"},
		Status:        models.OutboxStatusPending,
	}
	require.NoError(t, db.Create(outbox).Error)

	// Define ruleset with failing step
	rs := ruleset.Ruleset{
		Name:     "test-pipeline",
		Pipeline: []string{"failing_step", "success_step"},
	}

	// Execute pipeline (should fail)
	err = exec.Execute(ctx, revision, outbox.ID, &rs)
	assert.Error(t, err)

	// Verify execution was recorded as failed
	var execution models.DocumentRevisionPipelineExecution
	err = db.First(&execution, "revision_id = ?", revision.ID).Error
	require.NoError(t, err)

	assert.Equal(t, models.PipelineStatusFailed, execution.Status)
	assert.NotNil(t, execution.ErrorDetails)
	assert.Contains(t, execution.ErrorDetails, "failed_step")
}

// TestEndToEnd_Idempotency tests that the same event is not processed twice.
func TestEndToEnd_Idempotency(t *testing.T) {
	db := setupTestDB(t)
	logger := hclog.NewNullLogger()
	ctx := context.Background()

	// Create publisher
	pub := publisher.New(db, logger)

	// Create revision and publish twice
	docUUID := uuid.New()
	revision := &models.DocumentRevision{
		DocumentUUID: docUUID,
		DocumentID:   "test-doc",
		ProviderType: "google",
		Title:        "Test",
		ContentHash:  "samehash",
		Status:       "active",
		ModifiedTime: time.Now(),
	}

	// First publish
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(revision).Error; err != nil {
			return err
		}
		return pub.PublishRevisionCreated(ctx, tx, revision, nil)
	})
	require.NoError(t, err)

	// Second publish (should be idempotent)
	err = db.Transaction(func(tx *gorm.DB) error {
		return pub.PublishRevisionCreated(ctx, tx, revision, nil)
	})
	require.NoError(t, err)

	// Should only have one outbox entry
	var count int64
	err = db.Model(&models.DocumentRevisionOutbox{}).
		Where("document_uuid = ?", docUUID).
		Count(&count).Error
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)
}
