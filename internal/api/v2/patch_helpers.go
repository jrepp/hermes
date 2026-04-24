// Package api provides the Hermes v2 API handlers.
package api

import (
	"fmt"
	"net/http"
	"reflect"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp-forge/hermes/internal/email"
	"github.com/hashicorp-forge/hermes/internal/server"
	"github.com/hashicorp-forge/hermes/pkg/document"
	"github.com/hashicorp-forge/hermes/pkg/models"
)

type customFieldValidationError struct {
	field       document.CustomField
	logMessage  string
	userMessage string
}

func (e customFieldValidationError) Error() string {
	return e.userMessage
}

func validateEditableCustomFields(
	customFields []document.CustomField,
	doc document.Document,
) error {
	for i := range customFields {
		cef, ok := doc.CustomEditableFields[customFields[i].Name]
		if !ok {
			return customFieldValidationError{
				field:       customFields[i],
				logMessage:  "custom field not found",
				userMessage: "Bad request: invalid custom field",
			}
		}
		if customFields[i].DisplayName != cef.DisplayName {
			return customFieldValidationError{
				field:       customFields[i],
				logMessage:  "invalid custom field display name",
				userMessage: "Bad request: invalid custom field display name",
			}
		}
		if customFields[i].Type != cef.Type {
			return customFieldValidationError{
				field:       customFields[i],
				logMessage:  "invalid custom field type",
				userMessage: "Bad request: invalid custom field type",
			}
		}
	}

	return nil
}

func logCustomFieldValidationError(
	logger hclog.Logger,
	err error,
	r *http.Request,
	docID string,
) {
	cfErr, ok := err.(customFieldValidationError)
	if !ok {
		logger.Error("invalid custom field",
			"error", err,
			"method", r.Method,
			"path", r.URL.Path,
			"doc_id", docID,
		)
		return
	}

	args := []any{
		"error", err,
		"method", r.Method,
		"path", r.URL.Path,
		"custom_field", cfErr.field.Name,
		"doc_id", docID,
	}
	if cfErr.field.DisplayName != "" {
		args = append(args, "custom_field_display_name", cfErr.field.DisplayName)
	}
	if cfErr.field.Type != "" {
		args = append(args, "custom_field_type", cfErr.field.Type)
	}

	logger.Error(cfErr.logMessage, args...)
}

func sendNewOwnerNotification(
	r *http.Request,
	srv server.Server,
	docID string,
	oldOwnerEmail string,
	doc document.Document,
) error {
	docURL, err := getDocumentURL(srv.Config.BaseURL, docID)
	if err != nil {
		return fmt.Errorf("error getting document URL: %w", err)
	}

	newOwner := email.User{EmailAddress: doc.Owners[0]}
	ppl, err := srv.WorkspaceProvider.SearchPeople(r.Context(), doc.Owners[0])
	if err != nil {
		srv.Logger.Warn("error searching directory for new owner",
			"error", err,
			"method", r.Method,
			"path", r.URL.Path,
			"doc_id", docID,
			"person", doc.Owners[0],
		)
	}
	if len(ppl) == 1 {
		newOwner.Name = ppl[0].DisplayName
	}

	oldOwner := email.User{EmailAddress: oldOwnerEmail}
	ppl, err = srv.WorkspaceProvider.SearchPeople(r.Context(), oldOwnerEmail)
	if err != nil {
		srv.Logger.Warn("error searching directory for old owner",
			"error", err,
			"method", r.Method,
			"path", r.URL.Path,
			"doc_id", docID,
			"person", doc.Owners[0],
		)
	}
	if len(ppl) == 1 {
		oldOwner.Name = ppl[0].DisplayName
	}

	if err := email.SendNewOwnerEmail(
		email.NewOwnerEmailData{
			BaseURL:           srv.Config.BaseURL,
			DocumentShortName: doc.DocNumber,
			DocumentStatus:    doc.Status,
			DocumentTitle:     doc.Title,
			DocumentType:      doc.DocType,
			DocumentURL:       docURL,
			NewDocumentOwner:  newOwner,
			OldDocumentOwner:  oldOwner,
			Product:           doc.Product,
		},
		[]string{doc.Owners[0]},
		srv.Config.Email.FromAddress,
		getCompatProvider(srv.WorkspaceProvider),
	); err != nil {
		return fmt.Errorf("error sending new owner email: %w", err)
	}

	return nil
}

func usersFromEmails(emails []string) []*models.User {
	users := make([]*models.User, 0, len(emails))
	for i := range emails {
		users = append(users, &models.User{EmailAddress: emails[i]})
	}

	return users
}

func groupsFromEmails(emails []string) []*models.Group {
	groups := make([]*models.Group, len(emails))
	for i := range emails {
		groups[i] = &models.Group{EmailAddress: emails[i]}
	}

	return groups
}

func setModelCustomFieldDocumentIDs(
	customFields []*models.DocumentCustomField,
	documentID uint,
) {
	for i := range customFields {
		customFields[i].DocumentID = documentID
	}
}

func updateModelCustomFields(
	customFields []*models.DocumentCustomField,
	docType string,
	patchFields []document.CustomField,
) ([]*models.DocumentCustomField, error) {
	updated := customFields
	for i := range patchFields {
		field := patchFields[i]
		switch field.Type {
		case "STRING":
			value, ok := field.Value.(string)
			if !ok {
				return nil, customFieldValidationError{
					field:       field,
					logMessage:  "invalid value type for string custom field",
					userMessage: fmt.Sprintf("Bad request: invalid value type for custom field %q", field.Name),
				}
			}
			updated = models.UpsertStringDocumentCustomField(
				updated,
				docType,
				field.DisplayName,
				value,
			)
		case "PEOPLE":
			values, err := parsePeopleCustomFieldValue(field)
			if err != nil {
				return nil, err
			}

			updated, err = models.UpsertStringSliceDocumentCustomField(
				updated,
				docType,
				field.DisplayName,
				values,
			)
			if err != nil {
				return nil, customFieldValidationError{
					field:       field,
					logMessage:  "invalid value type for people custom field",
					userMessage: fmt.Sprintf("Bad request: invalid value type for custom field %q", field.Name),
				}
			}
		default:
			return nil, customFieldValidationError{
				field:       field,
				logMessage:  "invalid custom field type",
				userMessage: fmt.Sprintf("Bad request: invalid type for custom field %q", field.Name),
			}
		}
	}

	return updated, nil
}

func parsePeopleCustomFieldValue(field document.CustomField) ([]string, error) {
	if reflect.TypeOf(field.Value).Kind() != reflect.Slice {
		return nil, customFieldValidationError{
			field:       field,
			logMessage:  "invalid value type for people custom field",
			userMessage: fmt.Sprintf("Bad request: invalid value type for custom field %q", field.Name),
		}
	}

	values, ok := field.Value.([]any)
	if !ok {
		return nil, customFieldValidationError{
			field:       field,
			logMessage:  "invalid value type for people custom field",
			userMessage: fmt.Sprintf("Bad request: invalid value type for custom field %q", field.Name),
		}
	}

	parsed := make([]string, 0, len(values))
	for i := range values {
		value, ok := values[i].(string)
		if !ok {
			return nil, customFieldValidationError{
				field:       field,
				logMessage:  "invalid value type for people custom field",
				userMessage: fmt.Sprintf("Bad request: invalid value type for custom field %q", field.Name),
			}
		}
		parsed = append(parsed, value)
	}

	return parsed, nil
}
