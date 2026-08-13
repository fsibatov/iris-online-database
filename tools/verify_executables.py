"""Verify Go/Wails build metadata in the Windows amd64 release executable."""

from __future__ import annotations

import argparse
import shutil

# The command uses fixed argv, no shell, and a resolved Go executable path.
import subprocess  # nosec B404
from pathlib import Path


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
    expected = (
        "GOOS=windows",
        "GOARCH=amd64",
        "GOAMD64=v1",
        "CGO_ENABLED=0",
        "-trimpath=true",
        "-tags=desktop,wv2runtime.embed,production",
        "github.com/wailsapp/wails/v2\tv2.14.0",
    )
    missing = [marker for marker in expected if marker not in metadata]
    if missing:
        raise SystemExit("release executable has incomplete build metadata")
    binary = path.read_bytes()
    marker = f"IrisOnlineRelease/{args.version}/".encode()
    if marker not in binary:
        raise SystemExit("release application marker is missing")
    if b"IrisOnlineDiagnostic/" in binary or b"IrisOnlineDevelopment/" in binary:
        raise SystemExit("development marker found in release executable")
    lowered = binary.lower()
    if any(
        marker in lowered
        for marker in (b"/workspace/", b"/home/", b"\\users\\", b"/tmp/iris")
    ):
        raise SystemExit("absolute developer path found in release executable")
    print(f"{path.name}: Go/Wails metadata PASS")


if __name__ == "__main__":
    main()
