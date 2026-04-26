# Curl-Based API E2E Tests

Quick reference for the shell-based API testing scripts in this directory.

## Scripts

### curl-based-test.sh
- **Status:** ✅ Ready to use
- **Auth:** Manual setup required
- **Purpose:** Simple document CRUD workflow

**Usage:**
```bash
# 1. Get auth cookies (one-time)
cd ../e2e-playwright && npx ts-node ../../testing/get-auth-cookies.ts

# 2. Run test
cd ../api && ./curl-based-test.sh
```

### simple-api-test.sh
- **Status:** 🔧 Needs Playwright in testing dir
- **Auth:** Automated
- **Purpose:** Full automated CRUD test

### document-crud-test.sh
- **Status:** 🔧 Needs adjustment
- **Auth:** Automated
- **Purpose:** Comprehensive CRUD suite

## Quick Start

```bash
# From repo root
cd tests/e2e-playwright
npx ts-node ../../testing/get-auth-cookies.ts
cd ../api
./curl-based-test.sh
```

## Documentation

See `/docs-internal/ALTERNATIVE_E2E_TESTING_APPROACHES.md` for full details.
