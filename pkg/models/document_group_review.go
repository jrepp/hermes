package models

import (
	"fmt"
	"time"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// DocumentGroupReview represents a document review by a group.
//
//nolint:govet // Keep ORM field grouping readable; alignment churn is low value here.
type DocumentGroupReview struct {
	DocumentID uint `gorm:"primaryKey"`
	GroupID    uint `gorm:"primaryKey"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
	DeletedAt  gorm.DeletedAt `gorm:"index"`
	Document   Document
	Group      Group
}

// DocumentGroupReviews is a slice of document group reviews.
type DocumentGroupReviews []DocumentGroupReview

// BeforeSave is a hook to find or create associations before saving.
func (d *DocumentGroupReview) BeforeSave(tx *gorm.DB) error {
	// Validate required fields.
	if d.Document.hasNoFileID() {
		return fmt.Errorf("document must have either GoogleFileID or FileID")
	}
	if err := validation.ValidateStruct(&d.Group,
		validation.Field(
			&d.Group.EmailAddress, validation.Required),
	); err != nil {
		return err
	}

	if err := d.getAssociations(tx); err != nil {
		return fmt.Errorf("error getting associations: %w", err)
	}

	return nil
}

// Find finds all document group reviews with the provided query, and assigns
// them to the receiver.
func (d *DocumentGroupReviews) Find(db *gorm.DB, dr DocumentGroupReview) error {
	return findAssociatedReviews(
		db,
		&dr.Document,
		dr.Group.EmailAddress,
		"at least a Document's GoogleFileID or Group's EmailAddress is required",
		func(db *gorm.DB) (uint, error) {
			if err := dr.Group.Get(db); err != nil {
				return 0, fmt.Errorf("error getting group: %w", err)
			}

			return dr.Group.ID, nil
		},
		func(documentID uint, associatedID uint) interface{} {
			return DocumentGroupReview{DocumentID: documentID, GroupID: associatedID}
		},
		d,
	)
}

// Get gets the document group review from database db, and assigns it to the
// receiver.
func (d *DocumentGroupReview) Get(db *gorm.DB) error {
	// Validate required fields.
	if d.Document.hasNoFileID() {
		return fmt.Errorf("document must have either GoogleFileID or FileID")
	}
	if err := validation.ValidateStruct(&d.Group,
		validation.Field(&d.Group.EmailAddress, validation.Required),
	); err != nil {
		return err
	}

	if err := d.getAssociations(db); err != nil {
		return fmt.Errorf("error getting associations: %w", err)
	}

	return db.
		Where(DocumentGroupReview{
			DocumentID: d.DocumentID,
			GroupID:    d.GroupID,
		}).
		Preload(clause.Associations).
		First(&d).
		Error
}

// Update updates the document review in database db.
func (d *DocumentGroupReview) Update(db *gorm.DB) error {
	if err := d.getAssociations(db); err != nil {
		return fmt.Errorf("error getting associations: %w", err)
	}

	return db.
		Model(&d).
		Omit(clause.Associations).
		Updates(*d).
		Error
}

// getAssociations gets associations.
func (d *DocumentGroupReview) getAssociations(db *gorm.DB) error {
	// Get document.
	if err := d.Document.Get(db); err != nil {
		return fmt.Errorf("error getting document: %w", err)
	}
	d.DocumentID = d.Document.ID

	// Get group.
	if err := d.Group.Get(db); err != nil {
		return fmt.Errorf("error getting group: %w", err)
	}
	d.GroupID = d.Group.ID

	return nil
}
