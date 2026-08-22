// Package local provides a local workspace storage adapter.
package local

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/hashicorp-forge/hermes/pkg/domain"
)

// Config contains local workspace adapter configuration.
type Config struct {
	FileSystem  FileSystem  `hcl:"-"`
	SMTPConfig  *SMTPConfig `hcl:"smtp,block"`
	BasePath    string      `hcl:"base_path"`
	DocsPath    string      `hcl:"docs_path,optional"`
	DraftsPath  string      `hcl:"drafts_path,optional"`
	FoldersPath string      `hcl:"folders_path,optional"`
	UsersPath   string      `hcl:"users_path,optional"`
	TokensPath  string      `hcl:"tokens_path,optional"`

	// Domain scopes this adapter to a single site.
	//
	// When set, every default path is rooted at <BasePath>/<domain> instead of
	// <BasePath>, so two sites served by the same process never write into the
	// same directory. Explicitly configured paths must then also fall inside
	// that root; Validate rejects ones that escape it, because a stray
	// absolute path is exactly how one tenant's documents end up readable by
	// another.
	//
	// The zero value means "not domain-scoped", which is the single-tenant
	// layout used before multi-domain hosting.
	Domain domain.Name `hcl:"-"`
}

// Root returns the directory containing this workspace's data.
//
// It is BasePath for a single-tenant configuration, and the domain's subtree
// when the adapter is scoped to a site.
func (c *Config) Root() string {
	if c.Domain.IsZero() {
		return c.BasePath
	}

	// PathSegment is a validated DNS name: never empty, never "." or "..",
	// and free of separators, so joining it cannot escape BasePath.
	return filepath.Join(c.BasePath, c.Domain.PathSegment())
}

// withinRoot reports whether path lies inside root.
func withinRoot(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}

	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// SMTPConfig contains SMTP server configuration.
type SMTPConfig struct {
	Host     string `hcl:"host"`
	Username string `hcl:"username,optional"`
	Password string `hcl:"password,optional"`
	From     string `hcl:"from"`
	Port     int    `hcl:"port"`
}

// Validate checks configuration validity and sets defaults.
func (c *Config) Validate() error {
	if c.BasePath == "" {
		return fmt.Errorf("base_path cannot be empty")
	}

	root := c.Root()

	// Set defaults, rooted at the domain subtree when domain-scoped.
	if c.DocsPath == "" {
		c.DocsPath = filepath.Join(root, "docs")
	}
	if c.DraftsPath == "" {
		c.DraftsPath = filepath.Join(root, "drafts")
	}
	if c.FoldersPath == "" {
		c.FoldersPath = filepath.Join(root, "folders")
	}
	if c.UsersPath == "" {
		c.UsersPath = filepath.Join(root, "users.json")
	}
	if c.TokensPath == "" {
		c.TokensPath = filepath.Join(root, "tokens.json")
	}

	// A domain-scoped adapter must not be able to reach outside its own
	// subtree, whatever the operator wrote in the config file.
	if !c.Domain.IsZero() {
		for name, path := range map[string]string{
			"docs_path":    c.DocsPath,
			"drafts_path":  c.DraftsPath,
			"folders_path": c.FoldersPath,
			"users_path":   c.UsersPath,
			"tokens_path":  c.TokensPath,
		} {
			if !withinRoot(root, path) {
				return fmt.Errorf(
					"%s %q is outside the workspace root %q for domain %q",
					name, path, root, c.Domain)
			}
		}
	}

	// Default to OS filesystem if not set
	if c.FileSystem == nil {
		c.FileSystem = NewOsFileSystem()
	}

	return nil
}

// Example HCL configuration:
//
// workspace "local" {
//   base_path = "/var/hermes/workspace"
//   docs_path = "/var/hermes/workspace/documents"
//
//   smtp {
//     host = "smtp.example.com"
//     port = 587
//     username = "hermes"
//     password = "secret"
//     from = "noreply@example.com"
//   }
// }
