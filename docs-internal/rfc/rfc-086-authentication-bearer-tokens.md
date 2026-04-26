---
id: RFC-086
title: Authentication and Bearer Token Management
date: 2025-11-11
type: RFC
subtype: Security
status: Proposed
tags: [authentication, oidc, bearer-tokens, security, delegation]
related:
  - RFC-084
  - RFC-085
  - RFC-007
---

# Authentication and Bearer Token Management

## Executive Summary

This RFC proposes an authentication strategy for delegated operations between local and remote Hermes instances. When a local Hermes delegates operations to a remote Hermes (RFC-085), it must handle user authentication and proxy bearer tokens to the remote for authenticated API calls.

**Key Benefits**:
- Shared OIDC provider enables consistent authentication across instances
- Bearer token proxying maintains user context in delegated operations
- Discovery flow allows dynamic OIDC configuration
- Supports multiple auth strategies (shared OIDC, discovery, M2M API keys)

**Related RFCs**:
- **RFC-084**: Provider Interface Refactoring (multi-backend architecture)
- **RFC-085**: API Provider and Remote Delegation (API implementation)
- **RFC-007**: Multi-Provider Auth Architecture (existing auth system)

## Context

### The Authentication Challenge

When a local Hermes delegates operations to a remote Hermes, authentication becomes critical:

1. **User Authentication**: Local Hermes needs authenticated access to remote
2. **Token Validation**: Remote must validate bearer tokens from local
3. **Context Preservation**: User identity must be maintained across instances
4. **Security**: Tokens must be securely transmitted and validated

**Problem Scenario**:
```
┌─────────────┐           ┌─────────────┐
│ User        │  ─login─> │ Local       │  ─API call─>  ┌─────────────┐
│ Browser     │           │ Hermes      │               │ Remote      │
└─────────────┘           └─────────────┘               │ Hermes      │
                                │                        └─────────────┘
                                │                               │
                          How to authenticate?            Need valid token!
```

## Proposed Solution

### Authentication Architecture

**Solution**: Local Hermes discovers OIDC provider from remote, redirects users for authentication, and proxies bearer tokens.

```
┌─────────────────────────────────────────────────────────────────┐
│                    Authentication Flow                           │
└─────────────────────────────────────────────────────────────────┘

Step 1: Discovery
┌──────────────┐                              ┌──────────────┐
│ Local Hermes │ ──GET /api/v2/auth/config──> │ Remote       │
│              │ <────Auth Config Response──── │ Hermes       │
└──────────────┘                              └──────────────┘
Response:
{
  "oidc_provider": "https://auth.example.com",
  "oidc_issuer": "https://auth.example.com",
  "oidc_client_id": "hermes-client",
  "auth_type": "oidc" // or "google", "okta"
}

Step 2: User Login (via Local)
┌──────┐         ┌──────────────┐         ┌──────────────┐
│ User │ ─login─>│ Local Hermes │ ─────>  │ OIDC Provider│
│      │         │ /login       │ redirect│ (Shared or   │
│      │ <──────────────────────┼─────────│  Remote)     │
└──────┘   redirect with        └──────────────┘
           bearer token

Step 3: Token Proxying (Delegated Operations)
┌──────────────┐                              ┌──────────────┐
│ Local Hermes │ ──API request with Bearer──> │ Remote       │
│              │    Authorization: Bearer XYZ  │ Hermes       │
│              │ <────API Response────────────│              │
└──────────────┘                              └──────────────┘
```

### Authentication Strategies

#### Strategy 1: Shared OIDC Provider (Recommended)

Both local and remote Hermes use the same OIDC provider:

**Local Hermes Configuration**:
```hcl
# Local Hermes configuration
providers {
  workspace = "local"
}

# Use shared OIDC provider
dex {
  issuer_url    = "https://auth.example.com"
  client_id     = "hermes-client"
  client_secret = env("OIDC_CLIENT_SECRET")
  redirect_url  = "https://local.hermes.example.com/auth/callback"
}

local_workspace {
  base_path = "/var/hermes/docs"

  # Delegate to remote with token proxying
  delegate {
    people_provider       = "remote_api"
    team_provider         = "remote_api"
    notification_provider = "remote_api"

    remote_api {
      base_url         = "https://central.hermes.example.com"
      proxy_user_token = true  # Proxy user bearer tokens
      api_key          = env("HERMES_M2M_KEY")  # For background jobs
      timeout          = "30s"
      tls_verify       = true
    }
  }
}
```

