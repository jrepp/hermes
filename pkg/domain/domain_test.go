package domain_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/hashicorp-forge/hermes/pkg/domain"
)

func TestParseCanonicalizes(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		in   string
		want string
	}{
		"already canonical":  {"docs.jrepp.com", "docs.jrepp.com"},
		"uppercase":          {"DOCS.JREPP.COM", "docs.jrepp.com"},
		"mixed case":         {"Docs.Jrepp.Com", "docs.jrepp.com"},
		"trailing dot":       {"docs.jrepp.com.", "docs.jrepp.com"},
		"explicit port":      {"docs.jrepp.com:8000", "docs.jrepp.com"},
		"surrounding space":  {"  docs.jrepp.com  ", "docs.jrepp.com"},
		"port and case":      {"DOCS.jrepp.com:443", "docs.jrepp.com"},
		"single label":       {"localhost", "localhost"},
		"localhost withport": {"localhost:4200", "localhost"},
		"hyphenated label":   {"my-docs.jrepp.com", "my-docs.jrepp.com"},
		"digits":             {"s3.jrepp.com", "s3.jrepp.com"},
		"unicode to puny":    {"münchen.example", "xn--mnchen-3ya.example"},
		"uppercase unicode":  {"MÜNCHEN.example", "xn--mnchen-3ya.example"},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, err := domain.Parse(c.in)
			if err != nil {
				t.Fatalf("Parse(%q) returned error: %v", c.in, err)
			}
			if got.String() != c.want {
				t.Errorf("Parse(%q) = %q, want %q", c.in, got.String(), c.want)
			}
		})
	}
}

func TestParseIsIdempotent(t *testing.T) {
	t.Parallel()

	// Canonicalization that is not idempotent would let the same host produce
	// two different storage namespaces depending on how many times it had been
	// round-tripped through config.
	for _, in := range []string{"DOCS.JREPP.COM:8000", "münchen.example.", "localhost"} {
		once, err := domain.Parse(in)
		if err != nil {
			t.Fatalf("Parse(%q): %v", in, err)
		}
		twice, err := domain.Parse(once.String())
		if err != nil {
			t.Fatalf("Parse(%q): %v", once.String(), err)
		}
		if once != twice {
			t.Errorf("Parse not idempotent for %q: %q then %q", in, once, twice)
		}
	}
}

func TestParseRejects(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"empty":             "",
		"only whitespace":   "   ",
		"only root":         ".",
		"empty leading":     ".jrepp.com",
		"double dot":        "docs..jrepp.com",
		"leading hyphen":    "-docs.jrepp.com",
		"trailing hyphen":   "docs-.jrepp.com",
		"underscore":        "my_docs.jrepp.com",
		"slash":             "docs.jrepp.com/path",
		"space inside":      "docs jrepp.com",
		"scheme included":   "https://docs.jrepp.com",
		"at sign":           "user@docs.jrepp.com",
		"label too long":    strings.Repeat("a", 64) + ".com",
		"name too long":     strings.Repeat("a.", 200) + "com",
		"unterminated ipv6": "[::1",
		"null byte":         "docs\x00.jrepp.com",
	}

	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if got, err := domain.Parse(in); err == nil {
				t.Errorf("Parse(%q) unexpectedly succeeded, returning %q", in, got)
			}
		})
	}
}

func TestZeroName(t *testing.T) {
	t.Parallel()

	var zero domain.Name
	if !zero.IsZero() {
		t.Error("zero Name should report IsZero")
	}
	if zero.String() != "" {
		t.Errorf("zero Name String() = %q, want empty", zero.String())
	}
	if zero.SchemaName() != "" {
		t.Errorf("zero Name SchemaName() = %q, want empty", zero.SchemaName())
	}

	parsed := domain.MustParse("docs.jrepp.com")
	if parsed.IsZero() {
		t.Error("parsed Name should not report IsZero")
	}
}

func TestSchemaNameIsInjective(t *testing.T) {
	t.Parallel()

	// A naive "replace . and - with _" mapping collapses these two distinct
	// domains onto one schema, silently merging two tenants' data.
	a := domain.MustParse("a.b.com").SchemaName()
	b := domain.MustParse("a-b.com").SchemaName()
	if a == b {
		t.Fatalf("distinct domains share schema %q", a)
	}
	if want := "site_a_b_com"; a != want {
		t.Errorf("a.b.com schema = %q, want %q", a, want)
	}
	if want := "site_a__b_com"; b != want {
		t.Errorf("a-b.com schema = %q, want %q", b, want)
	}
}

