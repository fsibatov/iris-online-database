"""Verify Go/Wails build metadata in the Windows amd64 release executable."""

from __future__ import annotations

import argparse
import re
import shutil

# The command uses fixed argv, no shell, and a resolved Go executable path.
import subprocess  # nosec B404
from pathlib import Path

EXPECTED_METADATA_MARKERS = (
    ("TARGET_OS", "GOOS=windows"),
    ("TARGET_ARCH", "GOARCH=amd64"),
    ("TARGET_LEVEL", "GOAMD64=v1"),
    ("CGO_DISABLED", "CGO_ENABLED=0"),
    ("TRIMPATH", "-trimpath=true"),
    ("PRODUCTION_TAGS", "-tags=desktop,wv2runtime.embed,production"),
    ("WAILS_VERSION", "github.com/wailsapp/wails/v2\tv2.14.0"),
)


def missing_metadata_categories(metadata: str) -> list[str]:
    return [
        category
        for category, marker in EXPECTED_METADATA_MARKERS
        if marker not in metadata
    ]


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--directory", type=Path, required=True)
    parser.add_argument("--version", required=True)
    args = parser.parse_args()
    path = args.directory / f"iris-online-database-{args.version}-windows-amd64.exe"
    if not path.is_file():
        raise SystemExit("release executable is missing")
    go = shutil.which("go")
    if not go:
        raise SystemExit("Go executable is unavailable")
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
        raise SystemExit("could not read Go build metadata")
    metadata = result.stdout
    missing = missing_metadata_categories(metadata)
    if missing:
        raise SystemExit("release executable metadata mismatch: " + ",".join(missing))
    binary = path.read_bytes()
    marker = f"IrisOnlineRelease/{args.version}/".encode()
    if marker not in binary:
        raise SystemExit("release application marker is missing")
    if b"IrisOnlineDiagnostic/" in binary or b"IrisOnlineDevelopment/" in binary:
        raise SystemExit("development marker found in release executable")
    lowered = binary.lower()
    absolute_path_patterns = (
        rb"(?:^|[\x00\r\n ])/(?:home|workspace)/[^/\x00\r\n ]+/",
        rb"[a-z]:\\users\\[^\\\x00\r\n ]+\\",
        rb"(?:^|[\x00\r\n ])/tmp/iris[^/\x00\r\n ]*/",
    )
    if any(re.search(pattern, lowered) for pattern in absolute_path_patterns):
        raise SystemExit("absolute developer path found in release executable")
    print(f"{path.name}: Go/Wails metadata PASS")


if __name__ == "__main__":
    main()
