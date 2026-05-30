package openapi

import (
	"fmt"
	"testing"
)

func TestRoutesHaveOpenAPIMetadata(t *testing.T) {
	seen := map[string]struct{}{}
	for _, r := range Routes() {
		if r.Method == "" {
			t.Fatalf("route %q has empty method", r.Path)
		}
		if r.Path == "" {
			t.Fatalf("route %q has empty path", r.Summary)
		}
		if r.Summary == "" {
			t.Fatalf("route %s %s has empty summary", r.Method, r.Path)
		}
		if r.Tag == "" {
			t.Fatalf("route %s %s has empty tag", r.Method, r.Path)
		}
		if r.RBAC.Resource == "" || r.RBAC.Action == "" || r.RBAC.Authenticated == "" {
			t.Fatalf("route %s %s has incomplete RBAC metadata: %#v", r.Method, r.Path, r.RBAC)
		}

		key := fmt.Sprintf("%s %s", r.Method, r.Path)
		if _, ok := seen[key]; ok {
			t.Fatalf("duplicate route metadata for %s", key)
		}
		seen[key] = struct{}{}
	}
}

func TestDocumentIncludesRoutesAndRBAC(t *testing.T) {
	doc := Document()
	paths, ok := doc["paths"].(map[string]any)
	if !ok {
		t.Fatalf("document paths missing or wrong type: %#v", doc["paths"])
	}

	pathItem, ok := paths["/api/v2/web/config"].(map[string]any)
	if !ok {
		t.Fatalf("web config path missing: %#v", paths)
	}
	operation, ok := pathItem["get"].(map[string]any)
	if !ok {
		t.Fatalf("web config get operation missing: %#v", pathItem)
	}
	if _, ok := operation["x-rbac"].(RBAC); !ok {
		t.Fatalf("web config x-rbac missing: %#v", operation)
	}
}
