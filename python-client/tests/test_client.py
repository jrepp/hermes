"""Tests for synchronous client facade."""

from unittest.mock import AsyncMock, MagicMock, patch

import pytest

from hc_hermes.client import Hermes
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
def client(config: HermesConfig) -> Hermes:
    """Create sync client instance."""
    return Hermes(config=config)


def test_documents_get(client: Hermes) -> None:
    """Test getting a document synchronously."""
    doc_data = {
        "id": 1,
        "title": "Test Document",
        "status": "WIP",
    }

    mock_response = MagicMock()
    mock_response.json.return_value = doc_data

    with patch.object(
        client._async_client.documents, "get", new_callable=AsyncMock
    ) as mock_get:
        mock_get.return_value = MagicMock(
            title="Test Document",
            status=DocumentStatus.WIP,
        )

        doc = client.documents.get("DOC-123")

        assert doc.title == "Test Document"
        assert doc.status == DocumentStatus.WIP


def test_search_query(client: Hermes) -> None:
    """Test searching documents synchronously."""
    with patch.object(
        client._async_client.search, "query", new_callable=AsyncMock
    ) as mock_query:
        mock_query.return_value = MagicMock(
            nb_hits=5,
            hits=[],
        )

        results = client.search.query("test")

        assert results.nb_hits == 5


def test_projects_list(client: Hermes) -> None:
    """Test listing projects synchronously."""
    with patch.object(
        client._async_client.projects, "list", new_callable=AsyncMock
    ) as mock_list:
        mock_list.return_value = []

        projects = client.projects.list()

        assert isinstance(projects, list)


def test_set_auth_token(client: Hermes) -> None:
    """Test setting auth token."""
    client.set_auth_token("new-token")
    assert client._async_client._http_client._auth_token == "new-token"
