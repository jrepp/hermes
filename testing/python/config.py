"""Configuration for Hermes testing framework."""

from __future__ import annotations

import os
from pathlib import Path
from typing import Literal, Optional

from pydantic import BaseModel, ConfigDict, Field


class TestingConfig(BaseModel):
    """Configuration for Hermes testing environment."""
    
    model_config = ConfigDict(frozen=False)

    # Hermes API configuration
    hermes_base_url: str = Field(
        default_factory=lambda: os.getenv("HERMES_BASE_URL", "http://localhost:8001")
    )
    hermes_auth_token: Optional[str] = Field(
        default_factory=lambda: os.getenv("HERMES_AUTH_TOKEN")
    )

    # Testing environment paths
    testing_dir: Path = Field(
        default_factory=lambda: Path(__file__).parent.parent.resolve()
    )
    workspaces_dir: Path = Field(default=None)  # type: ignore
    fixtures_dir: Path = Field(default=None)  # type: ignore

    # Default values for test scenarios
    default_document_count: int = 10
    default_workspace: Literal["testing", "docs", "all"] = "all"

    # Indexing configuration
    indexer_poll_interval: int = 5  # seconds
    indexer_max_wait: int = 120  # seconds (2 minutes)

    # API configuration
    api_timeout: int = 30  # seconds
    api_max_retries: int = 3

    # Scenario authors
    test_authors: list[str] = Field(
        default_factory=lambda: [
            "alice@example.com",
            "bob@example.com",
            "charlie@example.com",
            "diana@example.com",
        ]
    )

    def __init__(self, **data):  # type: ignore
        """Initialize config with computed paths."""
        super().__init__(**data)
        if self.workspaces_dir is None:
            self.workspaces_dir = self.testing_dir / "workspaces"
        if self.fixtures_dir is None:
            self.fixtures_dir = self.testing_dir / "fixtures"


# Global configuration instance
config = TestingConfig()
