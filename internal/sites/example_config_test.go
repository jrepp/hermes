package sites_test

import (
	"testing"

	"github.com/hashicorp-forge/hermes/internal/config"
	"github.com/hashicorp-forge/hermes/internal/sites"
)

// TestExampleConfigIsUsable keeps local/config.example.hcl honest.
//
// The example is the first thing anyone copies when setting up a deployment.
// Left untested it rots silently — a renamed field or a newly required
// argument turns it into a file that cannot be loaded, and the failure
// surfaces to whoever is least equipped to debug it.
func TestExampleConfigIsUsable(t *testing.T) {
	t.Parallel()

	cfg, err := config.NewConfig("../../local/config.example.hcl", "")
	if err != nil {
		t.Fatalf("local/config.example.hcl does not parse: %v", err)
	}

	registry, err := sites.NewRegistry(cfg)
	if err != nil {
		t.Fatalf("local/config.example.hcl does not produce a valid site registry: %v", err)
	}

	if registry.Len() < 2 {
		t.Fatalf("example configures %d sites; it should demonstrate multi-domain hosting", registry.Len())
	}

	// The example must actually demonstrate isolation, not merely parse.
	schemas := map[string]string{}
	workspaces := map[string]string{}
	for _, site := range registry.Sites() {
		name := site.Domain.String()

		if prev, dup := schemas[site.SchemaName]; dup {
			t.Errorf("sites %q and %q share schema %q", prev, name, site.SchemaName)
		}
		schemas[site.SchemaName] = name

		if prev, dup := workspaces[site.WorkspacePath]; dup {
			t.Errorf("sites %q and %q share workspace %q", prev, name, site.WorkspacePath)
		}
		workspaces[site.WorkspacePath] = name
	}

	// The trusted-proxy list must parse, or the server would fail to start
	// with the example as written.
	if len(cfg.TrustedProxies) == 0 {
		t.Error("example should show trusted_proxies for the nginx front end")
	}
}
