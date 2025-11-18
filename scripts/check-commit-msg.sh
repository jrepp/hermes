#!/bin/bash
# Commit message validation script
# Enforces conventional commit format, quality standards, and prevents AI branding
#
# Validates:
# 1. No AI tool branding (Claude, Copilot, ChatGPT, etc.)
# 2. Reasonable complexity (not overly verbose)
# 3. Conventional commit format
# 4. Subject line length limits
# 5. Proper structure and formatting

set -e

COMMIT_MSG_FILE=$1
COMMIT_MSG=$(cat "$COMMIT_MSG_FILE")

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
MAX_MESSAGE_LENGTH=2000
MAX_BODY_LINES=50
MAX_LINE_LENGTH=100
MAX_SUBJECT_LENGTH=100
RECOMMENDED_SUBJECT_LENGTH=72

# Error collection
ERRORS=()
WARNINGS=()

echo -e "${BLUE}=== Commit Message Validation ===${NC}"

# Skip merge commits
if echo "$COMMIT_MSG" | grep -qE "^Merge (branch|pull request)"; then
    echo -e "${GREEN}✓ Merge commit - skipping validation${NC}"
    exit 0
fi

# Skip revert commits
if echo "$COMMIT_MSG" | grep -qE "^Revert "; then
    echo -e "${GREEN}✓ Revert commit - skipping validation${NC}"
    exit 0
fi

# ============================================================================
# AI BRANDING DETECTION
# ============================================================================

echo "Checking for AI tool branding..."

# Comprehensive AI branding patterns
AI_PATTERNS=(
    "claude"
    "copilot"
    "co-pilot"
    "chatgpt"
    "gpt-[0-9]"
    "openai"
    "anthropic"
    "github[[:space:]]+copilot"
    "generated[[:space:]]+with[[:space:]]+claude"
    "generated[[:space:]]+with[[:space:]]+copilot"
    "powered[[:space:]]+by"
    "🤖"
    "co-authored-by:[[:space:]]*claude"
    "co-authored-by:[[:space:]]*copilot"
    "assistant"
    "ai[[:space:]]+generated"
)

for pattern in "${AI_PATTERNS[@]}"; do
    if echo "$COMMIT_MSG" | grep -qiE "$pattern"; then
        ERRORS+=("AI branding detected: '$pattern' found in commit message")
    fi
done

# ============================================================================
# MESSAGE COMPLEXITY CHECKS
# ============================================================================

echo "Checking message complexity..."

# Check total message length
MSG_LENGTH=${#COMMIT_MSG}
if [ $MSG_LENGTH -gt $MAX_MESSAGE_LENGTH ]; then
    ERRORS+=("Commit message too long: $MSG_LENGTH characters (max: $MAX_MESSAGE_LENGTH)")
fi

# Count non-empty lines
NON_EMPTY_LINES=$(echo "$COMMIT_MSG" | grep -c '[^[:space:]]' || echo 0)
if [ $NON_EMPTY_LINES -gt $((MAX_BODY_LINES + 1)) ]; then
    WARNINGS+=("Too many lines: $NON_EMPTY_LINES lines (max: $MAX_BODY_LINES body + 1 subject)")
fi

# Check individual line lengths (skip subject line)
LINE_NUM=0
while IFS= read -r line; do
    LINE_NUM=$((LINE_NUM + 1))
    if [ $LINE_NUM -gt 1 ] && [ -n "${line// }" ]; then
        LINE_LEN=${#line}
        if [ $LINE_LEN -gt $MAX_LINE_LENGTH ]; then
            WARNINGS+=("Line $LINE_NUM too long: $LINE_LEN characters (max: $MAX_LINE_LENGTH)")
        fi
    fi
done <<< "$COMMIT_MSG"

# ============================================================================
# SUBJECT LINE VALIDATION
# ============================================================================

echo "Validating subject line..."

# Get the first line (subject)
SUBJECT=$(echo "$COMMIT_MSG" | head -n1)

# Check for empty commit message
if [ -z "$SUBJECT" ]; then
    ERRORS+=("Commit message cannot be empty")
fi

# Check subject line length
SUBJECT_LENGTH=${#SUBJECT}
if [ $SUBJECT_LENGTH -gt $MAX_SUBJECT_LENGTH ]; then
    ERRORS+=("Subject line too long: $SUBJECT_LENGTH characters (max $MAX_SUBJECT_LENGTH)")
elif [ $SUBJECT_LENGTH -gt $RECOMMENDED_SUBJECT_LENGTH ]; then
    WARNINGS+=("Subject line longer than recommended: $SUBJECT_LENGTH characters (recommended max $RECOMMENDED_SUBJECT_LENGTH)")
fi

# ============================================================================
# CONVENTIONAL COMMIT FORMAT
# ============================================================================

echo "Checking conventional commit format..."

# Valid types: feat, fix, docs, style, refactor, perf, test, build, ci, chore, revert
CONVENTIONAL_PATTERN='^(feat|fix|docs|style|refactor|perf|test|build|ci|chore|revert)(\([a-z0-9-]+\))?: .+'

if ! echo "$SUBJECT" | grep -qE "$CONVENTIONAL_PATTERN"; then
    WARNINGS+=("Subject doesn't follow conventional commit format: type(scope): description")
fi

# Check that subject doesn't end with period
if echo "$SUBJECT" | grep -qE '\.$'; then
    WARNINGS+=("Subject line should not end with a period")
fi

# Check for capitalization
if echo "$SUBJECT" | grep -qE '^[a-z]+(\([a-z0-9-]+\))?: [a-z]'; then
    WARNINGS+=("Description should start with a capital letter or lowercase verb")
fi

# ============================================================================
# QUALITY CHECKS
# ============================================================================

echo "Running quality checks..."

# Check for WIP/TODO markers
if echo "$COMMIT_MSG" | grep -qiE "WIP|work in progress|TODO|FIXME|XXX|HACK"; then
    WARNINGS+=("Commit contains WIP/TODO/FIXME markers - consider completing work before committing")
fi

# Check for body structure (line 2 should be blank if there's a body)
LINE_COUNT=$(echo "$COMMIT_MSG" | wc -l | tr -d ' ')
if [ "$LINE_COUNT" -gt 1 ]; then
    SECOND_LINE=$(echo "$COMMIT_MSG" | sed -n '2p')
    if [ -n "$SECOND_LINE" ]; then
        WARNINGS+=("Second line should be blank (separate subject from body)")
    fi
fi

# ============================================================================
# RESULTS
# ============================================================================

# Print errors
if [ ${#ERRORS[@]} -gt 0 ]; then
    echo ""
    echo -e "${RED}❌ Commit message validation failed:${NC}"
    echo ""
    for error in "${ERRORS[@]}"; do
        echo -e "${RED}  • $error${NC}"
    done
    echo ""
    echo -e "${RED}Please fix the issues above and try again.${NC}"
    echo ""
    exit 1
fi

# Print warnings
if [ ${#WARNINGS[@]} -gt 0 ]; then
    echo ""
    echo -e "${YELLOW}⚠️  Commit message warnings:${NC}"
    echo ""
    for warning in "${WARNINGS[@]}"; do
        echo -e "${YELLOW}  • $warning${NC}"
    done
    echo ""
    echo -e "${YELLOW}Note: These are warnings, not failures. Consider addressing them.${NC}"
    echo ""
fi

echo -e "${GREEN}✓ Commit message validation passed${NC}"
exit 0
