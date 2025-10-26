# Human Enablement Patterns: What Makes AI Agents Effective

**Date**: October 25, 2025  
**Collaborator**: Jacob Repp (jrepp)  
**Context**: Reflection on infrastructure and practices that enabled/hindered AI effectiveness  
**Repository**: hashicorp/hermes

## Executive Summary

This memo analyzes the **infrastructure, documentation, and interaction patterns** that made AI collaboration productive in this codebase versus patterns that created friction. The goal is to identify what worked so it can be replicated, and what didn't so it can be improved.

**Key Finding**: Well-structured context (docs, conventions, Makefiles) enabled autonomous work, while missing auth setup and scattered configuration created blockers.

---

## What Enabled High Effectiveness ✅

### 1. Comprehensive Copilot Instructions

**File**: `.github/copilot-instructions.md` (828 lines)

**What Made It Effective**:
- **Privacy controls section** - Explicitly listed what NOT to commit (credentials, internal domains)
- **Project structure** - Clear map of where things are
- **Critical workflows** - Step-by-step build/test sequences
- **Known issues** - Documented existing problems (linting, yarn version)
- **Port conventions** - Native vs. testing environment ports
- **Code patterns** - Go/TypeScript conventions with examples

**Impact**:
- Could navigate 5,000+ file codebase without getting lost
- Avoided committing sensitive data (knew about `.gitignore` patterns)
- Understood "native vs. testing" distinction immediately
- Followed existing conventions instead of inventing new ones

**Example**:
```markdown
**Port Conventions**:
- Native: Frontend 4200, Backend 8000, Postgres 5432
- Testing (in `./testing`): Frontend 4201, Backend 8001, Postgres 5433
```
→ Knew to use `localhost:8001` for testing environment without asking

**Best Practice Identified**: 
> **Front-load context in a single authoritative document.** Don't make AI search 10 READMEs - put project-critical info in one place.

---

### 2. Extensive Internal Documentation

**Location**: `docs-internal/` (40+ markdown files)

**What Made It Effective**:
- **Architecture diagrams** - Visual representation of auth flows, indexer design
- **Implementation guides** - Step-by-step for complex features
- **Migration summaries** - History of major refactors with rationale
- **Design rationale** - Why decisions were made (HCL vs. YAML, etc.)
- **Testing guides** - OAuth setup, E2E testing approaches

**Impact**:
- Understood **why** code was structured a certain way
- Avoided suggesting "improvements" that were already tried and rejected
- Found answers in docs instead of asking repetitive questions
- Could trace feature evolution (UUID migration, indexer refactor)

**Example**:
File `docs-internal/AUTH_ARCHITECTURE_DIAGRAMS.md` had:
- Sequence diagrams for OAuth flows
- Dex integration details
- Token refresh patterns

→ Understood auth was complex, didn't suggest naive "just add a password" solutions

**Best Practice Identified**:
> **Document the "why" not just the "what".** Knowing Algolia was replaced with backend proxy prevents suggesting "call Algolia directly from frontend."

---

### 3. Structured Makefile with Descriptive Targets

**File**: `testing/Makefile` (210+ lines)

**What Made It Effective**:
- **Comments above targets** - Explained what each does
- **Logical grouping** - Docker, testing, Python, scenarios
- **Common workflows** - `make up`, `make test-python`, `make canary`
- **Variable documentation** - What env vars affect behavior
- **Dependency chains** - Targets that call other targets

**Impact**:
- Could run correct commands without trial-and-error
- Understood testing workflow: `make up` → `make canary` → tests
- Added new targets following existing patterns
- Deprecated old targets with warnings instead of breaking changes

**Example**:
```makefile
.PHONY: canary
canary: ## Quick validation that testing environment is working
	@echo "🐦 Running canary test..."
	@curl -f http://localhost:8001/health || ...
```
→ Knew `make canary` was the "is everything working?" check

**Best Practice Identified**:
> **Makefiles are executable documentation.** Comments + targets = self-documenting workflows.

---

### 4. Consistent Directory Structure

**Pattern**:
```
testing/
├── python/           # All Python testing code
│   ├── tests/        # Pytest tests
│   ├── pyproject.toml
│   └── *.py          # Modules
├── scripts/          # Bash scripts (being deprecated)
├── workspaces/       # Test data
└── Makefile          # Entry point
```

**What Made It Effective**:
- **Flat layout** - No deep nesting, easy to find files
- **Clear boundaries** - Python in one place, bash in another
- **Convention over configuration** - Predictable locations

**Impact**:
- Could guess where files were without searching
- Knew to put new Python code in `testing/python/`
- Understood deprecation strategy (bash → Python migration)

