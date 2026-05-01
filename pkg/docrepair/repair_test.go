package docrepair

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp-forge/hermes/pkg/docdiscover"
	"github.com/hashicorp-forge/hermes/pkg/docschema"
	"github.com/hashicorp-forge/hermes/pkg/projectconfig"
)

func TestMigrateDryRunDoesNotMutate(t *testing.T) {
	root := copyFixture(t)
	docs := discoverFixture(t, root)
	path := filepath.Join(root, "docs", "missing.md")
	before := readFile(t, path)

	plan, err := Execute(docs, Options{Operation: OperationMigrate})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !plan.DryRun {
		t.Fatal("expected dry run plan")
	}
	if len(plan.Changes) == 0 {
		t.Fatal("expected planned changes")
	}
	if got := readFile(t, path); got != before {
		t.Fatal("dry run mutated file")
	}
}

func TestMigrateApplyAndValidateIdempotent(t *testing.T) {
	root := copyFixture(t)
	docs := discoverFixture(t, root)

	plan, err := Execute(docs, Options{Operation: OperationMigrate, Apply: true})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if plan.DryRun {
		t.Fatal("expected apply plan")
	}

	docs = discoverFixture(t, root)
	result := docschema.ValidateDocuments(docs)
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.File != "docs/bad.md" {
			t.Fatalf("expected only known malformed fixture diagnostic after apply, got %#v", result.Diagnostics)
		}
	}

	second, err := Execute(docs, Options{Operation: OperationMigrate, Apply: true})
	if err != nil {
		t.Fatalf("second Execute returned error: %v", err)
	}
	if len(second.Changes) != 0 {
		t.Fatalf("expected idempotent no-op, got %#v", second.Changes)
	}
}

func TestBulkUpdatePreservesUnknownFields(t *testing.T) {
	root := copyFixture(t)
	docs := discoverFixture(t, root)

	_, err := Execute(docs, Options{Operation: OperationBulkUpdate, Apply: true, Field: "owner", Value: "docs-team"})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	content := readFile(t, filepath.Join(root, "docs", "partial.md"))
	if !strings.Contains(content, "custom_field: keep-me") {
		t.Fatalf("unknown field was not preserved: %s", content)
	}
	if !strings.Contains(content, "owner: docs-team") {
		t.Fatalf("owner was not set: %s", content)
	}
}

func TestTimestampsPlanWarnsWhenUsingCurrentTime(t *testing.T) {
	root := copyFixture(t)
	docs := discoverFixture(t, root)
	now := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)

	plan, err := Execute(docs, Options{Operation: OperationTimestamps, Now: now})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if len(plan.Warnings) == 0 {
		t.Fatal("expected degraded git-history warning")
	}
}

func TestCompressIDsApply(t *testing.T) {
	root := copyFixture(t)
	docs := discoverFixture(t, root)

	plan, err := Execute(docs, Options{Operation: OperationCompressIDs, Apply: true, StartID: 7})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if len(plan.Changes) == 0 {
		t.Fatal("expected ID changes")
	}
	content := readFile(t, filepath.Join(root, "docs", "missing.md"))
	if !strings.Contains(content, "id: 008") && !strings.Contains(content, "id: 8") {
		t.Fatalf("expected compressed id in content: %s", content)
	}
}

func TestLinksReportsMissingTarget(t *testing.T) {
	root := copyFixture(t)
	docs := discoverFixture(t, root)

	plan, err := Execute(docs, Options{Operation: OperationLinks})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if len(plan.Warnings) == 0 {
		t.Fatal("expected missing link warning")
	}
}

func TestMalformedFrontmatterWarning(t *testing.T) {
	root := copyFixture(t)
	docs := discoverFixture(t, root)

	plan, err := Execute(docs, Options{Operation: OperationMigrate})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	found := false
	for _, warning := range plan.Warnings {
		if strings.Contains(warning.Message, "unterminated frontmatter") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected malformed frontmatter warning, got %#v", plan.Warnings)
	}
}

func discoverFixture(t *testing.T, root string) []docdiscover.Document {
	t.Helper()
	config, err := projectconfig.LoadConfig(filepath.Join(root, "projects.hcl"))
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}
	docs, err := docdiscover.Discover(config, docdiscover.Options{RootDir: root})
	if err != nil {
		t.Fatalf("Discover returned error: %v", err)
	}
	return docs
}

func copyFixture(t *testing.T) string {
	t.Helper()
	src := filepath.Join("testdata", "repair")
	dst := t.TempDir()
	copyDir(t, src, dst)
	return dst
}

func copyDir(t *testing.T, src, dst string) {
	t.Helper()
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatalf("ReadDir %s: %v", src, err)
	}
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		if entry.IsDir() {
			if err := os.MkdirAll(dstPath, 0o755); err != nil {
				t.Fatalf("MkdirAll %s: %v", dstPath, err)
			}
			copyDir(t, srcPath, dstPath)
			continue
		}
		data, err := os.ReadFile(srcPath) //nolint:gosec // test fixture copy.
		if err != nil {
			t.Fatalf("ReadFile %s: %v", srcPath, err)
		}
		if err := os.WriteFile(dstPath, data, 0o644); err != nil {
			t.Fatalf("WriteFile %s: %v", dstPath, err)
		}
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path) //nolint:gosec // test fixture read.
	if err != nil {
		t.Fatalf("ReadFile %s: %v", path, err)
	}
	return string(data)
}