**Remote Hermes Configuration**:
```hcl
# Remote Hermes configuration (same OIDC provider)
providers {
  workspace = "google"
  search    = "algolia"
}

dex {
  issuer_url    = "https://auth.example.com"  # Same issuer!
  client_id     = "hermes-client"             # Same client!
  client_secret = env("OIDC_CLIENT_SECRET")
  redirect_url  = "https://central.hermes.example.com/auth/callback"
}

google_workspace {
  credentials_file = "credentials.json"
  domain          = "example.com"
}

# API endpoint configuration
server {
  # Accept bearer tokens from shared OIDC provider
  api_auth {
    validate_tokens = true
    trusted_issuers = ["https://auth.example.com"]
  }
}
```

**How it works**:
1. User logs into Local Hermes via shared OIDC provider
2. Local Hermes receives bearer token from OIDC provider
3. Local Hermes proxies bearer token in `Authorization` header to Remote
4. Remote Hermes validates token against same OIDC provider
5. Both instances trust tokens from same issuer

#### Strategy 2: Remote OIDC Discovery

Local Hermes discovers OIDC provider from remote and redirects users:

**Local Hermes Configuration**:
```hcl
# Local Hermes configuration
providers {
  workspace = "local"
}

# Discover auth config from remote
auth_discovery {
  remote_url = "https://central.hermes.example.com"
  # Local queries: GET /api/v2/auth/config
}

local_workspace {
  delegate {
    people_provider = "remote_api"

    remote_api {
      base_url         = "https://central.hermes.example.com"
      proxy_user_token = true
    }
  }
}
```

**Discovery Flow**:
```go
// Local Hermes discovers auth config on startup
type AuthConfigResponse struct {
    OIDCProvider string `json:"oidc_provider"`
    OIDCIssuer   string `json:"oidc_issuer"`
    OIDCClientID string `json:"oidc_client_id"`
    AuthType     string `json:"auth_type"` // "oidc", "google", "okta"
}

// Local Hermes makes discovery request
resp, err := http.Get("https://central.hermes.example.com/api/v2/auth/config")
var authConfig AuthConfigResponse
json.NewDecoder(resp.Body).Decode(&authConfig)

// Local configures itself to use discovered OIDC provider
localHermes.ConfigureAuth(authConfig)
```

**Remote Endpoint** (`/api/v2/auth/config`):
```go
func (s *Server) handleAuthConfig(w http.ResponseWriter, r *http.Request) {
    response := AuthConfigResponse{
        OIDCProvider: s.config.Dex.IssuerURL,
        OIDCIssuer:   s.config.Dex.IssuerURL,
        OIDCClientID: s.config.Dex.ClientID,
        AuthType:     "oidc",
    }

    json.NewEncoder(w).Encode(response)
}
```

#### Strategy 3: Machine-to-Machine API Key

For non-user operations (background jobs, indexing), use API key:

```hcl
local_workspace {
  delegate {
    people_provider = "remote_api"

    remote_api {
      base_url = "https://central.hermes.example.com"
      api_key  = env("HERMES_M2M_API_KEY")  # Machine-to-machine key
      # Used for background operations, not user requests
    }
  }
}
```

### Token Proxying Implementation

#### Local Hermes Handler

Handler extracts user token and passes in context:

```go
// Handler in Local Hermes
func (s *Server) handleShareDocument(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    // Extract user's bearer token from request
    authHeader := r.Header.Get("Authorization")
    token := strings.TrimPrefix(authHeader, "Bearer ")

    // Create context with token for delegation
    ctx = context.WithValue(ctx, "user_token", token)

    // Provider automatically proxies token when delegating
    err := s.workspace.ShareDocument(ctx, docID, email, role)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    w.WriteHeader(http.StatusOK)
}
```

