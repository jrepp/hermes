package session

import (
	"crypto/sha256"
	"fmt"
	"os"
	"strings"
	"time"
)

// KeyEnvVar overrides the configured signing key. The key is a secret, so a
// deployment should supply it this way rather than committing it to an HCL
// file.
const KeyEnvVar = "HERMES_SESSION_KEY"

// DeriveKey turns a configured secret into a signing key.
//
// The secret is hashed rather than decoded. Accepting base64 would make the
// encoding ambiguous -- a 44-character secret is both valid base64 and 44
// perfectly good bytes -- and an operator who picked the wrong interpretation
// would get a silently weaker key. Hashing means any secret of sufficient
// length yields a full-width key, and the same secret always yields the same
// key, so sessions survive a restart.
func DeriveKey(secret string) ([]byte, error) {
	if len(secret) < MinKeyLength {
		return nil, fmt.Errorf(
			"session: signing secret is %d characters, need at least %d; "+
				"generate one with `openssl rand -base64 32`",
			len(secret), MinKeyLength)
	}

	sum := sha256.Sum256([]byte(secret))

	return sum[:], nil
}

// Options is the resolved session configuration.
type Options struct {
	Secret string
	TTL    time.Duration
	// Ephemeral reports that no secret was configured and one was generated
	// for this process. Sessions issued under an ephemeral key do not survive
	// a restart, and in a multi-process deployment they are not portable
	// between instances at all.
	Ephemeral bool
}

// Resolve determines the signing secret and TTL from configuration and the
// environment.
//
// When nothing is configured a random secret is generated. That fails closed:
// the cost is that everyone is logged out on restart, whereas a fixed default
// secret -- or trusting an unsigned cookie, which is what Hermes did before
// this package existed -- would let anyone mint a session for any user.
func Resolve(configuredSecret, configuredTTL string) (Options, error) {
	opts := Options{TTL: DefaultTTL}

	secret := configuredSecret
	if v, ok := os.LookupEnv(KeyEnvVar); ok {
		secret = v
	}
	secret = strings.TrimSpace(secret)

	// Placeholder values in the shipped examples must not be usable. An
	// operator who forgets to replace one should get a fresh ephemeral key and
	// a warning, not a session key that every reader of the repository knows.
	if secret == "" || isPlaceholderSecret(secret) {
		generated, err := GenerateKey()
		if err != nil {
			return Options{}, err
		}
		opts.Secret = generated
		opts.Ephemeral = true
	} else {
		if _, err := DeriveKey(secret); err != nil {
			return Options{}, err
		}
		opts.Secret = secret
	}

	if configuredTTL != "" {
		d, err := time.ParseDuration(configuredTTL)
		if err != nil {
			return Options{}, fmt.Errorf(
				"session: invalid ttl %q: %w", configuredTTL, err)
		}
		if d <= 0 {
			return Options{}, fmt.Errorf(
				"session: ttl must be positive, got %q", configuredTTL)
		}
		opts.TTL = d
	}

	return opts, nil
}

// NewSignerFromOptions builds a Signer from resolved options.
func NewSignerFromOptions(opts Options) (*Signer, error) {
	key, err := DeriveKey(opts.Secret)
	if err != nil {
		return nil, err
	}

	return NewSigner(key, opts.TTL)
}

// placeholderSecrets are the values that appear in committed examples and
// documentation.
var placeholderSecrets = []string{
	"change-me",
	"changeme",
	"replace-me",
	"secret",
}

func isPlaceholderSecret(secret string) bool {
	lower := strings.ToLower(secret)
	for _, p := range placeholderSecrets {
		if strings.HasPrefix(lower, p) {
			return true
		}
	}

	return false
}
