"""Verify the exact Windows release asset set and SHA-256 manifest."""

from __future__ import annotations

import argparse
import hashlib
import re
from pathlib import Path

from release_targets import RELEASE_TARGETS

CHECKSUM_LINE = re.compile(r"^([0-9a-f]{64})  ([^/\\]+)$")


def expected_asset_names(version: str) -> tuple[str, ...]:
    executables = tuple(target.filename(version) for target in RELEASE_TARGETS)
    return (*executables, "SHA256SUMS.txt")


def verify_release_assets(directory: Path, version: str) -> None:
    if not directory.is_dir():
        raise SystemExit("release asset directory is missing")

    expected = set(expected_asset_names(version))
    actual = {entry.name for entry in directory.iterdir() if entry.is_file()}
    if actual != expected:
        missing = sorted(expected - actual)
        extra = sorted(actual - expected)
        categories = []
        if missing:
            categories.append(f"missing={len(missing)}")
        if extra:
            categories.append(f"extra={len(extra)}")
        raise SystemExit("release asset set mismatch (" + ",".join(categories) + ")")

    executable_names = [target.filename(version) for target in RELEASE_TARGETS]
    for name in executable_names:
        path = directory / name
        if path.stat().st_size <= 0:
            raise SystemExit(f"release executable is empty: {name}")

    checksum_path = directory / "SHA256SUMS.txt"
    raw = checksum_path.read_bytes()
    if raw.startswith(b"\xef\xbb\xbf"):
        raise SystemExit("SHA256SUMS.txt must not contain a UTF-8 BOM")
    try:
        text = raw.decode("ascii")
    except UnicodeDecodeError as error:
        raise SystemExit("SHA256SUMS.txt must be ASCII") from error
    lines = [line for line in text.splitlines() if line]
    if len(lines) != len(executable_names):
        raise SystemExit("SHA256SUMS.txt must contain exactly three entries")

    manifest: dict[str, str] = {}
    for line in lines:
        match = CHECKSUM_LINE.fullmatch(line)
        if not match:
            raise SystemExit("SHA256SUMS.txt has an invalid line")
        digest, name = match.groups()
        if name in manifest:
            raise SystemExit("SHA256SUMS.txt contains a duplicate asset entry")
        manifest[name] = digest

    if set(manifest) != set(executable_names):
        raise SystemExit("SHA256SUMS.txt asset names do not match release policy")

    for name in executable_names:
        actual_digest = hashlib.sha256((directory / name).read_bytes()).hexdigest()
        if manifest[name] != actual_digest:
            raise SystemExit(f"SHA-256 mismatch: {name}")

    print("Release assets/SHA256: PASS files=4")


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--directory", required=True, type=Path)
    parser.add_argument("--version", required=True)
    args = parser.parse_args()
    verify_release_assets(args.directory.resolve(), args.version)


if __name__ == "__main__":
    main()
