---
id: rfc-016
created: 2025-11-15
author: Hermes Team
project_id: hermes
doc_uuid: b57e9754-f7c0-4a45-9fd1-bd8a3e69a1be
status: Draft
title: Hermes Admin Interface - Comprehensive Management UI
type: RFC
subtype: Feature Design
tags: [admin, analytics, identity, migrations, monitoring, ui]
related: [RFC-015, RFC-014, RFC-010, RFC-005]
---

# RFC-016: Hermes Admin Interface - Comprehensive Management UI

## Executive Summary

This RFC proposes a comprehensive admin interface for Hermes that provides system administrators and operators with centralized management, monitoring, and analytics capabilities. The admin UI consolidates management of user identities, projects, migrations, indexer health, and documentation analytics into a unified interface.

**Key Features**:
- **User Identity Management**: View and manage unified user identities across multiple OAuth providers
- **Project Administration**: Project lifecycle management, permissions, and metadata
- **Migration Orchestration**: Schedule, monitor, and manage document migrations between providers
- **Indexer Health Dashboard**: Real-time monitoring of indexer pipeline health and performance
- **Documentation Analytics**: Insights into document usage, creation patterns, and team productivity

## Context

### Current State

Hermes currently has:
- Basic document management UI
- API endpoints for CRUD operations
- Limited visibility into system health
- No centralized admin interface
- No migration management UI
- No identity management beyond basic user lookup

### Problem Statement

**P1: No Centralized Administration**
- Administrators must use direct database queries or API calls
- No UI for common admin tasks
- Difficult to troubleshoot user identity issues
- No visibility into system health metrics

**P2: Limited Identity Management**
- Users can have multiple linked identities (Google, GitHub, IBM Verify, Okta)
- No UI to view or manage identity linkages
- Cannot resolve "Who is this user?" questions easily
- No audit trail for identity changes

**P3: No Migration Management**
- RFC-015 defines migration system but no UI
- Cannot schedule or monitor migrations without API calls
- No visibility into migration progress or failures
- Cannot retry failed migrations easily

**P4: No System Observability**
- Indexer health unknown until something breaks
- No metrics on document processing pipeline
- Cannot diagnose bottlenecks or failures
- No alerts for degraded performance

**P5: Limited Analytics**
- No insights into document usage patterns
- Cannot identify most active teams or projects
- No way to measure documentation productivity
- Missing data for decision-making

## Proposed Solution

### Architecture Overview

```text
┌─────────────────────────────────────────────────────────────────────┐
│ Hermes Admin Interface (React/TypeScript SPA)                        │
├─────────────────────────────────────────────────────────────────────┤
│                                                                       │
│ ┌──────────────┐ ┌──────────────┐ ┌──────────────┐ ┌─────────────┐ │
│ │   Identity   │ │   Projects   │ │  Migrations  │ │   Indexer   │ │
│ │  Management  │ │     Admin    │ │  Dashboard   │ │   Health    │ │
│ └──────────────┘ └──────────────┘ └──────────────┘ └─────────────┘ │
│                                                                       │
│ ┌──────────────┐ ┌──────────────┐ ┌──────────────┐ ┌─────────────┐ │
│ │Documentation │ │   Provider   │ │ System Logs  │ │   Settings  │ │
│ │  Analytics   │ │  Management  │ │  & Audit     │ │    &Config  │ │
│ └──────────────┘ └──────────────┘ └──────────────┘ └─────────────┘ │
└────────────────────────────┬──────────────────────────────────────────┘
                             │
                             │ REST API + WebSocket (real-time updates)
                             ▼
┌─────────────────────────────────────────────────────────────────────┐
│ Admin API Layer (internal/api/v2/admin/)                             │
├─────────────────────────────────────────────────────────────────────┤
│                                                                       │
│ ┌──────────────────────────────────────────────────────────────┐    │
│ │ Identity API                                                  │    │
│ │ - GET /api/v2/admin/users                                     │    │
│ │ - GET /api/v2/admin/users/:id/identities                      │    │
│ │ - POST /api/v2/admin/users/:id/link-identity                  │    │
│ │ - DELETE /api/v2/admin/users/:id/unlink-identity              │    │
│ └──────────────────────────────────────────────────────────────┘    │
│                                                                       │
│ ┌──────────────────────────────────────────────────────────────┐    │
│ │ Project API                                                   │    │
│ │ - GET /api/v2/admin/projects                                  │    │
│ │ - PUT /api/v2/admin/projects/:id                              │    │
│ │ - POST /api/v2/admin/projects/:id/archive                     │    │
│ │ - GET /api/v2/admin/projects/:id/permissions                  │    │
│ └──────────────────────────────────────────────────────────────┘    │
│                                                                       │
│ ┌──────────────────────────────────────────────────────────────┐    │
│ │ Migration API (RFC-015)                                       │    │
│ │ - POST /api/v2/admin/migrations                               │    │
│ │ - GET /api/v2/admin/migrations/:id                            │    │
│ │ - POST /api/v2/admin/migrations/:id/pause                     │    │
│ │ - WS /api/v2/admin/migrations/:id/stream                      │    │
│ └──────────────────────────────────────────────────────────────┘    │
│                                                                       │
│ ┌──────────────────────────────────────────────────────────────┐    │
│ │ Indexer Health API                                            │    │
│ │ - GET /api/v2/admin/indexer/health                            │    │
│ │ - GET /api/v2/admin/indexer/metrics                           │    │
│ │ - WS /api/v2/admin/indexer/stream                             │    │
│ └──────────────────────────────────────────────────────────────┘    │
│                                                                       │
│ ┌──────────────────────────────────────────────────────────────┐    │
│ │ Analytics API                                                 │    │
│ │ - GET /api/v2/admin/analytics/documents                       │    │
│ │ - GET /api/v2/admin/analytics/teams                           │    │
│ │ - GET /api/v2/admin/analytics/activity                        │    │
│ └──────────────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────────┐
│ Data Layer                                                            │
├─────────────────────────────────────────────────────────────────────┤
│ PostgreSQL: users, projects, documents, migrations, audit_logs       │
│ Prometheus: Metrics and time-series data                             │
│ Redpanda: Real-time event stream (for WebSocket updates)             │
└─────────────────────────────────────────────────────────────────────┘

```

