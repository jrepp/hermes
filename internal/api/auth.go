// Package api provides api functionality.
package api

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp-forge/hermes/internal/config"
	"github.com/hashicorp-forge/hermes/internal/session"
	"github.com/hashicorp-forge/hermes/internal/sites"
	"github.com/hashicorp-forge/hermes/pkg/auth/adapters/dex"
	"github.com/hashicorp-forge/hermes/pkg/domain"
)

const (
	// State cookie name for CSRF protection
	stateCookieName = "hermes_oauth_state"
)

// dexConfigForRequest returns the Dex configuration to use for this request,
// with the OIDC redirect URI pointed at the site being served.
//
// OIDC requires the redirect_uri sent at authorization and at token exchange
// to be identical, and the browser has to come back to the host it left. A
// single configured redirect_url would send every site's login to one host, so
// a user signing in at notes.jrepp.com would land on docs.jrepp.com holding a
// session that is valid only there.
//
// Each site's callback URL must be registered on the Dex client. Dex accepts a
// list, so this is a configuration change on the identity provider, not a
// limitation here.
func dexConfigForRequest(cfg config.Config, r *http.Request) dex.Config {
	dexCfg := *cfg.Dex

	site, ok := sites.FromContext(r.Context())
	if !ok || site.BaseURL == "" {
		return dexCfg
	}
	dexCfg.RedirectURL = strings.TrimSuffix(site.BaseURL, "/") + "/auth/callback"

	return dexCfg
}

// LoginHandler redirects the user to the Dex OIDC authorization endpoint.
// It generates a random state parameter for CSRF protection and stores it in a cookie.
func LoginHandler(cfg config.Config, log hclog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only support Dex authentication
		if cfg.Dex == nil || cfg.Dex.Disabled {
			http.Error(w, "Dex authentication not configured", http.StatusInternalServerError)
			return
		}

		// Create Dex adapter
		adapter, err := dex.NewAdapter(dexConfigForRequest(cfg, r), log)
		if err != nil {
			// Building the adapter performs OIDC discovery, so this fails when
			// the identity provider is unreachable or not yet up -- not
			// because Hermes is misconfigured. Reporting 500 sent operators
			// looking at Hermes during exactly the window when the answer was
			// "Dex is down"; 502 says the dependency is the problem.
			log.Error("failed to reach the identity provider", "error", err)
			http.Error(w, "Identity provider unavailable", http.StatusBadGateway)
			return
		}

		// Generate random state for CSRF protection
		state, err := generateRandomState()
		if err != nil {
			log.Error("failed to generate state", "error", err)
			http.Error(w, "Failed to generate state", http.StatusInternalServerError)
			return
		}

		// Store state in cookie
		http.SetCookie(w, &http.Cookie{
			Name:     stateCookieName,
			Value:    state,
			Path:     "/",
			MaxAge:   int(5 * time.Minute / time.Second), // State expires in 5 minutes
			HttpOnly: true,
			Secure:   sites.SecureCookiesForRequest(r),
			SameSite: http.SameSiteLaxMode,
		})

		// Redirect to Dex authorization URL
		authURL := adapter.GetAuthCodeURL(state)
		log.Debug("redirecting to Dex authorization URL", "url", authURL)
		http.Redirect(w, r, authURL, http.StatusFound)
	})
}

