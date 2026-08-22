// Package auth provides auth functionality.
package auth

import (
	"fmt"
	"net/http"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp-forge/hermes/internal/auth/microsoft"
	"github.com/hashicorp-forge/hermes/internal/config"
	"github.com/hashicorp-forge/hermes/internal/session"
	pkgauth "github.com/hashicorp-forge/hermes/pkg/auth"
	googleadapter "github.com/hashicorp-forge/hermes/pkg/auth/adapters/google"
	oktaadapter "github.com/hashicorp-forge/hermes/pkg/auth/adapters/okta"
	"github.com/hashicorp-forge/hermes/pkg/domain"
	sp "github.com/hashicorp-forge/hermes/pkg/sharepointhelper"
	gw "github.com/hashicorp-forge/hermes/pkg/workspace/adapters/google"
)

// SessionCookieName is the name of the cookie holding the signed Hermes
// session.
//
// Deprecated: use session.CookieName. Retained so existing references keep
// compiling.
const SessionCookieName = session.CookieName

// DexSessionProvider authenticates a request from its signed session cookie.
type DexSessionProvider struct {
	signer *session.Signer
	log    hclog.Logger
}

// NewDexSessionProvider creates a new Dex session provider.
func NewDexSessionProvider(signer *session.Signer, log hclog.Logger) *DexSessionProvider {
	return &DexSessionProvider{signer: signer, log: log}
}

// Authenticate verifies the session cookie and returns the user it names.
//
// The cookie is a signed token, not a bare identity: the value is only
// believed after its HMAC verifies and the site it was issued for matches the
// site serving this request. Both checks live here rather than in the caller,
// so no code path can reach a handler with an unverified email.
func (p *DexSessionProvider) Authenticate(r *http.Request) (string, error) {
	cookie, err := r.Cookie(session.CookieName)
	if err != nil {
		p.log.Debug("no session cookie found", "error", err)
		return "", fmt.Errorf("no session cookie found")
	}

	// A nil signer means the server started without one, which should be
	// impossible. Refuse rather than fall back to trusting the cookie: that
	// fallback is precisely the bypass this provider exists to close.
	if p.signer == nil {
		return "", fmt.Errorf("session signer is not configured")
	}

	// An absent domain is the single-site case and matches a token issued
	// without one. It is never treated as "any site".
	reqDomain, _ := domain.FromContext(r.Context())

	tok, err := p.signer.Verify(cookie.Value, reqDomain)
	if err != nil {
		// Logged, not returned: the caller renders a generic 401, so an
		// attacker cannot distinguish a forged signature from an expired
		// session.
		p.log.Debug("session cookie rejected",
			"reason", err, "domain", reqDomain.String(), "path", r.URL.Path)
		return "", fmt.Errorf("invalid session")
	}

	p.log.Debug("authenticated via session cookie",
		"email", tok.Email, "domain", tok.Domain.String())

	return tok.Email, nil
}

// Name returns the provider name.
func (p *DexSessionProvider) Name() string {
	return "dex-session"
}

// AuthenticateRequest is middleware that authenticates an HTTP request using
// the appropriate authentication provider based on configuration.
func AuthenticateRequest(
	cfg config.Config, gwSvc *gw.Service, spSvc *sp.Service, signer *session.Signer,
	log hclog.Logger, next http.Handler,
) http.Handler {
	var provider pkgauth.Provider

	// Priority: Dex > Okta > Google
	switch {
	case cfg.Dex != nil && !cfg.Dex.Disabled:
		// If Dex is configured and enabled, use Dex session-based authentication.
		// For Dex, we use session cookies instead of bearer tokens
		provider = NewDexSessionProvider(signer, log)
	case cfg.Okta != nil && !cfg.Okta.Disabled:
		// If Okta is configured and enabled, use Okta authentication.
		oktaCfg := oktaadapter.Config{
			AuthServerURL: cfg.Okta.AuthServerURL,
			AWSRegion:     cfg.Okta.AWSRegion,
			ClientID:      cfg.Okta.ClientID,
			Disabled:      cfg.Okta.Disabled,
			JWTSigner:     cfg.Okta.JWTSigner,
		}

		adapter, err := oktaadapter.NewAdapter(oktaCfg, log)
		if err != nil {
			log.Error("error creating Okta authentication adapter", "error", err)
			return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				http.Error(w, "Internal server error", http.StatusInternalServerError)
			})
		}
		provider = adapter
	case cfg.SharePoint != nil:
		return microsoft.AuthenticateRequest(cfg.SharePoint, log, spSvc,
			pkgauth.RequireUserEmail(log, next))
	default:
		// Use Google authentication.
		provider = googleadapter.NewAdapter(gwSvc)
	}

	// Wrap the handler with authentication middleware and an additional
	// safety check to ensure the user email is set.
	return pkgauth.Middleware(provider, log)(
		pkgauth.RequireUserEmail(log, next),
	)
}
