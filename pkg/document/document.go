package document

import (
	"encoding/json"
	"fmt"
	"net/mail"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/iancoleman/strcase"
	"github.com/mitchellh/mapstructure"

	"github.com/hashicorp-forge/hermes/internal/config"
	"github.com/hashicorp-forge/hermes/internal/helpers"
	"github.com/hashicorp-forge/hermes/pkg/models"
)

const (
	// FieldTypePeople represents a people custom field type.
	FieldTypePeople = "PEOPLE"
	// FieldTypeString represents a string custom field type.
	FieldTypeString = "STRING"
)

// Document represents a Hermes document.
type Document struct {
	CustomEditableFields map[string]CustomDocTypeField `json:"customEditableFields,omitempty"`
	FileRevisions        map[string]string             `json:"fileRevisions,omitempty"`
	Title                string                        `json:"title,omitempty"`
	DocType              string                        `json:"docType,omitempty"`
	DocNumber            string                        `json:"docNumber,omitempty"`
	ThumbnailLink        string                        `json:"thumbnailLink,omitempty"`
	Status               string                        `json:"status,omitempty"`
	Summary              string                        `json:"summary,omitempty"`
	Product              string                        `json:"product,omitempty"`
	ObjectID             string                        `json:"objectID,omitempty"`
	Content              string                        `json:"content,omitempty"`
	Created              string                        `json:"created,omitempty"`
	MetaTags             []string                      `json:"_tags,omitempty"`
	ApproverGroups       []string                      `json:"approverGroups,omitempty"`
	CustomFields         []CustomField                 `json:"customFields,omitempty"`
	Contributors         []string                      `json:"contributors,omitempty"`
	LinkedDocs           []string                      `json:"linkedDocs,omitempty"`
	Tags                 []string                      `json:"tags,omitempty"`
	ChangesRequestedBy   []string                      `json:"changesRequestedBy,omitempty"`
	ApprovedBy           []string                      `json:"approvedBy,omitempty"`
	Owners               []string                      `json:"owners,omitempty"`
	OwnerPhotos          []string                      `json:"ownerPhotos,omitempty"`
	Approvers            []string                      `json:"approvers,omitempty"`
	CreatedTime          int64                         `json:"createdTime,omitempty"`
	ModifiedTime         int64                         `json:"modifiedTime,omitempty"`
	Archived             bool                          `json:"archived,omitempty"`
	Locked               bool                          `json:"locked,omitempty"`
	AppCreated           bool                          `json:"appCreated,omitempty"`
}

// CustomDocTypeField represents a custom field definition for a document type.
type CustomDocTypeField struct {
	// DisplayName is the display name of the custom document-type field.
	DisplayName string `json:"displayName"`

	// Type is the type of the custom document-type field. It is used by the
	// frontend to display the proper input component.
	// Valid values: "PEOPLE", "STRING".
	Type string `json:"type"`
}

// CustomField represents a custom field value on a document.
type CustomField struct {
	Value       any
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Type        string `json:"type"`
}

