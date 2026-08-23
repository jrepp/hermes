// Package models defines the Hermes database models.
package models

import (
	"fmt"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type relatedResourceModel interface {
	relatedResourceRef() relatedResourceReference
}

type relatedResourceReference struct {
	relatedResourceType string
	relatedResourceID   uint
}

func replaceRelatedResourcesTransaction(
	db *gorm.DB,
	findExisting func(*gorm.DB) ([]relatedResourceReference, error),
	deleteAll func(*gorm.DB) error,
	createAll func(*gorm.DB) error,
) error {
	return db.Transaction(func(tx *gorm.DB) error {
		rrs, err := findExisting(tx)
		if err != nil {
			return err
		}

		if err := deleteTypedRelatedResources(tx, rrs); err != nil {
			return err
		}

		if err := deleteAll(tx); err != nil {
			return err
		}

		return createAll(tx)
	})
}

func findRelatedResourceReferences[T relatedResourceModel](
	tx *gorm.DB,
	foreignKey string,
	foreignID uint,
) ([]relatedResourceReference, error) {
	rrs := []T{}
	if err := tx.
		Where(fmt.Sprintf("%s = ?", foreignKey), foreignID).
		Find(&rrs).Error; err != nil {
		return nil, fmt.Errorf("error finding existing related resources: %w", err)
	}

	refs := make([]relatedResourceReference, 0, len(rrs))
	for i := range rrs {
		refs = append(refs, rrs[i].relatedResourceRef())
	}

	return refs, nil
}

func deleteRelatedResources[T any](
	tx *gorm.DB,
	foreignKey string,
	foreignID uint,
) error {
	return tx.
		Unscoped().
		Where(fmt.Sprintf("%s = ?", foreignKey), foreignID).
		Delete(new(T)).Error
}

func createRelatedResources(length int, create func(i int) error) error {
	for i := 0; i < length; i++ {
		if err := create(i); err != nil {
			return err
		}
	}

	return nil
}

func replaceScopedRelatedResources[T relatedResourceModel, D any](
	db *gorm.DB,
	foreignKey string,
	foreignID uint,
	externalCount int,
	createExternal func(*gorm.DB, int) error,
	hermesCount int,
	createHermes func(*gorm.DB, int) error,
) error {
	return replaceRelatedResourcesTransaction(
		db,
		func(tx *gorm.DB) ([]relatedResourceReference, error) {
			return findRelatedResourceReferences[T](tx, foreignKey, foreignID)
		},
		func(tx *gorm.DB) error {
			if err := deleteRelatedResources[D](tx, foreignKey, foreignID); err != nil {
				return fmt.Errorf("error deleting existing related resources: %w", err)
			}

			return nil
		},
		func(tx *gorm.DB) error {
			if err := createRelatedResources(externalCount, func(i int) error {
				return createExternal(tx, i)
			}); err != nil {
				return err
			}

			return createRelatedResources(hermesCount, func(i int) error {
				return createHermes(tx, i)
			})
		},
	)
}

// typedRelatedResourceTables are the tables a related resource may live in.
//
// The table name is interpolated into the statement below, because a table
// cannot be a bind parameter. The value comes from a column the application
// writes rather than from a request, so it is not attacker-controlled today --
// but "today" is the whole of the argument, and an allowlist costs nothing.
// Anything unexpected is an error rather than a query.
var typedRelatedResourceTables = map[string]bool{
	"document_related_resource_external_links":   true,
	"document_related_resource_hermes_documents": true,
	"project_related_resource_external_links":    true,
	"project_related_resource_hermes_documents":  true,
}

func deleteTypedRelatedResources(
	tx *gorm.DB,
	rrs []relatedResourceReference,
) error {
	for i := range rrs {
		table := rrs[i].relatedResourceType
		if !typedRelatedResourceTables[table] {
			return fmt.Errorf(
				"refusing to delete from unknown related resource table %q", table)
		}

		if err := tx.
			Exec(
				fmt.Sprintf("DELETE FROM %q WHERE id = ?", table),
				rrs[i].relatedResourceID,
			).
			Error; err != nil {
			return fmt.Errorf(
				"error deleting existing typed related resources: %w", err)
		}
	}

	return nil
}

func resolveUsers(
	db *gorm.DB,
	users []*User,
	action string,
	resolve func(*User, *gorm.DB) error,
) ([]*User, error) {
	resolved := make([]*User, 0, len(users))
	for i := range users {
		if err := resolve(users[i], db); err != nil {
			return nil, fmt.Errorf("error %s: %w", action, err)
		}
		resolved = append(resolved, users[i])
	}

	return resolved, nil
}

func resolveGroups(
	db *gorm.DB,
	groups []*Group,
	action string,
) ([]*Group, error) {
	resolved := make([]*Group, 0, len(groups))
	for i := range groups {
		if err := groups[i].Get(db); err != nil {
			return nil, fmt.Errorf("error %s: %w", action, err)
		}
		resolved = append(resolved, groups[i])
	}

	return resolved, nil
}

func resolveDocumentCustomFields(
	db *gorm.DB,
	customFields []*DocumentCustomField,
	documentType DocumentType,
	documentTypeID uint,
) ([]*DocumentCustomField, error) {
	resolved := make([]*DocumentCustomField, 0, len(customFields))
	for i := range customFields {
		cf := customFields[i]
		cf.DocumentTypeCustomField.DocumentType = documentType
		cf.DocumentTypeCustomField.DocumentTypeID = documentTypeID

		if cf.DocumentTypeCustomFieldID == 0 {
			if err := cf.DocumentTypeCustomField.Get(db); err != nil {
				return nil, fmt.Errorf(
					"error getting document type custom field: %w", err)
			}
			cf.DocumentTypeCustomFieldID = cf.DocumentTypeCustomField.ID
		} else {
			if err := db.
				First(&cf.DocumentTypeCustomField, cf.DocumentTypeCustomFieldID).
				Error; err != nil {
				return nil, fmt.Errorf(
					"error getting document type custom field by ID: %w", err)
			}
		}

		resolved = append(resolved, cf)
	}

	return resolved, nil
}

func findAssociatedReviews(
	db *gorm.DB,
	document *Document,
	associatedIdentifier string,
	requiredMessage string,
	resolveAssociated func(*gorm.DB) (uint, error),
	buildQuery func(documentID uint, associatedID uint) interface{},
	dest interface{},
) error {
	// A document is identified by whichever field its provider populates:
	// GoogleFileID for Google Drive, FileID for the others. Checking
	// GoogleFileID alone meant a review could never be looked up for a
	// document held by any other provider -- the caller passed a perfectly
	// good identifier and got "GoogleFileID is required".
	documentIdentifier := document.GetFileIdentifier()

	if err := validation.Validate(
		documentIdentifier,
		validation.When(associatedIdentifier == "",
			validation.Required.Error(requiredMessage),
		),
	); err != nil {
		return err
	}
	if err := validation.Validate(
		associatedIdentifier,
		validation.When(documentIdentifier == "",
			validation.Required.Error(requiredMessage),
		),
	); err != nil {
		return err
	}

	documentID := uint(0)
	if documentIdentifier != "" {
		if err := document.Get(db); err != nil {
			return fmt.Errorf("error getting document: %w", err)
		}
		documentID = document.ID
	}

	associatedID := uint(0)
	if associatedIdentifier != "" {
		var err error
		associatedID, err = resolveAssociated(db)
		if err != nil {
			return err
		}
	}

	return db.
		Where(buildQuery(documentID, associatedID)).
		Preload(clause.Associations).
		Find(dest).
		Error
}
