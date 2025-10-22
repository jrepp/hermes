# ADR: .gts Template Compilation Runtime Error Investigation

**Date**: October 21, 2025  
**Status**: RESOLVED ✅  
**Severity**: Critical - Blocks all UI rendering (WAS BLOCKING, NOW FIXED)

## Problem Statement

The Ember.js frontend fails to render with a runtime error when attempting to load any route:

```
Error: Attempted to call `precompileTemplate` at runtime, but this API is meant to be used at compile time. You should use `compileTemplate` instead.
    at precompileTemplate (vendor.js:17797:305)
    at Module.callback (hermes.js:303:84)
```

## Root Cause Analysis

### Architecture Overview

The `.gts` (TypeScript template-in-JS) compilation pipeline has three stages:

1. **ember-template-imports Preprocessor** (v3.4.2)
   - Processes `**/*.{js,gjs,ts,gts}` files
   - Extracts `<template>` tags from source
   - Generates `precompileTemplate()` calls
   - Renames `.gts` → `.ts` for TypeScript compiler

2. **ember-template-imports Babel Plugin**
   - Converts extracted templates to standardized format
   - Outputs: `setComponentTemplate(precompileTemplate(\`...\`, {...}), Component)`

3. **babel-plugin-ember-template-compilation** (v2.4.1)
   - **SHOULD** transform `precompileTemplate()` calls at BUILD TIME
   - **SHOULD** replace with compiled template objects
   - **CURRENTLY FAILING** - Not executing or not transforming correctly

### Evidence

#### Built Output Analysis
```javascript
// File: dist/assets/hermes.js:303
class ActionComponent extends _component2.default {}
(0, _component.setComponentTemplate)(
  (0, _templateCompilation.precompileTemplate)(`
    <button type="button" class="action" ...attributes>
      {{yield}}
    </button>
  `, {
    strictMode: true
  }), 
  ActionComponent
);
```

**Problem**: `precompileTemplate` is being called at RUNTIME instead of being replaced with compiled template code at BUILD TIME.

#### Affected Components
All `.gts` components are affected, including:
- `app/components/action.gts` (Button wrapper)
- `app/components/modals.gts` (Modal container)
- `app/components/empty-state-text.gts`
- `app/components/animated-container.gts`
- ~30 total `.gts` files in the codebase

#### Package Versions
- `ember-source`: 6.7.0
- `ember-template-imports`: 3.4.2
- `babel-plugin-ember-template-compilation`: 2.4.1 (via ember-cli-htmlbars)
- `ember-cli-htmlbars`: 6.3.0
- `ember-cli-babel`: 8.2.0
- `@babel/core`: 7.28.4

## Attempted Fixes (All Unsuccessful)

### Attempt 1: Manual Babel Plugin Configuration
**Approach**: Explicitly add `babel-plugin-ember-template-compilation` to babel.config.js

```javascript
// babel.config.js
module.exports = {
  plugins: [
    ['@babel/plugin-proposal-decorators', { legacy: true }],
    ['babel-plugin-ember-template-compilation', {
      precompilerPath: 'ember-source/dist/ember-template-compiler.js',
    }],
    // ... other plugins
  ],
};
```

**Result**: ❌ No change - `precompileTemplate` still in runtime code  
**Analysis**: Manual configuration likely conflicts with ember-template-imports' auto-configuration

### Attempt 2: Switch to .babelrc.js
**Approach**: Move `babel.config.js` → `.babelrc.js` (different config resolution)

**Result**: ❌ No change  
**Analysis**: Config file location not the issue

### Attempt 3: Remove Manual Plugin Config
**Approach**: Let ember-template-imports handle all babel plugin registration

```javascript
// .babelrc.js - removed babel-plugin-ember-template-compilation
// NOTE: babel-plugin-ember-template-compilation is automatically 
// added by ember-template-imports addon
```

**Result**: ❌ No change  
**Analysis**: Confirms the plugin IS being registered, but not executing correctly

### Attempt 4: Enable TypeScript Transform
**Approach**: Add ember-cli-babel configuration to ember-cli-build.js

```javascript
// ember-cli-build.js
module.exports = function (defaults) {
  let app = new EmberApp(defaults, {
    'ember-cli-babel': {
      enableTypeScriptTransform: true,
    },
    // ...
  });
};
```

