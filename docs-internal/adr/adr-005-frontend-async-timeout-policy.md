---
id: adr-005
title: Frontend Async Timeout & Fallback Policy
status: Accepted
decision_type: Architectural Pattern
created: 2025-10-08
deciders: Hermes Team
author: Hermes Team
project_id: hermes
doc_uuid: 82333dc0-a780-4bef-817f-207916917ecb
date: 2025-10-08
type: ADR
subtype: Frontend Decision
tags: [frontend, promise, timeout, resilience, graceful-degradation]
related: [ADR-003]
---

# ADR-005: Frontend Async Timeout & Fallback Policy

> Every async API call in the Hermes frontend is wrapped in `withTimeout()` (or `withTimeoutAndFallback()`) with a graceful fallback. No frontend code may `await` an unbounded promise. UI components must render with partial data on failure rather than show an infinite spinner.

## Context

After a successful admin login, the dashboard hung on a loading spinner indefinitely. Root cause: `store.maybeFetchPeople.perform()` awaited a promise tied to API requests that returned `401` and never resolved. The frontend had no timeout protection, so the failed task hung forever and blocked the entire dashboard render.

This pattern was found in many places (`Promise.all` in routes, `task.perform()` in services). Without a project-wide rule, every new async call risks reintroducing the bug.

## Decision

1. **No unbounded `await`s in the frontend.** Every API call, task, or `Promise.all` is wrapped in one of the helpers in `web/app/utils/promise-timeout.ts`:
   - `withTimeout(promise, ms, label)` — rejects on timeout.
   - `withTimeoutAndFallback(promise, ms, fallback, label)` — resolves to `fallback` on timeout or error.
   - `withTimeoutError(promise, ms)` — throws a typed `TimeoutError` (for debugging).

2. **Default budgets:**
   - Person/group API requests: **15 s** (typical <1 s; allow slow network).
   - Search operations: **30 s** (multiple backend calls).
   - `maybeFetchPeople`-style aggregations / `Promise.all` over batched fetches: **30 s**.

3. **Failure mode is graceful degradation, not crash.** On timeout or error:
   - Render with an empty list / placeholder records, never block the page.
   - Log via the standard service logger (`🔄`/`📡`/`📬`/`✅`/`⚠️`/`❌` markers) so failures are diagnosable in the console.
   - Critical paths (dashboard `Promise.all`, route models) catch and return empty arrays so the route can still render.

4. **Pattern (canonical):**

   ```typescript
   await withTimeout(
     this.store.maybeFetchPeople.perform(documents),
     30000,
     'Fetching people for documents'
   );
   ```

## Consequences

### Positive
- No infinite spinners. The UI always reaches a rendered state.
- Failures are visible (logs) but not fatal (page renders).
- Cascade failures are bounded — a slow downstream cannot lock the dashboard.

### Negative
- Per-endpoint timeout values must be tuned and maintained.
- Legitimately slow operations may time out; budgets need occasional adjustment.
- More boilerplate around every async call site.

## Alternatives Considered

- **Single global timeout for all promises** — rejected: one budget doesn't fit search vs. people-lookup vs. document fetches.
- **Retry instead of timeout** — rejected: still requires an outer timeout to bound total wait; doesn't address hangs.
- **Backend-only request timeouts** — rejected: doesn't help with network failures, dropped connections, or 401-without-body responses.

## Operational Notes

- Track timeout-error rates in production logs; a sudden rise indicates backend regression.
- Known additional sites that still need auditing: `web/app/routes/authenticated.ts`, `web/app/routes/authenticated/projects/project.ts`, `web/app/routes/authenticated/results.ts`, `web/app/components/inputs/people-select.ts`, `web/app/components/header/toolbar.ts`.

## References

- Code: `web/app/utils/promise-timeout.ts`, `web/app/services/_store.ts`, `web/app/services/latest-docs.ts`, `web/app/services/recently-viewed.ts`, `web/app/routes/authenticated/dashboard.ts`.