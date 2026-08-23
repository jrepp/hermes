package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/hashicorp-forge/hermes/internal/server"
	pkgauth "github.com/hashicorp-forge/hermes/pkg/auth"
)

type MeGetResponse struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	Name          string `json:"name"`
	GivenName     string `json:"given_name"`
	FamilyName    string `json:"family_name"`
	Picture       string `json:"picture"`
	Locale        string `json:"locale,omitempty"`
	HD            string `json:"hd,omitempty"`
	VerifiedEmail bool   `json:"verified_email"`
}

// MeHandler returns an HTTP handler for the current user's profile.
//
//nolint:gocognit,gocyclo // User info resolution has several fallbacks that are clearer inline.
func MeHandler(srv server.Server) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Bind to the site serving this request before touching any
		// per-tenant resource; srv as constructed holds the process-wide
		// defaults, which in a multi-site deployment belong to no site.
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
		case "HEAD":
			w.WriteHeader(http.StatusOK)
			return

		case httpMethodGet:
			errResp := func(
				httpCode int, userErrMsg, logErrMsg string, err error,
				extraArgs ...interface{}) {
				srv.Logger.Error(logErrMsg,
					append([]interface{}{
						"error", err,
						"method", r.Method,
						"path", r.URL.Path,
					}, extraArgs...)...,
				)
				http.Error(w, userErrMsg, httpCode)
			}

			// Try to get user information from auth claims first
			claims, hasClaims := pkgauth.GetUserClaims(r.Context())

			var resp MeGetResponse

			if hasClaims && claims != nil {
				// Use claims from authentication provider (e.g., Dex OIDC)
				srv.Logger.Info("using user claims from auth context",
					"email", claims.Email,
					"name", claims.Name,
					"preferred_username", claims.PreferredUsername,
					"groups", claims.Groups)

				resp = MeGetResponse{
					ID:            claims.Email, // Use email as ID for non-workspace auth
					Email:         claims.Email,
					VerifiedEmail: true, // Verified by OIDC provider
					Name:          claims.Name,
					GivenName:     claims.GivenName,
					FamilyName:    claims.FamilyName,
				}

				// If name is empty, try to construct from given/family name or use preferred_username or email
				if resp.Name == "" {
					switch {
					case claims.PreferredUsername != "":
						resp.Name = claims.PreferredUsername
					case claims.GivenName != "" || claims.FamilyName != "":
						resp.Name = strings.TrimSpace(claims.GivenName + " " + claims.FamilyName)
					default:
						// Last resort: use email local part as display name
						resp.Name = strings.Split(claims.Email, "@")[0]
					}
				}
			}

			// Fallback to workspace provider (e.g., Google Workspace)
			srv.Logger.Debug("falling back to workspace provider for user info",
				"email", userEmail)

			ppl, err := srv.WorkspaceProvider.SearchPeople(
				r.Context(), userEmail)
			switch {
			case err != nil:
				// If workspace search fails (e.g., using local workspace with no indexed users),
				// return basic user info derived from email instead of failing
				srv.Logger.Warn("workspace search failed, returning basic user info from email",
					"email", userEmail,
					"error", err)

				resp = MeGetResponse{
					ID:            userEmail,
					Email:         userEmail,
					VerifiedEmail: true,                             // Verified by authentication
					Name:          strings.Split(userEmail, "@")[0], // Use email local part as name
				}
			case len(ppl) != 1:
				// If workspace search returns wrong number of results,
				// return basic user info instead of failing
				srv.Logger.Warn("workspace search returned unexpected number of results, returning basic user info",
					"email", userEmail,
					"result_count", len(ppl))

				resp = MeGetResponse{
					ID:            userEmail,
					Email:         userEmail,
					VerifiedEmail: true,                             // Verified by authentication
					Name:          strings.Split(userEmail, "@")[0], // Use email local part as name
				}

				// If configured, send an email to the user to notify them that their
				// account was not found in the directory.
				if srv.Config.Email != nil && srv.Config.Email.Enabled &&
					srv.Config.GoogleWorkspace != nil &&
					srv.Config.GoogleWorkspace.UserNotFoundEmail != nil &&
					srv.Config.GoogleWorkspace.UserNotFoundEmail.Enabled &&
					srv.Config.GoogleWorkspace.UserNotFoundEmail.Body != "" &&
					srv.Config.GoogleWorkspace.UserNotFoundEmail.Subject != "" {
					err = srv.WorkspaceProvider.SendEmail(
						r.Context(),
						[]string{userEmail},
						srv.Config.Email.FromAddress,
						srv.Config.GoogleWorkspace.UserNotFoundEmail.Subject,
						srv.Config.GoogleWorkspace.UserNotFoundEmail.Body,
					)
					if err != nil {
						srv.Logger.Error("error sending user not found email",
							"error", err,
							"method", r.Method,
							"path", r.URL.Path,
							"user_email", userEmail,
						)
					} else {
						srv.Logger.Info("user not found email sent",
							"method", r.Method,
							"path", r.URL.Path,
							"user_email", userEmail,
						)
					}
				}
			default:
				// Workspace search succeeded with exactly 1 result
				p := ppl[0]

				// Make sure that the result's email address is the same as the
				// authenticated user, is the primary email address, and is verified.
				// Make sure that the result's email address is the same as the
				// authenticated user. RFC-084 UserIdentity has Email field directly.
				if p.Email == "" || p.Email != userEmail {
					errResp(
						http.StatusInternalServerError,
						"Error getting user information",
						"wrong user in search result",
						err,
					)
					return
				}

				// Verify display name is set. RFC-084 UserIdentity has DisplayName field.
				if p.DisplayName == "" {
					errResp(
						http.StatusInternalServerError,
						"Error getting user information",
						"no display name in result",
						err,
					)
					return
				}

				// Build response from workspace provider data (RFC-084 UserIdentity)
				resp = MeGetResponse{
					ID:            p.Email, // Use email as ID
					Email:         p.Email,
					VerifiedEmail: true, // If SearchPeople returned it, it's verified
					Name:          p.DisplayName,
					// GivenName and FamilyName not available in RFC-084 UserIdentity
				}
				if p.PhotoURL != "" {
					resp.Picture = p.PhotoURL
				}
			}

			// Write response (common for both paths)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			enc := json.NewEncoder(w)
			if err := enc.Encode(resp); err != nil {
				errResp(
					http.StatusInternalServerError,
					"Error getting user information",
					"error encoding response",
					err,
				)
				return
			}

		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
	})
}
