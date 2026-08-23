package api

import (
	"encoding/json"
	"errors"
	"net/http"

	pkgauth "github.com/hashicorp-forge/hermes/pkg/auth"

	"gorm.io/gorm"

	"github.com/hashicorp-forge/hermes/internal/server"
	"github.com/hashicorp-forge/hermes/pkg/document"
	"github.com/hashicorp-forge/hermes/pkg/models"
)

// ProjectRelatedResourcesGetResponse contains related resources for a project.
type ProjectRelatedResourcesGetResponse struct {
	ExternalLinks   []ProjectRelatedResourcesGetResponseExternalLink   `json:"externalLinks,omitempty"`
	HermesDocuments []ProjectRelatedResourcesGetResponseHermesDocument `json:"hermesDocuments,omitempty"`
}

// ProjectRelatedResourcesGetResponseExternalLink represents an external link resource.
type ProjectRelatedResourcesGetResponseExternalLink struct {
	Name      string `json:"name"`
	URL       string `json:"url"`
	SortOrder int    `json:"sortOrder"`
}

// ProjectRelatedResourcesGetResponseHermesDocument represents a Hermes document resource.
type ProjectRelatedResourcesGetResponseHermesDocument struct {
	FileID         string   `json:"FileID"`
	Title          string   `json:"title"`
	DocumentType   string   `json:"documentType"`
	DocumentNumber string   `json:"documentNumber"`
	Product        string   `json:"product"`
	Status         string   `json:"status"`
	Summary        string   `json:"summary"`
	Owners         []string `json:"owners"`
	CreatedTime    int64    `json:"createdTime"`
	ModifiedTime   int64    `json:"modifiedTime"`
	SortOrder      int      `json:"sortOrder"`
}

// ProjectRelatedResourcesPutRequest represents a request to update related resources.
type ProjectRelatedResourcesPutRequest struct {
	ExternalLinks   []ProjectRelatedResourcesPutRequestExternalLink   `json:"externalLinks,omitempty"`
	HermesDocuments []ProjectRelatedResourcesPutRequestHermesDocument `json:"hermesDocuments,omitempty"`
}

// ProjectRelatedResourcesPutRequestExternalLink represents an external link in a PUT request.
type ProjectRelatedResourcesPutRequestExternalLink struct {
	Name      string `json:"name"`
	URL       string `json:"url"`
	SortOrder int    `json:"sortOrder"`
}

// ProjectRelatedResourcesPutRequestHermesDocument represents a Hermes document in a PUT request.
type ProjectRelatedResourcesPutRequestHermesDocument struct {
	FileID    string `json:"FileID"`
	SortOrder int    `json:"sortOrder"`
}