## Feature Specifications

### 1. User Identity Management

#### 1.1 Unified Identity View

**Purpose**: Manage user identities across multiple OAuth providers (Google, GitHub, IBM Verify, Okta, Dex).

**UI Components**:

```text
┌────────────────────────────────────────────────────────────────┐
│ Users & Identity Management                   [Search users]  │
├────────────────────────────────────────────────────────────────┤
│                                                                 │
│ ┌──────────────────────────────────────────────────────────┐  │
│ │ User: Jacob Repp                           [Edit] [Merge] │  │
│ ├──────────────────────────────────────────────────────────┤  │
│ │                                                           │  │
│ │ Primary Identity:                                         │  │
│ │ ✓ jacob.repp@hashicorp.com (Google Workspace)           │  │
│ │                                                           │  │
│ │ Linked Identities:                                        │  │
│ │ ✓ jrepp@ibm.com (IBM Verify)                   [Unlink] │  │
│ │ ✓ jacob-repp (GitHub)                          [Unlink] │  │
│ │ ✓ jacob.repp@hashicorp.com (Okta)              [Unlink] │  │
│ │                                                           │  │
│ │ [+ Link New Identity]                                     │  │
│ │                                                           │  │
│ │ Recent Activity:                                          │  │
│ │ • Created RFC-015 (2 hours ago)                          │  │
│ │ • Updated RFC-014 (yesterday)                            │  │
│ │ • Commented on PRD 042 (3 days ago)                      │  │
│ │                                                           │  │
│ │ Permissions:                                              │  │
│ │ • Admin: hermes-core                                     │  │
│ │ • Editor: all-projects                                   │  │
│ │ • Viewer: public-docs                                    │  │
│ │                                                           │  │
│ │ Statistics:                                               │  │
│ │ • Documents Created: 42                                  │  │
│ │ • Documents Edited: 156                                  │  │
│ │ • Last Active: 2 hours ago                               │  │
│ └──────────────────────────────────────────────────────────┘  │
│                                                                 │
│ Identity Audit Log:                                            │
│ ┌──────────────────────────────────────────────────────────┐  │
│ │ Date          Action              Details           By   │  │
│ ├──────────────────────────────────────────────────────────┤  │
│ │ 2025-11-14    Linked GitHub       jacob-repp        Self │  │
│ │ 2025-11-10    Linked IBM Verify   jrepp@ibm.com    Admin│  │
│ │ 2025-11-01    Created Account     jacob.repp@...    Self │  │
│ └──────────────────────────────────────────────────────────┘  │
└────────────────────────────────────────────────────────────────┘
```

**API Endpoints**:

```go
// GET /api/v2/admin/users
type ListUsersResponse struct {
    Users      []UserSummary `json:"users"`
    TotalCount int           `json:"total_count"`
    Page       int           `json:"page"`
}

type UserSummary struct {
    ID              string    `json:"id"`
    PrimaryEmail    string    `json:"primary_email"`
    DisplayName     string    `json:"display_name"`
    PhotoURL        string    `json:"photo_url"`
    LinkedIdentities int      `json:"linked_identities"`
    LastActive      time.Time `json:"last_active"`
    DocumentCount   int       `json:"document_count"`
}

// GET /api/v2/admin/users/:id/identities
type UserIdentitiesResponse struct {
    User       UserDetail     `json:"user"`
    Identities []Identity     `json:"identities"`
    Activity   []Activity     `json:"recent_activity"`
    Permissions []Permission  `json:"permissions"`
    Statistics  UserStats     `json:"statistics"`
}

type Identity struct {
    ID           string    `json:"id"`
    Provider     string    `json:"provider"`     // "google", "github", "ibm-verify", "okta"
    Email        string    `json:"email"`
    ProviderID   string    `json:"provider_id"`  // OAuth subject ID
    Verified     bool      `json:"verified"`
    IsPrimary    bool      `json:"is_primary"`
    LinkedAt     time.Time `json:"linked_at"`
    LinkedBy     string    `json:"linked_by"`
}

// POST /api/v2/admin/users/:id/link-identity
type LinkIdentityRequest struct {
    Provider   string `json:"provider"`
    Email      string `json:"email"`
    ProviderID string `json:"provider_id"`
}

// DELETE /api/v2/admin/users/:id/identities/:identity_id
// Unlinks an identity from a user

```

**Database Schema Extension**:

