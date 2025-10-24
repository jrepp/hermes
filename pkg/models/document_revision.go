package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// DocumentRevision tracks different versions of a document across providers.
// This enables migration tracking, conflict detection, and maintaining
// document history when documents move between workspace providers.
type DocumentRevision struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`

	// Document identification
	DocumentUUID     uuid.UUID `gorm:"type:uuid;not null;index:idx_doc_revisions_uuid" json:"documentUuid"`
	DocumentID       string    `gorm:"type:varchar(500);not null" json:"documentId"` // Provider-specific ID
	ProviderType     string    `gorm:"type:varchar(50);not null;index:idx_doc_revisions_provider" json:"providerType"`
	ProviderFolderID string    `gorm:"type:varchar(500)" json:"providerFolderId,omitempty"`

	// Document metadata
	Title        string    `gorm:"type:varchar(500)" json:"title"`
	ContentHash  string    `gorm:"type:varchar(64);index:idx_doc_revisions_hash" json:"contentHash"` // SHA-256
	ModifiedTime time.Time `gorm:"index:idx_doc_revisions_modified" json:"modifiedTime"`

	// Revision status
	Status string `gorm:"type:varchar(20);not null;default:'active';index:idx_doc_revisions_status" json:"status"`
	// Status values:
	// - "active": Current version in this provider
	// - "source": Original document being migrated from
	// - "target": Document being migrated to
	// - "archived": Old version, no longer active
	// - "conflict": Conflicting version detected

	// Project association - tracks which project owns this revision (supports migration tracking)
	ProjectUUID *uuid.UUID `gorm:"type:uuid;index:idx_doc_revisions_project_uuid" json:"projectUuid,omitempty"`

	// ProjectID is DEPRECATED - use ProjectUUID instead
	ProjectID *uint `gorm:"index:idx_doc_revisions_project" json:"projectId,omitempty"`

	// Migration tracking
	MigratedFrom *uint      `gorm:"index:idx_doc_revisions_migrated_from" json:"migratedFrom,omitempty"` // Foreign key to another revision
	MigratedAt   *time.Time `json:"migratedAt,omitempty"`
}

// TableName specifies the table name.
func (DocumentRevision) TableName() string {
	return "document_revisions"
}

// BeforeCreate hook to ensure DocumentUUID is set.
func (dr *DocumentRevision) BeforeCreate(tx *gorm.DB) error {
	// Generate UUID if not provided
	if dr.DocumentUUID == uuid.Nil {
		dr.DocumentUUID = uuid.New()
	}

	// Set default status if not provided
	if dr.Status == "" {
		dr.Status = "active"
	}

	return nil
}

// GetByUUID retrieves all revisions for a document UUID.
func GetRevisionsByUUID(db *gorm.DB, documentUUID uuid.UUID) ([]DocumentRevision, error) {
	var revisions []DocumentRevision
	err := db.Where("document_uuid = ?", documentUUID).
		Order("modified_time DESC").
		Find(&revisions).Error
	return revisions, err
}

// GetByDocumentID retrieves a revision by provider-specific document ID.
func GetRevisionByDocumentID(db *gorm.DB, providerType, documentID string) (*DocumentRevision, error) {
	var revision DocumentRevision
	err := db.Where("provider_type = ? AND document_id = ?", providerType, documentID).
		First(&revision).Error
	if err != nil {
		return nil, err
	}
	return &revision, nil
}

// GetActiveRevisions retrieves all active revisions for a document UUID.
func GetActiveRevisions(db *gorm.DB, documentUUID uuid.UUID) ([]DocumentRevision, error) {
	var revisions []DocumentRevision
	err := db.Where("document_uuid = ? AND status = ?", documentUUID, "active").
		Order("modified_time DESC").
		Find(&revisions).Error
	return revisions, err
}

// GetByContentHash finds revisions with the same content hash.
// Useful for detecting duplicate content or unchanged documents.
func GetRevisionsByContentHash(db *gorm.DB, contentHash string) ([]DocumentRevision, error) {
	var revisions []DocumentRevision
	err := db.Where("content_hash = ?", contentHash).
		Order("modified_time DESC").
		Find(&revisions).Error
	return revisions, err
}

// DetectConflicts finds potential conflicts for this revision.
// Returns other active revisions with different content hashes but same UUID.
func (dr *DocumentRevision) DetectConflicts(db *gorm.DB) ([]DocumentRevision, error) {
	var conflicts []DocumentRevision
	err := db.Where("document_uuid = ? AND status = ? AND content_hash != ? AND id != ?",
		dr.DocumentUUID, "active", dr.ContentHash, dr.ID).
		Order("modified_time DESC").
		Find(&conflicts).Error
	return conflicts, err
}

// MarkAsSource marks this revision as the source of a migration.
func (dr *DocumentRevision) MarkAsSource(db *gorm.DB) error {
	return db.Model(dr).Update("status", "source").Error
}

// MarkAsTarget marks this revision as the target of a migration.
func (dr *DocumentRevision) MarkAsTarget(db *gorm.DB, sourceRevisionID uint) error {
	return db.Model(dr).Updates(map[string]interface{}{
		"status":        "target",
		"migrated_from": sourceRevisionID,
		"migrated_at":   time.Now(),
	}).Error
}

// MarkAsConflict marks this revision as having a conflict.
func (dr *DocumentRevision) MarkAsConflict(db *gorm.DB) error {
	return db.Model(dr).Update("status", "conflict").Error
}

// MarkAsArchived archives this revision.
func (dr *DocumentRevision) MarkAsArchived(db *gorm.DB) error {
	return db.Model(dr).Update("status", "archived").Error
}

// Update updates the revision with new content and metadata.
func (dr *DocumentRevision) Update(db *gorm.DB, contentHash string, modifiedTime time.Time) error {
	return db.Model(dr).Updates(map[string]interface{}{
		"content_hash":  contentHash,
		"modified_time": modifiedTime,
	}).Error
}
