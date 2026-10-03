from __future__ import annotations

import argparse
import re
import shutil
import subprocess  # nosec B404
from pathlib import Path

from prepare_windows_legacy import compatibility_marker
from release_targets import RELEASE_TARGETS, TARGET_BY_GOARCH

COMMON_METADATA_MARKERS = (
    ("TARGET_OS", "GOOS=windows"),
    ("CGO_DISABLED", "CGO_ENABLED=0"),
    ("TRIMPATH", "-trimpath=true"),
    ("PRODUCTION_TAGS", "-tags=desktop,wv2runtime.embed,production"),
    ("WAILS_VERSION", "github.com/wailsapp/wails/v2\tv2.14.0"),
)


def expected_metadata_markers(
    goarch: str, legacy: bool = False
) -> tuple[tuple[str, str], ...]:
    target = TARGET_BY_GOARCH[goarch]
    common = tuple(
        (
            category,
            marker + ",windows_legacy"
            if legacy and category == "PRODUCTION_TAGS"
            else marker,
        )
        for category, marker in COMMON_METADATA_MARKERS
    )
    return (
        ("TARGET_ARCH", f"GOARCH={target.goarch}"),
        ("TARGET_LEVEL", target.build_level_marker),
        *common,
    )


def missing_metadata_categories(
    metadata: str, goarch: str = "amd64", legacy: bool = False
) -> list[str]:
    missing = []
    for category, marker in expected_metadata_markers(goarch, legacy):
        if category == "PRODUCTION_TAGS":
            tags = re.search(r"(?:^|\s)-tags=([^\s]+)", metadata)
            if not tags or set(tags[1].split(",")) != set(
                marker.split("=", 1)[1].split(",")
            ):
                missing.append(category)
        elif marker not in metadata:
            missing.append(category)
    return missing


def release_marker_matches(binary: bytes, version: str, commit: str) -> bool:
    if not re.fullmatch(r"[0-9a-f]{40}", commit):
        return False
    markers = set(re.findall(rb"IrisOnlineRelease/[0-9.]+/[0-9A-Za-z]+", binary))
    return markers == {f"IrisOnlineRelease/{version}/{commit}".encode()}


def verify_executable(
    path: Path, version: str, goarch: str, go: str, commit: str, legacy: bool = False
) -> None:
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
    missing = missing_metadata_categories(metadata, goarch, legacy)
    if missing:
        raise SystemExit(
            f"release executable metadata mismatch ({goarch}): " + ",".join(missing)
        )
    binary = path.read_bytes()
    if legacy:
        if compatibility_marker().encode() not in binary:
            raise SystemExit(f"Windows 7 compatibility marker is missing ({goarch})")
        if (
            b"internal/syscall/windows.ProcessPrng" in binary
            or b"internal/syscall/windows.BCryptGenRandom" not in binary
        ):
            raise SystemExit(
                f"Windows 7 random generator compatibility mismatch ({goarch})"
            )
    if not release_marker_matches(binary, version, commit):
        raise SystemExit(f"release application commit marker is invalid ({goarch})")
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
    parser.add_argument("--expected-commit", required=True)
    args = parser.parse_args()
    if not re.fullmatch(r"[0-9a-f]{40}", args.expected_commit):
        parser.error("expected commit must be a complete lowercase Git SHA")
    go = shutil.which("go")
    if not go:
        raise SystemExit("Go executable is unavailable")

    for target in RELEASE_TARGETS:
        path = args.directory / target.filename(args.version)
        if not path.is_file():
            raise SystemExit(f"release executable is missing: {path.name}")
        verify_executable(
            path, args.version, target.goarch, go, args.expected_commit, target.legacy
        )


if __name__ == "__main__":
    main()