#### Local Workspace Provider

Provider implementation proxies token to remote:

```go
type LocalWorkspaceProvider struct {
    storage   *LocalStorage
    remoteAPI *RemoteAPIClient
}

// PermissionProvider implementation (delegated)
func (p *LocalWorkspaceProvider) ShareDocument(ctx context.Context, providerID, email, role string) error {
    // Delegate to remote API with user's token
    return p.remoteAPI.ShareDocument(ctx, providerID, email, role)
}
```

#### Remote API Client

Client proxies user token in Authorization header:

```go
// RemoteAPIClient implementation
type RemoteAPIClient struct {
    baseURL    string
    httpClient *http.Client
    proxyToken bool   // Whether to proxy user tokens
    apiKey     string // Fallback API key for M2M
}

func (c *RemoteAPIClient) ShareDocument(ctx context.Context, providerID, email, role string) error {
    url := fmt.Sprintf("%s/api/v2/documents/%s/permissions", c.baseURL, providerID)

    body, _ := json.Marshal(map[string]string{
        "email": email,
        "role":  role,
    })

    req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
    req.Header.Set("Content-Type", "application/json")

    // Proxy user's bearer token if available
    if c.proxyToken {
        if userToken, ok := ctx.Value("user_token").(string); ok {
            req.Header.Set("Authorization", "Bearer "+userToken)
        } else if c.apiKey != "" {
            // Fallback to API key for background operations
            req.Header.Set("Authorization", "Bearer "+c.apiKey)
        }
    }

    resp, err := c.httpClient.Do(req)
    if err != nil {
        return fmt.Errorf("remote API error: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("remote returned status %d", resp.StatusCode)
    }

    return nil
}
```

## Security Considerations

### Token Validation

**Remote Hermes MUST validate bearer tokens**:

