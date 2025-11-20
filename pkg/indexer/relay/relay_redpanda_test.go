package relay

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

	"github.com/hashicorp-forge/hermes/pkg/models"
)

// createKafkaTopic creates a Kafka topic for testing.
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

// TestRelay_PublishToRedpanda tests the relay publishing to a real Redpanda instance.
func TestRelay_PublishToRedpanda(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

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

	// Get broker address
	brokers, err := redpandaContainer.KafkaSeedBroker(ctx)
	require.NoError(t, err)

	// Create topic
	topic := "test.document-revisions"
	createKafkaTopic(t, ctx, brokers, topic)

	// Setup test database
	db := setupTestDB(t)

	// Create test outbox entries
	docUUID := uuid.New()
	revision := &models.DocumentRevision{
		DocumentUUID: docUUID,
		DocumentID:   "test-doc",
		ProviderType: "google",
		Title:        "Test Document",
		ContentHash:  "hash123",
		Status:       "active",
		ModifiedTime: time.Now(),
	}
	require.NoError(t, db.Create(revision).Error)

	// Create outbox entry
	outboxEntry := &models.DocumentRevisionOutbox{
		RevisionID:    revision.ID,
		DocumentUUID:  docUUID,
		DocumentID:    "test-doc",
		IdempotentKey: models.GenerateIdempotentKey(docUUID, "hash123"),
		ContentHash:   "hash123",
		EventType:     models.RevisionEventCreated,
		ProviderType:  "google",
		Payload: map[string]interface{}{
			"test": "data",
			"revision": map[string]interface{}{
				"id": float64(revision.ID),
			},
		},
		Status: models.OutboxStatusPending,
	}
	require.NoError(t, db.Create(outboxEntry).Error)

	// Create relay
	relay, err := New(Config{
		DB:           db,
		Brokers:      []string{brokers},
		Topic:        topic,
		PollInterval: 100 * time.Millisecond,
		BatchSize:    10,
		Logger:       logger,
	})
	require.NoError(t, err)
	defer relay.Stop()

	// Process batch manually (don't start the polling loop)
	err = relay.processBatch(ctx)
	require.NoError(t, err)

	// Verify outbox entry was marked as published
	var reloaded models.DocumentRevisionOutbox
	err = db.First(&reloaded, outboxEntry.ID).Error
	require.NoError(t, err)
	assert.Equal(t, models.OutboxStatusPublished, reloaded.Status)
	assert.NotNil(t, reloaded.PublishedAt)

	// Create consumer to verify message was published
	consumer, err := kgo.NewClient(
		kgo.SeedBrokers(brokers),
		kgo.ConsumeTopics(topic),
		kgo.ConsumerGroup("test-consumer"),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
	)
	require.NoError(t, err)
	defer consumer.Close()

	// Fetch message with timeout
	fetchCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var receivedEvent *DocumentRevisionEvent
	for receivedEvent == nil {
		fetches := consumer.PollFetches(fetchCtx)
		if fetches.IsClientClosed() {
			break
		}
		if err := fetches.Err(); err != nil {
			t.Fatalf("fetch error: %v", err)
		}

		fetches.EachRecord(func(record *kgo.Record) {
			var event DocumentRevisionEvent
			err := json.Unmarshal(record.Value, &event)
			require.NoError(t, err)
			receivedEvent = &event
		})
	}

	// Verify message content
	require.NotNil(t, receivedEvent, "no message received from Redpanda")
	assert.Equal(t, outboxEntry.ID, receivedEvent.ID)
	assert.Equal(t, docUUID.String(), receivedEvent.DocumentUUID)
	assert.Equal(t, "test-doc", receivedEvent.DocumentID)
	assert.Equal(t, models.RevisionEventCreated, receivedEvent.EventType)
	assert.Equal(t, "google", receivedEvent.ProviderType)
	assert.Equal(t, "hash123", receivedEvent.ContentHash)
}

