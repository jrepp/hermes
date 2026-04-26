---
id: adr-074
title: Playwright Dual Strategy for Local Iteration and CI
date: 2025-10-09
type: ADR
subtype: Development Tooling
decision_type: Testing Strategy
status: Accepted
tags: ['playwright', 'testing', 'e2e', 'tooling']
related: ['ADR-070']
created: 2026-04-24
deciders: Hermes Team
project_id: hermes
doc_uuid: 7f575895-2a6a-430d-81b5-34f9be1fe1aa
---
# Playwright Dual Strategy for Local Iteration and CI

> Hermes uses **two Playwright modes**: `playwright-mcp` for interactive, agent-driven exploration and debugging, and **headless Playwright** for CI and pre-commit validation. UI elements are addressed by **`data-test-*` selectors**. Automated/agent contexts must run with `--reporter=line --max-failures=1`; **`--headed` is forbidden** in any non-interactive context because it hangs the terminal.

## Context

E2E coverage needs to support two very different workflows: an engineer or agent reproducing a bug interactively (where seeing the page and the network is essential), and a CI pipeline asserting regressions don't ship (where speed, determinism, and parseable output matter). A single tool/configuration optimized for one use case fails the other — interactive debugging in CI hangs, and headless-only debugging is opaque.

## Decision

Standardize on Playwright for both modes and split usage along strategy lines.

**Interactive mode — `playwright-mcp`:**
- Used by humans and agents to navigate, snapshot, fill forms, and inspect network traffic on a running stack (typically the testing Docker Compose at `:4201`/`:8001`, ADR-070).
- Designed for LLM/agent interaction: accessibility snapshots over screenshots, named element refs, structured network logs.
- Output is exploratory; findings get promoted into headless tests.

**Headless mode — `tests/e2e-playwright/`:**
- Source of truth for regression assertions; runs in CI and locally before commits.
- Selectors are `data-test-*` attributes only — never CSS classes or DOM structure.
- Required invocation in automated/agent contexts: `npx playwright test --reporter=line --max-failures=1`.
- **Never pass `--headed`** in automated/agent contexts; it opens a browser window and blocks indefinitely.

## Consequences

### Positive
- Same tool, same selectors, same fixtures across both modes; learnings transfer directly.
- Agents have a sanctioned, non-blocking way to explore the UI (`playwright-mcp`) and a sanctioned way to validate fixes (headless with `--reporter=line --max-failures=1`).
- `data-test-*` selectors decouple tests from styling and refactors.

### Negative
- Contributors must learn both modes and when to use each.
- Interactive `playwright-mcp` sessions are not automatically promoted to headless tests; that step is manual.

## Alternatives Considered

- **Selenium / Cypress / Puppeteer / TestCafe:** Each loses something Playwright provides — cross-browser, auto-wait, network interception, agent-friendly tooling, or CI ergonomics.
- **Headless-only (no `playwright-mcp`):** Faster CI but no good interactive debug story; agents end up screenshotting blindly.
- **Manual testing only:** Doesn't scale and provides no regression net.

## References

- `tests/e2e-playwright/` — headless suite and config
- ADR-070 (testing Docker Compose stack the suite targets)
