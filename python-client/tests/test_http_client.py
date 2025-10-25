"""Tests for HTTP client with authentication."""

from unittest.mock import AsyncMock, MagicMock, patch

import httpx
import pytest

from hc_hermes.config import HermesConfig
from hc_hermes.exceptions import (
    HermesAuthError,
    HermesConnectionError,
    HermesNotFoundError,
    HermesRateLimitError,
    HermesTimeoutError,
)
from hc_hermes.http_client import AsyncHTTPClient


@pytest.fixture
def config() -> HermesConfig:
    """Create test configuration."""
    return HermesConfig(
        base_url="http://test.example.com",
        auth_token="test-token",
        timeout=10.0,
        max_retries=2,
    )


@pytest.fixture
def client(config: HermesConfig) -> AsyncHTTPClient:
    """Create HTTP client instance."""
    return AsyncHTTPClient(config)


@pytest.mark.asyncio
async def test_client_context_manager(config: HermesConfig) -> None:
    """Test client as async context manager."""
    async with AsyncHTTPClient(config) as client:
        assert client._client is not None
    # Client should be closed after context
    assert client._client is None


@pytest.mark.asyncio
async def test_client_start_close(client: AsyncHTTPClient) -> None:
    """Test manual start and close."""
    assert client._client is None
    await client.start()
    assert client._client is not None
    await client.close()
    assert client._client is None


def test_get_headers(client: AsyncHTTPClient) -> None:
    """Test header generation."""
    headers = client._get_headers()
    assert headers["Authorization"] == "Bearer test-token"
    assert headers["Accept"] == "application/json"
    assert headers["Content-Type"] == "application/json"
    assert "hc-hermes-python" in headers["User-Agent"]


def test_set_auth_token(client: AsyncHTTPClient) -> None:
    """Test updating auth token."""
    client.set_auth_token("new-token")
    assert client._auth_token == "new-token"


@pytest.mark.asyncio
async def test_successful_get_request(client: AsyncHTTPClient) -> None:
    """Test successful GET request."""
    mock_response = MagicMock(spec=httpx.Response)
    mock_response.status_code = 200
    mock_response.json.return_value = {"test": "data"}
    mock_response.text = '{"test": "data"}'

    with patch.object(httpx.AsyncClient, "request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        await client.start()
        response = await client.get("test/path")

        assert response.status_code == 200
        assert response.json() == {"test": "data"}
        mock_request.assert_called_once()


@pytest.mark.asyncio
async def test_auth_error_401(client: AsyncHTTPClient) -> None:
    """Test 401 authentication error."""
    mock_response = MagicMock(spec=httpx.Response)
    mock_response.status_code = 401
    mock_response.text = "Unauthorized"
    mock_response.json.return_value = {"error": "Invalid token"}

    with patch.object(httpx.AsyncClient, "request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        await client.start()
        with pytest.raises(HermesAuthError, match="Authentication failed"):
            await client.get("test/path")


@pytest.mark.asyncio
async def test_not_found_error_404(client: AsyncHTTPClient) -> None:
    """Test 404 not found error."""
    mock_response = MagicMock(spec=httpx.Response)
    mock_response.status_code = 404
    mock_response.text = "Not found"
    mock_response.json.return_value = {"error": "Resource not found"}

    with patch.object(httpx.AsyncClient, "request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        await client.start()
        with pytest.raises(HermesNotFoundError):
            await client.get("test/path")


@pytest.mark.asyncio
async def test_rate_limit_error_429(client: AsyncHTTPClient) -> None:
    """Test 429 rate limit error."""
    mock_response = MagicMock(spec=httpx.Response)
    mock_response.status_code = 429
    mock_response.text = "Rate limit exceeded"
    mock_response.json.return_value = {"error": "Too many requests"}
    mock_response.headers = {"Retry-After": "60"}

    with patch.object(httpx.AsyncClient, "request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        await client.start()
        with pytest.raises(HermesRateLimitError) as exc_info:
            await client.get("test/path")

        assert exc_info.value.retry_after == 60


@pytest.mark.asyncio
async def test_timeout_with_retry(client: AsyncHTTPClient) -> None:
    """Test timeout with automatic retry."""
    with patch.object(httpx.AsyncClient, "request", new_callable=AsyncMock) as mock_request:
        # First two calls timeout, third succeeds
        mock_response = MagicMock(spec=httpx.Response)
        mock_response.status_code = 200
        mock_response.text = "success"

        mock_request.side_effect = [
            httpx.TimeoutException("Timeout 1"),
            httpx.TimeoutException("Timeout 2"),
            mock_response,
        ]

        await client.start()

        # Should succeed on third try (max_retries=2)
        with patch("asyncio.sleep", new_callable=AsyncMock):  # Speed up test
            response = await client.get("test/path")

        assert response.status_code == 200
        assert mock_request.call_count == 3


@pytest.mark.asyncio
async def test_timeout_exceeds_retries(client: AsyncHTTPClient) -> None:
    """Test timeout that exceeds max retries."""
    with patch.object(httpx.AsyncClient, "request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.TimeoutException("Timeout")

        await client.start()

        with patch("asyncio.sleep", new_callable=AsyncMock):
            with pytest.raises(HermesTimeoutError):
                await client.get("test/path")

        # Should try 3 times (initial + 2 retries)
        assert mock_request.call_count == 3


@pytest.mark.asyncio
async def test_connection_error_with_retry(client: AsyncHTTPClient) -> None:
    """Test connection error with retry."""
    with patch.object(httpx.AsyncClient, "request", new_callable=AsyncMock) as mock_request:
        mock_request.side_effect = httpx.ConnectError("Connection failed")

        await client.start()

        with patch("asyncio.sleep", new_callable=AsyncMock):
            with pytest.raises(HermesConnectionError):
                await client.get("test/path")


@pytest.mark.asyncio
async def test_post_request_with_json(client: AsyncHTTPClient) -> None:
    """Test POST request with JSON body."""
    mock_response = MagicMock(spec=httpx.Response)
    mock_response.status_code = 201
    mock_response.json.return_value = {"created": True}

    with patch.object(httpx.AsyncClient, "request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        await client.start()
        response = await client.post("test/path", json={"test": "data"})

        assert response.status_code == 201
        call_kwargs = mock_request.call_args[1]
        assert call_kwargs["json"] == {"test": "data"}


@pytest.mark.asyncio
async def test_all_http_methods(client: AsyncHTTPClient) -> None:
    """Test all HTTP method helpers."""
    mock_response = MagicMock(spec=httpx.Response)
    mock_response.status_code = 200

    with patch.object(httpx.AsyncClient, "request", new_callable=AsyncMock) as mock_request:
        mock_request.return_value = mock_response

        await client.start()

        await client.get("test")
        assert mock_request.call_args[0][0] == "GET"

        await client.post("test")
        assert mock_request.call_args[0][0] == "POST"

        await client.put("test")
        assert mock_request.call_args[0][0] == "PUT"

        await client.patch("test")
        assert mock_request.call_args[0][0] == "PATCH"

        await client.delete("test")
        assert mock_request.call_args[0][0] == "DELETE"