```sql
-- Extend person_identity table (RFC-005) with audit fields
ALTER TABLE person_identity ADD COLUMN linked_by TEXT;
ALTER TABLE person_identity ADD COLUMN linked_at TIMESTAMP NOT NULL DEFAULT NOW();

-- Identity audit log
CREATE TABLE identity_audit_log (
    id BIGSERIAL PRIMARY KEY,
    person_id BIGINT NOT NULL REFERENCES person(id),
    action VARCHAR(50) NOT NULL,  -- 'identity_linked', 'identity_unlinked', 'identity_verified', 'primary_changed'
    identity_type VARCHAR(50),
    identity_value TEXT,
    provider_type VARCHAR(50),
    performed_by BIGINT REFERENCES person(id),
    performed_at TIMESTAMP NOT NULL DEFAULT NOW(),
    ip_address INET,
    user_agent TEXT,
    details JSONB
);

CREATE INDEX idx_identity_audit_person ON identity_audit_log(person_id, performed_at DESC);
CREATE INDEX idx_identity_audit_performed_by ON identity_audit_log(performed_by);
```

#### 1.2 Identity Merge Workflow

**Use Case**: When two user accounts are discovered to be the same person.

```text
┌────────────────────────────────────────────────────────────────┐
│ Merge User Identities                                          │
├────────────────────────────────────────────────────────────────┤
│                                                                 │
│ Source User:                                                   │
│ ┌──────────────────────────────────────────────────────────┐  │
│ │ jacob.repp@hashicorp.com                                 │  │
│ │ • 42 documents                                            │  │
│ │ • 3 linked identities                                     │  │
│ └──────────────────────────────────────────────────────────┘  │
│                                                                 │
│                            ⬇️                                   │
│                         [Merge]                                │
│                            ⬇️                                   │
│                                                                 │
│ Target User:                                                   │
│ ┌──────────────────────────────────────────────────────────┐  │
│ │ jrepp@ibm.com                                            │  │
│ │ • 18 documents                                            │  │
│ │ • 1 linked identity                                       │  │
│ └──────────────────────────────────────────────────────────┘  │
│                                                                 │
│ After Merge:                                                   │
│ • All documents (60) transferred to target                    │
│ • All identities (4) linked to target                         │
│ • Source account archived                                     │
│ • Audit log created                                           │
│                                                                 │
│ ⚠️  This action cannot be undone!                             │
│                                                                 │
│ [Cancel]                         [Confirm Merge]              │
└────────────────────────────────────────────────────────────────┘

```

### 2. Project Administration

#### 2.1 Project List & Management

```text
┌────────────────────────────────────────────────────────────────┐
│ Projects Administration                [+ New Project]         │
├────────────────────────────────────────────────────────────────┤
│                                                                 │
│ Filters: [All Projects ▾] [Status ▾] [Team ▾]   [Search...]   │
│                                                                 │
│ ┌──────────────────────────────────────────────────────────┐  │
│ │ Project            Documents  Team        Status   ⋮    │  │
│ ├──────────────────────────────────────────────────────────┤  │
│ │ Platform Core      245        Platform   Active   ⋮    │  │
│ │ AGF PoC            89         AGF        Active   ⋮    │  │
│ │ Q4 Roadmap         34         Leadership Active   ⋮    │  │
│ │ Legacy Migration   12         Platform   Archived ⋮    │  │
│ └──────────────────────────────────────────────────────────┘  │
│                                                                 │
│ Bulk Actions: [Archive] [Transfer Ownership] [Export]         │
└────────────────────────────────────────────────────────────────┘
```

#### 2.2 Project Detail View

```text
┌────────────────────────────────────────────────────────────────┐
│ Project: Platform Core                 [Edit] [Archive] [⚙️]  │
├────────────────────────────────────────────────────────────────┤
│                                                                 │
│ Overview:                                                      │
│ • UUID: 550e8400-e29b-41d4-a716-446655440000                   │
│ • Status: Active                                               │
│ • Created: 2024-01-15                                          │
│ • Owner: Platform Team                                         │
│ • Documents: 245                                               │
│                                                                 │
│ ┌──────────────────────────────────────────────────────────┐  │
│ │ Permissions                                      [+ Add]  │  │
│ ├──────────────────────────────────────────────────────────┤  │
│ │ Team/User              Role           Actions            │  │
│ ├──────────────────────────────────────────────────────────┤  │
│ │ Platform Team          Admin          [Edit] [Remove]   │  │
│ │ Engineering Team       Editor         [Edit] [Remove]   │  │
│ │ jacob.repp@...         Editor         [Edit] [Remove]   │  │
│ │ all@hashicorp.com      Viewer         [Edit] [Remove]   │  │
│ └──────────────────────────────────────────────────────────┘  │
│                                                                 │
│ ┌──────────────────────────────────────────────────────────┐  │
│ │ Document Types & Templates                               │  │
│ ├──────────────────────────────────────────────────────────┤  │
│ │ • RFC (125 docs) - Architecture decisions               │  │
│ │ • PRD (45 docs) - Product requirements                  │  │
│ │ • ADR (75 docs) - Technical decisions                   │  │
│ └──────────────────────────────────────────────────────────┘  │
│                                                                 │
│ ┌──────────────────────────────────────────────────────────┐  │
│ │ Recent Activity                                           │  │
│ ├──────────────────────────────────────────────────────────┤  │
│ │ • RFC-015 created by jacob.repp (2 hours ago)           │  │
│ │ • PRD 142 updated by alice@... (5 hours ago)            │  │
│ │ • RFC-014 published by bob@... (yesterday)              │  │
│ └──────────────────────────────────────────────────────────┘  │
│                                                                 │
│ ┌──────────────────────────────────────────────────────────┐  │
│ │ Storage Providers                                         │  │
│ ├──────────────────────────────────────────────────────────┤  │
│ │ Primary: Google Workspace (google-prod)                  │  │
│ │ Mirror:  S3 Archive (s3-archive) - 245/245 docs synced  │  │
│ │ Backup:  Local Edge NYC (local-edge-nyc) - enabled      │  │
│ └──────────────────────────────────────────────────────────┘  │
└────────────────────────────────────────────────────────────────┘

```

