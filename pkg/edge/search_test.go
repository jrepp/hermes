package edge

import "testing"

func TestBuildIndexAndSearch(t *testing.T) {
	idx, indexed, err := BuildIndex(testOptions(false))
	if err != nil {
		t.Fatalf("BuildIndex returned error: %v", err)
	}
	if idx.Count() != 3 || len(indexed.Documents) != 3 {
		t.Fatalf("expected 3 indexed docs, got count=%d result=%#v", idx.Count(), indexed)
	}

	result, err := SearchLocal(testOptions(false), "clean", 10, true)
	if err != nil {
		t.Fatalf("SearchLocal returned error: %v", err)
	}
	if len(result.Hits) == 0 || result.Hits[0].Path != "docs/clean.md" {
		t.Fatalf("expected clean doc hit, got %#v", result.Hits)
	}
	if result.Hits[0].Debug == nil {
		t.Fatalf("expected debug output")
	}
}

func TestIndexCachePath(t *testing.T) {
	got := IndexCachePath("/tmp/repo", "docs")
	if got == "" {
		t.Fatal("expected cache path")
	}
}