**Best Practice Identified**:
> **Predictable structure beats clever organization.** Flat > nested, obvious > abstract.

---

### 5. Type-Safe Configuration with Pydantic

**Pattern**: Used throughout `testing/python/`

**What Made It Effective**:
- **Self-documenting** - Field types and defaults visible
- **Validation** - Errors at config load, not runtime
- **IDE support** - Autocomplete worked (when LSP configured)
- **Enums** - `ScenarioType`, `WorkspaceName` prevented typos

**Impact**:
- Could see what config options existed by reading model
- Understood relationships (workspace → path, scenario → generator)
- Caught bugs early (wrong enum value = immediate error)

**Example**:
```python
class TestingConfig(BaseModel):
    hermes_base_url: str = Field(
        default_factory=lambda: os.getenv("HERMES_BASE_URL", "http://localhost:8001")
    )
```
→ Knew default URL and how to override it

**Best Practice Identified**:
> **Types are documentation.** Pydantic models show what's required, what's optional, what the defaults are.

---

### 6. Existing Test Infrastructure

**What Made It Effective**:
- **pytest fixtures** - Reusable test setup
- **Test markers** - Could see integration vs. unit (even if not fully used)
- **Conftest.py** - Centralized fixture definitions
- **Clear test structure** - `test_*.py` files with descriptive names

**Impact**:
- Knew how to add new tests (follow existing pattern)
- Could run subsets (`pytest tests/test_generators.py`)
- Understood test dependencies (validator needs Hermes running)

**Best Practice Identified**:
> **Existing tests are templates.** New tests should look like old tests.

---

### 7. Linting and Formatting Tools

**Tools**: `ruff` configured in `pyproject.toml`

**What Made It Effective**:
- **Auto-fix** - `ruff check . --fix` corrected issues automatically
- **Consistent config** - Rules applied to all Python code
- **Fast feedback** - Could validate changes immediately
- **Clear errors** - Line numbers and explanation

**Impact**:
- Code quality stayed high without manual review
- Could iterate quickly (write → lint → fix → commit)
- Prevented bad patterns from spreading

**Best Practice Identified**:
> **Automated quality gates prevent drift.** Linters enforce conventions better than docs.

---

### 8. Rich Terminal Output Libraries

**Library**: `rich` for CLI formatting

**What Made It Effective**:
- **Visual hierarchy** - Colors, boxes, progress bars
- **Clear status** - ✓/✗ symbols, not just text
- **Professional UX** - Users can see what's happening

**Impact**:
- CLI output was readable in terminal logs
- Could see test progress without verbose mode
- Errors stood out visually

**Best Practice Identified**:
> **Good CLI UX helps debugging.** Rich output > plain print statements.

---

## What Created Friction ⚠️

### 1. Missing Authentication Setup for Tests

**Problem**: No documented way to get auth tokens for pytest

**Impact**:
- 4/18 tests failed with "Unauthorized"
- Had to diagnose auth flow from scratch
- Couldn't run scenario tests end-to-end
- Blocked on understanding Dex setup

**What Would Have Helped**:
```markdown
# testing/python/README.md

## Running Tests with Auth

Tests require a valid OAuth token from Dex:

```bash
# Get token
export HERMES_AUTH_TOKEN=$(python3 auth_helper.py get-token \
  --username test@example.com \
  --password password)

# Run tests
pytest tests/ -v
```

Or use the fixture (automatically handles tokens):
```python
@pytest.fixture(scope="session")
def hermes_auth_token():
    return get_token_from_dex()
```
```

**Best Practice Identified**:
> **Document the "zero-to-green" path.** How to go from git clone → all tests passing.

---

### 2. Scattered Configuration Files

**Problem**: Config in multiple places with unclear precedence

**Files**:
- `config.hcl` (tracked, 828 lines)
- `config-example.hcl` (example)
- `configs/config.hcl` (template)
- `testing/config.hcl` (testing-specific)
- `.env` (gitignored, may or may not exist)

**Impact**:
- Unclear which config was actually being used
- Had to trace which file `./hermes server` loaded
- Didn't know if changes should go in `config.hcl` or `testing/config.hcl`

**What Would Have Helped**:
```markdown
# Config File Precedence

1. Command line: `./hermes server -config=custom.hcl`
2. Environment: `HERMES_CONFIG_PATH=/path/to/config.hcl`
3. Default: `./config.hcl` (if exists)

**For testing**: Always use `testing/config.hcl` (testing environment)
**For native dev**: Use `config.hcl` in repo root
```

**Best Practice Identified**:
> **Explicit config precedence prevents confusion.** Document the loading order.

