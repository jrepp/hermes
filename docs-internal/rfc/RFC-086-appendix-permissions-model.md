# RFC-086 Appendix: API Provider Permissions Model

## Overview

This appendix defines the permission model for the API Provider (RFC-085), specifying which operations are allowed through the API and how they map to roles and claims.

**Core Principles**:
1. **Read-heavy design**: API provider primarily enables read operations and metadata queries
2. **Destructive actions restricted**: Delete, move, and rename operations require elevated permissions
3. **Project-level roles**: Users can have different roles per project
4. **Claims-based authorization**: Bearer tokens carry role claims validated by remote Hermes

## Permission Tree

### Operation Categories

```
API Operations
├── Document Operations
│   ├── Read Operations (Safe)
│   │   ├── GetDocument              [reader, editor, reviewer, admin]
│   │   ├── GetDocumentByUUID        [reader, editor, reviewer, admin]
│   │   ├── GetContent               [reader, editor, reviewer, admin]
│   │   ├── GetContentByUUID         [reader, editor, reviewer, admin]
│   │   ├── GetContentBatch          [reader, editor, reviewer, admin]
│   │   └── GetSubfolder             [reader, editor, reviewer, admin]
│   │
│   ├── Write Operations (Controlled)
│   │   ├── UpdateContent            [editor, admin]
│   │   ├── CreateDocument           [editor, admin]
│   │   ├── CopyDocument             [editor, admin]
│   │   ├── CreateFolder             [editor, admin]
│   │   └── RegisterDocument         [editor, admin]
│   │
│   └── Destructive Operations (Restricted)
│       ├── DeleteDocument           [admin only]
│       ├── MoveDocument             [admin only]
│       └── RenameDocument           [admin only]
│
├── Content Operations
│   ├── CompareContent               [reader, editor, reviewer, admin]
│   └── UpdateContent                [editor, admin]
│
├── Revision Operations (Read-Only via API)
│   ├── GetRevisionHistory           [reader, editor, reviewer, admin]
│   ├── GetRevision                  [reader, editor, reviewer, admin]
│   ├── GetRevisionContent           [reader, editor, reviewer, admin]
│   ├── GetAllDocumentRevisions      [reader, editor, reviewer, admin]
│   └── KeepRevisionForever          [admin only]
│
├── Permission Operations
│   ├── ListPermissions              [reader, editor, reviewer, admin]
│   ├── ShareDocument                [editor, admin]
│   ├── ShareDocumentWithDomain      [admin only]
│   ├── RemovePermission             [admin only]
│   └── UpdatePermission             [admin only]
│
├── People Operations (Directory)
│   ├── SearchPeople                 [reader, editor, reviewer, admin]
│   ├── GetPerson                    [reader, editor, reviewer, admin]
│   ├── GetPersonByUnifiedID         [reader, editor, reviewer, admin]
│   └── ResolveIdentity              [reader, editor, reviewer, admin]
│
├── Team Operations
│   ├── ListTeams                    [reader, editor, reviewer, admin]
│   ├── GetTeam                      [reader, editor, reviewer, admin]
│   ├── GetUserTeams                 [reader, editor, reviewer, admin]
│   └── GetTeamMembers               [reader, editor, reviewer, admin]
│
└── Notification Operations
    ├── SendEmail                    [editor, reviewer, admin]
    └── SendEmailWithTemplate        [editor, reviewer, admin]
```

## Role Definitions

### 1. Reader
**Purpose**: Read-only access to documents and metadata

**Permissions**:
- ✅ Read document metadata
- ✅ Read document content
- ✅ View revision history
- ✅ List permissions (but not modify)
- ✅ Search people and teams
- ❌ Create or edit documents
- ❌ Share documents
- ❌ Delete or move documents

**Use Cases**:
- External stakeholders
- Cross-team visibility
- Audit and compliance reviews

### 2. Editor
**Purpose**: Create and edit documents within a project

**Permissions**:
- ✅ All Reader permissions
- ✅ Create documents
- ✅ Edit document content
- ✅ Copy documents
- ✅ Share documents (grant reader/editor access)
- ✅ Send notifications
- ❌ Delete or move documents
- ❌ Modify permissions
- ❌ Share with entire domain

