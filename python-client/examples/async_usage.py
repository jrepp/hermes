"""Example: Async usage of Hermes Python client."""

import asyncio

from hc_hermes import AsyncHermes


async def main() -> None:
    """Main async function."""
    async with AsyncHermes(
        base_url="http://localhost:8000",
        auth_token="your-oauth-token-here",
    ) as client:
        # Fetch multiple documents concurrently
        print("Fetching multiple documents concurrently...")
        docs = await asyncio.gather(
            client.documents.get("DOC-123"),
            client.documents.get("DOC-456"),
            client.documents.get("DOC-789"),
        )

        for doc in docs:
            print(f"{doc.full_doc_number}: {doc.title}")

        # Search
        print("\nSearching...")
        results = await client.search.query("kubernetes")
        print(f"Found {results.nb_hits} results")

        # Get user profile
        print("\nGetting user profile...")
        profile = await client.me.get_profile()
        print(f"User: {profile.user.display_name}")


if __name__ == "__main__":
    asyncio.run(main())
