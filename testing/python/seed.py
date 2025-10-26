#!/usr/bin/env python3
"""Seed workspaces with test documents.

Usage:
    python seed.py --scenario basic --count 10 --clean
    python seed.py --scenario migration --count 5
    python seed.py --scenario multi-author --count 10 --clean
"""

import argparse
import sys
from pathlib import Path

# Add parent directory to path for imports
sys.path.insert(0, str(Path(__file__).parent))

from seeding import ScenarioType, WorkspaceSeeder  # noqa: E402


def main() -> int:
    """Seed workspaces based on CLI arguments.

    Returns:
        Exit code (0 for success)
    """
    parser = argparse.ArgumentParser(
        description="Seed Hermes workspaces with test documents"
    )
    parser.add_argument(
        "--scenario",
        type=str,
        choices=["basic", "migration", "multi-author"],
        default="basic",
        help="Scenario type to generate",
    )
    parser.add_argument(
        "--count",
        type=int,
        default=10,
        help="Number of documents to generate",
    )
    parser.add_argument(
        "--clean",
        action="store_true",
        help="Clean workspace before seeding",
    )

    args = parser.parse_args()

    try:
        seeder = WorkspaceSeeder()
        scenario = ScenarioType(args.scenario.replace("-", "_"))

        result = seeder.seed_scenario(
            scenario=scenario,
            count=args.count,
            clean=args.clean,
        )

        if isinstance(result, tuple):
            print(f"\n✓ Created {len(result[0]) + len(result[1])} total files")
        else:
            print(f"\n✓ Created {len(result)} files")

        return 0

    except Exception as e:
        print(f"\n❌ Seeding failed: {e}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
