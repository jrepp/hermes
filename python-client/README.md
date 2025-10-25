# hc-hermes

[![PyPI version](https://badge.fury.io/py/hc-hermes.svg)](https://badge.fury.io/py/hc-hermes)
[![Python Versions](https://img.shields.io/pypi/pyversions/hc-hermes.svg)](https://pypi.org/project/hc-hermes/)
[![License: MPL 2.0](https://img.shields.io/badge/License-MPL%202.0-brightgreen.svg)](https://opensource.org/licenses/MPL-2.0)

**Professional Python client library for the HashiCorp Hermes document management system.**

This library provides a clean, type-safe, and fully async Python interface to the Hermes V2 API, with built-in OAuth authentication, comprehensive Pydantic models, and utilities for working with Markdown documents with YAML frontmatter.

## Features

- ✅ **Full V2 API Support** - Complete coverage of Hermes V2 REST API endpoints
- ✅ **Type-Safe** - Built with Pydantic models for validation and IDE autocomplete
- ✅ **Async First** - Asyncio-based for high-performance concurrent operations
- ✅ **Synchronous Facade** - Simple blocking API for scripts and interactive use
- ✅ **OAuth 2.0** - Google OAuth authentication with automatic token refresh
- ✅ **Frontmatter Parser** - Parse and create documents from Markdown files
- ✅ **CLI Tool** - Command-line interface for common operations
- ✅ **Strict Linting** - Ruff with comprehensive rules for code quality
- ✅ **100% Type Coverage** - Full mypy strict mode compliance
- ✅ **Production Ready** - Comprehensive error handling and logging

## Installation

### From PyPI (when published)

```bash
pip install hc-hermes
```

### With CLI support

```bash
pip install hc-hermes[cli]
```

### For development

```bash
# Using uv (recommended)
uv pip install -e ".[dev,cli]"

# Or using pip
pip install -e ".[dev,cli]"
```

## Quick Start

### Synchronous API (Simple)

```python
from hc_hermes import Hermes

# Initialize client
client = Hermes(
    base_url="https://hermes.example.com",
    auth_token="your-oauth-token"
)

# Get a document
doc = client.documents.get("DOC-123")
print(f"Title: {doc.title}")
print(f"Status: {doc.status}")

# Search documents
results = client.search.query("RFC kubernetes")
for result in results:
    print(f"- {result.title} ({result.doc_number})")

# Update document
client.documents.update(
    doc_id="DOC-123",
    title="Updated Title",
    status="approved"
)
```

### Asynchronous API (High Performance)

```python
import asyncio
from hc_hermes import AsyncHermes

async def main():
    async with AsyncHermes(
        base_url="https://hermes.example.com",
        auth_token="your-oauth-token"
    ) as client:
        # Concurrent document fetching
        docs = await asyncio.gather(
            client.documents.get("DOC-123"),
            client.documents.get("DOC-456"),
            client.documents.get("DOC-789"),
        )
        
        for doc in docs:
            print(f"{doc.doc_number}: {doc.title}")

asyncio.run(main())
```

### Working with Markdown Files

```python
from hc_hermes import Hermes
from hc_hermes.utils import parse_markdown_document

# Parse a local Markdown file with frontmatter
doc_data = parse_markdown_document("path/to/rfc-123.md")

# Create document in Hermes
client = Hermes(base_url="...", auth_token="...")
doc = client.documents.create(
    title=doc_data.title,
    doc_type=doc_data.doc_type,
    product=doc_data.product,
    summary=doc_data.summary,
    content=doc_data.content,
)

print(f"Created document: {doc.doc_number}")
```

### Using the CLI

```bash
# Authenticate
hermes auth login

# Get document
hermes documents get DOC-123

# Search
hermes search "RFC kubernetes"

# Create from Markdown
hermes documents create-from-file rfc-123.md

# Update document content
hermes documents update-content DOC-123 --file updated-content.md

# List projects
hermes projects list
```

## Authentication

### Google OAuth Token

```python
from hc_hermes import Hermes

client = Hermes(
    base_url="https://hermes.hashicorp.com",
    auth_token="ya29.a0AfH6SMB..."  # Google OAuth token
)
```

### OAuth Flow (CLI)

The CLI provides an interactive OAuth flow:

```bash
hermes auth login
# Opens browser for Google OAuth
# Saves token to ~/.hermes/credentials.json
```

Then use the CLI without specifying tokens:

```bash
hermes documents get DOC-123
```

## API Coverage

### Documents
- `documents.get(doc_id)` - Get document by ID/UUID
- `documents.create(...)` - Create new document
- `documents.update(doc_id, ...)` - Update document metadata
- `documents.delete(doc_id)` - Delete document
- `documents.get_content(doc_id)` - Get document content
- `documents.update_content(doc_id, content)` - Update document content
- `documents.list_related_resources(doc_id)` - Get related resources
- `documents.update_related_resources(doc_id, ...)` - Update related resources

### Projects
- `projects.list()` - List all projects
- `projects.get(project_name)` - Get project details
- `projects.list_related_resources(project_name)` - Get project resources

### Search
- `search.query(q, filters={})` - Search documents
- `search.faceted_search(...)` - Advanced faceted search

### Reviews
- `reviews.list(doc_id)` - List document reviews
- `reviews.create(doc_id, reviewers)` - Request review
- `reviews.approve(doc_id)` - Approve document

### User/Me
- `me.get_profile()` - Get current user profile
- `me.get_reviews()` - Get documents awaiting my review
- `me.get_subscriptions()` - Get document subscriptions
- `me.recently_viewed_docs()` - Recently viewed documents

## Models

All API responses are typed with Pydantic models:

```python
from hc_hermes.models import (
    Document,
    DocumentStatus,
    Project,
    User,
    Review,
    SearchResult,
    DocumentContent,
)

# Type-safe document creation
doc = Document(
    title="My RFC",
    doc_type="RFC",
    product="terraform",
    status=DocumentStatus.WIP,
    summary="This is a test document",
)
```

## Utilities

### Frontmatter Parser

```python
from hc_hermes.utils import parse_markdown_document, DocumentParser

# Simple parsing
data = parse_markdown_document("rfc-123.md")
print(data.title, data.content)

# Advanced parsing with custom schema
parser = DocumentParser(
    required_fields=["title", "doc_type", "product"],
    optional_fields=["summary", "tags", "approvers"],
)
data = parser.parse_file("path/to/doc.md")
```

### Document Templates

```python
from hc_hermes.utils import create_document_template

# Generate RFC template
template = create_document_template(
    doc_type="RFC",
    title="My New RFC",
    product="vault",
    author="user@hashicorp.com"
)

with open("new-rfc.md", "w") as f:
    f.write(template)
```

## Development

### Setup

```bash
# Clone repository
git clone https://github.com/hashicorp/hermes.git
cd hermes/python-client

# Install uv (recommended)
curl -LsSf https://astral.sh/uv/install.sh | sh

# Create virtual environment and install dependencies
uv venv
source .venv/bin/activate  # or `.venv\Scripts\activate` on Windows
uv pip install -e ".[dev,cli]"
```

### Running Tests

```bash
# Run all tests with coverage
pytest

# Run specific test file
pytest tests/test_client.py

# Run with verbose output
pytest -v

# Run only async tests
pytest -k async

# Generate HTML coverage report
pytest --cov-report=html
open htmlcov/index.html
```

### Linting and Type Checking

```bash
# Lint with ruff (strict mode)
ruff check .

# Format code
ruff format .

# Type check with mypy
mypy src tests

# All checks
ruff check . && ruff format --check . && mypy src tests
```

### Pre-commit Checks

```bash
# Run all checks before committing
./scripts/pre-commit.sh
```

## Configuration

The client can be configured via environment variables:

```bash
export HERMES_BASE_URL="https://hermes.example.com"
export HERMES_AUTH_TOKEN="your-token"
export HERMES_TIMEOUT=30
export HERMES_MAX_RETRIES=3
```

Or via a configuration file (`~/.hermes/config.yaml`):

```yaml
base_url: "https://hermes.example.com"
auth:
  token: "your-token"
  # Or use OAuth flow
  oauth:
    client_id: "your-client-id"
    client_secret: "your-client-secret"
timeout: 30
max_retries: 3
```

## Contributing

Contributions welcome! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

Mozilla Public License 2.0 - See [LICENSE](../LICENSE) for details.

## Links

- **Documentation**: https://github.com/hashicorp/hermes/tree/main/python-client
- **Issues**: https://github.com/hashicorp/hermes/issues
- **Hermes Project**: https://github.com/hashicorp/hermes
