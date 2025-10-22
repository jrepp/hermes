#!/bin/bash

# Package Audit Script for Hermes Web Application
# Generates a comprehensive audit log of all packages with current and latest versions

OUTPUT_FILE="PACKAGE_AUDIT_LOG.md"
TEMP_FILE="/tmp/package-check.tmp"

echo "# Hermes Web Application Package Audit Log" > "$OUTPUT_FILE"
echo "" >> "$OUTPUT_FILE"
echo "Generated: $(date)" >> "$OUTPUT_FILE"
echo "" >> "$OUTPUT_FILE"
echo "## Summary" >> "$OUTPUT_FILE"
echo "" >> "$OUTPUT_FILE"

# Extract packages from package.json
PACKAGES=$(cat package.json | jq -r '.devDependencies + .dependencies | keys[]' | sort)

# Count total packages
TOTAL=$(echo "$PACKAGES" | wc -l | xargs)
echo "Total packages: $TOTAL" >> "$OUTPUT_FILE"
echo "" >> "$OUTPUT_FILE"

echo "## Core Build System & Framework" >> "$OUTPUT_FILE"
echo "" >> "$OUTPUT_FILE"
echo "| Package | Current Version | Latest Version | Status |" >> "$OUTPUT_FILE"
echo "|---------|----------------|----------------|--------|" >> "$OUTPUT_FILE"

# Core packages to check first
CORE_PACKAGES=(
  "@babel/core"
  "@babel/preset-env"
  "ember-cli"
  "ember-source"
  "ember-data"
  "ember-cli-babel"
  "ember-cli-htmlbars"
  "ember-cli-typescript"
  "typescript"
  "webpack"
  "babel-loader"
)

for pkg in "${CORE_PACKAGES[@]}"; do
  current=$(jq -r --arg pkg "$pkg" '(.devDependencies + .dependencies)[$pkg] // empty' package.json | sed 's/[\^~]//g')
  if [ -n "$current" ]; then
    echo -n "Checking $pkg... "
    latest=$(npm view "$pkg" version 2>/dev/null || echo "N/A")
    
    if [ "$current" = "$latest" ]; then
      status="✅ Current"
    elif [ "$latest" = "N/A" ]; then
      status="⚠️ Check manually"
    else
      status="🔄 Update available"
    fi
    
    echo "| \`$pkg\` | $current | $latest | $status |" >> "$OUTPUT_FILE"
    echo "done ($current → $latest)"
  fi
done

echo "" >> "$OUTPUT_FILE"
echo "## Babel Plugins & Presets" >> "$OUTPUT_FILE"
echo "" >> "$OUTPUT_FILE"
echo "| Package | Current Version | Latest Version | Status |" >> "$OUTPUT_FILE"
echo "|---------|----------------|----------------|--------|" >> "$OUTPUT_FILE"

# Check all babel-related packages
echo "$PACKAGES" | grep "@babel" | while read pkg; do
  current=$(jq -r --arg pkg "$pkg" '(.devDependencies + .dependencies)[$pkg] // empty' package.json | sed 's/[\^~]//g')
  if [ -n "$current" ] && [[ ! " ${CORE_PACKAGES[@]} " =~ " $pkg " ]]; then
    echo -n "Checking $pkg... "
    latest=$(npm view "$pkg" version 2>/dev/null || echo "N/A")
    
    if [ "$current" = "$latest" ]; then
      status="✅ Current"
    elif [ "$latest" = "N/A" ]; then
      status="⚠️ Check manually"
    else
      status="🔄 Update available"
    fi
    
    echo "| \`$pkg\` | $current | $latest | $status |" >> "$OUTPUT_FILE"
    echo "done ($current → $latest)"
  fi
done

echo "" >> "$OUTPUT_FILE"
echo "## Ember Core & CLI Packages" >> "$OUTPUT_FILE"
echo "" >> "$OUTPUT_FILE"
echo "| Package | Current Version | Latest Version | Status |" >> "$OUTPUT_FILE"
echo "|---------|----------------|----------------|--------|" >> "$OUTPUT_FILE"

