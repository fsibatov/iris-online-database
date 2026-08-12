"""Verify Go build metadata and application marker for Iris Online Windows executables."""

from __future__ import annotations

import argparse
import subprocess
from pathlib import Path


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--directory", type=Path, required=True)
    parser.add_argument("--version", required=True)
    parser.add_argument("--go-version", required=True, help="for example go1.26.5")
    parser.add_argument("--diagnostic", action="store_true")
    args = parser.parse_args()

    expected = {
        "x64": ("amd64", "GOAMD64=v1"),
        "x86": ("386", "GO386=softfloat"),
        "arm64": ("arm64", "GOARM64=v8.0"),
    }
    if args.diagnostic:
        marker = f"IrisOnlineDiagnostic/{args.version}/{args.go_version}"
        suffix = f"-diagnostic-{args.go_version}"
    else:
        marker = f"IrisOnlineRelease/{args.version}"
        suffix = ""

    for label, (arch, tuning) in expected.items():
        path = (
            args.directory / f"IrisOnlineDB-{args.version}{suffix}-Windows-{label}.exe"
        )
        if not path.is_file():
            raise SystemExit(f"missing executable: {path}")
        output = subprocess.check_output(["go", "version", "-m", str(path)], text=True)
        first = output.splitlines()[0]
        if args.go_version not in first:
            raise SystemExit(f"{path.name}: expected {args.go_version}, got {first}")
        if f"GOARCH={arch}" not in output:
            raise SystemExit(f"{path.name}: expected GOARCH={arch}")
        if tuning not in output:
            raise SystemExit(f"{path.name}: expected {tuning}")
        if marker.encode("ascii") not in path.read_bytes():
            raise SystemExit(f"{path.name}: missing application marker {marker}")
        print(f"{path.name}: OK ({args.go_version}, {arch}, {tuning}, {marker})")


if __name__ == "__main__":
    main()
