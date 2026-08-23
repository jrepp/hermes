package models

import (
	"fmt"
	"time"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// DocumentFileRevision is a model for a document's Google Drive file revisions.
//
//nolint:govet // Keep ORM field grouping readable; alignment churn is low value here.
type DocumentFileRevision struct {
	GoogleDriveFileRevisionID string `gorm:"primaryKey"`
	FileRevisionID            string `gorm:"-"`
	Name                      string `gorm:"primaryKey"`
	DocumentID                uint   `gorm:"primaryKey"`
	CreatedAt                 time.Time
	UpdatedAt                 time.Time
	DeletedAt                 gorm.DeletedAt `gorm:"index"`
	Document                  Document
}

// RevisionKey returns the provider-agnostic identifier for the revision.
//
// GoogleDriveFileRevisionID is the persisted primary key column, so it is
// authoritative for records loaded from the database. FileRevisionID is
// `gorm:"-"` and only set in memory by provider-agnostic callers, so it is the
// fallback.
func (fr *DocumentFileRevision) RevisionKey() string {
	if fr.GoogleDriveFileRevisionID != "" {
		return fr.GoogleDriveFileRevisionID
	}

	return fr.FileRevisionID
}

// normalizeRevisionID makes the two identifier fields agree before the record
// reaches the database.
//
// There is one identity here wearing two names. GoogleDriveFileRevisionID is
// the column, and part of the primary key; FileRevisionID is `gorm:"-"` and
// exists so provider-agnostic callers do not have to mention Google. Callers
// set one, the other, or both, and nothing reconciled them -- so a caller that
// set only FileRevisionID wrote a row whose primary key was the empty string
// and whose revision identifier was lost. pkg/document and the Algolia
// migration both do exactly that.
//
// Copying between them is what makes those callers correct, and it is why a
// record created with only FileRevisionID reads back with
// GoogleDriveFileRevisionID set.
func (fr *DocumentFileRevision) normalizeRevisionID() {
	switch {
	case fr.GoogleDriveFileRevisionID == "":
		fr.GoogleDriveFileRevisionID = fr.FileRevisionID
	case fr.FileRevisionID == "":
		fr.FileRevisionID = fr.GoogleDriveFileRevisionID
	}
}

// AfterFind mirrors the stored identifier back into the in-memory alias.
//
// FileRevisionID is `gorm:"-"`, so nothing populates it on a read: a record
// loaded from the database came back with the identifier in
// GoogleDriveFileRevisionID and an empty FileRevisionID, and any caller
// reading the provider-agnostic name got "". Writing both on the way out is
// the counterpart to reconciling them on the way in, and it means the field a
// caller set is the field it reads back.
func (fr *DocumentFileRevision) AfterFind(_ *gorm.DB) error {
	fr.normalizeRevisionID()

	return nil
}

// DocumentFileRevisions is a slice of document file revisions.
type DocumentFileRevisions []DocumentFileRevision

// Create creates a file revision for a document.
// Required fields in the receiver:
//   - Document ID or Google File ID
//   - Google Drive file revision ID
//   - Name of file revision
func (fr *DocumentFileRevision) Create(db *gorm.DB) error {
	// Preload Document.
	if fr.DocumentID == 0 {
		if err := fr.Document.Get(db); err != nil {
			return fmt.Errorf("error preloading Document: %w", err)
		}
		fr.DocumentID = fr.Document.ID
	}

	// One identity, two field names: reconcile them before the primary key is
	// written from one of them.
	fr.normalizeRevisionID()

	// Validate fields.
	if err := validation.ValidateStruct(fr,
		validation.Field(&fr.DocumentID, validation.Required),
		validation.Field(&fr.GoogleDriveFileRevisionID, validation.Required),
		validation.Field(&fr.Name, validation.Required),
	); err != nil {
		return err
	}

	return db.
		Omit("Document").
		Create(&fr).
		Error
}

// Find finds all file revisions for a provided document, and assigns them to
// the receiver.
func (frs *DocumentFileRevisions) Find(db *gorm.DB, doc Document) error {
	// Preload Document.
	if doc.ID == 0 {
		if err := doc.Get(db); err != nil {
			return fmt.Errorf("error preloading document: %w", err)
		}
	}

	// Validate fields.
	if err := validation.ValidateStruct(&doc,
		validation.Field(&doc.ID, validation.Required),
	); err != nil {
		return err
	}

	return db.
		Where(DocumentFileRevision{
			DocumentID: doc.ID,
		}).
		Preload(clause.Associations).
		Find(&frs).
		Error
}

// Get retrieves a file revision from database db.
func (fr *DocumentFileRevision) Get(db *gorm.DB) error {
	// Preload Document.
	if fr.DocumentID == 0 {
		if err := fr.Document.Get(db); err != nil {
			return fmt.Errorf("error preloading Document: %w", err)
		}
		fr.DocumentID = fr.Document.ID
	}

	// Same reconciliation as Create: First matches on the primary key fields,
	// so looking a record up by FileRevisionID alone would query for the empty
	// string and find whatever happened to be written that way.
	fr.normalizeRevisionID()

	// Validate fields.
	if err := validation.ValidateStruct(fr,
		validation.Field(&fr.DocumentID, validation.Required),
		validation.Field(&fr.GoogleDriveFileRevisionID, validation.Required),
		validation.Field(&fr.Name, validation.Required),
	); err != nil {
		return err
	}

	return db.
		Preload(clause.Associations).
		First(&fr).
		Error
}
