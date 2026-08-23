package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"reflect"
	"regexp"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/hashicorp-forge/hermes/internal/email"
	"github.com/hashicorp-forge/hermes/internal/helpers"
	"github.com/hashicorp-forge/hermes/internal/server"
	pkgauth "github.com/hashicorp-forge/hermes/pkg/auth"
	"github.com/hashicorp-forge/hermes/pkg/document"
	hcd "github.com/hashicorp-forge/hermes/pkg/hashicorpdocs"
	"github.com/hashicorp-forge/hermes/pkg/models"
)

// safeUintToInt converts uint to int, checking for overflow.
// Returns 0 if the value would overflow (though in practice database IDs
// should never be large enough to overflow).
func safeUintToInt(u uint) int {
	if u > math.MaxInt {
		return 0
	}
	return int(u)
}

// safeIntToUint converts int to uint, checking for negative values.
// Returns 0 if the value is negative.
func safeIntToUint(i int) uint {
	if i < 0 {
		return 0
	}
	return uint(i)
}

// DocumentPatchRequest contains a subset of documents fields that are allowed
// to be updated with a PATCH request.
type DocumentPatchRequest struct {
	Approvers      *[]string               `json:"approvers,omitempty"`
	ApproverGroups *[]string               `json:"approverGroups,omitempty"`
	Contributors   *[]string               `json:"contributors,omitempty"`
	CustomFields   *[]document.CustomField `json:"customFields,omitempty"`
	Owners         *[]string               `json:"owners,omitempty"`
	Status         *string                 `json:"status,omitempty"`
	Summary        *string                 `json:"summary,omitempty"`
	// Tags                []string `json:"tags,omitempty"`
	Title *string `json:"title,omitempty"`
}

type documentSubcollectionRequestType int

const (
	unspecifiedDocumentSubcollectionRequestType documentSubcollectionRequestType = iota
	noSubcollectionRequestType
	relatedResourcesDocumentSubcollectionRequestType
	shareableDocumentSubcollectionRequestType
	archivedDocumentSubcollectionRequestType
)

