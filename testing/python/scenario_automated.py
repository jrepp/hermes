#!/usr/bin/env python3
"""Fully automated test scenario with environment-based authentication.

This scenario demonstrates running tests with authentication tokens from
environment variables, bypassing interactive login flows.

Usage:
    # Option 1: Set token manually (from browser)
    export HERMES_AUTH_TOKEN="<token-from-browser-devtools>"
    python3 scenario_automated.py

    # Option 2: Use skip_auth mode (development only)
    # Configure server with: server { skip_auth = true }
    python3 scenario_automated.py

    # Option 3: Use Google service account (production)
    export GOOGLE_APPLICATION_CREDENTIALS="/path/to/service-account.json"
    export GOOGLE_WORKSPACE_ADMIN_EMAIL="admin@example.com"
    python3 scenario_automated.py --google-auth

For more details on authentication setup, see:
    testing/python/OAUTH_AUTOMATION_GUIDE.md
"""

from __future__ import annotations

import os
import sys
from pathlib import Path

# Add paths for imports
sys.path.insert(0, str(Path(__file__).parent.parent.parent / "python-client/src"))
sys.path.insert(0, str(Path(__file__).parent))

from rich.console import Console

from scenarios import ScenarioRunner

console = Console()


def get_token_from_browser_instructions() -> None:
    """Print instructions for getting token from browser."""
    console.print("\n[bold yellow]To get authentication token from browser:[/bold yellow]")
    console.print("1. Open browser to: http://localhost:4201")
    console.print("2. Sign in with test@hermes.local / password")
    console.print("3. Open DevTools (F12) → Application → Local Storage")
    console.print("4. Find 'hermes.authToken' or 'token' key")
    console.print("5. Copy the token value")
    console.print("6. Run: export HERMES_AUTH_TOKEN='<paste-token-here>'")
    console.print("7. Re-run this script\n")


def setup_authentication(use_google_auth: bool = False) -> bool:
    """Setup authentication token.

    Args:
        use_google_auth: Use Google service account if True

    Returns:
        True if authentication is configured, False otherwise
    """
    # Check if token already in environment
    if os.getenv("HERMES_AUTH_TOKEN"):
        console.print("✓ Using HERMES_AUTH_TOKEN from environment", style="green")
        return True

    # Try Google service account
    if use_google_auth:
        try:
            from auth_helper import get_google_service_account_token

            token = get_google_service_account_token()
            os.environ["HERMES_AUTH_TOKEN"] = token
            console.print("✓ Obtained token from Google service account", style="green")
            return True
        except Exception as e:
            console.print(f"❌ Failed to get Google token: {e}", style="red")
            return False

    # No authentication available
    console.print("⚠️  No authentication token configured", style="yellow")
    console.print("\nOptions:")
    console.print("  1. Use environment variable (see instructions below)")
    console.print("  2. Configure server with skip_auth = true (dev only)")
    console.print("  3. Use Google service account (--google-auth flag)")

    get_token_from_browser_instructions()
    return False


def main() -> int:
    """Run automated test scenario.

    Returns:
        Exit code (0 = success, 1 = failure)
    """
    console.print(
        "[bold blue]🤖 Automated Test Scenario[/bold blue]\n",
    )

    # Parse simple arguments
    use_google_auth = "--google-auth" in sys.argv
    skip_auth_check = "--skip-auth-check" in sys.argv

    # Setup authentication
    if not skip_auth_check:
        if not setup_authentication(use_google_auth):
            console.print(
                "\n[yellow]Tip: If server has skip_auth=true, "
                "use --skip-auth-check flag[/yellow]"
            )
            return 1

    # Create scenario runner
    runner = ScenarioRunner()

    # Run scenario
    try:
        console.print("\n[bold]Running basic scenario...[/bold]\n")
        results = runner.run_basic_scenario(
            count=20,
            clean=True,
            wait_for_indexing=False,  # Don't wait - indexer runs every 5 min
        )

        console.print("\n[bold green]✅ Scenario completed successfully![/bold green]")
        console.print(f"Results: {results}")
        return 0

    except Exception as e:
        console.print(f"\n[bold red]❌ Scenario failed: {e}[/bold red]")

        # Check if auth error
        if "Unauthorized" in str(e) or "Authentication" in str(e):
            console.print("\n[yellow]This appears to be an authentication error.[/yellow]")
            get_token_from_browser_instructions()

        return 1


if __name__ == "__main__":
    sys.exit(main())
