// Package domain defines the canonical hostname identity used throughout
// Hermes.
//
// A single Hermes process serves many subdomains. Nearly every subsystem —
// storage, search, sessions, database schemas — must be scoped to exactly one
// of them, and a mistake in that scoping leaks one tenant's documents into
// another. Passing hostnames as bare strings makes those mistakes invisible:
// nothing distinguishes a normalized hostname from a raw `Host` header, a
// config label, or a user-supplied value.
//
// Name closes that gap. It is a struct with an unexported field, so it cannot
// be produced by conversion — only Parse (or MustParse) can create a non-zero
// Name, and Parse is the single place normalization and validation happen.
// Any function that accepts a Name is therefore guaranteed a canonical value.
package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"unicode/utf8"

	"golang.org/x/net/idna"
)

const (
	// maxNameLength is the maximum length of a DNS name in presentation form.
	maxNameLength = 253

	// maxLabelLength is the maximum length of a single DNS label.
	maxLabelLength = 63

	// maxSchemaLength is PostgreSQL's identifier limit (NAMEDATALEN - 1).
	maxSchemaLength = 63

	// schemaPrefix namespaces per-domain schemas so they never collide with
	// PostgreSQL's own schemas ("public", "pg_catalog") or with any shared
	// Hermes schema.
	schemaPrefix = "site_"

	// schemaHashLength is the number of hex characters appended when a
	// readable schema name would exceed maxSchemaLength. 12 hex characters is
	// 48 bits, which makes an accidental collision negligible.
	schemaHashLength = 12
)

// Name is a canonical DNS hostname identifying a Hermes site.
//
// The zero Name is invalid and is reported by IsZero. Name is comparable, so
// it may be used directly as a map key.
type Name struct {
	// name is the canonical form: lowercase, ASCII (punycode for IDNs), no
	// port, no trailing dot.
	name string
}

// Parse converts a raw hostname into its canonical form.
//
// It accepts the shapes a hostname actually arrives in — an HTTP `Host` header
// with a port, a config label, a value with mixed case or a trailing dot — and
// rejects anything that is not a usable DNS name. Unicode is converted to
// punycode so that a domain has exactly one canonical representation.
func Parse(raw string) (Name, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return Name{}, fmt.Errorf("domain: empty hostname")
	}

	s, err := stripPort(s)
	if err != nil {
		return Name{}, err
	}

	// A trailing dot denotes the DNS root and is not part of the identity:
	// "example.com." and "example.com" are the same host.
	s = strings.TrimSuffix(s, ".")
	if s == "" {
		return Name{}, fmt.Errorf("domain: hostname is only a root label")
	}

	// Invalid UTF-8 must be rejected before IDNA sees it. idna.Lookup silently
	// maps undecodable bytes to U+FFFD and punycode-encodes the result, so
	// "\xff" would otherwise canonicalize to "xn--zn7c" — a value that does
	// not itself re-parse, breaking idempotency.
	if !utf8.ValidString(s) {
		return Name{}, fmt.Errorf("domain: hostname is not valid UTF-8")
	}

	// idna.Lookup applies the rules for looking a name up (as opposed to
	// registering one): it maps to lowercase, converts Unicode to punycode,
	// and rejects names that cannot be resolved.
	ascii, err := idna.Lookup.ToASCII(s)
	if err != nil {
		return Name{}, fmt.Errorf("domain: %q is not a valid hostname: %w", raw, err)
	}

	ascii = strings.ToLower(ascii)
	if err := validate(ascii); err != nil {
		return Name{}, err
	}

	// Canonicalization must be a fixed point. Everything downstream — schema
	// names, storage paths, session binding — assumes one domain has exactly
	// one representation, so reject any input whose normalized form would
	// itself normalize differently rather than let the discrepancy through.
	if stable, err := idna.Lookup.ToASCII(ascii); err != nil || stable != ascii {
		return Name{}, fmt.Errorf("domain: %q does not canonicalize to a stable form", raw)
	}

	return Name{name: ascii}, nil
}

// MustParse is Parse but panics on error. Use it only for compile-time
// constants and test fixtures, never for input that can come from a request or
// a config file.
func MustParse(raw string) Name {
	n, err := Parse(raw)
	if err != nil {
		panic(err)
	}

	return n
}

