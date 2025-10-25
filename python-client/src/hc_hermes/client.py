"""Synchronous facade for Hermes API client.

This module provides a simple, blocking interface to the async Hermes client
for use in scripts, interactive sessions, and synchronous code.
"""

from __future__ import annotations

import asyncio
from collections.abc import Callable
from functools import wraps
from typing import Any, TypeVar
from uuid import UUID

from hc_hermes.client_async import AsyncHermes
from hc_hermes.config import HermesConfig
from hc_hermes.models import (
    Document,
    DocumentContent,
    DocumentReview,
    MeProfile,
    Project,
    ProjectRelatedResourcesResponse,
    SearchResponse,
    WebConfig,
)

T = TypeVar("T")


def run_async(func: Callable[..., Any]) -> Callable[..., Any]:
    """Decorator to run async function in sync context."""

    @wraps(func)
    def wrapper(self: Any, *args: Any, **kwargs: Any) -> Any:
        """Wrapper that runs async function synchronously."""
        return asyncio.run(func(self, *args, **kwargs))

    return wrapper


class DocumentsAPI:
    """Synchronous API for document operations."""

    def __init__(self, async_client: AsyncHermes) -> None:
        """Initialize documents API."""
        self._async_client = async_client

    def get(self, doc_id: str | UUID) -> Document:
        """Get document by ID or UUID.
        
        Args:
            doc_id: Document GoogleFileID or UUID
            
        Returns:
            Document instance
        """
        return asyncio.run(self._async_client.documents.get(doc_id))

    def update(self, doc_id: str | UUID, **fields: Any) -> Document:
        """Update document fields via PATCH.
        
        Args:
            doc_id: Document GoogleFileID or UUID
            **fields: Fields to update (title, status, summary, etc.)
            
        Returns:
            Updated document instance
        """
        return asyncio.run(self._async_client.documents.update(doc_id, **fields))

    def delete(self, doc_id: str | UUID) -> None:
        """Delete document.
        
        Args:
            doc_id: Document GoogleFileID or UUID
        """
        asyncio.run(self._async_client.documents.delete(doc_id))

    def get_content(self, doc_id: str | UUID) -> DocumentContent:
        """Get document content (Markdown).
        
        Args:
            doc_id: Document GoogleFileID or UUID
            
        Returns:
            DocumentContent with Markdown content
        """
        return asyncio.run(self._async_client.documents.get_content(doc_id))

    def update_content(self, doc_id: str | UUID, content: str) -> None:
        """Update document content (Markdown).
        
        Args:
            doc_id: Document GoogleFileID or UUID
            content: Markdown content
        """
        asyncio.run(self._async_client.documents.update_content(doc_id, content))

    def get_related_resources(
        self, doc_id: str | UUID
    ) -> dict[str, list[dict[str, Any]]]:
        """Get related resources for document.
        
        Args:
            doc_id: Document GoogleFileID or UUID
            
        Returns:
            Dict with externalLinks and hermesDocuments lists
        """
        return asyncio.run(self._async_client.documents.get_related_resources(doc_id))

    def update_related_resources(
        self,
        doc_id: str | UUID,
        external_links: list[dict[str, Any]] | None = None,
        hermes_documents: list[dict[str, Any]] | None = None,
    ) -> None:
        """Update related resources for document.
        
        Args:
            doc_id: Document GoogleFileID or UUID
            external_links: List of external link objects
            hermes_documents: List of Hermes document references
        """
        asyncio.run(
            self._async_client.documents.update_related_resources(
                doc_id, external_links, hermes_documents
            )
        )


class ProjectsAPI:
    """Synchronous API for project operations."""

    def __init__(self, async_client: AsyncHermes) -> None:
        """Initialize projects API."""
        self._async_client = async_client

    def list(self) -> list[Project]:
        """List all workspace projects.
        
        Returns:
            List of Project instances
        """
        return asyncio.run(self._async_client.projects.list())

    def get(self, project_name: str) -> Project:
        """Get project by name.
        
        Args:
            project_name: Project name/ID
            
        Returns:
            Project instance
        """
        return asyncio.run(self._async_client.projects.get(project_name))

    def get_related_resources(
        self, project_name: str
    ) -> ProjectRelatedResourcesResponse:
        """Get related resources for project.
        
        Args:
            project_name: Project name/ID
            
        Returns:
            ProjectRelatedResourcesResponse with links and documents
        """
        return asyncio.run(
            self._async_client.projects.get_related_resources(project_name)
        )


