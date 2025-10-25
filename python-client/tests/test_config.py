"""Tests for configuration management."""


import pytest

from hc_hermes.config import HermesConfig


def test_default_config() -> None:
    """Test default configuration values."""
    config = HermesConfig()
    assert config.base_url == "http://localhost:8000"
    assert config.api_version == "v2"
    assert config.timeout == 30.0
    assert config.max_retries == 3
    assert config.verify_ssl is True
    assert config.log_level == "INFO"


def test_config_from_env(monkeypatch: pytest.MonkeyPatch) -> None:
    """Test configuration from environment variables."""
    monkeypatch.setenv("HERMES_BASE_URL", "https://hermes.example.com")
    monkeypatch.setenv("HERMES_AUTH_TOKEN", "test-token")
    monkeypatch.setenv("HERMES_TIMEOUT", "60")
    monkeypatch.setenv("HERMES_LOG_LEVEL", "DEBUG")

    config = HermesConfig()
    assert config.base_url == "https://hermes.example.com"
    assert config.auth_token == "test-token"
    assert config.timeout == 60.0
    assert config.log_level == "DEBUG"


def test_config_base_url_stripped() -> None:
    """Test that base URL trailing slash is removed."""
    config = HermesConfig(base_url="https://hermes.example.com/")
    assert config.base_url == "https://hermes.example.com"


def test_config_get_api_url() -> None:
    """Test API URL construction."""
    config = HermesConfig(base_url="https://hermes.example.com")

    # Test with leading slash
    url = config.get_api_url("/documents/DOC-123")
    assert url == "https://hermes.example.com/api/v2/documents/DOC-123"

    # Test without leading slash
    url = config.get_api_url("documents/DOC-123")
    assert url == "https://hermes.example.com/api/v2/documents/DOC-123"


def test_config_invalid_log_level() -> None:
    """Test that invalid log level raises error."""
    with pytest.raises(ValueError, match="log_level must be one of"):
        HermesConfig(log_level="INVALID")


def test_config_timeout_validation() -> None:
    """Test timeout validation."""
    # Valid timeout
    config = HermesConfig(timeout=60.0)
    assert config.timeout == 60.0

    # Negative timeout should fail
    with pytest.raises(ValueError):
        HermesConfig(timeout=-1.0)


def test_config_max_retries_validation() -> None:
    """Test max retries validation."""
    # Valid retries
    config = HermesConfig(max_retries=5)
    assert config.max_retries == 5

    # Too many retries should fail
    with pytest.raises(ValueError):
        HermesConfig(max_retries=20)

    # Negative retries should fail
    with pytest.raises(ValueError):
        HermesConfig(max_retries=-1)
