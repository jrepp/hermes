#!/usr/bin/env python3
"""Hermes Testing CLI - Unified testing framework entry point.

This CLI replaces the bash scripts in testing/scripts/ with a unified
Python-based testing framework. All functionality from the bash scripts
has been ported to Python modules.

Usage:
    hermes-test seed [options]           # Seed workspaces with test documents
    hermes-test scenario <name> [options]  # Run test scenarios
    hermes-test validate [options]       # Validate Hermes deployment
    hermes-test clean [options]          # Clean test data

Examples:
    # Seed workspaces (replaces seed-workspaces.sh)
    hermes-test seed --scenario basic --count 10 --clean

    # Run basic scenario (replaces scenario-basic.sh)
    hermes-test scenario basic --count 20

    # Run with authentication
    export HERMES_AUTH_TOKEN="<token>"
    hermes-test scenario basic

    # Validate deployment
    hermes-test validate --check-all
"""

from __future__ import annotations

import argparse
import os
import sys
from pathlib import Path

# Add python-client to path
TESTING_ROOT = Path(__file__).parent.parent.parent.parent
sys.path.insert(0, str(TESTING_ROOT / "python-client/src"))
sys.path.insert(0, str(Path(__file__).parent))

from rich.console import Console
from rich.panel import Panel
from rich.table import Table

from scenarios import ScenarioRunner
from seeding import ScenarioType, WorkspaceName, WorkspaceSeeder
from validation import HermesValidator

console = Console()


def cmd_seed(args: argparse.Namespace) -> int:
    """Seed workspaces with test documents.
    
    Replaces: testing/scripts/seed-workspaces.sh
    """
    console.print(
        Panel.fit(
            "[bold blue]Hermes Workspace Seeding[/bold blue]",
            border_style="blue",
        )
    )
    
    # Map CLI args to ScenarioType
    scenario_map = {
        "basic": ScenarioType.BASIC,
        "migration": ScenarioType.MIGRATION,
        "conflict": ScenarioType.CONFLICT,
        "multi-author": ScenarioType.MULTI_AUTHOR,
    }
    
    scenario = scenario_map.get(args.scenario)
    if not scenario:
        console.print(f"[red]❌ Invalid scenario: {args.scenario}[/red]")
        console.print(f"Valid scenarios: {', '.join(scenario_map.keys())}")
        return 1
    
    # Map workspace names - handle "all" specially
    if args.workspace == "all":
        # For migration and conflict scenarios, they handle multiple workspaces internally
        # For basic/multi-author, we'll seed testing workspace
        workspace = WorkspaceName.TESTING
    else:
        workspace_map = {
            "testing": WorkspaceName.TESTING,
            "docs": WorkspaceName.DOCS,
        }
        workspace = workspace_map.get(args.workspace, WorkspaceName.TESTING)
    
    # Create seeder
    seeder = WorkspaceSeeder()
    
    # Show configuration
    console.print(f"[blue]Scenario:[/blue] {args.scenario}")
    console.print(f"[blue]Count:[/blue] {args.count}")
    console.print(f"[blue]Workspace:[/blue] {args.workspace}")
    console.print(f"[blue]Clean:[/blue] {args.clean}")
    console.print()
    
    # Seed based on scenario
    try:
        if scenario == ScenarioType.BASIC:
            files = seeder.seed_basic(count=args.count, clean=args.clean, workspace=workspace)
        elif scenario == ScenarioType.MIGRATION:
            files = seeder.seed_migration(count=args.count, clean=args.clean)
        elif scenario == ScenarioType.CONFLICT:
            files = seeder.seed_conflict(count=args.count, clean=args.clean)
        elif scenario == ScenarioType.MULTI_AUTHOR:
            files = seeder.seed_multi_author(count=args.count, clean=args.clean, workspace=workspace)
        else:
            console.print(f"[red]❌ Scenario not implemented: {args.scenario}[/red]")
            return 1
        
        console.print(f"\n[green]✅ Seeded {len(files)} documents successfully[/green]")
        
        if args.verbose:
            console.print("\n[dim]Generated files:[/dim]")
            for f in files[:10]:  # Show first 10
                console.print(f"  [dim]{f}[/dim]")
            if len(files) > 10:
                console.print(f"  [dim]... and {len(files) - 10} more[/dim]")
        
        return 0
        
    except Exception as e:
        console.print(f"\n[red]❌ Seeding failed: {e}[/red]")
        if args.verbose:
            import traceback
            traceback.print_exc()
        return 1


