"""Create and verify the strict release-gate fingerprint for the current Git tree."""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import shutil

# Subprocesses use fixed argv, no shell, and resolved executable paths.
import subprocess  # nosec B404
import sys
import tempfile
from datetime import UTC, datetime
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
SCHEMA = 1
VERSION_COMMANDS = {
    "bandit",
    "git",
    "gitleaks",
    "go",
    "govulncheck",
    "node",
    "pip-audit",
    "ruff",
    "staticcheck",
    "wails",
}


class FingerprintError(RuntimeError):
    """A release-gate invariant was not satisfied."""


def git(root: Path, *args: str) -> str:
    executable = shutil.which("git")
    if not executable:
        raise FingerprintError("Git executable is unavailable")
    result = subprocess.run(
        [executable, "-C", str(root), *args],
        check=False,
        capture_output=True,
        text=True,
        encoding="utf-8",
        errors="replace",
        timeout=30,
    )  # nosec B603
    if result.returncode:
        raise FingerprintError("Git command failed")
    return result.stdout.strip()


def assert_release_tree(root: Path, expected_branch: str) -> tuple[str, str]:
    if git(root, "rev-parse", "--is-inside-work-tree") != "true":
        raise FingerprintError("Release root is not a Git working tree")
    head = git(root, "rev-parse", "HEAD")
    branch = git(root, "branch", "--show-current")
    if not branch:
        raise FingerprintError("Detached HEAD cannot be released")
    if expected_branch and branch != expected_branch:
        raise FingerprintError("Unexpected release branch")
    if git(root, "status", "--porcelain=v1", "--untracked-files=all"):
        raise FingerprintError("Git working tree is not clean")
    return head, branch


def tracked_entries(root: Path) -> list[tuple[Path, bytes]]:
    executable = shutil.which("git")
    if not executable:
        raise FingerprintError("Git executable is unavailable")
    raw = subprocess.run(
        [executable, "-C", str(root), "ls-files", "--stage", "-z"],
        check=True,
        capture_output=True,
        timeout=30,
    ).stdout  # nosec B603
    entries = []
    for record in raw.split(b"\0"):
        if not record:
            continue
        try:
            metadata, encoded = record.split(b"\t", 1)
            mode, _object_id, stage = metadata.split(b" ", 2)
        except ValueError as error:
            raise FingerprintError("Invalid Git index entry") from error
        if stage != b"0" or mode not in {b"100644", b"100755", b"120000"}:
            raise FingerprintError("Unsupported Git index entry")
        relative = Path(os.fsdecode(encoded))
        if relative.is_absolute() or ".." in relative.parts:
            raise FingerprintError("Unsafe tracked path")
        entries.append((relative, mode))
    return sorted(entries, key=lambda entry: entry[0].as_posix().encode())


def source_hash(root: Path) -> tuple[str, int]:
    digest = hashlib.sha256()
    entries = tracked_entries(root)
    for relative, mode in entries:
        path = root / relative
        if mode == b"120000" and path.is_symlink():
            content = os.readlink(path).encode()
        elif path.is_file():
            content = path.read_bytes()
        else:
            raise FingerprintError("Tracked source entry is missing")
        encoded_path = relative.as_posix().encode()
        digest.update(len(encoded_path).to_bytes(8, "big"))
        digest.update(encoded_path)
        digest.update(mode + b"\0")
        digest.update(len(content).to_bytes(8, "big"))
        digest.update(content)
    return digest.hexdigest(), len(entries)


def command_version(command: list[str]) -> str:
    if not command or command[0] not in VERSION_COMMANDS:
        return "unavailable"
    executable = shutil.which(command[0])
    if not executable:
        return "unavailable"
    try:
        result = subprocess.run(
            [executable, *command[1:]],
            check=False,
            capture_output=True,
            text=True,
            encoding="utf-8",
            errors="replace",
            timeout=15,
        )  # nosec B603
    except (OSError, subprocess.TimeoutExpired):
        return "unavailable"
    output = (result.stdout or result.stderr).strip().splitlines()
    return output[0][:240] if result.returncode == 0 and output else "unavailable"


