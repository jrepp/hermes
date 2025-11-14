//go:build integration
// +build integration

package edgesync

import (
	"os"
	"testing"

	"github.com/hashicorp-forge/hermes/tests/integration"
)

// TestMain is the entry point for edge sync integration tests.
// It starts containers before tests and tears them down after.
func TestMain(m *testing.M) {
	// Setup: Start containers (reuses existing fixture if already started)
	if err := integration.SetupFixtureSuite(); err != nil {
		println("❌ Failed to setup integration test fixture:", err.Error())
		os.Exit(1)
	}

	// Run tests
	code := m.Run()

	// Note: We don't teardown here because other test packages may still need the containers
	// The global fixture is torn down by the main integration package's TestMain

	// Exit with test result code
	os.Exit(code)
}
