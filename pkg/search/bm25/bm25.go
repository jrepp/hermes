// Package bm25 implements an embedded lexical BM25 index.
package bm25

import (
	"math"
	"regexp"
	"sort"
	"strings"
	"sync"
)

const (
	defaultK1 = 1.5
	defaultB  = 0.75
)

var tokenPattern = regexp.MustCompile(`[A-Za-z0-9]+`)

// Document is a lexical index document.
type Document struct {
	ID       string         `json:"id"`
	Title    string         `json:"title,omitempty"`
	Content  string         `json:"content,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// Result is a search hit.
type Result struct {
	Document Document `json:"document"`
	Score    float64  `json:"score"`
	Terms    []string `json:"terms"`
}

// Index is a thread-safe in-memory BM25 index.
type Index struct {
	mu        sync.RWMutex
	k1        float64
	b         float64
	docs      map[string]Document
	docTerms  map[string]map[string]int
	docLen    map[string]int
	df        map[string]int
	avgDocLen float64
}

// New returns an empty BM25 index.
func New() *Index {
	return &Index{
		k1:       defaultK1,
		b:        defaultB,
		docs:     make(map[string]Document),
		docTerms: make(map[string]map[string]int),
		docLen:   make(map[string]int),
		df:       make(map[string]int),
	}
}

// Add indexes or replaces a document.
func (i *Index) Add(doc Document) {
	i.mu.Lock()
	defer i.mu.Unlock()

	if doc.ID == "" {
		return
	}
	i.removeLocked(doc.ID)
	terms := termFrequency(Tokenize(doc.Title + " " + doc.Content))
	i.docs[doc.ID] = doc
	i.docTerms[doc.ID] = terms
	length := 0
	for term, count := range terms {
		length += count
		i.df[term]++
	}
	i.docLen[doc.ID] = length
	i.recomputeAvgLocked()
}

// Remove deletes a document from the index.
func (i *Index) Remove(id string) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.removeLocked(id)
	i.recomputeAvgLocked()
}

// Rebuild replaces the index contents.
func (i *Index) Rebuild(docs []Document) {
	i.mu.Lock()
	i.docs = make(map[string]Document)
	i.docTerms = make(map[string]map[string]int)
	i.docLen = make(map[string]int)
	i.df = make(map[string]int)
	i.avgDocLen = 0
	i.mu.Unlock()

	for _, doc := range docs {
		i.Add(doc)
	}
}

// Search returns top BM25 hits for a query.
func (i *Index) Search(query string, limit int) []Result {
	i.mu.RLock()
	defer i.mu.RUnlock()

	queryTerms := unique(Tokenize(query))
	if len(queryTerms) == 0 || len(i.docs) == 0 {
		return nil
	}
	if limit <= 0 {
		limit = 10
	}

	results := make([]Result, 0)
	for id, doc := range i.docs {
		score, matched := i.scoreLocked(id, queryTerms)
		if score <= 0 {
			continue
		}
		results = append(results, Result{Document: doc, Score: score, Terms: matched})
	}
	sort.Slice(results, func(a, b int) bool {
		if results[a].Score == results[b].Score {
			return results[a].Document.ID < results[b].Document.ID
		}
		return results[a].Score > results[b].Score
	})
	if len(results) > limit {
		results = results[:limit]
	}
	return results
}

// Count returns the number of indexed documents.
func (i *Index) Count() int {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return len(i.docs)
}

// Tokenize splits text into normalized searchable terms.
func Tokenize(text string) []string {
	text = splitIdentifiers(text)
	matches := tokenPattern.FindAllString(text, -1)
	tokens := make([]string, 0, len(matches))
	for _, match := range matches {
		token := stem(strings.ToLower(match))
		if token != "" {
			tokens = append(tokens, token)
		}
	}
	return tokens
}

func splitIdentifiers(text string) string {
	var b strings.Builder
	var prev rune
	for _, r := range text {
		if prev != 0 && ((isLower(prev) && isUpper(r)) || (isLetter(prev) && isDigit(r)) || (isDigit(prev) && isLetter(r))) {
			b.WriteRune(' ')
		}
		if r == '_' || r == '-' || r == '/' || r == '.' {
			b.WriteRune(' ')
		} else {
			b.WriteRune(r)
		}
		prev = r
	}
	return b.String()
}

func (i *Index) scoreLocked(id string, queryTerms []string) (float64, []string) {
	terms := i.docTerms[id]
	docLen := float64(i.docLen[id])
	if docLen == 0 || i.avgDocLen == 0 {
		return 0, nil
	}
	score := 0.0
	matched := make([]string, 0)
	for _, term := range queryTerms {
		tf := float64(terms[term])
		if tf == 0 {
			continue
		}
		matched = append(matched, term)
		idf := math.Log(1 + (float64(len(i.docs))-float64(i.df[term])+0.5)/(float64(i.df[term])+0.5))
		denom := tf + i.k1*(1-i.b+i.b*(docLen/i.avgDocLen))
		score += idf * (tf * (i.k1 + 1)) / denom
	}
	return score, matched
}

func (i *Index) removeLocked(id string) {
	terms, ok := i.docTerms[id]
	if !ok {
		return
	}
	for term := range terms {
		i.df[term]--
		if i.df[term] <= 0 {
			delete(i.df, term)
		}
	}
	delete(i.docs, id)
	delete(i.docTerms, id)
	delete(i.docLen, id)
}

func (i *Index) recomputeAvgLocked() {
	if len(i.docLen) == 0 {
		i.avgDocLen = 0
		return
	}
	total := 0
	for _, length := range i.docLen {
		total += length
	}
	i.avgDocLen = float64(total) / float64(len(i.docLen))
}

func termFrequency(tokens []string) map[string]int {
	freq := make(map[string]int)
	for _, token := range tokens {
		freq[token]++
	}
	return freq
}

func unique(tokens []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(tokens))
	for _, token := range tokens {
		if !seen[token] {
			seen[token] = true
			result = append(result, token)
		}
	}
	return result
}

func stem(token string) string {
	for _, suffix := range []string{"ing", "ed", "es", "s"} {
		if len(token) > len(suffix)+2 && strings.HasSuffix(token, suffix) {
			return strings.TrimSuffix(token, suffix)
		}
	}
	return token
}

func isLower(r rune) bool  { return r >= 'a' && r <= 'z' }
func isUpper(r rune) bool  { return r >= 'A' && r <= 'Z' }
func isDigit(r rune) bool  { return r >= '0' && r <= '9' }
func isLetter(r rune) bool { return isLower(r) || isUpper(r) }
