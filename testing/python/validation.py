"""Validation utilities for Hermes testing framework.

Provides assertions and validation helpers to verify system state,
document indexing, search functionality, and API responses.
"""

from __future__ import annotations

import asyncio
from datetime import datetime, timedelta
from typing import Any, Callable, Optional

from hc_hermes import Hermes
from hc_hermes.exceptions import HermesError
from hc_hermes.models import Document, SearchResponse
from rich.console import Console
from tenacity import retry, stop_after_delay, wait_fixed

from config import config

console = Console()


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


class ValidationError(Exception):
    """Raised when validation fails."""

    pass


class HermesValidator:
    """Validate Hermes system state and operations."""

    def __init__(self, client: Hermes | None = None) -> None:
        """Initialize validator.

        Args:
            client: Hermes client (creates new if None)
        """
        self.client = client or Hermes(
            base_url=config.hermes_base_url,
            auth_token=config.hermes_auth_token,
        )
        self.console = console
        
        # Token refresh support
        self._token_expires_at: Optional[datetime] = None
        self._token_refresh_callback: Optional[Callable[[], str]] = None

    def set_token_refresh(
        self,
        refresh_callback: Callable[[], str],
        expires_in_seconds: int = 3600,
    ) -> None:
        """Configure automatic token refresh for long-running scenarios.

        Args:
            refresh_callback: Function that returns a fresh auth token
            expires_in_seconds: Token lifetime in seconds (default: 1 hour)

        Example:
            >>> from auth_helper import get_dex_token_for_testing
            >>> validator.set_token_refresh(
            ...     refresh_callback=get_dex_token_for_testing,
            ...     expires_in_seconds=3600
            ... )
        """
        self._token_refresh_callback = refresh_callback
        self._token_expires_at = datetime.now() + timedelta(seconds=expires_in_seconds)
        self.console.print(
            f"✓ Token refresh configured (expires in {expires_in_seconds}s)",
            style="green",
        )

    def _refresh_token_if_needed(self) -> None:
        """Check token expiration and refresh if needed."""
        if not self._token_refresh_callback or not self._token_expires_at:
            return

        # Check if token is about to expire (within 5 minutes)
        if datetime.now() >= (self._token_expires_at - timedelta(minutes=5)):
            try:
                self.console.print("⟳ Refreshing authentication token...", style="yellow")
                new_token = self._token_refresh_callback()
                
                # Update config (affects new client instances)
                config.hermes_auth_token = new_token
                
                # Update expiration time
                self._token_expires_at = datetime.now() + timedelta(hours=1)
                
                self.console.print("✓ Token refreshed successfully", style="green")
            except Exception as e:
                self.console.print(
                    f"⚠️  Token refresh failed: {e}",
                    style="yellow",
                )

    def check_health(self) -> bool:
        """Check if Hermes is healthy.

        Returns:
            True if healthy, False otherwise
        """
        try:
            # Ensure we have a valid event loop before making async calls
            _safe_get_event_loop()
            # Try to get web config as health check
            self.client.get_web_config()
            return True
        except HermesError:
            return False

    def assert_healthy(self) -> None:
        """Assert Hermes is healthy, raise if not."""
        if not self.check_health():
            raise ValidationError(
                f"Hermes is not healthy at {config.hermes_base_url}\n"
                f"Start with: cd testing && make up"
            )
        self.console.print("✓ Hermes is healthy", style="green")

    @retry(
        stop=stop_after_delay(120),  # 2 minutes max
        wait=wait_fixed(5),  # Check every 5 seconds
        reraise=True,
    )
    def wait_for_indexing(self, expected_count: int, timeout: int | None = None) -> int:
        """Wait for documents to be indexed.

        Args:
            expected_count: Expected number of documents
            timeout: Maximum wait time in seconds (uses config default if None)

        Returns:
            Actual document count

        Raises:
            ValidationError: If timeout reached before expected count
        """
        # Refresh token if needed (for long waits)
        self._refresh_token_if_needed()
        
        # Note: tenacity handles the timeout via stop_after_delay
        try:
            # Recreate client to avoid event loop issues with asyncio.run()
            # The sync Hermes client uses asyncio.run() which can leave closed loops
            client = Hermes(
                base_url=config.hermes_base_url,
                auth_token=config.hermes_auth_token,
            )
            results = client.search.query("*", hits_per_page=1)
            actual_count = results.nb_hits

            self.console.print(
                f"  Documents indexed: {actual_count} / {expected_count}",
                style="blue",
            )

            if actual_count >= expected_count:
                return actual_count

            # If not enough, retry (tenacity will handle this)
            raise ValidationError(f"Only {actual_count}/{expected_count} indexed")

        except RuntimeError as e:
            # Handle event loop errors gracefully
            if "event loop" in str(e).lower() or "closed" in str(e).lower():
                raise ValidationError(
                    f"Event loop error (will retry): {e}"
                ) from e
            raise
        except HermesError as e:
            raise ValidationError(f"Failed to query documents: {e}") from e

    def assert_document_count(
        self,
        expected_count: int,
        doc_type: str | None = None,
        wait: bool = True,
    ) -> None:
        """Assert document count matches expected.

        Args:
            expected_count: Expected number of documents
            doc_type: Filter by document type (None for all)
            wait: Wait for indexing if True

        Raises:
            ValidationError: If count doesn't match
        """
        if wait:
            try:
                actual = self.wait_for_indexing(expected_count)
            except Exception as e:
                raise ValidationError(
                    f"Timeout waiting for {expected_count} documents: {e}"
                ) from e
        else:
            try:
                # Create fresh client to avoid event loop issues
                client = Hermes(
                    base_url=config.hermes_base_url,
                    auth_token=config.hermes_auth_token,
                )
                results = client.search.query("*", hits_per_page=1)
                actual = results.nb_hits
            except HermesError as e:
                raise ValidationError(f"Failed to query documents: {e}") from e

        if actual < expected_count:
            raise ValidationError(
                f"Expected {expected_count} documents, found {actual}\n"
                f"Note: Indexer scans every 5 minutes, you may need to wait longer."
            )

        self.console.print(
            f"✓ Found {actual} documents (expected {expected_count})",
            style="green",
        )

    def assert_search_results(
        self,
        query: str,
        min_results: int = 1,
        max_results: int | None = None,
    ) -> SearchResponse:
        """Assert search query returns expected number of results.

        Args:
            query: Search query
            min_results: Minimum expected results
            max_results: Maximum expected results (no limit if None)

        Returns:
            Search response

        Raises:
            ValidationError: If result count out of range
        """
        try:
            # Create fresh client to avoid event loop issues
            client = Hermes(
                base_url=config.hermes_base_url,
                auth_token=config.hermes_auth_token,
            )
            results = client.search.query(query, hits_per_page=100)
        except HermesError as e:
            raise ValidationError(f"Search failed for '{query}': {e}") from e

        actual = len(results.hits)

        if actual < min_results:
            raise ValidationError(
                f"Search '{query}' returned {actual} results, expected >= {min_results}"
            )

        if max_results is not None and actual > max_results:
            raise ValidationError(
                f"Search '{query}' returned {actual} results, expected <= {max_results}"
            )

        self.console.print(
            f"✓ Search '{query}' returned {actual} results",
            style="green",
        )

        return results

    def assert_document_exists(self, doc_id: str) -> Document:
        """Assert document exists and is accessible.

        Args:
            doc_id: Document ID or UUID

        Returns:
            Document instance

        Raises:
            ValidationError: If document not found
        """
        try:
            # Create fresh client to avoid event loop issues
            client = Hermes(
                base_url=config.hermes_base_url,
                auth_token=config.hermes_auth_token,
            )
            doc = client.documents.get(doc_id)
            self.console.print(
                f"✓ Document '{doc_id}' exists: {doc.title}",
                style="green",
            )
            return doc
        except HermesError as e:
            raise ValidationError(f"Document '{doc_id}' not found: {e}") from e

    def assert_document_content(
        self,
        doc_id: str,
        expected_substring: str,
    ) -> None:
        """Assert document content contains expected text.

        Args:
            doc_id: Document ID or UUID
            expected_substring: Text that should appear in content

        Raises:
            ValidationError: If content doesn't contain substring
        """
        try:
            # Create fresh client to avoid event loop issues
            client = Hermes(
                base_url=config.hermes_base_url,
                auth_token=config.hermes_auth_token,
            )
            content = client.documents.get_content(doc_id)
            if expected_substring not in content.content:
                raise ValidationError(
                    f"Document '{doc_id}' content missing '{expected_substring}'"
                )
            self.console.print(
                f"✓ Document '{doc_id}' contains expected content",
                style="green",
            )
        except HermesError as e:
            raise ValidationError(f"Failed to get content for '{doc_id}': {e}") from e

    def get_document_stats(self) -> dict[str, Any]:
        """Get document statistics.

        Returns:
            Dictionary with document counts by type, status, etc.
        """
        try:
            # Create fresh client to avoid event loop issues
            client = Hermes(
                base_url=config.hermes_base_url,
                auth_token=config.hermes_auth_token,
            )
            # Get all documents via search
            results = client.search.query("*", hits_per_page=1000)

            stats: dict[str, Any] = {
                "total": results.nb_hits,
                "by_type": {},
                "by_status": {},
            }

            for hit in results.hits:
                # Count by type
                doc_type = hit.document_type.name if hit.document_type else "Unknown"
                stats["by_type"][doc_type] = stats["by_type"].get(doc_type, 0) + 1

                # Count by status
                status = hit.status.value if hit.status else "Unknown"
                stats["by_status"][status] = stats["by_status"].get(status, 0) + 1

            return stats

        except HermesError as e:
            raise ValidationError(f"Failed to get document stats: {e}") from e

    def print_stats(self) -> None:
        """Print document statistics to console."""
        stats = self.get_document_stats()

        self.console.print("\n[bold]Document Statistics:[/bold]")
        self.console.print(f"  Total: {stats['total']}")

        if stats["by_type"]:
            self.console.print("\n  By Type:")
            for doc_type, count in sorted(stats["by_type"].items()):
                self.console.print(f"    {doc_type}: {count}")

        if stats["by_status"]:
            self.console.print("\n  By Status:")
            for status, count in sorted(stats["by_status"].items()):
                self.console.print(f"    {status}: {count}")

        self.console.print()


# Convenience instance
validator = HermesValidator()
