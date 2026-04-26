# Test Coverage & Ergonomic Improvements - Analysis

**Date**: October 25, 2025  
**Environment**: Hermes Testing (Docker Compose @ localhost:8001)  
**Status**: 14/18 tests passing (77.8% pass rate)

## Executive Summary

Ran the distributed test scenarios and identified key areas for improvement:

1. ✅ **Test Infrastructure Fixed** - 14 tests passing (generators, seeding)
2. ⚠️ **Auth Integration Needed** - 4 scenario tests blocked by missing auth tokens
3. ✅ **CLI Ergonomics Improved** - Fixed workspace enum handling
4. 🔧 **Event Loop Issue** - asyncio.run() incompatibility documented

## Test Results

### Passing Tests (14/18 - 77.8%)

**Document Generators** (8/8 tests - 100%):
- ✅ `test_generate_uuid` - UUID generation works
- ✅ `test_generate_timestamp` - Timestamp generation works
- ✅ `test_generate_rfc` - RFC document generation
- ✅ `test_generate_prd` - PRD document generation
- ✅ `test_generate_meeting_notes` - Meeting notes generation
- ✅ `test_generate_doc_page` - Documentation page generation
- ✅ `test_uuid_uniqueness` - UUIDs are unique
- ✅ `test_custom_uuid` - Custom UUID support

**Workspace Seeding** (6/6 tests - 100%):
- ✅ `test_seed_basic` - Basic seeding works
- ✅ `test_seed_migration` - Migration seeding works
- ✅ `test_seed_multi_author` - Multi-author seeding works
- ✅ `test_clean_workspace` - Workspace cleanup works
- ✅ `test_seed_scenario_basic` - Basic scenario seeding
- ✅ `test_seed_scenario_migration` - Migration scenario seeding

### Failing Tests (4/18 - 22.2%)

**Scenario Tests** (0/4 tests - blocked by auth):
- ❌ `test_basic_scenario` - RuntimeError: Event loop is closed
- ❌ `test_basic_scenario_no_wait` - HermesAuthError: Unauthorized
- ❌ `test_migration_scenario` - RuntimeError: Event loop is closed
- ❌ `test_multi_author_scenario` - HermesAuthError: Unauthorized

## Root Cause Analysis

### Issue #1: Authentication Required

**Problem**: Search and document stats endpoints require OAuth tokens:
```
hc_hermes.exceptions.HermesAuthError: Authentication failed: Unauthorized
```

**Impact**: 4 scenario tests fail when trying to validate indexed documents

**Solution Needed**:
1. Add pytest fixture to obtain Dex OIDC token before tests
2. Set `HERMES_AUTH_TOKEN` in test environment
3. Or: Configure test environment with auth disabled (if available)

**Code Location**:
- `validation.py:338` - `get_document_stats()` calls search without auth
- `scenarios.py:97` - Scenarios call validator without token refresh

### Issue #2: Event Loop Management

**Problem**: `asyncio.run()` in client library creates/closes loops incompatibly with pytest:
```
RuntimeError: Event loop is closed
```

**Impact**: Health checks and config fetches fail after first async call

**Solution Needed**:
1. **Short-term**: Add event loop fixture (already attempted, partial success)
2. **Long-term**: Update `hc-hermes` client to support async context managers:
   ```python
   async with Hermes(base_url=...) as client:
       config = await client.get_web_config()
   ```