**API Endpoints**:

```go
// GET /api/v2/admin/projects
type ListProjectsResponse struct {
    Projects   []ProjectSummary `json:"projects"`
    TotalCount int              `json:"total_count"`
}

// GET /api/v2/admin/projects/:id
type ProjectDetailResponse struct {
    Project     ProjectDetail    `json:"project"`
    Permissions []Permission     `json:"permissions"`
    DocTypes    []DocumentType   `json:"document_types"`
    Activity    []Activity       `json:"recent_activity"`
    Providers   []ProviderInfo   `json:"storage_providers"`
    Statistics  ProjectStats     `json:"statistics"`
}

// PUT /api/v2/admin/projects/:id/permissions
type UpdatePermissionsRequest struct {
    Permissions []PermissionGrant `json:"permissions"`
}

type PermissionGrant struct {
    SubjectType string `json:"subject_type"` // "user", "team", "domain"
    SubjectID   string `json:"subject_id"`
    Role        string `json:"role"`         // "admin", "editor", "viewer"
}

// POST /api/v2/admin/projects/:id/archive
// Archives a project (soft delete, can be restored)

// POST /api/v2/admin/projects/:id/transfer
type TransferProjectRequest struct {
    NewOwnerID string `json:"new_owner_id"`
    Reason     string `json:"reason"`
}
```

### 3. Migration Dashboard (RFC-015 Integration)

#### 3.1 Migration List View

```text
┌────────────────────────────────────────────────────────────────┐
│ Migrations                              [+ New Migration]      │
├────────────────────────────────────────────────────────────────┤
│                                                                 │
│ Active Migrations:                                             │
│ ┌──────────────────────────────────────────────────────────┐  │
│ │ google-to-s3-rfcs                                        │  │
│ │ Running • Started 2h ago                    [Pause] [❌]  │  │
│ │ ████████████░░░░░░░░░░  245/500 (49%)                    │  │
│ │ Google Workspace → S3 Archive                             │  │
│ │ Est. completion: 2h 15m                                   │  │
│ │ [View Details]                                            │  │
│ └──────────────────────────────────────────────────────────┘  │
│                                                                 │
│ Scheduled Migrations:                                          │
│ ┌──────────────────────────────────────────────────────────┐  │
│ │ weekly-backup-to-azure                                   │  │
│ │ Scheduled • Next run: Tomorrow 2:00 AM      [Edit] [❌]  │  │
│ │ Google Workspace → Azure Blob                             │  │
│ │ Recurrence: Weekly (every Sunday)                         │  │
│ │ Last run: Success (500/500 docs)                          │  │
│ └──────────────────────────────────────────────────────────┘  │
│                                                                 │
│ Recent Migrations:                                             │
│ ┌──────────────────────────────────────────────────────────┐  │
│ │ Name              From     To         Status      Date   │  │
│ ├──────────────────────────────────────────────────────────┤  │
│ │ archive-old-docs  Google   S3         ✅ Complete  Nov 14│  │
│ │ mirror-to-edge    Google   Local      ✅ Complete  Nov 10│  │
│ │ office365-test    Google   O365       ❌ Failed    Nov 8 │  │
│ └──────────────────────────────────────────────────────────┘  │
└────────────────────────────────────────────────────────────────┘

```

#### 3.2 Create Migration Wizard

