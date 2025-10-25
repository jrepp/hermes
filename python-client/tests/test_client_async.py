"""Tests for async API client."""

from unittest.mock import AsyncMock, MagicMock, patch
from uuid import uuid4

import pytest

from hc_hermes.client_async import AsyncHermes
from hc_hermes.config import HermesConfig
from hc_hermes.models import DocumentStatus


@pytest.fixture
def config() -> HermesConfig:
    """Create test configuration."""
    return HermesConfig(
        base_url="http://test.example.com",
        auth_token="test-token",
    )


@pytest.fixture
def client(config: HermesConfig) -> AsyncHermes:
    """Create async client instance."""
    return AsyncHermes(config=config)


@pytest.mark.asyncio
async def test_documents_get(client: AsyncHermes) -> None:
    """Test getting a document."""
    doc_data = {
        "id": 1,
        "title": "Test Document",
        "status": "WIP",
        "documentNumber": 123,
    }

    mock_response = MagicMock()
    mock_response.json.return_value = doc_data

    with patch.object(client._http_client, "get", new_callable=AsyncMock) as mock_get:
        mock_get.return_value = mock_response

        doc = await client.documents.get("DOC-123")

        assert doc.title == "Test Document"
        assert doc.status == DocumentStatus.WIP
        mock_get.assert_called_once_with("documents/DOC-123")


@pytest.mark.asyncio
async def test_documents_update(client: AsyncHermes) -> None:
    """Test updating a document."""
    updated_data = {
        "id": 1,
        "title": "Updated Title",
        "status": "Approved",
    }

    mock_response = MagicMock()
    mock_response.json.return_value = updated_data

    with patch.object(client._http_client, "patch", new_callable=AsyncMock) as mock_patch:
        mock_patch.return_value = mock_response

        doc = await client.documents.update("DOC-123", title="Updated Title", status="Approved")

        assert doc.title == "Updated Title"
        mock_patch.assert_called_once()
        call_kwargs = mock_patch.call_args[1]
        assert "title" in call_kwargs["json"]


@pytest.mark.asyncio
async def test_documents_get_content(client: AsyncHermes) -> None:
    """Test getting document content."""
    content_data = {"content": "# Test Document\n\nContent here"}

    mock_response = MagicMock()
    mock_response.json.return_value = content_data

    with patch.object(client._http_client, "get", new_callable=AsyncMock) as mock_get:
        mock_get.return_value = mock_response

        content = await client.documents.get_content("DOC-123")

        assert "Test Document" in content.content
        mock_get.assert_called_once_with("documents/DOC-123/content")


@pytest.mark.asyncio
async def test_documents_update_content(client: AsyncHermes) -> None:
    """Test updating document content."""
    mock_response = MagicMock()

    with patch.object(client._http_client, "put", new_callable=AsyncMock) as mock_put:
        mock_put.return_value = mock_response

        await client.documents.update_content("DOC-123", "# New Content")

        mock_put.assert_called_once()
        call_kwargs = mock_put.call_args[1]
        assert call_kwargs["json"]["content"] == "# New Content"


@pytest.mark.asyncio
async def test_projects_list(client: AsyncHermes) -> None:
    """Test listing projects."""
    projects_data = [
        {
            "project_uuid": str(uuid4()),
            "project_id": "test-project",
            "name": "test-project",
            "title": "Test Project",
            "jira_enabled": False,
        }
    ]

    mock_response = MagicMock()
    mock_response.json.return_value = projects_data

    with patch.object(client._http_client, "get", new_callable=AsyncMock) as mock_get:
        mock_get.return_value = mock_response

        projects = await client.projects.list()

        assert len(projects) == 1
        assert projects[0].name == "test-project"
        mock_get.assert_called_once_with("workspace-projects")


@pytest.mark.asyncio
async def test_projects_get(client: AsyncHermes) -> None:
    """Test getting a project."""
    project_data = {
        "project_uuid": str(uuid4()),
        "project_id": "test-project",
        "name": "test-project",
        "title": "Test Project",
    }

    mock_response = MagicMock()
    mock_response.json.return_value = project_data

    with patch.object(client._http_client, "get", new_callable=AsyncMock) as mock_get:
        mock_get.return_value = mock_response

        project = await client.projects.get("test-project")

        assert project.name == "test-project"
        mock_get.assert_called_once_with("workspace-projects/test-project")


@pytest.mark.asyncio
async def test_search_query(client: AsyncHermes) -> None:
    """Test searching documents."""
    search_data = {
        "hits": [
            {
                "objectID": "DOC-123",
                "title": "Test Document",
                "status": "WIP",
            }
        ],
        "nbHits": 1,
        "page": 0,
        "nbPages": 1,
        "hitsPerPage": 20,
    }

    mock_response = MagicMock()
    mock_response.json.return_value = search_data

    with patch.object(client._http_client, "post", new_callable=AsyncMock) as mock_post:
        mock_post.return_value = mock_response

        results = await client.search.query("test", filters={"product": "vault"})

        assert results.nb_hits == 1
        assert len(results.hits) == 1
        assert results.hits[0].title == "Test Document"
        mock_post.assert_called_once()


@pytest.mark.asyncio
async def test_me_get_profile(client: AsyncHermes) -> None:
    """Test getting user profile."""
    profile_data = {
        "user": {
            "id": 1,
            "email_address": "test@example.com",
            "name": "Test User",
        },
        "subscriptions": ["DOC-123", "DOC-456"],
    }

    mock_response = MagicMock()
    mock_response.json.return_value = profile_data

    with patch.object(client._http_client, "get", new_callable=AsyncMock) as mock_get:
        mock_get.return_value = mock_response

        profile = await client.me.get_profile()

        assert profile.user.email_address == "test@example.com"
        mock_get.assert_called_once_with("me")


@pytest.mark.asyncio
async def test_set_auth_token(client: AsyncHermes) -> None:
    """Test updating auth token."""
    client.set_auth_token("new-token")
    assert client._http_client._auth_token == "new-token"


@pytest.mark.asyncio
async def test_client_context_manager(config: HermesConfig) -> None:
    """Test client as async context manager."""
    async with AsyncHermes(config=config) as client:
        assert client._http_client._client is not None
