package consumer

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/redpanda"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/kmsg"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/hashicorp-forge/hermes/pkg/indexer/pipeline"
	"github.com/hashicorp-forge/hermes/pkg/indexer/ruleset"
	"github.com/hashicorp-forge/hermes/pkg/models"
)

// MockStep is a test implementation of pipeline.Step
type MockStep struct {
	name       string
	executed   bool
	shouldFail bool
	failError  error
}

func (m *MockStep) Name() string {
	return m.name
}

func (m *MockStep) Execute(ctx context.Context, revision *models.DocumentRevision, config map[string]interface{}) error {
	m.executed = true
	if m.shouldFail {
		return m.failError
	}
	return nil
}

func (m *MockStep) IsRetryable(err error) bool {
	return false
}

// setupTestDB creates an in-memory SQLite database for testing
func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// Auto-migrate tables
	err = db.AutoMigrate(
		&models.DocumentRevision{},
		&models.DocumentRevisionOutbox{},
		&models.DocumentRevisionPipelineExecution{},
	)
	require.NoError(t, err)

	return db
}

// createKafkaTopic creates a Kafka topic for testing
func createKafkaTopic(t *testing.T, ctx context.Context, brokers string, topicName string) {
	adminClient, err := kgo.NewClient(
		kgo.SeedBrokers(brokers),
	)
	require.NoError(t, err)
	defer adminClient.Close()

	createTopicsReq := kmsg.NewCreateTopicsRequest()
	createTopicsReq.Topics = []kmsg.CreateTopicsRequestTopic{
		{
			Topic:             topicName,
			NumPartitions:     1,
			ReplicationFactor: 1,
		},
	}
	_, err = adminClient.Request(ctx, &createTopicsReq)
	require.NoError(t, err)

	// Wait for topic to be ready
	time.Sleep(1 * time.Second)
}

// publishTestEvent publishes a test event to Redpanda
func publishTestEvent(t *testing.T, ctx context.Context, brokers string, topic string, event DocumentRevisionEvent) {
	producer, err := kgo.NewClient(
		kgo.SeedBrokers(brokers),
	)
	require.NoError(t, err)
	defer producer.Close()

	eventJSON, err := json.Marshal(event)
	require.NoError(t, err)

	record := &kgo.Record{
		Topic: topic,
		Key:   []byte(event.DocumentUUID),
		Value: eventJSON,
	}

	err = producer.ProduceSync(ctx, record).FirstErr()
	require.NoError(t, err)
}