// NewFromAlgoliaObject creates a document from a document Algolia object.
//
//nolint:gocognit,gocyclo // large Algolia-to-Go struct mapping
func NewFromAlgoliaObject(
	in map[string]any, docTypes []*config.DocumentType) (*Document, error) {

	doc := &Document{}

	if err := mapstructure.Decode(in, &doc); err != nil {
		return nil, fmt.Errorf("error decoding to document: %w", err)
	}

	// Build CustomFields and CustomEditableFields.
	// Note: This is redundant but we're doing this to maintain compatibility with
	// the way these documents have been defined. This will change with the next
	// version of the API.
	cefs := make(map[string]CustomDocTypeField)
	cfs := []CustomField{}

	objDocType, ok := in["docType"]
	if !ok {
		return nil, fmt.Errorf("docType not found in object")
	}

	foundDocType := false
	for _, dt := range docTypes {
		if dt.Name == objDocType {
			foundDocType = true
			for _, cf := range dt.CustomFields {
				ccName := strcase.ToLowerCamel(cf.Name)
				switch cf.Type {
				case "string":
					if v, ok := in[ccName]; ok {
						if v, ok := v.(string); ok {
							cfs = append(cfs, CustomField{
								Name:        ccName,
								DisplayName: cf.Name,
								Type:        FieldTypeString,
								Value:       v,
							})
						} else {
							return nil, fmt.Errorf(
								"wrong type for custom field key %q, want string", ccName)
						}
					}
					cefs[ccName] = CustomDocTypeField{
						DisplayName: cf.Name,
						Type:        FieldTypeString,
					}
				case "people":
					cfVal := []string{}
					if v, ok := in[ccName]; ok {
						if reflect.TypeOf(v).Kind() == reflect.Slice {
							slice, ok := v.([]any)
							if !ok {
								return nil, fmt.Errorf(
									"wrong type for custom field key %q, want []string",
									ccName)
							}
							for _, vv := range slice {
								if vv, ok := vv.(string); ok {
									cfVal = append(cfVal, vv)
								} else {
									return nil, fmt.Errorf(
										"wrong type for custom field key %q, want []string",
										ccName)
								}
							}
							cfs = append(cfs, CustomField{
								Name:        ccName,
								DisplayName: cf.Name,
								Type:        FieldTypePeople,
								Value:       cfVal,
							})
						} else {
							return nil, fmt.Errorf(
								"wrong type for custom field key %q, want []string", dt.Name)
						}
					}
					cefs[ccName] = CustomDocTypeField{
						DisplayName: cf.Name,
						Type:        FieldTypePeople,
					}
				default:
					return nil, fmt.Errorf(
						"unknown type for custom field key %q: %s", dt.Name, cf.Type)
				}
			}
			break
		}
	}
	if !foundDocType {
		return nil, fmt.Errorf("invalid doc type: %s", objDocType)
	}
	doc.CustomFields = cfs
	doc.CustomEditableFields = cefs

	return doc, nil
}