//nolint:gocognit,gocyclo // Small REST multiplexer with method-specific resource branches.
func projectsResourceRelatedResourcesHandler(
	srv server.Server,
	w http.ResponseWriter,
	r *http.Request,
	projectID uint,
) {
	logArgs := []any{
		"path", r.URL.Path,
		"project_id", projectID,
	}

	// Authorize request.
	userEmail := pkgauth.MustGetUserEmail(r.Context())
	if userEmail == "" {
		srv.Logger.Error("user email not found in request context", logArgs...)
		http.Error(
			w, "No authorization information for request", http.StatusUnauthorized)
		return
	}

	switch r.Method {
	case httpMethodGet:
		logArgs = append(logArgs, "method", r.Method)

		// Get project's typed related resources.
		proj := models.Project{
			Model: gorm.Model{
				ID: projectID,
			},
		}
		elrrs, hdrrs, err := proj.GetRelatedResources(srv.DB)
		if err != nil {
			srv.Logger.Error("error getting related resources",
				append([]interface{}{
					"error", err,
				}, logArgs...)...)
			http.Error(
				w, "Error processing request", http.StatusInternalServerError)
			return
		}

		// Build response.
		resp := ProjectRelatedResourcesGetResponse{
			ExternalLinks:   []ProjectRelatedResourcesGetResponseExternalLink{},
			HermesDocuments: []ProjectRelatedResourcesGetResponseHermesDocument{},
		}
		// Add external link related resources.
		for i := range elrrs {
			resp.ExternalLinks = append(resp.ExternalLinks,
				ProjectRelatedResourcesGetResponseExternalLink{
					Name:      elrrs[i].Name,
					URL:       elrrs[i].URL,
					SortOrder: elrrs[i].RelatedResource.SortOrder,
				})
		}
		// Add Hermes document related resources.
		for i := range hdrrs {
			entryLogArgs := append([]any(nil), logArgs...)
			entryLogArgs = append(entryLogArgs, "document_id", hdrrs[i].Document.GoogleFileID)
			// Convert database model to a document. We don't need document review
			// data for this endpoint.
			doc, err := document.NewFromDatabaseModel(
				hdrrs[i].Document, models.DocumentReviews{}, models.DocumentGroupReviews{})
			if err != nil {
				srv.Logger.Error("error converting database model to document type",
					append([]interface{}{
						"error", err,
					}, entryLogArgs...)...)
				http.Error(
					w, "Error processing request", http.StatusInternalServerError)
				return
			}

			resp.HermesDocuments = append(
				resp.HermesDocuments,
				ProjectRelatedResourcesGetResponseHermesDocument{
					FileID:         doc.ObjectID,
					Title:          doc.Title,
					CreatedTime:    doc.CreatedTime,
					DocumentType:   doc.DocType,
					DocumentNumber: doc.DocNumber,
					ModifiedTime:   doc.ModifiedTime,
					Owners:         doc.Owners,
					Product:        doc.Product,
					SortOrder:      hdrrs[i].RelatedResource.SortOrder,
					Status:         doc.Status,
					Summary:        doc.Summary,
				})
		}

		// Write response.
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		enc := json.NewEncoder(w)
		err = enc.Encode(resp)
		if err != nil {
			srv.Logger.Error("error encoding response",
				append([]interface{}{
					"error", err,
				}, logArgs...)...)
			http.Error(
				w, "Error processing request", http.StatusInternalServerError)
			return
		}

	case httpMethodPost, httpMethodPut:
		logArgs = append(logArgs, "method", r.Method)

		// Decode request.
		var req ProjectRelatedResourcesPutRequest
		if err := decodeRequest(r, &req); err != nil {
			srv.Logger.Error("error decoding request",
				append([]interface{}{
					"error", err,
				}, logArgs...)...)
			http.Error(
				w, "Bad request", http.StatusBadRequest)
			return
		}

		// Get project.
		proj := models.Project{}
		if err := proj.Get(srv.DB, projectID); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				srv.Logger.Warn("project not found", logArgs...)
				http.Error(w, "Project not found", http.StatusNotFound)
				return
			}

			srv.Logger.Error("error getting project from database",
				append([]interface{}{
					"error", err,
				}, logArgs...)...)
			http.Error(
				w, "Error processing request", http.StatusInternalServerError)
			return
		}

		// Build external link related resources for database model.
		elrrs := []models.ProjectRelatedResourceExternalLink{}
		for _, elrr := range req.ExternalLinks {
			elrrs = append(elrrs, models.ProjectRelatedResourceExternalLink{
				RelatedResource: models.ProjectRelatedResource{
					ProjectID: projectID,
					SortOrder: elrr.SortOrder,
				},
				Name: elrr.Name,
				URL:  elrr.URL,
			})
		}

		// Build Hermes document related resources for database model.
		hdrrs := []models.ProjectRelatedResourceHermesDocument{}
		for _, hdrr := range req.HermesDocuments {
			hdrrs = append(hdrrs, models.ProjectRelatedResourceHermesDocument{
				RelatedResource: models.ProjectRelatedResource{
					ProjectID: projectID,
					SortOrder: hdrr.SortOrder,
				},
				Document: models.DocumentByFileID(hdrr.FileID),
			})
		}

		// Replace related resources for project.
		if err := proj.ReplaceRelatedResources(srv.DB, elrrs, hdrrs); err != nil {
			srv.Logger.Error("error replacing related resources for document",
				append([]interface{}{
					"error", err,
				}, logArgs...)...)
			http.Error(
				w, "Error processing request", http.StatusInternalServerError)
			return
		}

		// Log success.
		reqJSON, err := json.Marshal(req)
		if err != nil {
			srv.Logger.Warn("error marshaling request to JSON",
				append([]interface{}{
					"error", err,
				}, logArgs...)...)
		}
		srv.Logger.Info("replaced related resources for project",
			append([]interface{}{
				"request", string(reqJSON),
				"user", userEmail,
			}, logArgs...)...)

	default:
		WriteMethodNotAllowed(w, r, httpMethodGet, httpMethodPost, httpMethodPut)
		return
	}
}