// DocumentHandler returns an HTTP handler for document operations.
//
//nolint:gocognit,gocyclo // Legacy HTTP entrypoint; large patch/get flows are kept together to avoid behavior drift.
func DocumentHandler(srv server.Server) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Bind to the site serving this request before touching the
		// database; srv as constructed holds the process-wide default.
		srv, ok := srv.ForRequest(w, r)
		if !ok {
			return
		}

		// Check if this is a document content request (/content suffix)
		// and delegate to DocumentContentHandler
		if strings.HasSuffix(r.URL.Path, "/content") {
			DocumentContentHandler(srv).ServeHTTP(w, r)
			return
		}

		// Parse document ID and request type from the URL path.
		docID, reqType, err := parseDocumentsURLPath(
			r.URL.Path, "documents")
		if err != nil {
			srv.Logger.Error("error parsing documents URL path",
				"error", err,
				"path", r.URL.Path,
				"method", r.Method,
			)
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		// Get document from database.
		// Support both GoogleFileID and UUID formats.
		// Try UUID first, fall back to GoogleFileID if not found or invalid UUID.
		model := srv.NewDocumentByFileID(docID)
		if err := model.GetByGoogleFileIDOrUUID(srv.DB, docID); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				srv.Logger.Warn("document record not found",
					"path", r.URL.Path,
					"method", r.Method,
					"doc_id", docID,
				)
				http.Error(w, "Document not found", http.StatusNotFound)
				return
			}

			srv.Logger.Error("error getting document from database",
				"error", err,
				"path", r.URL.Path,
				"method", r.Method,
				"doc_id", docID,
			)
			http.Error(w, "Error requesting document",
				http.StatusInternalServerError)
			return
		}

		// Get reviews for the document.
		// Use model.ID from the retrieved document to ensure we get the right reviews
		// regardless of how the document was looked up (UUID or GoogleFileID).
		var reviews models.DocumentReviews
		if err := reviews.Find(srv.DB, models.DocumentReview{
			Document: models.Document{
				Model: gorm.Model{
					ID: model.ID,
				},
			},
		}); err != nil {
			srv.Logger.Error("error getting reviews for document",
				"error", err,
				"method", r.Method,
				"path", r.URL.Path,
				"doc_id", docID,
			)
			http.Error(w, "Error processing request", http.StatusInternalServerError)
			return
		}

		// Get group reviews for the document.
		var groupReviews models.DocumentGroupReviews
		if err := groupReviews.Find(srv.DB, models.DocumentGroupReview{
			Document: models.Document{
				Model: gorm.Model{
					ID: model.ID,
				},
			},
		}); err != nil {
			srv.Logger.Error("error getting group reviews for document",
				"error", err,
				"method", r.Method,
				"path", r.URL.Path,
				"doc_id", docID,
			)
			return
		}

		// Convert database model to a document.
		doc, err := document.NewFromDatabaseModel(
			model, reviews, groupReviews)
		if err != nil {
			srv.Logger.Error("error converting database model to document type",
				"error", err,
				"method", r.Method,
				"path", r.URL.Path,
				"doc_id", docID,
			)
			http.Error(w, "Error processing request", http.StatusInternalServerError)
			return
		}
		// If the document was created through Hermes and has a status of "WIP", it
		// is a document draft and should be instead accessed through the drafts
		// API. We return a 404 to be consistent with v1 of the API, and will
		// improve this UX in the future when these APIs are combined.
		if doc.AppCreated && doc.Status == docStatusWIP {
			srv.Logger.Warn("attempted to access document draft via documents API",
				"method", r.Method,
				"path", r.URL.Path,
				"doc_id", docID,
			)
			http.Error(w, "Document not found", http.StatusNotFound)
			return
		}

		// Pass request off to associated subcollection (part of the URL after the
		// document ID) handler, if appropriate.
		switch reqType {
		case relatedResourcesDocumentSubcollectionRequestType:
			documentsResourceRelatedResourcesHandler(
				w, r, docID, *doc, srv.Config, srv.Logger, srv.SearchProvider, srv.DB, srv.IsSharePoint())
			return
		case shareableDocumentSubcollectionRequestType:
			srv.Logger.Warn("invalid shareable request for documents collection",
				"error", err,
				"path", r.URL.Path,
				"method", r.Method,
			)
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		switch r.Method {
		case httpMethodGet:
			now := time.Now()

			// Get document metadata from workspace provider so we can return the latest modified time.
			providerID := fmt.Sprintf("google:%s", docID)
			docMeta, err := srv.WorkspaceProvider.GetDocument(r.Context(), providerID)
			if err != nil {
				srv.Logger.Error("error getting document metadata from workspace",
					"error", err,
					"path", r.URL.Path,
					"method", r.Method,
					"doc_id", docID,
				)
				http.Error(w,
					"Error requesting document", http.StatusInternalServerError)
				return
			}

			// Use modified time from metadata
			modifiedTime := docMeta.ModifiedTime
			doc.ModifiedTime = modifiedTime.Unix()

			// Convert document to Algolia object because this is how it is expected
			// by the frontend.
			docObj, err := doc.ToAlgoliaObject(false)
			if err != nil {
				srv.Logger.Error("error converting document to Algolia object",
					"error", err,
					"method", r.Method,
					"path", r.URL.Path,
					"doc_id", docID,
				)
				http.Error(w, "Error processing request",
					http.StatusInternalServerError)
				return
			}

			// Set the directEditURL for direct link to the document
			directEditURL := ""
			if docMeta.ExtendedMetadata != nil {
				if webURL, ok := docMeta.ExtendedMetadata["web_url"].(string); ok {
					directEditURL = webURL
				}
				if webURL, ok := docMeta.ExtendedMetadata["web_view_link"].(string); ok {
					directEditURL = webURL
				}
			}
			docObj["directEditURL"] = directEditURL

			// Get projects associated with the document.
			projs, err := model.GetProjects(srv.DB)
			if err != nil {
				srv.Logger.Error("error getting projects associated with document",
					"error", err,
					"method", r.Method,
					"path", r.URL.Path,
					"doc_id", docID,
				)
				http.Error(w, "Error processing request",
					http.StatusInternalServerError)
				return
			}
			projIDs := make([]int, len(projs))
			for i := range projs {
				projIDs[i] = safeUintToInt(projs[i].ID)
			}
			docObj["projects"] = projIDs

			// Write response.
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)

			enc := json.NewEncoder(w)
			err = enc.Encode(docObj)
			if err != nil {
				srv.Logger.Error("error encoding document",
					"error", err,
					"doc_id", docID,
				)
				http.Error(w, "Error processing request",
					http.StatusInternalServerError)
				return
			}

			srv.Logger.Info("retrieved document",
				"doc_id", docID,
				"method", r.Method,
				"path", r.URL.Path,
				"status", doc.Status,
			)

			// Request post-processing.
			go func() {
				// Update recently viewed documents if this is a document view event. The
				// Add-To-Recently-Viewed header is set in the request from the frontend
				// to differentiate between document views and requests to only retrieve
				// document metadata.
				if r.Header.Get("Add-To-Recently-Viewed") != "" {
					// Get authenticated user's email address.
					userEmail := pkgauth.MustGetUserEmail(r.Context())

					if err := updateRecentlyViewedDocs(
						userEmail, docID, srv.DB, now,
					); err != nil {
						srv.Logger.Error("error updating recently viewed docs",
							"error", err,
							"doc_id", docID,
							"method", r.Method,
							"path", r.URL.Path,
						)
					}
				}
			}()

		case httpMethodPatch:
			// Decode request. The request struct validates that the request only
			// contains fields that are allowed to be patched.
			var req DocumentPatchRequest
			if err := decodeRequest(r, &req); err != nil {
				srv.Logger.Error("error decoding document patch request", "error", err)
				http.Error(w, fmt.Sprintf("Bad request: %q", err),
					http.StatusBadRequest)
				return
			}

			// Authorize request.
			userEmail := pkgauth.MustGetUserEmail(r.Context())
			if err := authorizeDocumentPatchRequest(
				userEmail, *doc, req,
			); err != nil {
				srv.Logger.Warn("error authorizing request",
					"error", err,
					"method", r.Method,
					"path", r.URL.Path,
					"doc_id", docID,
					"user", userEmail,
				)
				http.Error(w,
					fmt.Sprintf("Unauthorized: %v", err), http.StatusForbidden)
				return
			}

			// Check if document is locked (Google-only).
			if !srv.IsSharePoint() {
				locked, err := hcd.IsLocked(docID, srv.DB, srv.GWService, srv.Logger)
				if err != nil {
					srv.Logger.Error("error checking document locked status",
						"error", err,
						"path", r.URL.Path,
						"method", r.Method,
						"doc_id", docID,
					)
					http.Error(w, "Error getting document status", http.StatusNotFound)
					return
				}
				// Don't continue if document is locked.
				if locked {
					http.Error(w, "Document is locked", http.StatusLocked)
					return
				}
			}

			// Additional validation for contributor ownership acquisition
			if isContributorAcquiringOwnership(userEmail, *doc, req) {
				// Check if current owner is still active in the company
				currentOwner := doc.Owners[0]
				srv.Logger.Info("validating ownership acquisition: checking if current owner is alumni",
					"method", r.Method,
					"path", r.URL.Path,
					"doc_id", docID,
					"contributor", userEmail,
					"current_owner", currentOwner)

				// Search for the current owner in the people directory
				var ownerFound bool
				if srv.SharePoint != nil {
					people, err := srv.SharePoint.SearchPeople(currentOwner, 1)
					if err != nil {
						srv.Logger.Error("error searching for current owner in people directory",
							"error", err,
							"method", r.Method,
							"path", r.URL.Path,
							"doc_id", docID,
							"current_owner", currentOwner)
						http.Error(w, "Error validating ownership acquisition request",
							http.StatusInternalServerError)
						return
					}
					ownerFound = len(people) > 0
				} else {
					ppl, err := srv.GWService.SearchPeople(
						currentOwner, "emailAddresses,names")
					if err != nil {
						srv.Logger.Error("error searching for current owner in people directory",
							"error", err,
							"method", r.Method,
							"path", r.URL.Path,
							"doc_id", docID,
							"current_owner", currentOwner)
						http.Error(w, "Error validating ownership acquisition request",
							http.StatusInternalServerError)
						return
					}
					ownerFound = len(ppl) > 0
				}

				// If current owner is found in people directory, they are still active
				if ownerFound {
					srv.Logger.Warn("ownership acquisition denied: current owner is still active",
						"method", r.Method,
						"path", r.URL.Path,
						"doc_id", docID,
						"contributor", userEmail,
						"current_owner", currentOwner)
					http.Error(w,
						"Current owner is still active in company directory; Please contact them to transfer ownership",
						http.StatusForbidden)
					return
				}

				srv.Logger.Info("ownership acquisition authorized: current owner not found in company directory",
					"method", r.Method,
					"path", r.URL.Path,
					"doc_id", docID,
					"contributor", userEmail,
					"current_owner", currentOwner)
			}

			// Validate owners.
			if req.Owners != nil {
				if len(*req.Owners) != 1 {
					srv.Logger.Warn("invalid number of owners in patch request",
						"method", r.Method,
						"path", r.URL.Path,
						"doc_id", docID)
					http.Error(w,
						"Bad request: invalid number of owners (only 1 allowed)",
						http.StatusBadRequest)
					return
				}
			}

			// Validate custom fields.
			if req.CustomFields != nil {
				if err := validateEditableCustomFields(*req.CustomFields, *doc); err != nil {
					logCustomFieldValidationError(srv.Logger, err, r, docID)
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
			}

			// Validate document Status.
			if req.Status != nil {
				switch *req.Status {
				case docStatusApproved:
				case docStatusInReview:
				case docStatusObsolete:
				default:
					srv.Logger.Warn("invalid status",
						"method", r.Method,
						"path", r.URL.Path,
						"doc_id", docID)
					http.Error(w, "Bad request: invalid status", http.StatusBadRequest)
					return
				}
			}

			// Check if document is locked (Google Docs specific).
			googleProvider := getGoogleDocsProvider(srv.WorkspaceProvider)
			if googleProvider != nil {
				locked, err := hcd.IsLocked(docID, srv.DB, googleProvider, srv.Logger)
				if err != nil {
					srv.Logger.Error("error checking document locked status",
						"error", err,
						"path", r.URL.Path,
						"method", r.Method,
						"doc_id", docID,
					)
					http.Error(w, "Error getting document status", http.StatusNotFound)
					return
				}
				// Don't continue if document is locked.
				if locked {
					http.Error(w, "Document is locked", http.StatusLocked)
					return
				}
			}

			// Determine removed individual approvers and group approvers (to revoke access).
			var removedUserApprovers []string
			var removedGroupApprovers []string
			if req.Approvers != nil {
				if len(doc.Approvers) != 0 {
					if len(*req.Approvers) == 0 {
						// All approvers are being removed
						removedUserApprovers = doc.Approvers
					} else {
						// Compare approvers when there are stored approvers
						// and there are approvers in the request
						// Find approvers that exist in current doc but NOT in the request
						removedUserApprovers = compareSlices(
							*req.Approvers, doc.Approvers)
					}
				}
			}
			if req.ApproverGroups != nil {
				if len(doc.ApproverGroups) != 0 {
					if len(*req.ApproverGroups) == 0 {
						// All approver groups are being removed
						removedGroupApprovers = doc.ApproverGroups
					} else {
						// Find approver groups that exist in current doc but NOT in the request
						removedGroupApprovers = compareSlices(
							*req.ApproverGroups, doc.ApproverGroups)
					}
				}
			}

			// Patch document (for Algolia).
			// Approvers.
			if req.Approvers != nil {
				doc.Approvers = *req.Approvers
			}
			// Approver groups.
			if req.ApproverGroups != nil {
				doc.ApproverGroups = *req.ApproverGroups
			}
			// Contributors.
			var contributorsToRemoveSharing []string
			if req.Contributors != nil {
				// Find out contributors to remove from sharing the document
				if len(doc.Contributors) != 0 {
					if len(*req.Contributors) == 0 {
						// All contributors are being removed
						contributorsToRemoveSharing = doc.Contributors
					} else {
						// Compare contributors when there are stored contributors
						// and there are contributors in the request
						// Find contributors that exist in current doc but NOT in the request
						contributorsToRemoveSharing = compareSlices(
							*req.Contributors, doc.Contributors)
					}
				}

				doc.Contributors = *req.Contributors
			}
			// Custom fields.
			if req.CustomFields != nil {
				for _, cf := range *req.CustomFields {
					switch cf.Type {
					case fieldTypeString:
						if _, ok := cf.Value.(string); ok {
							if err := doc.UpsertCustomField(cf); err != nil {
								srv.Logger.Error("error upserting custom string field",
									"error", err,
									"method", r.Method,
									"path", r.URL.Path,
									"custom_field", cf.Name,
									"doc_id", docID,
								)
								http.Error(w,
									"Error patching document",
									http.StatusInternalServerError)
								return
							}
						}
					case fieldTypePeople:
						if reflect.TypeOf(cf.Value).Kind() != reflect.Slice {
							srv.Logger.Error("invalid value type for people custom field",
								"error", err,
								"method", r.Method,
								"path", r.URL.Path,
								"custom_field", cf.Name,
								"doc_id", docID)
							http.Error(w,
								fmt.Sprintf(
									"Bad request: invalid value type for custom field %q",
									cf.Name,
								),
								http.StatusBadRequest)
							return
						}
						values, ok := cf.Value.([]any)
						if !ok {
							srv.Logger.Error("invalid value type for people custom field",
								"error", err,
								"method", r.Method,
								"path", r.URL.Path,
								"custom_field", cf.Name,
								"doc_id", docID)
							http.Error(w,
								fmt.Sprintf(
									"Bad request: invalid value type for custom field %q",
									cf.Name,
								),
								http.StatusBadRequest)
							return
						}
						for _, v := range values {
							if _, ok := v.(string); !ok {
								srv.Logger.Error("invalid value type for people custom field",
									"error", err,
									"method", r.Method,
									"path", r.URL.Path,
									"custom_field", cf.Name,
									"doc_id", docID)
								http.Error(w,
									fmt.Sprintf(
										"Bad request: invalid value type for custom field %q",
										cf.Name,
									),
									http.StatusBadRequest)
								return
							}
						}
						if err := doc.UpsertCustomField(cf); err != nil {
							srv.Logger.Error("error upserting custom people field",
								"error", err,
								"method", r.Method,
								"path", r.URL.Path,
								"custom_field", cf.Name,
								"doc_id", docID,
							)
							http.Error(w,
								"Error patching document",
								http.StatusInternalServerError)
							return
						}
					default:
						srv.Logger.Error("invalid custom field type",
							"error", err,
							"method", r.Method,
							"path", r.URL.Path,
							"custom_field", cf.Name,
							"custom_field_type", cf.Type,
							"doc_id", docID)
						http.Error(w,
							fmt.Sprintf(
								"Bad request: invalid type for custom field %q",
								cf.Name,
							),
							http.StatusBadRequest)
						return
					}
				}
			}
			// Owner.
			if req.Owners != nil {
				// Give new owner edit access to the document.
				providerID := fmt.Sprintf("google:%s", docID)
				if err := srv.WorkspaceProvider.ShareDocument(
					r.Context(), providerID, doc.Owners[0], "writer"); err != nil {
					srv.Logger.Error("error sharing file with new owner",
						"error", err,
						"method", r.Method,
						"path", r.URL.Path,
						"doc_id", docID,
						"contributor", userEmail,
						"previous_owner", doc.Owners[0])
				}

				doc.Owners = *req.Owners

				// Give new owner edit access to the document.
				if srv.SharePoint != nil {
					if err := srv.SharePoint.ShareFile(
						docID, doc.Owners[0], "writer"); err != nil {
						srv.Logger.Error("error sharing file with new owner",
							"error", err,
							"method", r.Method,
							"path", r.URL.Path,
							"doc_id", docID,
							"new_owner", doc.Owners[0])
						http.Error(w, "Error patching document",
							http.StatusInternalServerError)
						return
					}
				} else {
					if err := srv.GWService.ShareFile(
						docID, doc.Owners[0], "writer"); err != nil {
						srv.Logger.Error("error sharing file with new owner",
							"error", err,
							"method", r.Method,
							"path", r.URL.Path,
							"doc_id", docID,
							"new_owner", doc.Owners[0])
						http.Error(w, "Error patching document",
							http.StatusInternalServerError)
						return
					}
				}
			}
			// Status.
			if req.Status != nil {
				doc.Status = *req.Status
			}
			// Summary.
			if req.Summary != nil {
				doc.Summary = *req.Summary
			}
			// Title.
			if req.Title != nil {
				doc.Title = *req.Title

				// Rename the file to match the new title.
				abbr := strings.SplitN(doc.DocNumber, "-", 2)[0]
				newFileName := fmt.Sprintf("%s-%s", abbr, *req.Title)

				if srv.SharePoint != nil {
					// Sanitize the file name for SharePoint.
					newFileName = strings.NewReplacer(
						"[", "(", "]", ")", "#", "-", "%", "-", "&", "and",
						"*", "-", ":", "-", "<", "-", ">", "-", "?", "",
						"/", "-", "\\", "-", "{", "(", "|", "-", "}", ")",
						"~", "-",
					).Replace(newFileName)
					newFileName = fmt.Sprintf("%s.docx", newFileName)

					if err := srv.SharePoint.RenameFile(docID, newFileName); err != nil {
						srv.Logger.Error("error renaming file",
							"error", err,
							"method", r.Method,
							"path", r.URL.Path,
							"doc_id", docID,
							"new_file_name", newFileName)
						srv.Logger.Warn("continuing document patch despite file rename failure")
					} else {
						srv.Logger.Info("successfully renamed file",
							"method", r.Method,
							"path", r.URL.Path,
							"doc_id", docID,
							"new_file_name", newFileName)
					}
				} else {
					if err := srv.GWService.RenameFile(docID, newFileName); err != nil {
						srv.Logger.Error("error renaming file",
							"error", err,
							"method", r.Method,
							"path", r.URL.Path,
							"doc_id", docID,
							"new_file_name", newFileName)
						srv.Logger.Warn("continuing document patch despite file rename failure")
					}
				}
			}

			approversToEmail := []string{}
			if req.Approvers != nil {
				approversToEmail = compareSlices(doc.Approvers, *req.Approvers)
			}

			// Give new document approvers edit access to the document.
			providerID := fmt.Sprintf("google:%s", docID)
			for _, a := range approversToEmail {
				if err := srv.WorkspaceProvider.ShareDocument(r.Context(), providerID, a, "writer"); err != nil {
					srv.Logger.Error("error sharing document with approver",
						"error", err,
						"doc_id", docID,
						"method", r.Method,
						"path", r.URL.Path,
						"approvers_count", len(removedUserApprovers),
					)
				}

				// Remove group approvers.
				for _, g := range removedGroupApprovers {
					// Only remove group if it doesn't match owner email(s).
					if !contains(doc.Owners, g) {
						if err := removeSharing(getCompatProvider(srv.WorkspaceProvider), docID, g); err != nil {
							srv.Logger.Error("error removing approver group from file",
								"error", err,
								"method", r.Method,
								"path", r.URL.Path,
								"doc_id", docID,
								"approver_group", g)
							http.Error(w, "Error patching document",
								http.StatusInternalServerError)
							return
						}
					}
				}
				if len(removedGroupApprovers) > 0 {
					srv.Logger.Info("removed approver groups from document",
						"method", r.Method,
						"path", r.URL.Path,
						"approver_groups_count", len(removedGroupApprovers),
					)
				}

				// Remove contributors.
				for _, c := range contributorsToRemoveSharing {
					// Only remove contributor if the email
					// associated with the permission doesn't
					// match owner email(s).
					if !contains(doc.Owners, c) {
						if err := removeSharing(getCompatProvider(srv.WorkspaceProvider), docID, c); err != nil {
							srv.Logger.Error("error removing contributor from file",
								"error", err,
								"method", r.Method,
								"path", r.URL.Path,
								"doc_id", docID,
								"contributor", c)
							http.Error(w, "Error patching document",
								http.StatusInternalServerError)
							return
						}
					}
				}
				if len(contributorsToRemoveSharing) > 0 {
					srv.Logger.Info("removed contributors from document",
						"method", r.Method,
						"path", r.URL.Path,
						"contributors_count", len(contributorsToRemoveSharing),
					)
				}
			}

			// Replace the doc header (Google-only; SharePoint headers
			// are managed by the Hermes Add-In for Word).
			if !srv.IsSharePoint() {
				if err := doc.ReplaceHeader(
					srv.Config.BaseURL, false, getCompatProvider(srv.WorkspaceProvider),
				); err != nil {
					srv.Logger.Error("error replacing document header",
						"error", err, "doc_id", docID)
					http.Error(w, "Error patching document",
						http.StatusInternalServerError)
					return
				}
			}

			// Replace the doc header (Google Docs specific).
			googleUpdater := getGoogleDocsUpdater(srv.WorkspaceProvider)
			if googleUpdater == nil {
				srv.Logger.Warn("ReplaceHeader skipped - not using Google Workspace", "doc_id", docID)
			} else if err := doc.ReplaceHeader(
				srv.Config.BaseURL, false, googleUpdater,
			); err != nil {
				srv.Logger.Error("error replacing document header",
					"error", err, "doc_id", docID)
				http.Error(w, "Error patching document",
					http.StatusInternalServerError)
				return
			}

			// Rename document with new title (Google Docs specific).
			if googleUpdater != nil {
				providerID := fmt.Sprintf("google:%s", docID)
				if err := srv.WorkspaceProvider.RenameDocument(r.Context(), providerID,
					fmt.Sprintf("[%s] %s", doc.DocNumber, doc.Title)); err != nil {
					srv.Logger.Warn("failed to rename document in workspace provider",
						"error", err,
						"doc_id", docID)
				}
			}

			// Get document record from database so we can modify it for updating.
			model := srv.NewDocumentByFileID(docID)
			if err := model.Get(srv.DB); err != nil {
				srv.Logger.Error("error getting document from database",
					"error", err,
					"method", r.Method,
					"path", r.URL.Path,
					"doc_id", docID,
				)
				http.Error(w, "Error patching document",
					http.StatusInternalServerError)
				return
			}
			// Approvers.
			if req.Approvers != nil {
				model.Approvers = usersFromEmails(doc.Approvers)
			}

			// Approver groups.
			if req.ApproverGroups != nil {
				model.ApproverGroups = groupsFromEmails(doc.ApproverGroups)
			}

			// Contributors.
			if req.Contributors != nil {
				model.Contributors = usersFromEmails(doc.Contributors)
			}

			// Custom fields.
			if req.CustomFields != nil {
				updatedFields, err := updateModelCustomFields(
					model.CustomFields,
					doc.DocType,
					*req.CustomFields,
				)
				if err != nil {
					logCustomFieldValidationError(srv.Logger, err, r, docID)
					http.Error(w, "Error patching document",
						http.StatusInternalServerError)
					return
				}
				model.CustomFields = updatedFields
			}
			setModelCustomFieldDocumentIDs(model.CustomFields, model.ID)

			// Document modified time.
			model.DocumentModifiedAt = time.Unix(doc.ModifiedTime, 0)

			// Owner.
			if req.Owners != nil {
				model.Owner = &models.User{
					EmailAddress: doc.Owners[0],
				}
			}

			// Status.
			if req.Status != nil {
				switch *req.Status {
				case docStatusApproved:
					model.Status = models.ApprovedDocumentStatus
				case docStatusInReview:
					model.Status = models.InReviewDocumentStatus
				case docStatusObsolete:
					model.Status = models.ObsoleteDocumentStatus
				}
			}

			// Summary.
			if req.Summary != nil {
				model.Summary = req.Summary
			}

			// Title.
			if req.Title != nil {
				model.Title = *req.Title
			}

			// Send email to new owner.
			if srv.Config.Email != nil && srv.Config.Email.Enabled &&
				req.Owners != nil {
				if err := sendNewOwnerNotification(r, srv, docID, userEmail, *doc); err != nil {
					srv.Logger.Error("error notifying new owner",
						"error", err,
						"method", r.Method,
						"path", r.URL.Path,
						"doc_id", docID,
					)
					http.Error(w, "Error patching document",
						http.StatusInternalServerError)
					return
				}
			}

			// Send emails to new approvers.
			if srv.Config.Email != nil && srv.Config.Email.Enabled {
				if len(approversToEmail) > 0 {
					// Get document URL.
					docURL, err := getDocumentURL(srv.Config.BaseURL, docID)
					if err != nil {
						srv.Logger.Error("error getting document URL",
							"error", err,
							"doc_id", docID,
							"method", r.Method,
							"path", r.URL.Path,
						)
						// Log error but don't fail the request.
					} else {
						// TODO: use an asynchronous method for sending emails because we
						// can't currently recover gracefully on a failure here.
						for _, approverEmail := range approversToEmail {
							err := email.SendReviewRequestedEmail(
								email.ReviewRequestedEmailData{
									BaseURL:           srv.Config.BaseURL,
									DocumentOwner:     doc.Owners[0],
									DocumentShortName: doc.DocNumber,
									DocumentTitle:     doc.Title,
									DocumentURL:       docURL,
									Product:           doc.Product,
									DocumentType:      doc.DocType,
									DocumentStatus:    doc.Status,
								},
								[]string{approverEmail},
								srv.Config.Email.FromAddress,
								getEmailSender(srv.WorkspaceProvider),
							)
							if err != nil {
								srv.Logger.Error("error sending approver email",
									"error", err,
									"doc_id", docID,
									"method", r.Method,
									"path", r.URL.Path,
								)
								http.Error(w, "Error patching document",
									http.StatusInternalServerError)
								return
							}
						}
						srv.Logger.Info("approver emails sent",
							"doc_id", docID,
							"method", r.Method,
							"path", r.URL.Path,
						)
					}
				}
			}

			// Update document and enqueue search projection atomically.
			if err := upsertDocumentWithSearchOutbox(srv.DB, &model, doc); err != nil {
				srv.Logger.Error("error updating document",
					"error", err,
					"method", r.Method,
					"path", r.URL.Path,
					"doc_id", docID,
				)
				http.Error(w, "Error patching document",
					http.StatusInternalServerError)
				return
			}

			w.WriteHeader(http.StatusOK)
			srv.Logger.Info("patched document",
				"doc_id", docID,
				"method", r.Method,
				"path", r.URL.Path,
			)

		default:
			WriteMethodNotAllowed(w, r, httpMethodGet, httpMethodPatch)
			return
		}
	})
}

