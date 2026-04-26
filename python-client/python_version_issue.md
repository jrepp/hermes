# Python Version Issue

## Problem

The virtual environment was created with **Python 3.9.6**, but the code uses **Python 3.10+ union syntax** (`int | None`, `str | None`) throughout. This causes import errors and test failures.

## Error Manifestation

```
TypeError: unsupported operand type(s) for |: 'type' and 'NoneType'
```

This error occurs when Python 3.9 tries to parse type annotations using the `|` operator (PEP 604), which was introduced in Python 3.10.

## Affected Files

All files using Python 3.10+ union syntax:
- `src/hc_hermes/models.py` - 100+ instances
- `src/hc_hermes/config.py` - Multiple instances
- `src/hc_hermes/http_client.py` - Multiple instances
- `src/hc_hermes/client_async.py` - Multiple instances
- `src/hc_hermes/client.py` - Multiple instances
- `src/hc_hermes/utils.py` - Multiple instances
- `src/hc_hermes/cli.py` - Multiple instances

## Solution Options

### Option 1: Recreate Virtual Environment with Python 3.10+ (RECOMMENDED)

**If you have Python 3.10+ installed:**

```bash
cd /Users/jrepp/hc/hermes/python-client

# Remove old venv
rm -rf .venv

# Create new venv with Python 3.10+ (adjust version as needed)
python3.10 -m venv .venv
# or
python3.11 -m venv .venv
# or
python3.12 -m venv .venv

# Activate and install dependencies
source .venv/bin/activate
pip install --upgrade pip
pip install -e ".[dev]"
```

**If you need to install Python 3.10+:**

On macOS with Homebrew:
```bash
brew install python@3.10
# or
brew install python@3.11
# or
brew install python@3.12
```

Then follow the venv recreation steps above.

### Option 2: Convert Union Syntax to typing.Optional/Union (NOT RECOMMENDED)

This would require changing all `X | None` to `Optional[X]` and all `X | Y` to `Union[X, Y]` throughout the codebase (~200+ changes). This is error-prone and defeats the purpose of using modern Python syntax.

## Verification

After recreating the venv, verify it works:

```bash
# Check Python version
.venv/bin/python --version  # Should show 3.10+

# Run tests
PYTHONPATH=src .venv/bin/pytest tests/ -v

# Run linting
.venv/bin/ruff check src/ tests/

# Run type checking
.venv/bin/mypy src/ tests/
```

## Current Status

- ✅ Linting (ruff) runs successfully (239 issues found, 83 auto-fixed)
- ❌ Testing (pytest) fails due to import errors
- ❌ Type checking (mypy) not tested yet (likely will fail with same issue)

## Next Steps

1. Install Python 3.10+ if not available
2. Recreate virtual environment with correct Python version
3. Reinstall dependencies
4. Run full test suite
5. Address linting issues (156 remaining after auto-fix)
