package models

import (
	"errors"
	"fmt"
	"time"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/hashicorp-forge/hermes/pkg/docid"
)

// Document is a model for a document.
type Document struct {
	DocumentCreatedAt  time.Time
	DocumentModifiedAt time.Time
	ProjectID          *string    `gorm:"type:varchar(64)"`
	ProjectUUID        *uuid.UUID `gorm:"type:uuid;index:idx_documents_project_uuid"`
	ProviderType       *string    `gorm:"type:varchar(50)"`
	ProviderDocumentID *string    `gorm:"type:varchar(500);index:idx_documents_provider_doc_id"`
	Summary            *string
	OwnerID            *uint       `gorm:"default:null"`
	DocumentUUID       *docid.UUID `gorm:"type:uuid;uniqueIndex:idx_documents_uuid"`
	Owner              *User       `gorm:"default:null;not null"`
	gorm.Model
	Title            string
	GoogleFileID     string `gorm:"index;not null;unique"`
	DocumentType     DocumentType
	Product          Product
	Contributors     []*User `gorm:"many2many:document_contributors;"`
	FileRevisions    []DocumentFileRevision
	Approvers        []*User  `gorm:"many2many:document_reviews;"`
	ApproverGroups   []*Group `gorm:"many2many:document_group_reviews;"`
	RelatedResources []*DocumentRelatedResource
	CustomFields     []*DocumentCustomField
	Status           DocumentStatus
	ProductID        uint `gorm:"index:latest_product_number"`
	DocumentNumber   int  `gorm:"index:latest_product_number"`
	DocumentTypeID   uint
	ShareableAsDraft bool
	Locked           bool
	Imported         bool
}

// Documents is a slice of documents.
type Documents []Document

// DocumentStatus is the status of the document (e.g., "WIP", "In-Review",
// "Approved", "Obsolete").
type DocumentStatus int

const (
	UnspecifiedDocumentStatus DocumentStatus = iota
	WIPDocumentStatus
	InReviewDocumentStatus
	ApprovedDocumentStatus
	ObsoleteDocumentStatus
)

// NewDocumentByFileID creates a Document with the correct file-ID field
// populated based on whether SharePoint is in use.
// When useSharePoint is true it sets FileID; otherwise it sets GoogleFileID.
func NewDocumentByFileID(fileID string, useSharePoint bool) Document {
	if useSharePoint {
		return Document{FileID: fileID}
	}
	return Document{GoogleFileID: fileID}
}

// GetFileIdentifier returns the active file ID regardless of which provider
// is active. SharePoint docs use FileID; Google docs use GoogleFileID.
func (d *Document) GetFileIdentifier() string {
	if d.FileID != "" {
		return d.FileID
	}
	return d.GoogleFileID
}

// hasNoFileID returns true if the document has no file identifier set.
func (d *Document) hasNoFileID() bool {
	return d.GoogleFileID == "" && d.FileID == ""
}

// BeforeCreate validates that every document has at least one file identifier.
func (d *Document) BeforeCreate(tx *gorm.DB) error {
	if d.hasNoFileID() {
		return fmt.Errorf("document must have either GoogleFileID or FileID")
	}
	return nil
}

// BeforeSave is a hook used to find associations before saving.
func (d *Document) BeforeSave(tx *gorm.DB) error {
	if err := d.getAssociations(tx); err != nil {
		return fmt.Errorf("error getting associations: %w", err)
	}

	return nil
}

// Create creates a document in database db.
func (d *Document) Create(db *gorm.DB) error {
	if err := validation.ValidateStruct(d,
		validation.Field(
			&d.ID,
			validation.When(d.hasNoFileID(),
				validation.Required.Error("either ID, GoogleFileID, or FileID is required"),
			),
		),
	); err != nil {
		return err
	}

	if err := d.createAssocations(db); err != nil {
		return fmt.Errorf("error creating associations: %w", err)
	}

	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.
			Model(&d).
			Where(Document{
				GoogleFileID: d.GoogleFileID,
				FileID:       d.FileID,
			}).
			Omit(clause.Associations).
			Create(&d).
			Error; err != nil {
			return err
		}

		if err := d.replaceAssocations(tx); err != nil {
			return fmt.Errorf("error replacing associations: %w", err)
		}

		return nil
	})
}