func TestSchemaNameIsValidPostgresIdentifier(t *testing.T) {
	t.Parallel()

	longest := strings.Repeat("a", 63) + "." + strings.Repeat("b", 63) + ".com"
	inputs := []string{
		"localhost",
		"docs.jrepp.com",
		"a-very-long-subdomain-name.staging.internal.jrepp.com",
		"xn--mnchen-3ya.example",
		longest,
	}

	seen := map[string]string{}
	for _, in := range inputs {
		n := domain.MustParse(in)
		schema := n.SchemaName()

		if len(schema) > 63 {
			t.Errorf("%s: schema %q is %d chars, exceeds PostgreSQL's 63", in, schema, len(schema))
		}
		if !strings.HasPrefix(schema, "site_") {
			t.Errorf("%s: schema %q lacks the site_ prefix", in, schema)
		}
		for _, r := range schema {
			isLower := r >= 'a' && r <= 'z'
			isDigit := r >= '0' && r <= '9'
			if !isLower && !isDigit && r != '_' {
				t.Errorf("%s: schema %q contains invalid identifier character %q", in, schema, r)
			}
		}
		if prev, dup := seen[schema]; dup {
			t.Errorf("schema collision: %q and %q both map to %q", prev, in, schema)
		}
		seen[schema] = in
	}
}

func TestSchemaNameIsDeterministic(t *testing.T) {
	t.Parallel()

	// Truncated-and-hashed names must be stable across processes, or a
	// restart would point the server at a different schema than the one the
	// migrations created.
	long := strings.Repeat("a", 60) + "." + strings.Repeat("b", 60) + ".example.com"
	n := domain.MustParse(long)
	if n.SchemaName() != n.SchemaName() {
		t.Fatal("SchemaName is not deterministic within a process")
	}
	if got := domain.MustParse(long).SchemaName(); got != n.SchemaName() {
		t.Fatalf("SchemaName differs across values: %q vs %q", got, n.SchemaName())
	}
}

