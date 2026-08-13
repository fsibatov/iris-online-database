"""Verify exact direct pins in the isolated Python audit environment."""

from __future__ import annotations

import argparse
import importlib.metadata
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]


def expected_versions(path: Path) -> dict[str, str]:
    result: dict[str, str] = {}
    for raw in path.read_text(encoding="utf-8").splitlines():
        line = raw.strip()
        if not line or line.startswith("#"):
            continue
        if line.count("==") != 1:
            raise ValueError("requirements must contain exact direct pins")
        package, version = (part.strip() for part in line.split("==", 1))
        if not package or not version or package.lower() in result:
            raise ValueError("requirements contain an invalid or duplicate pin")
        result[package.lower()] = version
    if not result:
        raise ValueError("requirements contain no direct pins")
    return result


def mismatches(path: Path) -> list[str]:
    problems = []
    for package, expected in expected_versions(path).items():
        try:
            actual = importlib.metadata.version(package)
        except importlib.metadata.PackageNotFoundError:
            problems.append(package)
            continue
        if actual != expected:
            problems.append(package)
    return problems


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument(
        "--requirements",
        type=Path,
        default=ROOT / "tools" / "requirements-audit.txt",
    )
    args = parser.parse_args()
    try:
        problems = mismatches(args.requirements)
    except (OSError, ValueError):
        print("Python audit environment: FAIL [INVALID_REQUIREMENTS]")
        return 2
    if problems:
        print(f"Python audit environment: FAIL [PIN_MISMATCH] count={len(problems)}")
        print("Affected packages: " + ", ".join(sorted(problems)))
        return 1
    print("Python audit environment: PASS")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