// Delete deletes a document in database db.
func (d *Document) Delete(db *gorm.DB) error {
	if err := validation.ValidateStruct(d,
		validation.Field(
			&d.ID,
			validation.When(d.hasNoFileID(),
				validation.Required.Error("either ID, GoogleFileID, or FileID is required"),
			),
		),
	); err != nil {
		return err
	}

	query := db.Model(&d)
	if d.GoogleFileID != "" {
		query = query.Where("google_file_id = ?", d.GoogleFileID)
	} else if d.FileID != "" {
		query = query.Where("file_id = ?", d.FileID)
	} else {
		query = query.Where("id = ?", d.ID)
	}

	return query.Delete(&d).Error
}

// Find finds all documents from database db with the provided query, and
// assigns them to the receiver.
func (d *Documents) Find(
	db *gorm.DB, query interface{}, queryArgs ...interface{}) error {

	return db.
		Where(query, queryArgs...).
		Preload(clause.Associations).
		Find(&d).Error
}

// FirstOrCreate finds the first document by Google file ID or creates a new
// record if it does not exist.
// func (d *Document) FirstOrCreate(db *gorm.DB) error {
// 	return db.
// 		Where(Document{FileID: d.FileID}).
// 		Preload(clause.Associations).
// 		FirstOrCreate(&d).Error
// }

// Get gets a document from database db by Google file ID, and assigns it to the
// receiver.
func (d *Document) Get(db *gorm.DB) error {
	if err := validation.ValidateStruct(d,
		validation.Field(
			&d.ID,
			validation.When(d.hasNoFileID(),
				validation.Required.Error("either ID, GoogleFileID, or FileID is required"),
			),
		),
	); err != nil {
		return err
	}

	query := db
	if d.GoogleFileID != "" {
		query = query.Where("google_file_id = ?", d.GoogleFileID)
	} else if d.FileID != "" {
		query = query.Where("file_id = ?", d.FileID)
	} else {
		query = query.Where("id = ?", d.ID)
	}

	if err := query.
		Preload(clause.Associations).
		Preload("RelatedResources", func(db *gorm.DB) *gorm.DB {
			return db.Order("document_related_resources.sort_order ASC")
		}).
		First(&d).
		Error; err != nil {
		return err
	}

	if err := d.getAssociations(db); err != nil {
		return fmt.Errorf("error getting associations: %w", err)
	}

	return nil
}

// GetLatestProductNumber gets the latest document number for a product.
func GetLatestProductNumber(db *gorm.DB,
	documentTypeName, productName string) (int, error) {
	// Validate required fields.
	if err := validation.Validate(db, validation.Required); err != nil {
		return 0, err
	}
	if err := validation.Validate(productName, validation.Required); err != nil {
		return 0, err
	}

	// Get document type.
	dt := DocumentType{
		Name: documentTypeName,
	}
	if err := dt.Get(db); err != nil {
		return 0, fmt.Errorf("error getting document type: %w", err)
	}

	// Get product.
	p := Product{
		Name: productName,
	}
	if err := p.Get(db); err != nil {
		return 0, fmt.Errorf("error getting product: %w", err)
	}

	// Get document with largest document number.
	var d Document
	if err := db.
		Where(Document{
			DocumentTypeID: dt.ID,
			ProductID:      p.ID,
		}).
		Where("document_number IS NOT NULL").
		Order("document_number desc").
		First(&d).
		Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, nil
		}
		return 0, err
	}

	// If highest existing number is below 2000, start from 2000
	// Otherwise continue the existing sequence
	if d.DocumentNumber < 2000 {
		return 1999, nil
	}

	return d.DocumentNumber, nil
}

