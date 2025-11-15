//go:build integration
// +build integration

package e2e

import (
	"os"
	"testing"

	"github.com/hashicorp-forge/hermes/tests/integration"
)

// TestMain is the entry point for E2E integration tests.
// It sets up the test fixture (PostgreSQL, Meilisearch) and tears it down after tests complete.
//
// Note: The full docker-compose stack (Central, Edge, Redpanda, Mailhog, etc.) must be
// running separately via: cd testing && docker compose up -d
func TestMain(m *testing.M) {
	// Setup: Start fixture containers (PostgreSQL, Meilisearch)
	if err := integration.SetupFixtureSuite(); err != nil {
		println("❌ Failed to setup integration test fixture:", err.Error())
		os.Exit(1)
	}

	// Run tests
	code := m.Run()

	// Teardown: Stop fixture containers
	integration.TeardownFixtureSuite()

	// Exit with test result code
	os.Exit(code)
}
