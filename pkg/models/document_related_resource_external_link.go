package models

import (
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// DocumentRelatedResourceExternalLink is a model for an external link related resource on a document.
type DocumentRelatedResourceExternalLink struct {
	gorm.Model
	Name            string                  `gorm:"default:null;not null"`
	URL             string                  `gorm:"default:null;not null"`
	RelatedResource DocumentRelatedResource `gorm:"polymorphic:RelatedResource"`
}

// DocumentRelatedResourceExternalLinks is a slice of external link related resources.
type DocumentRelatedResourceExternalLinks []DocumentRelatedResourceExternalLink

// Create creates an external link related resource in database db.
func (rr *DocumentRelatedResourceExternalLink) Create(db *gorm.DB) error {
	// Preload RelatedResource.Document.
	if rr.RelatedResource.DocumentID == 0 {
		query := db
		if rr.RelatedResource.Document.GoogleFileID != "" {
			query = query.Where("google_file_id = ?", rr.RelatedResource.Document.GoogleFileID)
		} else {
			query = query.Where("file_id = ?", rr.RelatedResource.Document.FileID)
		}
		if err := query.
			First(&rr.RelatedResource.Document).
			Error; err != nil {
			return fmt.Errorf("error preloading RelatedResource.Document: %w", err)
		}
		rr.RelatedResource.DocumentID = rr.RelatedResource.Document.ID
	}

	return db.
		Omit("RelatedResource.Document").
		Create(&rr).
		Error
}

// Get retrieves an external link related resource from database db.
func (rr *DocumentRelatedResourceExternalLink) Get(db *gorm.DB) error {
	return db.
		Preload(clause.Associations).
		Preload("RelatedResource.Document").
		First(&rr).Error
}
