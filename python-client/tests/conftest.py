"""Test configuration and fixtures."""

import pytest


@pytest.fixture
def mock_hermes_config() -> dict[str, str]:
    """Mock Hermes configuration."""
    return {
        "base_url": "http://localhost:8000",
        "auth_token": "test-token-123",
    }


@pytest.fixture
def sample_document_data() -> dict[str, any]:
    """Sample document data for testing."""
    return {
        "id": 1,
        "title": "Test Document",
        "status": "WIP",
        "documentNumber": 123,
        "product": {"id": 1, "name": "terraform", "abbreviation": "TF"},
        "documentType": {"id": 1, "name": "RFC"},
        "summary": "This is a test document",
    }
