#!/usr/bin/env python3
"""Run basic distributed indexing scenario.

This script:
1. Verifies Hermes is running
2. Seeds test documents
3. Waits for indexing
4. Verifies documents via API
5. Tests search functionality
"""

import sys
from pathlib import Path

# Add parent directory to path for imports
sys.path.insert(0, str(Path(__file__).parent))

from scenarios import runner  # noqa: E402


def main() -> int:
    """Run basic scenario.

    Returns:
        Exit code (0 for success)
    """
    try:
        runner.run_basic_scenario(count=10, clean=True, wait_for_indexing=True)
        return 0
    except Exception as e:
        print(f"\n❌ Scenario failed: {e}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