```text
┌────────────────────────────────────────────────────────────────┐
│ Create New Migration                          Step 1 of 4      │
├────────────────────────────────────────────────────────────────┤
│                                                                 │
│ Migration Name:                                                │
│ [archive-q3-2024-docs                                    ]    │
│                                                                 │
│ Source Provider:                                               │
│ [Google Workspace (google-prod)              ▾]               │
│                                                                 │
│ Destination Provider:                                          │
│ [S3 Archive (s3-archive)                     ▾]               │
│                                                                 │
│ Strategy:                                                      │
│ ○ Copy   (keep source, create copy in destination)            │
│ ○ Move   (remove from source after successful migration)      │
│ ● Mirror (keep both in sync continuously)                     │
│                                                                 │
│ [Cancel]                                           [Next →]   │
└────────────────────────────────────────────────────────────────┘

┌────────────────────────────────────────────────────────────────┐
│ Create New Migration                          Step 2 of 4      │
├────────────────────────────────────────────────────────────────┤
│                                                                 │
│ Select Documents:                                              │
│                                                                 │
│ Filter Criteria:                                               │
│ ┌──────────────────────────────────────────────────────────┐  │
│ │ Document Type:    [RFC                          ▾]       │  │
│ │ Status:           [Published                    ▾]       │  │
│ │ Project:          [All Projects                 ▾]       │  │
│ │ Modified Before:  [2024-09-30                   📅]      │  │
│ │ Tags:             [quarterly-review                ]     │  │
│ └──────────────────────────────────────────────────────────┘  │
│                                                                 │
│ Preview: 127 documents match criteria                          │
│ [Preview List]                                                 │
│                                                                 │
│ [← Back]                                           [Next →]   │
└────────────────────────────────────────────────────────────────┘

┌────────────────────────────────────────────────────────────────┐
│ Create New Migration                          Step 3 of 4      │
├────────────────────────────────────────────────────────────────┤
│                                                                 │
│ Schedule:                                                      │
│                                                                 │
│ ● Start Immediately                                            │
│ ○ Schedule for Later                                           │
│   Date: [2024-11-20 📅]  Time: [02:00 🕐]                     │
│                                                                 │
│ ○ Recurring Schedule                                           │
│   Cron Expression: [0 2 * * 0                            ]    │
│   (Every Sunday at 2:00 AM)                                    │
│                                                                 │
│ Configuration:                                                 │
│ Concurrency:     [5              ▾] parallel workers          │
│ Batch Size:      [100            ▾] documents per batch       │
│ Dry Run:         [☐] Test migration without writing           │
│                                                                 │
│ Transform Rules: (optional)                                    │
│ [+ Add Transformation]                                         │
│                                                                 │
│ [← Back]                                           [Next →]   │
└────────────────────────────────────────────────────────────────┘

┌────────────────────────────────────────────────────────────────┐
│ Create New Migration                          Step 4 of 4      │
├────────────────────────────────────────────────────────────────┤
│                                                                 │
│ Review & Confirm:                                              │
│                                                                 │
│ Name:         archive-q3-2024-docs                             │
│ Source:       Google Workspace (google-prod)                   │
│ Destination:  S3 Archive (s3-archive)                          │
│ Strategy:     Mirror                                           │
│ Documents:    127 documents                                    │
│ Schedule:     Start Immediately                                │
│                                                                 │
│ Estimated Duration: ~15 minutes                                │
│ Estimated Storage:  2.4 GB                                     │
│                                                                 │
│ ⚠️  Important:                                                 │
│ • Migration cannot be cancelled once started                   │
│ • Validation will run after completion                         │
│ • Rollback is available for 7 days                            │
│                                                                 │
│ [← Back]                            [Create Migration]        │
└────────────────────────────────────────────────────────────────┘
```

#### 3.3 Real-Time Migration Monitoring

**WebSocket Stream** for live updates:

```javascript
// Frontend: Connect to migration stream
const ws = new WebSocket('wss://hermes.example.com/api/v2/admin/migrations/123/stream');

ws.onmessage = (event) => {
  const update = JSON.parse(event.data);
  // {
  //   "type": "progress",
  //   "migrated_documents": 250,
  //   "failed_documents": 2,
  //   "current_document": "RFC-015",
  //   "throughput_docs_per_min": 45
  // }
  updateProgressBar(update);
};

```

### 4. Indexer Health Dashboard

#### 4.1 Pipeline Health Overview

```text
┌────────────────────────────────────────────────────────────────┐
│ Indexer Health Dashboard                        [Refresh]      │
├────────────────────────────────────────────────────────────────┤
│                                                                 │
│ Overall Status: ✅ Healthy                                     │
│                                                                 │
│ ┌──────────────────────────────────────────────────────────┐  │
│ │ Pipeline Stage          Status    Throughput   Lag       │  │
│ ├──────────────────────────────────────────────────────────┤  │
│ │ Document Revisions      ✅ OK     45 docs/min   0.2s     │  │
│ │ Outbox Relay            ✅ OK     45 msg/min    0.1s     │  │
│ │ Kafka Topic             ✅ OK     45 msg/min    23 msgs  │  │
│ │ Indexer Workers (3)     ✅ OK     45 docs/min   0.5s     │  │
│ │ Search Index (Meili)    ✅ OK     45 docs/min   1.2s     │  │
│ │ Embeddings              ⚠️  Slow  12 docs/min   45 msgs  │  │
│ │ LLM Summaries           ⚠️  Slow  8 docs/min    102 msgs │  │
│ └──────────────────────────────────────────────────────────┘  │
│                                                                 │
│ Alerts:                                                        │
│ ⚠️  Embeddings service slow (response time > 5s)              │
│ ⚠️  LLM summary backlog growing (102 pending)                 │
│                                                                 │
│ ┌──────────────────────────────────────────────────────────┐  │
│ │ Throughput (last hour)                                    │  │
│ │                                                           │  │
│ │   60 │           ╱╲                                      │  │
│ │      │          ╱  ╲        ╱╲                          │  │
│ │   40 │     ╱╲  ╱    ╲      ╱  ╲    ╱╲                  │  │
│ │      │    ╱  ╲╱      ╲    ╱    ╲  ╱  ╲                │  │
│ │   20 │   ╱            ╲  ╱      ╲╱    ╲               │  │
│ │      │  ╱              ╲╱             ╲              │  │
│ │    0 └────────────────────────────────────────────────┘  │  │
│ │       12pm    2pm     4pm     6pm     8pm    10pm      │  │
│ └──────────────────────────────────────────────────────────┘  │
│                                                                 │
│ ┌──────────────────────────────────────────────────────────┐  │
│ │ Error Rate (last hour)                                    │  │
│ │ 2.3% (5 failed / 215 total)                              │  │
│ │                                                           │  │
│ │ Recent Errors:                                            │  │
│ │ • OpenAI API rate limit (3 occurrences)                  │  │
│ │ • Meilisearch timeout (1 occurrence)                     │  │
│ │ • Document not found (1 occurrence)                      │  │
│ └──────────────────────────────────────────────────────────┘  │
└────────────────────────────────────────────────────────────────┘
```