// NewFromDatabaseModel creates a document from a document database model.
//
//nolint:gocyclo // maps many DB fields to document struct
func NewFromDatabaseModel(
	model models.Document,
	reviews models.DocumentReviews,
	groupReviews models.DocumentGroupReviews,
) (*Document, error) {
	doc := &Document{}

	// ObjectID.
	doc.ObjectID = model.GetFileIdentifier()

	// Title.
	doc.Title = model.Title

	// DocType.
	doc.DocType = model.DocumentType.Name

	// DocNumber.
	doc.DocNumber = fmt.Sprintf(
		"%s-%03d", model.Product.Abbreviation, model.DocumentNumber)
	if model.DocumentNumber == 0 {
		doc.DocNumber = fmt.Sprintf("%s-???", model.Product.Abbreviation)
	}

	// AppCreated.
	doc.AppCreated = !model.Imported

	// ApprovedBy, Approvers, ChangesRequestedBy.
	var approvedBy, changesRequestedBy []string
	approvers := make([]string, 0, len(reviews))
	for i := range reviews {
		r := &reviews[i]
		approvers = append(approvers, r.User.EmailAddress)

		switch r.Status {
		case models.ApprovedDocumentReviewStatus:
			approvedBy = append(approvedBy, r.User.EmailAddress)
		case models.ChangesRequestedDocumentReviewStatus:
			changesRequestedBy = append(changesRequestedBy, r.User.EmailAddress)
		}
	}
	doc.ApprovedBy = approvedBy
	doc.Approvers = approvers
	doc.ChangesRequestedBy = changesRequestedBy

	// ApproverGroups.
	approverGroups := make([]string, 0, len(groupReviews))
	for i := range groupReviews {
		r := &groupReviews[i]
		approverGroups = append(approverGroups, r.Group.EmailAddress)
	}
	doc.ApproverGroups = approverGroups

	// Contributors.
	contributors := make([]string, 0, len(model.Contributors))
	for _, c := range model.Contributors {
		contributors = append(contributors, c.EmailAddress)
	}
	doc.Contributors = contributors

	// Created.
	doc.Created = model.DocumentCreatedAt.Format("Jan 2, 2006")

	// CreatedTime.
	doc.CreatedTime = model.DocumentCreatedAt.Unix()

	// CustomEditableFields.
	customEditableFields := make(map[string]CustomDocTypeField)
	for i := range model.DocumentType.CustomFields {
		c := &model.DocumentType.CustomFields[i]
		var cType string
		switch c.Type {
		case models.PeopleDocumentTypeCustomFieldType:
			cType = FieldTypePeople
		case models.PersonDocumentTypeCustomFieldType:
			cType = "PERSON"
		case models.StringDocumentTypeCustomFieldType:
			cType = FieldTypeString
		}
		customEditableFields[strcase.ToLowerCamel(c.Name)] = CustomDocTypeField{
			DisplayName: c.Name,
			Type:        cType,
		}
	}
	doc.CustomEditableFields = customEditableFields

	// CustomFields.
	var customFields []CustomField
	for _, c := range model.CustomFields {
		cf := CustomField{
			Name:        strcase.ToLowerCamel(c.DocumentTypeCustomField.Name),
			DisplayName: c.DocumentTypeCustomField.Name,
		}
		switch c.DocumentTypeCustomField.Type {
		case models.PeopleDocumentTypeCustomFieldType:
			cf.Type = FieldTypePeople
			var val []string
			if err := json.Unmarshal([]byte(c.Value), &val); err != nil {
				return nil, fmt.Errorf("error unmarshaling value for field %q: %w",
					c.DocumentTypeCustomField.Name, err)
			}
			cf.Value = val
		case models.PersonDocumentTypeCustomFieldType:
			cf.Type = "PERSON"
			cf.Value = c.Value
		case models.StringDocumentTypeCustomFieldType:
			cf.Type = FieldTypeString
			cf.Value = c.Value
		}
		customFields = append(customFields, cf)
	}
	doc.CustomFields = customFields

	// FileRevisions.
	fileRevisions := make(map[string]string)
	for i := range model.FileRevisions {
		fr := &model.FileRevisions[i]
		fileRevisions[fr.RevisionKey()] = fr.Name
	}
	doc.FileRevisions = fileRevisions

	// Locked.
	doc.Locked = model.Locked

	// ModifiedTime.
	doc.ModifiedTime = model.DocumentModifiedAt.Unix()

	// Owners.
	if model.Owner != nil {
		doc.Owners = []string{model.Owner.EmailAddress}
	} else {
		doc.Owners = []string{}
	}

	// Note: OwnerPhotos is not stored in the database.

	// Product.
	doc.Product = model.Product.Name

	// Summary.
	if model.Summary != nil {
		doc.Summary = *model.Summary
	}

	// Status.
	var status string
	switch model.Status {
	case models.ApprovedDocumentStatus:
		status = "Approved"
	case models.InReviewDocumentStatus:
		status = "In-Review"
	case models.ObsoleteDocumentStatus:
		status = "Obsolete"
	case models.WIPDocumentStatus:
		status = "WIP"
	}
	doc.Status = status

	// Archived.
	doc.Archived = model.Archived

	// Note: ThumbnailLink is not stored in the database.

	return doc, nil
}

// ToAlgoliaObject converts a document to a document Algolia object.
func (d Document) ToAlgoliaObject(
	removeCustomEditableFields bool) (map[string]any, error) {

	// Remove CustomEditableFields, if configured.
	if removeCustomEditableFields {
		d.CustomEditableFields = nil
	}

	// Save and remove custom fields.
	cfs := d.CustomFields
	d.CustomFields = nil

	// Convert to Algolia object by marshaling to JSON and unmarshaling back.
	var obj map[string]any
	bytes, err := json.Marshal(d)
	if err != nil {
		return nil, fmt.Errorf("error marshaling document object to JSON: %w", err)
	}
	if err := json.Unmarshal(bytes, &obj); err != nil {
		return nil, fmt.Errorf("error unmarshaling JSON to object: %w", err)
	}

	// Set custom fields.
	for _, cf := range cfs {
		obj[cf.Name] = cf.Value
	}

	return obj, nil
}