---

### 3. Incomplete Workspace Enum

**Problem**: CLI allowed `--workspace all` but enum only had `TESTING` and `DOCS`

**Code**:
```python
class WorkspaceName(str, Enum):
    TESTING = "testing"
    DOCS = "docs"
    # No ALL!

workspace_map = {
    "all": WorkspaceName.ALL,  # AttributeError
}
```

**Impact**:
- CLI crashed on valid-looking option
- Had to fix enum or change CLI logic
- Wasted time debugging instead of using

**What Would Have Helped**:
Either:
1. Add `ALL` to enum
2. Or remove `all` from CLI choices
3. Or document "all is handled specially"

**Best Practice Identified**:
> **Validate CLI args against implementation.** If it's in `--help`, it should work.

---

### 4. Event Loop Architecture in Client Library

**Problem**: `asyncio.run()` creates/destroys loops incompatibly with pytest

**Code**:
```python
# In hc-hermes client
def get_web_config(self) -> WebConfig:
    return asyncio.run(self._async_client.get_web_config())  # Creates loop
```

**Impact**:
- Tests crashed with "Event loop is closed"
- Had to add workarounds (event loop fixtures)
- Still couldn't fully solve without client refactor
- Created false impression tests were broken

**What Would Have Helped**:
```python
# Support async context manager pattern
async with HermesAsync(base_url=...) as client:
    config = await client.get_web_config()  # Uses existing loop
```

**Best Practice Identified**:
> **Library design affects testability.** Async libraries should support both sync and async usage patterns.

---

### 5. Unclear "All Workspaces" Semantics

**Problem**: What does "all workspaces" mean?

**Ambiguity**:
- Does it mean "testing + docs"?
- Or "every workspace in config"?
- Or "seed both independently"?
- Or "seed once in shared location"?

**Impact**:
- Had to make assumptions
- Implemented partial fix (defaulted to TESTING)
- Left TODO for proper implementation

**What Would Have Helped**:
```python
class WorkspaceName(str, Enum):
    """Workspace identifiers.
    
    Note: 'all' is handled by seeding functions iterating over
    [TESTING, DOCS]. It's not a workspace itself.
    """
    TESTING = "testing"
    DOCS = "docs"
```

**Best Practice Identified**:
> **Document non-obvious semantics.** What "all" means should be explicit.

---

### 6. Database Schema Drift

**Problem**: `project_uuid` column was `NOT NULL` in DB but `Optional` in model

**Root Cause**: Migration added constraint but model wasn't updated

**Impact**:
- Hermes server crashed on startup
- Had to trace error through 4 layers (error → SQL → GORM → migration)
- Lost time debugging instead of testing

**What Would Have Helped**:
1. **Schema validation tests** - Check DB matches models
2. **Migration documentation** - Link migration → model changes
3. **Startup checks** - Detect drift and fail fast with clear message

**Best Practice Identified**:
> **Test schema assumptions.** Don't assume DB matches code after migrations.

---

### 7. No Quick Health Check

**Problem**: Had to guess if services were ready

**Manual Process**:
```bash
docker compose ps  # Are containers up?
curl localhost:8001/health  # Is backend responding?
curl localhost:4201/  # Is frontend up?
# But is Dex ready? Is DB migrated? Is search indexed?
```

**Impact**:
- Ran tests before services ready → confusing failures
- Added `make canary` during session but should have existed

**What Would Have Helped**:
```bash
# make health (or make ready)
Checking services...
✓ PostgreSQL (5433) - ready
✓ Meilisearch (7701) - ready  
✓ Dex (5558) - ready
✓ Hermes backend (8001) - ready
✓ Hermes frontend (4201) - ready
✓ Database migrated
✓ Search indexed (1 documents)

All systems ready! 🚀
```

**Best Practice Identified**:
> **One command to verify readiness.** Don't make developers check 5 services manually.

---

## Patterns That Enabled Autonomous Work

### 1. Clear Success Criteria
You provided: "run distributed test scenarios, look for opportunities to increase coverage"

**Why This Worked**:
- Concrete goal (run tests)
- Open-ended exploration (find improvements)
- No false precision ("make exactly 3 improvements")

### 2. Permissive Scope
"Use ./testing and existing python infrastructure" = don't create new harnesses

**Why This Worked**:
- Boundaries were clear (no new frameworks)
- Freedom within boundaries (how to improve was up to me)
- Prevented scope creep

### 3. Iterative Feedback
You interrupted when:
- I was stuck (missing quote)
- Direction was wrong (over-engineering)
- Context was needed ("this usually takes 30s")