# Check ember packages
echo "$PACKAGES" | grep "^ember-" | while read pkg; do
  # Skip if already in core packages
  if [[ ! " ${CORE_PACKAGES[@]} " =~ " $pkg " ]]; then
    current=$(jq -r --arg pkg "$pkg" '(.devDependencies + .dependencies)[$pkg] // empty' package.json | sed 's/[\^~]//g')
    if [ -n "$current" ]; then
      echo -n "Checking $pkg... "
      
      # Handle special case for git URLs
      if [[ "$current" == https* ]]; then
        latest="Git repository"
        status="ℹ️ Custom fork"
      else
        latest=$(npm view "$pkg" version 2>/dev/null || echo "N/A")
        
        if [ "$current" = "$latest" ]; then
          status="✅ Current"
        elif [ "$latest" = "N/A" ]; then
          status="⚠️ Check manually"
        else
          status="🔄 Update available"
        fi
      fi
      
      echo "| \`$pkg\` | $current | $latest | $status |" >> "$OUTPUT_FILE"
      echo "done"
    fi
  fi
done

echo "" >> "$OUTPUT_FILE"
echo "## @ember Scoped Packages" >> "$OUTPUT_FILE"
echo "" >> "$OUTPUT_FILE"
echo "| Package | Current Version | Latest Version | Status |" >> "$OUTPUT_FILE"
echo "|---------|----------------|----------------|--------|" >> "$OUTPUT_FILE"

# Check @ember packages
echo "$PACKAGES" | grep "^@ember/" | while read pkg; do
  current=$(jq -r --arg pkg "$pkg" '(.devDependencies + .dependencies)[$pkg] // empty' package.json | sed 's/[\^~]//g')
  if [ -n "$current" ]; then
    echo -n "Checking $pkg... "
    latest=$(npm view "$pkg" version 2>/dev/null || echo "N/A")
    
    if [ "$current" = "$latest" ]; then
      status="✅ Current"
    elif [ "$latest" = "N/A" ]; then
      status="⚠️ Check manually"
    else
      status="🔄 Update available"
    fi
    
    echo "| \`$pkg\` | $current | $latest | $status |" >> "$OUTPUT_FILE"
    echo "done ($current → $latest)"
  fi
done

echo "" >> "$OUTPUT_FILE"
echo "## Glimmer & Glint (TypeScript) Packages" >> "$OUTPUT_FILE"
echo "" >> "$OUTPUT_FILE"
echo "| Package | Current Version | Latest Version | Status |" >> "$OUTPUT_FILE"
echo "|---------|----------------|----------------|--------|" >> "$OUTPUT_FILE"

# Check glimmer/glint packages
echo "$PACKAGES" | grep -E "^@glimmer/|^@glint/" | while read pkg; do
  current=$(jq -r --arg pkg "$pkg" '(.devDependencies + .dependencies)[$pkg] // empty' package.json | sed 's/[\^~]//g')
  if [ -n "$current" ] && [ "$current" != "latest" ]; then
    echo -n "Checking $pkg... "
    latest=$(npm view "$pkg" version 2>/dev/null || echo "N/A")
    
    if [ "$current" = "$latest" ]; then
      status="✅ Current"
    elif [ "$latest" = "N/A" ]; then
      status="⚠️ Check manually"
    else
      status="🔄 Update available"
    fi
    
    echo "| \`$pkg\` | $current | $latest | $status |" >> "$OUTPUT_FILE"
    echo "done ($current → $latest)"
  elif [ "$current" = "latest" ]; then
    echo "| \`$pkg\` | latest (pinned) | - | ℹ️ Pinned to latest |" >> "$OUTPUT_FILE"
  fi
done

echo "" >> "$OUTPUT_FILE"
echo "## TypeScript & Type Definitions" >> "$OUTPUT_FILE"
echo "" >> "$OUTPUT_FILE"
echo "| Package | Current Version | Latest Version | Status |" >> "$OUTPUT_FILE"
echo "|---------|----------------|----------------|--------|" >> "$OUTPUT_FILE"