def cmd_scenario(args: argparse.Namespace) -> int:
    """Run test scenarios.
    
    Replaces: testing/scripts/scenario-basic.sh
    """
    # Map scenario names
    scenario_funcs = {
        "basic": "run_basic_scenario",
        "migration": "run_migration_scenario",
        "multi-author": "run_multi_author_scenario",
    }
    
    if args.name not in scenario_funcs:
        console.print(f"[red]❌ Invalid scenario: {args.name}[/red]")
        console.print(f"Valid scenarios: {', '.join(scenario_funcs.keys())}")
        return 1
    
    # Create runner
    runner = ScenarioRunner()
    
    # Setup token refresh if credentials available
    if args.token_refresh and os.getenv("DEX_TEST_USERNAME"):
        try:
            from auth_helper import get_dex_token_for_testing
            
            runner.validator.set_token_refresh(
                refresh_callback=get_dex_token_for_testing,
                expires_in_seconds=3600,
            )
        except Exception as e:
            console.print(f"[yellow]⚠️  Token refresh setup failed: {e}[/yellow]")
    
    # Run scenario
    try:
        func_name = scenario_funcs[args.name]
        func = getattr(runner, func_name)
        
        results = func(
            count=args.count,
            clean=args.clean,
            wait_for_indexing=args.wait,
        )
        
        console.print(f"\n[green]✅ Scenario '{args.name}' completed[/green]")
        console.print(f"Results: {results}")
        return 0
        
    except Exception as e:
        console.print(f"\n[red]❌ Scenario '{args.name}' failed: {e}[/red]")
        if args.verbose:
            import traceback
            traceback.print_exc()
        return 1


def cmd_validate(args: argparse.Namespace) -> int:
    """Validate Hermes deployment."""
    validator = HermesValidator()
    
    console.print(
        Panel.fit(
            "[bold blue]Hermes Deployment Validation[/bold blue]",
            border_style="blue",
        )
    )
    
    errors = []
    
    # Check health
    console.print("\n[yellow]Checking Hermes health...[/yellow]")
    try:
        validator.assert_healthy()
    except Exception as e:
        errors.append(("Health check", str(e)))
    
    # Check document counts if requested
    if args.check_all or args.check_docs:
        console.print("\n[yellow]Checking document statistics...[/yellow]")
        try:
            stats = validator.get_document_stats()
            
            table = Table(title="Document Statistics")
            table.add_column("Type", style="cyan")
            table.add_column("Count", style="green")
            
            table.add_row("Total", str(stats.get("total", 0)))
            for doc_type, count in sorted(stats.get("by_type", {}).items()):
                table.add_row(f"  {doc_type}", str(count))
            
            console.print(table)
            
        except Exception as e:
            errors.append(("Document stats", str(e)))
    
    # Search validation
    if args.check_all or args.check_search:
        console.print("\n[yellow]Checking search functionality...[/yellow]")
        try:
            results = validator.assert_search_results("test", min_results=0)
            console.print(f"[green]✅ Search working ({results.nb_hits} results)[/green]")
        except Exception as e:
            errors.append(("Search check", str(e)))
    
    # Summary
    console.print()
    if errors:
        console.print("[red]❌ Validation failed with errors:[/red]")
        for check, error in errors:
            console.print(f"  [red]• {check}: {error}[/red]")
        return 1
    else:
        console.print("[green]✅ All validations passed![/green]")
        return 0


def cmd_clean(args: argparse.Namespace) -> int:
    """Clean test data."""
    seeder = WorkspaceSeeder()
    
    console.print(
        Panel.fit(
            "[bold yellow]Cleaning Test Data[/bold yellow]",
            border_style="yellow",
        )
    )
    
    # Map workspace names
    workspace_map = {
        "testing": WorkspaceName.TESTING,
        "docs": WorkspaceName.DOCS,
        "all": WorkspaceName.ALL,
    }
    workspace = workspace_map.get(args.workspace, WorkspaceName.ALL)
    
    console.print(f"[yellow]Cleaning workspace:[/yellow] {args.workspace}")
    
    if args.confirm or console.input("\n[yellow]Are you sure? (y/N):[/yellow] ").lower() == 'y':
        try:
            seeder.clean_workspace(workspace)
            console.print(f"[green]✅ Workspace '{args.workspace}' cleaned[/green]")
            return 0
        except Exception as e:
            console.print(f"[red]❌ Clean failed: {e}[/red]")
            return 1
    else:
        console.print("[blue]Cancelled[/blue]")
        return 0


