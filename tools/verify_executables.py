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
    records = [line.lstrip(" \t").split("\t") for line in metadata.splitlines()]
    missing = []
    for category, marker in expected_metadata_markers(goarch):
        if category == "WAILS_VERSION":
            expected = ["dep", *marker.split("\t")]
            candidates = [
                (index, record) for index, record in enumerate(records)
                if len(record) >= 2 and record[:2] == expected[:2]
            ]
            present = len(candidates) == 1 and (
                candidates[0][1][:3] == expected
                and (
                    len(candidates[0][1]) == 3
                    or (
                        len(candidates[0][1]) == 4
                        and re.fullmatch(r"h1:[A-Za-z0-9+/=]+", candidates[0][1][3])
                        is not None
                    )
                )
                and not (
                    candidates[0][0] + 1 < len(records)
                    and records[candidates[0][0] + 1][:1] == ["=>"]
                )
            )
        else:
            key = marker.split("=", 1)[0] + "="
            candidates = [
                record for record in records
                if len(record) >= 2
                and record[0] == "build"
                and record[1].startswith(key)
            ]
            present = candidates == [["build", marker]]
        if not present:
            missing.append(category)
    return missing


def pinned_go_compiler_matches(metadata: str) -> bool:
    """The first `go version -m` line identifies the actual compiler."""
    pinned = (Path(__file__).resolve().parents[1] / ".go-version").read_text(
        encoding="utf-8"
    ).strip()
    if not re.fullmatch(r"[0-9]+\.[0-9]+\.[0-9]+", pinned):
        return False
    header = metadata.splitlines()[0] if metadata else ""
    return re.fullmatch(r".+: go" + re.escape(pinned), header) is not None


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
    if not pinned_go_compiler_matches(metadata):
        raise SystemExit(f"release Go compiler version mismatch ({goarch})")
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
        rb"(?:^|[\x00\r\n\t \"'=]|file://)/(?:home|workspace|users|root)/[^/\x00\r\n\t \"'=]+(?:/|[\x00\r\n\t \"'=]|$)",
        rb"[a-z]:[\\/]+users[\\/]+[^\\/\x00\r\n\t \"'=]+(?:[\\/]|[\x00\r\n\t \"'=]|$)",
        rb"(?:^|[\x00\r\n\t \"'=]|file://)/tmp/iris[^/\x00\r\n\t \"'=]*(?:/|[\x00\r\n\t \"'=]|$)",
        rb"(?:\\\\|//)[a-z0-9_.-]+[\\/](?:[a-z0-9_$.-]+[\\/])?users[\\/][^\\/\x00\r\n\t \"'=]+(?:[\\/]|[\x00\r\n\t \"'=]|$)",
        # Legacy Windows profiles can still appear through junctions or UNC shares.
        rb"""[a-z]:[\\/]+documents and settings[\\/]+[^\\/\x00\r\n\t"\'=]+(?:[\\/]|[\x00\r\n\t "\'=]|$)""",
        rb"""(?:\\\\|//)[a-z0-9_.-]+[\\/](?:[a-z0-9_$.-]+[\\/])?documents and settings[\\/]+[^\\/\x00\r\n\t"\'=]+(?:[\\/]|[\x00\r\n\t "\'=]|$)""",
    )
    # Windows resources can store paths as UTF-16LE rather than UTF-8/ASCII.
    # Check both byte alignments because a PE string may start at an odd offset.
    wide_patterns = tuple(pattern.decode("ascii") for pattern in absolute_path_patterns)
    wide_texts = (
        binary.decode("utf-16le", errors="ignore").lower(),
        binary[1:].decode("utf-16le", errors="ignore").lower(),
    )
    if (
        any(re.search(pattern, lowered) for pattern in absolute_path_patterns)
        or any(re.search(pattern, text) for text in wide_texts for pattern in wide_patterns)
    ):
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