// GetProjects gets all projects associated with document d.
func (d *Document) GetProjects(db *gorm.DB) ([]Project, error) {
	if err := validation.ValidateStruct(d,
		validation.Field(
			&d.ID,
			validation.When(d.hasNoFileID(),
				validation.Required.Error("either ID, GoogleFileID, or FileID is required"),
			),
		),
	); err != nil {
		return nil, err
	}

	// Get document ID if not known.
	if d.ID == 0 {
		doc := &Document{
			GoogleFileID: d.GoogleFileID,
			FileID:       d.FileID,
		}
		if err := doc.Get(db); err != nil {
			return nil, fmt.Errorf("error getting document: %w", err)
		}
		d.ID = doc.ID
	}

	// Find all projects that have the document as a related resource.
	var projs []Project
	if err := db.Table("projects").
		Joins("JOIN project_related_resources prr ON projects.id = prr.project_id").
		Joins("JOIN project_related_resource_hermes_documents prrhd ON prr.related_resource_id = prrhd.id").
		Where("prr.related_resource_type = ? AND prrhd.document_id = ?", "project_related_resource_hermes_documents", d.ID).
		Find(&projs).Error; err != nil {
		return nil, fmt.Errorf("error getting projects for document: %w", err)
	}

	return projs, nil
}

// ReplaceRelatedResources replaces related resources for document d.
func (d *Document) ReplaceRelatedResources(
	db *gorm.DB,
	elrrs []DocumentRelatedResourceExternalLink,
	hdrrs []DocumentRelatedResourceHermesDocument,
) error {
	if err := d.ensureID(db); err != nil {
		return err
	}

	if err := replaceScopedRelatedResources[DocumentRelatedResource, DocumentRelatedResource](
		db,
		"document_id",
		d.ID,
		len(elrrs),
		func(tx *gorm.DB, i int) error {
			if err := elrrs[i].Create(tx); err != nil {
				return fmt.Errorf(
					"error creating external link related resource: %w", err)
			}

			return nil
		},
		len(hdrrs),
		func(tx *gorm.DB, i int) error {
			if err := hdrrs[i].Create(tx); err != nil {
				return fmt.Errorf(
					"error creating Hermes document related resource: %w", err)
			}

			return nil
		},
	); err != nil {
		return fmt.Errorf("error replacing related resources: %w", err)
	}

	return nil
}

// GetRelatedResources returns typed related resources for document d.
func (d *Document) GetRelatedResources(db *gorm.DB) (
	elrrs []DocumentRelatedResourceExternalLink,
	hdrrs []DocumentRelatedResourceHermesDocument,
	err error,
) {
	if err = validation.ValidateStruct(d,
		validation.Field(
			&d.ID,
			validation.When(d.hasNoFileID(),
				validation.Required.Error("either ID, GoogleFileID, or FileID is required"),
			),
		),
	); err != nil {
		return
	}

	// Get the document.
	if err := d.Get(db); err != nil {
		return nil, nil, fmt.Errorf("error getting document: %w", err)
	}

	// Get related resources.
	for _, rr := range d.RelatedResources {
		switch rr.RelatedResourceType {
		case "document_related_resource_external_links":
			elrr := DocumentRelatedResourceExternalLink{}
			if err := db.
				Where("id = ?", rr.RelatedResourceID).
				Preload(clause.Associations).
				First(&elrr).Error; err != nil {
				return nil,
					nil,
					fmt.Errorf("error getting external link related resource: %w", err)
			}
			elrrs = append(elrrs, elrr)
		case "document_related_resource_hermes_documents":
			hdrr := DocumentRelatedResourceHermesDocument{}
			if err := db.
				Where("id = ?", rr.RelatedResourceID).
				Preload(clause.Associations).
				First(&hdrr).Error; err != nil {
				return nil,
					nil,
					fmt.Errorf(
						"error getting document for Hermes document related resource: %w",
						err)
			}
			hdrrs = append(hdrrs, hdrr)
		default:
			return nil,
				nil,
				fmt.Errorf("unknown related resource type: %s", rr.RelatedResourceType)
		}
	}

	return
}

// Upsert updates or inserts the receiver document into database db.
func (d *Document) Upsert(db *gorm.DB) error {
	if err := validation.ValidateStruct(d,
		validation.Field(
			&d.ID,
			validation.When(d.hasNoFileID(),
				validation.Required.Error("either ID, GoogleFileID, or FileID is required"),
			),
		),
	); err != nil {
		return err
	}

	if err := d.createAssocations(db); err != nil {
		return fmt.Errorf("error creating associations: %w", err)
	}

	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.
			Model(&d).
			Where(Document{
				GoogleFileID: d.GoogleFileID,
				FileID:       d.FileID,
			}).
			Select("*").
			Omit(clause.Associations).
			Assign(*d).
			FirstOrCreate(&d).
			Error; err != nil {
			return err
		}

		if err := d.replaceAssocations(tx); err != nil {
			return fmt.Errorf("error replacing associations: %w", err)
		}

		if err := d.Get(tx); err != nil {
			return fmt.Errorf("error getting the document after upsert: %w", err)
		}

		return nil
	})
}

