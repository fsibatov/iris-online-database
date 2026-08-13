"""Log-safe repository hygiene and secret-pattern audit for release gating."""

from __future__ import annotations

import argparse
import json
import re
import stat
from collections import Counter
from dataclasses import dataclass
from enum import Enum
from pathlib import Path

DEFAULT_ROOT = Path(__file__).resolve().parents[1]
SKIP_DIRS = {".git"}
FORBIDDEN_DIRS = {
    "dist",
    "coverage",
    "$coverage",
    "github-repo",
    "iris-online-source",
    "release-files",
    "tools-bin",
    "__pycache__",
    ".pytest_cache",
    ".mypy_cache",
    ".ruff_cache",
    ".venv",
}
FORBIDDEN_BUILD_DIRS = {"bin", "generated"}
FORBIDDEN_NAMES = {
    ".env",
    "appicon.png",
    "coverage.out",
    ".coverage",
    "$coverage",
    "iris-online-database",
    "pending-delete.json",
    "profile.json",
    "sha256sums.txt",
}
FORBIDDEN_SUFFIXES = {
    ".exe",
    ".dll",
    ".pdb",
    ".dmp",
    ".dump",
    ".pyc",
    ".pfx",
    ".p12",
    ".key",
    ".pem",
    ".syso",
    ".zip",
    ".rar",
    ".7z",
    ".log",
    ".tmp",
    ".bak",
    ".orig",
}
TEXT_SUFFIXES = {
    ".go",
    ".py",
    ".js",
    ".css",
    ".html",
    ".md",
    ".txt",
    ".json",
    ".yml",
    ".yaml",
    ".sh",
    ".ps1",
    ".env",
}
SECRET_PATTERNS = (
    re.compile(r"-----BEGIN (?:RSA |EC |DSA |OPENSSH )?PRIVATE KEY-----"),
    re.compile(r"\bAKIA[0-9A-Z]{16}\b"),
    re.compile(r"\b(?:gh[pousr]_[A-Za-z0-9_]{20,}|github_pat_[A-Za-z0-9_]{20,})\b"),
    re.compile(r"(?i)authorization\s*[:=]\s*bearer\s+[A-Za-z0-9._~+/=-]{20,}"),
)
ABSOLUTE_PATH_PATTERNS = (
    re.compile(r"(?i)\b[A-Z]:\\Users\\[^\\\s]+\\"),
    re.compile(r"(?<![\w/])/home/[^/\s]+/"),
)
AUDIT_MARKERS = re.compile(r"\b(?:TODO|FIXME|HACK|XXX)\b")


class Category(str, Enum):
    FORBIDDEN_DIRECTORY = "HYG001"
    FORBIDDEN_FILE = "HYG002"
    PYTHON_MODE = "HYG003"
    INVALID_UTF8 = "FMT001"
    INVALID_JSON = "FMT002"
    # This is a finding category, not a credential.
    SECRET = "SEC001"  # nosec B105
    DEVELOPER_PATH = "SEC002"


CATEGORY_MESSAGES = {
    Category.FORBIDDEN_DIRECTORY: "forbidden generated/release directory",
    Category.FORBIDDEN_FILE: "forbidden generated/release or credential file",
    Category.PYTHON_MODE: "Python tools must have no shebang and no executable bit",
    Category.INVALID_UTF8: "text file is not valid UTF-8",
    Category.INVALID_JSON: "JSON file is invalid",
    Category.SECRET: "possible credential material detected",
    Category.DEVELOPER_PATH: "absolute developer-specific path detected",
}


@dataclass(frozen=True)
class Finding:
    category: Category


def is_skipped(path: Path, root: Path) -> bool:
    try:
        parts = path.relative_to(root).parts
    except ValueError:
        return True
    return any(part in SKIP_DIRS for part in parts)


