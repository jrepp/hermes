"""Integration tests for distributed scenarios.

These tests require a running Hermes instance.
"""

import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).parent.parent))

import pytest

from scenarios import ScenarioRunner


@pytest.mark.integration
@pytest.mark.basic
class TestBasicScenario:
    """Test basic distributed indexing scenario."""

    @pytest.mark.slow
    def test_basic_scenario(self, runner: ScenarioRunner) -> None:
        """Run complete basic scenario with indexing wait."""
        stats = runner.run_basic_scenario(count=5, clean=True, wait_for_indexing=True)

        assert stats["total"] >= 5
        assert len(stats.get("by_type", {})) > 0

    def test_basic_scenario_no_wait(self, runner: ScenarioRunner) -> None:
        """Run basic scenario without waiting for indexing."""
        stats = runner.run_basic_scenario(count=3, clean=True, wait_for_indexing=False)

        # Stats might be 0 if indexing hasn't happened yet
        assert isinstance(stats, dict)
        assert "total" in stats


@pytest.mark.integration
@pytest.mark.migration
class TestMigrationScenario:
    """Test migration scenario with conflict detection."""

    @pytest.mark.slow
    def test_migration_scenario(self, runner: ScenarioRunner) -> None:
        """Run migration scenario."""
        stats = runner.run_migration_scenario(count=3, clean=True)

        assert isinstance(stats, dict)
        assert "total" in stats


@pytest.mark.integration
@pytest.mark.multi_author
class TestMultiAuthorScenario:
    """Test multi-author collaboration scenario."""

    @pytest.mark.slow
    def test_multi_author_scenario(self, runner: ScenarioRunner) -> None:
        """Run multi-author scenario."""
        stats = runner.run_multi_author_scenario(count=5, clean=True)

        assert isinstance(stats, dict)
        assert "total" in stats
        assert len(stats.get("by_type", {})) > 0
