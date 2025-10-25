"""Async HTTP client for Hermes API with OAuth authentication."""

from __future__ import annotations

import asyncio
import logging
from typing import Any

import httpx

from hc_hermes.config import HermesConfig
from hc_hermes.exceptions import (
    HermesAPIError,
    HermesAuthError,
    HermesConnectionError,
    HermesNotFoundError,
    HermesRateLimitError,
    HermesTimeoutError,
)

logger = logging.getLogger(__name__)


class AsyncHTTPClient:
    """Async HTTP client with OAuth authentication and error handling."""

    def __init__(self, config: HermesConfig) -> None:
        """Initialize async HTTP client.
        
        Args:
            config: Hermes configuration instance
        """
        self.config = config
        self._client: httpx.AsyncClient | None = None
        self._auth_token: str | None = config.auth_token

    async def __aenter__(self) -> AsyncHTTPClient:
        """Enter async context manager."""
        await self.start()
        return self

    async def __aexit__(self, *args: Any) -> None:
        """Exit async context manager."""
        await self.close()

    async def start(self) -> None:
        """Start the HTTP client session."""
        if self._client is not None:
            return

        self._client = httpx.AsyncClient(
            base_url=self.config.base_url,
            timeout=httpx.Timeout(self.config.timeout),
            verify=self.config.verify_ssl,
            headers=self._get_headers(),
        )

    async def close(self) -> None:
        """Close the HTTP client session."""
        if self._client is not None:
            await self._client.aclose()
            self._client = None

    def _get_headers(self) -> dict[str, str]:
        """Get HTTP headers including authentication."""
        headers = {
            "Accept": "application/json",
            "Content-Type": "application/json",
            "User-Agent": "hc-hermes-python/0.1.0",
        }

        if self._auth_token:
            headers["Authorization"] = f"Bearer {self._auth_token}"

        return headers

    def set_auth_token(self, token: str) -> None:
        """Update authentication token.
        
        Args:
            token: OAuth bearer token
        """
        self._auth_token = token
        if self._client is not None:
            self._client.headers["Authorization"] = f"Bearer {token}"

    async def request(
        self,
        method: str,
        path: str,
        *,
        params: dict[str, Any] | None = None,
        json: dict[str, Any] | None = None,
        data: dict[str, Any] | None = None,
        headers: dict[str, str] | None = None,
        retry_count: int = 0,
    ) -> httpx.Response:
        """Make HTTP request with error handling and retries.
        
        Args:
            method: HTTP method (GET, POST, PUT, PATCH, DELETE)
            path: API path (will be prefixed with /api/v2/)
            params: Query parameters
            json: JSON request body
            data: Form data
            headers: Additional headers
            retry_count: Current retry attempt number
            
        Returns:
            HTTP response
            
        Raises:
            HermesAuthError: Authentication failed
            HermesNotFoundError: Resource not found
            HermesRateLimitError: Rate limit exceeded
            HermesAPIError: Other API errors
            HermesConnectionError: Connection failed
            HermesTimeoutError: Request timed out
        """
        if self._client is None:
            await self.start()

        assert self._client is not None

        # Build full URL
        url = self.config.get_api_url(path)

        # Merge headers
        request_headers = self._get_headers()
        if headers:
            request_headers.update(headers)

        try:
            logger.debug(f"{method} {url}", extra={"params": params, "json": json})

            response = await self._client.request(
                method=method,
                url=url,
                params=params,
                json=json,
                data=data,
                headers=request_headers,
            )

            # Handle error responses
            if response.status_code >= 400:
                await self._handle_error_response(response)

            logger.debug(
                f"{method} {url} -> {response.status_code}",
                extra={"response": response.text[:200]},
            )

            return response

        except httpx.TimeoutException as e:
            if retry_count < self.config.max_retries:
                logger.warning(
                    f"Request timeout, retrying ({retry_count + 1}/{self.config.max_retries})"
                )
                await asyncio.sleep(2**retry_count)  # Exponential backoff
                return await self.request(
                    method,
                    path,
                    params=params,
                    json=json,
                    data=data,
                    headers=headers,
                    retry_count=retry_count + 1,
                )
            raise HermesTimeoutError(f"Request timed out: {e}") from e

        except httpx.ConnectError as e:
            if retry_count < self.config.max_retries:
                logger.warning(
                    f"Connection failed, retrying ({retry_count + 1}/{self.config.max_retries})"
                )
                await asyncio.sleep(2**retry_count)
                return await self.request(
                    method,
                    path,
                    params=params,
                    json=json,
                    data=data,
                    headers=headers,
                    retry_count=retry_count + 1,
                )
            raise HermesConnectionError(f"Connection failed: {e}") from e

        except httpx.HTTPError as e:
            raise HermesAPIError(f"HTTP error: {e}") from e

    async def _handle_error_response(self, response: httpx.Response) -> None:
        """Handle error responses and raise appropriate exceptions.
        
        Args:
            response: HTTP response with error status
            
        Raises:
            HermesAuthError: 401 or 403
            HermesNotFoundError: 404
            HermesRateLimitError: 429
            HermesAPIError: Other errors
        """
        status_code = response.status_code

        # Try to parse error body
        try:
            error_body = response.json()
            error_message = error_body.get("error", error_body.get("message", response.text))
        except Exception:
            error_message = response.text or f"HTTP {status_code} error"

        # Handle specific status codes
        if status_code == 401:
            raise HermesAuthError(
                f"Authentication failed: {error_message}",
                status_code=status_code,
                response_body=error_message,
            )

        if status_code == 403:
            raise HermesAuthError(
                f"Permission denied: {error_message}",
                status_code=status_code,
                response_body=error_message,
            )

        if status_code == 404:
            raise HermesNotFoundError(
                error_message,
                status_code=status_code,
                response_body=error_message,
            )

        if status_code == 429:
            retry_after = response.headers.get("Retry-After")
            retry_seconds = int(retry_after) if retry_after else None
            raise HermesRateLimitError(
                error_message,
                retry_after=retry_seconds,
                status_code=status_code,
                response_body=error_message,
            )

        # Generic API error
        raise HermesAPIError(
            error_message,
            status_code=status_code,
            response_body=error_message,
        )

    async def get(self, path: str, **kwargs: Any) -> httpx.Response:
        """Make GET request."""
        return await self.request("GET", path, **kwargs)

    async def post(self, path: str, **kwargs: Any) -> httpx.Response:
        """Make POST request."""
        return await self.request("POST", path, **kwargs)

    async def put(self, path: str, **kwargs: Any) -> httpx.Response:
        """Make PUT request."""
        return await self.request("PUT", path, **kwargs)

    async def patch(self, path: str, **kwargs: Any) -> httpx.Response:
        """Make PATCH request."""
        return await self.request("PATCH", path, **kwargs)

    async def delete(self, path: str, **kwargs: Any) -> httpx.Response:
        """Make DELETE request."""
        return await self.request("DELETE", path, **kwargs)
