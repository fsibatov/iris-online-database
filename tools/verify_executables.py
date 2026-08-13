"""Verify Go/Wails build metadata in every Windows release executable."""

from __future__ import annotations

import argparse
import re
import shutil

# The command uses fixed argv, no shell, and a resolved Go executable path.
import subprocess  # nosec B404
from pathlib import Path

from release_targets import RELEASE_TARGETS, TARGET_BY_GOARCH

COMMON_METADATA_MARKERS = (
    ("TARGET_OS", "GOOS=windows"),
    ("CGO_DISABLED", "CGO_ENABLED=0"),
    ("TRIMPATH", "-trimpath=true"),
    ("PRODUCTION_TAGS", "-tags=desktop,wv2runtime.embed,production"),
    ("WAILS_VERSION", "github.com/wailsapp/wails/v2\tv2.14.0"),
)

# Backward-compatible alias used by existing regression tests: amd64 is the
# first release target, but verification below covers the complete matrix.
EXPECTED_METADATA_MARKERS = (
    ("TARGET_ARCH", "GOARCH=amd64"),
    ("TARGET_LEVEL", "GOAMD64=v1"),
    *COMMON_METADATA_MARKERS,
)


def expected_metadata_markers(goarch: str) -> tuple[tuple[str, str], ...]:
    target = TARGET_BY_GOARCH[goarch]
    return (
        ("TARGET_ARCH", f"GOARCH={target.goarch}"),
        ("TARGET_LEVEL", target.build_level_marker),
        *COMMON_METADATA_MARKERS,
    )


def missing_metadata_categories(metadata: str, goarch: str = "amd64") -> list[str]:
    return [
        category
        for category, marker in expected_metadata_markers(goarch)
        if marker not in metadata
    ]


def verify_executable(path: Path, version: str, goarch: str, go: str) -> None:
    result = subprocess.run(
        [go, "version", "-m", str(path)],
        check=False,
        capture_output=True,
        text=True,
        encoding="utf-8",
        errors="replace",
        timeout=30,
    )  # nosec B603
    if result.returncode:
        raise SystemExit(f"could not read Go build metadata ({goarch})")
    metadata = result.stdout
    missing = missing_metadata_categories(metadata, goarch)
    if missing:
        raise SystemExit(
            f"release executable metadata mismatch ({goarch}): " + ",".join(missing)
        )
    binary = path.read_bytes()
    marker = f"IrisOnlineRelease/{version}/".encode()
    if marker not in binary:
        raise SystemExit(f"release application marker is missing ({goarch})")
    if b"IrisOnlineDiagnostic/" in binary or b"IrisOnlineDevelopment/" in binary:
        raise SystemExit(f"development marker found in release executable ({goarch})")
    lowered = binary.lower()
    absolute_path_patterns = (
        rb"(?:^|[\x00\r\n ])/(?:home|workspace)/[^/\x00\r\n ]+/",
        rb"[a-z]:\\users\\[^\\\x00\r\n ]+\\",
        rb"(?:^|[\x00\r\n ])/tmp/iris[^/\x00\r\n ]*/",
    )
    if any(re.search(pattern, lowered) for pattern in absolute_path_patterns):
        raise SystemExit(
            f"absolute developer path found in release executable ({goarch})"
        )
    print(f"{path.name}: Go/Wails metadata PASS ({goarch})")


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--directory", type=Path, required=True)
    parser.add_argument("--version", required=True)
    args = parser.parse_args()
    go = shutil.which("go")
    if not go:
        raise SystemExit("Go executable is unavailable")

    for target in RELEASE_TARGETS:
        path = args.directory / target.filename(args.version)
        if not path.is_file():
            raise SystemExit(f"release executable is missing: {path.name}")
        verify_executable(path, args.version, target.goarch, go)


if __name__ == "__main__":
    main()
