package api

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"github.com/hashicorp/go-multierror"
	"github.com/iancoleman/strcase"
	"github.com/stretchr/testify/assert"

	"github.com/hashicorp-forge/hermes/internal/config"
	"github.com/hashicorp-forge/hermes/pkg/models"
)

// compareSlices compares the first slice with the second
// and returns the elements that exist in the second slice
// that don't exist in the first
func compareSlices(a, b []string) []string {
	// Create a map with the length of slice "a"
	tempA := make(map[string]bool, len(a))
	for _, j := range a {
		tempA[j] = true
	}

	diffElems := []string{}
	for _, k := range b {
		// If elements in slice "b" are
		// not present in slice "a" then
		// append to diffElems slice
		if !tempA[k] {
			diffElems = append(diffElems, k)
		}
	}

	return diffElems
}

// parseResourceIDFromURL parses a URL path with the format
// "/api/v2/{apiPath}/{resourceID}" and returns the resource ID.
func parseResourceIDFromURL(url, apiPath string) (string, error) {
	// Remove API path from URL.
	url = strings.TrimPrefix(url, fmt.Sprintf("/api/v2/%s", apiPath))

	// Remove empty entries and validate path.
	urlPath := strings.Split(url, "/")
	var resultPath []string
	for _, v := range urlPath {
		// Only append non-empty values, this removes any empty strings in the
		// slice.
		if v != "" {
			resultPath = append(resultPath, v)
		}
	}
	resultPathLen := len(resultPath)
	// Only allow 1 value to be set in the resultPath slice. For example, if the
	// urlPath is set to "/{document_id}" then the resultPath slice would be
	// ["{document_id}"].
	if resultPathLen > 1 {
		return "", fmt.Errorf("invalid URL path")
	}
	// If there are no entries in the resultPath slice, then there was no resource
	// ID set in the URL path. Return an empty string.
	if resultPathLen == 0 {
		return "", fmt.Errorf("no document ID set in url path")
	}

	// Return resource ID.
	return resultPath[0], nil
}

// fakeT fulfills the assert.TestingT interface so we can use
// assert.ElementsMatch.
type fakeT struct{}

func (t fakeT) Errorf(string, ...interface{}) {}

