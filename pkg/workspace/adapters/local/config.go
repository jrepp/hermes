// Package local provides a local workspace storage adapter.
package local

import (
	"fmt"
	"path/filepath"
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

	// Set defaults
	if c.DocsPath == "" {
		c.DocsPath = filepath.Join(c.BasePath, "docs")
	}
	if c.DraftsPath == "" {
		c.DraftsPath = filepath.Join(c.BasePath, "drafts")
	}
	if c.FoldersPath == "" {
		c.FoldersPath = filepath.Join(c.BasePath, "folders")
	}
	if c.UsersPath == "" {
		c.UsersPath = filepath.Join(c.BasePath, "users.json")
	}
	if c.TokensPath == "" {
		c.TokensPath = filepath.Join(c.BasePath, "tokens.json")
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
