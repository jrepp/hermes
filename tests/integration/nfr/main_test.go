//go:build integration && nfr
// +build integration,nfr

package nfr

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp-forge/hermes/tests/integration"
)

func TestMain(m *testing.M) {
	if err := integration.SetupFixtureSuite(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to setup NFR fixture suite: %v\n", err)
		os.Exit(1)
	}

	code := m.Run()
	integration.TeardownFixtureSuite()
	os.Exit(code)
}
