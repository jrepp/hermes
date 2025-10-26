"""Scenario orchestration for distributed testing.

Coordinates test scenarios including seeding, validation, and reporting.
"""

from __future__ import annotations

from rich.console import Console
from rich.panel import Panel
from rich.table import Table

from config import config
from seeding import WorkspaceSeeder
from validation import HermesValidator

console = Console()


class ScenarioRunner:
    """Run and validate test scenarios."""

    def __init__(
        self,
        seeder: WorkspaceSeeder | None = None,
        validator: HermesValidator | None = None,
    ) -> None:
        """Initialize scenario runner.

        Args:
            seeder: Workspace seeder instance
            validator: Hermes validator instance
        """
        self.seeder = seeder or WorkspaceSeeder()
        self.validator = validator or HermesValidator()
        self.console = console

    def run_basic_scenario(
        self,
        count: int = 10,
        clean: bool = True,
        wait_for_indexing: bool = True,
    ) -> dict[str, int]:
        """Run basic distributed indexing scenario.

        Steps:
        1. Verify Hermes is running
        2. Seed test documents
        3. Wait for indexing
        4. Validate documents via API
        5. Test search functionality

        Args:
            count: Number of documents to generate
            clean: Clean workspace before seeding
            wait_for_indexing: Wait for documents to be indexed

        Returns:
            Dictionary with result counts
        """
        self.console.print(
            Panel.fit(
                "[bold blue]Basic Distributed Indexing Scenario[/bold blue]",
                border_style="blue",
            )
        )

        # Step 1: Health check
        self.console.print("\n[yellow][1/5] Verifying Hermes is running...[/yellow]")
        self.validator.assert_healthy()

        # Step 2: Seed documents
        self.console.print(f"\n[yellow][2/5] Seeding {count} test documents...[/yellow]")
        files = self.seeder.seed_basic(count=count, clean=clean)
        self.console.print(f"✓ Created {len(files)} files", style="green")

        # Step 3: Wait for indexing
        if wait_for_indexing:
            self.console.print(
                f"\n[yellow][3/5] Waiting for indexer...[/yellow]\n"
                f"[blue](Checking every {config.indexer_poll_interval}s, "
                f"max {config.indexer_max_wait}s)[/blue]"
            )
            try:
                self.validator.wait_for_indexing(count)
                self.console.print("✓ All documents indexed!", style="green")
            except Exception as e:
                self.console.print(
                    f"⚠️  Indexing incomplete: {e}\n"
                    f"Note: Indexer scans every 5 minutes. You may need to wait longer.",
                    style="yellow",
                )
        else:
            self.console.print("\n[yellow][3/5] Skipping indexer wait[/yellow]")

        # Step 4: Get statistics
        self.console.print("\n[yellow][4/5] Getting document statistics...[/yellow]")
        stats = self.validator.get_document_stats()

        # Step 5: Test search
        self.console.print("\n[yellow][5/5] Testing search functionality...[/yellow]")
        search_results = {}
        for query in ["test", "RFC", "distributed", "framework"]:
            results = self.validator.assert_search_results(query, min_results=0)
            search_results[query] = len(results.hits)

        # Print summary
        self._print_summary(stats, search_results)

        return stats

    def run_migration_scenario(
        self,
        count: int = 5,
        clean: bool = True,
    ) -> dict[str, int]:
        """Run migration scenario with conflict detection.

        Creates documents with same UUID in multiple workspaces to test
        migration workflows and conflict detection.

        Args:
            count: Number of documents to create per workspace
            clean: Clean workspaces before seeding

        Returns:
            Dictionary with result counts
        """
        self.console.print(
            Panel.fit(
                "[bold blue]Migration & Conflict Detection Scenario[/bold blue]",
                border_style="blue",
            )
        )

        # Step 1: Health check
        self.console.print("\n[yellow][1/4] Verifying Hermes is running...[/yellow]")
        self.validator.assert_healthy()

        # Step 2: Seed migration documents
        self.console.print(
            f"\n[yellow][2/4] Creating {count} documents per workspace...[/yellow]"
        )
        source_files, target_files = self.seeder.seed_migration(count=count, clean=clean)
        self.console.print(
            f"✓ Created {len(source_files)} source + {len(target_files)} target files",
            style="green",
        )

        # Step 3: Wait for indexing
        expected_total = len(source_files) + len(target_files)
        self.console.print(
            f"\n[yellow][3/4] Waiting for {expected_total} documents to index...[/yellow]"
        )
        try:
            self.validator.wait_for_indexing(expected_total)
        except Exception as e:
            self.console.print(f"⚠️  Indexing incomplete: {e}", style="yellow")

        # Step 4: Verify migration
        self.console.print("\n[yellow][4/4] Checking for duplicates/conflicts...[/yellow]")
        stats = self.validator.get_document_stats()

        # Note: Full conflict detection requires backend support
        self.console.print(
            "\n[blue]Note: Full conflict detection requires document_revisions "
            "table and content hash tracking in backend.[/blue]"
        )

        self._print_summary(stats, {})

        return stats

    def run_multi_author_scenario(
        self,
        count: int = 10,
        clean: bool = True,
    ) -> dict[str, int]:
        """Run multi-author scenario.

        Creates documents from different authors with staggered timestamps.

        Args:
            count: Number of documents to generate
            clean: Clean workspace before seeding

        Returns:
            Dictionary with result counts
        """
        self.console.print(
            Panel.fit(
                "[bold blue]Multi-Author Collaboration Scenario[/bold blue]",
                border_style="blue",
            )
        )

        # Step 1: Health check
        self.console.print("\n[yellow][1/4] Verifying Hermes is running...[/yellow]")
        self.validator.assert_healthy()

        # Step 2: Seed documents
        self.console.print(
            f"\n[yellow][2/4] Creating {count} multi-author documents...[/yellow]"
        )
        files = self.seeder.seed_multi_author(count=count, clean=clean)
        self.console.print(f"✓ Created {len(files)} files", style="green")

        # Step 3: Wait for indexing
        self.console.print("\n[yellow][3/4] Waiting for indexing...[/yellow]")
        try:
            self.validator.wait_for_indexing(count)
        except Exception as e:
            self.console.print(f"⚠️  Indexing incomplete: {e}", style="yellow")

        # Step 4: Get statistics
        self.console.print("\n[yellow][4/4] Getting document statistics...[/yellow]")
        stats = self.validator.get_document_stats()

        self._print_summary(stats, {})

        return stats

    def _print_summary(
        self,
        stats: dict[str, int | dict[str, int]],
        search_results: dict[str, int],
    ) -> None:
        """Print scenario summary.

        Args:
            stats: Document statistics
            search_results: Search result counts by query
        """
        self.console.print("\n" + "=" * 60, style="green")
        self.console.print("[bold green]Scenario Complete[/bold green]")
        self.console.print("=" * 60, style="green")

        # Document stats table
        table = Table(title="Document Statistics", show_header=True, header_style="bold")
        table.add_column("Category", style="cyan")
        table.add_column("Count", justify="right", style="green")

        table.add_row("Total Documents", str(stats.get("total", 0)))

        if "by_type" in stats and isinstance(stats["by_type"], dict):
            for doc_type, count in stats["by_type"].items():
                table.add_row(f"  {doc_type}", str(count))

        self.console.print("\n", table)

        # Search results if available
        if search_results:
            search_table = Table(
                title="Search Results", show_header=True, header_style="bold"
            )
            search_table.add_column("Query", style="cyan")
            search_table.add_column("Results", justify="right", style="green")

            for query, count in search_results.items():
                search_table.add_row(query, str(count))

            self.console.print("\n", search_table)

        # Next steps
        self.console.print("\n[bold yellow]Next Steps:[/bold yellow]")
        self.console.print(
            "  • Open web UI: [blue]http://localhost:4201[/blue]"
        )
        self.console.print(
            f"  • API docs: [blue]{config.hermes_base_url}/api/v2/documents[/blue]"
        )
        self.console.print(
            "  • Check logs: [blue]cd testing && docker compose logs hermes-indexer[/blue]"
        )
        self.console.print()


# Convenience instance
runner = ScenarioRunner()
