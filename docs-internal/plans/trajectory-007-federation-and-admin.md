---
id: trajectory-007
title: "Trajectory T7 — Multi-Provider Federation & Admin UI (post-1.0)"
status: Draft
created: 2026-04-27
date: 2026-04-27
author: Hermes Team
project_id: hermes
doc_uuid: ea1a5da7-98a4-419c-a83b-1a87cce099ba
type: Memo
subtype: Milestone
tags: [trajectory, roadmap, v1.x, federation, admin-ui, provider-abstraction]
related:
  - RFC-001
  - RFC-010
  - RFC-011
  - RFC-012
  - RFC-016
  - RFC-017
  - RFC-018
---

# Trajectory T7 — Multi-Provider Federation & Admin UI (post-1.0)

> Captures the explicitly v1.x bucket so it has a durable home and does not creep into v1.0 scope. None of these items block v1.0; all of them advance Hermes toward an edge-and-central federated deployment with an admin surface.

## Stable-release criteria served

None for v1.0 (by design). Sets up v1.x deliverables.

## Scope

In scope (v1.x):

- **Provider refactor (RFC-010):** clean up multi-backend doc model so federation doesn't fight the abstraction.
- **Federation (RFC-011):** edge → central pass-through and sync, UUID merging across providers, cross-provider identity joins.
- **Delegated bearer-token auth (RFC-012):** per-instance trust, scoped tokens.
- **Local↔central sync (RFC-001):** bidirectional sync of edge SQLite ↔ central PostgreSQL.
- **Document revisions / migration tracking (RFC-017):** for live Google → Git provider migrations with concurrent edits.
- **Admin UI (RFC-016):** four-phase plan (migration dashboard → indexer health → identity management → analytics).
- **Notification backend expansion (RFC-013 nice-to-haves):** Slack, Discord, Teams, encryption, Prometheus dashboards.
- **Document type expansion (RFC-006):** ADR/Memo/FRD/PATH templates.
- **Project config integration (RFC-019):** finish v2 API integration.

## Dependencies

- v1.0 must ship. Do not start any T7 phase before T1–T6 close, except design-only RFC iteration.

## Phases & exit criteria

T7 is staged but **not** scoped to a single release; each cluster will likely become its own trajectory once v1.0 ships. The phases below are checkpoints, not commitments.

### Phase A — Foundation (post-1.0)

- RFC-010 provider refactor merged.
- RFC-018 instance identity wired into auth + audit.

**Exit when:**

- Provider refactor lands without behavioural change in v1.0 endpoints (regression suite green).

### Phase B — Federation core

- RFC-011 pass-through reads + sync writes.
- RFC-012 bearer tokens.

**Exit when:**

- Two-instance demo: edge instance serves reads from central, writes propagate within 60 s.

### Phase C — Admin UI

- RFC-016 phases 1 + 2 (migration dashboard, indexer health).

**Exit when:**

- Operators can drive a migration and inspect indexer health without DB access.

### Phase D — Identity, revisions, doc types

- RFC-016 phases 3 + 4, RFC-017, RFC-006.

**Exit when:**

- Each underlying RFC reaches `Implemented`; this trajectory is then split or closed.

## Reviews / gates

Phase exit-only. Each cluster will likely graduate into its own dedicated trajectory once started.

## Risks

- **Scope creep into v1.0.** Mitigation: explicit policy — no T7 work merges to `main` until v1.0 ships.
- **Federation design drift** during the v1.0 freeze. Mitigation: keep RFCs in `Draft` and iterate them on paper; do not start implementation.

## References

- [RFC-001](../rfc/rfc-001-local-developer-mode.md), [RFC-010](../rfc/rfc-010-provider-interface-refactoring.md), [RFC-011](../rfc/rfc-011-multi-provider-architecture.md), [RFC-012](../rfc/rfc-012-authentication-bearer-tokens.md)
- [RFC-016: Hermes Admin Interface](../rfc/rfc-016-admin-interface.md)
- [RFC-017](../rfc/rfc-017-document-revisions-and-migration.md), [RFC-018](../rfc/rfc-018-instance-identity.md), [RFC-019](../rfc/rfc-019-project-config-integration.md), [RFC-006](../rfc/rfc-006-new-document-types.md)
- [Roadmap Implementation Tracker](roadmap-tracker.md)
