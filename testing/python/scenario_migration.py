#!/usr/bin/env python3
"""Run migration scenario with conflict detection.

This script creates documents with same UUID in multiple workspaces
to test migration workflows and conflict detection.
"""

import sys
from pathlib import Path

# Add parent directory to path for imports
sys.path.insert(0, str(Path(__file__).parent))

from scenarios import runner  # noqa: E402


def main() -> int:
    """Run migration scenario.

    Returns:
        Exit code (0 for success)
    """
    try:
        runner.run_migration_scenario(count=5, clean=True)
        return 0
    except Exception as e:
        print(f"\n❌ Scenario failed: {e}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
