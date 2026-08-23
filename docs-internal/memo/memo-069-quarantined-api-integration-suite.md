---
id: memo-069
title: "The api Integration Suite Was Never Compiled by CI"
status: Investigation
created: 2026-08-23
author: Hermes Team
project_id: hermes
doc_uuid: 39c7e981-fcaa-4b08-86fb-9b8f184295e5
type: Memo
subtype: Investigation
tags: [ci, testing, technical-debt, providers, adr-009]
related:
  - memo-068
  - ADR-009
---

# The api Integration Suite Was Never Compiled by CI

## What happened

The `Integration Tests (api)` job ran `cd tests/api && go test ./...` with no
build tag. Most of that suite is behind `//go:build integration`, so the job
compiled only the untagged files and reported green on a fraction of what it
appeared to cover. The sibling `e2e` job was worse: every file there carries
the tag, so the pattern matched no packages at all and the job had never run a
single test while reporting success.

Building the api suite with `-tags=integration` for the first time produces
**40 failing tests against 128 passing**. None of the failures come from the
toolchain or dependency work that surfaced them; they are pre-existing and
were simply never visible.

The suite is quarantined — it still builds without the tag in CI — and
`go vet -tags=integration ./...` was added so the excluded code cannot rot
while it waits. The quarantine is recorded in the workflow next to the suite.

## The failures, by cause

### 1. Nil provider panics an HTTP handler (product bug, not a test bug)

```
github.com/hashicorp-forge/hermes/pkg/workspace/adapters/google.(*Service).ListPermissions(0x0, ...)
	pkg/workspace/adapters/google/drive_helpers.go:451
github.com/hashicorp-forge/hermes/internal/api/v2.DraftsDocumentHandler.func1
	internal/api/v2/drafts.go:1204
```

The receiver is `0x0`. The handler is shaped as:

```go
if srv.SharePoint != nil {
    permissions, err := srv.SharePoint.ListPermissions(docID)
    ...
} else {
    permissions, err := srv.GWService.ListPermissions(docID)   // nil when Google is unconfigured
```

The `else` treats "SharePoint is absent" as "Google is present". When neither
is configured the call dereferences a nil `*Service` and the process takes a
SIGSEGV — an availability bug reachable from an authenticated request, not
merely a test failure.

This is the vendor-branching pattern [[memo-068]] catalogued. Six direct
`GWService` calls remain, each in the `else` of a `srv.SharePoint != nil`
test, and each has the same shape:

| File | Call |
| --- | --- |
| `internal/api/v2/drafts.go` | `ListPermissions`, `RenameFile`, `ShareFile` |
| `internal/api/v2/documents.go` | `RenameFile`, `ShareFile`, `SearchPeople` |

Per ADR-009 the fix is a capability interface selected once, not a vendor name
tested at each call site. Until then every one of these panics an unconfigured
deployment.

### 2. Endpoints answering 500 where 200 is expected

`tests/api/v2_drafts_test.go:154`, `api_complete_integration_test.go:562` and
others. Not yet diagnosed; some are likely downstream of the same nil provider.

### 3. A delete that leaves its row behind

`tests/api/documents_uuid_test.go:237-242` — the delete reports success and the
document is still readable afterwards.

### 4. Assertions predating the current content type

`tests/api/api_v1_test.go:170` expects `application/json` and receives
`application/json; charset=utf-8`. A stale assertion rather than a defect.

## Order to work in

1. The nil-provider panic — it is a live crash, and fixing it via the
   capability interface finishes the [[memo-068]] port at the same time.
2. Re-run the suite; the 500s may largely resolve with it.
3. The delete bug, which looks like a genuine data-layer defect.
4. The stale content-type assertions.
5. Remove `tags=` from the api entry in `parallel-ci.yml` so the suite gates
   again.

The `e2e` and `migration` suites are a separate problem: both want the full
`testing/docker-compose` stack rather than testcontainers, so they cannot run
in CI as written. Restoring them means giving them the self-contained setup
the tenancy and schema suites already have.