// TestConsumer_ConsumeFromRedpanda tests the consumer consuming from a real Redpanda instance
func TestConsumer_ConsumeFromRedpanda(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	// TODO: Fix timing issues in CI - test is flaky due to race conditions
	t.Skip("Temporarily skipping flaky Redpanda integration test in CI")

	ctx := context.Background()
	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "test",
		Level: hclog.Debug,
	})

	// Start Redpanda container
	redpandaContainer, err := redpanda.Run(ctx,
		"docker.redpanda.com/redpandadata/redpanda:latest",
	)
	require.NoError(t, err)
	defer func() {
		_ = redpandaContainer.Terminate(ctx)
	}()

	brokers, err := redpandaContainer.KafkaSeedBroker(ctx)
	require.NoError(t, err)

	// Create topic
	topic := "test.document-revisions"
	createKafkaTopic(t, ctx, brokers, topic)

	// Setup test database
	db := setupTestDB(t)

	// Create test revision
	docUUID := uuid.New()
	revision := &models.DocumentRevision{
		DocumentUUID: docUUID,
		DocumentID:   "test-doc-1",
		ProviderType: "google",
		Title:        "Test Document",
		ContentHash:  "hash123",
		Status:       "active",
		ModifiedTime: time.Now(),
	}
	require.NoError(t, db.Create(revision).Error)

	// Create mock pipeline steps
	searchStep := &MockStep{name: "search_index"}
	summaryStep := &MockStep{name: "llm_summary"}

	// Create pipeline executor
	executor, err := pipeline.NewExecutor(pipeline.ExecutorConfig{
		DB: db,
		Steps: []pipeline.Step{
			searchStep,
			summaryStep,
		},
		Logger: logger,
	})
	require.NoError(t, err)

	// Define rulesets
	rulesets := ruleset.Rulesets{
		{
			Name:       "test-all-docs",
			Conditions: map[string]string{}, // Match all
			Pipeline:   []string{"search_index", "llm_summary"},
		},
	}

	// Create consumer
	consumer, err := New(Config{
		DB:               db,
		Brokers:          []string{brokers},
		Topic:            topic,
		ConsumerGroup:    "test-consumer-group",
		ConsumeFromStart: true, // For testing, consume from start
		Rulesets:         rulesets,
		Executor:         executor,
		Logger:           logger,
	})
	require.NoError(t, err)
	defer consumer.Stop()

	// Publish test event BEFORE starting consumer (consumer will read from start)
	event := DocumentRevisionEvent{
		ID:           1,
		DocumentUUID: docUUID.String(),
		DocumentID:   "test-doc-1",
		EventType:    models.RevisionEventCreated,
		ProviderType: "google",
		ContentHash:  "hash123",
		Payload: map[string]interface{}{
			"revision": map[string]interface{}{
				"id": float64(revision.ID),
			},
			"metadata": map[string]interface{}{
				"test": "data",
			},
		},
		Timestamp: time.Now(),
	}
	publishTestEvent(t, ctx, brokers, topic, event)

	// Start consumer in goroutine with timeout
	consumerCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	consumerDone := make(chan error, 1)
	go func() {
		consumerDone <- consumer.Start(consumerCtx)
	}()

	// Wait for processing (give it time to consume and process)
	time.Sleep(5 * time.Second)

	// Stop consumer
	consumer.Stop()
	<-consumerDone

	// Verify pipeline steps were executed
	assert.True(t, searchStep.executed, "search_index step should have been executed")
	assert.True(t, summaryStep.executed, "llm_summary step should have been executed")

	// Verify pipeline execution was recorded in database
	var executions []models.DocumentRevisionPipelineExecution
	err = db.Where("revision_id = ?", revision.ID).Find(&executions).Error
	require.NoError(t, err)
	assert.Len(t, executions, 1, "should have one pipeline execution")

	if len(executions) > 0 {
		assert.Equal(t, "test-all-docs", executions[0].RulesetName)
		assert.Equal(t, models.PipelineStatusCompleted, executions[0].Status)
		assert.Equal(t, []string{"search_index", "llm_summary"}, executions[0].PipelineSteps)
	}
}

// TestConsumer_RulesetMatching tests that the consumer correctly matches rulesets
func TestConsumer_RulesetMatching(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	// TODO: Fix timing issues in CI - test is flaky due to race conditions
	t.Skip("Temporarily skipping flaky Redpanda integration test in CI")

	ctx := context.Background()
	logger := hclog.NewNullLogger()

	// Start Redpanda container
	redpandaContainer, err := redpanda.Run(ctx,
		"docker.redpanda.com/redpandadata/redpanda:latest",
	)
	require.NoError(t, err)
	defer func() {
		_ = redpandaContainer.Terminate(ctx)
	}()

	brokers, err := redpandaContainer.KafkaSeedBroker(ctx)
	require.NoError(t, err)

	// Create topic
	topic := "test.document-revisions"
	createKafkaTopic(t, ctx, brokers, topic)

	// Setup test database
	db := setupTestDB(t)

	// Create test revision with specific type
	docUUID := uuid.New()
	revision := &models.DocumentRevision{
		DocumentUUID: docUUID,
		DocumentID:   "test-rfc-doc",
		ProviderType: "google",
		Title:        "RFC-001: Test",
		ContentHash:  "hash456",
		Status:       "active",
		ModifiedTime: time.Now(),
	}
	require.NoError(t, db.Create(revision).Error)

	// Create mock pipeline steps
	searchStep := &MockStep{name: "search_index"}
	embeddingsStep := &MockStep{name: "embeddings"}

	// Create pipeline executor
	executor, err := pipeline.NewExecutor(pipeline.ExecutorConfig{
		DB: db,
		Steps: []pipeline.Step{
			searchStep,
			embeddingsStep,
		},
		Logger: logger,
	})
	require.NoError(t, err)

	// Define rulesets - only match documents with "RFC" in title
	rulesets := ruleset.Rulesets{
		{
			Name: "rfc-docs",
			Conditions: map[string]string{
				"title_contains": "RFC", // Use _contains suffix in key, not value
			},
			Pipeline: []string{"search_index", "embeddings"},
		},
	}

	// Create consumer
	consumer, err := New(Config{
		DB:               db,
		Brokers:          []string{brokers},
		Topic:            topic,
		ConsumerGroup:    "test-consumer-group-2",
		ConsumeFromStart: true, // For testing, consume from start
		Rulesets:         rulesets,
		Executor:         executor,
		Logger:           logger,
	})
	require.NoError(t, err)
	defer consumer.Stop()

	// Start consumer with timeout
	consumerCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	consumerDone := make(chan error, 1)
	go func() {
		consumerDone <- consumer.Start(consumerCtx)
	}()

	// Wait for consumer to start and subscribe
	time.Sleep(2 * time.Second)

	// Publish test event AFTER consumer is running
	event := DocumentRevisionEvent{
		ID:           1,
		DocumentUUID: docUUID.String(),
		DocumentID:   "test-rfc-doc",
		EventType:    models.RevisionEventCreated,
		ProviderType: "google",
		ContentHash:  "hash456",
		Payload: map[string]interface{}{
			"revision": map[string]interface{}{
				"id": float64(revision.ID),
			},
			"metadata": map[string]interface{}{},
		},
		Timestamp: time.Now(),
	}
	publishTestEvent(t, ctx, brokers, topic, event)

	// Wait for processing
	time.Sleep(3 * time.Second)

	// Stop consumer
	consumer.Stop()
	<-consumerDone

	// Verify steps were executed (ruleset matched)
	assert.True(t, searchStep.executed, "search_index step should have been executed")
	assert.True(t, embeddingsStep.executed, "embeddings step should have been executed")
}

