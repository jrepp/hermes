//go:build integration
// +build integration

package migration

import (
	"os"
	"testing"
)

// TestMain is the entry point for migration integration tests.
// Note: These tests use external Docker containers managed via docker-compose,
// not testcontainers. Prerequisites are checked in Phase0 of each test.
func TestMain(m *testing.M) {
	// Run tests
	code := m.Run()

	// Exit with test result code
	os.Exit(code)
}
