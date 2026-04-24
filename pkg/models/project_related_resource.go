package models

import (
	"gorm.io/gorm"
)

// ProjectRelatedResource is a model for a project related resource.
//
//nolint:govet // Keep ORM field grouping readable; alignment churn is low value here.
type ProjectRelatedResource struct {
	gorm.Model
	RelatedResourceType string `gorm:"default:null;not null"`
	ProjectID           uint   `gorm:"uniqueIndex:project_id_sort_order_unique"`
	RelatedResourceID   uint   `gorm:"default:null;not null"`
	SortOrder           int    `gorm:"default:null;not null;uniqueIndex:project_id_sort_order_unique"`
	Project             Project
}

func (p ProjectRelatedResource) relatedResourceRef() relatedResourceReference {
	return relatedResourceReference{
		relatedResourceType: p.RelatedResourceType,
		relatedResourceID:   p.RelatedResourceID,
	}
}
