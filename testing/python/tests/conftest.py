"""Pytest fixtures for Hermes distributed testing."""

import asyncio
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).parent.parent))

import pytest

from config import config
from generators import DocumentGenerator
from scenarios import ScenarioRunner
from seeding import WorkspaceSeeder
from validation import HermesValidator


@pytest.fixture(scope="session")
def event_loop_policy():
    """Set event loop policy for the session."""
    policy = asyncio.get_event_loop_policy()
    return policy


@pytest.fixture(scope="function")
def event_loop(event_loop_policy):
    """Create a new event loop for each test."""
    loop = event_loop_policy.new_event_loop()
    asyncio.set_event_loop(loop)
    yield loop
    # Clean up but don't close immediately - let pending operations finish
    try:
        loop.run_until_complete(asyncio.sleep(0))
        pending = asyncio.all_tasks(loop)
        for task in pending:
            task.cancel()
        loop.run_until_complete(asyncio.gather(*pending, return_exceptions=True))
    finally:
        loop.close()


@pytest.fixture(scope="session")
def testing_dir() -> Path:
    """Get testing directory path."""
    return config.testing_dir


@pytest.fixture(scope="session")
def workspaces_dir() -> Path:
    """Get workspaces directory path."""
    return config.workspaces_dir


@pytest.fixture
def generator() -> DocumentGenerator:
    """Get document generator instance."""
    return DocumentGenerator()


@pytest.fixture
def seeder() -> WorkspaceSeeder:
    """Get workspace seeder instance."""
    return WorkspaceSeeder()


@pytest.fixture(scope="session")
def validator() -> HermesValidator:
    """Get Hermes validator instance."""
    return HermesValidator()


@pytest.fixture
def runner(seeder: WorkspaceSeeder, validator: HermesValidator) -> ScenarioRunner:
    """Get scenario runner instance."""
    return ScenarioRunner(seeder=seeder, validator=validator)


@pytest.fixture(scope="session", autouse=True)
def check_hermes_running(validator: HermesValidator) -> None:
    """Check Hermes is running before tests (session-scoped).

    Raises:
        pytest.skip: If Hermes is not running
    """
    if not validator.check_health():
        pytest.skip(
            f"Hermes is not running at {config.hermes_base_url}. "
            "Start with: cd testing && make up"
        )
