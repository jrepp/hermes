---
id: trajectory-007-adversarial-review
title: "Adversarial Review — T7 Federation & Admin UI"
status: Draft
created: 2026-04-27
date: 2026-04-27
author: Hermes Team
project_id: hermes
doc_uuid: c355ac73-4521-4153-b674-9d2f57df07e0
type: Memo
subtype: Analysis
tags: [trajectory, adversarial-review, v1.x, federation, admin-ui]
related:
  - ADR-009
  - ADR-012
  - ADR-018
  - RFC-011
  - RFC-012
  - RFC-016
  - RFC-018
---

# Adversarial Review — T7 Federation & Admin UI

> This review tries to break [Trajectory T7](trajectory-007-federation-and-admin.md) before execution. The most important control is scope discipline: T7 should not enter implementation before v1.0, and when it does, it should be split into smaller trajectories.

## Run-readiness verdict

Do not run T7 as one implementation plan. Keep it as a parking lot until v1.0 ships, then split it into separate trajectories for provider refactor, federation core, delegated auth, admin UI, revisions, and document types.

## Highest-risk failure modes

1. **The plan bundles too many architectural changes.** Federation, delegated auth, instance identity, admin UI, bidirectional sync, revisions, notifications, document types, and project config each carry independent risk. Running them together will make sequencing and rollback impossible.

2. **"No T7 work merges to main" is too absolute for design and too weak for implementation boundaries.** The plan allows design-only RFC iteration but forbids implementation. It should also define how to handle prerequisite refactors discovered during v1.0 without smuggling federation work into release scope.

3. **Federation can violate provider abstraction if it shortcuts through remote APIs.** [ADR-009](../adr/adr-009-provider-abstraction-architecture.md) requires integrations through providers. Edge-to-central pass-through should still respect provider interfaces and server DI boundaries.

4. **Delegated bearer tokens can conflict with backend-centric OIDC/session rules.** [ADR-012](../adr/adr-012-multi-provider-auth-architecture.md) and related auth ADRs distinguish backend-centric OIDC sessions from provider-specific headers. RFC-012 must clearly separate service-to-service tokens from browser auth.

5. **Identity merging is underspecified.** [ADR-018](../adr/adr-018-document-identification-system.md) gives document UUID identity, but federation needs conflict rules for instance identity, provider IDs, project IDs, user identity, and simultaneous edits.

6. **Admin UI can accidentally become the control plane before APIs are safe.** Operators should be able to perform migrations and inspect health through APIs/runbooks before UI wraps those operations. Otherwise UI bugs can become operational blockers.

7. **Two-instance demo is not a sufficient federation proof.** It can pass without covering partitions, clock skew, duplicate delivery, auth revocation, split-brain writes, or schema/version mismatch.

## Missing decisions before execution

- Decide the first post-v1.0 slice and create a dedicated trajectory for it.
- Define service-to-service auth separately from browser/session auth.
- Define instance identity, trust bootstrap, token rotation, and revocation.
- Define conflict resolution and merge policy for documents, users, projects, and provider IDs.
- Define API-first admin operations before UI implementation.
- Define version compatibility rules between edge and central instances.

## Concrete pre-flight checklist

- After v1.0, split T7 into at least four plans: provider refactor, federation auth/identity, federation sync, admin UI.
- Require each RFC to state which ADR principles it relies on and whether it proposes to supersede any.
- Build a federation test matrix that includes network partition, duplicate events, out-of-order events, revoked token, stale schema, and simultaneous edits.
- Require admin UI work to consume stable v2 APIs only, with no direct database access.
- Add a release-channel/version compatibility note before edge-central sync is enabled.

## Suggested plan edits

- Change Phase A-D into "candidate future trajectories" rather than phases.
- Add an explicit rule that T7 implementation branches cannot be required for v1.0 release blockers.
- Add a post-v1.0 discovery phase that chooses one narrow first slice based on operational value.
- Add a reference to ADR-018 for document identity and require any merge behaviour to preserve stable UUID semantics.

## Go/no-go gate

Begin T7 only after v1.0 ships and one narrow slice has a dedicated RFC, owner, test matrix, rollback story, and ADR compliance review.
