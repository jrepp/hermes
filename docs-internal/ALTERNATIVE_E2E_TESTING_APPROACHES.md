# Alternative E2E Testing Approaches for Hermes

## Problem Summary

The Hermes web application currently has a **template compilation error** that prevents the UI from rendering:
- **Error**: `Attempted to call 'precompileTemplate' at runtime, but this API is meant to be used at compile time`
- **Scope**: Affects both native and containerized deployments
- **Impact**: Dashboard and all routes render blank pages
- **Likely Cause**: Recent changes to web components or template compilation configuration

## Alternative Test Approaches

### Option 1: API-Level Testing (✅ Recommended - No UI Required)

Test the full document lifecycle through REST API calls, bypassing the frontend entirely.

**Advantages:**
- ✅ Works immediately without fixing the frontend issue
- ✅ Tests the actual backend logic and data persistence
- ✅ Faster execution than browser-based tests
- ✅ More reliable and less flaky
- ✅ Can test local workspace document CRUD operations

**Implementation:**
```bash
# Run the API test
./tests/api/document-crud-test.sh
```

**What it tests:**
1. Authentication with Dex OIDC
2. Document creation via POST /api/v2/documents
3. Document update via PATCH /api/v2/documents/{id}
4. Document retrieval via GET /api/v2/documents/{id}
5. Document deletion via DELETE /api/v2/documents/{id}

**Script location:** `/Users/jrepp/hc/hermes/tests/api/document-crud-test.sh`

---

### Option 2: Fix the Template Compilation Issue (🔧 Root Cause Fix)

Investigate and resolve the `precompileTemplate` runtime error.

**Investigation Steps:**

1. **Check for incompatible template imports:**
   ```bash
   # Search for problematic template compilation patterns
   cd web
   grep -r "precompileTemplate\|compileTemplate" app/
   ```

2. **Verify ember-template-imports configuration:**
   - Check `web/.babelrc.js` - babel-plugin-ember-template-compilation should be auto-added
   - Check `web/ember-cli-build.js` - ensure proper .gts compilation setup
   - Check `web/package.json` - verify ember-template-imports version

3. **Test with a fresh build:**
   ```bash
   cd web
   rm -rf dist tmp node_modules/.cache
   yarn install
   yarn build
   ```

4. **Check for dynamic component usage:**
   - The error suggests something is trying to compile templates at runtime
   - Look for: `{{component}}` helper with dynamic names, lazy-loaded templates, or dynamic template strings

5. **Bisect recent commits:**
   ```bash
   # Recent commits that touched web/
   git log --oneline -10 -- web/
   
   # Test an earlier commit
   git checkout <earlier-commit-hash>
   cd testing && docker compose build web && docker compose up -d web
   ```

**Potential fixes:**
- Upgrade `ember-template-imports` to latest version
- Ensure all .gts files use proper template syntax
- Check if any component is incorrectly using template compilation APIs
- Verify Babel plugin configuration for template-tag components

---

### Option 3: Headless Browser Testing with Workarounds (⚠️ Partial Solution)

Use existing Playwright tests but work around the rendering issue.

**Approach A: Test earlier routes that might work:**
```typescript
// Try routes that don't use the problematic component
await page.goto('http://localhost:4201/authenticate');
await page.goto('http://localhost:4201/my');
```

**Approach B: Mock the problematic component:**
```javascript
// In Mirage or test setup, stub out the failing component
```

**Approach C: Use API + minimal UI validation:**
```bash
# 1. Create document via API
curl -X POST http://localhost:8001/api/v2/documents ...

# 2. Verify it appears in search/listing (if those pages work)
npx playwright test --grep "search"
```

---

### Option 4: Component-Level Testing (🧪 Unit/Integration Focus)

Test individual components in isolation using Ember's test framework.

**Advantages:**
- Tests don't depend on full app rendering
- Can test specific document-related components
- Faster feedback loop

**Example:**
```bash
cd web
yarn test:integration --filter="DocumentEditor"
yarn test:unit --filter="document"
```

---

### Option 5: Hybrid Approach (🎯 Best of Both Worlds)

Combine multiple strategies for comprehensive coverage:

1. **API tests** for backend logic and data persistence
2. **Component tests** for UI logic and interactions
3. **E2E tests** once the template issue is resolved

**Workflow:**
```bash
# 1. Run API tests (works now)
./tests/api/document-crud-test.sh

# 2. Run component tests (should work)
cd web && yarn test:integration

# 3. Fix template issue, then run full E2E
cd tests/e2e-playwright
npx playwright test
```

---

## Recommendation

**Immediate action:** Use **Option 1 (API-Level Testing)** to validate the document creation and update functionality right now.

**Next steps:**
1. Run the API test to verify backend functionality: `./tests/api/document-crud-test.sh`
2. Investigate the template compilation issue (Option 2) in parallel
3. Once fixed, integrate full E2E browser tests

**Long-term:** Maintain a mix of API tests (fast, reliable) and E2E tests (full user journey) for comprehensive coverage.

---

## Template Engine Question

### Should we use a different template engine?

**Short answer: No, stick with Ember's current template system.**

**Reasoning:**

1. **The issue is NOT with the template engine itself** - It's a build configuration or component implementation bug
2. **Ember's template system is mature and well-supported:**
   - Handlebars-based templates (.hbs)
   - Template-tag components (.gts) for modern TypeScript + template co-location
   - Excellent Glint integration for type safety

3. **Switching template engines would be extremely disruptive:**
   - Would require rewriting 500+ template files
   - Loss of HashiCorp Design System component compatibility
   - Break existing patterns and developer workflows
   - Months of migration effort

4. **The current stack is standard and recommended:**
   - `ember-template-imports`: Official Ember RFC for template-tag components
   - `.gts` files: Standard way to co-locate TypeScript and templates
   - Glint: Best-in-class template type checking

### What IS needed:

✅ **Fix the build configuration** - The `precompileTemplate` error suggests a misconfiguration, not a fundamental template engine problem

✅ **Update dependencies if needed** - Ensure ember-template-imports and related packages are compatible

✅ **Verify component patterns** - Check for any components using incorrect template compilation APIs

**The template engine itself is fine - we just need to fix the build issue.**

---

## Next Actions

1. ✅ **Run API test now:** `./tests/api/document-crud-test.sh`
2. 🔍 **Investigate template error:** Follow Option 2 investigation steps
3. 📝 **Document findings:** Update this file with root cause once identified
4. ✅ **Fix and verify:** Apply fix, rebuild, test with playwright-mcp
5. 🎯 **Maintain both:** Keep API tests + E2E tests for comprehensive coverage
