package session

import (
	"os"
	"testing"

	"github.com/hashicorp-forge/hermes/pkg/domain"
)

// requireKeyEnvUnset makes the key override genuinely absent for the duration
// of the test and restores it afterwards.
//
// t.Setenv can only set a variable to a value, and "" is not the same as unset
// for the code under test, so this does the save/restore itself.
func requireKeyEnvUnset(t *testing.T) {
	t.Helper()

	prev, had := os.LookupEnv(KeyEnvVar)
	if err := os.Unsetenv(KeyEnvVar); err != nil {
		t.Fatalf("unsetting %s: %v", KeyEnvVar, err)
	}
	t.Cleanup(func() {
		if had {
			_ = os.Setenv(KeyEnvVar, prev)
			return
		}
		_ = os.Unsetenv(KeyEnvVar)
	})
}

func zeroDomain() domain.Name { return domain.Name{} }
