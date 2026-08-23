// Package session issues and verifies Hermes session cookies.
//
// The cookie value is a signed, self-contained token rather than an opaque
// database handle, so no server-side session store is required. Its security
// rests on two properties:
//
//   - The value is authenticated with HMAC-SHA256 over the exact bytes that
//     were transmitted. A cookie the server did not issue cannot be forged
//     without the key, and a cookie it did issue cannot be edited.
//   - The token names the site it was issued for, and verification requires
//     that name to match the site handling the request.
//
// The second property is not redundant with the host-only cookie attribute.
// Cookies do not honour the same-origin policy: any host under jrepp.com --
// including one an attacker controls, or one Hermes does not serve -- may set
// a cookie scoped to `Domain=jrepp.com`, and the browser will then send it to
// every sibling subdomain. Without the embedded site name, a token minted for
// one tenant would authenticate its bearer on all of them.
package session

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/hashicorp-forge/hermes/pkg/domain"
)

const (
	// CookieName is the name of the Hermes session cookie.
	CookieName = "hermes_session"

	// DefaultTTL is how long an issued session stays valid when the
	// configuration does not say otherwise.
	DefaultTTL = 7 * 24 * time.Hour

	// MinKeyLength is the shortest accepted signing key. HMAC-SHA256 gains no
	// strength from keys longer than its 32-byte block-derived output, and
	// shorter ones are guessable, so 32 bytes is both the floor and the
	// recommended size.
	MinKeyLength = 32

	// tokenVersion prefixes every token and is covered by the signature. A
	// future format can be introduced without a verifier ever being tricked
	// into interpreting a v1 payload under v2 rules.
	tokenVersion = "hs1"

	nonceLength = 16

	// maxTokenLength bounds work done before the signature is checked. Browsers
	// cap a cookie at roughly 4096 bytes; anything larger is not something we
	// issued.
	maxTokenLength = 4096
)

// Verification failures. Callers should treat all of them as "not
// authenticated" and must not report which one occurred to the client: the
// distinction between a bad signature and an expired token is useful to an
// attacker probing for a valid key.
var (
	ErrMalformed    = errors.New("session: malformed token")
	ErrBadSignature = errors.New("session: signature does not verify")
	ErrExpired      = errors.New("session: token expired")
	ErrWrongDomain  = errors.New("session: token was issued for a different site")
	ErrNoEmail      = errors.New("session: token carries no email")
)

// Token is the verified content of a session cookie.
type Token struct {
	IssuedAt  time.Time
	ExpiresAt time.Time
	Email     string
	Nonce     string
	Domain    domain.Name
}

// payload is the wire form. Field names are short because the encoded payload
// travels on every request.
type payload struct {
	Email     string `json:"e"`
	Domain    string `json:"d,omitempty"`
	Nonce     string `json:"n"`
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
}

// Signer issues and verifies session tokens. It is safe for concurrent use.
type Signer struct {
	now func() time.Time
	key []byte
	ttl time.Duration
}

// NewSigner returns a Signer using key for HMAC-SHA256.
//
// The key must be at least MinKeyLength bytes. A ttl of zero selects
// DefaultTTL.
func NewSigner(key []byte, ttl time.Duration) (*Signer, error) {
	if len(key) < MinKeyLength {
		return nil, fmt.Errorf(
			"session: signing key is %d bytes, need at least %d",
			len(key), MinKeyLength)
	}
	if ttl < 0 {
		return nil, fmt.Errorf("session: ttl must not be negative, got %s", ttl)
	}
	if ttl == 0 {
		ttl = DefaultTTL
	}

	// Copy so a caller mutating its buffer -- zeroing a secret read from the
	// environment, say -- cannot silently invalidate every live session.
	k := make([]byte, len(key))
	copy(k, key)

	return &Signer{key: k, ttl: ttl, now: time.Now}, nil
}

// TTL returns how long newly issued tokens remain valid.
func (s *Signer) TTL() time.Duration { return s.ttl }

// Issue returns a signed token binding email to d.
//
// A zero d is permitted and means "not site-scoped": a single-site deployment
// that has configured no site blocks. Verify enforces the same distinction, so
// a token issued without a site never validates against a request that has
// one, and vice versa.
func (s *Signer) Issue(email string, d domain.Name) (string, error) {
	if email == "" {
		return "", ErrNoEmail
	}

	nonce := make([]byte, nonceLength)
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("session: generating nonce: %w", err)
	}

	now := s.now().UTC()
	p := payload{
		Email:     email,
		Domain:    d.String(),
		Nonce:     base64.RawURLEncoding.EncodeToString(nonce),
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(s.ttl).Unix(),
	}

	body, err := json.Marshal(p)
	if err != nil {
		return "", fmt.Errorf("session: encoding payload: %w", err)
	}

	signed := tokenVersion + "." + base64.RawURLEncoding.EncodeToString(body)

	return signed + "." + base64.RawURLEncoding.EncodeToString(s.mac(signed)), nil
}

