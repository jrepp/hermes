"""
hc-hermes: Python client library for HashiCorp Hermes document management system.

This package provides both synchronous and asynchronous interfaces to the Hermes V2 API,
with full type safety, OAuth authentication, and utilities for working with Markdown documents.
"""

from hc_hermes.client import Hermes
from hc_hermes.client_async import AsyncHermes
from hc_hermes.config import HermesConfig
from hc_hermes.exceptions import (
    HermesAPIError,
    HermesAuthError,
    HermesError,
    HermesNotFoundError,
    HermesValidationError,
)
from hc_hermes.models import (
    Document,
    DocumentStatus,
    DocumentType,
    Project,
    Review,
    User,
)

__version__ = "0.1.0"
__all__ = [
    # Main clients
    "Hermes",
    "AsyncHermes",
    # Configuration
    "HermesConfig",
    # Exceptions
    "HermesError",
    "HermesAPIError",
    "HermesAuthError",
    "HermesNotFoundError",
    "HermesValidationError",
    # Models
    "Document",
    "DocumentStatus",
    "DocumentType",
    "Project",
    "Review",
    "User",
]
