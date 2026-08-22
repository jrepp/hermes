package database

import (
	"fmt"
	"strings"
)

// MaxSchemaNameLength is PostgreSQL's identifier limit (NAMEDATALEN - 1).
const MaxSchemaNameLength = 63

// ValidateSchemaName checks that name is safe to interpolate into DDL and into
// a connection string.
//
// A schema name cannot be a bind parameter -- neither `CREATE SCHEMA $1` nor a
// parameterised search_path exists -- so it is always string-interpolated
// somewhere. Most schema names are derived from a domain.Name and are
// therefore already constrained, but `schema_name` is also settable directly
// in configuration, and that is the path an injected identifier would take.
// Validating once, here, means callers do not each have to decide how much to
// trust their input.
func ValidateSchemaName(name string) error {
	if name == "" {
		return fmt.Errorf("database: schema name is empty")
	}
	if len(name) > MaxSchemaNameLength {
		return fmt.Errorf(
			"database: schema name %q is %d bytes, over PostgreSQL's %d-byte limit",
			name, len(name), MaxSchemaNameLength)
	}

	// Lowercase only. PostgreSQL folds unquoted identifiers to lowercase, so
	// accepting "Site_Foo" would mean the name in the config and the name in
	// the catalog disagree -- and then a later exact-match lookup fails for
	// reasons nobody enjoys debugging.
	for i := 0; i < len(name); i++ {
		c := name[i]
		switch {
		case c >= 'a' && c <= 'z', c == '_':
		case c >= '0' && c <= '9':
			if i == 0 {
				return fmt.Errorf(
					"database: schema name %q starts with a digit", name)
			}
		default:
			return fmt.Errorf(
				"database: schema name %q contains %q; only [a-z0-9_] are allowed",
				name, string(c))
		}
	}

	// pg_ is reserved for system schemas and pg_catalog is always searched
	// first regardless, so a site schema under that prefix would be shadowed
	// or rejected by the server.
	if strings.HasPrefix(name, "pg_") {
		return fmt.Errorf(
			"database: schema name %q uses the reserved pg_ prefix", name)
	}

	return nil
}

// QuoteSchemaName renders name as a quoted SQL identifier.
//
// ValidateSchemaName already excludes everything that would need escaping;
// quoting anyway means a future relaxation of those rules cannot turn into an
// injection, and it keeps the DDL readable.
func QuoteSchemaName(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}
