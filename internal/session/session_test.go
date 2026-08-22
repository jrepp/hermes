package session

import (
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp-forge/hermes/pkg/domain"
)

const testSecret = "test-secret-that-is-long-enough-for-hmac"

func testSigner(t *testing.T) *Signer {
	t.Helper()

	key, err := DeriveKey(testSecret)
	if err != nil {
		t.Fatalf("DeriveKey: %v", err)
	}
	s, err := NewSigner(key, time.Hour)
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}

	return s
}

func TestIssueAndVerifyRoundTrip(t *testing.T) {
	s := testSigner(t)
	d := domain.MustParse("docs.jrepp.com")

	raw, err := s.Issue("user@jrepp.com", d)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	tok, err := s.Verify(raw, d)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if tok.Email != "user@jrepp.com" {
		t.Errorf("email = %q, want user@jrepp.com", tok.Email)
	}
	if tok.Domain != d {
		t.Errorf("domain = %v, want %v", tok.Domain, d)
	}
	if tok.Nonce == "" {
		t.Error("nonce is empty")
	}
	if !tok.ExpiresAt.After(tok.IssuedAt) {
		t.Errorf("expiry %v is not after issue %v", tok.ExpiresAt, tok.IssuedAt)
	}
}

// TestBareEmailIsRejected pins the vulnerability this package was written to
// close: before it existed the cookie value *was* the identity, so anyone
// could send `Cookie: hermes_session=someone@example.com` and be that person.
func TestBareEmailIsRejected(t *testing.T) {
	s := testSigner(t)
	d := domain.MustParse("docs.jrepp.com")

	forged := []string{
		"user@jrepp.com",
		"hs1.user@jrepp.com",
		"hs1.user@jrepp.com.",
		"hs1..",
		"..",
		"",
	}
	for _, raw := range forged {
		if _, err := s.Verify(raw, d); err == nil {
			t.Errorf("Verify(%q) succeeded; a forged cookie must never authenticate", raw)
		}
	}
}

func TestVerifyRejectsTamperedPayload(t *testing.T) {
	s := testSigner(t)
	d := domain.MustParse("docs.jrepp.com")

	raw, err := s.Issue("user@jrepp.com", d)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		t.Fatalf("token has %d parts, want 3", len(parts))
	}

	// Flip one character of the payload, keeping the original signature.
	body := []byte(parts[1])
	body[0] ^= 0x01
	tampered := parts[0] + "." + string(body) + "." + parts[2]

	if _, err := s.Verify(tampered, d); err == nil {
		t.Fatal("Verify accepted a token whose payload was edited")
	}
}

func TestVerifyRejectsTruncatedSignature(t *testing.T) {
	s := testSigner(t)
	d := domain.MustParse("docs.jrepp.com")

	raw, err := s.Issue("user@jrepp.com", d)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	// A verifier comparing only a prefix would accept this.
	truncated := raw[:len(raw)-4]
	if _, err := s.Verify(truncated, d); !errors.Is(err, ErrBadSignature) &&
		!errors.Is(err, ErrMalformed) {
		t.Fatalf("Verify(truncated) error = %v, want a rejection", err)
	}
}

func TestVerifyRejectsForeignKey(t *testing.T) {
	d := domain.MustParse("docs.jrepp.com")

	mine := testSigner(t)

	otherKey, err := DeriveKey("a completely different secret, also long enough")
	if err != nil {
		t.Fatalf("DeriveKey: %v", err)
	}
	theirs, err := NewSigner(otherKey, time.Hour)
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}

	raw, err := theirs.Issue("attacker@evil.example", d)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	if _, err := mine.Verify(raw, d); !errors.Is(err, ErrBadSignature) {
		t.Fatalf("Verify error = %v, want ErrBadSignature", err)
	}
}

// TestVerifyRejectsCrossSiteReplay is the multi-tenant case. Any host under
// jrepp.com can set a cookie scoped to jrepp.com, which the browser then sends
// to every sibling subdomain, so a token that verified on the wrong site would
// be a full cross-tenant takeover.
func TestVerifyRejectsCrossSiteReplay(t *testing.T) {
	s := testSigner(t)
	docs := domain.MustParse("docs.jrepp.com")
	notes := domain.MustParse("notes.jrepp.com")

	raw, err := s.Issue("user@jrepp.com", docs)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	if _, err := s.Verify(raw, notes); !errors.Is(err, ErrWrongDomain) {
		t.Fatalf("Verify against another site: error = %v, want ErrWrongDomain", err)
	}
}

func TestVerifyDistinguishesScopedFromUnscoped(t *testing.T) {
	s := testSigner(t)
	d := domain.MustParse("docs.jrepp.com")

	scoped, err := s.Issue("user@jrepp.com", d)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	unscoped, err := s.Issue("user@jrepp.com", domain.Name{})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	if _, err := s.Verify(scoped, domain.Name{}); !errors.Is(err, ErrWrongDomain) {
		t.Errorf("scoped token verified without a site: %v", err)
	}
	if _, err := s.Verify(unscoped, d); !errors.Is(err, ErrWrongDomain) {
		t.Errorf("unscoped token verified against a site: %v", err)
	}
	if _, err := s.Verify(unscoped, domain.Name{}); err != nil {
		t.Errorf("unscoped token failed against no site: %v", err)
	}
}