**Use Cases**:
- Document authors
- Team contributors
- Content creators

### 3. Reviewer
**Purpose**: Review documents and provide feedback (special editor variant)

**Permissions**:
- ✅ All Reader permissions
- ✅ Add comments/reviews
- ✅ Send review notifications
- ✅ Update document status (for review workflow)
- ❌ Create new documents
- ❌ Destructive operations

**Use Cases**:
- Technical reviewers
- Approvers
- Compliance checkers

### 4. Admin
**Purpose**: Full control over documents and permissions

**Permissions**:
- ✅ All Reader, Editor, Reviewer permissions
- ✅ Delete documents
- ✅ Move and rename documents
- ✅ Modify permissions
- ✅ Share with entire domain
- ✅ Keep revisions forever
- ✅ Register documents from edge instances

**Use Cases**:
- Project owners
- Team leads
- System administrators

## Token Claims Structure

Bearer tokens must include role claims for authorization:

```json
{
  "iss": "https://auth.example.com",
  "sub": "user@example.com",
  "aud": "hermes-client",
  "email": "user@example.com",
  "name": "Alice Johnson",
  "exp": 1699999999,
  "iat": 1699996399,

  // Hermes-specific claims
  "hermes_roles": {
    "global": "reader",
    "projects": {
      "platform-team": "editor",
      "security-team": "admin",
      "docs-team": "reviewer"
    }
  },

  // Required scopes
  "scope": "openid profile email hermes:read hermes:write"
}
```

### Claim Structure

**Global Role** (`hermes_roles.global`):
- Default role applied across all projects
- Values: `reader`, `editor`, `reviewer`, `admin`
- Fallback when no project-specific role exists

**Project Roles** (`hermes_roles.projects`):
- Map of project ID → role
- Overrides global role for specific projects
- Allows fine-grained access control

## Authorization Implementation

### Middleware for Authorization

```go
// Authorization middleware for remote Hermes API
func (s *Server) authorizeOperation(requiredRole string) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // Extract user claims from context (set by authentication middleware)
            claims, ok := r.Context().Value("user_claims").(*HermesClaims)
            if !ok {
                http.Error(w, "unauthorized: missing claims", http.StatusUnauthorized)
                return
            }

            // Get project ID from request (URL parameter, query string, or body)
            projectID := extractProjectID(r)

            // Check if user has required role
            if !hasRequiredRole(claims, projectID, requiredRole) {
                http.Error(w, fmt.Sprintf("forbidden: requires %s role", requiredRole), http.StatusForbidden)
                return
            }

            next.ServeHTTP(w, r)
        })
    }
}

// Role hierarchy: reader < editor < reviewer < admin
var roleHierarchy = map[string]int{
    "reader":   1,
    "editor":   2,
    "reviewer": 2, // Same level as editor
    "admin":    3,
}

func hasRequiredRole(claims *HermesClaims, projectID, requiredRole string) bool {
    // Get user's role for this project
    userRole := getUserRoleForProject(claims, projectID)

    // Check role hierarchy
    return roleHierarchy[userRole] >= roleHierarchy[requiredRole]
}

func getUserRoleForProject(claims *HermesClaims, projectID string) string {
    // Check project-specific role first
    if projectRole, ok := claims.HermesRoles.Projects[projectID]; ok {
        return projectRole
    }

    // Fall back to global role
    return claims.HermesRoles.Global
}
```

### Per-Endpoint Authorization

