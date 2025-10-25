# Hermes Python Client - Quick Start Guide

This guide will help you get started with the Hermes Python client library.

## Installation

### 1. Prerequisites

- Python 3.11 or higher
- uv package manager (recommended) or pip
- Access to a Hermes server instance

### 2. Install uv (if not already installed)

```bash
curl -LsSf https://astral.sh/uv/install.sh | sh
```

### 3. Install the Package

From source (current development):

```bash
cd python-client
uv pip install -e ".[cli]"
```

## Configuration

### Option 1: Environment Variables

```bash
export HERMES_BASE_URL="http://localhost:8000"
export HERMES_AUTH_TOKEN="your-oauth-token"
```

### Option 2: Configuration File

Create `~/.hermes/config.yaml`:

```yaml
base_url: "http://localhost:8000"
auth_token: "your-oauth-token"
timeout: 30
max_retries: 3
```

### Option 3: Programmatic

```python
from hc_hermes import Hermes

client = Hermes(
    base_url="http://localhost:8000",
    auth_token="your-oauth-token"
)
```

## Basic Usage

### Get a Document

```python
from hc_hermes import Hermes

client = Hermes(base_url="...", auth_token="...")
doc = client.documents.get("DOC-123")

print(f"Title: {doc.title}")
print(f"Status: {doc.status.value}")
print(f"Product: {doc.product.name if doc.product else 'N/A'}")
```

### Search Documents

```python
results = client.search.query(
    "RFC kubernetes",
    filters={"product": "vault", "status": "Approved"}
)

for hit in results.hits:
    print(f"{hit.doc_number}: {hit.title}")
```

### Update Document

```python
updated_doc = client.documents.update(
    "DOC-123",
    title="Updated Title",
    status="Approved",
    summary="New summary"
)
```

### Work with Document Content

```python
# Get content
content = client.documents.get_content("DOC-123")
print(content.content)

# Update content
client.documents.update_content("DOC-123", "# New Content\n\nUpdated text...")
```

## Using the CLI

### Basic Commands

```bash
# Get document
hermes documents get DOC-123

# Search
hermes search "RFC kubernetes" --product=vault --limit=10

# List projects
hermes projects list

# Create template
hermes template create my-rfc.md --type=RFC --title="My RFC" --product=vault
```

### Configuration

```bash
# Show current configuration
hermes config --show
```

## Advanced Usage

### Async Client

```python
import asyncio
from hc_hermes import AsyncHermes

async def main():
    async with AsyncHermes(base_url="...", auth_token="...") as client:
        # Concurrent operations
        docs = await asyncio.gather(
            client.documents.get("DOC-123"),
            client.documents.get("DOC-456"),
        )
        
        for doc in docs:
            print(doc.title)

asyncio.run(main())
```

### Frontmatter Parsing

```python
from hc_hermes.utils import parse_markdown_document

# Parse Markdown file with YAML frontmatter
parsed = parse_markdown_document("path/to/doc.md")

print(f"Title: {parsed.title}")
print(f"Type: {parsed.doc_type}")
print(f"Content length: {len(parsed.content)}")
```

### Error Handling

```python
from hc_hermes import Hermes
from hc_hermes.exceptions import (
    HermesNotFoundError,
    HermesAuthError,
    HermesAPIError,
)

client = Hermes(base_url="...", auth_token="...")

try:
    doc = client.documents.get("DOC-999")
except HermesNotFoundError:
    print("Document not found")
except HermesAuthError:
    print("Authentication failed")
except HermesAPIError as e:
    print(f"API error: {e.status_code} - {e.message}")
```

## Testing with Hermes Testing Environment

### 1. Start Testing Environment

```bash
cd ../testing
make up
```

### 2. Configure Client

```bash
export HERMES_BASE_URL="http://localhost:8001"
export HERMES_AUTH_TOKEN="test-token"  # Get from Dex login
```

### 3. Run Integration Tests

```bash
cd ../python-client
python examples/integration_test.py
```

## Common Patterns

### Batch Document Updates

```python
doc_ids = ["DOC-123", "DOC-456", "DOC-789"]

for doc_id in doc_ids:
    client.documents.update(doc_id, status="Approved")
    print(f"Approved {doc_id}")
```

### Search and Filter

```python
# Search for RFCs in a specific product
results = client.search.query(
    "authentication",
    filters={
        "docType": "RFC",
        "product": "vault",
        "status": "Approved"
    }
)

# Print results
for hit in results.hits:
    print(f"{hit.doc_number}: {hit.title}")
```

### Working with Projects

```python
# List all projects
projects = client.projects.list()

for project in projects:
    print(f"{project.name}: {project.title}")
    
    # Get related resources
    resources = client.projects.get_related_resources(project.name)
    print(f"  External links: {len(resources.external_links)}")
    print(f"  Hermes docs: {len(resources.hermes_documents)}")
```

## Troubleshooting

### Import Errors

If you see import errors, ensure the package is installed:

```bash
uv pip install -e ".[cli]"
```

### Authentication Errors

1. Check your auth token is valid
2. Verify the base URL is correct
3. Ensure the Hermes server is running

### Connection Errors

```python
from hc_hermes.config import HermesConfig

# Increase timeout and retries
config = HermesConfig(
    base_url="...",
    auth_token="...",
    timeout=60.0,
    max_retries=5
)

client = Hermes(config=config)
```

## Next Steps

- See [README.md](README.md) for full API documentation
- Check [examples/](examples/) for more usage examples
- Read [DEVELOPMENT.md](DEVELOPMENT.md) for contributing guidelines

## Support

- Issues: https://github.com/hashicorp/hermes/issues
- Documentation: https://github.com/hashicorp/hermes/tree/main/python-client