**Result**: ❌ No change  
**Analysis**: TypeScript transform enabled but doesn't affect template compilation

### Attempt 5: Clean Build
**Approach**: Remove all build artifacts and rebuild from scratch

```bash
rm -rf dist tmp
yarn ember server --port 4201 --proxy http://127.0.0.1:8001
```

**Result**: ❌ No change  
**Analysis**: Not a caching issue

## Additional Ideas to Try

### High Priority (Most Likely to Succeed)

#### 1. Check ember-cli-htmlbars Configuration
**Hypothesis**: ember-cli-htmlbars might need explicit configuration to run babel-plugin-ember-template-compilation on .gts-derived files.

**Investigation Steps**:
```javascript
// ember-cli-build.js
module.exports = function (defaults) {
  let app = new EmberApp(defaults, {
    'ember-cli-htmlbars': {
      // Try enabling these options
      inline: false, // Force separate template files?
      debug: true,   // Enable debug output
    },
  });
};
```

**Research**:
- Check ember-cli-htmlbars@6.3.0 CHANGELOG for .gts-related changes
- Look for `emberTemplateBabel` or `templateCompiler` options
- Review ember-cli-htmlbars source: `lib/ember-addon-main.js`

#### 2. Verify Babel Plugin Load Order
**Hypothesis**: babel-plugin-ember-template-compilation might be loading BEFORE ember-template-imports' plugin, causing ordering issues.

**Investigation Steps**:
```javascript
// .babelrc.js - Explicitly control order
const { addPlugin } = require('ember-cli-babel-plugin-helpers');

module.exports = {
  plugins: [
    ['@babel/plugin-proposal-decorators', { legacy: true }],
    // Ensure ember-template-imports plugin runs first
    require.resolve('ember-template-imports/src/babel-plugin'),
    // Then compilation plugin
    [require.resolve('babel-plugin-ember-template-compilation'), {
      compilerPath: require.resolve('ember-source/dist/ember-template-compiler'),
    }],
    // ... other plugins
  ],
};
```

**Validation**:
- Add `console.log()` to both babel plugins to verify execution order
- Check if templates are being processed twice

#### 3. Check Ember Auto Import Webpack Configuration
**Hypothesis**: Webpack might be bypassing Ember's babel pipeline for some files.

**Investigation Steps**:
```javascript
// ember-cli-build.js
module.exports = function (defaults) {
  let app = new EmberApp(defaults, {
    autoImport: {
      webpack: {
        module: {
          rules: [
            {
              test: /\.gts$/,
              use: [
                {
                  loader: 'babel-loader',
                  options: {
                    plugins: [
                      require.resolve('babel-plugin-ember-template-compilation'),
                    ],
                  },
                },
              ],
            },
          ],
        },
      },
    },
  });
};
```

**Research**:
- Check if .gts files are being processed by webpack instead of Broccoli
- Verify ember-auto-import@2.11.1 doesn't have special .gts handling

#### 4. Update ember-template-imports
**Hypothesis**: Newer version might have fixes for this issue.

**Investigation Steps**:
```bash
# Check latest version
yarn info ember-template-imports versions

# Try upgrading
yarn upgrade ember-template-imports@latest

# Or try specific known-good versions
yarn add ember-template-imports@^4.0.0  # If available
yarn add ember-template-imports@3.5.0   # Next patch
```

**Research**:
- Check GitHub issues: https://github.com/ember-template-imports/ember-template-imports/issues
- Search for: "precompileTemplate runtime error"
- Review CHANGELOG for v3.4.2 → v3.5.x → v4.x

#### 5. Check Template Compiler Path
**Hypothesis**: The template compiler path might be incorrect or inaccessible.

**Investigation Steps**:
```bash
# Verify template compiler exists
ls -la node_modules/ember-source/dist/ember-template-compiler.js

# Check if it's being loaded
cd web
node -e "console.log(require.resolve('ember-source/dist/ember-template-compiler.js'))"
```

**Fix if needed**:
```javascript
// ember-cli-build.js or .babelrc.js
const path = require('path');
const compilerPath = path.join(
  __dirname,
  'node_modules/ember-source/dist/ember-template-compiler.js'
);

// Use absolute path in config
```

