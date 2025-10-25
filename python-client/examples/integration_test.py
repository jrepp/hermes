"""Integration test example using the Hermes testing environment.

This example demonstrates how to use the Python client with the Hermes
testing environment (from ../testing/).

Prerequisites:
1. Start the testing environment:
   cd ../testing && make up

2. Set environment variables:
   export HERMES_BASE_URL="http://localhost:8001"
   export HERMES_AUTH_TOKEN="your-token-from-dex-login"

3. Run this script:
   python examples/integration_test.py
"""

import asyncio
import os
import sys

from hc_hermes import AsyncHermes, Hermes
from hc_hermes.exceptions import HermesError


def test_sync_client() -> None:
    """Test synchronous client against testing environment."""
    print("=== Testing Synchronous Client ===\n")

    # Get configuration from environment
    base_url = os.getenv("HERMES_BASE_URL", "http://localhost:8001")
    auth_token = os.getenv("HERMES_AUTH_TOKEN")

    if not auth_token:
        print("Warning: HERMES_AUTH_TOKEN not set, some operations may fail")

    try:
        client = Hermes(base_url=base_url, auth_token=auth_token)

        # Test 1: Get web config
        print("1. Getting web configuration...")
        config = client.get_web_config()
        print(f"   Auth Provider: {config.auth_provider}")
        print(f"   Base URL: {config.base_url}")

        # Test 2: List projects
        print("\n2. Listing projects...")
        projects = client.projects.list()
        print(f"   Found {len(projects)} projects")
        for project in projects[:3]:
            print(f"   - {project.name}: {project.title}")

        # Test 3: Search documents
        print("\n3. Searching documents...")
        results = client.search.query("test", hits_per_page=5)
        print(f"   Found {results.nb_hits} total results")
        for hit in results.hits:
            print(f"   - {hit.title} ({hit.status.value if hit.status else 'N/A'})")

        print("\n✓ Synchronous client tests passed!")

    except HermesError as e:
        print(f"\n✗ Error: {e}")
        sys.exit(1)


async def test_async_client() -> None:
    """Test asynchronous client against testing environment."""
    print("\n=== Testing Asynchronous Client ===\n")

    base_url = os.getenv("HERMES_BASE_URL", "http://localhost:8001")
    auth_token = os.getenv("HERMES_AUTH_TOKEN")

    try:
        async with AsyncHermes(base_url=base_url, auth_token=auth_token) as client:
            # Test 1: Concurrent project fetching
            print("1. Fetching projects concurrently...")
            projects = await client.projects.list()
            print(f"   Found {len(projects)} projects")

            # Test 2: Search
            print("\n2. Searching documents...")
            results = await client.search.query("RFC", hits_per_page=5)
            print(f"   Found {results.nb_hits} total results")

            # Test 3: Get user profile (requires auth)
            if auth_token:
                print("\n3. Getting user profile...")
                try:
                    profile = await client.me.get_profile()
                    print(f"   User: {profile.user.display_name}")
                except HermesError as e:
                    print(f"   Could not get profile: {e}")

        print("\n✓ Asynchronous client tests passed!")

    except HermesError as e:
        print(f"\n✗ Error: {e}")
        sys.exit(1)


def main() -> None:
    """Run integration tests."""
    print("Hermes Python Client - Integration Tests")
    print("=" * 50)

    # Check if testing environment is running
    base_url = os.getenv("HERMES_BASE_URL", "http://localhost:8001")
    print(f"\nTarget: {base_url}")
    print("(Ensure testing environment is running: cd ../testing && make up)\n")

    # Run sync tests
    test_sync_client()

    # Run async tests
    asyncio.run(test_async_client())

    print("\n" + "=" * 50)
    print("All integration tests passed! ✓")


if __name__ == "__main__":
    main()