func upsertDocumentWithSearchOutbox(db *gorm.DB, model *models.Document, doc *document.Document) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := model.Upsert(tx); err != nil {
			return err
		}

		payload, err := documentSearchPayload(doc)
		if err != nil {
			return err
		}

		return models.EnqueueSearchOutboxEventWithSequence(tx, &models.SearchOutboxEvent{
			EventType:     models.SearchEventDocumentUpdated,
			AggregateID:   doc.ObjectID,
			AggregateType: models.SearchAggregateDocument,
			IndexName:     models.SearchIndexDocuments,
			Operation:     models.SearchOutboxOperationUpsert,
			Payload:       payload,
		})
	})
}

func documentSearchPayload(doc *document.Document) (map[string]any, error) {
	payload, err := doc.ToAlgoliaObject(true)
	if err != nil {
		return nil, fmt.Errorf("error converting document to search object: %w", err)
	}
	return payload, nil
}

// updateRecentlyViewedDocs updates the recently viewed docs for a user with the
// provided email address, using the document file ID and viewed at time for a
// document view event.
func updateRecentlyViewedDocs(
	userAddr, docID string, db *gorm.DB, viewedAt time.Time) error {
	// Get user (if exists).
	u := models.User{
		EmailAddress: userAddr,
	}
	if err := u.Get(db); err != nil && !errors.Is(
		err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("error getting user in database: %w", err)
	}

	// Get viewed document in database.
	doc := models.Document{GoogleFileID: docID}
	if err := doc.Get(db); err != nil {
		return fmt.Errorf("error getting viewed document: %w", err)
	}

	// Find recently viewed documents (excluding the current viewed document).
	var rvd []models.RecentlyViewedDoc
	if err := db.Where(&models.RecentlyViewedDoc{UserID: safeUintToInt(u.ID)}).
		Not("document_id = ?", doc.ID).
		Limit(9).
		Order("viewed_at desc").
		Find(&rvd).Error; err != nil {
		return fmt.Errorf("error finding recently viewed docs for user: %w", err)
	}

	// Prepend viewed document to recently viewed documents.
	rvd = append(
		[]models.RecentlyViewedDoc{{
			DocumentID: safeUintToInt(doc.ID),
			UserID:     safeUintToInt(u.ID),
		}},
		rvd...)

	// Make slice of recently viewed document IDs.
	docIDs := make([]int, len(rvd))
	for i, d := range rvd {
		docIDs[i] = d.DocumentID
	}

	// Get document records for recently viewed documents.
	var docs []models.Document
	if err := db.Where("id IN ?", docIDs).Find(&docs).Error; err != nil {
		return fmt.Errorf("error getting documents: %w", err)
	}

	// Update user.
	u.RecentlyViewedDocs = docs
	if err := u.Upsert(db); err != nil {
		return fmt.Errorf("error upserting user: %w", err)
	}

	// Update ViewedAt time for the viewed document.
	viewedDoc := models.RecentlyViewedDoc{
		UserID:     safeUintToInt(u.ID),
		DocumentID: safeUintToInt(doc.ID),
		ViewedAt:   viewedAt,
	}
	if err := db.Updates(&viewedDoc).Error; err != nil {
		return fmt.Errorf(
			"error updating recently viewed document in database: %w", err)
	}

	return nil
}

