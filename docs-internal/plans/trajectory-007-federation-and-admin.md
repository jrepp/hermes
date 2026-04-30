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
  - ADR-009
  - ADR-012
  - ADR-018
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

> Read the [T7 adversarial review](trajectory-007-adversarial-review.md) before doing any T7 execution planning. The review's controlling conclusion is binding for this plan: do not run T7 as one implementation plan.

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

- v1.0 must ship. Do not start implementation before T1–T6 close, except design-only RFC iteration.
- After v1.0, choose one narrow first slice based on operational value and create a dedicated trajectory for that slice.
- Design-only RFC iteration during the v1.0 freeze may identify prerequisite refactors, but those refactors must not smuggle federation, delegated auth, admin UI, revisions, or bidirectional sync into v1.0 release scope.
- Any prerequisite refactor needed before v1.0 must be independently justified by a v1.0 trajectory and must preserve the current non-federated behavior.

## Execution Policy

T7 is a parking lot, not an implementation trajectory. It may collect related post-v1.0 ideas, but it must not be used as the execution vehicle for federation or admin UI work.

- No T7 implementation branch may be required to resolve a v1.0 release blocker.
- No T7 implementation work merges to `main` before v1.0 ships.
- After v1.0, split T7 into dedicated trajectories before implementation starts. Expected first splits are provider refactor, federation auth/identity, federation sync, and admin UI.
- Each split trajectory needs a dedicated RFC or RFC update, owner, test matrix, rollback story, and ADR compliance review.
- Each RFC must state which ADR principles it relies on and whether it proposes to supersede any existing ADR.

## Candidate future trajectories

These clusters are candidates for future trajectories. They are not ordered phases, release commitments, or authorization to start implementation.

### Candidate A — Provider and instance foundation (post-1.0)

- RFC-010 provider refactor merged.
- RFC-018 instance identity wired into auth + audit.

**Ready only when:**

- Provider refactor lands without behavioural change in v1.0 endpoints (regression suite green).
- Federation design continues to respect the provider boundary from [ADR-009](../adr/adr-009-provider-abstraction-architecture.md): external integrations go through `auth.Provider`, `workspace.Provider`, and `search.Provider` with `server.Server` as the V2 DI container.
- Instance identity defines trust bootstrap, token rotation, revocation, audit attribution, and version compatibility between edge and central instances.

### Candidate B — Federation auth and core sync

- RFC-011 pass-through reads + sync writes.
- RFC-012 bearer tokens.

**Ready only when:**

- Service-to-service delegated bearer tokens are explicitly separate from browser/session auth. They must not weaken the backend-centric OIDC/session rules in [ADR-012](../adr/adr-012-multi-provider-auth-architecture.md).
- Document merge behavior preserves stable UUID semantics from [ADR-018](../adr/adr-018-document-identification-system.md), including UUID, provider ID, and project identity behavior across provider migrations.
- Conflict policy covers documents, users, projects, provider IDs, simultaneous edits, and identity joins.
- The federation test matrix covers at least network partition, duplicate delivery, out-of-order delivery, revoked token, stale schema/version, clock skew, split-brain writes, and simultaneous edits.
- A two-instance demo may be an acceptance check, but it is not sufficient by itself.

### Candidate C — API-first admin operations and UI

- RFC-016 phases 1 + 2 (migration dashboard, indexer health).

**Ready only when:**

- Operators can drive migrations and inspect indexer health through stable V2 APIs and runbooks before UI implementation starts.
- Admin UI consumes stable V2 APIs only, with no direct database access.
- Admin operations have safe authorization, audit logs, rollback behavior, and failure-mode guidance before they become UI controls.

### Candidate D — Identity management, revisions, and document types

- RFC-016 phases 3 + 4, RFC-017, RFC-006.

**Ready only when:**

- The identity-management UI cannot merge or split identities in ways that violate [ADR-018](../adr/adr-018-document-identification-system.md).
- Revision tracking has a migration and conflict story for live Google to Git/local provider movement with concurrent edits.
- Document type expansion has template/versioning rules and does not depend on federation being enabled.

## Reviews / gates

- T7 starts only after v1.0 ships and one narrow slice has a dedicated trajectory.
- The first post-v1.0 slice must have a dedicated RFC or RFC update, owner, test matrix, rollback story, and ADR compliance review.
- Federation work must include a release-channel/version compatibility note before edge-central sync is enabled.
- Admin UI work must prove the underlying API/runbook operation is safe before wrapping it in UI.
- Close or split this parking-lot plan once its candidate clusters have dedicated trajectories.

## Risks

- **Scope creep into v1.0.** Mitigation: no T7 implementation work merges to `main` until v1.0 ships, and no T7 branch may be required to resolve a v1.0 blocker.
- **Overloaded execution plan.** Mitigation: treat T7 as a parking lot and split provider refactor, federation auth/identity, federation sync, admin UI, revisions, and document types into dedicated trajectories before implementation.
- **Federation bypasses provider boundaries.** Mitigation: enforce [ADR-009](../adr/adr-009-provider-abstraction-architecture.md) provider interfaces and V2 server DI boundaries in every RFC and review.
- **Delegated auth weakens browser auth rules.** Mitigation: keep service-to-service tokens separate from [ADR-012](../adr/adr-012-multi-provider-auth-architecture.md) browser/session auth.
- **Identity conflicts corrupt document history.** Mitigation: preserve [ADR-018](../adr/adr-018-document-identification-system.md) stable UUID semantics and require explicit merge/conflict policy before sync work.
- **Admin UI becomes the control plane too early.** Mitigation: require stable APIs and runbooks before UI implementation.
- **Demo-driven false confidence.** Mitigation: two-instance demos supplement, but do not replace, a federation failure-mode test matrix.

## References

- [ADR-009: Provider Abstraction Architecture](../adr/adr-009-provider-abstraction-architecture.md), [ADR-012: Multi-Provider Auth Architecture](../adr/adr-012-multi-provider-auth-architecture.md), [ADR-018: Document Identification System](../adr/adr-018-document-identification-system.md)
- [RFC-001](../rfc/rfc-001-local-developer-mode-with-central-hermes.md), [RFC-010](../rfc/rfc-010-provider-interface-refactoring.md), [RFC-011](../rfc/rfc-011-api-provider-remote-delegation.md), [RFC-012](../rfc/rfc-012-authentication-bearer-tokens.md)
- [RFC-016: Hermes Admin Interface](../rfc/rfc-016-admin-interface.md)
- [RFC-017](../rfc/rfc-017-document-revisions-and-migration.md), [RFC-018](../rfc/rfc-018-instance-identity.md), [RFC-019](../rfc/rfc-019-projectconfig-integration.md), [RFC-006](../rfc/rfc-006-new-document-types.md)
- [T7 adversarial review](trajectory-007-adversarial-review.md)
- [Roadmap Implementation Tracker](roadmap-tracker.md)
