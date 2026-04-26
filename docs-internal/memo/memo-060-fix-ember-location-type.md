---
id: memo-060
title: 'Fix: Ember locationType auto -> history'
type: Memo
subtype: Configuration Note
status: Accepted
tags: [configuration, ember, location-type, routing]
related: [ADR-001]
supersedes: ADR 036
created: 2026-04-24
author: Hermes Team
deciders: Hermes Team
project_id: hermes
doc_uuid: f1cf2017-f977-4461-8b3a-2630c42f118e
---

# MEMO-060: Fix - Ember locationType auto -> history

> Demoted from ADR 036. This is a one-line configuration fix with no ongoing
> architectural constraint beyond "do not switch back to `auto`" -- which is
> implicit in Ember 6.x removing the option. Kept as a memo for historical
> reference. The Ember build-system policy lives in ADR-001.

## Context

Development server failed to load with error:

``` text
Uncaught Error: Assertion Failed: Could not resolve a location class at 'location:auto'

```

**Root Cause**: `web/config/environment.js` used `locationType: "auto"`, which was deprecated and removed in Ember 6.x.

## Decision

Change `locationType` from `"auto"` to `"history"` in configuration.

```javascript
// Before
locationType: "auto",

// After
locationType: "history",
```

## Rationale

**Location Type Options**:
- **`history`**: Uses browser History API for clean URLs (`/documents`)
  - Recommended for modern apps
  - Requires server routing configuration
  - Better UX with clean URLs

- **`hash`**: Uses URL hash fragments (`/#/documents`)
  - Works without server configuration
  - Older approach but still valid

- **`auto`**: Deprecated in Ember 6.x
  - Automatically chose between history/hash
  - Must be replaced with explicit choice

**Why `history`**:
1. Hermes is a modern web application
2. Better UX with clean URLs
3. Development server handles routing correctly
4. Production deployment (nginx) supports history mode
5. Industry standard for SPAs

## Consequences

### Positive
- ✅ Application loads successfully
- ✅ Clean URLs without hash fragments
- ✅ Modern browser History API
- ✅ Ember 6.x compatible
- ✅ No location resolution errors

### Negative
- ❌ Requires server configuration for direct navigation
- ❌ Nginx/Apache must route all paths to index.html
- ❌ Slightly more complex deployment vs hash mode

## Alternatives Considered

1. **Use `hash` location type**
   - ✅ Simpler server configuration
   - ❌ Ugly URLs with hash fragments
   - ❌ Not modern best practice

2. **Stay with `auto`**
   - ❌ Not supported in Ember 6.x
   - ❌ Application won't load

3. **Custom location implementation**
   - ❌ Unnecessary complexity
   - ❌ Reinventing the wheel

## Implementation

**File Changed**: `web/config/environment.js`

**Server Requirements**:
- Development: ember-cli dev server already configured
- Production: Nginx must route all paths to `/index.html`

**Nginx Configuration** (for reference):

```nginx
location / {
  try_files $uri $uri/ /index.html;
}

```

## Verification

✅ Dev server rebuilt successfully (1091ms)
✅ Application loads at http://localhost:4200/
✅ No location resolution errors
✅ Navigation between routes works
✅ URL changes reflect in address bar
✅ Browser back/forward buttons work

## Future Considerations

- Ensure production nginx configuration supports history mode
- Document deployment requirements for operations team
- Add smoke tests for routing in CI/CD

## References

- Source: `FIX_LOCATION_TYPE_2025_10_07.md`
- Ember Router Docs: https://guides.emberjs.com/release/routing/
- Related: `EMBER_UPGRADE_STRATEGY.md`