// stripPort removes a ":port" suffix and unwraps a bracketed IPv6 literal.
//
// It deliberately does not use net.SplitHostPort, which errors on the common
// case of a host with no port at all.
func stripPort(s string) (string, error) {
	// Bracketed form, e.g. "[::1]:8000" or "[::1]".
	if strings.HasPrefix(s, "[") {
		end := strings.Index(s, "]")
		if end < 0 {
			return "", fmt.Errorf("domain: unterminated IPv6 literal in %q", s)
		}

		return s[1:end], nil
	}

	// An unbracketed value with more than one colon is a bare IPv6 literal;
	// the colons are part of the address, not a port separator.
	if strings.Count(s, ":") > 1 {
		return s, nil
	}

	i := strings.LastIndex(s, ":")
	if i < 0 {
		return s, nil
	}

	// Only strip the suffix when it is actually a port. A Host header's port
	// is always numeric, so requiring digits keeps "https://docs.example.com"
	// from being truncated to the scheme, which would otherwise parse as the
	// perfectly valid single-label host "https".
	if !isNumeric(s[i+1:]) {
		return "", fmt.Errorf("domain: %q has a non-numeric port suffix %q", s, s[i+1:])
	}

	return s[:i], nil
}

// isNumeric reports whether s is one or more ASCII digits.
func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}

	return true
}

// validate checks DNS structural rules that idna.Lookup does not enforce.
func validate(s string) error {
	if len(s) > maxNameLength {
		return fmt.Errorf("domain: hostname exceeds %d characters: %d", maxNameLength, len(s))
	}

	labels := strings.Split(s, ".")
	for _, label := range labels {
		if label == "" {
			return fmt.Errorf("domain: hostname %q contains an empty label", s)
		}
		if len(label) > maxLabelLength {
			return fmt.Errorf("domain: label %q exceeds %d characters", label, maxLabelLength)
		}
		if strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return fmt.Errorf("domain: label %q may not begin or end with a hyphen", label)
		}
		for _, r := range label {
			if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
				continue
			}

			return fmt.Errorf("domain: label %q contains invalid character %q", label, r)
		}
	}

	return nil
}

// String returns the canonical hostname, or "" for the zero Name.
func (n Name) String() string {
	return n.name
}

// IsZero reports whether n is the invalid zero value.
func (n Name) IsZero() bool {
	return n.name == ""
}

// PathSegment returns the directory name for this domain's on-disk storage.
//
// Parse guarantees the canonical form contains only lowercase letters, digits,
// hyphens and dots, with no empty labels, so it is already safe to use as a
// single path element: it can be neither "." nor ".." nor contain a separator.
func (n Name) PathSegment() string {
	return n.name
}

// SchemaName returns the PostgreSQL schema holding this domain's data.
//
// Dots become "_" and hyphens become "__". That mapping is injective, so
// distinct domains always yield distinct schema names — "a.b.com" becomes
// "site_a_b_com" while "a-b.com" becomes "site_a__b_com". A plain
// dot-and-hyphen-to-underscore substitution would map both to the same schema
// and silently merge two tenants.
//
// If the readable form would exceed PostgreSQL's 63-character identifier
// limit, it is truncated and a hash of the full canonical name is appended,
// which keeps the result unique and deterministic.
func (n Name) SchemaName() string {
	if n.IsZero() {
		return ""
	}

	var b strings.Builder
	b.WriteString(schemaPrefix)
	for _, r := range n.name {
		switch r {
		case '.':
			b.WriteString("_")
		case '-':
			b.WriteString("__")
		default:
			b.WriteRune(r)
		}
	}

	schema := b.String()
	if len(schema) <= maxSchemaLength {
		return schema
	}

	sum := sha256.Sum256([]byte(n.name))
	suffix := "_" + hex.EncodeToString(sum[:])[:schemaHashLength]
	keep := maxSchemaLength - len(suffix)

	return schema[:keep] + suffix
}

// MarshalText implements encoding.TextMarshaler so a Name round-trips through
// JSON and other text encodings as its canonical string.
func (n Name) MarshalText() ([]byte, error) {
	return []byte(n.name), nil
}

// UnmarshalText implements encoding.TextUnmarshaler, routing decoding through
// Parse so a decoded Name is canonical like any other.
func (n *Name) UnmarshalText(text []byte) error {
	parsed, err := Parse(string(text))
	if err != nil {
		return err
	}
	*n = parsed

	return nil
}