// createAssocations creates required assocations for a document.
func (d *Document) createAssocations(db *gorm.DB) error {
	approvers, err := resolveUsers(
		db,
		d.Approvers,
		"finding or creating approver",
		func(user *User, db *gorm.DB) error {
			return user.FirstOrCreate(db)
		},
	)
	if err != nil {
		return err
	}
	d.Approvers = approvers

	approverGroups := make([]*Group, 0, len(d.ApproverGroups))
	for i := range d.ApproverGroups {
		if err := d.ApproverGroups[i].FirstOrCreate(db); err != nil {
			return fmt.Errorf("error finding or creating approver groups: %w", err)
		}
		approverGroups = append(approverGroups, d.ApproverGroups[i])
	}
	d.ApproverGroups = approverGroups

	contributors, err := resolveUsers(
		db,
		d.Contributors,
		"finding or creating contributor",
		func(user *User, db *gorm.DB) error {
			return user.FirstOrCreate(db)
		},
	)
	if err != nil {
		return err
	}
	d.Contributors = contributors

	// Get document type if DocumentTypeID is not set.
	if d.DocumentTypeID == 0 && d.DocumentType.Name != "" {
		if err := d.DocumentType.Get(db); err != nil {
			return fmt.Errorf("error getting document type: %w", err)
		}
		d.DocumentTypeID = d.DocumentType.ID
	}

	// Find or create owner.
	if d.Owner != nil && d.Owner.EmailAddress != "" {
		if err := d.Owner.FirstOrCreate(db); err != nil {
			return fmt.Errorf("error finding or creating owner: %w", err)
		}
		d.OwnerID = &d.Owner.ID
	}

	// Get product if ProductID is not set.
	if d.ProductID == 0 && d.Product.Name != "" {
		if err := d.Product.Get(db); err != nil {
			return fmt.Errorf("error getting product: %w", err)
		}
		d.ProductID = d.Product.ID
	}

	return nil
}

// getAssociations gets associations.
func (d *Document) getAssociations(db *gorm.DB) error {
	approvers, err := resolveUsers(
		db,
		d.Approvers,
		"getting approver",
		func(user *User, db *gorm.DB) error {
			return user.Get(db)
		},
	)
	if err != nil {
		return err
	}
	d.Approvers = approvers

	approverGroups, err := resolveGroups(db, d.ApproverGroups, "getting approver group")
	if err != nil {
		return err
	}
	d.ApproverGroups = approverGroups

	contributors, err := resolveUsers(
		db,
		d.Contributors,
		"getting contributor",
		func(user *User, db *gorm.DB) error {
			return user.FirstOrCreate(db)
		},
	)
	if err != nil {
		return err
	}
	d.Contributors = contributors

	// Get document type.
	dt := d.DocumentType
	if err := dt.Get(db); err != nil {
		return fmt.Errorf("error getting document type: %w", err)
	}
	d.DocumentType = dt
	d.DocumentTypeID = dt.ID

	customFields, err := resolveDocumentCustomFields(
		db,
		d.CustomFields,
		d.DocumentType,
		d.DocumentTypeID,
	)
	if err != nil {
		return err
	}
	d.CustomFields = customFields

	// Get owner.
	if d.Owner != nil && d.Owner.EmailAddress != "" {
		if err := d.Owner.Get(db); err != nil {
			return fmt.Errorf("error getting owner: %w", err)
		}
		d.OwnerID = &d.Owner.ID
	}

	// Get product.
	if d.Product.Name != "" {
		if err := d.Product.Get(db); err != nil {
			return fmt.Errorf("error getting product: %w", err)
		}
		d.ProductID = d.Product.ID
	}

	return nil
}

