// Package api provides a workspace provider that delegates all operations to a remote Hermes instance via REST API.
//
// # Overview
//
// The API provider implements all 7 RFC-084 WorkspaceProvider interfaces by making HTTP requests to a remote
// Hermes server. This enables edge-to-central architectures where an edge Hermes instance can delegate
// operations to a central Hermes server.
//
// # Use Cases
//
// 1. Local Authoring with Central Tracking:
//   - Developer runs edge Hermes with local Git provider + API provider
//   - Local documents authored in Git
//   - Directory, permissions, notifications delegated to central Hermes
//
// 2. Multi-Tier Hermes Deployment:
//   - Edge Hermes instances in different regions/offices
//   - All delegate to central Hermes for coordination
//   - Central Hermes manages Google Workspace, identity, etc.
//
// 3. Offline-Capable Edge:
//   - Edge Hermes works offline with local provider
//   - When online, syncs metadata to central via API provider
//   - Central tracks all documents globally
//
// # Configuration Example
//
//	api_workspace {
//	  base_url   = "https://central.hermes.company.com"
//	  auth_token = env("HERMES_API_TOKEN")
//	  timeout    = "30s"
//	  tls_verify = true
//	  max_retries = 3
//	}
//
// # Architecture
//
//	┌─────────────────────────────────────────┐
//	│ Edge Hermes (Developer Laptop)          │
//	├─────────────────────────────────────────┤
//	│  Local Git Provider (primary)           │
//	│  API Provider (delegates to central)    │
//	└─────────────────┬───────────────────────┘
//	                  │ REST API
//	                  │ /api/v2/*
//	                  ▼
//	┌─────────────────────────────────────────┐
//	│ Central Hermes (Company Server)         │
//	├─────────────────────────────────────────┤
//	│  Google Workspace Provider              │
//	│  - Documents                             │
//	│  - Directory (People)                    │
//	│  - Groups (Teams)                        │
//	│  - Gmail (Notifications)                 │
//	└─────────────────────────────────────────┘
//
// # API Endpoints Required
//
// The remote Hermes instance must expose these REST API endpoints:
//
// Documents:
//   - GET    /api/v2/documents/:id
//   - GET    /api/v2/documents/uuid/:uuid
//   - POST   /api/v2/documents
//   - POST   /api/v2/documents/register
//   - POST   /api/v2/documents/:id/copy
//   - PUT    /api/v2/documents/:id/move
//   - DELETE /api/v2/documents/:id
//   - PATCH  /api/v2/documents/:id
//
// Content:
//   - GET  /api/v2/documents/:id/content
//   - GET  /api/v2/documents/uuid/:uuid/content
//   - PUT  /api/v2/documents/:id/content
//   - POST /api/v2/documents/batch/content
//   - POST /api/v2/documents/compare
//
// Revisions:
//   - GET  /api/v2/documents/:id/revisions
//   - GET  /api/v2/documents/:id/revisions/:revId
//   - GET  /api/v2/documents/:id/revisions/:revId/content
//   - POST /api/v2/documents/:id/revisions/:revId/keep
//   - GET  /api/v2/documents/uuid/:uuid/revisions/all
//
// Permissions:
//   - GET    /api/v2/documents/:id/permissions
//   - POST   /api/v2/documents/:id/permissions
//   - POST   /api/v2/documents/:id/permissions/domain
//   - DELETE /api/v2/documents/:id/permissions/:permId
//   - PATCH  /api/v2/documents/:id/permissions/:permId
//
// People:
//   - GET  /api/v2/people/search
//   - GET  /api/v2/people/:email
//   - GET  /api/v2/people/unified/:id
//   - POST /api/v2/people/resolve
//
// Teams:
//   - GET /api/v2/teams
//   - GET /api/v2/teams/:id
//   - GET /api/v2/teams/user/:email
//   - GET /api/v2/teams/:id/members
//
// Notifications:
//   - POST /api/v2/notifications/email
//   - POST /api/v2/notifications/email/template
//
// Capabilities (optional, for feature discovery):
//   - GET /api/v2/capabilities
//
// # Error Handling
//
// The provider includes:
//   - Automatic retry with exponential backoff
//   - Circuit breaker for network resilience
//   - Clear error messages with context
//   - Capability checking before operations
//
// # Performance Considerations
//
//   - HTTP/2 with connection pooling
//   - Configurable timeouts and retries
//   - Batch operations for bulk data transfer
//   - Capability discovery to avoid unnecessary requests
//
// # Security
//
//   - Bearer token authentication
//   - TLS with certificate verification
//   - Auth token not logged or serialized to JSON
//   - Configurable TLS verification for dev/test environments
package api
