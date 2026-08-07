#!/usr/bin/env python3
"""Run all reproducible Iris Online smoke tests."""
from __future__ import annotations

import argparse
import subprocess
import sys
from pathlib import Path


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--binary", required=True)
    args = parser.parse_args()
    root = Path(__file__).resolve().parent
    for script in ("api_smoke_test.py", "ui_smoke_test.py", "lifecycle_smoke_test.py", "rss_smoke_test.py"):
        command = [sys.executable, str(root / script), "--binary", args.binary]
        print("+", " ".join(command), flush=True)
        subprocess.run(command, check=True)
    print("All smoke tests: PASS")


if __name__ == "__main__":
    main()
