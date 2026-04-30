package models

import (
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SearchOutboxEvent stores search projection events for database-to-cache convergence.
type SearchOutboxEvent struct {
	UpdatedAt      time.Time      `json:"updatedAt"`
	CreatedAt      time.Time      `json:"createdAt"`
	AvailableAt    time.Time      `gorm:"not null;index:idx_search_outbox_available,priority:2" json:"availableAt"`
	LockedAt       *time.Time     `json:"lockedAt,omitempty"`
	LastAttemptAt  *time.Time     `json:"lastAttemptAt,omitempty"`
	CompletedAt    *time.Time     `json:"completedAt,omitempty"`
	Payload        map[string]any `gorm:"serializer:json;type:jsonb" json:"payload,omitempty"`
	EventType      string         `gorm:"type:varchar(50);not null;index:idx_search_outbox_event_type" json:"eventType"`
	AggregateID    string         `gorm:"type:varchar(255);not null;index:idx_search_outbox_aggregate_id;uniqueIndex:idx_search_outbox_aggregate_sequence,priority:2" json:"aggregateId"`
	AggregateType  string         `gorm:"type:varchar(50);not null;index:idx_search_outbox_ordering,priority:1;uniqueIndex:idx_search_outbox_aggregate_sequence,priority:1" json:"aggregateType"`
	IndexName      string         `gorm:"type:varchar(50);not null;index" json:"indexName"`
	Operation      string         `gorm:"type:varchar(20);not null" json:"operation"`
	IdempotencyKey string         `gorm:"type:varchar(255);not null;uniqueIndex" json:"idempotencyKey"`
	Status         string         `gorm:"type:varchar(20);not null;default:'pending';index:idx_search_outbox_status_created,priority:1;index:idx_search_outbox_available,priority:1" json:"status"`
	LockedBy       string         `gorm:"type:varchar(100)" json:"lockedBy,omitempty"`
	ErrorMessage   string         `gorm:"type:text" json:"errorMessage,omitempty"`
	ID             uint           `gorm:"primaryKey" json:"id"`
	Sequence       int64          `gorm:"not null;index:idx_search_outbox_ordering,priority:3;uniqueIndex:idx_search_outbox_aggregate_sequence,priority:3" json:"sequence"`
	AttemptCount   int            `gorm:"not null;default:0" json:"attemptCount"`
}

// SearchOutboxSequence stores the next per-aggregate sequence for search events.
type SearchOutboxSequence struct {
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
	AggregateType string    `gorm:"type:varchar(50);primaryKey" json:"aggregateType"`
	AggregateID   string    `gorm:"type:varchar(255);primaryKey" json:"aggregateId"`
	NextSequence  int64     `gorm:"not null;default:1" json:"nextSequence"`
}

// Search outbox event types defined by RFC-008.
const (
	SearchEventDocumentCreated    = "document.created"
	SearchEventDocumentUpdated    = "document.updated"
	SearchEventDocumentPublished  = "document.published"
	SearchEventDocumentDeleted    = "document.deleted"
	SearchEventDraftCreated       = "draft.created"
	SearchEventDraftUpdated       = "draft.updated"
	SearchEventDraftDeleted       = "draft.deleted"
	SearchEventDraftPublished     = "draft.published"
	SearchEventReviewStateChanged = "review_state.changed"
	SearchEventProjectCreated     = "project.created"
	SearchEventProjectUpdated     = "project.updated"
	SearchEventBackfill           = "search.backfill"
)

// Search outbox aggregate types.
const (
	SearchAggregateDocument = "document"
	SearchAggregateDraft    = "draft"
	SearchAggregateProject  = "project"
)

// Search outbox index names.
const (
	SearchIndexDocuments = "documents"
	SearchIndexDrafts    = "drafts"
	SearchIndexProjects  = "projects"
)

// Search outbox operations.
const (
	SearchOutboxOperationUpsert = "upsert"
	SearchOutboxOperationDelete = "delete"
)

// Search outbox statuses.
const (
	SearchOutboxStatusPending    = "pending"
	SearchOutboxStatusProcessing = "processing"
	SearchOutboxStatusCompleted  = "completed"
	SearchOutboxStatusFailed     = "failed"
	SearchOutboxStatusDLQ        = "dlq"
	SearchOutboxStatusSkipped    = "skipped"
)

// TableName specifies the table name.
func (SearchOutboxEvent) TableName() string {
	return "search_outbox_events"
}

// TableName specifies the table name.
func (SearchOutboxSequence) TableName() string {
	return "search_outbox_sequences"
}

// BeforeCreate validates defaults that must be present before enqueue.
func (e *SearchOutboxEvent) BeforeCreate(_ *gorm.DB) error {
	if e.EventType == "" {
		return errors.New("event_type is required")
	}
	if e.AggregateID == "" {
		return errors.New("aggregate_id is required")
	}
	if e.AggregateType == "" {
		return errors.New("aggregate_type is required")
	}
	if e.IndexName == "" {
		return errors.New("index_name is required")
	}
	if e.Operation != SearchOutboxOperationUpsert && e.Operation != SearchOutboxOperationDelete {
		return fmt.Errorf("operation must be %q or %q", SearchOutboxOperationUpsert, SearchOutboxOperationDelete)
	}
	if e.Sequence <= 0 {
		return errors.New("sequence must be greater than zero")
	}
	if e.IdempotencyKey == "" {
		e.IdempotencyKey = GenerateSearchOutboxIdempotencyKey(e.IndexName, e.AggregateID, e.EventType, e.Sequence)
	}
	if e.Status == "" {
		e.Status = SearchOutboxStatusPending
	}
	if e.AvailableAt.IsZero() {
		e.AvailableAt = time.Now()
	}

	return nil
}

// GenerateSearchOutboxIdempotencyKey creates the default RFC-008 duplicate-safety key.
func GenerateSearchOutboxIdempotencyKey(indexName, aggregateID, eventType string, sequence int64) string {
	return fmt.Sprintf("%s:%s:%s:%d", indexName, aggregateID, eventType, sequence)
}

// EnqueueSearchOutboxEvent inserts a search outbox event using the caller's transaction.
func EnqueueSearchOutboxEvent(tx *gorm.DB, event *SearchOutboxEvent) error {
	if tx == nil {
		return errors.New("transaction handle is required")
	}
	if event == nil {
		return errors.New("search outbox event is required")
	}

	return tx.Create(event).Error
}

// EnqueueSearchOutboxEventWithSequence allocates sequence and enqueues in one transaction.
func EnqueueSearchOutboxEventWithSequence(tx *gorm.DB, event *SearchOutboxEvent) error {
	if tx == nil {
		return errors.New("transaction handle is required")
	}
	if err := validateSearchOutboxEventForSequencedEnqueue(event); err != nil {
		return err
	}

	sequence, err := AllocateSearchOutboxSequence(tx, event.AggregateType, event.AggregateID)
	if err != nil {
		return err
	}
	event.Sequence = sequence

	return EnqueueSearchOutboxEvent(tx, event)
}

func validateSearchOutboxEventForSequencedEnqueue(event *SearchOutboxEvent) error {
	if event == nil {
		return errors.New("search outbox event is required")
	}
	if event.AggregateType == "" {
		return errors.New("aggregate_type is required")
	}
	if event.AggregateID == "" {
		return errors.New("aggregate_id is required")
	}
	if event.Sequence != 0 {
		return errors.New("sequence must be allocated by enqueue helper")
	}

	return nil
}

// AllocateSearchOutboxSequence reserves the next sequence for an aggregate.
func AllocateSearchOutboxSequence(tx *gorm.DB, aggregateType, aggregateID string) (int64, error) {
	if tx == nil {
		return 0, errors.New("transaction handle is required")
	}
	if aggregateType == "" {
		return 0, errors.New("aggregate_type is required")
	}
	if aggregateID == "" {
		return 0, errors.New("aggregate_id is required")
	}

	seq := SearchOutboxSequence{
		AggregateType: aggregateType,
		AggregateID:   aggregateID,
		NextSequence:  1,
	}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&seq).Error; err != nil {
		return 0, fmt.Errorf("create search outbox sequence: %w", err)
	}

	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("aggregate_type = ? AND aggregate_id = ?", aggregateType, aggregateID).
		First(&seq).Error; err != nil {
		return 0, fmt.Errorf("lock search outbox sequence: %w", err)
	}

	allocated := seq.NextSequence
	seq.NextSequence++
	if err := tx.Model(&seq).Update("next_sequence", seq.NextSequence).Error; err != nil {
		return 0, fmt.Errorf("advance search outbox sequence: %w", err)
	}

	return allocated, nil
}
