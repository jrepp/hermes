"""Configuration management for hc-hermes client."""

from __future__ import annotations

from pathlib import Path
from typing import Any, Optional

from pydantic import Field, field_validator
from pydantic_settings import BaseSettings, SettingsConfigDict


class HermesConfig(BaseSettings):
    """Configuration for Hermes client.
    
    Can be configured via environment variables (HERMES_*) or passed directly.
    """

    model_config = SettingsConfigDict(
        env_prefix="hermes_",
        env_file=".env",
        env_file_encoding="utf-8",
        case_sensitive=False,
        extra="ignore",
    )

    # Server configuration
    base_url: str = Field(
        default="http://localhost:8000",
        description="Base URL of the Hermes server",
    )

    # Authentication
    auth_token: Optional[str] = Field(
        default=None,
        description="OAuth bearer token for authentication",
    )

    # HTTP client settings
    timeout: float = Field(
        default=30.0,
        ge=0,
        description="Request timeout in seconds",
    )

    max_retries: int = Field(
        default=3,
        ge=0,
        le=10,
        description="Maximum number of retry attempts for failed requests",
    )

    verify_ssl: bool = Field(
        default=True,
        description="Verify SSL certificates",
    )

    # API version
    api_version: str = Field(
        default="v2",
        description="API version to use",
    )

    # Logging
    log_level: str = Field(
        default="INFO",
        description="Logging level (DEBUG, INFO, WARNING, ERROR, CRITICAL)",
    )

    # Credentials file path
    credentials_path: Path = Field(
        default=Path.home() / ".hermes" / "credentials.json",
        description="Path to stored credentials file",
    )

    @field_validator("base_url")
    @classmethod
    def validate_base_url(cls, v: str) -> str:
        """Ensure base URL doesn't end with a slash."""
        return v.rstrip("/")

    @field_validator("log_level")
    @classmethod
    def validate_log_level(cls, v: str) -> str:
        """Validate log level is valid."""
        valid_levels = {"DEBUG", "INFO", "WARNING", "ERROR", "CRITICAL"}
        v_upper = v.upper()
        if v_upper not in valid_levels:
            raise ValueError(f"log_level must be one of {valid_levels}")
        return v_upper

    def get_api_url(self, path: str) -> str:
        """Construct full API URL from path.
        
        Args:
            path: API path (e.g., "/documents/DOC-123")
            
        Returns:
            Full URL (e.g., "http://localhost:8000/api/v2/documents/DOC-123")
        """
        # Remove leading slash from path if present
        path = path.lstrip("/")
        return f"{self.base_url}/api/{self.api_version}/{path}"

    def dict(self, **kwargs: Any) -> dict[str, Any]:
        """Export configuration as dictionary."""
        return self.model_dump(**kwargs)

    @classmethod
    def from_file(cls, path: Path | str) -> HermesConfig:
        """Load configuration from YAML or JSON file.
        
        Args:
            path: Path to configuration file
            
        Returns:
            HermesConfig instance
        """
        import json
        from pathlib import Path

        path = Path(path)
        if not path.exists():
            raise FileNotFoundError(f"Configuration file not found: {path}")

        if path.suffix == ".json":
            with path.open() as f:
                data = json.load(f)
        elif path.suffix in {".yaml", ".yml"}:
            import yaml

            with path.open() as f:
                data = yaml.safe_load(f)
        else:
            raise ValueError(f"Unsupported file format: {path.suffix}")

        return cls(**data)
