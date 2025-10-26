#!/usr/bin/env python3
"""Long-running scenario demonstrating token refresh.

This scenario shows how to handle authentication tokens that expire
during long-running test operations.

Usage:
    # With token refresh from environment
    export DEX_TEST_USERNAME="test@hermes.local"
    export DEX_TEST_PASSWORD="password"
    python3 scenario_long_running.py

    # Or manually set token (will warn when it expires)
    export HERMES_AUTH_TOKEN="<your-token>"
    python3 scenario_long_running.py
"""

from __future__ import annotations

import os
import sys
import time
from pathlib import Path

# Add paths for imports
sys.path.insert(0, str(Path(__file__).parent.parent.parent / "python-client/src"))
sys.path.insert(0, str(Path(__file__).parent))

from rich.console import Console
from rich.panel import Panel

from scenarios import ScenarioRunner
from validation import HermesValidator

console = Console()


def setup_token_refresh(validator: HermesValidator) -> bool:
    """Setup automatic token refresh if credentials are available.

    Args:
        validator: Validator instance to configure

    Returns:
        True if refresh configured, False otherwise
    """
    # Check if we have credentials for token refresh
    if os.getenv("DEX_TEST_USERNAME") and os.getenv("DEX_TEST_PASSWORD"):
        try:
            from auth_helper import get_dex_token_for_testing

            # Create refresh callback
            def refresh_token() -> str:
                return get_dex_token_for_testing(
                    username=os.getenv("DEX_TEST_USERNAME"),
                    password=os.getenv("DEX_TEST_PASSWORD"),
                )

            # Configure validator to refresh token every hour
            validator.set_token_refresh(
                refresh_callback=refresh_token,
                expires_in_seconds=3600,  # 1 hour
            )
            console.print("✓ Automatic token refresh enabled", style="green")
            return True

        except Exception as e:
            console.print(f"⚠️  Failed to setup token refresh: {e}", style="yellow")
            return False

    elif os.getenv("GOOGLE_APPLICATION_CREDENTIALS"):
        try:
            from auth_helper import get_google_service_account_token

            # Create refresh callback for Google
            def refresh_token() -> str:
                return get_google_service_account_token(
                    subject=os.getenv("GOOGLE_WORKSPACE_ADMIN_EMAIL"),
                )

            validator.set_token_refresh(
                refresh_callback=refresh_token,
                expires_in_seconds=3600,
            )
            console.print("✓ Automatic token refresh enabled (Google)", style="green")
            return True

        except Exception as e:
            console.print(f"⚠️  Failed to setup Google token refresh: {e}", style="yellow")
            return False

    else:
        console.print(
            "ℹ️  No refresh credentials configured - token will not auto-refresh",
            style="blue",
        )
        console.print(
            "   Set DEX_TEST_USERNAME/DEX_TEST_PASSWORD or "
            "GOOGLE_APPLICATION_CREDENTIALS to enable",
            style="dim",
        )
        return False


def main() -> int:
    """Run long-running test scenario with token refresh.

    Returns:
        Exit code (0 = success, 1 = failure)
    """
    console.print(
        Panel.fit(
            "[bold blue]Long-Running Test Scenario with Token Refresh[/bold blue]",
            border_style="blue",
        )
    )

    # Create runner and validator
    runner = ScenarioRunner()
    validator = runner.validator

    # Setup token refresh
    setup_token_refresh(validator)

    try:
        # Phase 1: Seed many documents
        console.print("\n[bold]Phase 1: Seeding documents (batch 1)...[/bold]")
        results_1 = runner.run_basic_scenario(count=50, clean=True, wait_for_indexing=False)
        console.print(f"✓ Batch 1 complete: {results_1}")

        # Simulate long wait (in real scenario, this would be indexing time)
        console.print("\n[bold]Simulating long operation (30 seconds)...[/bold]")
        for i in range(6):
            time.sleep(5)
            console.print(f"  {(i+1)*5}s elapsed...", style="dim")

        # Phase 2: Seed more documents (token might have expired)
        console.print("\n[bold]Phase 2: Seeding documents (batch 2)...[/bold]")
        results_2 = runner.run_basic_scenario(count=50, clean=False, wait_for_indexing=False)
        console.print(f"✓ Batch 2 complete: {results_2}")

        # Phase 3: Validate (this will trigger token refresh if needed)
        console.print("\n[bold]Phase 3: Validation...[/bold]")
        console.print("Token refresh will occur automatically if needed")
        
        # Try to get stats (may fail if auth not configured)
        try:
            stats = validator.get_document_stats()
            console.print(f"✓ Document stats: {stats.get('total')} total documents")
        except Exception as e:
            console.print(f"⚠️  Validation skipped (auth required): {e}", style="yellow")

        console.print("\n[bold green]✅ Long-running scenario completed![/bold green]")
        return 0

    except Exception as e:
        console.print(f"\n[bold red]❌ Scenario failed: {e}[/bold red]")
        return 1


if __name__ == "__main__":
    sys.exit(main())