#### 4.2 Kafka/Redpanda Consumer Lag

```text
┌────────────────────────────────────────────────────────────────┐
│ Kafka Consumer Groups                                          │
├────────────────────────────────────────────────────────────────┤
│                                                                 │
│ ┌──────────────────────────────────────────────────────────┐  │
│ │ Consumer Group: hermes-indexer-workers                    │  │
│ ├──────────────────────────────────────────────────────────┤  │
│ │ Topic: hermes.document-revisions                          │  │
│ │                                                           │  │
│ │ Partition  Current Offset  Log End  Lag    Consumer      │  │
│ ├──────────────────────────────────────────────────────────┤  │
│ │ 0          12,450          12,458    8      worker-1     │  │
│ │ 1          15,234          15,242    8      worker-2     │  │
│ │ 2          11,098          11,105    7      worker-3     │  │
│ │                                                           │  │
│ │ Total Lag: 23 messages                                    │  │
│ │ Lag Status: ✅ Healthy (< 100 messages)                  │  │
│ └──────────────────────────────────────────────────────────┘  │
│                                                                 │
│ ┌──────────────────────────────────────────────────────────┐  │
│ │ Consumer Group: hermes-migration-workers                  │  │
│ ├──────────────────────────────────────────────────────────┤  │
│ │ Topic: hermes.migrations                                  │  │
│ │                                                           │  │
│ │ Partition  Current Offset  Log End  Lag    Consumer      │  │
│ ├──────────────────────────────────────────────────────────┤  │
│ │ 0          5,234           5,350     116    worker-1     │  │
│ │ 1          4,891           4,945     54     worker-2     │  │
│ │                                                           │  │
│ │ Total Lag: 170 messages                                   │  │
│ │ Lag Status: ⚠️  Warning (> 100 messages)                 │  │
│ └──────────────────────────────────────────────────────────┘  │
└────────────────────────────────────────────────────────────────┘

```

**API Endpoints**:

```go
// GET /api/v2/admin/indexer/health
type IndexerHealthResponse struct {
    OverallStatus string              `json:"overall_status"`  // "healthy", "degraded", "unhealthy"
    Stages        []PipelineStage     `json:"stages"`
    Alerts        []Alert             `json:"alerts"`
    Metrics       IndexerMetrics      `json:"metrics"`
}

type PipelineStage struct {
    Name        string  `json:"name"`
    Status      string  `json:"status"`
    Throughput  float64 `json:"throughput_docs_per_min"`
    Lag         string  `json:"lag"`
    ErrorRate   float64 `json:"error_rate"`
}

// GET /api/v2/admin/indexer/kafka/consumer-lag
type KafkaConsumerLagResponse struct {
    ConsumerGroups []ConsumerGroup `json:"consumer_groups"`
}

type ConsumerGroup struct {
    Name       string              `json:"name"`
    Topic      string              `json:"topic"`
    Partitions []PartitionLag      `json:"partitions"`
    TotalLag   int64               `json:"total_lag"`
    LagStatus  string              `json:"lag_status"`
}

// WebSocket: /api/v2/admin/indexer/stream
// Real-time metrics streaming
```

### 5. Documentation Analytics

#### 5.1 Overview Dashboard

```text
┌────────────────────────────────────────────────────────────────┐
│ Documentation Analytics                    Last 30 days ▾      │
├────────────────────────────────────────────────────────────────┤
│                                                                 │
│ ┌────────────────┐ ┌────────────────┐ ┌────────────────┐     │
│ │ Total Documents│ │ Created (30d)  │ │Active Authors  │     │
│ │    5,234       │ │      +127      │ │      45        │     │
│ │    +2.4%       │ │      +15%      │ │      +3        │     │
│ └────────────────┘ └────────────────┘ └────────────────┘     │
│                                                                 │
│ ┌──────────────────────────────────────────────────────────┐  │
│ │ Document Creation Trend                                   │  │
│ │                                                           │  │
│ │   150│                              ╱╲                   │  │
│ │      │                         ╱╲  ╱  ╲                 │  │
│ │   100│                    ╱╲  ╱  ╲╱    ╲                │  │
│ │      │               ╱╲  ╱  ╲╱           ╲    ╱╲        │  │
│ │    50│          ╱╲  ╱  ╲╱                ╲  ╱  ╲       │  │
│ │      │         ╱  ╲╱                      ╲╱    ╲      │  │
│ │     0└────────────────────────────────────────────────┘  │  │
│ │       Week 1  Week 2  Week 3  Week 4  Week 5  Week 6   │  │
│ └──────────────────────────────────────────────────────────┘  │
│                                                                 │
│ ┌──────────────────────────────────────────────────────────┐  │
│ │ Most Active Teams                                         │  │
│ ├──────────────────────────────────────────────────────────┤  │
│ │ Platform Engineering     ████████████░░░░░  89 docs      │  │
│ │ Product Management       ████████░░░░░░░░░  56 docs      │  │
│ │ Infrastructure           ██████░░░░░░░░░░░  42 docs      │  │
│ │ Data Science             ████░░░░░░░░░░░░░  28 docs      │  │
│ └──────────────────────────────────────────────────────────┘  │
│                                                                 │
│ ┌──────────────────────────────────────────────────────────┐  │
│ │ Document Types Distribution                               │  │
│ ├──────────────────────────────────────────────────────────┤  │
│ │ RFC (45%)      ████████████████░░░░░░░  2,345 docs       │  │
│ │ PRD (30%)      ██████████░░░░░░░░░░░░░  1,570 docs       │  │
│ │ ADR (15%)      █████░░░░░░░░░░░░░░░░░░    785 docs       │  │
│ │ Guide (10%)    ███░░░░░░░░░░░░░░░░░░░░    534 docs       │  │
│ └──────────────────────────────────────────────────────────┘  │
└────────────────────────────────────────────────────────────────┘

```