// Verify checks raw and returns its content, requiring that the token was
// issued for site d.
//
// The signature is checked over the received bytes before the payload is
// decoded, so no attacker-controlled structure is ever interpreted.
func (s *Signer) Verify(raw string, d domain.Name) (*Token, error) {
	if raw == "" || len(raw) > maxTokenLength {
		return nil, ErrMalformed
	}

	// The signature covers everything up to the final separator, so split from
	// the right: that keeps the boundary unambiguous no matter what the payload
	// encoding contains.
	i := strings.LastIndexByte(raw, '.')
	if i < 0 {
		return nil, ErrMalformed
	}
	signed, sig := raw[:i], raw[i+1:]

	if !strings.HasPrefix(signed, tokenVersion+".") {
		return nil, ErrMalformed
	}

	// Strict rejects a non-canonical encoding: the last base64 character of a
	// 32-byte MAC carries two bits that encode nothing, and Go's decoder
	// ignores them by default. Without this, several distinct cookie strings
	// decode to the same MAC and all verify as one session -- so the token is
	// malleable, and anything that ever keys on the string itself (a
	// revocation list, a session cache, an audit trail, a rate limiter) can be
	// handed unlimited distinct identifiers for a single session.
	//
	// Found by FuzzVerify, which flips one byte of an accepted token and
	// requires it to stop being accepted.
	got, err := base64.RawURLEncoding.Strict().DecodeString(sig)
	if err != nil {
		return nil, ErrMalformed
	}
	if !hmac.Equal(got, s.mac(signed)) {
		return nil, ErrBadSignature
	}

	// Past this point the bytes are known to be ours.
	// Strict here too, for the same reason: the payload must have exactly one
	// valid encoding, or one session has many equally valid cookie strings.
	body, err := base64.RawURLEncoding.Strict().DecodeString(signed[len(tokenVersion)+1:])
	if err != nil {
		return nil, ErrMalformed
	}
	var p payload
	if err := json.Unmarshal(body, &p); err != nil {
		return nil, ErrMalformed
	}
	if p.Email == "" {
		return nil, ErrNoEmail
	}

	// Reparse rather than trusting the stored spelling. A token minted by an
	// older build, or by a build whose canonicalization rules have since
	// changed, must not resolve to a domain that Parse would reject today.
	var tokenDomain domain.Name
	if p.Domain != "" {
		tokenDomain, err = domain.Parse(p.Domain)
		if err != nil {
			return nil, ErrMalformed
		}
	}
	if tokenDomain != d {
		return nil, ErrWrongDomain
	}

	now := s.now()
	if now.Unix() >= p.ExpiresAt {
		return nil, ErrExpired
	}

	return &Token{
		Email:     p.Email,
		Domain:    tokenDomain,
		Nonce:     p.Nonce,
		IssuedAt:  time.Unix(p.IssuedAt, 0).UTC(),
		ExpiresAt: time.Unix(p.ExpiresAt, 0).UTC(),
	}, nil
}

func (s *Signer) mac(signed string) []byte {
	m := hmac.New(sha256.New, s.key)
	m.Write([]byte(signed))

	return m.Sum(nil)
}

// Cookie returns the cookie carrying value.
//
// It deliberately sets no Domain attribute, which makes the cookie host-only:
// a session for docs.jrepp.com is not transmitted to notes.jrepp.com at all.
// The domain embedded in the token covers the case this cannot -- a cookie
// scoped to the parent domain by some other host under it.
func (s *Signer) Cookie(value string, secure bool) *http.Cookie {
	return &http.Cookie{
		Name:     CookieName,
		Value:    value,
		Path:     "/",
		MaxAge:   int(s.ttl / time.Second),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	}
}

// ClearCookie returns a cookie that expires the session.
//
// The attributes other than MaxAge must match those used at issue time or the
// browser treats this as a different cookie and leaves the original in place.
func ClearCookie(secure bool) *http.Cookie {
	return &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	}
}

// GenerateKey returns a new random signing key of the recommended length,
// encoded for use in configuration.
func GenerateKey() (string, error) {
	b := make([]byte, MinKeyLength)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("session: generating key: %w", err)
	}

	return base64.RawStdEncoding.EncodeToString(b), nil
}
