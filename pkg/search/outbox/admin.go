package outbox

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/hashicorp-forge/hermes/pkg/document"
	"github.com/hashicorp-forge/hermes/pkg/models"
)

// Stats reports search outbox queue depth and lag for health checks.
type Stats struct {
	OldestPendingAgeSeconds int64 `json:"oldestPendingAgeSeconds"`
	Pending                 int64 `json:"pending"`
	Processing              int64 `json:"processing"`
	Failed                  int64 `json:"failed"`
	DLQ                     int64 `json:"dlq"`
	Skipped                 int64 `json:"skipped"`
}

// GetStats returns search outbox queue depth and lag for health reporting.
func GetStats(db *gorm.DB, now time.Time) (Stats, error) {
	if db == nil {
		return Stats{}, errors.New("database is required")
	}
	if now.IsZero() {
		now = time.Now()
	}

	stats := Stats{}
	counts := map[string]*int64{
		models.SearchOutboxStatusPending:    &stats.Pending,
		models.SearchOutboxStatusProcessing: &stats.Processing,
		models.SearchOutboxStatusFailed:     &stats.Failed,
		models.SearchOutboxStatusDLQ:        &stats.DLQ,
		models.SearchOutboxStatusSkipped:    &stats.Skipped,
	}
	for status, target := range counts {
		if err := db.Model(&models.SearchOutboxEvent{}).
			Where("status = ?", status).
			Count(target).Error; err != nil {
			return Stats{}, fmt.Errorf("count %s search outbox events: %w", status, err)
		}
	}

	var oldest models.SearchOutboxEvent
	if err := db.Where("status IN ?", []string{
		models.SearchOutboxStatusPending,
		models.SearchOutboxStatusFailed,
	}).
		Order("available_at ASC, id ASC").
		First(&oldest).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return Stats{}, fmt.Errorf("get oldest search outbox event: %w", err)
	}
	if oldest.ID != 0 && oldest.AvailableAt.Before(now) {
		stats.OldestPendingAgeSeconds = int64(now.Sub(oldest.AvailableAt).Seconds())
	}

	return stats, nil
}

// ListFailedEvents returns failed and DLQ search outbox events for operator inspection.
func ListFailedEvents(db *gorm.DB, limit int) ([]models.SearchOutboxEvent, error) {
	if db == nil {
		return nil, errors.New("database is required")
	}
	if limit <= 0 {
		limit = 50
	}

	var events []models.SearchOutboxEvent
	if err := db.Where("status IN ?", []string{models.SearchOutboxStatusFailed, models.SearchOutboxStatusDLQ}).
		Order("updated_at ASC, id ASC").
		Limit(limit).
		Find(&events).Error; err != nil {
		return nil, fmt.Errorf("list failed search outbox events: %w", err)
	}

	return events, nil
}

// RetryEvent returns a failed, DLQ, or skipped event to pending state.
func RetryEvent(db *gorm.DB, id uint, now time.Time) error {
	if db == nil {
		return errors.New("database is required")
	}
	if id == 0 {
		return errors.New("event id is required")
	}
	if now.IsZero() {
		now = time.Now()
	}

	result := db.Model(&models.SearchOutboxEvent{}).
		Where("id = ? AND status IN ?", id, []string{
			models.SearchOutboxStatusFailed,
			models.SearchOutboxStatusDLQ,
			models.SearchOutboxStatusSkipped,
		}).
		Updates(map[string]any{
			"status":        models.SearchOutboxStatusPending,
			"available_at":  now,
			"locked_at":     nil,
			"locked_by":     "",
			"error_message": "",
			"updated_at":    now,
		})
	if result.Error != nil {
		return fmt.Errorf("retry search outbox event: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("search outbox event %d is not failed, dlq, or skipped", id)
	}

	return nil
}

// SkipEvent marks a failed or DLQ event skipped so later same-aggregate events can proceed.
func SkipEvent(db *gorm.DB, id uint, note string, now time.Time) error {
	if db == nil {
		return errors.New("database is required")
	}
	if id == 0 {
		return errors.New("event id is required")
	}
	if note == "" {
		return errors.New("operator note is required")
	}
	if now.IsZero() {
		now = time.Now()
	}

	result := db.Model(&models.SearchOutboxEvent{}).
		Where("id = ? AND status IN ?", id, []string{
			models.SearchOutboxStatusFailed,
			models.SearchOutboxStatusDLQ,
		}).
		Updates(map[string]any{
			"status":        models.SearchOutboxStatusSkipped,
			"locked_at":     nil,
			"locked_by":     "",
			"error_message": fmt.Sprintf("skipped by operator: %s", note),
			"updated_at":    now,
		})
	if result.Error != nil {
		return fmt.Errorf("skip search outbox event: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("search outbox event %d is not failed or dlq", id)
	}

	return nil
}

// RebuildCurrent enqueues a fresh projection from database truth for the same aggregate as event id.
func RebuildCurrent(db *gorm.DB, id uint) (*models.SearchOutboxEvent, error) {
	if db == nil {
		return nil, errors.New("database is required")
	}
	if id == 0 {
		return nil, errors.New("event id is required")
	}

	var rebuilt *models.SearchOutboxEvent
	err := db.Transaction(func(tx *gorm.DB) error {
		var original models.SearchOutboxEvent
		if err := tx.First(&original, id).Error; err != nil {
			return fmt.Errorf("get search outbox event: %w", err)
		}

		event, err := buildCurrentProjectionEvent(tx, &original)
		if err != nil {
			return err
		}
		if err := models.EnqueueSearchOutboxEventWithSequence(tx, event); err != nil {
			return fmt.Errorf("enqueue rebuilt search outbox event: %w", err)
		}
		rebuilt = event
		return nil
	})
	if err != nil {
		return nil, err
	}

	return rebuilt, nil
}

func buildCurrentProjectionEvent(tx *gorm.DB, original *models.SearchOutboxEvent) (*models.SearchOutboxEvent, error) {
	switch original.AggregateType {
	case models.SearchAggregateDocument, models.SearchAggregateDraft:
		return buildCurrentDocumentProjectionEvent(tx, original)
	case models.SearchAggregateProject:
		return buildCurrentProjectProjectionEvent(tx, original)
	case models.SearchAggregateLink:
		return buildCurrentLinkProjectionEvent(tx, original)
	default:
		return nil, fmt.Errorf("unsupported aggregate type %q", original.AggregateType)
	}
}

func buildCurrentDocumentProjectionEvent(tx *gorm.DB, original *models.SearchOutboxEvent) (*models.SearchOutboxEvent, error) {
	model := models.Document{GoogleFileID: original.AggregateID}
	if err := model.Get(tx); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return searchBackfillDeleteEvent(original), nil
		}
		return nil, fmt.Errorf("get document for rebuild-current: %w", err)
	}

	var reviews models.DocumentReviews
	if err := reviews.Find(tx, models.DocumentReview{Document: models.Document{GoogleFileID: original.AggregateID}}); err != nil {
		return nil, fmt.Errorf("get document reviews for rebuild-current: %w", err)
	}
	var groupReviews models.DocumentGroupReviews
	if err := groupReviews.Find(tx, models.DocumentGroupReview{Document: models.Document{GoogleFileID: original.AggregateID}}); err != nil {
		return nil, fmt.Errorf("get document group reviews for rebuild-current: %w", err)
	}

	doc, err := document.NewFromDatabaseModel(model, reviews, groupReviews)
	if err != nil {
		return nil, fmt.Errorf("convert document for rebuild-current: %w", err)
	}
	payload, err := doc.ToAlgoliaObject(original.IndexName == models.SearchIndexDocuments)
	if err != nil {
		return nil, fmt.Errorf("convert document search payload for rebuild-current: %w", err)
	}

	return searchBackfillUpsertEvent(original, payload), nil
}