**Why This Worked**:
- Caught mistakes early
- Redirected before too much wasted work
- Provided information I couldn't infer

### 4. Infrastructure for Iteration
- `make up` (one command to start everything)
- `ruff check . --fix` (auto-fix issues)
- `pytest tests/ -v` (run all tests)

**Why This Worked**:
- Fast feedback loops
- Could try → test → fix quickly
- No manual setup between iterations

---

## Anti-Patterns That Created Blockers

### 1. Assumed Knowledge
"Just run the tests" assumes I know:
- Auth is required
- How to get tokens
- What "ready" looks like
- Expected pass rate

**Impact**: Wasted time discovering these

### 2. Implicit Prerequisites
Tests required:
- Hermes running (not stated)
- Auth token (not documented)
- Fresh database (not enforced)

**Impact**: Cryptic failures

### 3. Incomplete Deprecation
Bash scripts marked deprecated but:
- Still in tree
- Still referenced in some Makefiles
- No timeline for removal
- Unclear if safe to delete

**Impact**: Confusion about what's "real"

---

## Recommendations for Future Collaboration

### For Repository Maintainers

**Do More Of** ✅:
1. **Front-load context** - Single authoritative docs (like copilot-instructions.md)
2. **Document the "why"** - Architecture decisions, trade-offs, rejected alternatives
3. **One-command workflows** - `make up`, `make test`, `make health`
4. **Auto-fix tooling** - Linters, formatters that fix themselves
5. **Type safety** - Pydantic, enums, type hints
6. **Clear structure** - Flat over nested, obvious over clever

**Do Less Of** ⚠️:
1. **Scattered config** - Consolidate or document precedence clearly
2. **Implicit prerequisites** - Document what's needed before each command
3. **Incomplete migrations** - Keep DB schema and models in sync
4. **Manual verification** - Automate health checks
5. **Ambiguous semantics** - "all" workspaces should be explicit
6. **Partial deprecations** - Fully remove or fully support, no limbo

### For AI Agents (Self-Guidance)

**Do More Of** ✅:
1. **Read docs first** - Check `docs-internal/`, `README.md`, copilot-instructions
2. **Validate assumptions** - "Is Hermes supposed to be running?"
3. **Ask about duration** - "Should this take 2 minutes or 20?"
4. **Follow existing patterns** - New code should look like old code
5. **Check health before tests** - Don't assume services are ready
6. **Admit uncertainty** - "I don't know if..." vs. guessing

**Do Less Of** ⚠️:
1. **Waiting indefinitely** - Set timeouts, ask if stuck
2. **Over-engineering** - Simple fix > architectural refactor
3. **Ignoring errors** - First error = stop and diagnose
4. **Assuming config** - Check which file is actually loaded
5. **Inventing conventions** - Use existing patterns
6. **Silent failures** - Surface errors, don't swallow them

---

## Case Study: What Made This Session Productive

### Enablers
1. **copilot-instructions.md** → Knew port 8001 for testing
2. **Makefile** → Used `make up` to start services
3. **docs-internal/** → Understood auth architecture
4. **ruff** → Auto-fixed linting issues
5. **Pydantic** → Saw config structure clearly
6. **pytest** → Knew how to add fixtures

### Friction Points
1. **No auth fixture** → 4 tests failed, had to diagnose
2. **Event loop issue** → Over-engineered workaround
3. **Workspace enum** → CLI crashed, had to fix
4. **DB schema drift** → Server crashed, had to trace

### Net Result
- Fixed 6 bugs in parallel
- Ran 18 tests (14 passing)
- Created comprehensive analysis
- But: **auth still not integrated** (main blocker)

**Key Insight**: Good infrastructure enabled autonomous work, but missing pieces (auth setup, health checks) created blockers that required human intervention.

---

## Conclusion

**You built infrastructure that makes AI collaboration effective:**

✅ Comprehensive documentation  
✅ Clear conventions  
✅ Automated tooling  
✅ Type safety  
✅ Structured workflows  

**Missing pieces that would unlock more:**

⚠️ Auth setup for tests  
⚠️ Health check automation  
⚠️ Config consolidation  
⚠️ Schema validation  

**The pattern**: AI agents are **force multipliers for well-structured codebases** but **get stuck on implicit knowledge and manual setup**.

Investment in developer experience (docs, Makefiles, linting, types) pays dividends for both humans and AI. The same things that help junior engineers help AI agents:

- Clear documentation
- Automated workflows
- Fast feedback loops
- Explicit conventions
- Type safety

**Bottom line**: You've done the hard work of making the codebase navigable. The remaining friction points (auth, health checks) are solvable with the same patterns you've already established.
