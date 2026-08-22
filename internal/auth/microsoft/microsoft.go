package microsoft

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/hashicorp/go-hclog"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/microsoft"

	"github.com/hashicorp-forge/hermes/internal/config"
	"github.com/hashicorp-forge/hermes/pkg/sharepointhelper"
)

// MicrosoftAuthenticator handles Microsoft authentication
type MicrosoftAuthenticator struct {
	Config *config.MicrosoftAuth
	Log    hclog.Logger
}

// New creates a new Microsoft authenticator
func New(cfg config.MicrosoftAuth, log hclog.Logger) (*MicrosoftAuthenticator, error) {
	return &MicrosoftAuthenticator{
		Config: &cfg,
		Log:    log,
	}, nil
}

// AuthenticateRequest is middleware that authenticates an HTTP request using Microsoft
func AuthenticateRequest(cfg *config.SharePointConfig, log hclog.Logger, spService *sharepointhelper.Service, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// For static assets and public paths, skip authentication but set a default email
		if strings.HasPrefix(r.URL.Path, "/assets/") ||
			strings.HasPrefix(r.URL.Path, "/public/") ||
			strings.HasPrefix(r.URL.Path, "/static/") ||
			strings.HasPrefix(r.URL.Path, "/images") ||
			strings.HasPrefix(r.URL.Path, "/addin/") ||
			strings.HasPrefix(r.URL.Path, "/.") ||
			r.URL.Path == "/favicon.ico" {
			// Set a default email in context to avoid errors downstream
			ctx := context.WithValue(r.Context(), "userEmail", "anonymous@static-asset")
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		// Special case for the authenticate page itself
		if r.URL.Path == "/authenticate" {
			// If code is present, this is the callback from Microsoft
			if r.URL.Query().Get("code") != "" {
				handleAuthCallback(w, r, cfg, log)
				return
			} else if r.URL.Query().Get("init") == "true" {
				// If init parameter is present, initiate the auth flow
				initiateAuthFlow(w, r, cfg, log)
				return
			} else if r.Method == "GET" && strings.Contains(r.Header.Get("Accept"), "text/html") {
				// If it looks like a browser request directly to /authenticate, initiate the auth flow
				initiateAuthFlow(w, r, cfg, log)
				return
			} else {
				// For other cases like API requests, serve the page normally
				next.ServeHTTP(w, r)
				return
			}
		}

		// Check for existing auth token in header or cookie
		token := extractTokenFromRequest(r)
		if token != "" {
			// Validate token with Microsoft using SharePoint service
			if validateUserToken(token, log, spService) {
				// Set user email in context
				email, err := getUserEmailFromToken(token, log, spService)
				if err == nil && email != "" {

					// Set both user email AND Microsoft token in context for downstream handlers
					ctx := context.WithValue(r.Context(), "userEmail", email)
					ctx = context.WithValue(ctx, "microsoftToken", token)

					next.ServeHTTP(w, r.WithContext(ctx))
					return
				} else {
					log.Warn("Failed to get user email from token", "error", err)
				}
			} else {
				log.Warn("token validation failed")
			}
		}

		// If it's an API request or AJAX request, return 401 with JSON response
		if strings.HasPrefix(r.URL.Path, "/api/") || r.Header.Get("X-Requested-With") == "XMLHttpRequest" {
			log.Warn("unauthorized API request", "path", r.URL.Path)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error":    "Unauthorized",
				"redirect": "/authenticate",
				"message":  "Please authenticate to access this resource",
			})
			return
		}

		// For regular web pages, redirect to authenticate
		if r.URL.Path != "/authenticate" {
			http.Redirect(w, r, "/authenticate", http.StatusFound)
			return
		}

		// Allow the /authenticate page to be served
		next.ServeHTTP(w, r)
	})
}

// tokenCookieNames are the cookies that may carry a Microsoft access token,
// in preference order.
var tokenCookieNames = []string{"microsoft_token", "token", "auth_token"}

