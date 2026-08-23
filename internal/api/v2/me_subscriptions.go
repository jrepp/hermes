package api

import (
	"encoding/json"
	"net/http"

	"github.com/hashicorp-forge/hermes/internal/server"
	pkgauth "github.com/hashicorp-forge/hermes/pkg/auth"
	"github.com/hashicorp-forge/hermes/pkg/models"
)

// MeSubscriptionsPostRequest represents a request to update subscriptions.
type MeSubscriptionsPostRequest struct {
	Subscriptions []string `json:"subscriptions"`
}

// MeSubscriptionsHandler returns an HTTP handler for subscription operations.
//
//nolint:gocognit,gocyclo // Small subscription endpoint with centralized branching.
func MeSubscriptionsHandler(srv server.Server) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Bind to the site serving this request before touching the
		// database; srv as constructed holds the process-wide default.
		srv, ok := srv.ForRequest(w, r)
		if !ok {
			return
		}

		errResp := func(httpCode int, userErrMsg, logErrMsg string, err error) {
			srv.Logger.Error(logErrMsg,
				"method", r.Method,
				"path", r.URL.Path,
				"error", err,
			)
			http.Error(w, userErrMsg, httpCode)
		}

		// Authorize request.
		userEmail, ok := pkgauth.GetUserEmail(r.Context())
		if !ok || userEmail == "" {
			errResp(
				http.StatusUnauthorized,
				"No authorization information for request",
				"no user email found in request context",
				nil,
			)
			return
		}

		switch r.Method {
		case httpMethodGet:
			// Find or create user.
			u := models.User{
				EmailAddress: userEmail,
			}
			if err := u.FirstOrCreate(srv.DB); err != nil {
				errResp(
					http.StatusInternalServerError,
					"Error authorizing the request",
					"error finding or creating user",
					err,
				)
				return
			}

			// Build response of product subscriptions.
			var products []string
			for i := range u.ProductSubscriptions {
				products = append(products, u.ProductSubscriptions[i].Name)
			}

			// Write response.
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			enc := json.NewEncoder(w)
			if err := enc.Encode(products); err != nil {
				errResp(
					http.StatusInternalServerError,
					"Error finding product subscriptions",
					"error encoding products to JSON",
					err,
				)
				return
			}

		case httpMethodPost:
			// Decode request.
			var req MeSubscriptionsPostRequest
			if err := decodeRequest(r, &req); err != nil {
				errResp(
					http.StatusBadRequest,
					"Bad request",
					"error decoding request",
					err,
				)
				return
			}

			// Find or create user.
			u := models.User{
				EmailAddress: userEmail,
			}
			if err := u.FirstOrCreate(srv.DB); err != nil {
				errResp(
					http.StatusInternalServerError,
					"Error authorizing the request",
					"error finding or creating user",
					err,
				)
				return
			}

			// Build user product subscriptions.
			var subs []models.Product
			for _, p := range req.Subscriptions {
				subs = append(subs, models.Product{Name: p})
			}
			u.ProductSubscriptions = subs

			// Upsert user.
			if err := u.Upsert(srv.DB); err != nil {
				errResp(
					http.StatusInternalServerError,
					"Error updating user subscriptions",
					"error upserting user",
					err,
				)
				return
			}

			// Write response.
			w.WriteHeader(http.StatusOK)

		default:
			WriteMethodNotAllowed(w, r, httpMethodGet, httpMethodPost)
			return
		}
	})
}