```go
// Document endpoints with role requirements
func (s *Server) registerDocumentRoutes(r *mux.Router) {
    // Read operations - require "reader" role
    r.Handle("/api/v2/documents/{id}",
        s.authorizeOperation("reader")(http.HandlerFunc(s.handleGetDocument))).
        Methods("GET")

    r.Handle("/api/v2/documents/{id}/content",
        s.authorizeOperation("reader")(http.HandlerFunc(s.handleGetContent))).
        Methods("GET")

    // Write operations - require "editor" role
    r.Handle("/api/v2/documents/{id}/content",
        s.authorizeOperation("editor")(http.HandlerFunc(s.handleUpdateContent))).
        Methods("PUT")

    r.Handle("/api/v2/documents",
        s.authorizeOperation("editor")(http.HandlerFunc(s.handleCreateDocument))).
        Methods("POST")

    // Destructive operations - require "admin" role
    r.Handle("/api/v2/documents/{id}",
        s.authorizeOperation("admin")(http.HandlerFunc(s.handleDeleteDocument))).
        Methods("DELETE")

    r.Handle("/api/v2/documents/{id}/move",
        s.authorizeOperation("admin")(http.HandlerFunc(s.handleMoveDocument))).
        Methods("PUT")
}

// Permission endpoints
func (s *Server) registerPermissionRoutes(r *mux.Router) {
    // List permissions - require "reader"
    r.Handle("/api/v2/documents/{id}/permissions",
        s.authorizeOperation("reader")(http.HandlerFunc(s.handleListPermissions))).
        Methods("GET")

    // Share document - require "editor"
    r.Handle("/api/v2/documents/{id}/permissions",
        s.authorizeOperation("editor")(http.HandlerFunc(s.handleShareDocument))).
        Methods("POST")

    // Domain sharing - require "admin"
    r.Handle("/api/v2/documents/{id}/permissions/domain",
        s.authorizeOperation("admin")(http.HandlerFunc(s.handleShareWithDomain))).
        Methods("POST")

    // Remove permission - require "admin"
    r.Handle("/api/v2/documents/{id}/permissions/{permId}",
        s.authorizeOperation("admin")(http.HandlerFunc(s.handleRemovePermission))).
        Methods("DELETE")
}
```

## Project ID Extraction

Projects can be identified through multiple mechanisms:

### 1. URL Path Parameter
```
GET /api/v2/projects/platform-team/documents/doc-123
```

### 2. Query Parameter
```
GET /api/v2/documents/doc-123?project=platform-team
```

### 3. Document Metadata
```go
func extractProjectID(r *http.Request) string {
    // Try URL path parameter
    if projectID := mux.Vars(r)["projectId"]; projectID != "" {
        return projectID
    }

    // Try query parameter
    if projectID := r.URL.Query().Get("project"); projectID != "" {
        return projectID
    }

    // Try document metadata lookup (if document ID provided)
    if docID := mux.Vars(r)["id"]; docID != "" {
        doc, err := s.workspace.GetDocument(r.Context(), docID)
        if err == nil && doc.Project != "" {
            return doc.Project
        }
    }

    // Default to empty (use global role)
    return ""
}
```

## Configuration Examples

### OIDC Provider Configuration (Dex)

Configure Dex to include Hermes role claims:

```yaml
# Dex configuration
connectors:
  - type: ldap
    id: ldap
    name: Corporate LDAP
    config:
      host: ldap.example.com:636
      userSearch:
        baseDN: ou=users,dc=example,dc=com
        filter: "(objectClass=person)"
        username: uid
        idAttr: uid
        emailAttr: mail
        nameAttr: cn
      groupSearch:
        baseDN: ou=groups,dc=example,dc=com
        filter: "(objectClass=groupOfNames)"
        userMatchers:
          - userAttr: DN
            groupAttr: member
        nameAttr: cn

# Static clients
staticClients:
  - id: hermes-client
    redirectURIs:
      - 'https://local.hermes.example.com/auth/callback'
      - 'https://central.hermes.example.com/auth/callback'
    name: 'Hermes'
    secret: <secret>

# Custom claims mapper (extends Dex)
claimsMapper:
  hermes_roles:
    # Map LDAP groups to Hermes roles
    global:
      - ldapGroup: "cn=all-employees,ou=groups,dc=example,dc=com"
        role: "reader"
    projects:
      platform-team:
        - ldapGroup: "cn=platform-editors,ou=groups,dc=example,dc=com"
          role: "editor"
        - ldapGroup: "cn=platform-admins,ou=groups,dc=example,dc=com"
          role: "admin"
      security-team:
        - ldapGroup: "cn=security-team,ou=groups,dc=example,dc=com"
          role: "admin"
```

### Remote Hermes Configuration