# Check typescript and @types packages
echo "$PACKAGES" | grep -E "^typescript$|^@types/|^@typescript-eslint/" | while read pkg; do
  current=$(jq -r --arg pkg "$pkg" '(.devDependencies + .dependencies)[$pkg] // empty' package.json | sed 's/[\^~]//g')
  if [ -n "$current" ] && [ "$current" != "latest" ]; then
    echo -n "Checking $pkg... "
    latest=$(npm view "$pkg" version 2>/dev/null || echo "N/A")
    
    if [ "$current" = "$latest" ]; then
      status="✅ Current"
    elif [ "$latest" = "N/A" ]; then
      status="⚠️ Check manually"
    else
      status="🔄 Update available"
    fi
    
    echo "| \`$pkg\` | $current | $latest | $status |" >> "$OUTPUT_FILE"
    echo "done"
  elif [ "$current" = "latest" ]; then
    echo "| \`$pkg\` | latest (pinned) | - | ℹ️ Pinned to latest |" >> "$OUTPUT_FILE"
  fi
done

echo "" >> "$OUTPUT_FILE"
echo "## HashiCorp Design System" >> "$OUTPUT_FILE"
echo "" >> "$OUTPUT_FILE"
echo "| Package | Current Version | Latest Version | Status |" >> "$OUTPUT_FILE"
echo "|---------|----------------|----------------|--------|" >> "$OUTPUT_FILE"

# Check HashiCorp packages
echo "$PACKAGES" | grep "^@hashicorp/" | while read pkg; do
  current=$(jq -r --arg pkg "$pkg" '(.devDependencies + .dependencies)[$pkg] // empty' package.json | sed 's/[\^~]//g')
  if [ -n "$current" ]; then
    echo -n "Checking $pkg... "
    latest=$(npm view "$pkg" version 2>/dev/null || echo "N/A")
    
    if [ "$current" = "$latest" ]; then
      status="✅ Current"
    elif [ "$latest" = "N/A" ]; then
      status="⚠️ Check manually"
    else
      status="🔄 Update available"
    fi
    
    echo "| \`$pkg\` | $current | $latest | $status |" >> "$OUTPUT_FILE"
    echo "done ($current → $latest)"
  fi
done

echo "" >> "$OUTPUT_FILE"
echo "## ESLint & Code Quality" >> "$OUTPUT_FILE"
echo "" >> "$OUTPUT_FILE"
echo "| Package | Current Version | Latest Version | Status |" >> "$OUTPUT_FILE"
echo "|---------|----------------|----------------|--------|" >> "$OUTPUT_FILE"

# Check eslint packages
echo "$PACKAGES" | grep -E "^eslint|prettier" | while read pkg; do
  current=$(jq -r --arg pkg "$pkg" '(.devDependencies + .dependencies)[$pkg] // empty' package.json | sed 's/[\^~]//g')
  if [ -n "$current" ]; then
    echo -n "Checking $pkg... "
    latest=$(npm view "$pkg" version 2>/dev/null || echo "N/A")
    
    if [ "$current" = "$latest" ]; then
      status="✅ Current"
    elif [ "$latest" = "N/A" ]; then
      status="⚠️ Check manually"
    else
      status="🔄 Update available"
    fi
    
    echo "| \`$pkg\` | $current | $latest | $status |" >> "$OUTPUT_FILE"
    echo "done ($current → $latest)"
  fi
done

echo "" >> "$OUTPUT_FILE"
echo "## Testing & QUnit" >> "$OUTPUT_FILE"
echo "" >> "$OUTPUT_FILE"
echo "| Package | Current Version | Latest Version | Status |" >> "$OUTPUT_FILE"
echo "|---------|----------------|----------------|--------|" >> "$OUTPUT_FILE"

# Check testing packages
echo "$PACKAGES" | grep -E "^qunit|^sinon|^mockdate" | while read pkg; do
  current=$(jq -r --arg pkg "$pkg" '(.devDependencies + .dependencies)[$pkg] // empty' package.json | sed 's/[\^~]//g')
  if [ -n "$current" ]; then
    echo -n "Checking $pkg... "
    latest=$(npm view "$pkg" version 2>/dev/null || echo "N/A")
    
    if [ "$current" = "$latest" ]; then
      status="✅ Current"
    elif [ "$latest" = "N/A" ]; then
      status="⚠️ Check manually"
    else
      status="🔄 Update available"
    fi
    
    echo "| \`$pkg\` | $current | $latest | $status |" >> "$OUTPUT_FILE"
    echo "done ($current → $latest)"
  fi
