package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/hashicorp-forge/hermes/internal/server"
	pkgauth "github.com/hashicorp-forge/hermes/pkg/auth"
)

// handleGetUserProfileGoogle handles the GET request for user profile data using Google Workspace.
func handleGetUserProfileGoogle(srv server.Server, w http.ResponseWriter, r *http.Request, userEmail string) {
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

	ppl, err := srv.GWService.SearchPeople(
		userEmail, "emailAddresses,names,photos")
	if err != nil {
		errResp(
			http.StatusInternalServerError,
			"Error getting user information",
			"error searching people directory",
			err,
		)
		return
	}

	// Verify that the result only contains one person.
	if len(ppl) != 1 {
		errResp(
			http.StatusInternalServerError,
			"Error getting user information",
			fmt.Sprintf(
				"wrong number of people in search result: %d", len(ppl)),
			nil,
			"user_email", userEmail,
		)

		// If configured, send an email to the user to notify them that their
		// account was not found in the directory.
		if srv.Config.Email != nil && srv.Config.Email.Enabled &&
			srv.Config.GoogleWorkspace != nil &&
			srv.Config.GoogleWorkspace.UserNotFoundEmail != nil &&
			srv.Config.GoogleWorkspace.UserNotFoundEmail.Enabled &&
			srv.Config.GoogleWorkspace.UserNotFoundEmail.Body != "" &&
			srv.Config.GoogleWorkspace.UserNotFoundEmail.Subject != "" {
			_, err = srv.GWService.SendEmail(
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

		return
	}
	p := ppl[0]

	// Make sure that the result's email address is the same as the
	// authenticated user, is the primary email address, and is verified.
	if len(p.EmailAddresses) == 0 ||
		p.EmailAddresses[0].Value != userEmail ||
		!p.EmailAddresses[0].Metadata.Primary ||
		!p.EmailAddresses[0].Metadata.Verified {
		errResp(
			http.StatusInternalServerError,
			"Error getting user information",
			"wrong user in search result",
			err,
		)
		return
	}

	// Verify other required values are set.
	if len(p.Names) == 0 {
		errResp(
			http.StatusInternalServerError,
			"Error getting user information",
			"no names in result",
			err,
		)
		return
	}

	// Write response.
	resp := MeGetResponse{
		ID:         p.EmailAddresses[0].Metadata.Source.Id,
		Email:      p.EmailAddresses[0].Value,
		Name:       p.Names[0].DisplayName,
		GivenName:  p.Names[0].GivenName,
		FamilyName: p.Names[0].FamilyName,
	}
	if len(p.Photos) > 0 {
		resp.Picture = p.Photos[0].Url
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	enc := json.NewEncoder(w)
	err = enc.Encode(resp)
	if err != nil {
		errResp(
			http.StatusInternalServerError,
			"Error getting user information",
			"error encoding response",
			err,
		)
		return
	}
}

type MeGetResponse struct {
	ID         string `json:"id"`
	Email      string `json:"email"`
	Name       string `json:"name"`
	GivenName  string `json:"given_name"`
	FamilyName string `json:"family_name"`
	Picture    string `json:"picture"`
	Locale     string `json:"locale,omitempty"`
	HD         string `json:"hd,omitempty"`
}

// convertGraphUserToMeResponse converts Microsoft Graph user directly to Me response
func convertGraphUserToMeResponse(user *sharepointhelper.Person) MeGetResponse {
	// Use primary email (mail field) or fallback to userPrincipalName
	email := user.Mail
	if email == "" {
		email = user.UserPrincipalName
	}

	// Create display name if empty
	displayName := user.DisplayName
	if displayName == "" {
		displayName = user.GivenName + " " + user.Surname
	}

	// Extract domain from email for HD field
	var hd string
	if email != "" {
		parts := strings.Split(email, "@")
		if len(parts) > 1 {
			hd = parts[1]
		}
	}

	// Set the profile picture URL to use our backend API v2
	pictureURL := fmt.Sprintf("/api/v2/people?photo=%s&v=%d", url.QueryEscape(email), time.Now().Unix())

	return MeGetResponse{
		ID:         user.ID,
		Email:      email,
		Name:       displayName,
		GivenName:  user.GivenName,
		FamilyName: user.Surname,
		Picture:    pictureURL,
		Locale:     "en",
		HD:         hd,
	}
}

// handleGetUserProfile handles the GET request for user profile data.
// It delegates to SharePoint or Google depending on which backend is configured.
func handleGetUserProfile(srv server.Server, w http.ResponseWriter, r *http.Request, userEmail string) {
	srv.Logger.Info("me handler called",
		"method", r.Method,
		"path", r.URL.Path,
		"user_email", userEmail,
	)

	if srv.SharePoint != nil {
		handleGetUserProfileSharePoint(srv, w, r, userEmail)
	} else {
		handleGetUserProfileGoogle(srv, w, r, userEmail)
	}
}

// handleGetUserProfileSharePoint handles the GET request for user profile data using Microsoft Graph.
func handleGetUserProfileSharePoint(srv server.Server, w http.ResponseWriter, r *http.Request, userEmail string) {
	// Get user directly from Microsoft Graph API
	user, err := srv.SharePoint.GetPersonByEmail(userEmail)
	if err != nil {
		srv.Logger.Error("error getting user from Microsoft Graph",
			"error", err,
			"user_email", userEmail,
		)

		// Check if it's a user not found error (404)
		if errors.Is(err, sharepointhelper.ErrUserNotFound) {
			srv.Logger.Error("user not found in Microsoft Graph",
				"user_email", userEmail,
			)
			http.Error(w, "User not found", http.StatusNotFound)
			return
		}

		// For all other errors, return 500
		http.Error(w, "Error getting user profile", http.StatusInternalServerError)
		return
	}

	// Convert directly to response format
	resp := convertGraphUserToMeResponse(user)

	srv.Logger.Info("returning Microsoft Graph user data",
		"user_id", resp.ID,
		"user_name", resp.Name,
		"user_email", resp.Email,
	)

	// Write response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	enc := json.NewEncoder(w)
	err = enc.Encode(resp)
	if err != nil {
		srv.Logger.Error("error encoding user response",
			"error", err,
			"method", r.Method,
			"path", r.URL.Path,
		)
		http.Error(w, "Error getting user information", http.StatusInternalServerError)
		return
	}
}

func MeHandler(srv server.Server) http.Handler {
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

		case "GET":
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
					if claims.PreferredUsername != "" {
						resp.Name = claims.PreferredUsername
					} else if claims.GivenName != "" || claims.FamilyName != "" {
						resp.Name = strings.TrimSpace(claims.GivenName + " " + claims.FamilyName)
					} else {
						// Last resort: use email local part as display name
						resp.Name = strings.Split(claims.Email, "@")[0]
					}
				}
			} else {
				// Fallback to workspace provider (e.g., Google Workspace)
				srv.Logger.Debug("falling back to workspace provider for user info",
					"email", userEmail)

				ppl, err := srv.WorkspaceProvider.SearchPeople(
					r.Context(), userEmail)
				if err != nil {
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
				} else if len(ppl) != 1 {
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
				} else {
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
			}

			// Write response (common for both paths)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			enc := json.NewEncoder(w)
			err := enc.Encode(resp)
			if err != nil {
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