```hcl
# Remote Hermes configuration
server {
  api_auth {
    validate_tokens = true
    trusted_issuers = ["https://auth.example.com"]

    # Role-based authorization
    authorization {
      enabled = true
      default_role = "reader"

      # Role requirements per operation category
      roles {
        read_operations      = "reader"
        write_operations     = "editor"
        destructive_operations = "admin"
        permission_operations  = "admin"
      }
    }
  }
}
```

## Special Cases

### Cross-Project Operations

For operations spanning multiple projects, require the minimum role across all projects:

```go
func checkMultiProjectAccess(claims *HermesClaims, projectIDs []string, requiredRole string) bool {
    for _, projectID := range projectIDs {
        userRole := getUserRoleForProject(claims, projectID)
        if roleHierarchy[userRole] < roleHierarchy[requiredRole] {
            return false
        }
    }
    return true
}
```

### Machine-to-Machine API Keys

For background operations without user context, use M2M API keys with restricted permissions:

```hcl
# M2M API key configuration
api_keys {
  # Indexer service - read-only access
  indexer {
    key = env("INDEXER_API_KEY")
    role = "reader"
    allowed_operations = ["GetDocument", "GetContent", "ListDocuments"]
  }

  # Sync service - read and register
  sync_service {
    key = env("SYNC_API_KEY")
    role = "editor"
    allowed_operations = ["RegisterDocument", "GetDocument", "UpdateContent"]
  }
}
```

### Audit Logging

Log all authorization decisions for security auditing:

```go
func (s *Server) auditAuthorizationDecision(claims *HermesClaims, operation, projectID, decision string) {
    s.logger.Info("authorization_decision",
        "user", claims.Email,
        "operation", operation,
        "project", projectID,
        "role", getUserRoleForProject(claims, projectID),
        "decision", decision,
        "timestamp", time.Now().UTC(),
    )
}
```

## Security Considerations

### 1. Principle of Least Privilege
- Default to `reader` role if no role specified
- Require explicit grants for elevated permissions
- Review role assignments regularly

### 2. Role Escalation Prevention
- Editors cannot grant admin access
- Admins cannot modify their own permissions
- Role changes require separate admin approval

### 3. Token Scope Validation
- Verify `hermes:read` scope for read operations
- Verify `hermes:write` scope for write operations
- Reject tokens with insufficient scope

### 4. Rate Limiting per Role
```go
// Different rate limits per role
rateLimits := map[string]int{
    "reader":   100,  // 100 req/min
    "editor":   200,  // 200 req/min
    "reviewer": 150,  // 150 req/min
    "admin":    500,  // 500 req/min
}
```

## Migration Path

### Phase 1: Read-Only API (Week 5)
- Implement authentication and token validation
- Enable read operations for all authenticated users
- No role-based restrictions yet

### Phase 2: Role-Based Write Operations (Week 6)
- Add role claims to tokens
- Implement editor/admin distinction
- Protect write operations with role checks

### Phase 3: Project-Level Roles (Week 7)
- Extend claims to support project-specific roles
- Update authorization logic for project context
- Migrate users to project-specific role assignments

### Phase 4: Audit and Refinement (Week 8)
- Review authorization logs
- Adjust role permissions based on usage
- Add fine-grained permissions if needed

## Testing Strategy

### Unit Tests
```go
func TestRoleHierarchy(t *testing.T) {
    tests := []struct {
        userRole     string
        requiredRole string
        expectAccess bool
    }{
        {"admin", "reader", true},
        {"editor", "reader", true},
        {"reader", "editor", false},
        {"reader", "admin", false},
        {"admin", "admin", true},
    }

    for _, tt := range tests {
        hasAccess := roleHierarchy[tt.userRole] >= roleHierarchy[tt.requiredRole]
        assert.Equal(t, tt.expectAccess, hasAccess)
    }
}
```

### Integration Tests
```go
func TestAPIAuthorization(t *testing.T) {
    // Test reader can read but not write
    token := generateToken("user@example.com", "reader", nil)

    // Should succeed
    resp := makeRequest("GET", "/api/v2/documents/doc-123", token)
    assert.Equal(t, http.StatusOK, resp.StatusCode)

    // Should fail
    resp = makeRequest("DELETE", "/api/v2/documents/doc-123", token)
    assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}
```

## Real-World Examples

### Example 1: External Partner Read-Only Access