// extractTokenFromRequest returns the Microsoft access token presented by the
// request, or "" if there is none.
//
// It looks only at the Authorization header and at cookies known to carry a
// token. It used to have a further fallback: if a user_email cookie was
// present, it walked every cookie on the request and returned the first value
// longer than 100 characters, on the reasoning that "a token should be fairly
// long".
//
// That is a credential-disclosure bug, because the value it picks is sent
// onward to Microsoft Graph as a bearer token. Any long cookie on the domain
// qualifies -- an analytics identifier, another application's session, or
// Hermes' own signed hermes_session cookie, which is comfortably over 100
// characters. Which one it chose depended on the order the browser happened to
// send them in.
//
// Removing the fallback means a request without a recognised token cookie
// fails authentication instead of guessing. Failing is the correct outcome;
// guessing a credential and forwarding it to a third party is not.
func extractTokenFromRequest(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token != "" {
			return token
		}
	}

	for _, name := range tokenCookieNames {
		cookie, err := r.Cookie(name)
		if err == nil && cookie.Value != "" {
			return cookie.Value
		}
	}

	return ""
}

// validateUserToken validates the token with Microsoft using SharePoint service
func validateUserToken(token string, log hclog.Logger, spService *sharepointhelper.Service) bool {
	if spService == nil {
		log.Error("SharePoint service is required for token validation")
		return false
	}

	return spService.ValidateUserToken(token)
}

// getUserEmailFromToken gets the user email from the token using SharePoint service
func getUserEmailFromToken(token string, log hclog.Logger, spService *sharepointhelper.Service) (string, error) {
	if spService == nil {
		log.Error("SharePoint service is required for getting user email")
		return "", fmt.Errorf("SharePoint service is required")
	}

	return spService.GetUserEmailFromToken(token)
}

func initiateAuthFlow(w http.ResponseWriter, r *http.Request, cfg *config.SharePointConfig, logger hclog.Logger) {
	logger.Debug("initiating Microsoft auth flow")

	// Check if this is a popup flow request.
	// The "popup" parameter is expected to be "true" or "false" (case-insensitive).
	popupParam := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("popup")))
	isPopup := popupParam == "true"

	// Create OAuth2 config for Microsoft
	oauth2Config := &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  cfg.RedirectURI,
		Endpoint:     microsoft.AzureADEndpoint(cfg.TenantID),
		Scopes:       []string{"openid", "profile", "email", "User.Read"},
	}

	// Generate authorization URL with a random state for security
	state := fmt.Sprintf("%d", time.Now().UnixNano())
	if isPopup {
		state = "popup_" + state
	}
	url := oauth2Config.AuthCodeURL(state, oauth2.AccessTypeOffline)

	// Set a cookie to indicate auth is in progress
	http.SetCookie(w, &http.Cookie{
		Name:     "auth_in_progress",
		Value:    "microsoft",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   600, // 10 minutes
		SameSite: http.SameSiteNoneMode,
		Secure:   true,
	})

	// Store popup state in a cookie if this is a popup flow
	if isPopup {
		http.SetCookie(w, &http.Cookie{
			Name:     "auth_is_popup",
			Value:    "true",
			Path:     "/",
			HttpOnly: true,
			MaxAge:   600, // 10 minutes
			SameSite: http.SameSiteNoneMode,
			Secure:   true,
		})
	}

	logger.Debug("Auth cookie set, redirecting to Microsoft login page", "isPopup", isPopup)

	// Redirect to Microsoft login page
	http.Redirect(w, r, url, http.StatusFound)
}

