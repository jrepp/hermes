"""Example: Working with Markdown files."""

from pathlib import Path

from hc_hermes import Hermes
from hc_hermes.utils import create_document_template, parse_markdown_document

# Create a document template
print("Creating document template...")
template = create_document_template(
    doc_type="RFC",
    title="My New RFC",
    product="vault",
    author="author@example.com",
    summary="This RFC proposes a new feature",
)

# Save template
template_path = Path("my-rfc.md")
template_path.write_text(template)
print(f"Template saved to {template_path}")

# Parse an existing document
print("\nParsing existing document...")
parsed = parse_markdown_document("my-rfc.md")
print(f"Title: {parsed.title}")
print(f"Type: {parsed.doc_type}")
print(f"Product: {parsed.product}")
print(f"Status: {parsed.status}")

# Update document content in Hermes
print("\nUpdating document content in Hermes...")
client = Hermes(
    base_url="http://localhost:8000",
    auth_token="your-oauth-token-here",
)

# Get content with frontmatter
content_with_frontmatter = template_path.read_text()

# Update (requires existing document)
# client.documents.update_content("DOC-123", content_with_frontmatter)
# print("Document content updated!")

# Clean up
template_path.unlink()
print("Template file removed")