**Scenario**: Security audit firm needs read-only access to security documentation for compliance review.

**Configuration**:
```json
{
  "sub": "auditor@external-firm.com",
  "email": "auditor@external-firm.com",
  "name": "Jane Auditor",
  "hermes_roles": {
    "global": "reader",
    "projects": {
      "security-team": "reader"
    }
  }
}
```

**Allowed Operations**:
- ✅ View all documents in `security-team` project
- ✅ Read document content and metadata
- ✅ View revision history
- ✅ List permissions (to understand access patterns)
- ❌ Create, edit, or delete documents
- ❌ Share documents or modify permissions
- ❌ Access documents in other projects

**Token Generation** (via admin portal):
```bash
# Admin generates time-limited token for external auditor
hermes-admin tokens create \
  --email "auditor@external-firm.com" \
  --name "Jane Auditor - Security Audit Q4 2024" \
  --role "reader" \
  --project "security-team" \
  --expires "2024-12-31T23:59:59Z" \
  --max-uses 1000
```

### Example 2: Contractor with Time-Limited Editor Access

**Scenario**: Technical writer contractor hired for 3 months to create documentation.

**Configuration**:
```json
{
  "sub": "contractor@freelance.com",
  "email": "contractor@freelance.com",
  "name": "Bob Writer",
  "exp": 1707264000,
  "hermes_roles": {
    "global": "reader",
    "projects": {
      "docs-team": "editor",
      "platform-team": "reader"
    }
  }
}
```

**Allowed Operations**:
- ✅ Create and edit documents in `docs-team` project
- ✅ Copy documents within `docs-team`
- ✅ Share documents (grant reader/editor to others)
- ✅ Send notification emails
- ✅ Read documents in `platform-team` (for reference)
- ❌ Delete or move documents
- ❌ Modify permissions
- ❌ Access documents in other projects

**Token Refresh Flow**:
```go
// Contractor's edge Hermes refreshes token periodically
func (c *Client) refreshToken(ctx context.Context) error {
    // Exchange refresh token with OIDC provider
    token, err := c.oidcClient.RefreshToken(ctx, c.refreshToken)
    if err != nil {
        return fmt.Errorf("token refresh failed: %w", err)
    }

    // Verify expiration not exceeded
    if time.Unix(token.Claims.Exp, 0).Before(time.Now()) {
        return fmt.Errorf("contractor access expired")
    }

    c.bearerToken = token.AccessToken
    return nil
}
```

### Example 3: Cross-Project Compliance Auditor

**Scenario**: Internal compliance officer needs read access across all projects for audit trails.

**Configuration**:
```json
{
  "sub": "compliance@company.com",
  "email": "compliance@company.com",
  "name": "Alice Compliance",
  "hermes_roles": {
    "global": "reader",
    "projects": {
      "security-team": "reviewer",
      "platform-team": "reviewer",
      "docs-team": "reviewer",
      "legal-team": "admin"
    }
  }
}
```

**Use Case Flow**:
```go
// Compliance officer runs audit report across all projects
func (c *ComplianceService) GenerateAuditReport(ctx context.Context, startDate, endDate time.Time) (*AuditReport, error) {
    projects := []string{"security-team", "platform-team", "docs-team", "legal-team"}

    report := &AuditReport{
        Period: fmt.Sprintf("%s to %s", startDate, endDate),
        Projects: make(map[string]*ProjectAudit),
    }

    for _, projectID := range projects {
        // Query documents modified in date range
        docs, err := c.workspace.ListDocuments(ctx, projectID, &ListOptions{
            ModifiedAfter: startDate,
            ModifiedBefore: endDate,
        })
        if err != nil {
            return nil, fmt.Errorf("failed to list docs for %s: %w", projectID, err)
        }

        // For each document, get revision history
        for _, doc := range docs {
            revisions, err := c.workspace.GetRevisionHistory(ctx, doc.ID)
            if err != nil {
                continue // Log but don't fail entire report
            }

            report.Projects[projectID].Documents = append(
                report.Projects[projectID].Documents,
                &DocumentAudit{
                    ID: doc.ID,
                    Title: doc.Title,
                    RevisionCount: len(revisions),
                    LastModified: doc.ModifiedTime,
                    ModifiedBy: doc.ModifiedBy,
                },
            )
        }
    }

    return report, nil
}
```