def main() -> int:
    """Main CLI entry point."""
    parser = argparse.ArgumentParser(
        description="Hermes Testing CLI - Unified testing framework",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  # Seed workspaces
  %(prog)s seed --scenario basic --count 10 --clean
  
  # Run scenario
  %(prog)s scenario basic --count 20 --wait
  
  # Validate deployment
  %(prog)s validate --check-all
  
  # Clean workspaces
  %(prog)s clean --workspace testing

Environment Variables:
  HERMES_BASE_URL          Hermes server URL (default: http://localhost:8001)
  HERMES_AUTH_TOKEN        OAuth bearer token for authentication
  DEX_TEST_USERNAME        Dex username for token refresh
  DEX_TEST_PASSWORD        Dex password for token refresh

For more information, see:
  testing/python/README.md
  testing/python/OAUTH_AUTOMATION_GUIDE.md
        """,
    )
    
    parser.add_argument(
        "-v", "--verbose",
        action="store_true",
        help="Enable verbose output",
    )
    
    subparsers = parser.add_subparsers(dest="command", help="Command to run")
    
    # Seed command
    seed_parser = subparsers.add_parser(
        "seed",
        help="Seed workspaces with test documents",
    )
    seed_parser.add_argument(
        "--scenario",
        choices=["basic", "migration", "conflict", "multi-author"],
        default="basic",
        help="Scenario type (default: basic)",
    )
    seed_parser.add_argument(
        "--count",
        type=int,
        default=10,
        help="Number of documents to generate (default: 10)",
    )
    seed_parser.add_argument(
        "--workspace",
        choices=["testing", "docs", "all"],
        default="all",
        help="Target workspace (default: all)",
    )
    seed_parser.add_argument(
        "--clean",
        action="store_true",
        help="Clean workspace before seeding",
    )
    
    # Scenario command
    scenario_parser = subparsers.add_parser(
        "scenario",
        help="Run test scenarios",
    )
    scenario_parser.add_argument(
        "name",
        choices=["basic", "migration", "multi-author"],
        help="Scenario name",
    )
    scenario_parser.add_argument(
        "--count",
        type=int,
        default=10,
        help="Number of documents (default: 10)",
    )
    scenario_parser.add_argument(
        "--clean",
        action="store_true",
        help="Clean workspace before running",
    )
    scenario_parser.add_argument(
        "--wait",
        action="store_true",
        help="Wait for indexing to complete",
    )
    scenario_parser.add_argument(
        "--token-refresh",
        action="store_true",
        help="Enable automatic token refresh",
    )
    
    # Validate command
    validate_parser = subparsers.add_parser(
        "validate",
        help="Validate Hermes deployment",
    )
    validate_parser.add_argument(
        "--check-all",
        action="store_true",
        help="Run all validation checks",
    )
    validate_parser.add_argument(
        "--check-docs",
        action="store_true",
        help="Check document statistics",
    )
    validate_parser.add_argument(
        "--check-search",
        action="store_true",
        help="Check search functionality",
    )
    
    # Clean command
    clean_parser = subparsers.add_parser(
        "clean",
        help="Clean test data",
    )
    clean_parser.add_argument(
        "--workspace",
        choices=["testing", "docs", "all"],
        default="all",
        help="Workspace to clean (default: all)",
    )
    clean_parser.add_argument(
        "-y", "--confirm",
        action="store_true",
        help="Skip confirmation prompt",
    )
    
    args = parser.parse_args()
    
    if not args.command:
        parser.print_help()
        return 1
    
    # Route to command handler
    commands = {
        "seed": cmd_seed,
        "scenario": cmd_scenario,
        "validate": cmd_validate,
        "clean": cmd_clean,
    }
    
    handler = commands.get(args.command)
    if handler:
        return handler(args)
    else:
        console.print(f"[red]Unknown command: {args.command}[/red]")
        return 1


if __name__ == "__main__":
    sys.exit(main())