def toolchain() -> dict[str, str]:
    return {
        "git": command_version(["git", "--version"]),
        "go": command_version(["go", "version"]),
        "node": command_version(["node", "--version"]),
        "python": f"Python {sys.version_info.major}.{sys.version_info.minor}.{sys.version_info.micro}",
        "wails": command_version(["wails", "version"]),
        "ruff": command_version(["ruff", "--version"]),
        "bandit": command_version(["bandit", "--version"]),
        "pip-audit": command_version(["pip-audit", "--version"]),
        "gitleaks": command_version(["gitleaks", "version"]),
        "staticcheck": command_version(["staticcheck", "-version"]),
        "govulncheck": command_version(["govulncheck", "-version"]),
    }


def fingerprint_path(root: Path) -> Path:
    git_dir = Path(git(root, "rev-parse", "--git-dir"))
    if not git_dir.is_absolute():
        git_dir = root / git_dir
    return git_dir.resolve() / "iris-release-fingerprint.json"


def build_fingerprint(root: Path, expected_branch: str) -> dict[str, object]:
    head, branch = assert_release_tree(root, expected_branch)
    digest, count = source_hash(root)
    return {
        "schema": SCHEMA,
        "application": "iris-online-database",
        "version": (root / "VERSION").read_text(encoding="utf-8").strip(),
        "head": head,
        "branch": branch,
        "source_sha256": digest,
        "tracked_files": count,
        "created_at": datetime.now(UTC).isoformat(timespec="seconds"),
        "toolchain": toolchain(),
    }


def write_fingerprint(root: Path, expected_branch: str) -> Path:
    payload = build_fingerprint(root, expected_branch)
    destination = fingerprint_path(root)
    destination.parent.mkdir(parents=True, exist_ok=True)
    with tempfile.NamedTemporaryFile(
        mode="w",
        encoding="utf-8",
        newline="\n",
        prefix="iris-fingerprint-",
        suffix=".tmp",
        dir=destination.parent,
        delete=False,
    ) as handle:
        json.dump(payload, handle, ensure_ascii=False, indent=2, sort_keys=True)
        handle.write("\n")
        temporary = Path(handle.name)
    os.replace(temporary, destination)
    return destination


def verify_fingerprint(root: Path, expected_branch: str) -> None:
    destination = fingerprint_path(root)
    try:
        saved = json.loads(destination.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as error:
        raise FingerprintError("Release fingerprint is missing or invalid") from error
    head, branch = assert_release_tree(root, expected_branch)
    digest, count = source_hash(root)
    expected = {
        "schema": SCHEMA,
        "application": "iris-online-database",
        "version": (root / "VERSION").read_text(encoding="utf-8").strip(),
        "head": head,
        "branch": branch,
        "source_sha256": digest,
        "tracked_files": count,
    }
    if any(saved.get(key) != value for key, value in expected.items()):
        raise FingerprintError("Release fingerprint does not match the current source")


def main() -> int:
    parser = argparse.ArgumentParser()
    action = parser.add_mutually_exclusive_group(required=True)
    action.add_argument("--write", action="store_true")
    action.add_argument("--verify", action="store_true")
    parser.add_argument("--root", type=Path, default=ROOT)
    parser.add_argument("--expected-branch", default="main")
    args = parser.parse_args()
    root = args.root.resolve()
    try:
        if args.write:
            write_fingerprint(root, args.expected_branch)
            print("Release fingerprint: WRITTEN")
        else:
            verify_fingerprint(root, args.expected_branch)
            print("Release fingerprint: VALID")
    except FingerprintError as error:
        print(f"Release fingerprint: FAIL ({error})")
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
