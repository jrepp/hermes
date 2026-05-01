//go:build integration && nfr
// +build integration,nfr

package nfr

import (
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp-forge/hermes/tests/integration"
)

var (
	fixtureSetupOnce sync.Once
	fixtureSetupErr  error
)

func requireFixture(t *testing.T) {
	t.Helper()

	fixtureSetupOnce.Do(func() {
		fixtureSetupErr = integration.SetupFixtureSuite()
	})
	require.NoError(t, fixtureSetupErr)
}

func TestMain(m *testing.M) {
	code := m.Run()
	integration.TeardownFixtureSuite()
	os.Exit(code)
}
