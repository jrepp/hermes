#!/usr/bin/env python3
"""Run multi-author collaboration scenario.

This script creates documents from different authors with staggered
timestamps to test multi-author workflows.
"""

import sys
from pathlib import Path

# Add parent directory to path for imports
sys.path.insert(0, str(Path(__file__).parent))

from scenarios import runner  # noqa: E402


def main() -> int:
    """Run multi-author scenario.

    Returns:
        Exit code (0 for success)
    """
    try:
        runner.run_multi_author_scenario(count=10, clean=True)
        return 0
    except Exception as e:
        print(f"\n❌ Scenario failed: {e}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
