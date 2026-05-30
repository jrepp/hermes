---
id: memo-067
title: "SharePoint Provider Porting Progress"
status: Final
created: 2026-05-30
date: 2026-05-30
author: Hermes Team
project_id: hermes
doc_uuid: 5078dad9-dabf-4c5b-8dd3-85bd3c032e49
type: Memo
subtype: Milestone
tags: [sharepoint, provider-abstraction, workspace, docid, rebase]
related:
  - ADR-009
  - ADR-017
  - ADR-018
---

# MEMO-067: SharePoint Provider Porting Progress

> This memo records the post-rebase SharePoint porting state after reconciling upstream SharePoint work with Hermes' provider abstraction model. It captures what has been moved behind provider interfaces, what compatibility shims remain, and which V2 handler issues still block full server compilation.

## Background

After rebasing `jrepp/hermes:main` onto `hashicorp-forge/hermes:main`, upstream SharePoint support landed as a parallel path: `pkg/sharepointhelper.Service`, Microsoft auth middleware, and V2 handler branches referenced `srv.SharePoint`, `srv.GWService`, and concrete Google/SharePoint helpers directly.

That conflicted with the current architectural direction in ADR-009 and ADR-017: external integrations should go through provider interfaces, and V2 handlers should receive dependencies through `server.Server` rather than concrete SDK clients.

## Completed Porting Work

- Added a SharePoint workspace provider adapter at `pkg/workspace/adapters/sharepoint/` implementing `workspace.WorkspaceProvider`.
- Added `providers.workspace = "sharepoint"` startup support in `internal/cmd/commands/server/server.go`.
- Restored HCL SharePoint config support via `internal/config.SharePointConfig`.
- Added `docid.ProviderTypeSharePoint`.
- Implemented cross-drive SharePoint provider IDs using:

```text
sharepoint:site:<site-id>:drive:<drive-id>:item:<item-id>
```

- Added SharePoint Hermes UUID persistence in `pkg/sharepointhelper/hermes_uuid.go`:
  - `GetHermesUUID`
  - `SetHermesUUID`
  - `FindFileByHermesUUID`
- Updated the SharePoint workspace adapter so `GetDocument`, `CreateDocumentWithUUID`, `CopyDocument`, `RegisterDocument`, and `GetDocumentByUUID` use persisted `HermesUuid` metadata instead of generating transient UUIDs on reads.
- Normalized `internal/email` around a context-aware provider-style sender shape aligned with `workspace.NotificationProvider`.
- Restored `pkg/hashicorpdocs/locked.go` to Google-only lock checking; SharePoint should not fork that helper.
- Cleaned `web/web.go` runtime config so it reports `auth_provider = "microsoft"` and `workspace_provider = "sharepoint"` without stale `Microsoft` config payload fields.

## Temporary Compatibility Shims

Some V2 handlers still contain legacy direct-service paths. To keep the porting incremental, `internal/server.Server` temporarily exposes:

- `GWService *google.Service`
- `SharePoint *sharepointhelper.Service`
- `GetEmailSender()`
- `IsSharePoint()`
- `NewDocumentByFileID()`

These are compatibility shims, not the target architecture. They should be removed after the remaining V2 handlers use `WorkspaceProvider` capabilities directly.

## Current Validation

Focused package validation passes:

```bash
go test ./internal/email ./pkg/hashicorpdocs ./web ./pkg/workspace/adapters/sharepoint ./pkg/sharepointhelper ./internal/config ./internal/auth
```

Full V2/server compilation is still blocked by unrelated post-rebase handler drift in:

- `internal/api/v2/documents.go`
- `internal/api/v2/drafts.go`
- `internal/api/v2/documents_related_resources.go`

Examples of remaining issues include missing local variables after merged sharing/email flows, stale handler call signatures, and direct `GWService`/`SharePoint` calls that still need provider-backed replacements.

## Next Porting Steps

1. Replace V2 direct `srv.GWService` and `srv.SharePoint` calls with `srv.WorkspaceProvider` methods.
2. Introduce focused provider capability helpers only where generic `workspace.WorkspaceProvider` is insufficient.
3. Fix the remaining V2 handler merge drift until `go test ./internal/api/v2 ./internal/cmd/commands/server ./internal/server` compiles.
4. Remove `server.Server` compatibility shims once no V2 handler depends on them.
5. Add integration coverage for SharePoint provider ID parsing and Hermes UUID field persistence using a mocked Graph transport.

## References

- [ADR-009: Provider Abstraction Architecture](../adr/adr-009-provider-abstraction-architecture.md)
- [ADR-017: V2 API Provider Abstraction Pattern](../adr/adr-017-api-refactoring-and-testing-strategy.md)
- [ADR-018: Document Identification System](../adr/adr-018-document-identification-system.md)
- `pkg/workspace/adapters/sharepoint/`
- `pkg/sharepointhelper/hermes_uuid.go`
