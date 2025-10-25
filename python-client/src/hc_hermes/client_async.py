"""Async API client for Hermes V2 API."""

from __future__ import annotations

from typing import Any
from uuid import UUID

from hc_hermes.config import HermesConfig
from hc_hermes.http_client import AsyncHTTPClient
from hc_hermes.models import (
    Document,
    DocumentContent,
    DocumentPatchRequest,
    DocumentReview,
    MeProfile,
    Project,
    ProjectRelatedResourcesResponse,
    SearchResponse,
    WebConfig,
)


class AsyncDocumentsAPI:
    """Async API for document operations."""

    def __init__(self, client: AsyncHTTPClient) -> None:
        """Initialize documents API."""
        self._client = client

    async def get(self, doc_id: str | UUID) -> Document:
        """Get document by ID or UUID.
        
        Args:
            doc_id: Document GoogleFileID or UUID
            
        Returns:
            Document instance
        """
        response = await self._client.get(f"documents/{doc_id}")
        return Document.model_validate(response.json())

    async def update(self, doc_id: str | UUID, **fields: Any) -> Document:
        """Update document fields via PATCH.
        
        Args:
            doc_id: Document GoogleFileID or UUID
            **fields: Fields to update (title, status, summary, etc.)
            
        Returns:
            Updated document instance
        """
        # Validate and convert fields
        patch_data = DocumentPatchRequest(**fields)
        response = await self._client.patch(
            f"documents/{doc_id}",
            json=patch_data.model_dump(exclude_none=True),
        )
        return Document.model_validate(response.json())

    async def delete(self, doc_id: str | UUID) -> None:
        """Delete document.
        
        Args:
            doc_id: Document GoogleFileID or UUID
        """
        await self._client.delete(f"documents/{doc_id}")

    async def get_content(self, doc_id: str | UUID) -> DocumentContent:
        """Get document content (Markdown).
        
        Args:
            doc_id: Document GoogleFileID or UUID
            
        Returns:
            DocumentContent with Markdown content
        """
        response = await self._client.get(f"documents/{doc_id}/content")
        data = response.json()
        return DocumentContent(content=data.get("content", ""), document_id=str(doc_id))

    async def update_content(self, doc_id: str | UUID, content: str) -> None:
        """Update document content (Markdown).
        
        Args:
            doc_id: Document GoogleFileID or UUID
            content: Markdown content
        """
        await self._client.put(f"documents/{doc_id}/content", json={"content": content})

    async def get_related_resources(
        self, doc_id: str | UUID
    ) -> dict[str, list[dict[str, Any]]]:
        """Get related resources for document.
        
        Args:
            doc_id: Document GoogleFileID or UUID
            
        Returns:
            Dict with externalLinks and hermesDocuments lists
        """
        response = await self._client.get(f"documents/{doc_id}/related-resources")
        return response.json()

    async def update_related_resources(
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
        payload: dict[str, Any] = {}
        if external_links is not None:
            payload["externalLinks"] = external_links
        if hermes_documents is not None:
            payload["hermesDocuments"] = hermes_documents

        await self._client.put(f"documents/{doc_id}/related-resources", json=payload)


class AsyncProjectsAPI:
    """Async API for project operations."""

    def __init__(self, client: AsyncHTTPClient) -> None:
        """Initialize projects API."""
        self._client = client

    async def list(self) -> list[Project]:
        """List all workspace projects.
        
        Returns:
            List of Project instances
        """
        response = await self._client.get("workspace-projects")
        data = response.json()
        return [Project.model_validate(p) for p in data]

    async def get(self, project_name: str) -> Project:
        """Get project by name.
        
        Args:
            project_name: Project name/ID
            
        Returns:
            Project instance
        """
        response = await self._client.get(f"workspace-projects/{project_name}")
        return Project.model_validate(response.json())

    async def get_related_resources(
        self, project_name: str
    ) -> ProjectRelatedResourcesResponse:
        """Get related resources for project.
        
        Args:
            project_name: Project name/ID
            
        Returns:
            ProjectRelatedResourcesResponse with links and documents
        """
        response = await self._client.get(f"projects/{project_name}/related-resources")
        return ProjectRelatedResourcesResponse.model_validate(response.json())


class AsyncSearchAPI:
    """Async API for search operations."""

    def __init__(self, client: AsyncHTTPClient) -> None:
        """Initialize search API."""
        self._client = client

    async def query(
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
        payload = {
            "query": q,
            "page": page,
            "hitsPerPage": hits_per_page,
        }
        if filters:
            payload["filters"] = filters

        response = await self._client.post(f"search/{index}", json=payload)
        return SearchResponse.model_validate(response.json())


class AsyncReviewsAPI:
    """Async API for review operations."""

    def __init__(self, client: AsyncHTTPClient) -> None:
        """Initialize reviews API."""
        self._client = client

    async def get_my_reviews(self) -> list[DocumentReview]:
        """Get documents awaiting my review.
        
        Returns:
            List of DocumentReview instances
        """
        response = await self._client.get("me/reviews")
        data = response.json()
        return [DocumentReview.model_validate(r) for r in data]


class AsyncMeAPI:
    """Async API for current user operations."""

    def __init__(self, client: AsyncHTTPClient) -> None:
        """Initialize me API."""
        self._client = client

    async def get_profile(self) -> MeProfile:
        """Get current user profile.
        
        Returns:
            MeProfile instance
        """
        response = await self._client.get("me")
        return MeProfile.model_validate(response.json())

    async def get_reviews(self) -> list[DocumentReview]:
        """Get documents awaiting my review.
        
        Returns:
            List of DocumentReview instances
        """
        response = await self._client.get("me/reviews")
        data = response.json()
        return [DocumentReview.model_validate(r) for r in data]

    async def get_subscriptions(self) -> list[str]:
        """Get my document subscriptions.
        
        Returns:
            List of document IDs
        """
        response = await self._client.get("me/subscriptions")
        data = response.json()
        return data.get("subscriptions", [])

    async def recently_viewed_docs(self, limit: int = 10) -> list[Document]:
        """Get recently viewed documents.
        
        Args:
            limit: Maximum number of documents to return
            
        Returns:
            List of Document instances
        """
        response = await self._client.get(
            "me/recently-viewed-docs", params={"limit": limit}
        )
        data = response.json()
        return [Document.model_validate(d) for d in data]


class AsyncHermes:
    """Async Hermes API client.
    
    Example:
        async with AsyncHermes(base_url="...", auth_token="...") as client:
            doc = await client.documents.get("DOC-123")
            results = await client.search.query("RFC kubernetes")
    """

    def __init__(
        self,
        base_url: str | None = None,
        auth_token: str | None = None,
        config: HermesConfig | None = None,
        **kwargs: Any,
    ) -> None:
        """Initialize async Hermes client.
        
        Args:
            base_url: Hermes server URL
            auth_token: OAuth bearer token
            config: HermesConfig instance (overrides other args)
            **kwargs: Additional config options
        """
        if config is None:
            config = HermesConfig(
                base_url=base_url or "http://localhost:8000",
                auth_token=auth_token,
                **kwargs,
            )

        self.config = config
        self._http_client = AsyncHTTPClient(config)

        # API namespaces
        self.documents = AsyncDocumentsAPI(self._http_client)
        self.projects = AsyncProjectsAPI(self._http_client)
        self.search = AsyncSearchAPI(self._http_client)
        self.reviews = AsyncReviewsAPI(self._http_client)
        self.me = AsyncMeAPI(self._http_client)

    async def __aenter__(self) -> AsyncHermes:
        """Enter async context manager."""
        await self._http_client.start()
        return self

    async def __aexit__(self, *args: Any) -> None:
        """Exit async context manager."""
        await self._http_client.close()

    async def get_web_config(self) -> WebConfig:
        """Get web configuration (auth provider, etc.).
        
        Returns:
            WebConfig instance
        """
        response = await self._http_client.get("web/config")
        return WebConfig.model_validate(response.json())

    def set_auth_token(self, token: str) -> None:
        """Update authentication token.
        
        Args:
            token: OAuth bearer token
        """
        self._http_client.set_auth_token(token)
