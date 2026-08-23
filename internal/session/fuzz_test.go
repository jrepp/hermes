package session

import (
	"strings"
	"testing"

	"github.com/hashicorp-forge/hermes/pkg/domain"
)

// FuzzVerify throws arbitrary bytes at the one function that reads
// attacker-controlled input.
//
// The cookie value arrives from the client, and Verify splits it, base64
// decodes two parts, checks a MAC, and unmarshals JSON. Any of those steps can
// be handed something malformed, and a panic in an unauthenticated code path
// is a denial of service.
//
// The properties checked are the ones that hold for every input:
//
//   - Nothing panics.
//   - A result is either a token or an error, never both.
//   - Anything that verifies carries an email and the site it was checked
//     against -- so a forgery cannot arrive as an authenticated nobody, or as
//     somebody on another tenant.
//   - Verification is deterministic, so acceptance cannot depend on timing or
//     on state left behind by an earlier call.
//   - Flipping any single byte of something that verified makes it stop
//     verifying. That is the unforgeability property stated in a way a fuzzer
//     can actually test.
//
// Note what is *not* asserted: that only tokens produced by Issue verify. The
// obvious way to write that -- compare against a token minted in the setup --
// does not work, because the fuzzing workers re-run the setup and mint their
// own, while the seed corpus still holds the original. The check would fail on
// its own seed.
func FuzzVerify(f *testing.F) {
	signer := testSigner(f)
	site := domain.MustParse("docs.jrepp.com")

	valid, err := signer.Issue("user@jrepp.com", site)
	if err != nil {
		f.Fatalf("Issue: %v", err)
	}

	// Seeds: the real thing, its pieces, and the shapes a hand-written forgery
	// takes.
	f.Add(valid)
	f.Add("")
	f.Add("hs1")
	f.Add("hs1.")
	f.Add("hs1..")
	f.Add("user@jrepp.com")
	f.Add("hs1.eyJlIjoiYSJ9.")
	f.Add(strings.Repeat("A", 5000))
	f.Add("hs1.!!!!.!!!!")
	f.Add(valid[:len(valid)-1])
	f.Add(strings.ToUpper(valid))
	if i := strings.LastIndexByte(valid, '.'); i > 0 {
		f.Add(valid[:i])
		f.Add(valid[i+1:])
		f.Add(valid + "." + valid[i+1:])
	}

	f.Fuzz(func(t *testing.T, raw string) {
		tok, err := signer.Verify(raw, site)

		if err != nil {
			if tok != nil {
				t.Fatalf("Verify returned both a token and an error for %q", raw)
			}

			return
		}

		if tok == nil {
			t.Fatalf("Verify returned neither a token nor an error for %q", raw)
		}
		if tok.Email == "" {
			t.Fatalf("%q verified as a session with no email", raw)
		}
		if tok.Domain != site {
			t.Fatalf("%q verified against %v but carries %v", raw, site, tok.Domain)
		}

		// Deterministic: acceptance must not depend on timing or on state.
		if _, err := signer.Verify(raw, site); err != nil {
			t.Fatalf("%q verified once and then failed: %v", raw, err)
		}

		// Unforgeable: no single-byte edit of an accepted token may still be
		// accepted.
		for i := 0; i < len(raw); i++ {
			edited := []byte(raw)
			edited[i] ^= 0x01
			if _, err := signer.Verify(string(edited), site); err == nil {
				t.Fatalf("editing byte %d of an accepted token left it accepted", i)
			}
		}
	})
}

// FuzzVerifyAcrossSites checks the domain binding survives arbitrary input:
// whatever is thrown at it, a token must never verify against a site it was
// not issued for.
func FuzzVerifyAcrossSites(f *testing.F) {
	signer := testSigner(f)
	docs := domain.MustParse("docs.jrepp.com")
	notes := domain.MustParse("notes.jrepp.com")

	issued, err := signer.Issue("user@jrepp.com", docs)
	if err != nil {
		f.Fatalf("Issue: %v", err)
	}
	f.Add(issued)
	f.Add("")
	f.Add("hs1.x.y")

	f.Fuzz(func(t *testing.T, raw string) {
		if _, err := signer.Verify(raw, notes); err == nil {
			t.Fatalf("%q verified against notes.jrepp.com", raw)
		}
	})
}