done

echo "" >> "$OUTPUT_FILE"
echo "## CSS & Styling" >> "$OUTPUT_FILE"
echo "" >> "$OUTPUT_FILE"
echo "| Package | Current Version | Latest Version | Status |" >> "$OUTPUT_FILE"
echo "|---------|----------------|----------------|--------|" >> "$OUTPUT_FILE"

# Check CSS packages
echo "$PACKAGES" | grep -E "tailwind|postcss|autoprefixer|sass" | while read pkg; do
  current=$(jq -r --arg pkg "$pkg" '(.devDependencies + .dependencies)[$pkg] // empty' package.json | sed 's/[\^~]//g')
  if [ -n "$current" ]; then
    echo -n "Checking $pkg... "
    latest=$(npm view "$pkg" version 2>/dev/null || echo "N/A")
    
    if [ "$current" = "$latest" ]; then
      status="✅ Current"
    elif [ "$latest" = "N/A" ]; then
      status="⚠️ Check manually"
    else
      status="🔄 Update available"
    fi
    
    echo "| \`$pkg\` | $current | $latest | $status |" >> "$OUTPUT_FILE"
    echo "done ($current → $latest)"
  fi
done

echo "" >> "$OUTPUT_FILE"
echo "## Other Dependencies" >> "$OUTPUT_FILE"
echo "" >> "$OUTPUT_FILE"
echo "| Package | Current Version | Latest Version | Status |" >> "$OUTPUT_FILE"
echo "|---------|----------------|----------------|--------|" >> "$OUTPUT_FILE"

# Check remaining packages
CHECKED_PATTERNS="@babel|^ember-|^@ember/|^@glimmer/|^@glint/|^typescript$|^@types/|^@typescript-eslint/|^@hashicorp/|^eslint|prettier|^qunit|^sinon|^mockdate|tailwind|postcss|autoprefixer|sass"

echo "$PACKAGES" | grep -Ev "$CHECKED_PATTERNS" | while read pkg; do
  current=$(jq -r --arg pkg "$pkg" '(.devDependencies + .dependencies)[$pkg] // empty' package.json | sed 's/[\^~]//g')
  if [ -n "$current" ]; then
    echo -n "Checking $pkg... "
    
    # Handle special cases
    if [[ "$current" == https* ]]; then
      latest="Git repository"
      status="ℹ️ Custom fork"
    else
      latest=$(npm view "$pkg" version 2>/dev/null || echo "N/A")
      
      if [ "$current" = "$latest" ]; then
        status="✅ Current"
      elif [ "$latest" = "N/A" ]; then
        status="⚠️ Check manually"
      else
        status="🔄 Update available"
      fi
    fi
    
    echo "| \`$pkg\` | $current | $latest | $status |" >> "$OUTPUT_FILE"
    echo "done"
  fi
done

echo "" >> "$OUTPUT_FILE"
echo "## Notes" >> "$OUTPUT_FILE"
echo "" >> "$OUTPUT_FILE"
echo "- ✅ Current: Package is on the latest stable version" >> "$OUTPUT_FILE"
echo "- 🔄 Update available: A newer stable version is available" >> "$OUTPUT_FILE"
echo "- ⚠️ Check manually: Could not automatically verify (might be deprecated or renamed)" >> "$OUTPUT_FILE"
echo "- ℹ️ Custom fork: Using a custom git repository fork" >> "$OUTPUT_FILE"
echo "- ℹ️ Pinned to latest: Package.json specifies 'latest' tag" >> "$OUTPUT_FILE"
echo "" >> "$OUTPUT_FILE"
echo "## Recommendations" >> "$OUTPUT_FILE"
echo "" >> "$OUTPUT_FILE"
echo "1. Review packages marked with 🔄 for potential updates" >> "$OUTPUT_FILE"
echo "2. Test thoroughly after updating core framework packages (Ember, Babel)" >> "$OUTPUT_FILE"
echo "3. Update dependencies in groups by category to isolate issues" >> "$OUTPUT_FILE"
echo "4. Check CHANGELOG/release notes for breaking changes before updating" >> "$OUTPUT_FILE"

echo ""
echo "✅ Audit complete! Results written to $OUTPUT_FILE"
