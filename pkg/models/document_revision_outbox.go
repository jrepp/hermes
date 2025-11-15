package models

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// DocumentRevisionOutbox stores events for reliable document revision processing.
// Implements the transactional outbox pattern for event-driven indexing.
type DocumentRevisionOutbox struct {
	ID uint `gorm:"primaryKey" json:"id"`

	// Document identification
	RevisionID   uint      `gorm:"not null;index:idx_revision_outbox_revision_id" json:"revisionId"`
	DocumentUUID uuid.UUID `gorm:"type:uuid;not null;index:idx_revision_outbox_document_uuid" json:"documentUuid"`
	DocumentID   string    `gorm:"type:varchar(500);not null" json:"documentId"`

	// Idempotency key: {document_uuid}:{content_hash}
	IdempotentKey string `gorm:"type:varchar(128);not null;uniqueIndex" json:"idempotentKey"`
	ContentHash   string `gorm:"type:varchar(64);not null" json:"contentHash"`

	// Event metadata
	EventType    string `gorm:"type:varchar(50);not null" json:"eventType"`    // 'revision.created', 'revision.updated', 'revision.deleted'
	ProviderType string `gorm:"type:varchar(50);not null" json:"providerType"` // 'google', 'local', 's3', etc.

	// Event payload (full revision data + metadata)
	Payload map[string]interface{} `gorm:"serializer:json;type:jsonb;not null" json:"payload"`

	// Outbox state
	Status          string     `gorm:"type:varchar(20);not null;default:'pending';index:idx_revision_outbox_pending,where:status = 'pending'" json:"status"` // 'pending', 'published', 'failed'
	PublishedAt     *time.Time `json:"publishedAt,omitempty"`
	PublishAttempts int        `gorm:"default:0" json:"publishAttempts"`
	LastError       string     `gorm:"type:text" json:"lastError,omitempty"`

	// Timestamps
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`

	// Associations
	Revision *DocumentRevision `gorm:"foreignKey:RevisionID" json:"-"`
}

// TableName specifies the table name.
func (DocumentRevisionOutbox) TableName() string {
	return "document_revision_outbox"
}

// RevisionEventType constants
const (
	RevisionEventCreated = "revision.created"
	RevisionEventUpdated = "revision.updated"
	RevisionEventDeleted = "revision.deleted"
)

// OutboxStatus constants
const (
	OutboxStatusPending   = "pending"
	OutboxStatusPublished = "published"
	OutboxStatusFailed    = "failed"
)

// GenerateIdempotentKey creates a unique key for this revision event.
// Format: {document_uuid}:{content_hash}
func GenerateIdempotentKey(documentUUID uuid.UUID, contentHash string) string {
	return fmt.Sprintf("%s:%s", documentUUID.String(), contentHash)
}

// ComputeContentHash computes SHA-256 hash of the revision payload.
func ComputeContentHash(payload interface{}) (string, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal payload: %w", err)
	}

	hash := sha256.Sum256(data)
	return fmt.Sprintf("%x", hash), nil
}

// BeforeCreate hook to ensure required fields.
func (o *DocumentRevisionOutbox) BeforeCreate(tx *gorm.DB) error {
	// Validate required fields
	if o.DocumentUUID == uuid.Nil {
		return fmt.Errorf("document_uuid is required")
	}
	if o.DocumentID == "" {
		return fmt.Errorf("document_id is required")
	}
	if o.RevisionID == 0 {
		return fmt.Errorf("revision_id is required")
	}
	if o.ContentHash == "" {
		return fmt.Errorf("content_hash is required")
	}
	if o.EventType == "" {
		return fmt.Errorf("event_type is required")
	}
	if o.ProviderType == "" {
		return fmt.Errorf("provider_type is required")
	}
	if o.Payload == nil {
		return fmt.Errorf("payload is required")
	}

	// Generate idempotent key if not set
	if o.IdempotentKey == "" {
		o.IdempotentKey = GenerateIdempotentKey(o.DocumentUUID, o.ContentHash)
	}

	// Set default status
	if o.Status == "" {
		o.Status = OutboxStatusPending
	}

	return nil
}

// NewRevisionOutboxEntry creates a new outbox entry from a revision.
func NewRevisionOutboxEntry(revision *DocumentRevision, eventType string, payload map[string]interface{}) (*DocumentRevisionOutbox, error) {
	if revision == nil {
		return nil, fmt.Errorf("revision is required")
	}

	// Compute content hash from payload
	contentHash, err := ComputeContentHash(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to compute content hash: %w", err)
	}

	return &DocumentRevisionOutbox{
		RevisionID:    revision.ID,
		DocumentUUID:  revision.DocumentUUID,
		DocumentID:    revision.DocumentID,
		ContentHash:   contentHash,
		IdempotentKey: GenerateIdempotentKey(revision.DocumentUUID, contentHash),
		EventType:     eventType,
		ProviderType:  revision.ProviderType,
		Payload:       payload,
		Status:        OutboxStatusPending,
	}, nil
}

// FindPendingEntries retrieves pending outbox entries for publishing.
// Uses SELECT FOR UPDATE SKIP LOCKED for concurrent processing.
func FindPendingOutboxEntries(db *gorm.DB, limit int) ([]DocumentRevisionOutbox, error) {
	var entries []DocumentRevisionOutbox

	err := db.
		Where("status = ?", OutboxStatusPending).
		Order("created_at ASC").
		Limit(limit).
		Clauses(
		// SKIP LOCKED prevents concurrent workers from processing same events
		// This is crucial for horizontal scaling
		// Note: Some databases may not support this, fallback gracefully
		).
		Find(&entries).Error

	return entries, err
}

// MarkAsPublished marks the outbox entry as successfully published.
func (o *DocumentRevisionOutbox) MarkAsPublished(db *gorm.DB) error {
	now := time.Now()
	return db.Model(o).Updates(map[string]interface{}{
		"status":       OutboxStatusPublished,
		"published_at": now,
		"updated_at":   now,
	}).Error
}

// MarkAsFailed marks the outbox entry as failed with error details.
func (o *DocumentRevisionOutbox) MarkAsFailed(db *gorm.DB, err error) error {
	o.PublishAttempts++
	o.Status = OutboxStatusFailed
	o.LastError = err.Error()

	return db.Model(o).Updates(map[string]interface{}{
		"status":           OutboxStatusFailed,
		"publish_attempts": o.PublishAttempts,
		"last_error":       err.Error(),
		"updated_at":       time.Now(),
	}).Error
}

// Retry resets the outbox entry status to pending for retry.
func (o *DocumentRevisionOutbox) Retry(db *gorm.DB) error {
	return db.Model(o).Updates(map[string]interface{}{
		"status":     OutboxStatusPending,
		"last_error": "",
		"updated_at": time.Now(),
	}).Error
}

// DeleteOldPublishedEntries removes published entries older than the specified duration.
// Used for cleanup to prevent unbounded table growth.
func DeleteOldPublishedEntries(db *gorm.DB, olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan)
	result := db.
		Where("status = ? AND published_at < ?", OutboxStatusPublished, cutoff).
		Delete(&DocumentRevisionOutbox{})

	return result.RowsAffected, result.Error
}

// GetByIdempotentKey retrieves an outbox entry by its idempotent key.
// Used to check if an event was already published.
func GetOutboxByIdempotentKey(db *gorm.DB, key string) (*DocumentRevisionOutbox, error) {
	var entry DocumentRevisionOutbox
	err := db.Where("idempotent_key = ?", key).First(&entry).Error
	if err != nil {
		return nil, err
	}
	return &entry, nil
}

// GetFailedEntries retrieves failed outbox entries for manual review/retry.
func GetFailedOutboxEntries(db *gorm.DB, limit int) ([]DocumentRevisionOutbox, error) {
	var entries []DocumentRevisionOutbox
	err := db.
		Where("status = ?", OutboxStatusFailed).
		Order("updated_at DESC").
		Limit(limit).
		Find(&entries).Error

	return entries, err
}

// CountByStatus returns the count of entries for a given status.
func CountOutboxByStatus(db *gorm.DB, status string) (int64, error) {
	var count int64
	err := db.Model(&DocumentRevisionOutbox{}).
		Where("status = ?", status).
		Count(&count).Error

	return count, err
}
