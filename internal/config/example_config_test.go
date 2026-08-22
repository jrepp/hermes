package config_test

import (
	"testing"

	"github.com/hashicorp-forge/hermes/internal/config"
)

// TestShippedExamplesLoad is the cheapest possible guard on the first thing
// anyone does with this repository.
//
// The README's opening instruction is `cp config-example.hcl config.hcl`, and
// for some time that produced a file the server could not parse: four
// document_type blocks correctly omitted the Google-only `template` argument,
// which the schema marked required. Nobody noticed because nothing loaded the
// example.
func TestShippedExamplesLoad(t *testing.T) {
	t.Parallel()

	for _, path := range []string{
		"../../config-example.hcl",
		"../../local/config.example.hcl",
	} {
		if _, err := config.NewConfig(path, ""); err != nil {
			t.Errorf("%s does not load: %v", path, err)
		}
	}
}