def is_forbidden_directory(path: Path, root: Path) -> bool:
    relative = path.relative_to(root)
    if path.name in FORBIDDEN_DIRS or path.name.startswith("iris-online-source-"):
        return True
    return (
        len(relative.parts) >= 2
        and relative.parts[0] == "build"
        and relative.parts[1] in FORBIDDEN_BUILD_DIRS
    )


def iter_source_files(root: Path):
    for path in root.rglob("*"):
        if path.is_symlink() or not path.is_file() or is_skipped(path, root):
            continue
        parents = (
            parent
            for parent in path.parents
            if parent != root and root in parent.parents
        )
        if any(is_forbidden_directory(parent, root) for parent in parents):
            continue
        yield path


def read_text(path: Path) -> str | None:
    if path.suffix.lower() not in TEXT_SUFFIXES and path.name not in {
        ".gitignore",
        ".go-version",
        "VERSION",
    }:
        return None
    try:
        return path.read_bytes().decode("utf-8-sig")
    except UnicodeDecodeError:
        return None


def audit(root: Path) -> tuple[list[Finding], Counter[str]]:
    findings: list[Finding] = []
    warnings: Counter[str] = Counter()

    for path in root.rglob("*"):
        if path.is_symlink() or not path.is_dir() or is_skipped(path, root):
            continue
        if is_forbidden_directory(path, root):
            findings.append(Finding(Category.FORBIDDEN_DIRECTORY))

    for path in root.rglob("*"):
        if path.is_symlink() and not is_skipped(path, root):
            findings.append(Finding(Category.FORBIDDEN_FILE))

    for path in iter_source_files(root):
        relative = path.relative_to(root).as_posix()
        lower_name = path.name.lower()
        if lower_name in FORBIDDEN_NAMES or path.suffix.lower() in FORBIDDEN_SUFFIXES:
            findings.append(Finding(Category.FORBIDDEN_FILE))

        text = read_text(path)
        is_expected_text = path.suffix.lower() in TEXT_SUFFIXES or path.name in {
            ".gitignore",
            ".go-version",
            "VERSION",
        }
        if is_expected_text and text is None:
            findings.append(Finding(Category.INVALID_UTF8))
            continue
        if text is None:
            continue

        if path.suffix.lower() == ".py" and relative.startswith("tools/"):
            executable = bool(
                path.stat().st_mode & (stat.S_IXUSR | stat.S_IXGRP | stat.S_IXOTH)
            )
            if text.startswith("#!") or executable:
                findings.append(Finding(Category.PYTHON_MODE))

        if relative != "tools/repository_audit.py":
            if any(pattern.search(text) for pattern in SECRET_PATTERNS):
                findings.append(Finding(Category.SECRET))
            if any(pattern.search(text) for pattern in ABSOLUTE_PATH_PATTERNS):
                findings.append(Finding(Category.DEVELOPER_PATH))
            marker_count = sum(
                bool(AUDIT_MARKERS.search(line)) for line in text.splitlines()
            )
            if marker_count:
                warnings["manual-code-marker"] += marker_count

        if path.suffix.lower() == ".json":
            try:
                json.loads(text)
            except json.JSONDecodeError:
                findings.append(Finding(Category.INVALID_JSON))

    return findings, warnings


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--root", type=Path, default=DEFAULT_ROOT)
    args = parser.parse_args()
    root = args.root.resolve()
    if not root.is_dir():
        print("Repository audit: FAIL")
        print("FAIL [CFG001] audit root is not a directory (count=1)")
        return 2

    findings, warnings = audit(root)
    counts = Counter(finding.category for finding in findings)
    if counts:
        print("Repository audit: FAIL")
        for category in sorted(counts, key=lambda item: item.value):
            print(
                f"FAIL [{category.value}] {CATEGORY_MESSAGES[category]} "
                f"(count={counts[category]})"
            )
        return 1

    print("Repository audit: PASS")
    if warnings:
        for category, count in sorted(warnings.items()):
            print(f"WARN [{category}] review required (count={count})")
    else:
        print("WARN: none")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