func handleAuthCallback(w http.ResponseWriter, r *http.Request, cfg *config.SharePointConfig, logger hclog.Logger) {
	// Log the start of the callback handling
	logger.Info("Handling Microsoft auth callback", "path", r.URL.Path)

	// Get authorization code from query parameters
	code := r.URL.Query().Get("code")
	if code == "" {
		logger.Error("No authorization code provided in callback")
		http.Error(w, "No authorization code provided", http.StatusBadRequest)
		return
	}
	logger.Info("Authorization code received", "code_length", len(code))

	// Create OAuth2 config for Microsoft
	oauth2Config := &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  cfg.RedirectURI,
		Endpoint:     microsoft.AzureADEndpoint(cfg.TenantID),
		Scopes:       []string{"openid", "profile", "email", "User.Read"},
	}

	// Exchange authorization code for token
	logger.Info("Exchanging authorization code for token")
	token, err := oauth2Config.Exchange(context.Background(), code)
	if err != nil {
		logger.Error("Error exchanging code for token", "error", err)
		http.Error(w, "Failed to authenticate: "+err.Error(), http.StatusInternalServerError)
		return
	}
	logger.Info("Successfully exchanged code for token", "token_type", token.TokenType, "expires", token.Expiry)

	// Get user email from Microsoft Graph
	logger.Info("Fetching user email from Microsoft Graph API")
	client := oauth2Config.Client(context.Background(), token)
	resp, err := client.Get("https://graph.microsoft.com/v1.0/me")
	if err != nil {
		logger.Error("Error calling Microsoft Graph API", "error", err)
		http.Error(w, "Failed to get user info: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logger.Error("Microsoft Graph API returned non-200 status", "status", resp.StatusCode)
		http.Error(w, "Failed to get user info", http.StatusInternalServerError)
		return
	}

	// Parse the response body
	var data struct {
		Mail              string `json:"mail"`
		UserPrincipalName string `json:"userPrincipalName"`
		DisplayName       string `json:"displayName"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		logger.Error("Error decoding Microsoft Graph API response", "error", err)
		http.Error(w, "Failed to parse user info: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Use mail if available, otherwise use userPrincipalName
	email := data.Mail
	if email == "" {
		email = data.UserPrincipalName
	}
	if email == "" {
		logger.Error("No email found in Microsoft Graph API response")
		http.Error(w, "Failed to get user email", http.StatusInternalServerError)
		return
	}
	logger.Info("User authenticated successfully", "email", email, "name", data.DisplayName)

	// Check if this is a popup authentication flow
	isPopupFlow := false
	if cookie, err := r.Cookie("auth_is_popup"); err == nil && cookie.Value == "true" {
		isPopupFlow = true
	}

	// Set cookies with authentication information
	http.SetCookie(w, &http.Cookie{
		Name:     "user_email",
		Value:    email,
		Path:     "/",
		HttpOnly: true,
		MaxAge:   3600,
		SameSite: http.SameSiteNoneMode,
		Secure:   true,
	})

	http.SetCookie(w, &http.Cookie{
		Name:     "microsoft_token",
		Value:    token.AccessToken,
		Path:     "/",
		HttpOnly: true,
		MaxAge:   int(time.Until(token.Expiry).Seconds()),
		SameSite: http.SameSiteNoneMode,
		Secure:   true,
	})

	// Clear the popup indicator cookie
	if isPopupFlow {
		http.SetCookie(w, &http.Cookie{
			Name:     "auth_is_popup",
			Value:    "",
			Path:     "/",
			HttpOnly: true,
			MaxAge:   -1, // Delete cookie
			SameSite: http.SameSiteNoneMode,
			Secure:   true,
		})
	}

	// Log success before redirect
	logger.Info("Authentication successful, redirecting", "email", email, "isPopup", isPopupFlow)

	// If this is a popup flow, redirect to the auth callback page
	// Otherwise, redirect to the dashboard
	if isPopupFlow {
		http.Redirect(w, r, "/addin/auth-callback.html", http.StatusFound)
	} else {
		http.Redirect(w, r, "/dashboard", http.StatusFound)
	}
}