func TestVerifyRejectsExpired(t *testing.T) {
	s := testSigner(t)
	d := domain.MustParse("docs.jrepp.com")

	base := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return base }

	raw, err := s.Issue("user@jrepp.com", d)
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	// One second before expiry the session is still good.
	s.now = func() time.Time { return base.Add(time.Hour - time.Second) }
	if _, err := s.Verify(raw, d); err != nil {
		t.Fatalf("Verify just before expiry: %v", err)
	}

	// At expiry it is not.
	s.now = func() time.Time { return base.Add(time.Hour) }
	if _, err := s.Verify(raw, d); !errors.Is(err, ErrExpired) {
		t.Fatalf("Verify at expiry: error = %v, want ErrExpired", err)
	}
}

// TestIssueIsUnpredictable guards the nonce. Without it two logins by the same
// user in the same second would produce byte-identical cookies, which leaks
// that fact to anyone observing them and removes the only handle a future
// revocation list could use.
func TestIssueIsUnpredictable(t *testing.T) {
	s := testSigner(t)
	d := domain.MustParse("docs.jrepp.com")

	fixed := time.Date(2026, 8, 21, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return fixed }

	seen := make(map[string]bool, 64)
	for i := 0; i < 64; i++ {
		raw, err := s.Issue("user@jrepp.com", d)
		if err != nil {
			t.Fatalf("Issue: %v", err)
		}
		if seen[raw] {
			t.Fatalf("Issue produced a duplicate token at iteration %d", i)
		}
		seen[raw] = true
	}
}

func TestIssueRejectsEmptyEmail(t *testing.T) {
	s := testSigner(t)
	if _, err := s.Issue("", domain.MustParse("docs.jrepp.com")); !errors.Is(err, ErrNoEmail) {
		t.Fatalf("Issue(\"\") error = %v, want ErrNoEmail", err)
	}
}

func TestVerifyRejectsOversizedToken(t *testing.T) {
	s := testSigner(t)
	huge := "hs1." + strings.Repeat("A", maxTokenLength) + ".sig"
	if _, err := s.Verify(huge, domain.Name{}); !errors.Is(err, ErrMalformed) {
		t.Fatalf("Verify(oversized) error = %v, want ErrMalformed", err)
	}
}

func TestNewSignerRejectsWeakKey(t *testing.T) {
	if _, err := NewSigner([]byte("short"), time.Hour); err == nil {
		t.Fatal("NewSigner accepted a 5-byte key")
	}
	if _, err := NewSigner(make([]byte, MinKeyLength), -time.Hour); err == nil {
		t.Fatal("NewSigner accepted a negative ttl")
	}
}

// TestSignerCopiesKey makes sure a caller that zeroes its secret buffer after
// construction -- good hygiene -- does not silently invalidate every session.
func TestSignerCopiesKey(t *testing.T) {
	key, err := DeriveKey(testSecret)
	if err != nil {
		t.Fatalf("DeriveKey: %v", err)
	}
	s, err := NewSigner(key, time.Hour)
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}

	raw, err := s.Issue("user@jrepp.com", domain.Name{})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	for i := range key {
		key[i] = 0
	}

	if _, err := s.Verify(raw, domain.Name{}); err != nil {
		t.Fatalf("Verify after caller zeroed its key buffer: %v", err)
	}
}

func TestCookieAttributes(t *testing.T) {
	s := testSigner(t)
	c := s.Cookie("value", true)

	if c.Name != CookieName {
		t.Errorf("name = %q, want %q", c.Name, CookieName)
	}
	// A Domain attribute would make the cookie visible to every sibling
	// subdomain, which is exactly the sharing this design avoids.
	if c.Domain != "" {
		t.Errorf("cookie sets Domain=%q; it must stay host-only", c.Domain)
	}
	if !c.HttpOnly {
		t.Error("cookie is not HttpOnly")
	}
	if !c.Secure {
		t.Error("cookie is not Secure when asked for a secure cookie")
	}
	if c.SameSite != http.SameSiteLaxMode {
		t.Errorf("SameSite = %v, want Lax", c.SameSite)
	}
	if c.MaxAge != int(time.Hour/time.Second) {
		t.Errorf("MaxAge = %d, want %d", c.MaxAge, int(time.Hour/time.Second))
	}
}

// TestClearCookieMatchesIssuedAttributes guards a subtle browser rule: a
// deletion cookie whose Path/Secure/SameSite differ from the original is a
// *different* cookie, so the session would survive logout.
func TestClearCookieMatchesIssuedAttributes(t *testing.T) {
	s := testSigner(t)

	for _, secure := range []bool{true, false} {
		issued := s.Cookie("value", secure)
		cleared := ClearCookie(secure)

		if issued.Path != cleared.Path ||
			issued.Secure != cleared.Secure ||
			issued.HttpOnly != cleared.HttpOnly ||
			issued.SameSite != cleared.SameSite ||
			issued.Domain != cleared.Domain {
			t.Errorf("secure=%v: cleared cookie attributes differ from issued:\n issued=%+v\ncleared=%+v",
				secure, issued, cleared)
		}
		if cleared.MaxAge >= 0 {
			t.Errorf("secure=%v: cleared MaxAge = %d, want negative", secure, cleared.MaxAge)
		}
		if cleared.Value != "" {
			t.Errorf("secure=%v: cleared value = %q, want empty", secure, cleared.Value)
		}
	}
}

func TestGenerateKeyIsUsable(t *testing.T) {
	k, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	if len(k) < MinKeyLength {
		t.Fatalf("generated key is %d characters, below the %d minimum it must satisfy",
			len(k), MinKeyLength)
	}
	if _, err := DeriveKey(k); err != nil {
		t.Fatalf("DeriveKey on a generated key: %v", err)
	}

	other, err := GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	if k == other {
		t.Fatal("GenerateKey returned the same key twice")
	}
}