### Example 4: CI/CD Service Account (M2M)

**Scenario**: Automated documentation generation pipeline needs to create and update docs.

**Configuration**:
```hcl
# M2M API key for CI/CD pipeline
api_keys {
  docs_generator {
    key = env("DOCS_GENERATOR_API_KEY")
    role = "editor"
    projects = ["docs-team", "api-docs"]

    # Restrict to specific operations
    allowed_operations = [
      "CreateDocument",
      "UpdateContent",
      "GetDocument",
      "GetContent",
      "RegisterDocument"
    ]

    # Rate limit
    rate_limit = 500  # requests per minute

    # IP allowlist (optional)
    allowed_ips = ["10.0.0.0/8", "192.168.1.100"]
  }
}
```

**API Key Usage**:
```bash
# CI/CD pipeline authenticates with API key
curl -X POST https://central.hermes.company.com/api/v2/documents \
  -H "Authorization: Bearer ${DOCS_GENERATOR_API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "title": "API Reference - Generated",
    "project": "api-docs",
    "content": "# API Documentation\n\n...",
    "metadata": {
      "generated": true,
      "generator": "openapi-to-hermes",
      "source": "api-spec.yaml"
    }
  }'
```

### Example 5: Emergency Admin Escalation

**Scenario**: On-call engineer needs temporary admin access to resolve incident affecting document availability.

**Break-glass Procedure**:
```bash
# 1. On-call engineer requests emergency access via CLI
hermes-oncall emergency-access request \
  --project "platform-team" \
  --role "admin" \
  --reason "Incident #1234: Document service outage" \
  --duration "2h"

# 2. System requires approval from TWO admins
# Admin 1 approves via Slack bot
/hermes approve-emergency EMA-2024-001 @oncall-engineer

# Admin 2 approves via web UI
# (Sends push notification to admin's phone)

# 3. Temporary token generated with elevated permissions
# Token automatically revoked after 2 hours
{
  "sub": "oncall@company.com",
  "email": "oncall@company.com",
  "name": "On-Call Engineer",
  "exp": 1699999999,  # 2 hours from now
  "hermes_roles": {
    "global": "reader",
    "projects": {
      "platform-team": "admin"  # Temporary escalation
    }
  },
  "emergency_access": {
    "ticket": "INC-1234",
    "approved_by": ["admin1@company.com", "admin2@company.com"],
    "reason": "Document service outage",
    "granted_at": "2024-11-12T10:30:00Z",
    "expires_at": "2024-11-12T12:30:00Z"
  }
}
```

**Audit Trail**:
```go
// All emergency access operations are logged with extra context
func (s *Server) auditEmergencyAccess(ctx context.Context, claims *HermesClaims, operation string) {
    if claims.EmergencyAccess != nil {
        s.logger.Warn("emergency_access_operation",
            "user", claims.Email,
            "operation", operation,
            "ticket", claims.EmergencyAccess.Ticket,
            "reason", claims.EmergencyAccess.Reason,
            "approved_by", claims.EmergencyAccess.ApprovedBy,
            "expires_at", claims.EmergencyAccess.ExpiresAt,
            "timestamp", time.Now().UTC(),
        )

        // Send real-time alert to security team
        s.sendSecurityAlert(ctx, "Emergency admin operation", claims)
    }
}
```

### Example 6: Multi-Region Team Collaboration

**Scenario**: Global platform team with different responsibilities across regions.

**US Team Lead Configuration**:
```json
{
  "sub": "lead-us@company.com",
  "email": "lead-us@company.com",
  "name": "US Team Lead",
  "hermes_roles": {
    "global": "reader",
    "projects": {
      "platform-team": "admin",
      "platform-us": "admin",
      "platform-eu": "editor",
      "platform-apac": "editor"
    }
  }
}
```

**EU Engineer Configuration**:
```json
{
  "sub": "engineer-eu@company.com",
  "email": "engineer-eu@company.com",
  "name": "EU Engineer",
  "hermes_roles": {
    "global": "reader",
    "projects": {
      "platform-team": "editor",
      "platform-us": "reader",
      "platform-eu": "editor",
      "platform-apac": "reader"
    }
  }
}
```