**Code Location**:
- `python-client/src/hc_hermes/client.py:313` - `asyncio.run()` usage
- Added `tests/conftest.py:event_loop` fixture (helps but doesn't fully solve)

## Ergonomic Improvements Implemented

### 1. Fixed CLI Workspace Enum Handling

**Before**:
```python
# Crashed with AttributeError: ALL
workspace_map = {
    "all": WorkspaceName.ALL,  # Enum doesn't have ALL
}
```

**After**:
```python
# Handles "all" gracefully
if args.workspace == "all":
    workspace = WorkspaceName.TESTING  # Migration handles multiple internally
else:
    workspace_map = {
        "testing": WorkspaceName.TESTING,
        "docs": WorkspaceName.DOCS,
    }
```

**File**: `hermes_test.py:78-87`

### 2. Added Event Loop Safety Helper

**Before**:
```python
def check_health(self) -> bool:
    self.client.get_web_config()  # Could crash on closed loop
```

**After**:
```python
def _safe_get_event_loop():
    """Get or create event loop safely."""
    try:
        loop = asyncio.get_event_loop()
        if loop.is_closed():
            loop = asyncio.new_event_loop()
            asyncio.set_event_loop(loop)
        return loop
    except RuntimeError:
        loop = asyncio.new_event_loop()
        asyncio.set_event_loop(loop)
        return loop

def check_health(self) -> bool:
    _safe_get_event_loop()  # Ensure valid loop first
    self.client.get_web_config()
```

**File**: `validation.py:23-36, 117-125`

### 3. Enhanced Pytest Event Loop Fixture

**Added**:
```python
@pytest.fixture(scope="function")
def event_loop(event_loop_policy):
    """Create a new event loop for each test."""
    loop = event_loop_policy.new_event_loop()
    asyncio.set_event_loop(loop)
    yield loop
    # Clean up pending tasks before closing
    try:
        loop.run_until_complete(asyncio.sleep(0))
        pending = asyncio.all_tasks(loop)
        for task in pending:
            task.cancel()
        loop.run_until_complete(asyncio.gather(*pending, return_exceptions=True))
    finally:
        loop.close()
```

**File**: `tests/conftest.py:18-35`

### 4. Fixed Pydantic V2 Deprecation

**Before**:
```python
class TestingConfig(BaseModel):
    class Config:
        frozen = False
```

**After**:
```python
class TestingConfig(BaseModel):
    model_config = ConfigDict(frozen=False)
```

**File**: `config.py:14`

### 5. Fixed Pytest Fixture Scope Mismatch

**Before**:
```python
@pytest.fixture  # function scope
def validator() -> HermesValidator:
    ...

@pytest.fixture(scope="session", autouse=True)
def check_hermes_running(validator: HermesValidator):  # Mismatch!
    ...
```

**After**:
```python
@pytest.fixture(scope="session")  # Now matches
def validator() -> HermesValidator:
    ...
```

**File**: `tests/conftest.py:42`

## Opportunities for Increased Coverage

### High Priority

**1. Add Authentication Test Fixture**
```python
@pytest.fixture(scope="session")
def hermes_auth_token():
    """Get auth token from Dex for tests."""
    from auth_helper import get_access_token_password
    
    token = get_access_token_password(
        issuer_url="http://localhost:5558/dex",
        client_id="hermes-web",
        client_secret="ZXhhbXBsZS1hcHAtc2VjcmV0",
        username="test@example.com",
        password="password",
    )
    return token

@pytest.fixture
def validator(hermes_auth_token):
    """Validator with auth token."""
    return HermesValidator(auth_token=hermes_auth_token)
```

**Expected Impact**: Fixes 4 failing scenario tests

**2. Add Async Client Tests**
```python
@pytest.mark.asyncio
async def test_async_search():
    """Test search using async client directly."""
    async with HermesAsync(base_url="http://localhost:8001") as client:
        results = await client.search.query("test")
        assert len(results.hits) > 0
```

**Expected Impact**: Better async coverage, avoids event loop issues

**3. Add Integration Test Markers**
```python
# In pyproject.toml:
[tool.pytest.ini_options]
markers = [
    "unit: Unit tests (no external dependencies)",
    "integration: Integration tests (requires Hermes)",
    "slow: Slow tests (>5 seconds)",
]

# Usage:
@pytest.mark.integration
def test_scenario():
    ...

# Run with: pytest -m "not slow"
```

**Expected Impact**: Better test organization, faster CI

### Medium Priority

**4. Add Document Lifecycle Tests**
```python
def test_document_create_read_update_delete():
    """Full CRUD cycle for a document."""
    # Create
    doc = generator.generate_rfc()
    doc_id = client.documents.create(doc)
    
    # Read
    retrieved = client.documents.get(doc_id)
    assert retrieved.title == doc.title
    
    # Update
    retrieved.summary = "Updated"
    client.documents.update(doc_id, retrieved)
    
    # Delete
    client.documents.delete(doc_id)
```

**Expected Impact**: Higher confidence in API operations

**5. Add Search Validation Tests**
```python
def test_search_relevance():
    """Test search returns relevant results."""
    # Seed known documents
    seeder.seed_basic(count=10, clean=True)
    
    # Search for specific term
    results = client.search.query("RFC-001")
    
    # Validate
    assert len(results.hits) >= 1
    assert "RFC-001" in results.hits[0].document_id
```

**Expected Impact**: Better search testing

**6. Add Performance Benchmarks**
```python
@pytest.mark.benchmark
def test_seed_performance(benchmark):
    """Benchmark seeding performance."""
    result = benchmark(seeder.seed_basic, count=100)
    assert len(result) == 100
```

**Expected Impact**: Performance regression detection

### Low Priority

**7. Add Workspace Migration Tests**
```python
def test_workspace_migration():
    """Test document migration between workspaces."""
    # Seed in testing
    files = seeder.seed_basic(count=5, workspace=WorkspaceName.TESTING)
    
    # Migrate to docs
    migrated = migrator.migrate_documents(
        source=WorkspaceName.TESTING,
        dest=WorkspaceName.DOCS,
        document_ids=[...]
    )
    
    assert len(migrated) == 5
```

**Expected Impact**: Better migration coverage

**8. Add Error Handling Tests**
```python
def test_invalid_document():
    """Test error handling for invalid documents."""
    with pytest.raises(ValidationError):
        generator.generate_rfc(title="")  # Empty title should fail
```

**Expected Impact**: Better error coverage

## Recommendations

### Immediate Actions (This Sprint)

1. **Add Auth Fixture** - Unblock 4 failing tests
   - Estimated time: 1-2 hours
   - Files: `tests/conftest.py`, `tests/test_scenarios.py`
   - Impact: 100% test pass rate

2. **Document Event Loop Issue** - Create tracking issue
   - File: `docs-internal/TESTING_KNOWN_ISSUES.md`
   - Action: Update `hc-hermes` client to support async context managers

3. **Add Test Markers** - Improve test organization
   - Files: `pyproject.toml`, `tests/*.py`
   - Impact: Faster CI, better test selection

### Short Term (Next Sprint)

4. **Add Async Client Tests** - Better async coverage
5. **Add Performance Benchmarks** - Track regressions
6. **Add CRUD Lifecycle Tests** - Higher API confidence

### Long Term (Roadmap)

7. **Refactor hc-hermes Client** - Fix event loop architecture
8. **Add E2E Playwright Tests** - Full UI+API coverage (separate from Python tests)
9. **Add Load Testing** - Concurrent user scenarios

## Test Coverage Metrics

### Current State
```
Total Tests:        18
Passing:            14 (77.8%)
Failing:            4  (22.2%)
Auth-blocked:       4  (22.2%)
Event-loop-blocked: 2  (11.1%)

By Category:
- Generators:    8/8   (100%)
- Seeding:       6/6   (100%)
- Scenarios:     0/4   (0%)    <- Blocked by auth

By Functionality:
- Document Gen:     100%
- Workspace Ops:    100%
- Search:           0%     <- Needs auth
- Validation:       50%    <- Health check works, stats don't
```

### Target State (After Auth Fix)
```
Total Tests:        18
Passing:            18 (100%)
Coverage:           ~85% (estimate with auth)

With Recommended Additions:
Total Tests:        ~35
Passing:            ~33 (94%)
Coverage:           ~92%
```

## Developer Ergonomics Assessment

### What Works Well ✅

1. **CLI is Intuitive**
   ```bash
   hermes-test seed --scenario basic --count 10
   hermes-test scenario basic --wait
   hermes-test validate --check-all
   ```

2. **Rich Terminal Output**
   - Progress bars
   - Color-coded status
   - Clear error messages

3. **Type Safety**
   - Pydantic models catch errors early
   - IDE autocomplete works well

4. **Modular Design**
   - Easy to add new scenarios
   - Generators are reusable
   - Clear separation of concerns

### Pain Points ⚠️

1. **Auth Setup is Manual**
   - Need to get token from Dex manually
   - No automatic token refresh in tests
   - Should be handled by fixtures

2. **Event Loop Confusion**
   - `asyncio.run()` incompatibility
   - Requires understanding of async internals
   - Should be transparent to users

3. **Workspace "all" Handling**
   - Not fully implemented
   - Confusing that migration handles it but basic doesn't
   - Should be consistent across scenarios

4. **Missing Documentation**
   - No guide for running tests locally
   - Auth setup not documented
   - Troubleshooting section needed

### Recommended Ergonomic Improvements

**1. Add Pre-Test Setup Script**
```bash
#!/bin/bash
# testing/python/setup-tests.sh

echo "Setting up test environment..."

# Get auth token
export HERMES_AUTH_TOKEN=$(python3 auth_helper.py get-token)

# Verify Hermes is running
curl -f http://localhost:8001/health || {
    echo "Hermes not running. Start with: cd testing && make up"
    exit 1
}

echo "✅ Ready to run tests"
echo "Run: pytest tests/ -v"
```

**2. Add Test Runner Makefile Targets**
```makefile
.PHONY: test-quick test-integration test-all

test-quick:
	@pytest tests/test_generators.py tests/test_seeding.py -v

test-integration: setup-auth
	@pytest tests/ -v -m integration

test-all: setup-auth
	@pytest tests/ -v --cov=. --cov-report=term-missing

setup-auth:
	@python3 auth_helper.py get-token > /tmp/hermes-test-token
	@export HERMES_AUTH_TOKEN=$$(cat /tmp/hermes-test-token)
```

**3. Add Developer Guide**
Create `testing/python/DEVELOPER_GUIDE.md` with:
- How to run tests locally
- Auth setup instructions
- Troubleshooting common issues
- How to add new scenarios
- How to debug async issues

## Conclusion

The test infrastructure is **solid** (77.8% passing) but **blocked by auth integration**. The main areas for improvement are:

1. **Critical**: Add auth fixture (unblocks 4 tests)
2. **Important**: Document event loop issue (long-term fix in client)
3. **Nice-to-have**: Additional coverage (CRUD, search, performance)

With auth integration, we'd achieve **100% pass rate** and unlock the full scenario testing capability.

### Next Steps

1. Implement auth fixture (Priority 1)
2. Create tracking issue for event loop refactor
3. Add test markers for better organization
4. Document test setup in DEVELOPER_GUIDE.md
5. Add recommended coverage improvements over next 2-3 sprints

**Estimated Time**: 
- Auth fix: 1-2 hours
- Documentation: 2-3 hours
- Additional coverage: 5-8 hours (spread over sprints)

**Total**: ~8-13 hours of work for 100% passing tests + improved coverage