// ToDatabaseModels converts a document to a document and document reviews
// database records.
//
//nolint:gocognit,gocyclo // maps many document fields to DB models
func (d Document) ToDatabaseModels(
	docTypes []*config.DocumentType, products []*config.Product,
	useSharePoint bool,
) (
	models.Document, models.DocumentReviews, error,
) {
	doc := models.NewDocumentByFileID(d.ObjectID, useSharePoint)
	reviews := models.DocumentReviews{}

	// Title.
	doc.Title = d.Title

	// DocumentType.Name.
	foundDocType := false
	for _, dt := range docTypes {
		if dt.Name == d.DocType {
			foundDocType = true
			doc.DocumentType.Name = dt.Name
			break
		}
	}
	if !foundDocType {
		return doc, reviews, fmt.Errorf("document type not found: %s", d.DocType)
	}

	// DocumentNumber.
	splitDocNum := strings.Split(d.DocNumber, "-")
	if len(splitDocNum) == 2 {
		docNumInt, err := strconv.Atoi(splitDocNum[1])
		if err == nil {
			doc.DocumentNumber = docNumInt
		}
	}

	// Imported.
	doc.Imported = !d.AppCreated

	// Contributors.
	var contributors []*models.User
	for _, c := range d.Contributors {
		// Validate email address.
		if _, err := mail.ParseAddress(c); err == nil {
			u := &models.User{
				EmailAddress: c,
			}
			contributors = append(contributors, u)
		}
	}
	doc.Contributors = contributors

	// DocumentCreatedAt.
	doc.DocumentCreatedAt = time.Unix(d.CreatedTime, 0)

	// CustomFields.
	customFields := []*models.DocumentCustomField{}
	for _, cf := range d.CustomFields {
		switch cf.Type {
		case FieldTypeString:
			if v, ok := cf.Value.(string); ok {
				customFields = append(customFields, &models.DocumentCustomField{
					DocumentTypeCustomField: models.DocumentTypeCustomField{
						Name: cf.DisplayName,
						DocumentType: models.DocumentType{
							Name: doc.DocumentType.Name,
						},
					},
					Value: v,
				})
			}
		case "PEOPLE":
			if reflect.TypeOf(cf.Value).Kind() == reflect.Slice {
				if v, ok := cf.Value.([]string); ok {
					cfValJSON, err := json.Marshal(v)
					if err != nil {
						return doc, reviews, fmt.Errorf(
							"error marshaling custom field value to JSON: %w", err)
					}
					customFields = append(customFields, &models.DocumentCustomField{
						DocumentTypeCustomField: models.DocumentTypeCustomField{
							Name: cf.DisplayName,
							DocumentType: models.DocumentType{
								Name: doc.DocumentType.Name,
							},
						},
						Value: string(cfValJSON),
					})
				}
			}
		}
	}
	doc.CustomFields = customFields

	// FileRevisions.
	fileRevisions := models.DocumentFileRevisions{}
	for frID, frName := range d.FileRevisions {
		fileRevisions = append(fileRevisions, models.DocumentFileRevision{
			Document:       models.NewDocumentByFileID(doc.GetFileIdentifier(), useSharePoint),
			FileRevisionID: frID,
			Name:           frName,
		})
	}
	doc.FileRevisions = fileRevisions

	// Locked.
	doc.Locked = d.Locked

	// DocumentModifiedAt.
	doc.DocumentModifiedAt = time.Unix(d.ModifiedTime, 0)

	// Owners.
	if len(d.Owners) > 0 {
		doc.Owner = &models.User{
			EmailAddress: d.Owners[0],
		}
	}

	// Note: OwnerPhotos is not stored in the database.

	// Product.
	foundProduct := false
	for _, p := range products {
		if p.Name == d.Product {
			foundProduct = true
			doc.Product.Name = d.Product
			doc.Product.Abbreviation = p.Abbreviation
		}
	}
	if !foundProduct {
		return doc, reviews, fmt.Errorf("product not found: %s", d.Product)
	}

	// Summary.
	summary := d.Summary
	doc.Summary = &summary

	// Status.
	switch strings.ToLower(d.Status) {
	case "wip":
		doc.Status = models.WIPDocumentStatus
	case "in review", "in-review":
		doc.Status = models.InReviewDocumentStatus
	case "approved":
		doc.Status = models.ApprovedDocumentStatus
	case "obsolete":
		doc.Status = models.ObsoleteDocumentStatus
	}

	// Note: ThumbnailLink is not stored in the database.

	// Build approvers and document reviews.
	var approvers []*models.User
	for _, a := range d.Approvers {
		u := models.User{
			EmailAddress: a,
		}
		// Validate email address.
		if _, err := mail.ParseAddress(u.EmailAddress); err != nil {
			continue
		}
		approvers = append(approvers, &u)

		if helpers.StringSliceContains(d.ApprovedBy, a) {
			reviews = append(reviews, models.DocumentReview{
				Document: models.NewDocumentByFileID(d.ObjectID, useSharePoint),
				User:     u,
				Status:   models.ApprovedDocumentReviewStatus,
			})
		} else if helpers.StringSliceContains(d.ChangesRequestedBy, a) {
			reviews = append(reviews, models.DocumentReview{
				Document: models.NewDocumentByFileID(d.ObjectID, useSharePoint),
				User:     u,
				Status:   models.ChangesRequestedDocumentReviewStatus,
			})
		}
	}
	doc.Approvers = approvers

	// Approver groups.
	var approverGroups []*models.Group
	for _, a := range d.ApproverGroups {
		g := models.Group{
			EmailAddress: a,
		}
		// Validate email address.
		if _, err := mail.ParseAddress(g.EmailAddress); err != nil {
			continue
		}
		approverGroups = append(approverGroups, &g)
	}
	doc.ApproverGroups = approverGroups

	return doc, reviews, nil
}