```go
// Remote Hermes middleware
func (s *Server) validateBearerToken(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        authHeader := r.Header.Get("Authorization")
        if authHeader == "" {
            http.Error(w, "missing authorization header", http.StatusUnauthorized)
            return
        }

        token := strings.TrimPrefix(authHeader, "Bearer ")

        // Validate token against OIDC provider
        claims, err := s.oidcValidator.Validate(r.Context(), token)
        if err != nil {
            http.Error(w, "invalid token", http.StatusUnauthorized)
            return
        }

        // Check token scope
        if !hasRequiredScope(claims, "hermes:read", "hermes:write") {
            http.Error(w, "insufficient scope", http.StatusForbidden)
            return
        }

        // Check token expiration
        if claims.Expiry.Before(time.Now()) {
            http.Error(w, "token expired", http.StatusUnauthorized)
            return
        }

        // Add claims to context
        ctx := context.WithValue(r.Context(), "user_claims", claims)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

### Token Scope Requirements

Bearer tokens must have appropriate scope:

```json
{
  "iss": "https://auth.example.com",
  "sub": "user@example.com",
  "aud": "hermes-client",
  "scope": "openid profile email hermes:read hermes:write",
  "exp": 1699999999
}
```

**Required Scopes**:
- `openid`: OIDC identity
- `profile`: User profile info
- `email`: User email
- `hermes:read`: Read access to documents
- `hermes:write`: Write access to documents

### CORS Configuration

Remote Hermes must allow CORS from local Hermes origins:

```go
// Remote Hermes CORS middleware
func (s *Server) corsMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Allow specific local Hermes instances
        allowedOrigins := []string{
            "https://local.hermes.example.com",
            "https://edge.hermes.example.com",
        }

        origin := r.Header.Get("Origin")
        if contains(allowedOrigins, origin) {
            w.Header().Set("Access-Control-Allow-Origin", origin)
            w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, PATCH")
            w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
            w.Header().Set("Access-Control-Allow-Credentials", "true")
        }

        if r.Method == "OPTIONS" {
            w.WriteHeader(http.StatusOK)
            return
        }

        next.ServeHTTP(w, r)
    })
}
```

**Note**: If using backend-to-backend communication (not browser-initiated), CORS is not required.

### Token Storage

**Local Hermes token storage**:

```go
// Store token in session (HttpOnly cookie)
func (s *Server) handleAuthCallback(w http.ResponseWriter, r *http.Request) {
    // Exchange code for token
    token, err := s.oidcClient.Exchange(r.Context(), r.URL.Query().Get("code"))
    if err != nil {
        http.Error(w, "failed to exchange code", http.StatusInternalServerError)
        return
    }

    // Store in HttpOnly cookie (not accessible to JavaScript)
    http.SetCookie(w, &http.Cookie{
        Name:     "hermes_token",
        Value:    token.AccessToken,
        Path:     "/",
        HttpOnly: true,  // Prevents XSS
        Secure:   true,  // HTTPS only
        SameSite: http.SameSiteStrictMode,
        MaxAge:   int(token.Expiry.Sub(time.Now()).Seconds()),
    })

    http.Redirect(w, r, "/", http.StatusFound)
}
```

### Refresh Tokens

**Local Hermes manages token refresh**:

```go
// Middleware to refresh expired tokens
func (s *Server) ensureValidToken(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        cookie, err := r.Cookie("hermes_token")
        if err != nil {
            http.Redirect(w, r, "/login", http.StatusFound)
            return
        }

        // Parse token to check expiration
        claims, err := s.parseToken(cookie.Value)
        if err != nil {
            http.Redirect(w, r, "/login", http.StatusFound)
            return
        }

        // Refresh if expiring soon (within 5 minutes)
        if claims.Expiry.Sub(time.Now()) < 5*time.Minute {
            newToken, err := s.refreshToken(r.Context(), cookie.Value)
            if err != nil {
                http.Redirect(w, r, "/login", http.StatusFound)
                return
            }

            // Update cookie with new token
            http.SetCookie(w, &http.Cookie{
                Name:     "hermes_token",
                Value:    newToken.AccessToken,
                Path:     "/",
                HttpOnly: true,
                Secure:   true,
                SameSite: http.SameSiteStrictMode,
                MaxAge:   int(newToken.Expiry.Sub(time.Now()).Seconds()),
            })
        }

        next.ServeHTTP(w, r)
    })
}
```

### Token Expiration Enforcement

**Remote Hermes enforces expiration**:

```go
func (s *Server) validateToken(ctx context.Context, token string) (*Claims, error) {
    claims, err := s.oidcValidator.Validate(ctx, token)
    if err != nil {
        return nil, err
    }

    // Enforce expiration
    if claims.Expiry.Before(time.Now()) {
        return nil, fmt.Errorf("token expired at %s", claims.Expiry)
    }

    return claims, nil
}
```

## Configuration Examples

### Full Configuration: Local with Shared OIDC

**Local Hermes (Edge Node)**:
```hcl
providers {
  workspace = "local"
  search    = "meilisearch"
}

# Shared OIDC provider
dex {
  issuer_url    = "https://auth.example.com"
  client_id     = "hermes-client"
  client_secret = env("OIDC_CLIENT_SECRET")
  redirect_url  = "https://local.hermes.example.com/auth/callback"
}

local_workspace {
  base_path = "/var/hermes/docs"

  delegate {
    people_provider       = "remote_api"
    team_provider         = "remote_api"
    notification_provider = "remote_api"

    remote_api {
      base_url         = "https://central.hermes.example.com"
      proxy_user_token = true  # Proxy user bearer tokens
      api_key          = env("HERMES_M2M_KEY")  # For background jobs
      timeout          = "30s"
      tls_verify       = true
    }
  }
}
```

**Remote Hermes (Central Node)**:
```hcl
providers {
  workspace = "google"
  search    = "algolia"
}

# Same OIDC provider as local instances
dex {
  issuer_url    = "https://auth.example.com"  # Same issuer!
  client_id     = "hermes-client"             # Same client!
  client_secret = env("OIDC_CLIENT_SECRET")
  redirect_url  = "https://central.hermes.example.com/auth/callback"
}

google_workspace {
  credentials_file = "credentials.json"
  domain          = "example.com"
}