// parseDocumentsURLPath parses the document ID and subcollection request type
// from a documents/drafts API URL path.
func parseDocumentsURLPath(path, collection string) (
	docID string,
	reqType documentSubcollectionRequestType,
	err error,
) {
	// Pattern accepts both GoogleFileID format ([0-9A-Za-z_\-]+)
	// and UUID format with optional uuid/ prefix (uuid/)?[0-9a-f-]+ or bare UUIDs
	// Examples:
	// - GoogleFileID: "1abc2def3ghi4jkl5mno6pqr"
	// - UUID with prefix: "uuid/550e8400-e29b-41d4-a716-446655440000"
	// - Bare UUID: "550e8400-e29b-41d4-a716-446655440000"
	noSubcollectionRE := regexp.MustCompile(
		fmt.Sprintf(
			`^\/api\/v2\/%s\/((?:uuid\/)?[0-9A-Za-z_\-]+)$`,
			collection))
	relatedResourcesSubcollectionRE := regexp.MustCompile(
		fmt.Sprintf(
			`^\/api\/v2\/%s\/((?:uuid\/)?[0-9A-Za-z_\-]+)\/related-resources$`,
			collection))
	// shareable isn't really a subcollection, but we'll go with it.
	shareableRE := regexp.MustCompile(
		fmt.Sprintf(
			`^\/api\/v2\/%s\/((?:uuid\/)?[0-9A-Za-z_\-]+)\/shareable$`,
			collection))
	// archived isn't really a subcollection, but we'll go with it.
	archivedRE := regexp.MustCompile(
		fmt.Sprintf(
			`^\/api\/v2\/%s\/([0-9A-Za-z_\-]+)\/archived$`,
			collection))

	switch {
	case noSubcollectionRE.MatchString(path):
		matches := noSubcollectionRE.FindStringSubmatch(path)
		if len(matches) != 2 {
			return "", unspecifiedDocumentSubcollectionRequestType, fmt.Errorf(
				"wrong number of string submatches for resource URL path")
		}
		return matches[1], noSubcollectionRequestType, nil

	case relatedResourcesSubcollectionRE.MatchString(path):
		matches := relatedResourcesSubcollectionRE.
			FindStringSubmatch(path)
		if len(matches) != 2 {
			return "",
				relatedResourcesDocumentSubcollectionRequestType,
				fmt.Errorf(
					"wrong number of string submatches for related resources subcollection URL path")
		}
		return matches[1], relatedResourcesDocumentSubcollectionRequestType, nil

	case shareableRE.MatchString(path):
		matches := shareableRE.
			FindStringSubmatch(path)
		if len(matches) != 2 {
			return "",
				shareableDocumentSubcollectionRequestType,
				fmt.Errorf(
					"wrong number of string submatches for shareable subcollection URL path")
		}
		return matches[1], shareableDocumentSubcollectionRequestType, nil

	case archivedRE.MatchString(path):
		matches := archivedRE.
			FindStringSubmatch(path)
		if len(matches) != 2 {
			return "",
				archivedDocumentSubcollectionRequestType,
				fmt.Errorf(
					"wrong number of string submatches for archived subcollection URL path")
		}
		return matches[1], archivedDocumentSubcollectionRequestType, nil

	default:
		return "",
			unspecifiedDocumentSubcollectionRequestType,
			fmt.Errorf("path did not match any URL strings")
	}
}

