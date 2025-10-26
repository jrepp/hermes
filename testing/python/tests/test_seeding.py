"""Tests for workspace seeding functionality."""

import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).parent.parent))


from seeding import ScenarioType, WorkspaceName, WorkspaceSeeder


class TestWorkspaceSeeder:
    """Test workspace seeding functionality."""

    def test_seed_basic(self, seeder: WorkspaceSeeder, workspaces_dir: Path) -> None:
        """Test basic seeding."""
        files = seeder.seed_basic(count=3, clean=True)

        assert len(files) == 3
        assert all(f.exists() for f in files)
        assert all(f.suffix == ".md" for f in files)

        # Check files in correct directories
        file_dirs = {f.parent.name for f in files}
        assert file_dirs.intersection({"rfcs", "prds", "meetings"})

    def test_seed_migration(self, seeder: WorkspaceSeeder) -> None:
        """Test migration seeding."""
        source_files, target_files = seeder.seed_migration(count=2, clean=True)

        assert len(source_files) == 2
        assert len(target_files) == 2
        assert all(f.exists() for f in source_files)
        assert all(f.exists() for f in target_files)

    def test_seed_multi_author(self, seeder: WorkspaceSeeder) -> None:
        """Test multi-author seeding."""
        files = seeder.seed_multi_author(count=3, clean=True)

        assert len(files) == 3
        assert all(f.exists() for f in files)

    def test_clean_workspace(
        self, seeder: WorkspaceSeeder, workspaces_dir: Path
    ) -> None:
        """Test workspace cleaning."""
        # Seed some files
        seeder.seed_basic(count=2, clean=False)

        # Clean
        seeder.clean_workspace(WorkspaceName.TESTING)

        # Verify no markdown files remain
        testing_dir = workspaces_dir / "testing"
        md_files = list(testing_dir.rglob("*.md"))
        # Only README should remain (if it exists)
        assert all("README" in f.name for f in md_files)

    def test_seed_scenario_basic(self, seeder: WorkspaceSeeder) -> None:
        """Test seeding via scenario enum."""
        result = seeder.seed_scenario(ScenarioType.BASIC, count=3, clean=True)

        assert isinstance(result, list)
        assert len(result) == 3

    def test_seed_scenario_migration(self, seeder: WorkspaceSeeder) -> None:
        """Test seeding migration scenario via enum."""
        result = seeder.seed_scenario(ScenarioType.MIGRATION, count=2, clean=True)

        assert isinstance(result, tuple)
        assert len(result) == 2  # source and target lists