// CallbackHandler handles the OAuth2 callback from Dex.
// It exchanges the authorization code for an ID token, validates it,
// and establishes a session for the authenticated user.
//
//nolint:gocognit // Callback flow is linear but includes several explicit auth failure branches.
func CallbackHandler(cfg config.Config, signer *session.Signer, log hclog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only support Dex authentication
		if cfg.Dex == nil || cfg.Dex.Disabled {
			http.Error(w, "Dex authentication not configured", http.StatusInternalServerError)
			return
		}

		// Create Dex adapter
		adapter, err := dex.NewAdapter(dexConfigForRequest(cfg, r), log)
		if err != nil {
			// Building the adapter performs OIDC discovery, so this fails when
			// the identity provider is unreachable or not yet up -- not
			// because Hermes is misconfigured. Reporting 500 sent operators
			// looking at Hermes during exactly the window when the answer was
			// "Dex is down"; 502 says the dependency is the problem.
			log.Error("failed to reach the identity provider", "error", err)
			http.Error(w, "Identity provider unavailable", http.StatusBadGateway)
			return
		}

		// Get state from cookie
		stateCookie, err := r.Cookie(stateCookieName)
		if err != nil {
			log.Error("state cookie not found", "error", err)
			http.Error(w, "Invalid authentication state", http.StatusBadRequest)
			return
		}

		// Validate state parameter
		state := r.URL.Query().Get("state")
		if state == "" || state != stateCookie.Value {
			// The state is a single-use CSRF token; logging its value puts a
			// credential in the log for no diagnostic gain. Whether each side
			// was present is what actually distinguishes the cases -- a
			// missing parameter, an expired cookie, or a genuine mismatch.
			log.Error("OIDC state mismatch",
				"have_cookie", stateCookie.Value != "",
				"have_param", state != "")
			http.Error(w, "Invalid state parameter", http.StatusBadRequest)
			return
		}

		// Clear state cookie
		http.SetCookie(w, &http.Cookie{
			Name:     stateCookieName,
			Value:    "",
			Path:     "/",
			MaxAge:   -1,
			HttpOnly: true,
		})

		// Get authorization code
		code := r.URL.Query().Get("code")
		if code == "" {
			// Check for error response
			errorCode := r.URL.Query().Get("error")
			errorDesc := r.URL.Query().Get("error_description")
			log.Error("authorization failed", "error", errorCode, "description", errorDesc)
			http.Error(w, fmt.Sprintf("Authorization failed: %s", errorDesc), http.StatusBadRequest)
			return
		}

		// Exchange code for email
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		email, err := adapter.ExchangeCode(ctx, code)
		if err != nil {
			log.Error("failed to exchange code", "error", err)
			http.Error(w, "Failed to complete authentication", http.StatusInternalServerError)
			return
		}

		log.Info("user authenticated successfully", "email", email)

		if signer == nil {
			log.Error("no session signer configured; refusing to issue a session")
			http.Error(w, "Authentication configuration error", http.StatusInternalServerError)
			return
		}

		// Bind the session to the site that served the callback. A token
		// minted here will not verify against any other site, so a cookie that
		// reaches a sibling subdomain -- which any host under the parent
		// domain can arrange -- authenticates nobody there.
		reqDomain, _ := domain.FromContext(r.Context())

		token, err := signer.Issue(email, reqDomain)
		if err != nil {
			log.Error("failed to issue session", "error", err)
			http.Error(w, "Failed to complete authentication", http.StatusInternalServerError)
			return
		}

		http.SetCookie(w, signer.Cookie(token, sites.SecureCookiesForRequest(r)))

		// Build redirect URL - use BaseURL from config if available
		// This ensures we redirect to the frontend URL (e.g., http://localhost:4201)
		// instead of the backend URL (e.g., http://localhost:8001)
		redirectPath := "/dashboard"

		// Check if there's a redirect path in the query params
		if redirect := r.URL.Query().Get("redirect"); redirect != "" {
			// Validate redirect URL to prevent open redirects
			if u, err := url.Parse(redirect); err == nil && u.Host == "" {
				redirectPath = redirect
			}
		}

		// Construct absolute URL if a base URL is known. The site's own origin
		// wins over the global one: sending a docs.jrepp.com login back to
		// whatever base_url happens to be set at the top level would drop the
		// user on another tenant.
		configuredBase := cfg.BaseURL
		if site, ok := sites.FromContext(r.Context()); ok && site.BaseURL != "" {
			configuredBase = site.BaseURL
		}

		var redirectURL string
		if configuredBase != "" {
			baseURL, err := url.Parse(configuredBase)
			if err != nil {
				log.Error("invalid base_url in configuration", "base_url", configuredBase, "error", err)
				redirectURL = redirectPath // Fallback to relative path
			} else {
				baseURL.Path = redirectPath
				redirectURL = baseURL.String()
			}
		} else {
			// Fallback to relative redirect if BaseURL not configured
			redirectURL = redirectPath
		}

		log.Info("redirecting after authentication",
			"url", redirectURL, "base_url", configuredBase, "email", email)
		http.Redirect(w, r, redirectURL, http.StatusFound)
	})
}

// LogoutHandler clears the session cookie and redirects to the home page.
func LogoutHandler(log hclog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Clear the session cookie. The attributes must match those used at
		// issue time or the browser keeps the original alongside this one.
		http.SetCookie(w, session.ClearCookie(sites.SecureCookiesForRequest(r)))

		log.Debug("user logged out")

		// Redirect to home
		http.Redirect(w, r, "/", http.StatusFound)
	})
}

// generateRandomState generates a cryptographically secure random state string.
func generateRandomState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}
	return base64.URLEncoding.EncodeToString(b), nil
}
