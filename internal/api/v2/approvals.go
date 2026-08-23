package api

import (
	"fmt"
	"net/http"

	pkgauth "github.com/hashicorp-forge/hermes/pkg/auth"

	"gorm.io/gorm"

	"github.com/hashicorp-forge/hermes/internal/email"
	"github.com/hashicorp-forge/hermes/internal/helpers"
	"github.com/hashicorp-forge/hermes/internal/server"
	"github.com/hashicorp-forge/hermes/pkg/document"
	hcd "github.com/hashicorp-forge/hermes/pkg/hashicorpdocs"
	"github.com/hashicorp-forge/hermes/pkg/models"
)

const (
	// HTTP methods
	httpMethodGet    = "GET"
	httpMethodPost   = "POST"
	httpMethodPatch  = "PATCH"
	httpMethodPut    = "PUT"
	httpMethodDelete = "DELETE"

	// Document statuses
	docStatusInReview = "In-Review"
	docStatusApproved = "Approved"
	docStatusObsolete = "Obsolete"
	docStatusWIP      = "WIP"

	// Custom field types
	fieldTypeString = "STRING"
	fieldTypePeople = "PEOPLE"

	// Workspace provider types
	workspaceProviderLocal = "local"

	// Status values
	statusActive = "active"
)

// ApprovalsHandler returns an HTTP handler for document approval operations.
//
//nolint:gocognit,gocyclo // Legacy HTTP entrypoint; splitting further is high-churn and behavior-sensitive.
func ApprovalsHandler(srv server.Server) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Bind to the site serving this request before touching the
		// database; srv as constructed holds the process-wide default.
		srv, ok := srv.ForRequest(w, r)
		if !ok {
			return
		}

		// Validate request.
		docID, err := parseResourceIDFromURL(r.URL.Path, "approvals")
		if err != nil {
			srv.Logger.Error("error parsing document ID",
				"error", err,
				"method", r.Method,
				"path", r.URL.Path,
			)
			http.Error(w, "Document ID not found", http.StatusNotFound)
			return
		}

		// Get document from database.
		model := srv.NewDocumentByFileID(docID)
		if err := model.Get(srv.DB); err != nil {
			srv.Logger.Error("error getting document from database",
				"error", err,
				"path", r.URL.Path,
				"method", r.Method,
				"doc_id", docID,
			)
			http.Error(w, "Error accessing document",
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
			http.Error(w, "Error accessing document",
				http.StatusInternalServerError)
			return
		}

		userEmail := pkgauth.MustGetUserEmail(r.Context())

		switch r.Method {
		case httpMethodDelete:
			// Authorize request.
			if doc.Status != docStatusInReview {
				http.Error(w,
					"Can only request changes of documents in the \"In-Review\" status",
					http.StatusBadRequest)
				return
			}
			if !contains(doc.Approvers, userEmail) {
				srv.Logger.Warn("unauthorized changes request: user not an approver",
					"method", r.Method,
					"path", r.URL.Path,
					"doc_id", docID,
					"user_email", userEmail)
				http.Error(w, "Not authorized as a document approver",
					http.StatusUnauthorized)
				return
			}
			if contains(doc.ChangesRequestedBy, userEmail) {
				srv.Logger.Warn("changes already requested by user",
					"method", r.Method,
					"path", r.URL.Path,
					"doc_id", docID,
					"user_email", userEmail)
				http.Error(w, "Document already has changes requested by user",
					http.StatusBadRequest)
				return
			}

			// Check if document is locked (Google Docs specific).
			// Extract Google provider for Google-specific operations
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

			// Add email to slice of users who have requested changes of the document.
			doc.ChangesRequestedBy = append(doc.ChangesRequestedBy, userEmail)

			// If user had previously approved, delete email from slice of users who
			// have approved the document.
			var newApprovedBy []string
			for _, a := range doc.ApprovedBy {
				if a != userEmail {
					newApprovedBy = append(newApprovedBy, a)
				}
			}
			doc.ApprovedBy = newApprovedBy

			// Get latest file revision using RFC-084 interface.
			// Note: We need to construct a providerID from the docID
			providerID := fmt.Sprintf("google:%s", docID)
			latestRev, err := getLatestRevisionRFC084(r.Context(), providerID, srv.WorkspaceProvider)
			if err != nil {
				srv.Logger.Error("error getting latest revision",
					"error", err,
					"method", r.Method,
					"path", r.URL.Path,
					"doc_id", docID)
				http.Error(w, "Error requesting changes of document",
					http.StatusInternalServerError)
				return
			}

			// Mark latest revision to be kept forever.
			err = srv.WorkspaceProvider.KeepRevisionForever(r.Context(), providerID, latestRev.RevisionID)
			if err != nil {
				srv.Logger.Error("error marking revision to keep forever",
					"error", err,
					"method", r.Method,
					"path", r.URL.Path,
					"doc_id", docID,
					"rev_id", latestRev.RevisionID)
				http.Error(w, "Error updating document status",
					http.StatusInternalServerError)
				return
			}

			// Record file revision in the Algolia document object.
			revisionName := fmt.Sprintf("Changes requested by %s", userEmail)
			doc.SetFileRevision(latestRev.RevisionID, revisionName)

			// Build file revision for the database.
			fr := models.DocumentFileRevision{
				Document: models.Document{
					GoogleFileID: docID,
				},
				GoogleDriveFileRevisionID: latestRev.RevisionID,
				FileRevisionID:            latestRev.RevisionID,
				Name:                      revisionName,
			}

			// Update review state and enqueue search projection atomically.
			if err := updateReviewStateWithSearchOutbox(srv.DB, &fr, doc); err != nil {
				srv.Logger.Error("error updating review state",
					"error", err,
					"method", r.Method,
					"path", r.URL.Path,
					"doc_id", docID,
					"rev_id", latestRev.RevisionID)
				http.Error(w, "Error updating document status",
					http.StatusInternalServerError)
				return
			}

			// Replace the doc header (Google Docs specific).
			// Extract Google provider for Google-specific operations
			googleUpdater := getGoogleDocsUpdater(srv.WorkspaceProvider)
			if googleUpdater == nil {
				srv.Logger.Warn("ReplaceHeader skipped - not using Google Workspace",
					"doc_id", docID)
			} else {
				if err := doc.ReplaceHeader(
					srv.Config.BaseURL, false, googleUpdater,
				); err != nil {
					srv.Logger.Error("error replacing doc header",
						"error", err,
						"doc_id", docID,
						"method", r.Method,
						"path", r.URL.Path,
					)
					http.Error(w, "Error updating document status",
						http.StatusInternalServerError)
					return
				}
			}

			// Write response.
			w.WriteHeader(http.StatusOK)

			// Log success.
			srv.Logger.Info("changes requested successfully",
				"doc_id", docID,
				"method", r.Method,
				"path", r.URL.Path,
			)

		case "OPTIONS":
			// Document is not in review or approved status.
			if doc.Status != docStatusInReview && doc.Status != docStatusApproved {
				w.Header().Set("Allowed", "")
				return
			}

			// Document already approved by user.
			if contains(doc.ApprovedBy, userEmail) {
				w.Header().Set("Allowed", "")
				return
			}

			// User is not an approver or in an approver group.
			inApproverGroup, err := isUserInGroups(
				r.Context(), userEmail, doc.ApproverGroups, srv.WorkspaceProvider)
			if err != nil {
				srv.Logger.Error("error calculating if user is in an approver group",
					"error", err,
					"method", r.Method,
					"path", r.URL.Path,
					"doc_id", docID,
				)
				http.Error(w, "Error accessing document",
					http.StatusInternalServerError)
				return
			}
			if !contains(doc.Approvers, userEmail) && !inApproverGroup {
				w.Header().Set("Allowed", "")
				return
			}

			// User can approve.
			w.Header().Set("Allowed", "POST")
			return

		case httpMethodPost:
			// Authorize request.
			if doc.Status != docStatusInReview && doc.Status != docStatusApproved {
				http.Error(w,
					`Document status must be "In-Review" or "Approved" to approve`,
					http.StatusBadRequest)
				return
			}
			if contains(doc.ApprovedBy, userEmail) {
				srv.Logger.Warn("document already approved by user",
					"method", r.Method,
					"path", r.URL.Path,
					"doc_id", docID,
					"user_email", userEmail)
				http.Error(w,
					"Document already approved by user",
					http.StatusBadRequest)
				return
			}
			inApproverGroup, err := isUserInGroups(
				r.Context(), userEmail, doc.ApproverGroups, srv.WorkspaceProvider)
			if err != nil {
				srv.Logger.Error("error calculating if user is in an approver group",
					"error", err,
					"method", r.Method,
					"path", r.URL.Path,
					"doc_id", docID,
				)
				http.Error(w, "Error accessing document",
					http.StatusInternalServerError)
				return
			}
			if !contains(doc.Approvers, userEmail) && !inApproverGroup {
				srv.Logger.Warn("unauthorized approval attempt: user not an approver",
					"method", r.Method,
					"path", r.URL.Path,
					"doc_id", docID,
					"user_email", userEmail)
				http.Error(w,
					"Not authorized as a document approver",
					http.StatusUnauthorized)
				return
			}

			// Check if document is locked (Google Docs specific).
			// Extract Google provider for Google-specific operations
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

			// If the user is a group approver, they won't be in the approvers list.
			addApprover := false
			if !contains(doc.Approvers, userEmail) {
				addApprover = true
				doc.Approvers = append(doc.Approvers, userEmail)
			}

			// Add email to slice of users who have approved the document.
			doc.ApprovedBy = append(doc.ApprovedBy, userEmail)

			// If the user had previously requested changes, delete email from slice
			// of users who have requested changes of the document.
			var newChangesRequestedBy []string
			for _, a := range doc.ChangesRequestedBy {
				if a != userEmail {
					newChangesRequestedBy = append(newChangesRequestedBy, a)
				}
			}
			doc.ChangesRequestedBy = newChangesRequestedBy

			// Get latest file revision using RFC-084 interface.
			// Note: We need to construct a providerID from the docID
			providerID := fmt.Sprintf("google:%s", docID)
			latestRev, err := getLatestRevisionRFC084(r.Context(), providerID, srv.WorkspaceProvider)
			if err != nil {
				srv.Logger.Error("error getting latest revision",
					"error", err,
					"method", r.Method,
					"path", r.URL.Path,
					"doc_id", docID)
				http.Error(w, "Error creating review",
					http.StatusInternalServerError)
				return
			}

			// Mark latest revision to be kept forever.
			err = srv.WorkspaceProvider.KeepRevisionForever(r.Context(), providerID, latestRev.RevisionID)
			if err != nil {
				srv.Logger.Error("error marking revision to keep forever",
					"error", err,
					"method", r.Method,
					"path", r.URL.Path,
					"doc_id", docID,
					"rev_id", latestRev.RevisionID)
				http.Error(w, "Error approving document",
					http.StatusInternalServerError)
				return
			}

			// Record file revision in the Algolia document object.
			revisionName := fmt.Sprintf("Approved by %s", userEmail)
			doc.SetFileRevision(latestRev.RevisionID, revisionName)

			// Build file revision for the database.
			fr := models.DocumentFileRevision{
				Document: models.Document{
					GoogleFileID: docID,
				},
				GoogleDriveFileRevisionID: latestRev.RevisionID,
				FileRevisionID:            latestRev.RevisionID,
				Name:                      revisionName,
			}

			// Update review state and enqueue search projection atomically.
			if err := updateReviewStateWithSearchOutbox(srv.DB, &fr, doc, func(tx *gorm.DB) error {
				if !addApprover {
					return nil
				}

				model.Approvers = append(model.Approvers, &models.User{
					EmailAddress: userEmail,
				})
				if err := model.Upsert(tx); err != nil {
					return fmt.Errorf("error updating document in the database to add approver: %w", err)
				}

				return nil
			}); err != nil {
				srv.Logger.Error("error updating review state",
					"error", err,
					"method", r.Method,
					"path", r.URL.Path,
					"doc_id", docID,
					"rev_id", latestRev.RevisionID)
				http.Error(w, "Error updating document status",
					http.StatusInternalServerError)
				return
			}

			// Replace the doc header (Google Docs specific).
			// Extract Google provider for Google-specific operations
			googleUpdater := getGoogleDocsUpdater(srv.WorkspaceProvider)
			if googleUpdater == nil {
				srv.Logger.Warn("ReplaceHeader skipped - not using Google Workspace",
					"doc_id", docID)
			} else {
				err = doc.ReplaceHeader(srv.Config.BaseURL, false, googleUpdater)
				if err != nil {
					srv.Logger.Error("error replacing doc header",
						"error", err,
						"doc_id", docID,
						"method", r.Method,
						"path", r.URL.Path,
					)
					http.Error(w, "Error approving document",
						http.StatusInternalServerError)
					return
				}
			}

			// Write response.
			w.WriteHeader(http.StatusOK)

			// Log success.
			srv.Logger.Info("approval created",
				"doc_id", docID,
				"method", r.Method,
				"path", r.URL.Path,
			)

			// Log document access with Datadog ACCESS tag
			srv.Logger.Info("ACCESS",
				"user_email", userEmail,
				"doc_id", docID,
				"operation", "document_approved",
				"updated_attributes", "[approvedBy, status]",
				"mode", "published",
			)

			// Request post-processing.
			go func() {
				if srv.Config.Email == nil || !srv.Config.Email.Enabled || len(doc.Owners) == 0 {
					return
				}

				approver := email.User{EmailAddress: userEmail}
				ppl, err := srv.WorkspaceProvider.SearchPeople(r.Context(), userEmail)
				if err != nil {
					srv.Logger.Warn("error searching directory for approver",
						"error", err,
						"method", r.Method,
						"path", r.URL.Path,
						"doc_id", docID,
						"person", doc.Owners[0],
					)
				}
				if len(ppl) == 1 {
					approver.Name = ppl[0].DisplayName
				}

				docURL, err := getDocumentURL(srv.Config.BaseURL, docID)
				if err != nil {
					srv.Logger.Error("error getting document URL",
						"error", err,
						"doc_id", docID,
						"method", r.Method,
						"path", r.URL.Path,
					)
					return
				}

				if err := email.SendDocumentApprovedEmail(
					email.DocumentApprovedEmailData{
						BaseURL:          srv.Config.BaseURL,
						DocumentOwner:    doc.Owners[0],
						DocumentApprover: approver,
						DocumentNonApproverCount: len(doc.Approvers) -
							len(doc.ApprovedBy),
						DocumentShortName: doc.DocNumber,
						DocumentTitle:     doc.Title,
						DocumentType:      doc.DocType,
						DocumentStatus:    doc.Status,
						DocumentURL:       docURL,
						Product:           doc.Product,
					},
					[]string{doc.Owners[0]},
					srv.Config.Email.FromAddress,
					getEmailSender(srv.WorkspaceProvider),
				); err != nil {
					srv.Logger.Error("error sending document approved email",
						"error", err,
						"method", r.Method,
						"path", r.URL.Path,
						"doc_id", docID,
					)
				}
			}()

		default:
			WriteMethodNotAllowed(w, r, httpMethodDelete, "OPTIONS", httpMethodPost)
			return
		}
	})
}

