package api

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"strings"
	"testing"
)

// TestHandlersScopeToTheRequestSite is a static guard on tenant isolation.
//
// Every V2 handler is constructed once, at startup, with a server.Server
// holding the process-wide defaults. In a multi-site deployment that Server's
// DB belongs to no tenant, so a handler must call ForRequest to bind to the
// site actually being served. A handler that forgets reads and writes another
// tenant's data, and does so silently -- there is no error, just wrong rows.
//
// Two failure modes are checked, and the second is the one that actually
// happened: EdgeSyncHandler built its DocumentSyncService from srv.DB at
// construction time, outside the request closure, so no amount of scoping
// inside the closure would have helped it.
func TestHandlersScopeToTheRequestSite(t *testing.T) {
	t.Parallel()

	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(fi fs.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	if err != nil {
		t.Fatalf("parsing package: %v", err)
	}

	for _, pkg := range pkgs {
		for path, file := range pkg.Files {
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || !isHandlerConstructor(fn) {
					continue
				}
				checkConstructor(t, fset, path, fn)
			}
		}
	}
}

// isHandlerConstructor reports whether fn takes a server.Server named srv and
// returns an http.Handler -- the shape every V2 endpoint is registered with.
func isHandlerConstructor(fn *ast.FuncDecl) bool {
	if fn.Type.Params == nil || fn.Type.Results == nil ||
		len(fn.Type.Results.List) != 1 {
		return false
	}

	if !isSelector(fn.Type.Results.List[0].Type, "http", "Handler") {
		return false
	}

	for _, p := range fn.Type.Params.List {
		if !isSelector(p.Type, "server", "Server") {
			continue
		}
		for _, name := range p.Names {
			if name.Name == "srv" {
				return true
			}
		}
	}

	return false
}

func checkConstructor(t *testing.T, fset *token.FileSet, path string, fn *ast.FuncDecl) {
	t.Helper()

	// Record the span of every function literal in the body. Anything outside
	// them runs once at startup rather than per request.
	type span struct{ from, to token.Pos }
	var literals []span
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if lit, ok := n.(*ast.FuncLit); ok {
			literals = append(literals, span{lit.Pos(), lit.End()})
		}

		return true
	})
	insideLiteral := func(pos token.Pos) bool {
		for _, s := range literals {
			if pos >= s.from && pos < s.to {
				return true
			}
		}

		return false
	}

	var usesDB, scopes bool
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		ident, ok := sel.X.(*ast.Ident)
		if !ok || ident.Name != "srv" {
			return true
		}

		switch sel.Sel.Name {
		case "DB":
			usesDB = true
			if !insideLiteral(sel.Pos()) {
				t.Errorf("%s: %s reads srv.DB at %s, outside the request closure.\n"+
					"That captures the process-wide database once at startup, so every "+
					"site is served from whichever schema the default connection points at.\n"+
					"Move it inside the handler and take it from the scoped srv.",
					path, fn.Name.Name, fset.Position(sel.Pos()))
			}
		case "ForRequest", "ForDomain":
			scopes = true
		}

		return true
	})

	if usesDB && !scopes {
		t.Errorf("%s: %s uses srv.DB but never calls srv.ForRequest.\n"+
			"Add this as the first statement of the handler:\n"+
			"\tsrv, ok := srv.ForRequest(w, r)\n"+
			"\tif !ok {\n\t\treturn\n\t}",
			path, fn.Name.Name)
	}
}

func isSelector(expr ast.Expr, pkg, name string) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	ident, ok := sel.X.(*ast.Ident)

	return ok && ident.Name == pkg && sel.Sel.Name == name
}
