package microsoft

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func request(t *testing.T, header string, cookies map[string]string) *http.Request {
	t.Helper()

	r := httptest.NewRequest(http.MethodGet, "http://example.com/api/v2/me", nil)
	if header != "" {
		r.Header.Set("Authorization", header)
	}
	for name, value := range cookies {
		r.AddCookie(&http.Cookie{Name: name, Value: value})
	}

	return r
}

func TestExtractTokenPrefersTheAuthorizationHeader(t *testing.T) {
	t.Parallel()

	r := request(t, "Bearer header-token", map[string]string{"microsoft_token": "cookie-token"})

	if got := extractTokenFromRequest(r); got != "header-token" {
		t.Errorf("token = %q, want header-token", got)
	}
}

func TestExtractTokenReadsKnownCookies(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"microsoft_token", "token", "auth_token"} {
		r := request(t, "", map[string]string{name: "value-" + name})
		if got := extractTokenFromRequest(r); got != "value-"+name {
			t.Errorf("%s: token = %q", name, got)
		}
	}
}

// TestExtractTokenIgnoresUnrelatedCookies is the regression test for a
// credential-disclosure bug.
//
// The old fallback walked every cookie on the request and returned the first
// value longer than 100 characters, whenever a user_email cookie was present.
// Whatever it returned was sent onward to Microsoft Graph as a bearer token --
// so an analytics identifier, another application's session, or Hermes' own
// signed hermes_session cookie could be forwarded to a third party. Which one
// depended on the order the browser sent them in.
func TestExtractTokenIgnoresUnrelatedCookies(t *testing.T) {
	t.Parallel()

	long := strings.Repeat("x", 250)
	hermesSession := "hs1." + strings.Repeat("y", 200) + "." + strings.Repeat("z", 43)

	for name, value := range map[string]string{
		"hermes_session": hermesSession,
		"_ga":            long,
		"other_app_sid":  long,
		"csrf":           long,
	} {
		r := request(t, "", map[string]string{
			"user_email": "someone@example.com",
			name:         value,
		})

		if got := extractTokenFromRequest(r); got != "" {
			t.Errorf("cookie %q was returned as a bearer token (%d chars); "+
				"it would have been sent to Microsoft Graph", name, len(got))
		}
	}
}

// TestExtractTokenIgnoresUserEmailItself guards the specific shape of the old
// bug: user_email present, no token cookie, several long cookies to choose
// from.
func TestExtractTokenIgnoresUserEmailItself(t *testing.T) {
	t.Parallel()

	r := request(t, "", map[string]string{
		"user_email":  "someone@example.com",
		"session":     strings.Repeat("a", 300),
		"remember_me": strings.Repeat("b", 150),
	})

	if got := extractTokenFromRequest(r); got != "" {
		t.Errorf("token = %q, want none", got)
	}
}

func TestExtractTokenWithNothingPresent(t *testing.T) {
	t.Parallel()

	if got := extractTokenFromRequest(request(t, "", nil)); got != "" {
		t.Errorf("token = %q, want empty", got)
	}
	// A non-bearer Authorization header is not a token.
	if got := extractTokenFromRequest(request(t, "Basic abc123", nil)); got != "" {
		t.Errorf("token = %q, want empty", got)
	}
	// An empty bearer value falls through rather than authenticating as "".
	if got := extractTokenFromRequest(request(t, "Bearer ", nil)); got != "" {
		t.Errorf("token = %q, want empty", got)
	}
}