func isContributorAcquiringOwnership(
	userEmail string,
	doc document.Document,
	req DocumentPatchRequest,
) bool {
	if req.Owners == nil || len(*req.Owners) != 1 {
		return false
	}

	if strings.EqualFold(doc.Owners[0], userEmail) {
		return false
	}

	if !helpers.StringSliceContainsFold(doc.Contributors, userEmail) {
		return false
	}

	return strings.EqualFold((*req.Owners)[0], userEmail)
}

// authorizeDocumentPatchRequest authorizes a PATCH request to a document.
//   - Document owners can patch any field.
//   - Approvers can only patch the Approvers field to remove themselves.
//   - Contributors can only patch the Owners field to acquire ownership (setting themselves as the sole owner).
//     Additional validation in the main handler ensures the current owner is no longer with the company.
func authorizeDocumentPatchRequest(
	userEmail string,
	doc document.Document,
	req DocumentPatchRequest,
) error {
	// The document owner can patch any field.
	if strings.EqualFold(doc.Owners[0], userEmail) {
		return nil
	}

	// Approvers can only patch the Approvers field to remove themselves as an
	// approver.
	if helpers.StringSliceContains(doc.Approvers, userEmail) {
		// Request should only have one non-nil field, Approvers.
		numNonNilFields := 0
		reqValue := reflect.ValueOf(req)
		for i := 0; i < reqValue.NumField(); i++ {
			fieldValue := reqValue.Field(i)
			if fieldValue.Kind() == reflect.Ptr && !fieldValue.IsNil() {
				numNonNilFields++
			}
		}
		if numNonNilFields != 1 || req.Approvers == nil {
			return errors.New(
				"approvers can only patch the approvers field to remove themselves as an approver")
		}

		// Remove duplicates from request and document approvers to be safe.
		reqApprovers := helpers.RemoveStringSliceDuplicates(*req.Approvers)
		docApprovers := helpers.RemoveStringSliceDuplicates(doc.Approvers)

		// Request approvers should be one less than document approvers.
		if len(reqApprovers) != len(docApprovers)-1 {
			return errors.New(
				"approvers can only patch a document to remove themselves as an approver")
		}

		// Request approvers should be a subset of document approvers and not
		// contain the requesting user.
		for _, ra := range reqApprovers {
			if strings.EqualFold(ra, userEmail) || !helpers.StringSliceContains(docApprovers, ra) {
				return errors.New(
					"approvers can only patch a document to remove themselves as an approver")
			}
		}

		return nil
	}

	// Contributors can only patch the Owners field to acquire ownership of the document.
	if helpers.StringSliceContainsFold(doc.Contributors, userEmail) {
		// Request should only have one non-nil field, Owners.
		numNonNilFields := 0
		reqValue := reflect.ValueOf(req)
		for i := 0; i < reqValue.NumField(); i++ {
			fieldValue := reqValue.Field(i)
			if fieldValue.Kind() == reflect.Ptr && !fieldValue.IsNil() {
				numNonNilFields++
			}
		}
		if numNonNilFields != 1 || req.Owners == nil {
			return errors.New(
				"contributors can only patch the owners field to acquire ownership")
		}

		// The Owners field should contain exactly one email (the contributor's email).
		if len(*req.Owners) != 1 {
			return errors.New(
				"contributors can only acquire ownership by setting themselves as the sole owner")
		}

		// The email in the Owners field must match the requesting user's email.
		if !strings.EqualFold((*req.Owners)[0], userEmail) {
			return errors.New(
				"contributors can only acquire ownership by setting themselves as the owner")
		}

		return nil
	}

	return errors.New("only owners, approvers, or contributors can patch a document")
}