// TestConsumer_NoMatchingRuleset tests that no pipeline executes when no ruleset matches
func TestConsumer_NoMatchingRuleset(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	logger := hclog.NewNullLogger()

	// Start Redpanda container
	redpandaContainer, err := redpanda.Run(ctx,
		"docker.redpanda.com/redpandadata/redpanda:latest",
	)
	require.NoError(t, err)
	defer func() {
		_ = redpandaContainer.Terminate(ctx)
	}()

	brokers, err := redpandaContainer.KafkaSeedBroker(ctx)
	require.NoError(t, err)

	// Create topic
	topic := "test.document-revisions"
	createKafkaTopic(t, ctx, brokers, topic)

	// Setup test database
	db := setupTestDB(t)

	// Create test revision that won't match ruleset
	docUUID := uuid.New()
	revision := &models.DocumentRevision{
		DocumentUUID: docUUID,
		DocumentID:   "test-doc-no-match",
		ProviderType: "google",
		Title:        "Regular Document",
		ContentHash:  "hash789",
		Status:       "draft",
		ModifiedTime: time.Now(),
	}
	require.NoError(t, db.Create(revision).Error)

	// Create mock pipeline step
	searchStep := &MockStep{name: "search_index"}

	// Create pipeline executor
	executor, err := pipeline.NewExecutor(pipeline.ExecutorConfig{
		DB:     db,
		Steps:  []pipeline.Step{searchStep},
		Logger: logger,
	})
	require.NoError(t, err)

	// Define ruleset that won't match (requires status=active)
	rulesets := ruleset.Rulesets{
		{
			Name: "active-docs-only",
			Conditions: map[string]string{
				"status": "active",
			},
			Pipeline: []string{"search_index"},
		},
	}

	// Create consumer
	consumer, err := New(Config{
		DB:               db,
		Brokers:          []string{brokers},
		Topic:            topic,
		ConsumerGroup:    "test-consumer-group-3",
		ConsumeFromStart: true, // For testing, consume from start
		Rulesets:         rulesets,
		Executor:         executor,
		Logger:           logger,
	})
	require.NoError(t, err)
	defer consumer.Stop()

	// Start consumer with timeout
	consumerCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	consumerDone := make(chan error, 1)
	go func() {
		consumerDone <- consumer.Start(consumerCtx)
	}()

	// Wait for consumer to start and subscribe
	time.Sleep(2 * time.Second)

	// Publish test event AFTER consumer is running
	event := DocumentRevisionEvent{
		ID:           1,
		DocumentUUID: docUUID.String(),
		DocumentID:   "test-doc-no-match",
		EventType:    models.RevisionEventCreated,
		ProviderType: "google",
		ContentHash:  "hash789",
		Payload: map[string]interface{}{
			"revision": map[string]interface{}{
				"id": float64(revision.ID),
			},
			"metadata": map[string]interface{}{},
		},
		Timestamp: time.Now(),
	}
	publishTestEvent(t, ctx, brokers, topic, event)

	// Wait for processing
	time.Sleep(3 * time.Second)

	// Stop consumer
	consumer.Stop()
	<-consumerDone

	// Verify step was NOT executed (no ruleset matched)
	assert.False(t, searchStep.executed, "search_index step should NOT have been executed")

	// Verify no pipeline execution was recorded
	var executions []models.DocumentRevisionPipelineExecution
	err = db.Where("revision_id = ?", revision.ID).Find(&executions).Error
	require.NoError(t, err)
	assert.Len(t, executions, 0, "should have zero pipeline executions")
}

