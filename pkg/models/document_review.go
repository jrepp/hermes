package models

import (
	"fmt"
	"time"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// DocumentReview represents a document review by a user.
//
//nolint:govet // Keep ORM field grouping readable; alignment churn is low value here.
type DocumentReview struct {
	DocumentID uint `gorm:"primaryKey"`
	UserID     uint `gorm:"primaryKey"`
	Status     DocumentReviewStatus
	CreatedAt  time.Time
	UpdatedAt  time.Time
	DeletedAt  gorm.DeletedAt `gorm:"index"`
	Document   Document
	User       User
}

// DocumentReviewStatus represents the status of a document review.
type DocumentReviewStatus int

// Document review status constants.
const (
	UnspecifiedDocumentReviewStatus DocumentReviewStatus = iota
	ApprovedDocumentReviewStatus
	ChangesRequestedDocumentReviewStatus
)

// DocumentReviews is a slice of document reviews.
type DocumentReviews []DocumentReview

// BeforeSave is a hook to find or create associations before saving.
func (d *DocumentReview) BeforeSave(tx *gorm.DB) error {
	// Validate required fields.
	if d.Document.hasNoFileID() {
		return fmt.Errorf("document must have either GoogleFileID or FileID")
	}
	if err := validation.ValidateStruct(&d.User,
		validation.Field(
			&d.User.EmailAddress, validation.Required),
	); err != nil {
		return err
	}

	if err := d.getAssociations(tx); err != nil {
		return fmt.Errorf("error getting associations: %w", err)
	}

	return nil
}

// Find finds all document reviews with the provided query, and assigns them to
// the receiver.
func (d *DocumentReviews) Find(db *gorm.DB, dr DocumentReview) error {
	return findAssociatedReviews(
		db,
		&dr.Document,
		dr.User.EmailAddress,
		"at least a Document's file identifier or User's EmailAddress is required",
		func(db *gorm.DB) (uint, error) {
			if err := dr.User.Get(db); err != nil {
				return 0, fmt.Errorf("error getting user: %w", err)
			}

			return dr.User.ID, nil
		},
		func(documentID uint, associatedID uint) interface{} {
			return DocumentReview{DocumentID: documentID, UserID: associatedID}
		},
		d,
	)
}

// Get gets the document review from database db, and assigns it to the
// receiver.
func (d *DocumentReview) Get(db *gorm.DB) error {
	// Validate required fields.
	if d.Document.hasNoFileID() {
		return fmt.Errorf("document must have either GoogleFileID or FileID")
	}
	if err := validation.ValidateStruct(&d.User,
		validation.Field(&d.User.EmailAddress, validation.Required),
	); err != nil {
		return err
	}

	if err := d.getAssociations(db); err != nil {
		return fmt.Errorf("error getting associations: %w", err)
	}

	return db.
		Where(DocumentReview{
			DocumentID: d.DocumentID,
			UserID:     d.UserID,
		}).
		Preload(clause.Associations).
		First(&d).
		Error
}

// Update updates the document review in database db.
func (d *DocumentReview) Update(db *gorm.DB) error {
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
func (d *DocumentReview) getAssociations(db *gorm.DB) error {
	// Get document.
	if err := d.Document.Get(db); err != nil {
		return fmt.Errorf("error getting document: %w", err)
	}
	d.DocumentID = d.Document.ID

	// Get user.
	if err := d.User.Get(db); err != nil {
		return fmt.Errorf("error getting user: %w", err)
	}
	d.UserID = d.User.ID

	return nil
}
