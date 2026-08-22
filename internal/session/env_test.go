package session_test

import (
	"os"
	"testing"
)

func lookupEnv(key string) (string, bool) { return os.LookupEnv(key) }

func unsetEnv(t *testing.T, key string) {
	t.Helper()
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("unsetting %s: %v", key, err)
	}
}

func setEnv(t *testing.T, key, value string) {
	t.Helper()
	if err := os.Setenv(key, value); err != nil {
		t.Fatalf("setting %s: %v", key, err)
	}
}
