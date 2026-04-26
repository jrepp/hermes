---
id: adr-018
title: Document Identification System (DocID)
status: Accepted
decision_type: Data Model
created: 2025-10-26
deciders: Hermes Team
author: Hermes Team
project_id: hermes
doc_uuid: 3e7af144-9069-41dc-adca-ba16e6172960
date: 2025-10-26
type: ADR
tags: [document-id, uuid, multi-provider, data-model]
related: [ADR-009, ADR-017, RFC-021, MEMO-039]
---

# ADR-018: Document Identification System (DocID)

> Document identity is a stable `UUID` (provider-agnostic). Storage location is a separate `ProviderID` (`google:…`, `local:…`, `remote-hermes:…`). The full reference is `CompositeID{UUID, ProviderID, Project}`. APIs accept both legacy provider-specific IDs and the new `uuid/<id>` short format during migration.

## Context

Hermes originally identified documents by `GoogleFileID`, hard-coding the Google Workspace backend into the data model and the API surface. With multi-provider support (local filesystem, remote Hermes instances) and the workspace abstraction (ADR-007, ADR-009), we needed an identifier that:

- survives provider migrations,
- can represent the same logical document in multiple storage locations,
- supports content-drift detection across copies,
- and can be exposed through APIs without breaking existing clients.

See [RFC-021](../rfc/rfc-021-document-identification-system.md) for type hierarchy, format examples, migration phases, performance benchmarks, and worked use cases.

## Decision

1. **`UUID` is the stable, provider-agnostic identity** of a document. Implemented in `pkg/docid` as a type-safe wrapper around `github.com/google/uuid`, with `sql.Scanner` / `driver.Valuer` and JSON marshalers. Once assigned, a UUID never changes.

2. **`ProviderID = ProviderType + id`** describes where a document is stored. Provider types are `google`, `local`, and `remote-hermes`. String form: `"<type>:<id>"`. The same `UUID` may have multiple `ProviderID`s simultaneously (canonical + copies).

3. **`CompositeID = {UUID, ProviderID, Project}`** is the full reference used at API and workspace boundaries. Three completeness levels: UUID-only (lookup), UUID+Provider (fetch from specific backend), Complete (with project context).

4. **API short format is `uuid/<uuid>`** (e.g. `/api/v2/documents/uuid/550e8400-...`). The full colon-separated form and a URI form (`uuid/<uuid>?provider=...&id=...&project=...`) are also supported. APIs **accept both legacy provider-specific IDs (e.g. raw `GoogleFileID`) and the new `uuid/<id>` format** during the migration window.

5. **Database migration is non-breaking and additive.** New nullable columns (`document_uuid`, `provider_type`, `provider_id`, `project_id`) are added; `google_file_id` is retained for backward compatibility. UUIDs are backfilled by a background job that is idempotent, resumable, and rate-limited.

6. **`pkg/docid` is the single source of types.** Handlers, models, workspace adapters, and the search layer all use `docid.UUID`, `docid.ProviderID`, `docid.CompositeID` rather than raw strings.

### Canonical example

```go
uuid := docid.NewUUID()
googleID, _ := docid.GoogleFileID("1a2b3c4d")
cid := docid.NewCompositeID(uuid, googleID, "rfc-archive")

cid.ShortString() // "uuid/550e8400-..."
cid.String()      // "uuid:550e8400...:provider:google:id:1a2b3c4d:project:rfc-archive"
```

## Consequences

### Positive
- Documents survive provider migrations (e.g. Google → local archive) without losing identity.
- The same logical document can exist in multiple locations with content-drift detection via SHA256 comparison on the shared UUID.
- API clients can adopt the UUID format incrementally without breaking changes.
- Type-safe IDs prevent accidental string-mixing bugs at compile time.
- Workspace adapters and the search layer share a single identifier vocabulary.

### Negative
- Two ID formats coexist during the (multi-month) migration window; routing and lookup code must handle both.
- Existing documents must be backfilled with UUIDs (background job, ~100 docs/hour given Google API rate limits).
- Adds a small allocation/parsing cost on the request path (<5µs per `CompositeID.Parse`).

## Alternatives Considered

- **Keep `GoogleFileID` as the canonical ID** — rejected; locks the data model to Google Workspace and prevents migrations to local/remote providers.
- **Use the database row ID as the canonical identifier** — rejected; not portable across instances or backups, and not stable across re-imports.
- **Use only a UUID, no `ProviderID`** — rejected; we lose the ability to address a specific storage location when a document has multiple copies, and provider routing becomes ambiguous.
- **Encode provider info inside the UUID (e.g. namespace UUIDs)** — rejected; opaque to consumers, harder to debug, and couples identity to storage again.

## References

- [RFC-021: Document Identification System (DocID)](../rfc/rfc-021-document-identification-system.md) — type hierarchy, formats, migration phases, benchmarks.
- [ADR-009: Provider Abstraction Architecture](adr-009-provider-abstraction-architecture.md) — the broader provider model.
- [ADR-017: V2 API Provider Abstraction Pattern](adr-017-api-refactoring-and-testing-strategy.md) — V2 API uses `docid` types at the boundary.
- Code: `pkg/docid/`, `pkg/models/document.go`, `internal/api/v2/documents.go`, `pkg/workspace/types.go`.