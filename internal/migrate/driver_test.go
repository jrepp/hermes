package migrate_test

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/hashicorp-forge/hermes/internal/migrate"
)

// TestServerBinaryExcludesSQLite enforces ADR-019.
//
// The SQLite migration driver links modernc.org/sqlite, a pure-Go SQLite
// implementation weighing several megabytes, and the server binary can never
// use it -- it refuses SQLite outright and tells the operator to run
// hermes-migrate. It had been linked in anyway, because internal/migrate is
// reachable from cmd/hermes through internal/db and imported the driver
// unconditionally.
//
// Nothing about that is visible from reading the server's own imports, which
// is exactly why it needs a test rather than a comment.
func TestServerBinaryExcludesSQLite(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		pkg       string
		wantSQLie bool
	}{
		{"../../cmd/hermes", false},
		{"../../cmd/hermes-migrate", true},
	} {
		deps, err := exec.Command("go", "list", "-deps", tc.pkg).Output()
		if err != nil {
			t.Skipf("go list unavailable: %v", err)
		}

		hasSQLite := strings.Contains(string(deps), "modernc.org/sqlite")
		switch {
		case hasSQLite && !tc.wantSQLie:
			t.Errorf("%s links modernc.org/sqlite.\n"+
				"ADR-019 keeps SQLite out of the server binary. Something now "+
				"imports internal/migrate/sqlitedriver, directly or through a "+
				"dependency; find it with: go list -deps %s | grep sqlite",
				tc.pkg, tc.pkg)
		case !hasSQLite && tc.wantSQLie:
			t.Errorf("%s no longer links modernc.org/sqlite, so it cannot "+
				"migrate a SQLite database", tc.pkg)
		}
	}
}

// TestUnsupportedDriverExplainsItself checks the error an operator actually
// sees, since "unsupported driver: sqlite" from a binary that documents SQLite
// support is a confusing thing to read.
func TestUnsupportedDriverExplainsItself(t *testing.T) {
	t.Parallel()

	err := migrate.RunMigrations(nil, "sqlite")
	if err == nil {
		t.Fatal("the server-side package accepted the sqlite driver")
	}
	if !strings.Contains(err.Error(), "hermes-migrate") {
		t.Errorf("error does not point at the binary that can do it: %v", err)
	}

	err = migrate.RunMigrations(nil, "mysql")
	if err == nil {
		t.Fatal("an unknown driver was accepted")
	}
	if !strings.Contains(err.Error(), "supported:") {
		t.Errorf("error does not list what is supported: %v", err)
	}
}

func TestPostgresIsAlwaysRegistered(t *testing.T) {
	t.Parallel()

	var found bool
	for _, name := range migrate.SupportedDrivers() {
		if name == "postgres" {
			found = true
		}
	}
	if !found {
		t.Errorf("postgres is not registered; drivers are %v", migrate.SupportedDrivers())
	}
}
