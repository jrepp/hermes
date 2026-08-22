package session

import (
	"bytes"
	"testing"
	"time"
)

func TestDeriveKeyIsDeterministic(t *testing.T) {
	a, err := DeriveKey(testSecret)
	if err != nil {
		t.Fatalf("DeriveKey: %v", err)
	}
	b, err := DeriveKey(testSecret)
	if err != nil {
		t.Fatalf("DeriveKey: %v", err)
	}

	// Sessions must survive a restart, which they only do if the same secret
	// always yields the same key.
	if !bytes.Equal(a, b) {
		t.Fatal("DeriveKey is not deterministic")
	}
	if len(a) != MinKeyLength {
		t.Fatalf("derived key is %d bytes, want %d", len(a), MinKeyLength)
	}

	c, err := DeriveKey(testSecret + "!")
	if err != nil {
		t.Fatalf("DeriveKey: %v", err)
	}
	if bytes.Equal(a, c) {
		t.Fatal("different secrets derived the same key")
	}
}

func TestDeriveKeyRejectsShortSecret(t *testing.T) {
	if _, err := DeriveKey("too-short"); err == nil {
		t.Fatal("DeriveKey accepted a 9-character secret")
	}
}

func TestResolveUsesConfiguredSecret(t *testing.T) {
	requireKeyEnvUnset(t)

	opts, err := Resolve(testSecret, "")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if opts.Ephemeral {
		t.Error("Resolve reported ephemeral despite a configured secret")
	}
	if opts.Secret != testSecret {
		t.Errorf("secret = %q, want the configured one", opts.Secret)
	}
	if opts.TTL != DefaultTTL {
		t.Errorf("ttl = %v, want %v", opts.TTL, DefaultTTL)
	}
}

func TestResolveEnvOverridesConfig(t *testing.T) {
	t.Setenv(KeyEnvVar, "an-environment-supplied-secret-long-enough")

	opts, err := Resolve(testSecret, "")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if opts.Secret != "an-environment-supplied-secret-long-enough" {
		t.Errorf("secret = %q, want the environment value", opts.Secret)
	}
}

// TestResolveRejectsPlaceholders is the point of isPlaceholderSecret: the
// committed example carries "change-me", and an operator who ships it
// unchanged would be signing with a secret published in the repository.
func TestResolveRejectsPlaceholders(t *testing.T) {
	for _, secret := range []string{
		"change-me",
		"CHANGE-ME-to-a-real-32-character-secret",
		"changeme-changeme-changeme-changeme",
		"replace-me-with-something-random-please",
	} {
		t.Setenv(KeyEnvVar, secret)

		opts, err := Resolve("", "")
		if err != nil {
			t.Fatalf("Resolve(%q): %v", secret, err)
		}
		if !opts.Ephemeral {
			t.Errorf("Resolve(%q) accepted a placeholder as a signing secret", secret)
		}
		if opts.Secret == secret {
			t.Errorf("Resolve(%q) kept the placeholder", secret)
		}
	}
}

func TestResolveGeneratesEphemeralKey(t *testing.T) {
	requireKeyEnvUnset(t)

	opts, err := Resolve("", "")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if !opts.Ephemeral {
		t.Error("Resolve did not report the generated key as ephemeral")
	}

	// The generated key must actually work, not merely exist.
	signer, err := NewSignerFromOptions(opts)
	if err != nil {
		t.Fatalf("NewSignerFromOptions: %v", err)
	}
	raw, err := signer.Issue("user@jrepp.com", zeroDomain())
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if _, err := signer.Verify(raw, zeroDomain()); err != nil {
		t.Fatalf("Verify: %v", err)
	}
}

func TestResolveTTL(t *testing.T) {
	requireKeyEnvUnset(t)

	opts, err := Resolve(testSecret, "2h30m")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if opts.TTL != 2*time.Hour+30*time.Minute {
		t.Errorf("ttl = %v, want 2h30m", opts.TTL)
	}

	for _, bad := range []string{"forever", "-1h", "0"} {
		if _, err := Resolve(testSecret, bad); err == nil {
			t.Errorf("Resolve accepted ttl %q", bad)
		}
	}
}

func TestResolveRejectsShortSecret(t *testing.T) {
	t.Setenv(KeyEnvVar, "hunter2")
	if _, err := Resolve("", ""); err == nil {
		t.Fatal("Resolve accepted a 7-character secret; short secrets must be an error, " +
			"not a silent downgrade to an ephemeral key")
	}
}