// compareAlgoliaAndDatabaseDocument compares data for a document stored in
// Algolia and the database to determine any inconsistencies, which are returned
// back as a (multierror) error.
func compareAlgoliaAndDatabaseDocument(
	algoDoc map[string]any,
	dbDoc models.Document,
	dbDocReviews models.DocumentReviews,
	docTypes []*config.DocumentType,
) error {

	var result *multierror.Error

	// Compare objectID.
	algoGoogleFileID, err := getStringValue(algoDoc, "objectID")
	if err != nil {
		result = multierror.Append(
			result, fmt.Errorf("error getting objectID value: %w", err))
	}
	if algoGoogleFileID != dbDoc.GoogleFileID {
		result = multierror.Append(result,
			fmt.Errorf(
				"objectID not equal, algolia=%v, db=%v",
				algoGoogleFileID, dbDoc.GoogleFileID),
		)
	}

	// Compare title.
	algoTitle, err := getStringValue(algoDoc, "title")
	if err != nil {
		result = multierror.Append(
			result, fmt.Errorf("error getting title value: %w", err))
	} else {
		if algoTitle != dbDoc.Title {
			result = multierror.Append(result,
				fmt.Errorf(
					"title not equal, algolia=%v, db=%v",
					algoTitle, dbDoc.Title),
			)
		}
	}

	// Compare docType.
	algoDocType, err := getStringValue(algoDoc, "docType")
	if err != nil {
		result = multierror.Append(
			result, fmt.Errorf("error getting docType value: %w", err))
	} else {
		dbDocType := dbDoc.DocumentType.Name
		if algoDocType != dbDocType {
			result = multierror.Append(result,
				fmt.Errorf(
					"docType not equal, algolia=%v, db=%v",
					algoDocType, dbDocType),
			)
		}
	}

	// Compare docNumber.
	algoDocNumber, err := getStringValue(algoDoc, "docNumber")
	if err != nil {
		result = multierror.Append(
			result, fmt.Errorf("error getting docNumber value: %w", err))
	} else {
		// Replace "-???" (how draft doc numbers are defined in Algolia) with a
		// zero.
		re := regexp.MustCompile(`-\?\?\?$`)
		algoDocNumber = re.ReplaceAllString(algoDocNumber, "-000")

		var dbDocNumber string
		// If document number in Algolia isn't empty, build the database document
		// number. If it is empty, we expect the database document number to be
		// empty too.
		if algoDocNumber != "" {
			// Note that we pad the database document number to three digits here like
			// we do when assigning a document number when a doc review is requested.
			dbDocNumber = fmt.Sprintf(
				"%s-%03d", dbDoc.Product.Abbreviation, dbDoc.DocumentNumber)
		}
		if algoDocNumber != dbDocNumber {
			// Some legacy documents may not have the three digit number padding so
			// check that too.
			dbDocNumberNoPadding := fmt.Sprintf(
				"%s-%d", dbDoc.Product.Abbreviation, dbDoc.DocumentNumber)
			if algoDocNumber != dbDocNumberNoPadding {
				result = multierror.Append(result,
					fmt.Errorf(
						"docNumber not equal, algolia=%v, db=%v",
						algoDocNumber, dbDocNumber),
				)
			}
		}
	}

	// Compare appCreated.
	algoAppCreated, err := getBooleanValue(algoDoc, "appCreated")
	if err != nil {
		result = multierror.Append(
			result, fmt.Errorf("error getting appCreated value: %w", err))
	} else {
		dbAppCreated := !dbDoc.Imported
		if algoAppCreated != dbAppCreated {
			result = multierror.Append(result,
				fmt.Errorf(
					"appCreated not equal, algolia=%v, db=%v",
					algoAppCreated, dbAppCreated),
			)
		}
	}

	// Compare approvedBy.
	algoApprovedBy, err := getStringSliceValue(algoDoc, "approvedBy")
	if err != nil {
		result = multierror.Append(
			result, fmt.Errorf("error getting approvedBy value: %w", err))
	}
	dbApprovedBy := []string{}
	for _, r := range dbDocReviews {
		if r.Status == models.ApprovedDocumentReviewStatus {
			dbApprovedBy = append(dbApprovedBy, r.User.EmailAddress)
		}
	}
	if !assert.ElementsMatch(fakeT{}, algoApprovedBy, dbApprovedBy) {
		result = multierror.Append(result,
			fmt.Errorf(
				"approvedBy not equal, algolia=%v, db=%v",
				algoApprovedBy, dbApprovedBy),
		)
	}

	// Compare approvers.
	algoApprovers, err := getStringSliceValue(algoDoc, "approvers")
	if err != nil {
		result = multierror.Append(
			result, fmt.Errorf("error getting approvers value: %w", err))
	}
	dbApprovers := []string{}
	for _, a := range dbDoc.Approvers {
		dbApprovers = append(dbApprovers, a.EmailAddress)
	}
	if !assert.ElementsMatch(fakeT{}, algoApprovers, dbApprovers) {
		result = multierror.Append(result,
			fmt.Errorf(
				"approvers not equal, algolia=%v, db=%v",
				algoApprovers, dbApprovers),
		)
	}

	// Compare changesRequestedBy.
	algoChangesRequestedBy, err := getStringSliceValue(
		algoDoc, "changesRequestedBy")
	if err != nil {
		result = multierror.Append(
			result, fmt.Errorf("error getting changesRequestedBy value: %w", err))
	}
	dbChangesRequestedBy := []string{}
	for _, r := range dbDocReviews {
		if r.Status == models.ChangesRequestedDocumentReviewStatus {
			dbChangesRequestedBy = append(dbChangesRequestedBy, r.User.EmailAddress)
		}
	}
	if !assert.ElementsMatch(
		fakeT{}, algoChangesRequestedBy, dbChangesRequestedBy,
	) {
		result = multierror.Append(result,
			fmt.Errorf(
				"changesRequestedBy not equal, algolia=%v, db=%v",
				algoChangesRequestedBy, dbChangesRequestedBy),
		)
	}

	// Compare contributors.
	algoContributors, err := getStringSliceValue(algoDoc, "contributors")
	if err != nil {
		result = multierror.Append(
			result, fmt.Errorf("error getting contributors value: %w", err))
	}
	dbContributors := []string{}
	for _, c := range dbDoc.Contributors {
		dbContributors = append(dbContributors, c.EmailAddress)
	}
	if !assert.ElementsMatch(fakeT{}, algoContributors, dbContributors) {
		result = multierror.Append(result,
			fmt.Errorf(
				"contributors not equal, algolia=%v, db=%v",
				algoContributors, dbContributors),
		)
	}

	// Compare createdTime.
	algoCreatedTime, err := getInt64Value(algoDoc, "createdTime")
	if err != nil {
		result = multierror.Append(
			result, fmt.Errorf("error getting createdTime value: %w", err))
	} else {
		dbCreatedTime := dbDoc.DocumentCreatedAt.Unix()
		if algoCreatedTime != dbCreatedTime {
			result = multierror.Append(result,
				fmt.Errorf(
					"createdTime not equal, algolia=%v, db=%v",
					algoCreatedTime, dbCreatedTime),
			)
		}
	}

	// Compare custom fields.
	if err := compareCustomFields(docTypes, algoDoc, algoDocType, dbDoc, "algolia"); err != nil {
		result = multierror.Append(result, err)
	}

	// Compare fileRevisions.
	algoFileRevisions, err := getMapStringStringValue(algoDoc, "fileRevisions")
	if err != nil {
		result = multierror.Append(
			result, fmt.Errorf("error getting fileRevisions value: %w", err))
	} else {
		dbFileRevisions := make(map[string]string)
		for _, fr := range dbDoc.FileRevisions {
			dbFileRevisions[fr.GoogleDriveFileRevisionID] = fr.Name
		}
		if !reflect.DeepEqual(algoFileRevisions, dbFileRevisions) {
			result = multierror.Append(result,
				fmt.Errorf(
					"fileRevisions not equal, algolia=%v, db=%v",
					algoFileRevisions, dbFileRevisions),
			)
		}
	}

	// Compare modifiedTime.
	algoModifiedTime, err := getInt64Value(algoDoc, "modifiedTime")
	if err != nil {
		result = multierror.Append(
			result, fmt.Errorf("error getting modifiedTime value: %w", err))
	} else {
		dbModifiedTime := dbDoc.DocumentModifiedAt.Unix()
		if algoModifiedTime != dbModifiedTime {
			result = multierror.Append(result,
				fmt.Errorf(
					"modifiedTime not equal, algolia=%v, db=%v",
					algoModifiedTime, dbModifiedTime),
			)
		}
	}

	// Compare owner.
	// NOTE: this does not address multiple owners, which can exist for Algolia
	// document objects (documents in the database currently only have one owner).
	algoOwners, err := getStringSliceValue(algoDoc, "owners")
	if err != nil {
		result = multierror.Append(
			result, fmt.Errorf("error getting owners value: %w", err))
	} else {
		var algoOwner, dbOwner string
		if dbDoc.Owner != nil {
			dbOwner = dbDoc.Owner.EmailAddress
		}
		if len(algoOwners) > 0 {
			algoOwner = algoOwners[0]
		}
		if algoOwner != dbOwner {
			result = multierror.Append(result,
				fmt.Errorf(
					"owners not equal, algolia=%#v, db=%#v",
					algoOwner, dbOwner),
			)
		}
	}

	// Compare product.
	algoProduct, err := getStringValue(algoDoc, "product")
	if err != nil {
		result = multierror.Append(
			result, fmt.Errorf("error getting product value: %w", err))
	} else {
		dbProduct := dbDoc.Product.Name
		if algoProduct != dbProduct {
			result = multierror.Append(result,
				fmt.Errorf(
					"product not equal, algolia=%v, db=%v",
					algoProduct, dbProduct),
			)
		}
	}

	// Compare status.
	algoStatus, err := getStringValue(algoDoc, "status")
	if err != nil {
		result = multierror.Append(
			result, fmt.Errorf("error getting status value: %w", err))
	} else {
		var dbStatus string
		switch dbDoc.Status {
		case models.WIPDocumentStatus:
			dbStatus = "WIP"
		case models.InReviewDocumentStatus:
			dbStatus = "In-Review"
		case models.ApprovedDocumentStatus:
			dbStatus = "Approved"
		case models.ObsoleteDocumentStatus:
			dbStatus = "Obsolete"
		}

		// Standardize on "In-Review" Algolia status for the sake of comparison.
		if algoStatus == "In Review" {
			algoStatus = "In-Review"
		}

		if algoStatus != dbStatus {
			result = multierror.Append(result,
				fmt.Errorf(
					"status not equal, algolia=%v, db=%v",
					algoStatus, dbStatus),
			)
		}
	}

	// Compare summary.
	algoSummary, err := getStringValue(algoDoc, "summary")
	if err != nil {
		result = multierror.Append(
			result, fmt.Errorf("error getting summary value: %w", err))
	} else {
		dbSummary := dbDoc.Summary
		if dbSummary != nil && algoSummary != *dbSummary {
			result = multierror.Append(result,
				fmt.Errorf(
					"summary not equal, algolia=%v, db=%v",
					algoSummary, *dbSummary),
			)
		}
	}

	return result.ErrorOrNil()
}

