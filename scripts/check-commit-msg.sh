#!/bin/bash
# Commit message validation script
# Enforces conventional commit format and quality standards

set -e

COMMIT_MSG_FILE=$1
COMMIT_MSG=$(cat "$COMMIT_MSG_FILE")

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "=== Commit Message Validation ==="

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

# Get the first line (subject)
SUBJECT=$(echo "$COMMIT_MSG" | head -n1)

# Check for empty commit message
if [ -z "$SUBJECT" ]; then
    echo -e "${RED}❌ Commit message cannot be empty${NC}"
    exit 1
fi

# Check subject line length (max 72 characters recommended, 100 hard limit)
SUBJECT_LENGTH=${#SUBJECT}
if [ $SUBJECT_LENGTH -gt 100 ]; then
    echo -e "${RED}❌ Subject line too long: $SUBJECT_LENGTH characters (max 100)${NC}"
    echo -e "${YELLOW}Subject: $SUBJECT${NC}"
    exit 1
elif [ $SUBJECT_LENGTH -gt 72 ]; then
    echo -e "${YELLOW}⚠️  Subject line longer than recommended: $SUBJECT_LENGTH characters (recommended max 72)${NC}"
    echo -e "${YELLOW}Subject: $SUBJECT${NC}"
fi

# Check for conventional commit format: type(scope): description
# Valid types: feat, fix, docs, style, refactor, perf, test, build, ci, chore, revert
CONVENTIONAL_PATTERN='^(feat|fix|docs|style|refactor|perf|test|build|ci|chore|revert)(\([a-z0-9-]+\))?: .+'

if ! echo "$SUBJECT" | grep -qE "$CONVENTIONAL_PATTERN"; then
    echo -e "${YELLOW}⚠️  Subject line doesn't follow conventional commit format${NC}"
    echo -e "${YELLOW}Expected format: type(scope): description${NC}"
    echo -e "${YELLOW}Valid types: feat, fix, docs, style, refactor, perf, test, build, ci, chore, revert${NC}"
    echo -e "${YELLOW}Example: feat(api): add user authentication endpoint${NC}"
    echo ""
    echo -e "${YELLOW}Your commit: $SUBJECT${NC}"
    echo ""
    echo -e "${YELLOW}Note: This is a warning, not a failure. Consider following conventional commits.${NC}"
fi

# Check that subject doesn't end with period
if echo "$SUBJECT" | grep -qE '\.$'; then
    echo -e "${YELLOW}⚠️  Subject line should not end with a period${NC}"
fi

# Check for capitalization (should not start with lowercase after type)
if echo "$SUBJECT" | grep -qE '^[a-z]+(\([a-z0-9-]+\))?: [a-z]'; then
    echo -e "${YELLOW}⚠️  Description should start with a capital letter or lowercase verb${NC}"
    echo -e "${YELLOW}Your commit: $SUBJECT${NC}"
fi

# Check for common mistakes
if echo "$COMMIT_MSG" | grep -qi "WIP\|work in progress\|TODO\|FIXME\|XXX\|HACK"; then
    echo -e "${YELLOW}⚠️  Commit contains WIP/TODO/FIXME markers${NC}"
    echo -e "${YELLOW}Consider completing the work before committing${NC}"
fi

# Check for tool branding that should be removed
if echo "$COMMIT_MSG" | grep -qi "Generated with \|Co-Authored-By: Claude\|🤖"; then
    echo -e "${RED}❌ Commit message contains tool branding${NC}"
    echo -e "${RED}Please remove 'Generated with' or 'Co-Authored-By: Claude' markers${NC}"
    exit 1
fi

# Check for body (line 2 should be blank if there's a body)
LINE_COUNT=$(echo "$COMMIT_MSG" | wc -l | tr -d ' ')
if [ "$LINE_COUNT" -gt 1 ]; then
    SECOND_LINE=$(echo "$COMMIT_MSG" | sed -n '2p')
    if [ -n "$SECOND_LINE" ]; then
        echo -e "${YELLOW}⚠️  Second line should be blank (separate subject from body)${NC}"
    fi
fi

echo -e "${GREEN}✓ Commit message validation passed${NC}"
exit 0
