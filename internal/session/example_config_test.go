package session_test

import (
	"strings"
	"testing"

	"github.com/hashicorp-forge/hermes/internal/config"
	"github.com/hashicorp-forge/hermes/internal/session"
)

// TestExampleConfigSessionBlock keeps the session half of
// local/config.example.hcl honest, in both directions: the block has to parse,
// and the placeholder it ships with must not be usable as a signing secret.
//
// The second half matters more than it looks. The example is committed, so its
// key is public; if Resolve ever started accepting it, every deployment that
// copied the file unchanged would be signing sessions with a secret anyone can
// read, and nothing would look wrong.
func TestExampleConfigSessionBlock(t *testing.T) {
	cfg, err := config.NewConfig("../../local/config.example.hcl", "")
	if err != nil {
		t.Fatalf("local/config.example.hcl does not parse: %v", err)
	}

	if cfg.Session == nil {
		t.Fatal("example has no session block; deployments need to be shown how to set a signing key")
	}

	// Isolate from a developer's own environment.
	prev, had := lookupEnv(session.KeyEnvVar)
	unsetEnv(t, session.KeyEnvVar)
	t.Cleanup(func() {
		if had {
			setEnv(t, session.KeyEnvVar, prev)
		}
	})

	opts, err := session.Resolve(cfg.Session.Key, cfg.Session.TTL)
	if err != nil {
		t.Fatalf("example session block is not resolvable: %v", err)
	}
	if !opts.Ephemeral {
		t.Errorf("example key %q was accepted as a real signing secret", cfg.Session.Key)
	}
	if opts.TTL <= 0 {
		t.Errorf("example ttl resolved to %v", opts.TTL)
	}

	// A real secret in the same block must work, so the instructions in the
	// example are followable.
	real, err := session.GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	opts, err = session.Resolve(real, cfg.Session.TTL)
	if err != nil {
		t.Fatalf("Resolve with a generated key: %v", err)
	}
	if opts.Ephemeral {
		t.Error("a generated key was treated as a placeholder")
	}
	if _, err := session.NewSignerFromOptions(opts); err != nil {
		t.Fatalf("NewSignerFromOptions: %v", err)
	}

	// The example documents `openssl rand -base64 32`; make sure output of
	// that shape is long enough to be accepted.
	if len(strings.TrimSpace(real)) < session.MinKeyLength {
		t.Errorf("generated key is %d characters, under the documented minimum", len(real))
	}
}