// UpsertCustomField adds or updates a custom field value.
//
//nolint:gocognit,gocyclo // handles multiple custom field types
func (d *Document) UpsertCustomField(cf CustomField) error {
	// Build new document CustomFields.
	var newCFs []CustomField
	foundCF := false
	for _, docCF := range d.CustomFields {
		if docCF.Name == cf.Name {
			// Validate rest of custom field.
			if cf.DisplayName != docCF.DisplayName {
				return fmt.Errorf("incorrect display name for custom field")
			}
			switch cf.Type {
			case FieldTypePeople:
				if reflect.TypeOf(cf.Value).Kind() != reflect.Slice {
					return fmt.Errorf("incorrect value type for custom field")
				}
				slice, ok := cf.Value.([]any)
				if !ok {
					return fmt.Errorf("incorrect value type for custom field")
				}
				for _, v := range slice {
					// Make sure slice is a string slice.
					if _, ok := v.(string); !ok {
						return fmt.Errorf("incorrect value type for custom field")
					}
				}
				// If the value is empty, remove it from the document (by not appending
				// here).
				if len(slice) > 0 {
					newCFs = append(newCFs, cf)
				}
			case FieldTypeString:
				v, ok := cf.Value.(string)
				if !ok {
					return fmt.Errorf("incorrect value type for custom field")
				}
				// If the value is empty, remove it from the document (by not appending
				// here).
				if v != "" {
					newCFs = append(newCFs, cf)
				}
			}
			foundCF = true
		} else {
			newCFs = append(newCFs, docCF)
		}
	}

	// If we didn't find the custom field, insert it.
	if !foundCF {
		newCFs = append(newCFs, cf)
	}

	d.CustomFields = newCFs

	return nil
}

// DeleteFileRevision removes a file revision from the document.
func (d *Document) DeleteFileRevision(revisionID string) {
	delete(d.FileRevisions, revisionID)
}

// SetFileRevision adds or updates a file revision on the document.
func (d *Document) SetFileRevision(revisionID, revisionName string) {
	if d.FileRevisions == nil {
		d.FileRevisions = map[string]string{
			revisionID: revisionName,
		}
	} else {
		d.FileRevisions[revisionID] = revisionName
	}
}

// GetStringValue extracts a string value from a map by key.
func GetStringValue(in map[string]any, key string) (string, error) {
	v, ok := in[key]
	if !ok {
		return "", fmt.Errorf("key %q not found", key)
	}
	sv, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("wrong type for key %q, want string", key)
	}
	return sv, nil
}

// GetStringSliceValue extracts a string slice value from a map by key.
func GetStringSliceValue(in map[string]any, key string) ([]string, error) {
	ret := []string{}
	v, ok := in[key]
	if !ok {
		return nil, fmt.Errorf("key %q not found", key)
	}
	if reflect.TypeOf(v).Kind() != reflect.Slice {
		return nil, fmt.Errorf("wrong type for key %q, want []string", key)
	}
	slice, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("wrong type for key %q, want []string", key)
	}
	for _, vv := range slice {
		if vv, ok := vv.(string); ok {
			ret = append(ret, vv)
		} else {
			return nil, fmt.Errorf("wrong type for key %q, want []string", key)
		}
	}
	return ret, nil
}