// TestRelay_MultipleBatches tests processing multiple batches of outbox entries.
func TestRelay_MultipleBatches(t *testing.T) {
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

	// Create 5 test outbox entries
	for i := 0; i < 5; i++ {
		docUUID := uuid.New()
		revision := &models.DocumentRevision{
			DocumentUUID: docUUID,
			DocumentID:   "test-doc",
			ProviderType: "google",
			Title:        "Test Document",
			ContentHash:  "hash123",
			Status:       "active",
			ModifiedTime: time.Now(),
		}
		require.NoError(t, db.Create(revision).Error)

		outboxEntry := &models.DocumentRevisionOutbox{
			RevisionID:    revision.ID,
			DocumentUUID:  docUUID,
			DocumentID:    "test-doc",
			IdempotentKey: models.GenerateIdempotentKey(docUUID, "hash123"),
			ContentHash:   "hash123",
			EventType:     models.RevisionEventCreated,
			ProviderType:  "google",
			Payload: map[string]interface{}{
				"test": "data",
			},
			Status: models.OutboxStatusPending,
		}
		require.NoError(t, db.Create(outboxEntry).Error)
	}

	// Create relay with small batch size
	relay, err := New(Config{
		DB:           db,
		Brokers:      []string{brokers},
		Topic:        topic,
		PollInterval: 100 * time.Millisecond,
		BatchSize:    2, // Process 2 at a time
		Logger:       logger,
	})
	require.NoError(t, err)
	defer relay.Stop()

	// Process multiple batches
	for i := 0; i < 3; i++ {
		err = relay.processBatch(ctx)
		require.NoError(t, err)
		time.Sleep(100 * time.Millisecond)
	}

	// Verify all entries were published
	var publishedCount int64
	err = db.Model(&models.DocumentRevisionOutbox{}).
		Where("status = ?", models.OutboxStatusPublished).
		Count(&publishedCount).Error
	require.NoError(t, err)
	assert.Equal(t, int64(5), publishedCount)
}

// TestRelay_FailureHandling tests error handling when Kafka is unavailable.
func TestRelay_FailureHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	logger := hclog.NewNullLogger()

	// Setup test database (no Redpanda - will fail to connect)
	db := setupTestDB(t)

	// Create test outbox entry
	docUUID := uuid.New()
	revision := &models.DocumentRevision{
		DocumentUUID: docUUID,
		DocumentID:   "test-doc",
		ProviderType: "google",
		Title:        "Test Document",
		ContentHash:  "hash123",
		Status:       "active",
		ModifiedTime: time.Now(),
	}
	require.NoError(t, db.Create(revision).Error)

	outboxEntry := &models.DocumentRevisionOutbox{
		RevisionID:    revision.ID,
		DocumentUUID:  docUUID,
		DocumentID:    "test-doc",
		IdempotentKey: models.GenerateIdempotentKey(docUUID, "hash123"),
		ContentHash:   "hash123",
		EventType:     models.RevisionEventCreated,
		ProviderType:  "google",
		Payload: map[string]interface{}{
			"test": "data",
		},
		Status: models.OutboxStatusPending,
	}
	require.NoError(t, db.Create(outboxEntry).Error)

	// Create relay with invalid broker (will fail to publish)
	topic := "test.document-revisions"
	relay, err := New(Config{
		DB:           db,
		Brokers:      []string{"localhost:9999"}, // Non-existent broker
		Topic:        topic,
		PollInterval: 100 * time.Millisecond,
		BatchSize:    10,
		Logger:       logger,
	})
	require.NoError(t, err)
	defer relay.Stop()

	// Try to process batch (should fail but not panic)
	err = relay.processBatch(ctx)
	// processBatch logs errors but doesn't return them
	require.NoError(t, err)

	// Verify outbox entry was marked as failed
	var reloaded models.DocumentRevisionOutbox
	err = db.First(&reloaded, outboxEntry.ID).Error
	require.NoError(t, err)
	assert.Equal(t, models.OutboxStatusFailed, reloaded.Status)
	assert.NotEmpty(t, reloaded.LastError)
	assert.Equal(t, 1, reloaded.PublishAttempts)
}