func (d *Document) ensureID(db *gorm.DB) error {
	if err := validation.ValidateStruct(d,
		validation.Field(
			&d.ID,
			validation.When(d.GoogleFileID == "",
				validation.Required.Error("either ID or GoogleFileID is required"),
			),
		),
		validation.Field(
			&d.GoogleFileID,
			validation.When(d.ID == 0,
				validation.Required.Error("either ID or GoogleFileID is required"),
			),
		),
	); err != nil {
		return err
	}

	if d.ID != 0 {
		return nil
	}

	doc := &Document{GoogleFileID: d.GoogleFileID}
	if err := doc.Get(db); err != nil {
		return fmt.Errorf("error getting document: %w", err)
	}

	d.ID = doc.ID
	return nil
}

// replaceAssocations replaces assocations for a document.
func (d *Document) replaceAssocations(db *gorm.DB) error {
	// Replace approvers.
	if err := db.
		Session(&gorm.Session{SkipHooks: true}).
		Model(&d).
		Unscoped().
		Association("Approvers").
		Replace(d.Approvers); err != nil {
		return err
	}

	// Replace approver groups.
	if err := db.
		Session(&gorm.Session{SkipHooks: true}).
		Model(&d).
		Unscoped().
		Association("ApproverGroups").
		Replace(d.ApproverGroups); err != nil {
		return err
	}

	// Replace contributors.
	if err := db.
		Session(&gorm.Session{SkipHooks: true}).
		Model(&d).
		Association("Contributors").
		Replace(d.Contributors); err != nil {
		return err
	}

	// Replace custom fields.
	if err := db.Transaction(func(db *gorm.DB) error {
		if err := validation.ValidateStruct(d,
			validation.Field(
				&d.ID,
				validation.When(d.hasNoFileID(),
					validation.Required.Error("either ID, GoogleFileID, or FileID is required"),
				),
			),
		); err != nil {
			return err
		}

		// Get document ID if not known.
		if d.ID == 0 {
			doc := &Document{
				GoogleFileID: d.GoogleFileID,
				FileID:       d.FileID,
			}
			if err := doc.Get(db); err != nil {
				return fmt.Errorf("error getting document: %w", err)
			}
			d.ID = doc.ID
		}

		// Delete existing DocumentCustomFields.
		if err := db.
			Unscoped(). // Hard delete instead of soft delete.
			Where("document_id = ?", d.ID).
			Delete(&DocumentCustomField{}).Error; err != nil {
			return fmt.Errorf(
				"error deleting existing document custom fields: %w", err)
		}

		// Create all DocumentCustomFields.
		for _, cf := range d.CustomFields {
			cf.DocumentID = d.ID
			if err := cf.Create(db); err != nil {
				return fmt.Errorf(
					"error creating document custom field: %w", err)
			}
		}

		return nil
	}); err != nil {
		return fmt.Errorf("error replacing document custom fields: %w", err)
	}

	return nil
}

// GetDocumentUUID returns the document UUID if set, or generates a new one.
// This is useful during migration when documents don't have UUIDs yet.
func (d *Document) GetDocumentUUID() docid.UUID {
	if d.DocumentUUID != nil && !d.DocumentUUID.IsZero() {
		return *d.DocumentUUID
	}
	return docid.NewUUID()
}

// SetDocumentUUID sets the document UUID.
func (d *Document) SetDocumentUUID(uuid docid.UUID) {
	d.DocumentUUID = &uuid
}

// GetByUUID retrieves a document by its UUID.
func (d *Document) GetByUUID(db *gorm.DB, uuid docid.UUID) error {
	return db.
		Preload(clause.Associations).
		Where("document_uuid = ?", uuid).
		First(&d).
		Error
}

// GetByGoogleFileIDOrUUID retrieves a document by GoogleFileID or UUID.
// Tries UUID first (preferred), falls back to GoogleFileID for backward compatibility.
func (d *Document) GetByGoogleFileIDOrUUID(db *gorm.DB, id string) error {
	// Try parsing as UUID first
	if uuid, err := docid.ParseUUID(id); err == nil {
		if err := d.GetByUUID(db, uuid); err == nil {
			return nil
		}
		// UUID parse succeeded but no document found, fall through to GoogleFileID
	}

	// Fall back to GoogleFileID lookup
	d.GoogleFileID = id
	return d.Get(db)
}

// HasUUID returns true if the document has a UUID assigned.
func (d *Document) HasUUID() bool {
	return d.DocumentUUID != nil && !d.DocumentUUID.IsZero()
}
