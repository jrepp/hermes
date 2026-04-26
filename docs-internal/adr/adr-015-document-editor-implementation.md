---
id: adr-015
title: Provider-Aware Document Editor
date: 2025-10-08
type: ADR
subtype: Frontend Architecture
decision_type: Frontend Architecture
status: Accepted
tags: [document-editor, workspace, frontend]
related: [ADR-007, ADR-009]
created: 2025-10-08
deciders: Hermes Team
project_id: hermes
doc_uuid: ab8a1733-0037-4010-9c43-9b3772bff37b
---

# Provider-Aware Document Editor

> The document editor branches at runtime on the active workspace provider, published as **`workspace_provider` ("google" | "local")** on `GET /api/v2/web/config`. Google documents render in an iframe (read-only here, edited in Google Docs). Local documents use a text editor backed by **`GET/PUT /api/v2/documents/:id/content`**. PUT is allowed **only** for the local workspace; Google PUT returns **501 Not Implemented**.

## Context

ADR-007 established a local-filesystem workspace alongside Google Workspace, and ADR-009 abstracted both behind `workspace.Provider`. The frontend editor still has to render and edit documents, but the two providers have fundamentally different editing models (rich-text in Google Docs vs. plain Markdown on disk). A single one-size-fits-all editor would either block local editing or pretend Google editing happens in-app when it does not.

## Decision

- **Backend signaling:** add `workspace_provider` ("google" | "local") to `GET /api/v2/web/config` so the frontend can branch without sniffing.
- **Content endpoints (`internal/api/v2/document_content.go`):**
  - `GET /api/v2/documents/:id/content` — returns plain text for both providers (Google extracts text from the Docs API).
  - `PUT /api/v2/documents/:id/content` — local only; authorized to **owner and contributors**; rejects with **423 Locked** if the document is locked; reindexing happens asynchronously via the indexer; Google requests return **501 Not Implemented**.
- **Frontend (`web/app/components/document/`):** branches on `configSvc.config.workspace_provider`. Google → iframe + "Open in New Tab". Local → read-only `<pre>` with **Edit Document**, switching to a textarea with **Save Changes / Discard Changes**.
- **Status codes for PUT:** 200, 400, 403, 404, 423, 501.

## Consequences

### Positive
- One component, two coherent UXs; users get the editor that matches the underlying store.
- Local editing has explicit authorization, locking, and async reindex semantics — no silent overwrites.
- Returning 501 (not 403/404) for Google PUT is honest about the capability and surfaces the limitation cleanly.

### Negative
- Two UI branches to keep in sync as document features grow.
- Local edits trigger eventual-consistency reindexing; users can briefly see stale search results after a save.

## Alternatives Considered

- **Build a rich-text editor for both providers:** Major scope, and would still not match Google Docs' collaboration features; unjustifiable for the local case.
- **Read-only everywhere; require external editing:** Defeats the point of the local-workspace path (ADR-007) which exists to enable local edits.
- **Have the frontend probe capabilities per-document:** Extra round trips for information the backend already knows from configuration.

## References

- `internal/api/v2/document_content.go`, `web/web.go`
- `web/app/components/document/`, `web/app/services/config.ts`
- ADR-007 (local workspace), ADR-009 (provider abstraction)