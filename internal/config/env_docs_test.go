package config_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// envVarPattern matches a HERMES_* name wherever it appears.
var envVarPattern = regexp.MustCompile(`HERMES_[A-Z0-9_]+`)

// TestDocumentedEnvVarsAreReal keeps the environment-variable guide honest.
//
// The guide had drifted badly: it documented 28 variables that no code
// anywhere read, including plausible-looking ones like
// HERMES_SERVER_POSTGRES_HOST and HERMES_DEX_ISSUER_URL. A variable that
// silently does nothing is worse than no variable at all -- someone sets it,
// the setting is ignored, and nothing indicates why.
//
// Hermes is configured through HCL, so the environment surface is small on
// purpose and this test keeps it that way in both directions.
func TestDocumentedEnvVarsAreReal(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	guide := filepath.Join(root, "docs-internal", "guides", "dev", "env-vars.md")

	content, err := os.ReadFile(guide)
	if err != nil {
		t.Fatalf("reading the env var guide: %v", err)
	}

	// Only names in backticks are being documented as usable. Prose that names
	// a variable to say it does *not* exist must not trip this.
	documented := map[string]bool{}
	for _, m := range regexp.MustCompile("`(HERMES_[A-Z0-9_]+)`").FindAllStringSubmatch(string(content), -1) {
		documented[m[1]] = true
	}
	if len(documented) == 0 {
		t.Fatal("found no documented variables; the guide or this test has moved")
	}

	real := envVarsInRepo(t, root)

	var phantom []string
	for name := range documented {
		if !real[name] {
			phantom = append(phantom, name)
		}
	}
	sort.Strings(phantom)

	if len(phantom) > 0 {
		t.Errorf("%s documents %d variable(s) that nothing reads:\n  %s\n\n"+
			"Either wire them up or remove them. Documenting a variable that has "+
			"no effect sends people looking for a bug that is not there.",
			filepath.Base(guide), len(phantom), strings.Join(phantom, "\n  "))
	}
}

// TestServerEnvVarsAreDocumented catches the other direction: a new override
// added to the server that nobody can discover.
//
// Scoped to the server command because that is the operator-facing surface.
// Test-only and agent-only variables are documented too, but they are not
// something a deployment needs to know about, so they are not enforced.
func TestServerEnvVarsAreDocumented(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)

	serverSrc, err := os.ReadFile(filepath.Join(
		root, "internal", "cmd", "commands", "server", "server.go"))
	if err != nil {
		t.Fatalf("reading server.go: %v", err)
	}
	sessionSrc, err := os.ReadFile(filepath.Join(root, "internal", "session", "key.go"))
	if err != nil {
		t.Fatalf("reading session key.go: %v", err)
	}

	guide, err := os.ReadFile(filepath.Join(
		root, "docs-internal", "guides", "dev", "env-vars.md"))
	if err != nil {
		t.Fatalf("reading the env var guide: %v", err)
	}
	documented := string(guide)

	used := map[string]bool{}
	for _, src := range [][]byte{serverSrc, sessionSrc} {
		for _, m := range regexp.MustCompile(`"(HERMES_[A-Z0-9_]+)"`).
			FindAllStringSubmatch(string(src), -1) {
			used[m[1]] = true
		}
	}

	var missing []string
	for name := range used {
		if !strings.Contains(documented, "`"+name+"`") {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)

	if len(missing) > 0 {
		t.Errorf("the server reads %d variable(s) the guide does not mention:\n  %s\n\n"+
			"Add them to docs-internal/guides/dev/env-vars.md.",
			len(missing), strings.Join(missing, "\n  "))
	}
}

// envVarsInRepo collects every HERMES_* name that appears in source or
// infrastructure, which is the set a documented variable could plausibly be
// read from.
func envVarsInRepo(t *testing.T, root string) map[string]bool {
	t.Helper()

	found := map[string]bool{}
	skipDirs := map[string]bool{
		".git": true, "node_modules": true, "build": true,
		"docs": true, "docs-internal": true, "docs-demo": true,
	}

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// An unreadable corner of the tree is not this test's business.
			return nil //nolint:nilerr // keep walking
		}
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}

			return nil
		}

		switch filepath.Ext(path) {
		case ".go", ".yml", ".yaml", ".sh", ".hcl", ".ts", ".js", ".json", ".env":
		default:
			if !strings.HasPrefix(d.Name(), "Dockerfile") &&
				!strings.HasPrefix(d.Name(), ".env") &&
				d.Name() != "Makefile" {
				return nil
			}
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil //nolint:nilerr // keep walking
		}
		for _, name := range envVarPattern.FindAllString(string(content), -1) {
			found[name] = true
		}

		return nil
	})
	if err != nil {
		t.Fatalf("walking the repository: %v", err)
	}

	return found
}

func repoRoot(t *testing.T) string {
	t.Helper()

	// This package lives at <root>/internal/config.
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolving the repository root: %v", err)
	}

	return root
}