func getBooleanValue(in map[string]any, key string) (bool, error) {
	var result bool

	if v, ok := in[key]; ok {
		if vv, ok := v.(bool); ok {
			return vv, nil
		} else {
			return false, fmt.Errorf(
				"invalid type: value is not a boolean, type: %T", v)
		}
	}

	return result, nil
}

func getInt64Value(in map[string]any, key string) (int64, error) {
	var result int64

	if v, ok := in[key]; ok {
		// These interface{} values are inferred as float64 and need to be converted
		// to int64.
		if vv, ok := v.(float64); ok {
			return int64(vv), nil
		} else {
			return 0, fmt.Errorf(
				"invalid type: value is not an float64 (expected), type: %T", v)
		}
	}

	return result, nil
}

func getMapStringStringValue(in map[string]any, key string) (
	map[string]string, error,
) {
	result := make(map[string]string)

	if v, ok := in[key]; ok {
		if reflect.TypeOf(v).Kind() == reflect.Map {
			mapVal, ok := v.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("invalid type: cannot convert to map[string]any")
			}
			for vk, vv := range mapVal {
				if vv, ok := vv.(string); ok {
					result[vk] = vv
				} else {
					return nil, fmt.Errorf(
						"invalid type: map value element is not a string")
				}
			}
			return result, nil
		} else {
			return nil, fmt.Errorf("invalid type: value is not a map")
		}
	}

	return result, nil
}