#### 5.2 Team Productivity View

```text
┌────────────────────────────────────────────────────────────────┐
│ Team: Platform Engineering                                     │
├────────────────────────────────────────────────────────────────┤
│                                                                 │
│ ┌────────────────┐ ┌────────────────┐ ┌────────────────┐     │
│ │ Documents      │ │ Active Members │ │ Avg. Time to   │     │
│ │    245         │ │      12        │ │ Publish: 4 days│     │
│ └────────────────┘ └────────────────┘ └────────────────┘     │
│                                                                 │
│ ┌──────────────────────────────────────────────────────────┐  │
│ │ Top Contributors (last 30 days)                           │  │
│ ├──────────────────────────────────────────────────────────┤  │
│ │ Author              Created  Updated  Comments           │  │
│ ├──────────────────────────────────────────────────────────┤  │
│ │ jacob.repp@...      15       42       89                 │  │
│ │ alice@...           12       38       67                 │  │
│ │ bob@...             10       25       45                 │  │
│ └──────────────────────────────────────────────────────────┘  │
│                                                                 │
│ ┌──────────────────────────────────────────────────────────┐  │
│ │ Document Status Breakdown                                 │  │
│ ├──────────────────────────────────────────────────────────┤  │
│ │ Published         ██████████████░░░░░░  145 (59%)        │  │
│ │ In Review         █████░░░░░░░░░░░░░░░   48 (20%)        │  │
│ │ Draft             ████░░░░░░░░░░░░░░░░   38 (16%)        │  │
│ │ Deprecated        ██░░░░░░░░░░░░░░░░░░   14 (5%)         │  │
│ └──────────────────────────────────────────────────────────┘  │
│                                                                 │
│ ┌──────────────────────────────────────────────────────────┐  │
│ │ Popular Documents (views, last 30 days)                   │  │
│ ├──────────────────────────────────────────────────────────┤  │
│ │ 1. RFC-010: Provider Interface (1,245 views)             │  │
│ │ 2. RFC-014: Event-Driven Indexer (987 views)             │  │
│ │ 3. RFC-015: S3 Storage Backend (756 views)               │  │
│ └──────────────────────────────────────────────────────────┘  │
└────────────────────────────────────────────────────────────────┘
```

**API Endpoints**:

```go
// GET /api/v2/admin/analytics/overview
type AnalyticsOverviewResponse struct {
    TotalDocuments   int               `json:"total_documents"`
    Created30d       int               `json:"created_30d"`
    ActiveAuthors    int               `json:"active_authors"`
    CreationTrend    []DataPoint       `json:"creation_trend"`
    ActiveTeams      []TeamActivity    `json:"active_teams"`
    DocTypeDistribution []DocTypeStats `json:"doc_type_distribution"`
}

// GET /api/v2/admin/analytics/teams/:id
type TeamAnalyticsResponse struct {
    Team            TeamSummary       `json:"team"`
    Statistics      TeamStats         `json:"statistics"`
    TopContributors []Contributor     `json:"top_contributors"`
    StatusBreakdown []StatusCount     `json:"status_breakdown"`
    PopularDocs     []PopularDoc      `json:"popular_documents"`
}

// GET /api/v2/admin/analytics/documents/:id/views
type DocumentViewsResponse struct {
    DocumentID string      `json:"document_id"`
    Views30d   int         `json:"views_30d"`
    Viewers    []string    `json:"unique_viewers"`
    ViewTrend  []DataPoint `json:"view_trend"`
}

```

## Implementation Plan

### Phase 1: Foundation (Weeks 1-2)

**Week 1: Admin API Scaffolding**
- Create admin API routes structure (`internal/api/v2/admin/`)
- Implement authentication & authorization middleware
- Add admin role checks
- Create base response types

**Week 2: Database Schema**
- Add identity audit log table
- Add project permissions tracking
- Add analytics views (materialized for performance)
- Create indexes for admin queries

### Phase 2: Identity Management (Weeks 3-4)

**Week 3: Identity API**
- Implement user list endpoint
- Implement identity view endpoint
- Implement link/unlink identity endpoints
- Add identity audit logging

**Week 4: Identity UI**
- Build user list component
- Build identity detail view
- Build identity merge workflow
- Add audit log viewer