func buildCurrentProjectProjectionEvent(tx *gorm.DB, original *models.SearchOutboxEvent) (*models.SearchOutboxEvent, error) {
	id, err := strconv.ParseUint(original.AggregateID, 10, 64)
	if err != nil || id == 0 {
		return nil, fmt.Errorf("project aggregate id %q must be a positive integer", original.AggregateID)
	}

	var project models.Project
	if err := project.Get(tx, uint(id)); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return searchBackfillDeleteEvent(original), nil
		}
		return nil, fmt.Errorf("get project for rebuild-current: %w", err)
	}

	return searchBackfillUpsertEvent(original, map[string]any{
		"createdTime":  project.ProjectCreatedAt.Unix(),
		"creator":      project.Creator.EmailAddress,
		"description":  project.Description,
		"jiraIssueID":  project.JiraIssueID,
		"modifiedTime": project.ProjectModifiedAt.Unix(),
		"objectID":     fmt.Sprintf("%d", project.ID),
		"status":       project.Status.String(),
		"title":        project.Title,
	}), nil
}

func buildCurrentLinkProjectionEvent(tx *gorm.DB, original *models.SearchOutboxEvent) (*models.SearchOutboxEvent, error) {
	documentID, _ := original.Payload["documentID"].(string)
	if documentID == "" {
		return nil, errors.New("link rebuild-current requires original payload documentID")
	}

	model := models.Document{GoogleFileID: documentID}
	if err := model.Get(tx); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return searchBackfillDeleteEvent(original), nil
		}
		return nil, fmt.Errorf("get link document for rebuild-current: %w", err)
	}
	docType := strings.ToLower(model.DocumentType.Name)
	docNumber := strings.ToLower(fmt.Sprintf("%s-%03d", model.Product.Abbreviation, model.DocumentNumber))
	if model.DocumentNumber == 0 {
		docNumber = strings.ToLower(fmt.Sprintf("%s-???", model.Product.Abbreviation))
	}
	if docType == "" || docNumber == "" {
		return searchBackfillDeleteEvent(original), nil
	}
	objectID := fmt.Sprintf("/%s/%s", docType, docNumber)

	event := searchBackfillUpsertEvent(original, map[string]any{
		"documentID": model.GoogleFileID,
		"objectID":   objectID,
	})
	event.AggregateID = objectID
	return event, nil
}

func searchBackfillUpsertEvent(original *models.SearchOutboxEvent, payload map[string]any) *models.SearchOutboxEvent {
	return &models.SearchOutboxEvent{
		EventType:     models.SearchEventBackfill,
		AggregateID:   original.AggregateID,
		AggregateType: original.AggregateType,
		IndexName:     original.IndexName,
		Operation:     models.SearchOutboxOperationUpsert,
		Payload:       payload,
	}
}

func searchBackfillDeleteEvent(original *models.SearchOutboxEvent) *models.SearchOutboxEvent {
	return &models.SearchOutboxEvent{
		EventType:     models.SearchEventBackfill,
		AggregateID:   original.AggregateID,
		AggregateType: original.AggregateType,
		IndexName:     original.IndexName,
		Operation:     models.SearchOutboxOperationDelete,
	}
}