func TestPathSegmentIsSafe(t *testing.T) {
	t.Parallel()

	for _, in := range []string{"docs.jrepp.com", "localhost", "a-b.example"} {
		seg := domain.MustParse(in).PathSegment()
		if seg == "." || seg == ".." {
			t.Errorf("%s: path segment %q traverses directories", in, seg)
		}
		if strings.ContainsAny(seg, `/\`) {
			t.Errorf("%s: path segment %q contains a separator", in, seg)
		}
	}
}

func TestNameIsUsableAsMapKey(t *testing.T) {
	t.Parallel()

	m := map[domain.Name]string{
		domain.MustParse("a.example"): "first",
		domain.MustParse("b.example"): "second",
	}

	// Equality must follow the canonical form, not the input spelling.
	if got := m[domain.MustParse("A.EXAMPLE:8000")]; got != "first" {
		t.Errorf("lookup by non-canonical spelling = %q, want %q", got, "first")
	}
}

func TestTextRoundTrip(t *testing.T) {
	t.Parallel()

	type payload struct {
		Site domain.Name `json:"site"`
	}

	in := payload{Site: domain.MustParse("docs.jrepp.com")}
	encoded, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if want := `{"site":"docs.jrepp.com"}`; string(encoded) != want {
		t.Errorf("Marshal = %s, want %s", encoded, want)
	}

	var out payload
	if err := json.Unmarshal(encoded, &out); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if out.Site != in.Site {
		t.Errorf("round trip changed value: %q -> %q", in.Site, out.Site)
	}
}

func TestUnmarshalTextCanonicalizes(t *testing.T) {
	t.Parallel()

	// Decoding must go through Parse; otherwise a config or API payload could
	// introduce a non-canonical Name that bypasses every downstream guarantee.
	var n domain.Name
	if err := n.UnmarshalText([]byte("DOCS.JREPP.COM:8000")); err != nil {
		t.Fatalf("UnmarshalText: %v", err)
	}
	if want := "docs.jrepp.com"; n.String() != want {
		t.Errorf("UnmarshalText produced %q, want %q", n, want)
	}

	if err := n.UnmarshalText([]byte("not a hostname")); err == nil {
		t.Error("UnmarshalText accepted an invalid hostname")
	}
}

func TestContextRoundTrip(t *testing.T) {
	t.Parallel()

	want := domain.MustParse("docs.jrepp.com")
	ctx := domain.NewContext(context.Background(), want)

	got, ok := domain.FromContext(ctx)
	if !ok {
		t.Fatal("FromContext reported no domain")
	}
	if got != want {
		t.Errorf("FromContext = %q, want %q", got, want)
	}

	if _, err := domain.RequireFromContext(ctx); err != nil {
		t.Errorf("RequireFromContext: %v", err)
	}
}

func TestContextAbsent(t *testing.T) {
	t.Parallel()

	// A missing domain must be reported, never defaulted: silently choosing a
	// domain here is what would write one tenant's data into another's space.
	if _, ok := domain.FromContext(context.Background()); ok {
		t.Error("FromContext found a domain in a bare context")
	}
	if _, err := domain.RequireFromContext(context.Background()); err == nil {
		t.Error("RequireFromContext accepted a context with no domain")
	}

	// A zero Name stored in a context is treated as absent rather than as a
	// valid domain whose name happens to be empty.
	ctx := domain.NewContext(context.Background(), domain.Name{})
	if _, ok := domain.FromContext(ctx); ok {
		t.Error("FromContext accepted a zero Name")
	}
}

func FuzzParse(f *testing.F) {
	seeds := []string{
		"docs.jrepp.com", "DOCS.JREPP.COM:8000", "localhost", "münchen.example",
		"", ".", "..", "-a.com", "a-.com", "[::1]:8000", strings.Repeat("a", 300),
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, raw string) {
		n, err := domain.Parse(raw)
		if err != nil {
			return
		}

		// Every value Parse accepts must satisfy the invariants the rest of
		// the program relies on.
		if n.IsZero() {
			t.Fatalf("Parse(%q) succeeded but produced the zero Name", raw)
		}

		again, err := domain.Parse(n.String())
		if err != nil {
			t.Fatalf("Parse(%q) produced %q, which does not re-parse: %v", raw, n, err)
		}
		if again != n {
			t.Fatalf("Parse(%q) is not idempotent: %q then %q", raw, n, again)
		}

		schema := n.SchemaName()
		if len(schema) > 63 {
			t.Fatalf("Parse(%q) -> schema %q exceeds 63 characters", raw, schema)
		}
		for _, r := range schema {
			isLower := r >= 'a' && r <= 'z'
			isDigit := r >= '0' && r <= '9'
			if !isLower && !isDigit && r != '_' {
				t.Fatalf("Parse(%q) -> schema %q has invalid character %q", raw, schema, r)
			}
		}

		seg := n.PathSegment()
		if seg == "" || seg == "." || seg == ".." || strings.ContainsAny(seg, `/\`) {
			t.Fatalf("Parse(%q) -> unsafe path segment %q", raw, seg)
		}
	})
}

// TestSlugIsInjective is the property everything downstream depends on. The
// slug keys PostgreSQL schemas and search index names, so two domains sharing
// one would mean two tenants sharing storage.
func TestSlugIsInjective(t *testing.T) {
	t.Parallel()

	// Pairs chosen to collide under a naive "replace . and - with _".
	hosts := []string{
		"a.b.com", "a-b.com", "a.b-c.com", "a-b.c.com", "a--b.com",
		"docs.jrepp.com", "docs-jrepp.com", "docs.jrepp.co.m",
		"x.y.z.com", "x-y.z.com", "x.y-z.com",
	}

	seen := make(map[string]string, len(hosts))
	for _, host := range hosts {
		slug := domain.MustParse(host).Slug()
		if prev, dup := seen[slug]; dup {
			t.Errorf("%q and %q both slugify to %q", prev, host, slug)
		}
		seen[slug] = host
	}
}

func TestSlugIsIdentifierSafe(t *testing.T) {
	t.Parallel()

	for _, host := range []string{
		"docs.jrepp.com", "a-b.example.co.uk", "xn--bcher-kva.example.com",
		"1.2.3.4", strings.Repeat("a", 60) + ".example.com",
	} {
		slug := domain.MustParse(host).Slug()
		if slug == "" {
			t.Errorf("%q slugified to the empty string", host)
			continue
		}
		if slug[0] >= '0' && slug[0] <= '9' {
			// Leading digits are fine for a search index but not for an
			// unquoted SQL identifier, which is why SchemaName adds a prefix.
			if strings.HasPrefix(domain.MustParse(host).SchemaName(), "site_") == false {
				t.Errorf("%q has a digit-leading slug and no schema prefix", host)
			}
		}
		for i := 0; i < len(slug); i++ {
			c := slug[i]
			ok := (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_'
			if !ok {
				t.Errorf("%q slugified to %q, which contains %q", host, slug, string(c))
				break
			}
		}
	}
}

// TestSchemaNameIsSlugPlusPrefix pins the relationship, so the two encodings
// cannot drift apart and start disagreeing about which domain owns what.
func TestSchemaNameIsSlugPlusPrefix(t *testing.T) {
	t.Parallel()

	for _, host := range []string{
		// Long enough to force truncation, using valid labels: a single label
		// may not exceed 63 characters, so length comes from label count.
		"docs.jrepp.com", "a-b.com", strings.Repeat("abcdefghij.", 20) + "example.com",
	} {
		n := domain.MustParse(host)
		if got, want := n.SchemaName(), "site_"+n.Slug(); got != want {
			t.Errorf("%q: SchemaName = %q, want %q", host, got, want)
		}
		if len(n.SchemaName()) > 63 {
			t.Errorf("%q: schema name is %d bytes, over the identifier limit",
				host, len(n.SchemaName()))
		}
	}
}

func TestSearchNamespaceMatchesSlug(t *testing.T) {
	t.Parallel()

	n := domain.MustParse("docs.jrepp.com")
	if n.SearchNamespace() != n.Slug() {
		t.Errorf("SearchNamespace = %q, Slug = %q; they must be the same encoding",
			n.SearchNamespace(), n.Slug())
	}
}

func TestZeroNameHasNoSlug(t *testing.T) {
	t.Parallel()

	var zero domain.Name
	if zero.Slug() != "" {
		t.Errorf("zero Slug = %q, want empty", zero.Slug())
	}
	if zero.SchemaName() != "" {
		t.Errorf("zero SchemaName = %q, want empty", zero.SchemaName())
	}
}