func getStringValue(in map[string]any, key string) (string, error) {
	var result string

	if v, ok := in[key]; ok {
		if vv, ok := v.(string); ok {
			return vv, nil
		} else {
			return "", fmt.Errorf("invalid type: value is not a string, type: %T", v)
		}
	}

	return result, nil
}

func getStringSliceValue(in map[string]any, key string) ([]string, error) {
	result := []string{}

	if v, ok := in[key]; ok && v != nil {
		if reflect.TypeOf(v).Kind() == reflect.Slice {
			sliceVal, ok := v.([]any)
			if !ok {
				return nil, fmt.Errorf("invalid type: cannot convert to []any")
			}
			for _, vv := range sliceVal {
				if vv, ok := vv.(string); ok {
					result = append(result, vv)
				} else {
					return nil, fmt.Errorf("invalid type: slice element is not a string")
				}
			}
			return result, nil
		} else {
			return nil, fmt.Errorf("invalid type: value is not a slice")
		}
	}

	return result, nil
}

// compareCustomFields compares custom fields between a source document (search/algolia) and database document.
// sourcePrefix is used for error messages (e.g., "search" or "algolia").
func compareCustomFields(
	docTypes []*config.DocumentType,
	sourceDoc map[string]any,
	sourceDocType string,
	dbDoc models.Document,
	sourcePrefix string,
) error {
	var result error
	foundDocType := false

	for _, dt := range docTypes {
		if dt.Name == sourceDocType {
			foundDocType = true
			for _, cf := range dt.CustomFields {
				sourceCFName := strcase.ToLowerCamel(cf.Name)

				switch cf.Type {
				case "string":
					sourceCFVal, err := getStringValue(sourceDoc, sourceCFName)
					if err != nil {
						result = multierror.Append(
							result, fmt.Errorf(
								"error getting custom field (%s) value: %w", sourceCFName, err))
					} else {
						var dbCFVal string
						for _, c := range dbDoc.CustomFields {
							if c.DocumentTypeCustomField.Name == cf.Name {
								dbCFVal = c.Value
								break
							}
						}
						if sourceCFVal != dbCFVal {
							result = multierror.Append(result,
								fmt.Errorf(
									"custom field %s not equal, %s=%v, db=%v",
									sourceCFName, sourcePrefix, sourceCFVal, dbCFVal),
							)
						}
					}
				case "people":
					sourceCFVal, err := getStringSliceValue(sourceDoc, sourceCFName)
					if err != nil {
						result = multierror.Append(
							result, fmt.Errorf(
								"error getting custom field (%s) value: %w", sourceCFName, err))
					} else {
						var dbCFVal []string
						for _, c := range dbDoc.CustomFields {
							if c.DocumentTypeCustomField.Name == cf.Name {
								// Unmarshal person custom field value to string slice.
								if err := json.Unmarshal(
									[]byte(c.Value), &dbCFVal,
								); err != nil {
									result = multierror.Append(result,
										fmt.Errorf(
											"error unmarshaling custom field %s to string slice",
											sourceCFName),
									)
								}
								break
							}
						}
						if !assert.ElementsMatch(fakeT{}, sourceCFVal, dbCFVal) {
							result = multierror.Append(result,
								fmt.Errorf(
									"custom field %s not equal, %s=%v, db=%v",
									sourceCFName, sourcePrefix, sourceCFVal, dbCFVal),
							)
						}
					}
				default:
					result = multierror.Append(result,
						fmt.Errorf(
							"unknown type for custom field key %q: %s", dt.Name, cf.Type))
				}
			}
			break
		}
	}

	if !foundDocType {
		result = multierror.Append(result,
			fmt.Errorf(
				"doc type %q not found", sourceDocType))
	}

	return result
}
