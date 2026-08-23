package api

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"strings"
	"testing"
)

// tenantFields are the Server fields ForDomain rebinds per site. Reading any
// of them from an unscoped Server serves the primary site's data to whoever
// asked, which is the whole failure this package's scoping exists to prevent.
//
// The database is the obvious one, and was for a while the only one checked --
// which is how SearchHandler came to search the primary site's index on every
// site while touching srv.DB not once.
var tenantFields = map[string]bool{
	"DB":                true,
	"SearchProvider":    true,
	"WorkspaceProvider": true,
}

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

	var usesTenantResource, scopes bool
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		ident, ok := sel.X.(*ast.Ident)
		if !ok || ident.Name != "srv" {
			return true
		}

		switch {
		case tenantFields[sel.Sel.Name]:
			usesTenantResource = true
			if !insideLiteral(sel.Pos()) {
				t.Errorf("%s: %s reads srv.%s at %s, outside the request closure.\n"+
					"That captures the process-wide value once at startup, so every "+
					"site is served from whichever tenant the default points at.\n"+
					"Move it inside the handler and take it from the scoped srv.",
					path, fn.Name.Name, sel.Sel.Name, fset.Position(sel.Pos()))
			}
		case sel.Sel.Name == "ForRequest", sel.Sel.Name == "ForDomain":
			scopes = true
		}

		return true
	})

	if usesTenantResource && !scopes {
		t.Errorf("%s: %s uses a per-tenant resource but never calls srv.ForRequest.\n"+
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

// TestDatabaseHelpersAreReachedOnlyFromScopedHandlers closes the gap the
// constructor check leaves open.
//
// Only a handler constructor is required to call ForRequest, but the database
// access itself often lives in a helper -- handleIndexerRegister,
// projectsResourceRelatedResourcesHandler, and so on -- which receives an
// already-scoped Server from its caller. That works, and nothing checks it: a
// helper wired to a new unscoped caller would read the wrong tenant's data
// with no test failing.
//
// So every function that takes a server.Server and touches srv.DB is traced
// back to its callers. Each root must be a constructor that scopes. A helper
// with no callers at all is reported too: nothing establishes how a future
// caller will reach it, and the first one to appear will not be checked.
func TestDatabaseHelpersAreReachedOnlyFromScopedHandlers(t *testing.T) {
	t.Parallel()

	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(fi fs.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	if err != nil {
		t.Fatalf("parsing package: %v", err)
	}

	scoped := map[string]bool{}   // functions that call ForRequest/ForDomain
	usesDB := map[string]bool{}   // functions that read srv.DB
	takesSrv := map[string]bool{} // functions with a server.Server parameter
	calls := map[string][]string{}
	callers := map[string][]string{}

	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Body == nil {
					continue
				}
				name := fn.Name.Name
				takesSrv[name] = hasServerParam(fn)

				ast.Inspect(fn.Body, func(n ast.Node) bool {
					switch node := n.(type) {
					case *ast.SelectorExpr:
						if ident, ok := node.X.(*ast.Ident); ok && ident.Name == "srv" {
							switch node.Sel.Name {
							case "ForRequest", "ForDomain":
								scoped[name] = true
							default:
								if tenantFields[node.Sel.Name] {
									usesDB[name] = true
								}
							}
						}
					case *ast.CallExpr:
						if ident, ok := node.Fun.(*ast.Ident); ok {
							calls[name] = append(calls[name], ident.Name)
							callers[ident.Name] = append(callers[ident.Name], name)
						}
					}

					return true
				})
			}
		}
	}

	for name := range usesDB {
		if scoped[name] || !takesSrv[name] {
			continue
		}

		if len(callers[name]) == 0 {
			t.Errorf("%s takes a server.Server and reads a per-tenant resource, but nothing "+
				"calls it.\n"+
				"Nothing establishes which site it would run against, so the "+
				"first caller to appear will not be checked. Either take a "+
				"context and scope inside, or remove it.", name)
			continue
		}

		for _, root := range unscopedRoots(name, callers, scoped, map[string]bool{}) {
			t.Errorf("%s reads a per-tenant resource, and is reachable from %s, which never "+
				"scopes to a site.\n"+
				"Add to that handler:\n"+
				"\tsrv, ok := srv.ForRequest(w, r)\n\tif !ok {\n\t\treturn\n\t}",
				name, root)
		}
	}
}

// unscopedRoots walks callers upward and returns any that neither scope nor
// have callers of their own.
func unscopedRoots(
	name string, callers map[string][]string, scoped, seen map[string]bool,
) []string {
	if seen[name] {
		return nil
	}
	seen[name] = true

	var roots []string
	for _, caller := range callers[name] {
		switch {
		case scoped[caller]:
			// Reaching a function that scopes ends this path.
		case len(callers[caller]) == 0:
			roots = append(roots, caller)
		default:
			roots = append(roots, unscopedRoots(caller, callers, scoped, seen)...)
		}
	}

	return roots
}

// hasServerParam reports whether fn takes a server.Server named srv.
func hasServerParam(fn *ast.FuncDecl) bool {
	if fn.Type.Params == nil {
		return false
	}
	for _, p := range fn.Type.Params.List {
		if !isSelector(p.Type, "server", "Server") {
			continue
		}
		for _, n := range p.Names {
			if n.Name == "srv" {
				return true
			}
		}
	}

	return false
}
