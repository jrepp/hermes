# HCL Projects Configuration - Design Rationale

## Why HCL Instead of JSON?

### 1. **Native to HashiCorp Ecosystem**

Hermes already uses HCL for `config.hcl`. Using HCL for projects maintains consistency:

```hcl
# config.hcl
postgres {
  host = "localhost"
  port = 5432
}

# projects.hcl - Same syntax!
project "testing" {
  provider "local" {
    workspace_path = "./workspace_data"
  }
}
```

### 2. **Environment Variables are First-Class**

HCL has built-in `env()` function:

```hcl
# HCL - Clean and obvious
provider "google" {
  credentials_path = env("GOOGLE_CREDENTIALS_PATH")
}
```

vs JSON workarounds:

```json
{
  "credentialsPath": "${GOOGLE_CREDENTIALS_PATH}"
}
```

(Requires custom templating logic in Go)

### 3. **Comments are Native**

HCL supports `#` and `//` comments:

```hcl
project "testing" {
  # This is a test project for local development
  short_name = "TEST"  # Used in TEST-001, TEST-002, etc.
  
  provider "local" {
    // Git integration for version tracking
    git {
      repository = "https://github.com/hashicorp-forge/hermes"
    }
  }
}
```

JSON requires awkward `_comment` fields:

```json
{
  "shortName": "TEST",
  "_comment": "Used in TEST-001, TEST-002, etc."
}
```

### 4. **Modular with `import`**

HCL natively supports imports:

```hcl
# projects.hcl
import "projects/testing.hcl"
import "projects/docs.hcl"
import "projects/security.hcl"
```

Each team can own their own project file!

JSON requires:
- Single monolithic file, OR
- Custom loading logic to merge multiple files

### 5. **Better for Configuration Management**

**Git-Friendly Diffs**:
```diff
 project "testing" {
   title = "Hermes Testing"
-  status = "active"
+  status = "archived"
 }
```

vs JSON (noisy commas, brackets):
```diff
   {
     "title": "Hermes Testing",
-    "status": "active"
+    "status": "archived",
   }
```

**Merge Conflicts**: HCL's structure makes conflicts easier to resolve.

### 6. **Type Safety with Go's hcl/v2**

HashiCorp's `github.com/hashicorp/hcl/v2` package provides:
- Strong typing
- Validation at parse time
- Better error messages
- Same parser as Terraform, Vault, Nomad

```go
type ProjectConfig struct {
    Name       string            `hcl:"name,label"`
    Title      string            `hcl:"title"`
    ShortName  string            `hcl:"short_name"`
    Status     string            `hcl:"status"`
    Providers  []ProviderConfig  `hcl:"provider,block"`
}
```

### 7. **Shorter Syntax**

**HCL**:
```hcl
project "testing" {
  short_name = "TEST"
  status     = "active"
}
```

**JSON**:
```json
{
  "projectId": "testing",
  "shortName": "TEST",
  "status": "active"
}
```

Less typing, less syntax noise.

### 8. **Templates are Obvious**

Prefix with `_template-` in filename:

```
projects/
├── testing.hcl              ✅ Loaded
├── docs.hcl                 ✅ Loaded
├── _template-google.hcl     ❌ Not loaded (template)
└── _template-migration.hcl  ❌ Not loaded (template)
```

JSON required:
- `"status": "archived"` in content (easy to miss)
- OR custom naming convention + filtering logic

### 9. **Blocks are More Readable**

**HCL nested blocks**:
```hcl
project "rfcs" {
  provider "google" {
    migration_status = "source"
  }
  
  provider "local" {
    migration_status = "target"
  }
  
  migration {
    conflict_detection_enabled = true
    notify_on_conflict        = true
  }
}
```

**JSON nested objects** (harder to scan):
```json
{
  "projectId": "rfcs",
  "providers": [
    {
      "type": "google",
      "migrationStatus": "source"
    },
    {
      "type": "local",
      "migrationStatus": "target"
    }
  ],
  "migration": {
    "conflictDetectionEnabled": true,
    "notifyOnConflict": true
  }
}
```

### 10. **Organizational Benefits**

**One File Per Project**:
```
projects/
├── platform-team.hcl       # Platform team owns this
├── security-team.hcl       # Security team owns this
├── docs-team.hcl           # Docs team owns this
```

**Benefits**:
- Clear ownership (CODEOWNERS file)
- Parallel development (no merge conflicts)
- Easy to add/remove projects
- Clear git history per team

**JSON approach**: Single monolithic file with all teams editing = constant conflicts.

