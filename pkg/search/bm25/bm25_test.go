package bm25

import (
	"sync"
	"testing"
)

func TestTokenizeSplitsIdentifiersAndStems(t *testing.T) {
	tokens := Tokenize("SearchOutbox-indexing documents")
	want := map[string]bool{"search": true, "outbox": true, "index": true, "document": true}
	for _, token := range tokens {
		delete(want, token)
	}
	if len(want) != 0 {
		t.Fatalf("missing expected tokens: %#v from %#v", want, tokens)
	}
}

func TestSearchRanking(t *testing.T) {
	idx := New()
	idx.Add(Document{ID: "1", Title: "Search Outbox", Content: "outbox outbox reliability"})
	idx.Add(Document{ID: "2", Title: "Storage", Content: "object storage"})

	results := idx.Search("outbox reliability", 10)
	if len(results) == 0 || results[0].Document.ID != "1" {
		t.Fatalf("expected document 1 first, got %#v", results)
	}
}

func TestSearchNoMatch(t *testing.T) {
	idx := New()
	idx.Add(Document{ID: "1", Content: "alpha beta"})
	if results := idx.Search("gamma", 10); len(results) != 0 {
		t.Fatalf("expected no results, got %#v", results)
	}
}

func TestRemove(t *testing.T) {
	idx := New()
	idx.Add(Document{ID: "1", Content: "alpha"})
	idx.Remove("1")
	if idx.Count() != 0 {
		t.Fatalf("expected empty index")
	}
	if results := idx.Search("alpha", 10); len(results) != 0 {
		t.Fatalf("expected no results after remove, got %#v", results)
	}
}

func TestRebuild(t *testing.T) {
	idx := New()
	idx.Add(Document{ID: "1", Content: "alpha"})
	idx.Rebuild([]Document{{ID: "2", Content: "beta"}})
	if results := idx.Search("alpha", 10); len(results) != 0 {
		t.Fatalf("expected old document removed, got %#v", results)
	}
	if results := idx.Search("beta", 10); len(results) != 1 || results[0].Document.ID != "2" {
		t.Fatalf("expected rebuilt document, got %#v", results)
	}
}

func TestDeterministicEqualScoreOrdering(t *testing.T) {
	idx := New()
	idx.Add(Document{ID: "b", Content: "same"})
	idx.Add(Document{ID: "a", Content: "same"})
	results := idx.Search("same", 10)
	if len(results) != 2 || results[0].Document.ID != "a" || results[1].Document.ID != "b" {
		t.Fatalf("expected deterministic ID ordering, got %#v", results)
	}
}

func TestConcurrentAccess(t *testing.T) {
	idx := New()
	var wg sync.WaitGroup
	for i := 0; i < 25; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			idx.Add(Document{ID: string(rune('a' + i)), Content: "concurrent search"})
			_ = idx.Search("search", 5)
		}(i)
	}
	wg.Wait()
	if idx.Count() != 25 {
		t.Fatalf("expected 25 docs, got %d", idx.Count())
	}
}
