package server

import (
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"strconv"
	"testing"
)

// TestEndpointPatternsAreUnique is the regression test for a startup crash.
//
// http.ServeMux panics on a duplicate pattern, so registering
// "/api/v2/documents/" twice did not shadow one handler with the other -- it
// took the entire server down before it ever listened, in every configuration.
// The endpoint list is long and grouped by feature, which makes a duplicate
// easy to add and impossible to see by reading.
//
// The patterns are read out of the source rather than restated here, so the
// test cannot drift away from the list it is checking.
func TestEndpointPatternsAreUnique(t *testing.T) {
	t.Parallel()

	patterns := endpointPatternsFromSource(t)
	if len(patterns) < 20 {
		t.Fatalf("found only %d endpoint patterns; the extraction has stopped "+
			"matching the source", len(patterns))
	}

	seen := make(map[string]bool, len(patterns))
	for _, p := range patterns {
		if seen[p] {
			t.Errorf("pattern %q is registered more than once; http.ServeMux "+
				"panics on that, taking the server down at startup", p)
		}
		seen[p] = true
	}

	// Registering them for real is the authoritative check: ServeMux also
	// rejects patterns that conflict without being byte-identical.
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("registering the endpoint patterns panics: %v", r)
		}
	}()
	mux := http.NewServeMux()
	for _, p := range patterns {
		mux.Handle(p, http.NotFoundHandler())
	}
}

// endpointPatternsFromSource collects the path of every `endpoint` composite
// literal in server.go — both the bare `{handler, "/path"}` form used inside
// the slice literals and the explicit `endpoint{handler, "/path"}` form used
// when appending.
func endpointPatternsFromSource(t *testing.T) []string {
	t.Helper()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "server.go", nil, 0)
	if err != nil {
		t.Fatalf("parsing server.go: %v", err)
	}

	var patterns []string
	ast.Inspect(file, func(n ast.Node) bool {
		lit, ok := n.(*ast.CompositeLit)
		if !ok {
			return true
		}

		// The slice literals are []endpoint{...}; their elements are the
		// composite literals carrying a handler and a path.
		if !isEndpointSlice(lit) && !isEndpointLit(lit) {
			return true
		}

		for _, elt := range lit.Elts {
			inner, ok := elt.(*ast.CompositeLit)
			if !ok {
				continue
			}
			if p, ok := patternOf(inner); ok {
				patterns = append(patterns, p)
			}
		}
		if p, ok := patternOf(lit); ok {
			patterns = append(patterns, p)
		}

		return true
	})

	return patterns
}

func isEndpointSlice(lit *ast.CompositeLit) bool {
	arr, ok := lit.Type.(*ast.ArrayType)
	if !ok {
		return false
	}
	ident, ok := arr.Elt.(*ast.Ident)

	return ok && ident.Name == "endpoint"
}

func isEndpointLit(lit *ast.CompositeLit) bool {
	ident, ok := lit.Type.(*ast.Ident)

	return ok && ident.Name == "endpoint"
}

// patternOf returns the path from a two-element {handler, "/path"} literal.
func patternOf(lit *ast.CompositeLit) (string, bool) {
	if len(lit.Elts) != 2 {
		return "", false
	}
	str, ok := lit.Elts[1].(*ast.BasicLit)
	if !ok || str.Kind != token.STRING {
		return "", false
	}
	value, err := strconv.Unquote(str.Value)
	if err != nil || value == "" || value[0] != '/' {
		return "", false
	}

	return value, true
}