class SearchAPI:
    """Synchronous API for search operations."""

    def __init__(self, async_client: AsyncHermes) -> None:
        """Initialize search API."""
        self._async_client = async_client

    def query(
        self,
        q: str,
        index: str = "docs",
        filters: dict[str, Any] | None = None,
        page: int = 0,
        hits_per_page: int = 20,
    ) -> SearchResponse:
        """Search documents.
        
        Args:
            q: Search query
            index: Search index name (default: "docs")
            filters: Optional filters (product, docType, status, etc.)
            page: Page number (0-indexed)
            hits_per_page: Results per page
            
        Returns:
            SearchResponse with results
        """
        return asyncio.run(
            self._async_client.search.query(q, index, filters, page, hits_per_page)
        )


class ReviewsAPI:
    """Synchronous API for review operations."""

    def __init__(self, async_client: AsyncHermes) -> None:
        """Initialize reviews API."""
        self._async_client = async_client

    def get_my_reviews(self) -> list[DocumentReview]:
        """Get documents awaiting my review.
        
        Returns:
            List of DocumentReview instances
        """
        return asyncio.run(self._async_client.reviews.get_my_reviews())


class MeAPI:
    """Synchronous API for current user operations."""

    def __init__(self, async_client: AsyncHermes) -> None:
        """Initialize me API."""
        self._async_client = async_client

    def get_profile(self) -> MeProfile:
        """Get current user profile.
        
        Returns:
            MeProfile instance
        """
        return asyncio.run(self._async_client.me.get_profile())

    def get_reviews(self) -> list[DocumentReview]:
        """Get documents awaiting my review.
        
        Returns:
            List of DocumentReview instances
        """
        return asyncio.run(self._async_client.me.get_reviews())

    def get_subscriptions(self) -> list[str]:
        """Get my document subscriptions.
        
        Returns:
            List of document IDs
        """
        return asyncio.run(self._async_client.me.get_subscriptions())

    def recently_viewed_docs(self, limit: int = 10) -> list[Document]:
        """Get recently viewed documents.
        
        Args:
            limit: Maximum number of documents to return
            
        Returns:
            List of Document instances
        """
        return asyncio.run(self._async_client.me.recently_viewed_docs(limit))


class Hermes:
    """Synchronous Hermes API client.
    
    This is a simple, blocking wrapper around the async client for ease of use
    in scripts and interactive sessions.
    
    Example:
        client = Hermes(base_url="...", auth_token="...")
        doc = client.documents.get("DOC-123")
        results = client.search.query("RFC kubernetes")
    """

    def __init__(
        self,
        base_url: str | None = None,
        auth_token: str | None = None,
        config: HermesConfig | None = None,
        **kwargs: Any,
    ) -> None:
        """Initialize synchronous Hermes client.
        
        Args:
            base_url: Hermes server URL
            auth_token: OAuth bearer token
            config: HermesConfig instance (overrides other args)
            **kwargs: Additional config options
        """
        self._async_client = AsyncHermes(
            base_url=base_url,
            auth_token=auth_token,
            config=config,
            **kwargs,
        )

        # API namespaces
        self.documents = DocumentsAPI(self._async_client)
        self.projects = ProjectsAPI(self._async_client)
        self.search = SearchAPI(self._async_client)
        self.reviews = ReviewsAPI(self._async_client)
        self.me = MeAPI(self._async_client)

    def get_web_config(self) -> WebConfig:
        """Get web configuration (auth provider, etc.).
        
        Returns:
            WebConfig instance
        """
        return asyncio.run(self._async_client.get_web_config())

    def set_auth_token(self, token: str) -> None:
        """Update authentication token.
        
        Args:
            token: OAuth bearer token
        """
        self._async_client.set_auth_token(token)