## Short Names - Key Benefits

### Before (JSON): Long Identifiers

```json
{
  "projectId": "hermes-testing-environment",
  "tla": "TEST"
}
```

Document IDs: `hermes-testing-environment-001` (too long!)

### After (HCL): Short Names

```hcl
project "testing" {  # File key (short)
  short_name = "TEST"  # Display identifier
}
```

Document IDs: `TEST-001`, `TEST-042` ✅

**Benefits**:
- **Concise**: Shorter URLs, IDs, references
- **Memorable**: TEST-001 vs hermes-testing-001
- **Consistent**: Aligns with common patterns (RFC-001, PRD-042)
- **Flexible**: Can change title without breaking document IDs

## Migration Path

### Phase 1: Support Both (Backwards Compatible)

```go
// internal/config/projects_loader.go
func LoadProjects(path string) (*ProjectsConfig, error) {
    if strings.HasSuffix(path, ".hcl") {
        return loadHCL(path)
    }
    return loadJSON(path)  // Legacy support
}
```

### Phase 2: Deprecate JSON

- Add deprecation warning when loading JSON
- Update documentation to show HCL only
- Provide conversion tool: `./hermes projects migrate-to-hcl`

### Phase 3: Remove JSON Support

- Remove JSON loading code
- Remove JSON schema
- Keep only HCL

## Real-World Example

**Before (JSON - 50 lines for one project)**:
```json
{
  "version": "1.0.0",
  "projects": [
    {
      "projectId": "engineering-rfcs",
      "title": "Engineering Request for Comments",
      "friendlyName": "RFC",
      "tla": "RFC",
      "description": "Technical design documents",
      "status": "active",
      "providers": [
        {
          "type": "local",
          "migrationStatus": "active",
          "config": {
            "workspacePath": "./rfcs",
            "gitRepository": "https://github.com/example/rfcs",
            "gitBranch": "main",
            "allowedFileExtensions": ["md"],
            "indexingEnabled": true
          }
        }
      ],
      "metadata": {
        "createdAt": "2025-10-22T00:00:00Z",
        "owner": "platform-team",
        "tags": ["rfc", "engineering"]
      }
    }
  ]
}
```

**After (HCL - 23 lines for one project)**:
```hcl
# projects/engineering-rfcs.hcl
project "rfcs" {
  title         = "Engineering Request for Comments"
  friendly_name = "RFC"
  short_name    = "RFC"
  description   = "Technical design documents"
  status        = "active"
  
  provider "local" {
    workspace_path = "./rfcs"
    
    git {
      repository = "https://github.com/example/rfcs"
      branch     = "main"
    }
    
    indexing {
      enabled            = true
      allowed_extensions = ["md"]
    }
  }
  
  metadata {
    created_at = "2025-10-22T00:00:00Z"
    owner      = "platform-team"
    tags       = ["rfc", "engineering"]
  }
}
```

**54% reduction in lines!**

## Developer Experience

### Adding a New Project (HCL)

```bash
# 1. Create file
cat > testing/projects/my-project.hcl <<EOF
project "my-project" {
  short_name = "MYPROJ"
  # ... config
}
EOF

# 2. Import in main config
echo 'import "projects/my-project.hcl"' >> testing/projects.hcl

# 3. Done!
```

### Adding a New Project (JSON)

```bash
# 1. Open monolithic JSON file
vim testing/projects.json

# 2. Find the right place in the array
# 3. Add commas correctly
# 4. Validate JSON syntax
# 5. Hope you didn't break someone else's project
```

## Conclusion

HCL is the **clear winner** for Hermes projects configuration:

✅ **Native to HashiCorp ecosystem** - Consistent with existing config  
✅ **Better modularity** - One file per project  
✅ **Cleaner syntax** - 50%+ fewer lines  
✅ **Built-in env vars** - No custom templating needed  
✅ **Comments** - Native support  
✅ **Imports** - Native modular loading  
✅ **Type safety** - Strong typing with hcl/v2  
✅ **Git-friendly** - Better diffs, fewer conflicts  
✅ **Team ownership** - Clear CODEOWNERS per file  
✅ **Short names** - Concise document identifiers  

JSON was a good starting point, but HCL is the right long-term solution for a HashiCorp project.

---

**Related Files**:
- `testing/projects.hcl` - Main configuration
- `testing/projects/*.hcl` - Individual project configs
- `testing/projects/README.md` - Configuration guide
- `docs-internal/DISTRIBUTED_PROJECTS_ARCHITECTURE.md` - Architecture
