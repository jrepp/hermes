package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// DocumentSummary stores AI-generated summaries and analysis of documents.
// This enables caching AI results and avoiding redundant API calls.
type DocumentSummary struct {
	ID uint `gorm:"primaryKey" json:"id"`

	// Document identification
	DocumentID   string     `gorm:"type:varchar(500);not null;index:idx_doc_summaries_doc_id" json:"documentId"`
	DocumentUUID *uuid.UUID `gorm:"type:uuid;index:idx_doc_summaries_uuid" json:"documentUuid,omitempty"`

	// Summary content
	ExecutiveSummary string      `gorm:"type:text;not null" json:"executiveSummary"`
	KeyPoints        StringArray `gorm:"type:jsonb" json:"keyPoints"`
	Topics           StringArray `gorm:"type:jsonb" json:"topics"`
	Tags             StringArray `gorm:"type:jsonb" json:"tags"`

	// AI analysis
	SuggestedStatus string   `gorm:"type:varchar(50)" json:"suggestedStatus,omitempty"`
	Confidence      *float64 `gorm:"type:double precision" json:"confidence,omitempty"`

	// Metadata
	Model            string `gorm:"type:varchar(100);not null;index:idx_doc_summaries_model" json:"model"`
	Provider         string `gorm:"type:varchar(50);not null" json:"provider"`
	TokensUsed       *int   `gorm:"type:integer" json:"tokensUsed,omitempty"`
	GenerationTimeMs *int   `gorm:"type:integer" json:"generationTimeMs,omitempty"`

	// Document context at time of generation
	DocumentTitle string `gorm:"type:varchar(500)" json:"documentTitle,omitempty"`
	DocumentType  string `gorm:"type:varchar(50);index:idx_doc_summaries_doc_type" json:"documentType,omitempty"`
	ContentHash   string `gorm:"type:varchar(64);index:idx_doc_summaries_content_hash" json:"contentHash,omitempty"`
	ContentLength *int   `gorm:"type:integer" json:"contentLength,omitempty"`

	// Timestamps
	GeneratedAt time.Time `gorm:"not null;index:idx_doc_summaries_generated,sort:desc" json:"generatedAt"`
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"createdAt"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime" json:"updatedAt"`
}

// TableName specifies the table name.
func (DocumentSummary) TableName() string {
	return "document_summaries"
}

// StringArray is a custom type for storing string arrays in JSONB.
type StringArray []string

// Scan implements the sql.Scanner interface.
func (s *StringArray) Scan(value interface{}) error {
	if value == nil {
		*s = StringArray{}
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("failed to unmarshal JSONB value: %v", value)
	}

	var arr []string
	if err := json.Unmarshal(bytes, &arr); err != nil {
		return err
	}

	*s = StringArray(arr)
	return nil
}

// Value implements the driver.Valuer interface.
func (s StringArray) Value() (driver.Value, error) {
	if s == nil {
		return nil, nil
	}
	return json.Marshal(s)
}

// BeforeCreate hook to ensure required fields.
func (ds *DocumentSummary) BeforeCreate(tx *gorm.DB) error {
	if ds.GeneratedAt.IsZero() {
		ds.GeneratedAt = time.Now()
	}
	if ds.DocumentID == "" {
		return fmt.Errorf("document_id is required")
	}
	if ds.ExecutiveSummary == "" {
		return fmt.Errorf("executive_summary is required")
	}
	if ds.Model == "" {
		return fmt.Errorf("model is required")
	}
	if ds.Provider == "" {
		return fmt.Errorf("provider is required")
	}
	return nil
}

// GetLatestByDocumentID retrieves the most recent summary for a document.
func GetLatestSummaryByDocumentID(db *gorm.DB, documentID string) (*DocumentSummary, error) {
	var summary DocumentSummary
	err := db.Where("document_id = ?", documentID).
		Order("generated_at DESC").
		First(&summary).Error
	if err != nil {
		return nil, err
	}
	return &summary, nil
}

// GetLatestByUUID retrieves the most recent summary for a document UUID.
func GetLatestSummaryByUUID(db *gorm.DB, documentUUID uuid.UUID) (*DocumentSummary, error) {
	var summary DocumentSummary
	err := db.Where("document_uuid = ?", documentUUID).
		Order("generated_at DESC").
		First(&summary).Error
	if err != nil {
		return nil, err
	}
	return &summary, nil
}

// GetByDocumentIDAndModel retrieves a summary for a specific document and model.
func GetSummaryByDocumentIDAndModel(db *gorm.DB, documentID, model string) (*DocumentSummary, error) {
	var summary DocumentSummary
	err := db.Where("document_id = ? AND model = ?", documentID, model).
		First(&summary).Error
	if err != nil {
		return nil, err
	}
	return &summary, nil
}

// IsStale checks if the summary is older than the specified duration.
func (ds *DocumentSummary) IsStale(maxAge time.Duration) bool {
	return time.Since(ds.GeneratedAt) > maxAge
}

// MatchesContentHash checks if the summary was generated from the same content.
func (ds *DocumentSummary) MatchesContentHash(hash string) bool {
	return ds.ContentHash == hash
}

// DeleteOldSummaries removes summaries older than the specified age.
func DeleteOldSummaries(db *gorm.DB, olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan)
	result := db.Where("generated_at < ?", cutoff).Delete(&DocumentSummary{})
	return result.RowsAffected, result.Error
}