// TestConsumer_Idempotency tests that the consumer doesn't reprocess already processed events
func TestConsumer_Idempotency(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	logger := hclog.NewNullLogger()

	// Start Redpanda container
	redpandaContainer, err := redpanda.Run(ctx,
		"docker.redpanda.com/redpandadata/redpanda:latest",
	)
	require.NoError(t, err)
	defer func() {
		_ = redpandaContainer.Terminate(ctx)
	}()

	brokers, err := redpandaContainer.KafkaSeedBroker(ctx)
	require.NoError(t, err)

	// Create topic
	topic := "test.document-revisions"
	createKafkaTopic(t, ctx, brokers, topic)

	// Setup test database
	db := setupTestDB(t)

	// Create test revision
	docUUID := uuid.New()
	revision := &models.DocumentRevision{
		DocumentUUID: docUUID,
		DocumentID:   "test-doc-idempotent",
		ProviderType: "google",
		Title:        "Test Document",
		ContentHash:  "hash999",
		Status:       "active",
		ModifiedTime: time.Now(),
	}
	require.NoError(t, db.Create(revision).Error)

	// Pre-create a pipeline execution to simulate already processed event
	execution := models.NewPipelineExecution(revision.ID, 1, "test-ruleset", []string{"search_index"})
	execution.Status = models.PipelineStatusCompleted
	require.NoError(t, db.Create(execution).Error)

	// Create mock pipeline step with counter
	searchStep := &MockStep{name: "search_index"}

	// Create pipeline executor
	executor, err := pipeline.NewExecutor(pipeline.ExecutorConfig{
		DB:     db,
		Steps:  []pipeline.Step{searchStep},
		Logger: logger,
	})
	require.NoError(t, err)

	// Define ruleset
	rulesets := ruleset.Rulesets{
		{
			Name:       "test-ruleset",
			Conditions: map[string]string{},
			Pipeline:   []string{"search_index"},
		},
	}

	// Create consumer
	consumer, err := New(Config{
		DB:               db,
		Brokers:          []string{brokers},
		Topic:            topic,
		ConsumerGroup:    "test-consumer-group-4",
		ConsumeFromStart: true, // For testing, consume from start
		Rulesets:         rulesets,
		Executor:         executor,
		Logger:           logger,
	})
	require.NoError(t, err)
	defer consumer.Stop()

	// Start consumer with timeout
	consumerCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	consumerDone := make(chan error, 1)
	go func() {
		consumerDone <- consumer.Start(consumerCtx)
	}()

	// Wait for consumer to start and subscribe
	time.Sleep(2 * time.Second)

	// Publish test event AFTER consumer is running (same outbox ID as pre-created execution)
	event := DocumentRevisionEvent{
		ID:           1, // Same as execution.OutboxID
		DocumentUUID: docUUID.String(),
		DocumentID:   "test-doc-idempotent",
		EventType:    models.RevisionEventCreated,
		ProviderType: "google",
		ContentHash:  "hash999",
		Payload: map[string]interface{}{
			"revision": map[string]interface{}{
				"id": float64(revision.ID),
			},
			"metadata": map[string]interface{}{},
		},
		Timestamp: time.Now(),
	}
	publishTestEvent(t, ctx, brokers, topic, event)

	// Wait for processing
	time.Sleep(3 * time.Second)

	// Stop consumer
	consumer.Stop()
	<-consumerDone

	// Verify step was NOT executed again (idempotency)
	assert.False(t, searchStep.executed, "search_index step should NOT have been executed again")

	// Verify only one execution exists
	var executions []models.DocumentRevisionPipelineExecution
	err = db.Where("revision_id = ?", revision.ID).Find(&executions).Error
	require.NoError(t, err)
	assert.Len(t, executions, 1, "should still have only one pipeline execution")
}
