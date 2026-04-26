---
id: adr-NNN
title: "<Short imperative title — the rule, not the problem>"
status: Proposed
decision_type: Architectural Pattern
created: YYYY-MM-DD
deciders: Hermes Team
author: Hermes Team
project_id: hermes
doc_uuid: <run `uuidgen | tr '[:upper:]' '[:lower:]'`>
date: YYYY-MM-DD
type: ADR
tags: [tag-one, tag-two]
related:
  - ADR-NNN
  - RFC-NNN
---

# ADR-NNN: <Same title as frontmatter>

> One paragraph (2–4 lines) stating the rule the project is committing to. A reader who only reads this blockquote should be able to repeat the rule back. No background, no history, no "we will explore" — just the decision.

## Context

Why this decision is needed. Constraints, prior choices, the forces in tension. Keep it short — link out for narrative, benchmarks, and migration plans.

If a longer narrative exists, link it: *See [RFC-NNN](../rfc/rfc-NNN-<slug>.md) for type hierarchy, migration phases, and benchmarks.*

## Decision

State the rule clearly and numerically.

1. **<Rule one in bold>** — one or two sentences explaining the rule and how it manifests in code (`pkg/foo/bar.go`).
2. **<Rule two>** — etc.
3. **<Rule three>** — etc.

Keep this section tight. Each numbered item is a commitment the project makes.

### Canonical example (optional)

A small code snippet (≤15 lines) showing the decision in use. Skip if the rule is self-evident.

```go
// example
```

## Consequences

### Positive
- What gets easier or safer because of this decision.
- One bullet per consequence.

### Negative
- Real costs and trade-offs. Be honest — a missing Negative section is a smell.
- Migration burden, runtime cost, complexity, lock-in.

## Alternatives Considered

- **<Alternative A>** — rejected because <reason>.
- **<Alternative B>** — rejected because <reason>.
- **<Alternative C>** — rejected because <reason>.

Two to four alternatives is the sweet spot. If you have only one, you haven't considered enough; if you have eight, this is a memo.

## References

- [RFC-NNN: <title>](../rfc/rfc-NNN-<slug>.md) — narrative, benchmarks, migration phases.
- [ADR-NNN: <title>](adr-NNN-<slug>.md) — related rule.
- Code: `pkg/foo/`, `internal/api/v2/bar.go`.

<!--
AUTHORING REMINDERS — delete before committing.

DO:
- Keep the file under ~150 lines.
- Write the rule in present tense and active voice.
- Use code paths (`pkg/docid/uuid.go`) instead of pasting code.
- Update `adr-002-readme.md` (index + cross-cutting principles if relevant) in the same change.

DO NOT:
- Pin specific package versions in an ADR. Pinning lives in `package.json` / `go.mod`.
- Include implementation status checklists, "Future Work", or measured-results tables.
- Bundle two decisions in one ADR — split them.
- Include incident timelines, debugging logs, or session notes — those go in a memo.
- Reference legacy uppercase MD source files. Cite code paths or RFCs instead.
-->
