"""Example: Basic usage of Hermes Python client."""

from hc_hermes import Hermes

# Initialize client
client = Hermes(
    base_url="http://localhost:8000",
    auth_token="your-oauth-token-here",
)

# Get a document
print("Fetching document...")
doc = client.documents.get("DOC-123")
print(f"Title: {doc.title}")
print(f"Status: {doc.status.value}")
print(f"Product: {doc.product.name if doc.product else 'N/A'}")

# Search for documents
print("\nSearching for RFCs...")
results = client.search.query("RFC", filters={"docType": "RFC"})
print(f"Found {results.nb_hits} results")

for hit in results.hits[:5]:
    print(f"  - {hit.doc_number}: {hit.title}")

# Get document content
print("\nFetching document content...")
content = client.documents.get_content("DOC-123")
print(f"Content length: {len(content.content)} characters")