// TestRelay_RetryFailed tests retrying failed outbox entries.
func TestRelay_RetryFailed(t *testing.T) {
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

	// Create test outbox entry marked as failed
	docUUID := uuid.New()
	revision := &models.DocumentRevision{
		DocumentUUID: docUUID,
		DocumentID:   "test-doc",
		ProviderType: "google",
		Title:        "Test Document",
		ContentHash:  "hash123",
		Status:       "active",
		ModifiedTime: time.Now(),
	}
	require.NoError(t, db.Create(revision).Error)

	outboxEntry := &models.DocumentRevisionOutbox{
		RevisionID:      revision.ID,
		DocumentUUID:    docUUID,
		DocumentID:      "test-doc",
		IdempotentKey:   models.GenerateIdempotentKey(docUUID, "hash123"),
		ContentHash:     "hash123",
		EventType:       models.RevisionEventCreated,
		ProviderType:    "google",
		Payload:         map[string]interface{}{"test": "data"},
		Status:          models.OutboxStatusFailed,
		PublishAttempts: 1,
		LastError:       "previous failure",
	}
	require.NoError(t, db.Create(outboxEntry).Error)

	// Create relay
	relay, err := New(Config{
		DB:           db,
		Brokers:      []string{brokers},
		Topic:        topic,
		PollInterval: 100 * time.Millisecond,
		BatchSize:    10,
		Logger:       logger,
	})
	require.NoError(t, err)
	defer relay.Stop()

	// Retry failed entries
	err = relay.RetryFailed(ctx, 10)
	require.NoError(t, err)

	// Verify entry was published successfully
	var reloaded models.DocumentRevisionOutbox
	err = db.First(&reloaded, outboxEntry.ID).Error
	require.NoError(t, err)
	assert.Equal(t, models.OutboxStatusPublished, reloaded.Status)
	assert.NotNil(t, reloaded.PublishedAt)
}

// TestRelay_CleanupOldEntries_WithRedpanda tests cleanup of old published entries with Redpanda.
func TestRelay_CleanupOldEntries_WithRedpanda(t *testing.T) {
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

	// Create old published entry
	docUUID := uuid.New()
	revision := &models.DocumentRevision{
		DocumentUUID: docUUID,
		DocumentID:   "test-doc",
		ProviderType: "google",
		Title:        "Test Document",
		ContentHash:  "hash123",
		Status:       "active",
		ModifiedTime: time.Now(),
	}
	require.NoError(t, db.Create(revision).Error)

	outboxEntry := &models.DocumentRevisionOutbox{
		RevisionID:    revision.ID,
		DocumentUUID:  docUUID,
		DocumentID:    "test-doc",
		IdempotentKey: models.GenerateIdempotentKey(docUUID, "hash123"),
		ContentHash:   "hash123",
		EventType:     models.RevisionEventCreated,
		ProviderType:  "google",
		Payload:       map[string]interface{}{"test": "data"},
		Status:        models.OutboxStatusPublished,
	}
	require.NoError(t, db.Create(outboxEntry).Error)

	// Set published_at to 8 days ago
	oldTime := time.Now().Add(-8 * 24 * time.Hour)
	err = db.Model(&outboxEntry).Update("published_at", oldTime).Error
	require.NoError(t, err)

	// Create relay
	relay, err := New(Config{
		DB:           db,
		Brokers:      []string{brokers},
		Topic:        topic,
		PollInterval: 100 * time.Millisecond,
		BatchSize:    10,
		Logger:       logger,
	})
	require.NoError(t, err)
	defer relay.Stop()

	// Cleanup entries older than 7 days
	err = relay.CleanupOldEntries(7 * 24 * time.Hour)
	require.NoError(t, err)

	// Verify entry was deleted
	var count int64
	err = db.Model(&models.DocumentRevisionOutbox{}).
		Where("id = ?", outboxEntry.ID).
		Count(&count).Error
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)
}