func updateReviewStateWithSearchOutbox(
	db *gorm.DB,
	fr *models.DocumentFileRevision,
	doc *document.Document,
	extraMutations ...func(*gorm.DB) error,
) error {
	return db.Transaction(func(tx *gorm.DB) error {
		for _, mutation := range extraMutations {
			if mutation == nil {
				continue
			}
			if err := mutation(tx); err != nil {
				return err
			}
		}

		if fr != nil {
			if err := fr.Create(tx); err != nil {
				return fmt.Errorf("error creating document file revision: %w", err)
			}
		}

		if err := updateDocumentReviewsInDatabase(*doc, tx, false); err != nil {
			return fmt.Errorf("error updating document reviews in the database: %w", err)
		}

		payload, err := documentSearchPayload(doc)
		if err != nil {
			return err
		}

		return models.EnqueueSearchOutboxEventWithSequence(tx, &models.SearchOutboxEvent{
			EventType:     models.SearchEventReviewStateChanged,
			AggregateID:   doc.ObjectID,
			AggregateType: models.SearchAggregateDocument,
			IndexName:     models.SearchIndexDocuments,
			Operation:     models.SearchOutboxOperationUpsert,
			Payload:       payload,
		})
	})
}

// updateDocumentReviewsInDatabase takes a document and updates the associated
// document reviews in the database.
func updateDocumentReviewsInDatabase(doc document.Document, db *gorm.DB, useSharePoint bool) error {
	var docReviews []models.DocumentReview
	for _, a := range doc.Approvers {
		u := models.User{
			EmailAddress: a,
		}
		if helpers.StringSliceContains(doc.ApprovedBy, a) {
			docReviews = append(docReviews, models.DocumentReview{
				Document: models.NewDocumentByFileID(doc.ObjectID, useSharePoint),
				User:     u,
				Status:   models.ApprovedDocumentReviewStatus,
			})
		} else if helpers.StringSliceContains(doc.ChangesRequestedBy, a) {
			docReviews = append(docReviews, models.DocumentReview{
				Document: models.NewDocumentByFileID(doc.ObjectID, useSharePoint),
				User:     u,
				Status:   models.ChangesRequestedDocumentReviewStatus,
			})
		}
	}

	// Upsert document reviews in database.
	for i := range docReviews {
		if err := docReviews[i].Update(db); err != nil {
			return fmt.Errorf("error upserting document review: %w", err)
		}
	}

	return nil
}