### Medium Priority (Configuration Tweaks)

#### 6. Try Glimmer-VM Strict Mode Compatibility
**Hypothesis**: strictMode: true in template config might be causing issues.

**Investigation Steps**:
```javascript
// Check if ember-template-imports has a config for strictMode
// ember-cli-build.js
module.exports = function (defaults) {
  let app = new EmberApp(defaults, {
    // Try disabling strict mode
    strictMode: false,
  });
};
```

#### 7. Check TypeScript Compiler Integration
**Hypothesis**: ember-cli-typescript might be processing .ts files before babel sees them.

**Investigation Steps**:
```bash
# Check processing order in build logs
EMBER_CLI_DEBUG=* yarn ember build 2>&1 | grep -E "(gts|babel|typescript)" > build-debug.log

# Look for .gts → .ts transformation timing
```

**Fix if needed**:
```javascript
// tsconfig.json - Ensure TypeScript doesn't process templates
{
  "compilerOptions": {
    // ... existing options
  },
  "exclude": [
    "**/*.gts",  // Let ember-template-imports handle these
    "**/*.gjs"
  ]
}
```

#### 8. Verify Broccoli Pipeline
**Hypothesis**: Broccoli might be filtering out .gts files from babel processing.

**Investigation Steps**:
```bash
# Check Broccoli debug output
DEBUG=broccoli* yarn ember build 2>&1 | tee broccoli-debug.log

# Look for .gts file processing
grep -i "gts" broccoli-debug.log
```

### Low Priority (Workarounds)

#### 9. Convert .gts to .ts + .hbs
**Approach**: Temporarily convert problematic .gts files to separate template files.

**Example**:
```typescript
// Before: app/components/action.gts
import Component from "@glimmer/component";

export default class ActionComponent extends Component<ActionComponentSignature> {
  <template>
    <button type="button" class="action" ...attributes>
      {{yield}}
    </button>
  </template>
}
```

```typescript
// After: app/components/action.ts
import Component from "@glimmer/component";

export default class ActionComponent extends Component<ActionComponentSignature> {}
```

```handlebars
{{! app/components/action.hbs }}
<button type="button" class="action" ...attributes>
  {{yield}}
</button>
```

**Pros**: Known working pattern  
**Cons**: Reverts modern .gts syntax, requires changes to ~30 files

#### 10. Use Runtime Template Compilation
**Approach**: If build-time fails, fall back to runtime compilation.

```javascript
// app/utils/compile-template-runtime.js
import { compileTemplate } from '@ember/template-compilation';

export function compileTemplateAtRuntime(templateString, options) {
  return compileTemplate(templateString, options);
}
```

**Pros**: Bypasses build issue  
**Cons**: Performance hit, not the intended pattern

#### 11. Downgrade to Ember 5.x
**Approach**: If .gts support is broken in Ember 6.7.0, try an older version.

```bash
yarn add ember-source@^5.12.0
yarn add ember-data@^4.12.0
```

**Pros**: Might have better .gts support  
**Cons**: Loses Ember 6 features, major downgrade

## Debugging Tools

### 1. Enable Babel Debug Output
```bash
BABEL_ENV=development DEBUG=babel* yarn ember build 2>&1 | tee babel-debug.log
```

### 2. Inspect Preprocessor Output
```javascript
// Create a test script: debug-gts.js
const fs = require('fs');
const { preprocessEmbeddedTemplates } = require('ember-template-imports/src/preprocess-embedded-templates');

const source = fs.readFileSync('app/components/action.gts', 'utf8');
const result = preprocessEmbeddedTemplates(source, {
  relativePath: 'app/components/action.gts',
  getTemplateLocalsRequirePath: () => require.resolve('ember-source/dist/ember-template-compiler'),
  templateTagConfig: {}
});

console.log('=== PREPROCESSED OUTPUT ===');
console.log(result.output);
```

```bash
node debug-gts.js
```

### 3. Check Built Module Format
```bash
# Extract the problematic module from built assets
grep -A 20 "ActionComponent" web/dist/assets/hermes.js | head -30

# Compare with expected output (should have compiled template, not precompileTemplate call)
```

