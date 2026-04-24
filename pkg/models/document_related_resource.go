package models

import (
	"gorm.io/gorm"
)

// DocumentRelatedResource is a model for a document related resource.
//
//nolint:govet // Keep ORM field grouping readable; alignment churn is low value here.
type DocumentRelatedResource struct {
	gorm.Model
	RelatedResourceType string `gorm:"default:null;not null"`
	DocumentID          uint   `gorm:"uniqueIndex:document_id_sort_order_unique"`
	RelatedResourceID   uint   `gorm:"default:null;not null"`
	SortOrder           int    `gorm:"default:null;not null;uniqueIndex:document_id_sort_order_unique"`
	Document            Document
}

func (d DocumentRelatedResource) relatedResourceRef() relatedResourceReference {
	return relatedResourceReference{
		relatedResourceType: d.RelatedResourceType,
		relatedResourceID:   d.RelatedResourceID,
	}
}
