package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/hashicorp-forge/hermes/internal/config"
	"github.com/hashicorp-forge/hermes/internal/email"
	"github.com/hashicorp-forge/hermes/internal/server"
	pkgauth "github.com/hashicorp-forge/hermes/pkg/auth"
	"github.com/hashicorp-forge/hermes/pkg/document"
	hcd "github.com/hashicorp-forge/hermes/pkg/hashicorpdocs"
	"github.com/hashicorp-forge/hermes/pkg/models"
	"github.com/hashicorp-forge/hermes/pkg/search"
	"github.com/hashicorp-forge/hermes/pkg/workspace"
)

type DraftsRequest struct {
	Contributors        []string `json:"contributors,omitempty"`
	DocType             string   `json:"docType,omitempty"`
	Product             string   `json:"product,omitempty"`
	ProductAbbreviation string   `json:"productAbbreviation,omitempty"`
	Summary             string   `json:"summary,omitempty"`
	Tags                []string `json:"tags,omitempty"`
	Title               string   `json:"title"`
}

// DraftsPatchRequest contains a subset of drafts fields that are allowed to
// be updated with a PATCH request.
type DraftsPatchRequest struct {
	Approvers      *[]string               `json:"approvers,omitempty"`
	ApproverGroups *[]string               `json:"approverGroups,omitempty"`
	Contributors   *[]string               `json:"contributors,omitempty"`
	CustomFields   *[]document.CustomField `json:"customFields,omitempty"`
	Owners         *[]string               `json:"owners,omitempty"`
	Product        *string                 `json:"product,omitempty"`
	Summary        *string                 `json:"summary,omitempty"`
	// Tags                []string `json:"tags,omitempty"`
	Title *string `json:"title,omitempty"`
}

type DraftsResponse struct {
	ID string `json:"id"`
}

