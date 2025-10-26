"""Workspace seeding utilities for Hermes testing.

Manages test document creation, workspace organization, and data lifecycle.
"""

from __future__ import annotations

from enum import Enum
from pathlib import Path
from typing import Literal

from rich.console import Console
from rich.progress import Progress, SpinnerColumn, TextColumn

from config import config
from generators import DocumentGenerator, DocumentStatus

console = Console()


class WorkspaceName(str, Enum):
    """Available workspace names."""

    TESTING = "testing"
    DOCS = "docs"


class ScenarioType(str, Enum):
    """Test scenario types."""

    BASIC = "basic"
    MIGRATION = "migration"
    CONFLICT = "conflict"
    MULTI_AUTHOR = "multi_author"


class WorkspaceSeeder:
    """Seed workspaces with test documents."""

    def __init__(
        self,
        workspaces_dir: Path | None = None,
        generator: DocumentGenerator | None = None,
    ) -> None:
        """Initialize workspace seeder.

        Args:
            workspaces_dir: Root directory for workspaces
            generator: Document generator instance
        """
        self.workspaces_dir = workspaces_dir or config.workspaces_dir
        self.generator = generator or DocumentGenerator()
        self.console = console

    def _ensure_workspace_structure(self, workspace: WorkspaceName) -> None:
        """Create workspace directory structure.

        Args:
            workspace: Workspace name
        """
        base = self.workspaces_dir / workspace.value
        for subdir in ["rfcs", "prds", "meetings", "drafts", "docs"]:
            (base / subdir).mkdir(parents=True, exist_ok=True)

    def clean_workspace(
        self, workspace: WorkspaceName | Literal["all"] = "all"
    ) -> None:
        """Remove all documents from workspace.

        Args:
            workspace: Workspace to clean or 'all'
        """
        workspaces = (
            [WorkspaceName.TESTING, WorkspaceName.DOCS]
            if workspace == "all"
            else [WorkspaceName(workspace)]
        )

        for ws in workspaces:
            ws_path = self.workspaces_dir / ws.value
            if ws_path.exists():
                # Remove all files but keep directory structure
                for item in ws_path.rglob("*"):
                    if item.is_file():
                        item.unlink()
                self.console.print(f"✓ Cleaned {ws.value} workspace", style="green")

    def seed_basic(
        self,
        count: int = 10,
        workspace: WorkspaceName = WorkspaceName.TESTING,
        clean: bool = False,
    ) -> list[Path]:
        """Seed workspace with basic test documents.

        Args:
            count: Total number of documents to generate
            workspace: Target workspace
            clean: Clean workspace before seeding

        Returns:
            List of created file paths
        """
        if clean:
            self.clean_workspace(workspace)

        self._ensure_workspace_structure(workspace)
        created_files: list[Path] = []

        # Distribute document types
        rfc_count = count // 3
        prd_count = count // 3
        meeting_count = count - rfc_count - prd_count

        base = self.workspaces_dir / workspace.value

        with Progress(
            SpinnerColumn(),
            TextColumn("[progress.description]{task.description}"),
            console=self.console,
        ) as progress:
            # Generate RFCs
            task = progress.add_task(f"Generating {rfc_count} RFCs...", total=rfc_count)
            for i in range(1, rfc_count + 1):
                content = self.generator.generate_rfc(
                    number=i,
                    title=f"RFC-{i:03d}: Test Distributed System Design",
                    status=DocumentStatus.DRAFT,
                    author="alice@example.com",
                )
                filepath = base / "rfcs" / f"RFC-{i:03d}-test-design.md"
                filepath.write_text(content)
                created_files.append(filepath)
                progress.advance(task)

            # Generate PRDs
            task = progress.add_task(f"Generating {prd_count} PRDs...", total=prd_count)
            for i in range(1, prd_count + 1):
                content = self.generator.generate_prd(
                    number=i,
                    title=f"PRD-{i:03d}: Test Feature Requirements",
                    status=DocumentStatus.DRAFT,
                    author="bob@example.com",
                )
                filepath = base / "prds" / f"PRD-{i:03d}-test-feature.md"
                filepath.write_text(content)
                created_files.append(filepath)
                progress.advance(task)

            # Generate Meeting Notes
            task = progress.add_task(
                f"Generating {meeting_count} meeting notes...", total=meeting_count
            )
            for i in range(1, meeting_count + 1):
                content = self.generator.generate_meeting_notes(
                    number=i,
                    title=f"Meeting-{i:03d}: Test Team Sync",
                    attendees=["alice@example.com", "bob@example.com"],
                )
                filepath = base / "meetings" / f"MEET-{i:03d}-team-sync.md"
                filepath.write_text(content)
                created_files.append(filepath)
                progress.advance(task)

        # Create README
        readme_content = f"""# {workspace.value.title()} Workspace

Generated by Hermes distributed testing framework.

**Scenario**: Basic
**Document Count**: {count}
**Generated**: {self.generator.generate_timestamp()}

## Contents

- RFCs: {rfc_count} documents in `rfcs/`
- PRDs: {prd_count} documents in `prds/`
- Meetings: {meeting_count} documents in `meetings/`

## Usage

These documents are automatically indexed by the Hermes indexer and
made available through search.

To verify indexing:
```bash
# Check document count
curl http://localhost:8001/api/v2/documents | jq '.total'

# Search for documents
curl "http://localhost:8001/api/v2/search?q=test" | jq '.hits | length'
```

## Generated Files

```
{chr(10).join(f"- {f.relative_to(base)}" for f in created_files)}
```
"""
        readme_path = base / "README.md"
        readme_path.write_text(readme_content)

        self.console.print(
            f"\n✓ Generated {count} documents in {workspace.value} workspace",
            style="bold green",
        )
        return created_files

    def seed_migration(
        self,
        count: int = 5,
        source_workspace: WorkspaceName = WorkspaceName.DOCS,
        target_workspace: WorkspaceName = WorkspaceName.TESTING,
        clean: bool = False,
    ) -> tuple[list[Path], list[Path]]:
        """Seed workspaces for migration scenario.

        Creates documents with same UUID in both workspaces but slightly
        different content to simulate migration and test conflict detection.

        Args:
            count: Number of documents to create
            source_workspace: Source workspace for migration
            target_workspace: Target workspace for migration
            clean: Clean workspaces before seeding

        Returns:
            Tuple of (source_files, target_files)
        """
        if clean:
            self.clean_workspace("all")

        self._ensure_workspace_structure(source_workspace)
        self._ensure_workspace_structure(target_workspace)

        source_base = self.workspaces_dir / source_workspace.value
        target_base = self.workspaces_dir / target_workspace.value

        source_files: list[Path] = []
        target_files: list[Path] = []

        with Progress(
            SpinnerColumn(),
            TextColumn("[progress.description]{task.description}"),
            console=self.console,
        ) as progress:
            task = progress.add_task(f"Generating {count} migration docs...", total=count)

            for i in range(1, count + 1):
                # Use same UUID for both copies
                doc_uuid = self.generator.generate_uuid()

                # Create in source workspace
                source_content = self.generator.generate_rfc(
                    number=i,
                    doc_uuid=doc_uuid,
                    title=f"RFC-{i:03d}: Migration Test Document",
                    status=DocumentStatus.APPROVED,
                    author="alice@example.com",
                )
                source_path = source_base / "rfcs" / f"RFC-{i:03d}-migration.md"
                source_path.write_text(source_content)
                source_files.append(source_path)

                # Create in target workspace with slight modification
                target_content = self.generator.generate_rfc(
                    number=i,
                    doc_uuid=doc_uuid,  # Same UUID
                    title=f"RFC-{i:03d}: Migration Test Document (Modified)",
                    status=DocumentStatus.IN_REVIEW,  # Different status
                    author="bob@example.com",  # Different author
                )
                target_path = target_base / "rfcs" / f"RFC-{i:03d}-migration.md"
                target_path.write_text(target_content)
                target_files.append(target_path)

                progress.advance(task)

        self.console.print(
            f"\n✓ Created {count} documents in each workspace with matching UUIDs",
            style="bold green",
        )
        self.console.print(
            "  Use this scenario to test migration conflict detection",
            style="yellow",
        )

        return source_files, target_files

    def seed_multi_author(
        self,
        count: int = 10,
        workspace: WorkspaceName = WorkspaceName.TESTING,
        clean: bool = False,
    ) -> list[Path]:
        """Seed workspace with multi-author documents.

        Creates documents from different authors with staggered timestamps
        and various statuses.

        Args:
            count: Total number of documents to generate
            workspace: Target workspace
            clean: Clean workspace before seeding

        Returns:
            List of created file paths
        """
        if clean:
            self.clean_workspace(workspace)

        self._ensure_workspace_structure(workspace)
        created_files: list[Path] = []
        base = self.workspaces_dir / workspace.value

        authors = config.test_authors
        statuses = list(DocumentStatus)

        with Progress(
            SpinnerColumn(),
            TextColumn("[progress.description]{task.description}"),
            console=self.console,
        ) as progress:
            task = progress.add_task(
                f"Generating {count} multi-author docs...", total=count
            )

            for i in range(1, count + 1):
                # Rotate through authors and statuses
                author = authors[i % len(authors)]
                status = statuses[i % len(statuses)]

                # Stagger timestamps (1 day apart)
                offset_days = -(count - i)
                created_time = self.generator.generate_timestamp(offset_days)

                # Vary document types
                if i % 3 == 0:
                    content = self.generator.generate_rfc(
                        number=i,
                        title=f"RFC-{i:03d}: Multi-Author Test",
                        status=status,
                        author=author,
                        created=created_time,
                    )
                    filepath = base / "rfcs" / f"RFC-{i:03d}-multi-author.md"
                elif i % 3 == 1:
                    content = self.generator.generate_prd(
                        number=i,
                        title=f"PRD-{i:03d}: Multi-Author Feature",
                        status=status,
                        author=author,
                        created=created_time,
                    )
                    filepath = base / "prds" / f"PRD-{i:03d}-multi-author.md"
                else:
                    content = self.generator.generate_meeting_notes(
                        number=i,
                        title=f"Meeting-{i:03d}: Multi-Author Sync",
                        attendees=[author, authors[(i + 1) % len(authors)]],
                        created=created_time,
                    )
                    filepath = base / "meetings" / f"MEET-{i:03d}-multi-author.md"

                filepath.write_text(content)
                created_files.append(filepath)
                progress.advance(task)

        self.console.print(
            f"\n✓ Generated {count} documents from {len(authors)} authors",
            style="bold green",
        )

        return created_files

    def seed_scenario(
        self,
        scenario: ScenarioType,
        count: int | None = None,
        clean: bool = False,
    ) -> list[Path] | tuple[list[Path], list[Path]]:
        """Seed workspace with specified scenario.

        Args:
            scenario: Scenario type to generate
            count: Number of documents (uses default if None)
            clean: Clean workspace before seeding

        Returns:
            List of created file paths (or tuple for migration scenario)
        """
        count = count or config.default_document_count

        if scenario == ScenarioType.BASIC:
            return self.seed_basic(count=count, clean=clean)
        elif scenario == ScenarioType.MIGRATION:
            return self.seed_migration(count=count, clean=clean)
        elif scenario == ScenarioType.MULTI_AUTHOR:
            return self.seed_multi_author(count=count, clean=clean)
        else:
            raise ValueError(f"Unsupported scenario: {scenario}")


# Convenience instance
seeder = WorkspaceSeeder()