### 4. Verify Plugin Registration
```javascript
// Add to ember-cli-build.js
module.exports = function (defaults) {
  console.log('=== Checking babel plugins ===');
  const app = new EmberApp(defaults, {
    babel: {
      plugins: [
        // Add a debug plugin
        function() {
          return {
            visitor: {
              Program(path, state) {
                console.log('Processing file:', state.filename);
                console.log('Plugins:', state.opts.plugins.map(p => p.key));
              }
            }
          };
        }
      ]
    }
  });
  return app.toTree();
};
```

## Recommended Next Steps

1. **FIRST**: Try updating ember-template-imports (Idea #4) - least invasive, highest success probability
2. **SECOND**: Check babel plugin load order (Idea #2) - common cause of template compilation issues
3. **THIRD**: Verify ember-cli-htmlbars config (Idea #1) - might need explicit .gts support
4. **FOURTH**: Check ember-auto-import webpack rules (Idea #3) - could be bypassing babel
5. **IF ALL FAIL**: Convert .gts to .ts + .hbs (Idea #9) - guaranteed to work but loses modern syntax

## Impact Assessment

### Blocked Functionality
- ✅ Backend API - Fully functional
- ✅ Authentication (Dex) - Working
- ✅ Data loading - All services load correctly
- ❌ UI Rendering - Completely blocked by .gts compilation error
- ❌ E2E Testing - Cannot proceed without working UI

### Affected Routes
- `/` (redirects to dashboard)
- `/dashboard` (primary landing page)
- `/documents/*` (likely all document routes)
- `/projects/*` (likely all project routes)
- All routes using `application.hbs` template (which includes header with Action.gts)

## Related Issues

- Check ember-template-imports GitHub: https://github.com/ember-template-imports/ember-template-imports/issues
- Check babel-plugin-ember-template-compilation: https://github.com/ember-template-imports/babel-plugin-ember-template-compilation/issues
- Search for: "precompileTemplate runtime" "gts compilation" "template-imports babel"

## References

- ember-template-imports README: `web/node_modules/ember-template-imports/README.md`
- babel-plugin-ember-template-compilation source: `web/node_modules/babel-plugin-ember-template-compilation/`
- ember-cli-htmlbars source: `web/node_modules/ember-cli-htmlbars/lib/ember-addon-main.js`
- @ember/template-compilation source: `web/node_modules/ember-source/dist/packages/@ember/template-compilation/index.js`

## Decision

**Status**: ✅ RESOLVED

**Solution Implemented**: Upgraded ember-template-imports from v3.4.2 to v4.3.0 (Idea #4 from the investigation)

**Result**: 
- ✅ `precompileTemplate` calls completely removed from runtime code (0 instances in built assets)
- ✅ Templates now properly compiled using `createTemplateFactory` (139 instances in built assets)
- ✅ Production build succeeds without errors
- ✅ TypeScript type checking passes
- ✅ All .gts components now compile correctly at build time

**Changes Made**:
```json
// web/package.json
- "ember-template-imports": "^3.4.2",
+ "ember-template-imports": "^4.3.0",
```

**Validation Steps Performed**:
1. `yarn add ember-template-imports@^4.3.0` - Upgrade successful
2. `yarn ember build --environment=production` - Build successful
3. `grep "precompileTemplate" dist/assets/hermes*.js` - No matches (issue fixed!)
4. `grep -c "createTemplateFactory" dist/assets/hermes*.js` - 139 matches (correct format)
5. `yarn test:types` - TypeScript compilation passes

**Root Cause**: The issue was a bug or limitation in ember-template-imports v3.4.2 where the babel-plugin-ember-template-compilation was not properly transforming template code during the build process. Version 4.3.0 includes fixes that properly integrate with the Ember 6.x build pipeline.

**Alternative**: If updates don't work within 2 hours, convert the 5 most commonly used .gts components to .ts + .hbs format to unblock E2E testing while continuing investigation. *(Not needed - first solution worked!)*

---

**Last Updated**: October 21, 2025 (10:30 PM)  
**Investigator**: GitHub Copilot (AI Agent)  
**Session Duration**: ~2 hours of debugging + 15 minutes to fix  
**Time to Resolution**: 15 minutes after applying recommended fix