func DraftsHandler(srv server.Server) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		errResp := func(httpCode int, userErrMsg, logErrMsg string, err error) {
			srv.Logger.Error(logErrMsg,
				"method", r.Method,
				"path", r.URL.Path,
				"error", err,
			)
			http.Error(w, userErrMsg, httpCode)
		}

		// Authorize request.
		userEmail := pkgauth.MustGetUserEmail(r.Context())
		if userEmail == "" {
			errResp(
				http.StatusUnauthorized,
				"No authorization information for request",
				"no user email found in request context",
				nil,
			)
			return
		}

		switch r.Method {
		case "POST":
			// Decode request.
			var req DraftsRequest
			if err := decodeRequest(r, &req); err != nil {
				srv.Logger.Error("error decoding drafts request", "error", err)
				http.Error(w, fmt.Sprintf("Bad request: %q", err),
					http.StatusBadRequest)
				return
			}

			// Validate document type.
			if !validateDocType(srv.Config.DocumentTypes.DocumentType, req.DocType) {
				srv.Logger.Error("invalid document type",
					"method", r.Method,
					"path", r.URL.Path,
					"doc_type", req.DocType,
				)
				http.Error(
					w, "Bad request: invalid document type", http.StatusBadRequest)
				return
			}

			if req.Title == "" {
				http.Error(w, "Bad request: title is required", http.StatusBadRequest)
				return
			}

			// Get doc type template.
			template := getDocTypeTemplate(
				srv.Config.DocumentTypes.DocumentType, req.DocType)
			if template == "" {
				srv.Logger.Error("Bad request: no template configured for doc type",
					"method", r.Method,
					"path", r.URL.Path,
					"doc_type", req.DocType,
				)
				http.Error(w,
					"Bad request: no template configured for doc type",
					http.StatusBadRequest)
				return
			}

			// Build title.
			if req.ProductAbbreviation == "" {
				req.ProductAbbreviation = "TODO"
			}
			title := fmt.Sprintf("[%s-???] %s", req.ProductAbbreviation, req.Title)

			var (
				err     error
				docMeta *workspace.DocumentMetadata
			)

			// Copy template to new draft file using RFC-084.
			// Use the appropriate provider prefix based on workspace configuration
			workspaceProvider := "google" // default for backwards compatibility
			if srv.Config.Providers != nil && srv.Config.Providers.Workspace != "" {
				workspaceProvider = srv.Config.Providers.Workspace
			}
			templateProviderID := fmt.Sprintf("%s:%s", workspaceProvider, template)

			// Get the appropriate destination folder based on provider
			destFolderID := srv.Config.GoogleWorkspace.DraftsFolder
			if workspaceProvider == "local" && srv.Config.LocalWorkspace != nil {
				destFolderID = srv.Config.LocalWorkspace.DraftsPath
			}

			// Use RFC-084 CopyDocument (RFC-084 doesn't support user impersonation directly)
			docMeta, err = srv.WorkspaceProvider.CopyDocument(
				r.Context(), templateProviderID, destFolderID, title)
			if err != nil {
				srv.Logger.Error("error creating draft",
					"error", err,
					"method", r.Method,
					"path", r.URL.Path,
					"template", template,
					"drafts_folder", srv.Config.GoogleWorkspace.DraftsFolder,
				)
				http.Error(w, "Error creating document draft",
					http.StatusInternalServerError)
				return
			}

			// Extract file ID from provider ID (format: "provider:fileID")
			// Strip any provider prefix (google:, local:, etc.)
			fileID := docMeta.ProviderID
			if idx := strings.Index(fileID, ":"); idx != -1 {
				fileID = fileID[idx+1:]
			}

			// Build created date.
			ct := docMeta.CreatedTime
			cd := ct.Format("Jan 2, 2006")

			// Get owner photo by searching Google Workspace directory.
			op := []string{}
			people, err := srv.WorkspaceProvider.SearchPeople(r.Context(), userEmail)
			if err != nil {
				srv.Logger.Error(
					"error searching directory for person",
					"error", err,
					"method", r.Method,
					"path", r.URL.Path,
					"person", userEmail,
				)
			}
			if len(people) > 0 && people[0].PhotoURL != "" {
				op = append(op, people[0].PhotoURL)
			}

			// Create tag
			// Note: The o_id tag may be empty for environments such as development.
			// For environments like pre-prod and prod, it will be set as
			// Okta authentication is enforced before this handler is called for
			// those environments. Maybe, if id isn't set we use
			// owner emails in the future?
			id := r.Header.Get("x-amzn-oidc-identity")
			metaTags := []string{
				"o_id:" + id,
			}

			// Build document.
			doc := &document.Document{
				ObjectID:     fileID,
				Title:        req.Title,
				AppCreated:   true,
				Contributors: req.Contributors,
				Created:      cd,
				CreatedTime:  ct.Unix(),
				DocNumber:    fmt.Sprintf("%s-???", req.ProductAbbreviation),
				DocType:      req.DocType,
				MetaTags:     metaTags,
				ModifiedTime: ct.Unix(),
				Owners:       []string{userEmail},
				OwnerPhotos:  op,
				Product:      req.Product,
				Status:       "WIP",
				Summary:      req.Summary,
				// Tags:         req.Tags,
			}

			// For local workspace, expand template variables in the document content.
			// This replaces placeholders like {{title}}, {{owner}}, {{created_date}} etc.
			// with actual values from the document metadata.
			if srv.Config.LocalWorkspace != nil {
				docContent, err := srv.WorkspaceProvider.GetContent(r.Context(), docMeta.ProviderID)
				if err != nil {
					srv.Logger.Warn("error getting document content for template expansion",
						"error", err,
						"doc_id", fileID,
					)
				} else if strings.Contains(docContent.Body, "{{") {
					// Only expand if template variables are present
					templateData := document.NewTemplateDataFromDocument(doc)
					expandedContent := document.ExpandTemplate(docContent.Body, templateData)

					// Update the document content with expanded template
					_, err = srv.WorkspaceProvider.UpdateContent(r.Context(), docMeta.ProviderID, expandedContent)
					if err != nil {
						srv.Logger.Error("error updating document with expanded template",
							"error", err,
							"doc_id", fileID,
						)
						http.Error(w, "Error creating document draft",
							http.StatusInternalServerError)
						return
					}

					srv.Logger.Info("expanded template variables in document",
						"doc_id", fileID,
						"doc_type", req.DocType,
					)
				}
			}

			// Replace the doc header (Google Docs specific).
			googleUpdater := getGoogleDocsUpdater(srv.WorkspaceProvider)
			if googleUpdater == nil {
				srv.Logger.Warn("ReplaceHeader skipped - not using Google Workspace", "doc_id", fileID)
			} else if err = doc.ReplaceHeader(
				srv.Config.BaseURL, true, googleUpdater,
			); err != nil {
				srv.Logger.Error("error replacing draft doc header",
					"error", err,
					"method", r.Method,
					"path", r.URL.Path,
					"doc_id", fileID,
				)
				http.Error(w, "Error creating document draft",
					http.StatusInternalServerError)
				return
			}

			// Create document in the database.
			var contributors []*models.User
			for _, c := range req.Contributors {
				contributors = append(contributors, &models.User{
					EmailAddress: c,
				})
			}
			createdTime := docMeta.CreatedTime
			model := models.Document{
				GoogleFileID:       fileID,
				Contributors:       contributors,
				DocumentCreatedAt:  createdTime,
				DocumentModifiedAt: createdTime,
				DocumentType: models.DocumentType{
					Name: req.DocType,
				},
				Owner: &models.User{
					EmailAddress: userEmail,
				},
				Product: models.Product{
					Name: req.Product,
				},
				Status:  models.WIPDocumentStatus,
				Summary: &req.Summary,
				Title:   req.Title,
			}
			if err := model.Create(srv.DB); err != nil {
				srv.Logger.Error("error creating document in database",
					"error", err,
					"method", r.Method,
					"path", r.URL.Path,
					"doc_id", fileID,
				)
				http.Error(w, "Error creating document draft",
					http.StatusInternalServerError)
				return
			}

			// Share document with the owner
			// Skip sharing for local workspace (not supported)
			if err := srv.WorkspaceProvider.ShareDocument(r.Context(), docMeta.ProviderID, userEmail, "writer"); err != nil {
				// Only log as warning for local workspace, not an error
				if workspaceProvider == "local" {
					srv.Logger.Debug("skipping document sharing for local workspace",
						"method", r.Method,
						"path", r.URL.Path,
						"doc_id", fileID,
					)
				} else {
					srv.Logger.Error("error sharing document with the owner",
						"error", err,
						"method", r.Method,
						"path", r.URL.Path,
						"doc_id", fileID,
					)
					http.Error(w, "Error creating document draft",
						http.StatusInternalServerError)
					return
				}
			}

			// Share document with contributors.
			// Google Drive API limitation is that you can only share files with one
			// user at a time.
			// Skip sharing for local workspace (not supported)
			for _, c := range req.Contributors {
				if err := srv.WorkspaceProvider.ShareDocument(r.Context(), docMeta.ProviderID, c, "writer"); err != nil {
					// Only log as warning for local workspace, not an error
					if workspaceProvider == "local" {
						srv.Logger.Debug("skipping contributor sharing for local workspace",
							"method", r.Method,
							"path", r.URL.Path,
							"doc_id", fileID,
							"contributor", c,
						)
						continue
					}
					srv.Logger.Error("error sharing file with the contributor",
						"error", err,
						"method", r.Method,
						"path", r.URL.Path,
						"doc_id", fileID,
						"contributor", c,
					)
					http.Error(w, "Error creating document draft",
						http.StatusInternalServerError)
					return
				}
			}

			// TODO: Delete draft file in the case of an error.

			// Write response.
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)

			resp := &DraftsResponse{
				ID: fileID,
			}

			enc := json.NewEncoder(w)
			err = enc.Encode(resp)
			if err != nil {
				srv.Logger.Error("error encoding drafts response",
					"error", err,
					"method", r.Method,
					"path", r.URL.Path,
					"doc_id", fileID,
				)
				http.Error(w, "Error creating document draft",
					http.StatusInternalServerError)
				return
			}

			srv.Logger.Info("created draft",
				"method", r.Method,
				"path", r.URL.Path,
				"doc_id", fileID,
			)

			// Request post-processing.
			go func() {
				// Convert document.Document to search.Document for indexing
				searchDoc := &search.Document{
					ObjectID:     doc.ObjectID,
					DocID:        doc.ObjectID,
					Title:        doc.Title,
					DocNumber:    doc.DocNumber,
					DocType:      doc.DocType,
					Product:      doc.Product,
					Status:       doc.Status,
					Owners:       doc.Owners,
					Contributors: doc.Contributors,
					Approvers:    doc.Approvers,
					Summary:      doc.Summary,
					Content:      doc.Content,
					CreatedTime:  doc.CreatedTime,
					ModifiedTime: doc.ModifiedTime,
				}

				// Save document object in search index.
				err := srv.SearchProvider.DraftIndex().Index(r.Context(), searchDoc)
				if err != nil {
					srv.Logger.Error("error saving draft doc in search index",
						"error", err,
						"method", r.Method,
						"path", r.URL.Path,
						"doc_id", fileID,
					)
					http.Error(w, "Error creating document draft",
						http.StatusInternalServerError)
					return
				}

				// Compare search index and database documents to find data inconsistencies.
				// Get document object from search index.
				indexedDoc, err := srv.SearchProvider.DraftIndex().GetObject(r.Context(), fileID)
				if err != nil {
					srv.Logger.Error("error getting search object for data comparison",
						"error", err,
						"method", r.Method,
						"path", r.URL.Path,
						"doc_id", fileID,
					)
					return
				}

				// Convert search.Document to map for comparison
				algoDocBytes, err := json.Marshal(indexedDoc)
				if err != nil {
					srv.Logger.Error("error marshaling indexed document for comparison",
						"error", err,
						"method", r.Method,
						"path", r.URL.Path,
						"doc_id", fileID)
					return
				}
				var algoDoc map[string]any
				if err := json.Unmarshal(algoDocBytes, &algoDoc); err != nil {
					srv.Logger.Error("error unmarshaling indexed document for comparison",
						"error", err,
						"method", r.Method,
						"path", r.URL.Path,
						"doc_id", fileID)
					return
				}

				// Get document from database.
				dbDoc := models.Document{
					GoogleFileID: fileID,
				}
				if err := dbDoc.Get(srv.DB); err != nil {
					srv.Logger.Error(
						"error getting document from database for data comparison",
						"error", err,
						"path", r.URL.Path,
						"method", r.Method,
						"doc_id", fileID,
					)
					return
				}
				// Get all reviews for the document.
				var reviews models.DocumentReviews
				if err := reviews.Find(srv.DB, models.DocumentReview{
					Document: models.Document{
						GoogleFileID: fileID,
					},
				}); err != nil {
					srv.Logger.Error(
						"error getting all reviews for document for data comparison",
						"error", err,
						"method", r.Method,
						"path", r.URL.Path,
						"doc_id", fileID,
					)
					return
				}
				if err := CompareAlgoliaAndDatabaseDocument(
					algoDoc, dbDoc, reviews, srv.Config.DocumentTypes.DocumentType,
				); err != nil {
					srv.Logger.Warn(
						"inconsistencies detected between Algolia and database docs",
						"error", err,
						"method", r.Method,
						"path", r.URL.Path,
						"doc_id", fileID,
					)
				}
			}()

		case "GET":
			// Try database-first approach for better testability
			// If query parameters are provided, fall back to Algolia search
			q := r.URL.Query()

			// Check if this is a simple list request (no search params)
			hasSearchParams := q.Get("facetFilters") != "" || q.Get("facets") != "" || q.Get("hitsPerPage") != ""

			if !hasSearchParams && srv.DB != nil {
				// Simple database query for drafts owned by or contributed to by user
				drafts, err := getDraftsFromDatabase(srv.DB, userEmail)
				if err != nil {
					srv.Logger.Error("error retrieving drafts from database",
						"error", err,
						"method", r.Method,
						"path", r.URL.Path,
					)
					http.Error(w, "Error retrieving document drafts",
						http.StatusInternalServerError)
					return
				}

				// Write response
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)

				enc := json.NewEncoder(w)
				if err := enc.Encode(drafts); err != nil {
					srv.Logger.Error("error encoding drafts",
						"error", err,
						"method", r.Method,
						"path", r.URL.Path,
					)
					return
				}

				srv.Logger.Info("retrieved drafts from database",
					"method", r.Method,
					"path", r.URL.Path,
					"count", len(drafts),
				)
				return
			}

			// Legacy Algolia search path (for production use with search parameters)
			// Get OIDC ID
			id := r.Header.Get("x-amzn-oidc-identity")

			// Parse query
			facetFiltersStr := q.Get("facetFilters")
			facetsStr := q.Get("facets")
			hitsPerPageStr := q.Get("hitsPerPage")
			maxValuesPerFacetStr := q.Get("maxValuesPerFacet")
			pageStr := q.Get("page")

			// Parse facetFilters, handling empty strings
			var facetFilters []string
			if facetFiltersStr != "" {
				facetFilters = strings.Split(facetFiltersStr, ",")
			}

			// Parse facets, handling empty strings
			var facets []string
			if facetsStr != "" {
				facets = strings.Split(facetsStr, ",")
			}
			hitsPerPage, err := strconv.Atoi(hitsPerPageStr)
			if err != nil {
				srv.Logger.Error("error converting to int",
					"error", err,
					"method", r.Method,
					"path", r.URL.Path,
					"hits_per_page", hitsPerPageStr,
				)
				http.Error(w, "Error retrieving document drafts",
					http.StatusInternalServerError)
				return
			}
			_, err = strconv.Atoi(maxValuesPerFacetStr)
			if err != nil {
				srv.Logger.Error("error converting to int",
					"error", err,
					"method", r.Method,
					"path", r.URL.Path,
					"max_values_per_facet", maxValuesPerFacetStr,
				)
				http.Error(w, "Error retrieving document drafts",
					http.StatusInternalServerError)
				return
			}
			page, err := strconv.Atoi(pageStr)
			if err != nil {
				srv.Logger.Error("error converting to int",
					"error", err,
					"method", r.Method,
					"path", r.URL.Path,
					"page", pageStr,
				)
				http.Error(w, "Error retrieving document drafts",
					http.StatusInternalServerError)
				return
			}

			// Build search query for the new provider API
			sortBy := q.Get("sortBy")
			sortOrder := "desc"
			if sortBy == "dateAsc" {
				sortOrder = "asc"
			}

			// Convert facetFilters to the new filters format
			filters := make(map[string][]string)
			for _, filter := range facetFilters {
				if filter == "" {
					continue
				}
				parts := strings.Split(filter, ":")
				if len(parts) == 2 {
					filters[parts[0]] = append(filters[parts[0]], parts[1])
				}
			}

			// Add owner/contributor filter
			if filters["owners"] == nil {
				filters["owners"] = []string{}
			}
			filters["owners"] = append(filters["owners"], userEmail)

			if filters["contributors"] == nil {
				filters["contributors"] = []string{}
			}
			filters["contributors"] = append(filters["contributors"], userEmail)

			searchQuery := &search.SearchQuery{
				Query:     "",
				Page:      page,
				PerPage:   hitsPerPage,
				Filters:   filters,
				Facets:    facets,
				SortBy:    "createdTime",
				SortOrder: sortOrder,
			}

			// Retrieve all documents from search provider
			resp, err := srv.SearchProvider.DraftIndex().Search(r.Context(), searchQuery)
			if err != nil {
				srv.Logger.Error("error retrieving document drafts from search provider",
					"error", err,
					"method", r.Method,
					"path", r.URL.Path,
				)
				http.Error(w, "Error retrieving document drafts",
					http.StatusInternalServerError)
				return
			} // Write response.
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)

			enc := json.NewEncoder(w)
			err = enc.Encode(resp)
			if err != nil {
				srv.Logger.Error("error encoding document drafts",
					"error", err,
					"method", r.Method,
					"path", r.URL.Path,
				)
				http.Error(w, "Error requesting document draft",
					http.StatusInternalServerError)
				return
			}

			srv.Logger.Info("retrieved document drafts",
				"method", r.Method,
				"path", r.URL.Path,
				"o_id", id,
			)

		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
	})
}

