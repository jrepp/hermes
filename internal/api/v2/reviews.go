package api

import (
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/hashicorp/go-multierror"
	"google.golang.org/api/drive/v3"
	"gorm.io/gorm"

	"github.com/hashicorp-forge/hermes/internal/config"
	"github.com/hashicorp-forge/hermes/internal/email"
	"github.com/hashicorp-forge/hermes/internal/server"
	"github.com/hashicorp-forge/hermes/pkg/document"
	hcd "github.com/hashicorp-forge/hermes/pkg/hashicorpdocs"
	"github.com/hashicorp-forge/hermes/pkg/models"
	"github.com/hashicorp-forge/hermes/pkg/workspace"
)

// ReviewsHandler returns an HTTP handler for document review operations.
//
//nolint:gocognit,gocyclo // Legacy review workflow handler with many coordinated side effects.
func ReviewsHandler(srv server.Server) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Bind to the site serving this request before touching the
		// database; srv as constructed holds the process-wide default.
		srv, ok := srv.ForRequest(w, r)
		if !ok {
			return
		}

		switch r.Method {
		case "POST":
			// Validate request.
			docID, err := parseResourceIDFromURL(r.URL.Path, "reviews")
			if err != nil {
				srv.Logger.Error("error parsing document ID from reviews path",
					"error", err,
					"method", r.Method,
					"path", r.URL.Path,
				)
				http.Error(w, "Document ID not found", http.StatusNotFound)
				return
			}

			// Check if document is locked (Google Docs specific).
			googleProvider := getGoogleDocsProvider(srv.WorkspaceProvider)
			if googleProvider == nil {
				srv.Logger.Warn("IsLocked skipped - not using Google Workspace", "doc_id", docID)
			} else {
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

			// Begin database transaction.
			var revertFuncs []func() error
			tx := srv.DB.Begin()
			revertFuncs = append(revertFuncs, func() error {
				// Rollback database transaction.
				if err = tx.Rollback().Error; err != nil {
					return fmt.Errorf("error rolling back database transaction: %w", err)
				}

				return nil
			})

			// Get document from database.
			model := models.Document{
				GoogleFileID: docID,
			}
			if err := model.Get(tx); err != nil {
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
			if err := reviews.Find(tx, models.DocumentReview{
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
				http.Error(w, "Error accessing document",
					http.StatusInternalServerError)
				return
			}

			// Validate document status.
			if doc.Status != "WIP" {
				srv.Logger.Warn("document is not in WIP status",
					"doc_id", docID,
					"method", r.Method,
					"path", r.URL.Path,
				)
				http.Error(w,
					"Cannot create review for a document that is not in WIP status",
					http.StatusUnprocessableEntity)
				return
			}

			// Get latest product number.
			latestNum, err := models.GetLatestProductNumber(
				tx, doc.DocType, doc.Product)
			if err != nil {
				srv.Logger.Error("error getting product document number",
					"error", err,
					"doc_id", docID,
					"method", r.Method,
					"path", r.URL.Path,
				)
				http.Error(w, "Error creating review",
					http.StatusInternalServerError)
				return
			}

			// Get product from database so we can get the product abbreviation.
			product := models.Product{
				Name: doc.Product,
			}
			if err := product.Get(tx); err != nil {
				srv.Logger.Error("error getting product",
					"error", err,
					"doc_id", docID,
					"method", r.Method,
					"path", r.URL.Path,
				)
				http.Error(w, "Error creating review",
					http.StatusInternalServerError)
				return
			}

			// Reset the document creation time to the current time of publish.
			now := time.Now()
			doc.Created = now.Format("Jan 2, 2006")
			doc.CreatedTime = now.Unix()

			// Set the document number.
			nextDocNum := latestNum + 1
			doc.DocNumber = fmt.Sprintf("%s-%03d",
				product.Abbreviation,
				nextDocNum)

			// Change document status to "In-Review".
			doc.Status = "In-Review"

			// Replace the doc header.
			err = doc.ReplaceHeader(srv.Config.BaseURL, false, getGoogleDocsUpdater(srv.WorkspaceProvider))
			revertFuncs = append(revertFuncs, func() error {
				// Change back document number to "ABC-???" and status to "WIP".
				doc.DocNumber = fmt.Sprintf("%s-???", product.Abbreviation)
				doc.Status = "WIP"

				if err = doc.ReplaceHeader(
					srv.Config.BaseURL, false, getGoogleDocsUpdater(srv.WorkspaceProvider),
				); err != nil {
					return fmt.Errorf("error replacing doc header: %w", err)
				}

				return nil
			})
			if err != nil {
				srv.Logger.Error("error replacing doc header",
					"error", err, "doc_id", docID)
				http.Error(w, "Error creating review",
					http.StatusInternalServerError)

				if err := revertReviewsPost(revertFuncs); err != nil {
					srv.Logger.Error("error reverting review creation",
						"error", err,
						"doc_id", docID,
						"method", r.Method,
						"path", r.URL.Path)
				}
				return
			}
			srv.Logger.Info("doc header replaced",
				"doc_id", docID,
				"method", r.Method,
				"path", r.URL.Path,
			)

			// Get document metadata from workspace provider to get the latest modified time.
			providerID := fmt.Sprintf("google:%s", docID)
			docMeta, err := srv.WorkspaceProvider.GetDocument(r.Context(), providerID)
			if err != nil {
				srv.Logger.Error("error getting document metadata from workspace",
					"error", err,
					"path", r.URL.Path,
					"method", r.Method,
					"doc_id", docID,
				)
				http.Error(w, "Error creating review", http.StatusInternalServerError)
				if err := revertReviewsPost(revertFuncs); err != nil {
					srv.Logger.Error("error reverting review creation",
						"error", err,
						"doc_id", docID,
						"method", r.Method,
						"path", r.URL.Path)
				}
				return
			}

			// Use modified time directly (no parsing needed - RFC-084 returns time.Time)
			modifiedTime := docMeta.ModifiedTime
			if err != nil {
				srv.Logger.Error("error parsing modified time",
					"error", err,
					"path", r.URL.Path,
					"method", r.Method,
					"doc_id", docID,
				)
				http.Error(w, "Error creating review", http.StatusInternalServerError)
				if err := revertReviewsPost(revertFuncs); err != nil {
					srv.Logger.Error("error reverting review creation",
						"error", err,
						"doc_id", docID,
						"method", r.Method,
						"path", r.URL.Path)
				}
				return
			}
			doc.ModifiedTime = modifiedTime.Unix()

			// Get latest revision using RFC-084.
			providerID = fmt.Sprintf("google:%s", docID)
			latestRev, err := getLatestRevisionRFC084(r.Context(), providerID, srv.WorkspaceProvider)
			if err != nil {
				srv.Logger.Error("error getting latest revision",
					"error", err,
					"method", r.Method,
					"path", r.URL.Path,
					"doc_id", docID)
				http.Error(w, "Error creating review",
					http.StatusInternalServerError)

				if err := revertReviewsPost(revertFuncs); err != nil {
					srv.Logger.Error("error reverting review creation",
						"error", err,
						"doc_id", docID,
						"method", r.Method,
						"path", r.URL.Path)
				}
				return
			}

			// Mark latest revision to be kept forever.
			err = srv.WorkspaceProvider.KeepRevisionForever(r.Context(), providerID, latestRev.RevisionID)
			// Note: RFC-084 does not support undoing KeepRevisionForever, so we don't add a revert function.
			// In case of errors, the revision will remain kept forever.
			if err != nil {
				srv.Logger.Error("error marking revision to keep forever",
					"error", err,
					"method", r.Method,
					"path", r.URL.Path,
					"doc_id", docID,
					"rev_id", latestRev.RevisionID)
				http.Error(w, "Error creating review",
					http.StatusInternalServerError)

				if err := revertReviewsPost(revertFuncs); err != nil {
					srv.Logger.Error("error reverting review creation",
						"error", err,
						"doc_id", docID,
						"method", r.Method,
						"path", r.URL.Path)
				}
				return
			}
			srv.Logger.Info("doc revision set to be kept forever",
				"doc_id", docID,
				"method", r.Method,
				"path", r.URL.Path,
			)

			// Record file revision in the Algolia document object.
			revisionName := "Requested review"
			doc.SetFileRevision(latestRev.RevisionID, revisionName)

			// Create file revision in the database.
			fr := models.DocumentFileRevision{
				Document: models.Document{
					GoogleFileID: docID,
				},
				GoogleDriveFileRevisionID: latestRev.RevisionID,
				FileRevisionID:            latestRev.RevisionID,
				Name:                      revisionName,
			}
			if err := fr.Create(tx); err != nil {
				srv.Logger.Error("error creating document file revision",
					"error", err,
					"method", r.Method,
					"path", r.URL.Path,
					"doc_id", docID,
					"rev_id", latestRev.RevisionID)
				http.Error(w, "Error creating review",
					http.StatusInternalServerError)

				if err := revertReviewsPost(revertFuncs); err != nil {
					srv.Logger.Error("error reverting review creation",
						"error", err,
						"doc_id", docID,
						"method", r.Method,
						"path", r.URL.Path)
				}
				return
			}

			// Move document to published docs location.
			providerID = fmt.Sprintf("google:%s", docID)
			_, err = srv.WorkspaceProvider.MoveDocument(
				r.Context(), providerID, srv.Config.GoogleWorkspace.DocsFolder)
			revertFuncs = append(revertFuncs, func() error {
				// Move document back to drafts folder in Google Drive.
				if _, err := srv.WorkspaceProvider.MoveDocument(
					r.Context(), fmt.Sprintf("google:%s", doc.ObjectID), srv.Config.GoogleWorkspace.DraftsFolder); err != nil {

					return fmt.Errorf("error moving doc back to drafts folder: %w", err)

				}

				return nil
			})
			if err != nil {
				srv.Logger.Error("error moving file to docs folder",
					"error", err,
					"doc_id", docID,
					"method", r.Method,
					"path", r.URL.Path)
				http.Error(w, "Error creating review",
					http.StatusInternalServerError)

				if err := revertReviewsPost(revertFuncs); err != nil {
					srv.Logger.Error("error reverting review creation",
						"error", err,
						"doc_id", docID,
						"method", r.Method,
						"path", r.URL.Path)
				}
				return
			}
			srv.Logger.Info("doc moved to published document folder",
				"doc_id", docID,
				"method", r.Method,
				"path", r.URL.Path,
			)

			// Create shortcut in hierarchical folder structure.
			_, err = createShortcut(srv.Config, *doc, getCompatProvider(srv.WorkspaceProvider))
			if err != nil {
				srv.Logger.Error("error creating shortcut",
					"error", err,
					"doc_id", docID,
					"method", r.Method,
					"path", r.URL.Path)
				http.Error(w, "Error creating review",
					http.StatusInternalServerError)

				if err := revertReviewsPost(revertFuncs); err != nil {
					srv.Logger.Error("error reverting review creation",
						"error", err,
						"doc_id", docID,
						"method", r.Method,
						"path", r.URL.Path)
				}
				return
			}
			srv.Logger.Info("doc shortcut created",
				"doc_id", docID,
				"method", r.Method,
				"path", r.URL.Path,
			)

			// Update document in the database.
			d := models.Document{
				GoogleFileID: docID,
			}
			if err := d.Get(tx); err != nil {
				srv.Logger.Error("error getting document in database",
					"error", err,
					"doc_id", docID,
					"method", r.Method,
					"path", r.URL.Path)
				http.Error(w, "Error creating review",
					http.StatusInternalServerError)

				if err := revertReviewsPost(revertFuncs); err != nil {
					srv.Logger.Error("error reverting review creation",
						"error", err,
						"doc_id", docID,
						"method", r.Method,
						"path", r.URL.Path)
				}
				return
			}
			d.DocumentCreatedAt = now // Reset to document published time.
			d.Status = models.InReviewDocumentStatus
			d.DocumentNumber = nextDocNum
			d.DocumentModifiedAt = modifiedTime
			if err := d.Upsert(tx); err != nil {
				srv.Logger.Error("error upserting document in database",
					"error", err,
					"doc_id", docID,
					"method", r.Method,
					"path", r.URL.Path)
				http.Error(w, "Error creating review",
					http.StatusInternalServerError)

				if err := revertReviewsPost(revertFuncs); err != nil {
					srv.Logger.Error("error reverting review creation",
						"error", err,
						"doc_id", docID,
						"method", r.Method,
						"path", r.URL.Path)
				}
				return
			}

			// Create slice of all approvers consisting of individuals and groups.
			allApprovers := append(append([]string{}, doc.Approvers...), doc.ApproverGroups...)

			// Give document approvers and approver groups edit access to the
			// document.
			for _, a := range allApprovers {
				if err := srv.WorkspaceProvider.ShareDocument(r.Context(), fmt.Sprintf("google:%s", docID), a, "writer"); err != nil {
					srv.Logger.Error("error sharing file with approver",
						"error", err,
						"doc_id", docID,
						"method", r.Method,
						"path", r.URL.Path,
						"approver", a)
					http.Error(w, "Error creating review",
						http.StatusInternalServerError)

					if err := revertReviewsPost(revertFuncs); err != nil {
						srv.Logger.Error("error reverting review creation",
							"error", err,
							"doc_id", docID,
							"method", r.Method,
							"path", r.URL.Path)
					}
					return
				}
			}

			// Get document URL.
			docURL, err := getDocumentURL(srv.Config.BaseURL, docID)
			if err != nil {
				srv.Logger.Error("error getting document URL",
					"error", err,
					"doc_id", docID,
					"method", r.Method,
					"path", r.URL.Path,
				)
				http.Error(w, "Error creating review",
					http.StatusInternalServerError)
				if err := revertReviewsPost(revertFuncs); err != nil {
					srv.Logger.Error("error reverting review creation",
						"error", err,
						"doc_id", docID,
						"method", r.Method,
						"path", r.URL.Path)
				}
				return
			}

			// Send emails to approvers, if enabled.
			if srv.Config.Email != nil && srv.Config.Email.Enabled {
				if len(allApprovers) > 0 {
					// TODO: use an asynchronous method for sending emails because we
					// can't currently recover gracefully from a failure here.
					for _, approverEmail := range allApprovers {
						err := email.SendReviewRequestedEmail(
							email.ReviewRequestedEmailData{
								BaseURL:           srv.Config.BaseURL,
								DocumentOwner:     doc.Owners[0],
								DocumentShortName: doc.DocNumber,
								DocumentType:      doc.DocType,
								DocumentTitle:     doc.Title,
								DocumentStatus:    doc.Status,
								DocumentURL:       docURL,
								Product:           doc.Product,
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
							http.Error(w, "Error creating review",
								http.StatusInternalServerError)
							if err := revertReviewsPost(revertFuncs); err != nil {
								srv.Logger.Error("error reverting review creation",
									"error", err,
									"doc_id", docID,
									"method", r.Method,
									"path", r.URL.Path)
							}
							return
						}
						srv.Logger.Info("doc approver email sent",
							"doc_id", docID,
							"method", r.Method,
							"path", r.URL.Path,
						)
					}
				}
			}

			if err := enqueueReviewCreatedSearchOutbox(tx, doc); err != nil {
				srv.Logger.Error("error enqueueing review search outbox events",
					"error", err,
					"doc_id", docID,
					"method", r.Method,
					"path", r.URL.Path,
				)
				http.Error(w, "Error creating review",
					http.StatusInternalServerError)
				if err := revertReviewsPost(revertFuncs); err != nil {
					srv.Logger.Error("error reverting review creation",
						"error", err,
						"doc_id", docID,
						"method", r.Method,
						"path", r.URL.Path)
				}
				return
			}

			// Commit the database transaction.
			if err := tx.Commit().Error; err != nil {
				srv.Logger.Error("error committing database transaction",
					"error", err,
					"doc_id", docID,
					"method", r.Method,
					"path", r.URL.Path,
				)
				http.Error(w, "Error creating review",
					http.StatusInternalServerError)

				if err := revertReviewsPost(revertFuncs); err != nil {
					srv.Logger.Error("error reverting review creation",
						"error", err,
						"doc_id", docID,
						"method", r.Method,
						"path", r.URL.Path)
				}
				return
			}

			// Write response.
			w.WriteHeader(http.StatusOK)

			// Log success.
			srv.Logger.Info("review created",
				"doc_id", docID,
				"method", r.Method,
				"path", r.URL.Path,
			)

			// Request post-processing.
			go func() {
				// Send emails to product subscribers, if enabled.
				if srv.Config.Email != nil && srv.Config.Email.Enabled {
					p := models.Product{
						Name: doc.Product,
					}
					if err := p.Get(srv.DB); err != nil {
						srv.Logger.Error("error getting product from database",
							"error", err,
							"doc_id", docID,
							"method", r.Method,
							"path", r.URL.Path,
						)
						return
					}

					if len(p.UserSubscribers) > 0 {
						// TODO: use an asynchronous method for sending emails because we
						// can't currently recover gracefully from a failure here.
						for i := range p.UserSubscribers {
							err := email.SendSubscriberDocumentPublishedEmail(
								email.SubscriberDocumentPublishedEmailData{
									BaseURL:           srv.Config.BaseURL,
									DocumentOwner:     doc.Owners[0],
									DocumentShortName: doc.DocNumber,
									DocumentTitle:     doc.Title,
									DocumentType:      doc.DocType,
									DocumentURL:       docURL,
									Product:           doc.Product,
								},
								[]string{p.UserSubscribers[i].EmailAddress},
								srv.Config.Email.FromAddress,
								getEmailSender(srv.WorkspaceProvider),
							)
							if err != nil {
								srv.Logger.Error("error sending subscriber email",
									"error", err,
									"method", r.Method,
									"path", r.URL.Path,
									"doc_id", docID,
								)
							} else {
								srv.Logger.Info("doc subscriber email sent",
									"doc_id", docID,
									"method", r.Method,
									"path", r.URL.Path,
									"product", doc.Product,
								)
							}
						}
					}
				}
			}()

		default:
			WriteMethodNotAllowed(w, r, "POST")
			return
		}
	})
}

func enqueueReviewCreatedSearchOutbox(tx *gorm.DB, doc *document.Document) error {
	payload, err := documentSearchPayload(doc)
	if err != nil {
		return err
	}

	if err := models.EnqueueSearchOutboxEventWithSequence(tx, &models.SearchOutboxEvent{
		EventType:     models.SearchEventDocumentPublished,
		AggregateID:   doc.ObjectID,
		AggregateType: models.SearchAggregateDocument,
		IndexName:     models.SearchIndexDocuments,
		Operation:     models.SearchOutboxOperationUpsert,
		Payload:       payload,
	}); err != nil {
		return fmt.Errorf("enqueue document published search event: %w", err)
	}

	if err := models.EnqueueSearchOutboxEventWithSequence(tx, &models.SearchOutboxEvent{
		EventType:     models.SearchEventDraftPublished,
		AggregateID:   doc.ObjectID,
		AggregateType: models.SearchAggregateDraft,
		IndexName:     models.SearchIndexDrafts,
		Operation:     models.SearchOutboxOperationDelete,
	}); err != nil {
		return fmt.Errorf("enqueue draft published search event: %w", err)
	}

	return enqueueDocumentRedirectSearchOutbox(tx, doc)
}

func enqueueDocumentRedirectSearchOutbox(tx *gorm.DB, doc *document.Document) error {
	if doc.DocNumber == "" || doc.DocType == "" {
		return nil
	}

	objectID := fmt.Sprintf("/%s/%s", strings.ToLower(doc.DocType), strings.ToLower(doc.DocNumber))
	return models.EnqueueSearchOutboxEventWithSequence(tx, &models.SearchOutboxEvent{
		EventType:     models.SearchEventLinkCreated,
		AggregateID:   objectID,
		AggregateType: models.SearchAggregateLink,
		IndexName:     models.SearchIndexLinks,
		Operation:     models.SearchOutboxOperationUpsert,
		Payload: map[string]any{
			"documentID": doc.ObjectID,
			"objectID":   objectID,
		},
	})
}

// createShortcut creates a shortcut in the hierarchical folder structure
// ("Shortcuts Folder/RFC/MyProduct/") under docsFolder.
func createShortcut(
	cfg *config.Config,
	doc document.Document,
	provider workspace.Provider) (shortcut *drive.File, retErr error) {

	// Get folder for doc type.
	docTypeFolderID, err := provider.GetSubfolder(
		cfg.GoogleWorkspace.ShortcutsFolder, doc.DocType)
	if err != nil {
		return nil, fmt.Errorf("error getting doc type subfolder: %w", err)
	}

	// Doc type folder wasn't found, so create it.
	if docTypeFolderID == "" {
		var docTypeFolder *drive.File
		docTypeFolder, err = provider.CreateFolder(
			doc.DocType, cfg.GoogleWorkspace.ShortcutsFolder)
		if err != nil {
			return nil, fmt.Errorf("error creating doc type subfolder: %w", err)
		}
		docTypeFolderID = docTypeFolder.Id
	}

	// Get folder for doc type + product.
	productFolderID, err := provider.GetSubfolder(docTypeFolderID, doc.Product)
	if err != nil {
		return nil, fmt.Errorf("error getting product subfolder: %w", err)
	}

	// Product folder wasn't found, so create it.
	if productFolderID == "" {
		var productFolder *drive.File
		productFolder, err = provider.CreateFolder(
			doc.Product, docTypeFolderID)
		if err != nil {
			return nil, fmt.Errorf("error creating product subfolder: %w", err)
		}
		productFolderID = productFolder.Id
	}

	// Create shortcut.
	if shortcut, err = provider.CreateShortcut(
		doc.ObjectID,
		productFolderID); err != nil {

		return nil, fmt.Errorf("error creating shortcut: %w", err)
	}

	return
}

// getDocumentURL returns a Hermes document URL.
func getDocumentURL(baseURL, docID string) (string, error) {
	docURL, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("error parsing base URL: %w", err)
	}

	docURL.Path = path.Join(docURL.Path, "document", docID)
	docURLString := docURL.String()
	docURLString = strings.TrimRight(docURLString, "/")

	return docURLString, nil
}

// revertReviewsPost attempts to revert the actions that occur when a review is
// created. This is to be used in the case of an error during the review-
// creation process.
func revertReviewsPost(funcs []func() error) error {
	var result *multierror.Error

	for _, fn := range funcs {
		if err := fn(); err != nil {
			result = multierror.Append(result, err)
		}
	}

	return result.ErrorOrNil()
}
