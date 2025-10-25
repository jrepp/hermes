"""Exception classes for hc-hermes client."""

from __future__ import annotations

from typing import Any


class HermesError(Exception):
    """Base exception for all Hermes client errors."""

    def __init__(self, message: str, **kwargs: Any) -> None:
        """Initialize exception with message and optional context."""
        super().__init__(message)
        self.message = message
        self.context = kwargs


class HermesAPIError(HermesError):
    """Exception raised for API errors."""

    def __init__(
        self,
        message: str,
        status_code: int | None = None,
        response_body: str | dict[str, Any] | None = None,
        **kwargs: Any,
    ) -> None:
        """Initialize API error with status code and response details."""
        super().__init__(message, **kwargs)
        self.status_code = status_code
        self.response_body = response_body

    def __str__(self) -> str:
        """Format error message with status code."""
        if self.status_code:
            return f"[{self.status_code}] {self.message}"
        return self.message


class HermesAuthError(HermesError):
    """Exception raised for authentication/authorization errors."""



class HermesNotFoundError(HermesAPIError):
    """Exception raised when a resource is not found (404)."""

    def __init__(
        self,
        message: str = "Resource not found",
        resource_type: str | None = None,
        resource_id: str | None = None,
        **kwargs: Any,
    ) -> None:
        """Initialize not found error with resource details."""
        super().__init__(message, status_code=404, **kwargs)
        self.resource_type = resource_type
        self.resource_id = resource_id

    def __str__(self) -> str:
        """Format error message with resource details."""
        if self.resource_type and self.resource_id:
            return f"{self.resource_type} '{self.resource_id}' not found"
        return self.message


class HermesValidationError(HermesError):
    """Exception raised for validation errors."""

    def __init__(
        self,
        message: str,
        field: str | None = None,
        value: Any = None,
        **kwargs: Any,
    ) -> None:
        """Initialize validation error with field details."""
        super().__init__(message, **kwargs)
        self.field = field
        self.value = value

    def __str__(self) -> str:
        """Format error message with field details."""
        if self.field:
            return f"Validation error for field '{self.field}': {self.message}"
        return self.message


class HermesConnectionError(HermesError):
    """Exception raised for connection errors."""



class HermesTimeoutError(HermesError):
    """Exception raised for timeout errors."""



class HermesRateLimitError(HermesAPIError):
    """Exception raised when rate limit is exceeded."""

    def __init__(
        self,
        message: str = "Rate limit exceeded",
        retry_after: int | None = None,
        **kwargs: Any,
    ) -> None:
        """Initialize rate limit error with retry timing."""
        super().__init__(message, status_code=429, **kwargs)
        self.retry_after = retry_after

    def __str__(self) -> str:
        """Format error message with retry timing."""
        if self.retry_after:
            return f"{self.message}. Retry after {self.retry_after} seconds."
        return self.message
