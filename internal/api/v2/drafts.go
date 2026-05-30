package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/hashicorp-forge/hermes/internal/config"
	"github.com/hashicorp-forge/hermes/internal/email"
	"github.com/hashicorp-forge/hermes/internal/helpers"
	"github.com/hashicorp-forge/hermes/internal/server"
	pkgauth "github.com/hashicorp-forge/hermes/pkg/auth"
	"github.com/hashicorp-forge/hermes/pkg/document"
	hcd "github.com/hashicorp-forge/hermes/pkg/hashicorpdocs"
	"github.com/hashicorp-forge/hermes/pkg/models"
	"github.com/hashicorp-forge/hermes/pkg/search"
	"github.com/hashicorp-forge/hermes/pkg/workspace"
)

// DraftsRequest represents a request to create or manage a draft document.
type DraftsRequest struct {
	DocType             string   `json:"docType,omitempty"`
	Product             string   `json:"product,omitempty"`
	ProductAbbreviation string   `json:"productAbbreviation,omitempty"`
	Summary             string   `json:"summary,omitempty"`
	Title               string   `json:"title"`
	Contributors        []string `json:"contributors,omitempty"`
	Tags                []string `json:"tags,omitempty"`
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

// DraftsResponse represents the response for draft operations.
type DraftsResponse struct {
	ID string `json:"id"`
}

// DraftsHandler returns an HTTP handler for draft operations.
//
//nolint:gocognit,gocyclo // Legacy HTTP entrypoint; request handling is intentionally centralized.
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
		case httpMethodPost:
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
				srv.Logger.Warn("draft title is required",
					"method", r.Method,
					"path", r.URL.Path,
					"user_email", userEmail)
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
			title := fmt.Sprintf("%s-%s", req.ProductAbbreviation, req.Title)

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
			if workspaceProvider == workspaceProviderLocal && srv.Config.LocalWorkspace != nil {
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
			if err := createDraftWithSearchOutbox(srv.DB, &model, doc); err != nil {
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
				if workspaceProvider == workspaceProviderLocal {
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
					if workspaceProvider == workspaceProviderLocal {
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

			} // Write response.
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

		case httpMethodGet:
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

// DraftsDocumentHandler returns an HTTP handler for individual draft document operations.
//
//nolint:gocognit,gocyclo // Legacy HTTP entrypoint; large draft flows are kept together to minimize behavior churn.
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
		model := srv.NewDocumentByFileID(docID)
		if err := model.Get(srv.DB); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				srv.Logger.Warn("document draft record not found",
					"path", r.URL.Path,
					"method", r.Method,
					"doc_id", docID,
				)
				http.Error(w, "Draft not found", http.StatusNotFound)
				return
			}

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

		// Get reviews for the document.
		var reviews models.DocumentReviews
		if err := reviews.Find(srv.DB, models.DocumentReview{
			Document: srv.NewDocumentByFileID(docID),
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
			Document: srv.NewDocumentByFileID(docID),
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
		if doc.Status != docStatusWIP {
			http.Error(w, "Draft not found", http.StatusNotFound)
			return
		}

		// Authorize request (only allow owners or contributors to get past this
		// point in the handler). We further authorize some methods later that
		// require owner access only.
		userEmail := pkgauth.MustGetUserEmail(r.Context())
		var isOwner, isContributor bool
		if len(doc.Owners) > 0 && strings.EqualFold(doc.Owners[0], userEmail) {
			isOwner = true
		}
		if contains(doc.Contributors, userEmail) {
			isContributor = true
		}
		if !isOwner && !isContributor && !model.ShareableAsDraft {
			srv.Logger.Warn("unauthorized draft access attempt",
				"method", r.Method,
				"path", r.URL.Path,
				"doc_id", docID,
				"user_email", userEmail,
				"is_owner", isOwner,
				"is_contributor", isContributor,
				"shareable_as_draft", model.ShareableAsDraft)
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
				w, r, docID, *doc, srv.Config, srv.Logger, srv.SearchProvider, srv.DB, srv.IsSharePoint())
			return
		case shareableDocumentSubcollectionRequestType:
			draftsShareableHandler(w, r, docID, *doc, *srv.Config, srv.Logger,
				srv.SearchProvider, getCompatProvider(srv.WorkspaceProvider), srv.DB, srv.IsSharePoint())
			return
		}

		switch r.Method {
		case httpMethodGet:
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
				dbDoc := srv.NewDocumentByFileID(docID)
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
					Document: srv.NewDocumentByFileID(docID),
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

		case httpMethodDelete:
			// Authorize request.
			if !isOwner {
				srv.Logger.Warn("unauthorized draft deletion attempt",
					"method", r.Method,
					"path", r.URL.Path,
					"doc_id", docID,
					"user_email", userEmail)
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

			// Delete document in the database and enqueue search projection update.
			d := models.Document{
				GoogleFileID: docID,
			}
			if err := deleteDraftWithSearchOutbox(srv.DB, &d, docID); err != nil {
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

		case httpMethodPatch:
			// Authorize request.
			if !isOwner {
				srv.Logger.Warn("unauthorized draft patch attempt",
					"method", r.Method,
					"path", r.URL.Path,
					"doc_id", docID,
					"user_email", userEmail)
				http.Error(w,
					"Only owners can patch a draft document",
					http.StatusForbidden)
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
				if err := validateEditableCustomFields(*req.CustomFields, *doc); err != nil {
					logCustomFieldValidationError(srv.Logger, err, r, docID)
					http.Error(w, err.Error(), http.StatusBadRequest)
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
				if srv.SharePoint != nil {
					if err := srv.SharePoint.ShareFileWithMultipleUsers(docID, "writer", contributorsToAddSharing); err != nil {
						srv.Logger.Error("error sharing file with the contributor",
							"error", err,
							"method", r.Method,
							"path", r.URL.Path,
							"doc_id", docID,
							"contributor", contributorsToAddSharing)
						http.Error(w, "Error patching document draft",
							http.StatusInternalServerError)
						return
					}
				} else {
					for _, c := range contributorsToAddSharing {
						if err := srv.GWService.ShareFile(docID, c, "writer"); err != nil {
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
				}
				srv.Logger.Info("shared document with contributors",
					"method", r.Method,
					"path", r.URL.Path,
					"contributors_count", len(contributorsToAddSharing),
				)

				docURL := fmt.Sprintf("%s/document/%s?draft=true", srv.Config.BaseURL, docID)

				srv.Logger.Info("contributor email queued",
					"doc_id", docID,
					"contributor_count", len(contributorsToAddSharing),
					"method", r.Method,
					"path", r.URL.Path,
				)

				go helpers.SendEmailWithRetry(
					&srv,
					func() error {
						return email.SendContributorAddedEmail(
							email.ContributorAddedEmailData{
								BaseURL:           srv.Config.BaseURL,
								DocumentOwner:     doc.Owners[0],
								DocumentShortName: doc.DocNumber,
								DocumentTitle:     doc.Title,
								DocumentType:      doc.DocType,
								DocumentStatus:    doc.Status,
								DocumentURL:       docURL,
								Product:           doc.Product,
							},
							contributorsToAddSharing,
							srv.Config.Email.FromAddress,
							srv.GetEmailSender(),
						)
					},
					docID,
					"contributor_added",
					r,
				)
			}
			// Build permission map for contributor removal.
			emailToPermissionIDsMap := make(map[string][]string)
			if srv.SharePoint != nil {
				permissions, err := srv.SharePoint.ListPermissions(docID)
				if err != nil {
					srv.Logger.Error("error getting file permissions",
						"error", err,
						"path", r.URL.Path,
						"method", r.Method,
						"doc_id", docID,
					)
					http.Error(w,
						"Error requesting document draft", http.StatusInternalServerError)
					return
				}
				for _, p := range permissions {
					if p.GrantedTo.User.Email == "" {
						continue
					}
					if slices.Contains(p.Role, "owner") {
						continue
					}
					email := p.GrantedTo.User.Email
					if _, exists := emailToPermissionIDsMap[email]; !exists {
						emailToPermissionIDsMap[email] = make([]string, 0)
					}
					emailToPermissionIDsMap[email] = append(
						emailToPermissionIDsMap[email], p.ID)
				}
			} else {
				permissions, err := srv.GWService.ListPermissions(docID)
				if err != nil {
					srv.Logger.Error("error getting file permissions",
						"error", err,
						"path", r.URL.Path,
						"method", r.Method,
						"doc_id", docID,
					)
					http.Error(w,
						"Error requesting document draft", http.StatusInternalServerError)
					return
				}
				for _, p := range permissions {
					if p.EmailAddress == "" {
						continue
					}
					if p.Role == "owner" {
						continue
					}
					if _, exists := emailToPermissionIDsMap[p.EmailAddress]; !exists {
						emailToPermissionIDsMap[p.EmailAddress] = make([]string, 0)
					}
					emailToPermissionIDsMap[p.EmailAddress] = append(
						emailToPermissionIDsMap[p.EmailAddress], p.Id)
				}
			}

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
				model.Approvers = usersFromEmails(doc.Approvers)
			}

			// Approver groups.
			if req.ApproverGroups != nil {
				doc.ApproverGroups = *req.ApproverGroups
				model.ApproverGroups = groupsFromEmails(doc.ApproverGroups)
			}

			// Contributors.
			if req.Contributors != nil {
				doc.Contributors = *req.Contributors
				model.Contributors = usersFromEmails(doc.Contributors)
			}

			// Custom fields.
			if req.CustomFields != nil {
				srv.Logger.Info("processing custom fields",
					"doc_id", docID,
					"custom_fields_count", len(*req.CustomFields))

				for _, cf := range *req.CustomFields {
					if cf.Type == fieldTypePeople {
						if _, err := parsePeopleCustomFieldValue(cf); err != nil {
							logCustomFieldValidationError(srv.Logger, err, r, docID)
							http.Error(w, err.Error(), http.StatusBadRequest)
							return
						}
					}

					switch cf.Type {
					case fieldTypeString:
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
					case fieldTypePeople:

						// IMPORTANT: Query database for old stakeholders BEFORE any modifications
						var oldStakeholders []string
						if strings.EqualFold(cf.Name, "stakeholders") || strings.EqualFold(cf.DisplayName, "Stakeholders") {
							srv.Logger.Info("querying database for old stakeholders",
								"cf_name", cf.Name,
								"doc_id", docID)

							// Query the document's current custom fields directly from database
							var dbDoc models.Document
							err := srv.DB.
								Preload("CustomFields.DocumentTypeCustomField").
								Where("id = ?", model.ID).
								First(&dbDoc).Error

							if err == nil {
								// Find the stakeholders custom field in the database result
								for _, existingCF := range dbDoc.CustomFields {
									if existingCF.DocumentTypeCustomField.Name == "Stakeholders" {
										// Parse the JSON value from database
										var stakeholderEmails []string
										if err := json.Unmarshal([]byte(existingCF.Value), &stakeholderEmails); err == nil {
											oldStakeholders = stakeholderEmails
											srv.Logger.Info("found old stakeholders from database",
												"old_stakeholders_count", len(oldStakeholders),
												"doc_id", docID)
										} else {
											srv.Logger.Warn("failed to unmarshal old stakeholders",
												"error", err,
												"doc_id", docID,
												"field_name", existingCF.DocumentTypeCustomField.Name)
										}
										break
									}
								}
							} else {
								srv.Logger.Warn("failed to query database for old stakeholders",
									"error", err,
									"doc_id", docID)
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

						updatedFields, err := updateModelCustomFields(
							model.CustomFields,
							doc.DocType,
							[]document.CustomField{cf},
						)
						if err != nil {
							logCustomFieldValidationError(srv.Logger, err, r, docID)
							http.Error(w, err.Error(), http.StatusBadRequest)
							return
						}
						model.CustomFields = updatedFields
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
			setModelCustomFieldDocumentIDs(model.CustomFields, model.ID)

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

				// Rename the file to match the new title.
				// Extract the product abbreviation from DocNumber (e.g., "HCP-???" -> "HCP").
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
				}

				var renameErr error
				if srv.SharePoint != nil {
					renameErr = srv.SharePoint.RenameFile(docID, newFileName)
				} else {
					renameErr = srv.GWService.RenameFile(docID, newFileName)
				}
				if renameErr != nil {
					srv.Logger.Error("error renaming file",
						"error", renameErr,
						"method", r.Method,
						"path", r.URL.Path,
						"doc_id", docID,
						"new_file_name", newFileName)
					// Non-fatal: continue even if rename fails
					srv.Logger.Warn("continuing draft patch despite file rename failure")
				} else {
					srv.Logger.Info("successfully renamed file",
						"method", r.Method,
						"path", r.URL.Path,
						"doc_id", docID,
						"new_file_name", newFileName)
				}
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
					http.Error(w, "Error updating document draft",
						http.StatusInternalServerError)
					return
				}
			}

			// Update document in the database and enqueue search projection update.
			if err := updateDraftWithSearchOutbox(srv.DB, &model, doc); err != nil {
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

		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
	})
}

func createDraftWithSearchOutbox(db *gorm.DB, model *models.Document, doc *document.Document) error {
	return enqueueDraftUpsertWithSearchOutbox(db, model.Create, doc, models.SearchEventDraftCreated)
}

func updateDraftWithSearchOutbox(db *gorm.DB, model *models.Document, doc *document.Document) error {
	return enqueueDraftUpsertWithSearchOutbox(db, model.Upsert, doc, models.SearchEventDraftUpdated)
}

func enqueueDraftUpsertWithSearchOutbox(
	db *gorm.DB,
	mutate func(*gorm.DB) error,
	doc *document.Document,
	eventType string,
) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := mutate(tx); err != nil {
			return err
		}

		payload, err := draftSearchPayload(doc)
		if err != nil {
			return err
		}

		return models.EnqueueSearchOutboxEventWithSequence(tx, &models.SearchOutboxEvent{
			EventType:     eventType,
			AggregateID:   doc.ObjectID,
			AggregateType: models.SearchAggregateDraft,
			IndexName:     models.SearchIndexDrafts,
			Operation:     models.SearchOutboxOperationUpsert,
			Payload:       payload,
		})
	})
}

func deleteDraftWithSearchOutbox(db *gorm.DB, model *models.Document, docID string) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := model.Delete(tx); err != nil {
			return err
		}

		return models.EnqueueSearchOutboxEventWithSequence(tx, &models.SearchOutboxEvent{
			EventType:     models.SearchEventDraftDeleted,
			AggregateID:   docID,
			AggregateType: models.SearchAggregateDraft,
			IndexName:     models.SearchIndexDrafts,
			Operation:     models.SearchOutboxOperationDelete,
		})
	})
}

func draftSearchPayload(doc *document.Document) (map[string]any, error) {
	payload, err := doc.ToAlgoliaObject(false)
	if err != nil {
		return nil, fmt.Errorf("error converting draft to search object: %w", err)
	}
	return payload, nil
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
	for i := range documents {
		result[i] = map[string]interface{}{
			"id":           documents[i].GoogleFileID,
			"title":        documents[i].Title,
			"status":       documents[i].Status,
			"product":      documents[i].Product.Name,
			"documentType": documents[i].DocumentType.Name,
			"createdTime":  documents[i].DocumentCreatedAt,
			"modifiedTime": documents[i].DocumentModifiedAt,
		}

		// Add owner if present
		if documents[i].Owner != nil {
			result[i]["owners"] = []string{documents[i].Owner.EmailAddress}
		}

		// Add contributors if present
		if len(documents[i].Contributors) > 0 {
			contributors := make([]string, len(documents[i].Contributors))
			for j, c := range documents[i].Contributors {
				contributors[j] = c.EmailAddress
			}
			result[i]["contributors"] = contributors
		}

		// Add approvers if present
		if len(documents[i].Approvers) > 0 {
			approvers := make([]string, len(documents[i].Approvers))
			for j, a := range documents[i].Approvers {
				approvers[j] = a.EmailAddress
			}
			result[i]["approvers"] = approvers
		}
	}

	return result, nil
}