**APAC Contractor Configuration**:
```json
{
  "sub": "contractor-apac@vendor.com",
  "email": "contractor-apac@vendor.com",
  "name": "APAC Contractor",
  "exp": 1709596800,  # 6 month contract
  "hermes_roles": {
    "global": null,  # No global access
    "projects": {
      "platform-apac": "editor"
    }
  }
}
```

**Collaboration Pattern**:
```yaml
# Document sharing workflow
workflow:
  - step: US team lead creates architecture proposal
    project: platform-team
    role: admin
    operations:
      - CreateDocument(title="Multi-Region Deployment Strategy")
      - ShareDocument(with=["platform-us", "platform-eu", "platform-apac"])

  - step: EU engineer reviews and adds regional requirements
    project: platform-team
    role: editor
    operations:
      - GetDocument(id="...")
      - UpdateContent(add_section="EU Data Residency Requirements")

  - step: APAC contractor cannot access platform-team proposal
    project: platform-team
    role: null
    result: HTTP 403 Forbidden

  - step: US team lead copies proposal to APAC project
    project: platform-team
    role: admin
    operations:
      - CopyDocument(from="platform-team", to="platform-apac")

  - step: APAC contractor now accesses copied document
    project: platform-apac
    role: editor
    operations:
      - GetDocument(id="...")
      - UpdateContent(add_section="APAC Infrastructure Details")
```

### Example 7: Reviewer Role in Approval Workflow

**Scenario**: Document approval process for security policies requiring technical review before publication.

**Security Policy Author**:
```json
{
  "sub": "author@company.com",
  "email": "author@company.com",
  "name": "Policy Author",
  "hermes_roles": {
    "projects": {
      "security-team": "editor"
    }
  }
}
```

**Technical Reviewer**:
```json
{
  "sub": "reviewer@company.com",
  "email": "reviewer@company.com",
  "name": "Technical Reviewer",
  "hermes_roles": {
    "projects": {
      "security-team": "reviewer"
    }
  }
}
```

**Approval Workflow**:
```go
// 1. Author creates draft policy
func (w *Workflow) createDraftPolicy(ctx context.Context) error {
    doc, err := w.workspace.CreateDocument(ctx, &workspace.DocumentMetadata{
        Title: "Security Policy: Data Encryption at Rest",
        Project: "security-team",
        Status: "draft",
        Metadata: map[string]interface{}{
            "requires_review": true,
            "reviewers": []string{"reviewer@company.com"},
        },
    })
    if err != nil {
        return err
    }

    // Notify reviewer
    return w.workspace.SendEmail(ctx,
        []string{"reviewer@company.com"},
        "author@company.com",
        "Review Request: Data Encryption Policy",
        fmt.Sprintf("Please review document: %s", doc.ID),
    )
}

// 2. Reviewer adds comments and updates status (reviewer role can update status)
func (w *Workflow) reviewPolicy(ctx context.Context, docID string, approved bool) error {
    // Reviewer can update document metadata (status) but not content
    doc, err := w.workspace.GetDocument(ctx, docID)
    if err != nil {
        return err
    }

    doc.Status = "reviewed"
    doc.Metadata["review_status"] = approved
    doc.Metadata["reviewed_by"] = "reviewer@company.com"
    doc.Metadata["reviewed_at"] = time.Now().UTC().String()

    // Update document (PATCH endpoint)
    if err := w.workspace.UpdateDocumentMetadata(ctx, doc); err != nil {
        return err
    }

    // Notify author
    return w.workspace.SendEmailWithTemplate(ctx,
        []string{doc.CreatedBy},
        "review_complete",
        map[string]any{
            "document_title": doc.Title,
            "approved": approved,
            "reviewer": "reviewer@company.com",
            "comments_url": fmt.Sprintf("/documents/%s#comments", doc.ID),
        },
    )
}
```

## References

- **RFC-085**: API Provider and Remote Delegation
- **RFC-086**: Authentication and Bearer Token Management
- **RBAC**: Role-Based Access Control principles
- **OAuth 2.0**: RFC 6749 - Bearer Token Usage
