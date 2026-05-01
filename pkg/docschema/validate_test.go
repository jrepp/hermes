package docschema

import (
	"path/filepath"
	"testing"

	"github.com/hashicorp-forge/hermes/pkg/docdiscover"
	"github.com/hashicorp-forge/hermes/pkg/projectconfig"
)

func TestValidateDocumentsMonorepoFixture(t *testing.T) {
	root := filepath.Join("..", "docdiscover", "testdata", "monorepo")
	config, err := projectconfig.LoadConfig(filepath.Join(root, "projects.hcl"))
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}
	docs, err := docdiscover.Discover(config, docdiscover.Options{RootDir: root})
	if err != nil {
		t.Fatalf("Discover returned error: %v", err)
	}

	result := ValidateDocuments(docs)
	if len(result.Diagnostics) != 0 {
		t.Fatalf("expected no diagnostics, got %#v", result.Diagnostics)
	}
}

func TestValidateDocumentsUnknownSchema(t *testing.T) {
	root := filepath.Join("..", "docdiscover", "testdata", "monorepo", "invalid-schema")
	config, err := projectconfig.LoadConfig(filepath.Join(root, "projects.hcl"))
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}
	docs, err := docdiscover.Discover(config, docdiscover.Options{RootDir: root})
	if err != nil {
		t.Fatalf("Discover returned error: %v", err)
	}

	result := ValidateDocuments(docs)
	if len(result.Diagnostics) != 1 {
		t.Fatalf("expected 1 diagnostic, got %#v", result.Diagnostics)
	}
	if result.Diagnostics[0].Field != "schema" || result.Diagnostics[0].Severity != SeverityError {
		t.Fatalf("unexpected diagnostic: %#v", result.Diagnostics[0])
	}
}