// getWorkspaceProviderID constructs a provider ID (format: "provider:docID") based on workspace configuration
func getWorkspaceProviderID(cfg *config.Config, docID string) string {
	workspaceProvider := "google" // default for backwards compatibility
	if cfg.Providers != nil && cfg.Providers.Workspace != "" {
		workspaceProvider = cfg.Providers.Workspace
	}
	return fmt.Sprintf("%s:%s", workspaceProvider, docID)
}

func DraftsDocumentHandler(srv server.Server) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Parse document ID and request type from the URL path.
		docID, reqType, err := parseDocumentsURLPath(
			r.URL.Path, "drafts")
		if err != nil {
			srv.Logger.Error("error parsing drafts URL path",
				"error", err,
				"path", r.URL.Path,
				"method", r.Method,
			)
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		// Get document from database.
		model := models.Document{
			GoogleFileID: docID,
		}
		if err := model.Get(srv.DB); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				srv.Logger.Warn("document draft record not found",
					"path", r.URL.Path,
					"method", r.Method,
					"doc_id", docID,
				)
				http.Error(w, "Draft not found", http.StatusNotFound)
				return
			} else {
				srv.Logger.Error("error getting document draft from database",
					"error", err,
					"path", r.URL.Path,
					"method", r.Method,
					"doc_id", docID,
				)
				http.Error(w, "Error requesting document draft",
					http.StatusInternalServerError)
				return
			}
		}

		// Get reviews for the document.
		var reviews models.DocumentReviews
		if err := reviews.Find(srv.DB, models.DocumentReview{
			Document: models.Document{
				GoogleFileID: docID,
			},
		}); err != nil {
			srv.Logger.Error("error getting reviews for document",
				"error", err,
				"method", r.Method,
				"path", r.URL.Path,
				"doc_id", docID,
			)
			return
		}

		// Get group reviews for the document.
		var groupReviews models.DocumentGroupReviews
		if err := groupReviews.Find(srv.DB, models.DocumentGroupReview{
			Document: models.Document{
				GoogleFileID: docID,
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
			http.Error(w, "Error accessing draft document",
				http.StatusInternalServerError)
			return
		}

		// Make sure document is a draft.
		if doc.Status != "WIP" {
			http.Error(w, "Draft not found", http.StatusNotFound)
			return
		}

		// Authorize request (only allow owners or contributors to get past this
		// point in the handler). We further authorize some methods later that
		// require owner access only.
		userEmail := pkgauth.MustGetUserEmail(r.Context())
		var isOwner, isContributor bool
		if len(doc.Owners) > 0 && doc.Owners[0] == userEmail {
			isOwner = true
		}
		if contains(doc.Contributors, userEmail) {
			isContributor = true
		}
		if !isOwner && !isContributor && !model.ShareableAsDraft {
			http.Error(w,
				"Only owners or contributors can access a non-shared draft document",
				http.StatusUnauthorized)
			return
		}

		// Pass request off to associated subcollection (part of the URL after the
		// draft document ID) handler, if appropriate.
		switch reqType {
		case relatedResourcesDocumentSubcollectionRequestType:
			documentsResourceRelatedResourcesHandler(
				w, r, docID, *doc, srv.Config, srv.Logger, srv.SearchProvider, srv.DB)
			return
		case shareableDocumentSubcollectionRequestType:
			draftsShareableHandler(w, r, docID, *doc, *srv.Config, srv.Logger,
				srv.SearchProvider, getCompatProvider(srv.WorkspaceProvider), srv.DB)
			return
		}

		switch r.Method {
		case "GET":
			now := time.Now()

			// Get document metadata from workspace provider so we can return the latest modified time.
			providerID := getWorkspaceProviderID(srv.Config, docID)
			docMeta, err := srv.WorkspaceProvider.GetDocument(r.Context(), providerID)
			if err != nil {
				srv.Logger.Error("error getting document metadata from workspace",
					"error", err,
					"path", r.URL.Path,
					"method", r.Method,
					"doc_id", docID,
				)
				http.Error(w,
					"Error requesting document draft", http.StatusInternalServerError)
				return
			}

			// Use modified time from metadata.
			doc.ModifiedTime = docMeta.ModifiedTime.Unix()

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
				http.Error(w, "Error getting document draft",
					http.StatusInternalServerError)
				return
			}

			// Write response.
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)

			enc := json.NewEncoder(w)
			err = enc.Encode(docObj)
			if err != nil {
				srv.Logger.Error("error encoding document draft",
					"error", err,
					"method", r.Method,
					"path", r.URL.Path,
					"doc_id", docID,
				)
				http.Error(w, "Error requesting document draft",
					http.StatusInternalServerError)
				return
			}

			srv.Logger.Info("retrieved document draft",
				"method", r.Method,
				"path", r.URL.Path,
				"doc_id", docID,
			)

			// Request post-processing.
			go func() {
				// Update recently viewed documents if this is a document view event. The
				// Add-To-Recently-Viewed header is set in the request from the frontend
				// to differentiate between document views and requests to only retrieve
				// document metadata.
				if r.Header.Get("Add-To-Recently-Viewed") != "" {
					if err := updateRecentlyViewedDocs(
						userEmail, docID, srv.DB, now,
					); err != nil {
						srv.Logger.Error("error updating recently viewed docs",
							"error", err,
							"path", r.URL.Path,
							"method", r.Method,
							"doc_id", docID,
						)
					}
				}

				// Compare search index and database documents to find data inconsistencies.
				// Get document object from search index.
				indexedDoc, err := srv.SearchProvider.DraftIndex().GetObject(r.Context(), docID)
				if err != nil {
					// Only warn because we might be in the process of saving the search index
					// object for a new draft.
					srv.Logger.Warn("error getting search object for data comparison",
						"error", err,
						"method", r.Method,
						"path", r.URL.Path,
						"doc_id", docID,
					)
					return
				}

				// Convert search.Document to map for comparison
				algoDocBytes, err := json.Marshal(indexedDoc)
				if err != nil {
					srv.Logger.Error("error marshaling indexed document for comparison",
						"error", err,
						"method", r.Method,
						"path", r.URL.Path,
						"doc_id", docID)
					return
				}
				var algoDoc map[string]any
				if err := json.Unmarshal(algoDocBytes, &algoDoc); err != nil {
					srv.Logger.Error("error unmarshaling indexed document for comparison",
						"error", err,
						"method", r.Method,
						"path", r.URL.Path,
						"doc_id", docID)
					return
				}
				// Get document from database.
				dbDoc := models.Document{
					GoogleFileID: docID,
				}
				if err := dbDoc.Get(srv.DB); err != nil {
					srv.Logger.Error(
						"error getting document from database for data comparison",
						"error", err,
						"path", r.URL.Path,
						"method", r.Method,
						"doc_id", docID,
					)
					return
				}
				// Get all reviews for the document.
				var reviews models.DocumentReviews
				if err := reviews.Find(srv.DB, models.DocumentReview{
					Document: models.Document{
						GoogleFileID: docID,
					},
				}); err != nil {
					srv.Logger.Error(
						"error getting all reviews for document for data comparison",
						"error", err,
						"method", r.Method,
						"path", r.URL.Path,
						"doc_id", docID,
					)
					return
				}
				if err := CompareAlgoliaAndDatabaseDocument(
					algoDoc, dbDoc, reviews, srv.Config.DocumentTypes.DocumentType,
				); err != nil {
					srv.Logger.Warn(
						"inconsistencies detected between Algolia and database docs",
						"error", err,
						"method", r.Method,
						"path", r.URL.Path,
						"doc_id", docID,
					)
				}
			}()

		case "DELETE":
			// Authorize request.
			if !isOwner {
				http.Error(w,
					"Only owners can delete a draft document",
					http.StatusUnauthorized)
				return
			}

			// Delete document in workspace provider.
			providerID := getWorkspaceProviderID(srv.Config, docID)
			err = srv.WorkspaceProvider.DeleteDocument(r.Context(), providerID)
			if err != nil {
				srv.Logger.Error(
					"error deleting document",
					"error", err,
					"method", r.Method,
					"path", r.URL.Path,
					"doc_id", docID,
				)
				http.Error(w, "Error deleting document draft",
					http.StatusInternalServerError)
				return
			}

			// Delete object from search index.
			err := srv.SearchProvider.DraftIndex().Delete(r.Context(), docID)
			if err != nil {
				srv.Logger.Error(
					"error deleting document draft from search index",
					"error", err,
					"method", r.Method,
					"path", r.URL.Path,
					"doc_id", docID,
				)
				http.Error(w, "Error deleting document draft",
					http.StatusInternalServerError)
				return
			}

			// Note: Delete is synchronous with the new provider API
			if false { // Remove the old Wait() logic
				srv.Logger.Error(
					"error deleting document draft from search index",
					"error", err,
					"method", r.Method,
					"path", r.URL.Path,
					"doc_id", docID,
				)
				http.Error(w, "Error deleting document draft",
					http.StatusInternalServerError)
				return
			}

			// Delete document in the database.
			d := models.Document{
				GoogleFileID: docID,
			}
			if err := d.Delete(srv.DB); err != nil {
				srv.Logger.Error(
					"error deleting document draft in database",
					"error", err,
					"method", r.Method,
					"path", r.URL.Path,
					"doc_id", docID,
				)
				http.Error(w, "Error deleting document draft",
					http.StatusInternalServerError)
				return
			}

			resp := &DraftsResponse{
				ID: docID,
			}

			// Write response.
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)

			enc := json.NewEncoder(w)
			err = enc.Encode(resp)
			if err != nil {
				srv.Logger.Error(
					"error encoding response",
					"error", err,
					"method", r.Method,
					"path", r.URL.Path,
					"doc_id", docID,
				)
				http.Error(w, "Error deleting document draft",
					http.StatusInternalServerError)
				return
			}

		case "PATCH":
			// Authorize request.
			if !isOwner {
				http.Error(w,
					"Only owners can patch a draft document",
					http.StatusUnauthorized)
				return
			}

			// Decode request. The request struct validates that the request only
			// contains fields that are allowed to be patched.
			var req DraftsPatchRequest
			if err := decodeRequest(r, &req); err != nil {
				srv.Logger.Error("error decoding draft patch request", "error", err)
				http.Error(w, fmt.Sprintf("Bad request: %q", err),
					http.StatusBadRequest)
				return
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

			// Validate product if it is in the patch request.
			var productAbbreviation string
			if req.Product != nil && *req.Product != "" {
				p := models.Product{Name: *req.Product}
				if err := p.Get(srv.DB); err != nil {
					srv.Logger.Error("error getting product",
						"error", err,
						"method", r.Method,
						"path", r.URL.Path,
						"product", req.Product,
						"doc_id", docID)
					http.Error(w, "Bad request: invalid product",
						http.StatusBadRequest)
					return
				}

				// Set product abbreviation because we use this later to update the
				// doc number in the Algolia object.
				productAbbreviation = p.Abbreviation
			}

			// Validate custom fields.
			if req.CustomFields != nil {
				for _, cf := range *req.CustomFields {
					cef, ok := doc.CustomEditableFields[cf.Name]
					if !ok {
						srv.Logger.Error("custom field not found",
							"error", err,
							"method", r.Method,
							"path", r.URL.Path,
							"custom_field", cf.Name,
							"doc_id", docID)
						http.Error(w, "Bad request: invalid custom field",
							http.StatusBadRequest)
						return
					}
					if cf.DisplayName != cef.DisplayName {
						srv.Logger.Error("invalid custom field display name",
							"error", err,
							"method", r.Method,
							"path", r.URL.Path,
							"custom_field", cf.Name,
							"custom_field_display_name", cf.DisplayName,
							"doc_id", docID)
						http.Error(w, "Bad request: invalid custom field display name",
							http.StatusBadRequest)
						return
					}
					if cf.Type != cef.Type {
						srv.Logger.Error("invalid custom field type",
							"error", err,
							"method", r.Method,
							"path", r.URL.Path,
							"custom_field", cf.Name,
							"custom_field_type", cf.Type,
							"doc_id", docID)
						http.Error(w, "Bad request: invalid custom field type",
							http.StatusBadRequest)
						return
					}
				}
			}

			// Check if document is locked (Google Docs specific).
			googleProvider := getGoogleDocsProvider(srv.WorkspaceProvider)
			if googleProvider != nil {
				locked, err := hcd.IsLocked(docID, srv.DB, googleProvider, srv.Logger)
				if err != nil {
					srv.Logger.Error("error checking document locked status",
						"error", err,
						"method", r.Method,
						"path", r.URL.Path,
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

			// Compare contributors in request and stored object in Algolia
			// before we save the patched objected
			// Find out contributors to share the document with
			var contributorsToAddSharing []string
			var contributorsToRemoveSharing []string
			if req.Contributors != nil {
				if len(doc.Contributors) == 0 && len(*req.Contributors) != 0 {
					// If there are no contributors of the document
					// add the contributors in the request
					contributorsToAddSharing = *req.Contributors
				} else if len(*req.Contributors) != 0 {
					// Only compare when there are stored contributors
					// and contributors in the request
					contributorsToAddSharing = compareSlices(
						doc.Contributors, *req.Contributors)
				}
				// Find out contributors to remove from sharing the document
				// var contributorsToRemoveSharing []string
				// TODO: figure out how we want to handle user removing all contributors
				// from the sidebar select
				if len(doc.Contributors) != 0 && len(*req.Contributors) != 0 {
					// Compare contributors when there are stored contributors
					// and there are contributors in the request
					contributorsToRemoveSharing = compareSlices(
						*req.Contributors, doc.Contributors)
				}
			}

			// Share file with contributors.
			// Google Drive API limitation is that you can only share files with one
			// user at a time.
			providerID := getWorkspaceProviderID(srv.Config, docID)
			for _, c := range contributorsToAddSharing {
				if err := srv.WorkspaceProvider.ShareDocument(r.Context(), providerID, c, "writer"); err != nil {
					srv.Logger.Error("error sharing file with the contributor",
						"error", err,
						"method", r.Method,
						"path", r.URL.Path,
						"doc_id", docID,
						"contributor", c)
					http.Error(w, "Error patching document draft",
						http.StatusInternalServerError)
					return
				}
			}
			if len(contributorsToAddSharing) > 0 {
				srv.Logger.Info("shared document with contributors",
					"method", r.Method,
					"path", r.URL.Path,
					"contributors_count", len(contributorsToAddSharing),
				)
			}

			// Remove contributors from file.
			// This unfortunately needs to be done one user at a time
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
						http.Error(w, "Error patching document draft",
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

			// Approvers.
			if req.Approvers != nil {
				doc.Approvers = *req.Approvers

				var approvers []*models.User
				for _, a := range doc.Approvers {
					u := models.User{
						EmailAddress: a,
					}
					approvers = append(approvers, &u)
				}
				model.Approvers = approvers
			}

			// Approver groups.
			if req.ApproverGroups != nil {
				doc.ApproverGroups = *req.ApproverGroups

				approverGroups := make([]*models.Group, len(doc.ApproverGroups))
				for i, a := range doc.ApproverGroups {
					g := models.Group{
						EmailAddress: a,
					}
					approverGroups[i] = &g
				}
				model.ApproverGroups = approverGroups
			}

			// Contributors.
			if req.Contributors != nil {
				doc.Contributors = *req.Contributors

				var contributors []*models.User
				for _, a := range doc.Contributors {
					u := &models.User{
						EmailAddress: a,
					}
					contributors = append(contributors, u)
				}
				model.Contributors = contributors
			}

			// Custom fields.
			if req.CustomFields != nil {
				for _, cf := range *req.CustomFields {
					switch cf.Type {
					case "STRING":
						if v, ok := cf.Value.(string); ok {
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

							model.CustomFields = models.UpsertStringDocumentCustomField(
								model.CustomFields,
								doc.DocType,
								cf.DisplayName,
								v,
							)
						} else {
							srv.Logger.Error("invalid value type for string custom field",
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
					case "PEOPLE":
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
						cfVal := []string{}
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
							if v, ok := v.(string); ok {
								cfVal = append(cfVal, v)
							} else {
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

						model.CustomFields, err = models.
							UpsertStringSliceDocumentCustomField(
								model.CustomFields,
								doc.DocType,
								cf.DisplayName,
								cfVal,
							)
						if err != nil {
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

			// Make sure all custom fields in the database model have the document ID.
			for _, cf := range model.CustomFields {
				cf.DocumentID = model.ID
			}

			// Document modified time.
			model.DocumentModifiedAt = time.Unix(doc.ModifiedTime, 0)

			// Owner.
			if req.Owners != nil {
				doc.Owners = *req.Owners
				model.Owner = &models.User{
					EmailAddress: doc.Owners[0],
				}

				// Share document with new owner.
				providerID = getWorkspaceProviderID(srv.Config, docID)
				if err := srv.WorkspaceProvider.ShareDocument(
					r.Context(), providerID, doc.Owners[0], "writer"); err != nil {
					srv.Logger.Error("error sharing file with new owner",
						"error", err,
						"method", r.Method,
						"path", r.URL.Path,
						"doc_id", docID,
						"new_owner", doc.Owners[0])
					http.Error(w, "Error patching document draft",
						http.StatusInternalServerError)
					return
				}
			}

			// Product.
			if req.Product != nil {
				doc.Product = *req.Product
				model.Product = models.Product{Name: *req.Product}

				// Remove product ID so it gets updated during upsert (or else it will
				// override the product name).
				model.ProductID = 0

				// Update doc number in document.
				doc.DocNumber = fmt.Sprintf("%s-???", productAbbreviation)
			}

			// Summary.
			if req.Summary != nil {
				doc.Summary = *req.Summary
				model.Summary = req.Summary
			}

			// Title.
			if req.Title != nil {
				doc.Title = *req.Title
				model.Title = *req.Title
			}

			// Send email to new owner.
			if srv.Config.Email != nil && srv.Config.Email.Enabled &&
				req.Owners != nil {
				// Get document URL.
				docURL, err := getDocumentURL(srv.Config.BaseURL, docID)
				if err != nil {
					srv.Logger.Error("error getting document URL",
						"error", err,
						"doc_id", docID,
						"method", r.Method,
						"path", r.URL.Path,
					)
					http.Error(w, "Error updating document draft",
						http.StatusInternalServerError)
					return
				}

				// Get name of new document owner.
				newOwner := email.User{
					EmailAddress: doc.Owners[0],
				}
				ppl, err := srv.WorkspaceProvider.SearchPeople(
					r.Context(), doc.Owners[0])
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

				// Get name of old document owner.
				oldOwner := email.User{
					EmailAddress: userEmail,
				}
				ppl, err = srv.WorkspaceProvider.SearchPeople(
					r.Context(), userEmail)
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
					srv.Logger.Error("error sending new owner email",
						"error", err,
						"method", r.Method,
						"path", r.URL.Path,
						"doc_id", docID,
					)
					http.Error(w, "Error updating document draft",
						http.StatusInternalServerError)
					return
				}
			}

			// Update document in the database.
			if err := model.Upsert(srv.DB); err != nil {
				srv.Logger.Error("error updating document in the database",
					"error", err,
					"method", r.Method,
					"path", r.URL.Path,
					"doc_id", docID,
				)
				http.Error(w, "Error updating document draft",
					http.StatusInternalServerError)
				return
			}

			// Replace the doc header (Google Docs specific).
			googleUpdater := getGoogleDocsUpdater(srv.WorkspaceProvider)
			if googleUpdater == nil {
				srv.Logger.Warn("ReplaceHeader skipped - not using Google Workspace", "doc_id", docID)
			} else if err := doc.ReplaceHeader(
				srv.Config.BaseURL, true, googleUpdater,
			); err != nil {
				srv.Logger.Error("error replacing draft doc header",
					"error", err,
					"method", r.Method,
					"path", r.URL.Path,
					"doc_id", docID,
				)
				http.Error(w, "Error replacing header of document draft",
					http.StatusInternalServerError)
				return
			}

			// Rename document with new title.
			providerID = getWorkspaceProviderID(srv.Config, docID)
			if err := srv.WorkspaceProvider.RenameDocument(r.Context(), providerID,
				fmt.Sprintf("[%s] %s", doc.DocNumber, doc.Title)); err != nil {
				srv.Logger.Warn("failed to rename document in workspace provider",
					"error", err,
					"doc_id", docID)
			}

			w.WriteHeader(http.StatusOK)

			srv.Logger.Info("patched draft document",
				"method", r.Method,
				"path", r.URL.Path,
				"doc_id", docID,
			)

			// Request post-processing.
			go func() {
				// Convert document.Document to search.Document for indexing
				searchDoc := &search.Document{
					ObjectID:     doc.ObjectID,
					DocID:        doc.ObjectID,
					Title:        doc.Title,
					DocNumber:    doc.DocNumber,
					DocType:      doc.DocType,
					Product:      doc.Product,
					Status:       doc.Status,
					Owners:       doc.Owners,
					Contributors: doc.Contributors,
					Approvers:    doc.Approvers,
					Summary:      doc.Summary,
					Content:      doc.Content,
					CreatedTime:  doc.CreatedTime,
					ModifiedTime: doc.ModifiedTime,
				}

				// Save modified draft doc object in search index.
				err := srv.SearchProvider.DraftIndex().Index(r.Context(), searchDoc)
				if err != nil {
					srv.Logger.Error("error saving patched draft doc in search index",
						"error", err,
						"method", r.Method,
						"path", r.URL.Path,
						"doc_id", docID,
					)
					return
				}

				// Note: Index is synchronous with the new provider API
				if false { // Remove the old Wait() logic
					srv.Logger.Error("error saving patched draft doc in search index",
						"error", err,
						"method", r.Method,
						"path", r.URL.Path,
						"doc_id", docID,
					)
					return
				}

				// Compare search index and database documents to find data inconsistencies.
				// Get document object from search index.
				indexedDoc, err := srv.SearchProvider.DraftIndex().GetObject(r.Context(), docID)
				if err != nil {
					srv.Logger.Error("error getting search object for data comparison",
						"error", err,
						"method", r.Method,
						"path", r.URL.Path,
						"doc_id", docID,
					)
					return
				}

				// Convert search.Document to map for comparison
				algoDocBytes, err := json.Marshal(indexedDoc)
				if err != nil {
					srv.Logger.Error("error marshaling indexed document for comparison",
						"error", err,
						"method", r.Method,
						"path", r.URL.Path,
						"doc_id", docID)
					return
				}
				var algoDoc map[string]any
				if err := json.Unmarshal(algoDocBytes, &algoDoc); err != nil {
					srv.Logger.Error("error unmarshaling indexed document for comparison",
						"error", err,
						"method", r.Method,
						"path", r.URL.Path,
						"doc_id", docID)
					return
				}

				// Get document from database.
				dbDoc := models.Document{
					GoogleFileID: docID,
				}
				if err := dbDoc.Get(srv.DB); err != nil {
					srv.Logger.Error(
						"error getting document from database for data comparison",
						"error", err,
						"path", r.URL.Path,
						"method", r.Method,
						"doc_id", docID,
					)
					return
				}
				// Get all reviews for the document.
				var reviews models.DocumentReviews
				if err := reviews.Find(srv.DB, models.DocumentReview{
					Document: models.Document{
						GoogleFileID: docID,
					},
				}); err != nil {
					srv.Logger.Error(
						"error getting all reviews for document for data comparison",
						"error", err,
						"method", r.Method,
						"path", r.URL.Path,
						"doc_id", docID,
					)
					return
				}
				if err := CompareAlgoliaAndDatabaseDocument(
					algoDoc, dbDoc, reviews, srv.Config.DocumentTypes.DocumentType,
				); err != nil {
					srv.Logger.Warn(
						"inconsistencies detected between Algolia and database docs",
						"error", err,
						"method", r.Method,
						"path", r.URL.Path,
						"doc_id", docID,
					)
				}
			}()

		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
	})
}

// getDocTypeTemplate returns the file ID of the template for a specified
// document type or an empty string if not found.
func getDocTypeTemplate(
	docTypes []*config.DocumentType,
	docType string,
) string {
	template := ""

	for _, t := range docTypes {
		if t.Name == docType {
			template = t.Template
			break
		}
	}

	return template
}

// validateDocType returns true if the name (docType) is contained in the a
// slice of configured document types.
func validateDocType(
	docTypes []*config.DocumentType,
	docType string,
) bool {
	for _, t := range docTypes {
		if t.Name == docType {
			return true
		}
	}

	return false
}

// removeSharing lists permissions for a document and then
// deletes the permission for the supplied user email
func removeSharing(provider workspace.Provider, docID, email string) error {
	permissions, err := provider.ListPermissions(docID)
	if err != nil {
		return err
	}
	for _, p := range permissions {
		if p.EmailAddress == email {
			return provider.DeletePermission(docID, p.Id)
		}
	}
	return nil
}

// getDraftsFromDatabase retrieves drafts from the database for a given user.
// Returns drafts where the user is either an owner or contributor.
func getDraftsFromDatabase(db *gorm.DB, userEmail string) ([]map[string]interface{}, error) {
	var documents []models.Document

	// Find documents where user is owner or contributor and status is WIP (draft)
	err := db.
		Preload("Owner").
		Preload("Contributors").
		Preload("Approvers").
		Preload("Product").
		Preload("DocumentType").
		Joins("LEFT JOIN document_contributors ON documents.id = document_contributors.document_id").
		Joins("LEFT JOIN users AS contributors ON document_contributors.user_id = contributors.id").
		Joins("LEFT JOIN users AS owners ON documents.owner_id = owners.id").
		Where("documents.status = ?", models.WIPDocumentStatus).
		Where("owners.email_address = ? OR contributors.email_address = ?", userEmail, userEmail).
		Group("documents.id").
		Find(&documents).Error

	if err != nil {
		return nil, err
	}

	// Convert to response format
	result := make([]map[string]interface{}, len(documents))
	for i, doc := range documents {
		result[i] = map[string]interface{}{
			"id":           doc.GoogleFileID,
			"title":        doc.Title,
			"status":       doc.Status,
			"product":      doc.Product.Name,
			"documentType": doc.DocumentType.Name,
			"createdTime":  doc.DocumentCreatedAt,
			"modifiedTime": doc.DocumentModifiedAt,
		}

		// Add owner if present
		if doc.Owner != nil {
			result[i]["owners"] = []string{doc.Owner.EmailAddress}
		}

		// Add contributors if present
		if len(doc.Contributors) > 0 {
			contributors := make([]string, len(doc.Contributors))
			for j, c := range doc.Contributors {
				contributors[j] = c.EmailAddress
			}
			result[i]["contributors"] = contributors
		}

		// Add approvers if present
		if len(doc.Approvers) > 0 {
			approvers := make([]string, len(doc.Approvers))
			for j, a := range doc.Approvers {
				approvers[j] = a.EmailAddress
			}
			result[i]["approvers"] = approvers
		}
	}

	return result, nil
}