# API endpoint configuration
server {
  # Accept bearer tokens from shared OIDC provider
  api_auth {
    validate_tokens = true
    trusted_issuers = ["https://auth.example.com"]

    # Required token scopes
    required_scopes = ["openid", "profile", "email", "hermes:read", "hermes:write"]
  }
}
```

## Authentication Endpoints

### GET /api/v2/auth/config

Discover OIDC configuration from remote:

**Request**:
```
GET /api/v2/auth/config HTTP/1.1
Host: central.hermes.example.com
```

**Response**:
```json
{
  "oidc_provider": "https://auth.example.com",
  "oidc_issuer": "https://auth.example.com",
  "oidc_client_id": "hermes-client",
  "auth_type": "oidc",
  "discovery_url": "https://auth.example.com/.well-known/openid-configuration"
}
```

### POST /api/v2/auth/validate

Validate bearer token (optional, for debugging):

**Request**:
```
POST /api/v2/auth/validate HTTP/1.1
Host: central.hermes.example.com
Authorization: Bearer eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9...

{}
```

**Response** (Success):
```json
{
  "valid": true,
  "subject": "user@example.com",
  "issuer": "https://auth.example.com",
  "expiry": "2025-11-12T10:30:00Z",
  "scopes": ["openid", "profile", "email", "hermes:read", "hermes:write"]
}
```

**Response** (Failure):
```json
{
  "valid": false,
  "error": "token expired"
}
```

## Implementation Plan

### Phase 1: Discovery Endpoint (Week 5)

- [ ] Create `/api/v2/auth/config` endpoint on remote Hermes
- [ ] Return OIDC provider configuration
- [ ] Add to API documentation

### Phase 2: Token Validation (Week 5)

- [ ] Implement token validation middleware on remote Hermes
- [ ] Validate tokens against OIDC provider
- [ ] Check token scope and expiration
- [ ] Add error handling and logging

### Phase 3: Token Proxying (Week 6)

- [ ] Update local Hermes handlers to extract bearer tokens
- [ ] Pass tokens in context to workspace provider
- [ ] Update RemoteAPIClient to proxy tokens
- [ ] Add fallback to API key for background operations

### Phase 4: Security Hardening (Week 6)

- [ ] Implement token refresh logic
- [ ] Add CORS configuration
- [ ] Secure token storage (HttpOnly cookies)
- [ ] Security audit and penetration testing

## Success Metrics

- [ ] User can log into local Hermes and access remote resources
- [ ] Bearer tokens successfully proxied to remote Hermes
- [ ] Remote Hermes validates tokens against OIDC provider
- [ ] Token refresh works seamlessly
- [ ] Zero security incidents during deployment
- [ ] Authentication latency < 100ms

## Risks & Mitigations

### Risk 1: Token Exposure

**Risk**: Bearer tokens exposed in transit or storage

**Mitigation**:
- HTTPS required (TLS 1.3)
- HttpOnly cookies (not accessible to JavaScript)
- Short token lifetime (15 minutes)
- Refresh tokens for renewal

### Risk 2: OIDC Provider Outage

**Risk**: OIDC provider down, users can't authenticate

**Mitigation**:
- Cached token validation (for short period)
- Multiple OIDC provider fallbacks
- Clear error messages
- Monitoring and alerting

### Risk 3: Token Validation Overhead

**Risk**: Token validation adds latency

**Mitigation**:
- Cache validated tokens (5 minutes)
- Async validation where possible
- Monitor validation latency
- Optimize OIDC provider queries

## References

- **RFC-084**: Provider Interface Refactoring (multi-backend architecture)
- **RFC-085**: API Provider and Remote Delegation (API implementation)
- **RFC-007**: Multi-Provider Auth Architecture (existing auth system)
- **OIDC Specification**: https://openid.net/specs/openid-connect-core-1_0.html

## Timeline

- **Week 5**: Discovery endpoint and token validation
- **Week 6**: Token proxying and security hardening
- **Total**: 2 weeks to production-ready authentication

---

**Status**: Proposed
**Dependencies**: RFC-084 (interfaces), RFC-085 (API provider)
**Next Steps**: Implement discovery endpoint and token validation middleware