### Phase 3: Project Administration (Weeks 5-6)

**Week 5: Project API**
- Implement project list endpoint
- Implement project detail endpoint
- Implement permissions management
- Add project archive/restore

**Week 6: Project UI**
- Build project list component
- Build project detail view
- Build permissions editor
- Add bulk operations

### Phase 4: Migration Dashboard (Weeks 7-8)

**Week 7: Migration API Extensions**
- Add migration list endpoint with filtering
- Add WebSocket streaming for real-time updates
- Add schedule management endpoints
- Add retry/cancel endpoints

**Week 8: Migration UI**
- Build migration list view
- Build migration creation wizard
- Build real-time monitoring view
- Add Kafka lag monitoring

### Phase 5: Indexer Health (Weeks 9-10)

**Week 9: Health Monitoring API**
- Implement indexer health endpoint
- Implement Kafka consumer lag endpoint
- Add WebSocket streaming for metrics
- Add alert generation

**Week 10: Health Dashboard UI**
- Build pipeline status view
- Build Kafka lag monitoring
- Build metrics charts (throughput, error rate)
- Add alert notifications

### Phase 6: Analytics (Weeks 11-12)

**Week 11: Analytics API**
- Implement overview endpoint
- Implement team analytics endpoint
- Implement document views tracking
- Create analytics database views

**Week 12: Analytics UI**
- Build overview dashboard
- Build team productivity view
- Build document popularity charts
- Add export functionality

### Phase 7: Polish & Launch (Weeks 13-14)

**Week 13: Integration & Testing**
- End-to-end testing
- Performance optimization
- Security audit
- Documentation

**Week 14: Production Rollout**
- Deploy to staging
- User acceptance testing
- Deploy to production
- Monitor and iterate

## Success Metrics

### Adoption Metrics
- 80%+ of admins use UI instead of direct DB queries within 3 months
- 50%+ of migrations created via UI within 1 month
- < 5 min average time to diagnose user identity issues

### Performance Metrics
- Admin UI loads in < 2s (p95)
- Real-time updates have < 500ms latency
- Analytics queries return in < 1s

### Operational Metrics
- 90% reduction in time spent on manual identity management
- 50% reduction in migration-related support tickets
- Zero indexer issues go undetected > 5 minutes

## Security Considerations

### Access Control

```go
// Middleware for admin routes
func AdminAuthMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        user := auth.GetCurrentUser(c)

        // Check if user has admin role
        if !user.HasRole("admin") {
            c.JSON(403, gin.H{"error": "admin access required"})
            c.Abort()
            return
        }

        c.Next()
    }
}

// Fine-grained permissions
type AdminPermission string

const (
    PermissionViewUsers      AdminPermission = "admin:users:view"
    PermissionEditUsers      AdminPermission = "admin:users:edit"
    PermissionViewProjects   AdminPermission = "admin:projects:view"
    PermissionEditProjects   AdminPermission = "admin:projects:edit"
    PermissionManageMigrations AdminPermission = "admin:migrations:manage"
    PermissionViewAnalytics  AdminPermission = "admin:analytics:view"
)
```

### Audit Logging

All admin actions are logged:

```sql
CREATE TABLE admin_audit_log (
    id BIGSERIAL PRIMARY KEY,
    performed_by BIGINT NOT NULL REFERENCES person(id),
    action VARCHAR(100) NOT NULL,
    resource_type VARCHAR(50) NOT NULL,  -- 'user', 'project', 'migration'
    resource_id TEXT NOT NULL,
    changes JSONB,
    ip_address INET,
    user_agent TEXT,
    performed_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_admin_audit_performed_by ON admin_audit_log(performed_by, performed_at DESC);
CREATE INDEX idx_admin_audit_resource ON admin_audit_log(resource_type, resource_id);

```

### Rate Limiting

```go
// Rate limit admin API endpoints
adminRouter.Use(ratelimit.Middleware(ratelimit.Config{
    Rate:   100, // requests
    Period: time.Minute,
    Key:    func(c *gin.Context) string {
        user := auth.GetCurrentUser(c)
        return fmt.Sprintf("admin:%s", user.ID)
    },
}))
```

## References

- **RFC-015**: S3 Storage Backend and Document Migration System
- **RFC-014**: Event-Driven Document Indexer with Pipeline Rulesets
- **RFC-010**: Provider Interface Refactoring - Multi-Backend Document Model
- **RFC-005**: Document Search Index Outbox Pattern
- **RFC-008**: Outbox Pattern for Document Synchronization

## Open Questions

1. **Analytics Data Retention**: How long to keep detailed analytics data?
   - **Proposal**: 90 days detailed, 1 year aggregated

2. **Real-time Update Frequency**: How often to push WebSocket updates?
   - **Proposal**: Every 1 second for active migrations, every 5 seconds for metrics

3. **Admin Role Granularity**: Should we support fine-grained admin permissions?
   - **Proposal**: Yes, implement RBAC with permission groups

4. **Export Formats**: What formats for analytics export?
   - **Proposal**: CSV, JSON, PDF reports

5. **Mobile Responsiveness**: Should admin UI work on mobile?
   - **Proposal**: Yes, but desktop-first design

---

**Document ID**: RFC-016
**Status**: Draft
**Author**: Engineering Team
**Created**: 2025-11-15
**Last Updated**: 2025-11-15