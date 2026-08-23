---
id: memo-068
title: "SharePoint Support Leaked Past the Provider Interfaces"
status: Investigation
created: 2026-08-23
author: Hermes Team
project_id: hermes
doc_uuid: 5798d116-058d-4dd3-9eb0-b6026c089e65
type: Memo
subtype: Investigation
tags: [providers, sharepoint, architecture, adr-009, technical-debt]
related:
  - ADR-009
  - ADR-017
---

# SharePoint Support Leaked Past the Provider Interfaces

ADR-009 requires every external integration to go through a provider interface
— `auth.Provider`, `workspace.Provider`, `search.Provider` — selected by
configuration. SharePoint support (`d18e2109`) did not do that. It added a
second identity to the domain models, a provider boolean to the persistence
layer, and provider-name branching to request handlers.

This memo records what leaked, what has been pulled back, and what has not.

## Why it matters beyond tidiness

The leak is not only a shape problem. Every defect fixed in this area during the
audit was a direct consequence of it:

| Symptom | Cause |
|---|---|
| Every document query failed on a migrated database | `documents.file_id` declared by the model, never migrated |
| Revisions written with an empty primary key | `FileRevisionID` required by `Create` and tagged `gorm:"-"` |
| A second SharePoint document could not be created | `google_file_id NOT NULL UNIQUE` predates a second provider |
| Reviews unreachable for non-Google documents | Lookup validated `GoogleFileID` alone |
| Every `indexer_folders` query failed | `share_point_folder_id` declared, never migrated |
| `/api/v2/drafts` returned 500 | Same drift, surfaced through a join |

Each is the same mistake in a different place: a second provider was added by
duplicating a field beside the Google one rather than by introducing an
abstraction, so every consumer had to learn about both — and the ones that were
not updated broke.

## What leaked

**Domain models carry provider-specific identity.** `Document` has both
`GoogleFileID` and `FileID`; `DocumentFileRevision` has
`GoogleDriveFileRevisionID` and `FileRevisionID`; `IndexerFolder` has
`GoogleDriveID` and `SharePointFolderID`. `pkg/models` names a vendor in 69
places.

**The persistence layer branched on provider.**
`models.NewDocumentByFileID(fileID string, useSharePoint bool)` put provider
selection inside `pkg/models`, and the boolean was threaded through handler
helpers so each caller had to know the answer.

**The DI container exposed the provider's identity.** `Server.IsSharePoint()`
let any handler branch on the configured vendor, and `Server.GWService` /
`Server.SharePoint` are provider-specific fields whose own comments call them
"a temporary compatibility escape hatch".

**Schema constraints assumed one provider.** `google_file_id NOT NULL UNIQUE`
is only correct while every document has a Google file ID.

## What has been pulled back

`Document.Get` now matches either identifier column when only one is supplied.
A lookup by file ID does not need to know who stored the document, which is
what removes the boolean's reason to exist: it was there to decide which column
to put a string in.

With that in place:

- `models.DocumentByFileID(id)` replaces the boolean-taking constructor at
  every call site outside `pkg/document`.
- `Server.IsSharePoint()` and `Server.NewDocumentByFileID` are gone.
- `useSharePoint` is gone from the `internal/api` handlers and their helpers.
- The two remaining provider-shaped conditions in `documents.go` now test
  `srv.GWService != nil` — the capability the code actually needs — rather than
  negating a vendor name. A third provider takes the same path without anyone
  having to remember to extend a negated list.

`internal/api` and `internal/server` no longer branch on which workspace
provider is configured.

## What has not

- **`pkg/document.ToDatabaseModels` still takes `useSharePoint`.** This is a
  genuine persistence seam: it maps a document into storage and something must
  choose the column. The right answer is for the workspace provider to supply
  the identifier, not for a boolean to be passed in.
- **`Server.GWService` and `Server.SharePoint` remain.** Document locking
  (`hcd.IsLocked`) is the clearest case: it belongs on `workspace.Provider` as
  a capability, after which the last two conditions in `documents.go` disappear
  too.
- **The models still carry two identity fields each.** Collapsing them onto one
  column is the honest fix and the largest: `GoogleFileID` alone appears in over
  three hundred places, and holding NULL would make it a `*string`.
- **`google_file_id` is misnamed.** It is the provider's file identifier and has
  been since the second provider arrived. Renaming it is a mechanical migration
  plus those three hundred references.

## Suggested order

1. Add the locking capability to `workspace.Provider`; drop `GWService` from
   the handlers.
2. Have the workspace provider supply the document identifier, and remove the
   last `useSharePoint`.
3. Collapse the paired identity fields onto one column, renaming
   `google_file_id` to `file_id` in the same migration.

Steps 1 and 2 are contained. Step 3 is a day's mechanical work and should not
be started until the tests around it run — which, until this audit, they did
not: the whole of `pkg/models`' database coverage skips unless
`HERMES_TEST_POSTGRESQL_DSN` is set, and ten of those tests had been failing.

## References

- [ADR-009: Provider Abstraction Architecture](../adr/adr-009-provider-abstraction-architecture.md)
- [ADR-017: API Refactoring and Testing Strategy](../adr/adr-017-api-refactoring-and-testing-strategy.md)
- Code: `pkg/models/document.go`, `internal/server/server.go`, `internal/api/v2/`
