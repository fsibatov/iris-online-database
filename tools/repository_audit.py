"""Repository hygiene/security audit for release gating."""

from __future__ import annotations

import json
import re
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
SKIP_DIRS = {".git"}
FORBIDDEN_DIRS = {
    "dist",
    "__pycache__",
    ".pytest_cache",
    ".mypy_cache",
    ".ruff_cache",
}
FORBIDDEN_NAMES = {
    ".env",
    "coverage.out",
    ".coverage",
    "$coverage",
    "iris-online-database",
    "pending-delete.json",
    "profile.json",
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
SECRET_PATTERNS = {
    "private key": re.compile(
        r"-----BEGIN (?:RSA |EC |DSA |OPENSSH )?PRIVATE KEY-----"
    ),
    "AWS access key": re.compile(r"\bAKIA[0-9A-Z]{16}\b"),
    "GitHub token": re.compile(
        r"\b(?:gh[pousr]_[A-Za-z0-9_]{20,}|github_pat_[A-Za-z0-9_]{20,})\b"
    ),
    "generic bearer token": re.compile(
        r"(?i)authorization\s*[:=]\s*bearer\s+[A-Za-z0-9._~+/=-]{20,}"
    ),
}
ABSOLUTE_PATH_PATTERNS = {
    "Windows user path": re.compile(r"(?i)\b[A-Z]:\\Users\\[^\\\s]+\\"),
    "Unix home path": re.compile(r"(?<![\w/])/home/[^/\s]+/"),
}
AUDIT_MARKERS = re.compile(r"\b(?:TODO|FIXME|HACK|XXX)\b")


def iter_files():
    for path in ROOT.rglob("*"):
        if not path.is_file():
            continue
        if any(part in SKIP_DIRS | FORBIDDEN_DIRS for part in path.parts):
            continue
        yield path


def relative(path: Path) -> str:
    return path.relative_to(ROOT).as_posix()


def read_text(path: Path) -> str | None:
    if path.suffix.lower() not in TEXT_SUFFIXES and path.name not in {
        ".gitignore",
        ".go-version",
    }:
        return None
    raw = path.read_bytes()
    try:
        return raw.decode("utf-8-sig")
    except UnicodeDecodeError as exc:
        raise AssertionError(f"{relative(path)}: invalid UTF-8: {exc}") from exc


def main() -> int:
    failures: list[str] = []
    warnings: list[str] = []

    for path in ROOT.rglob("*"):
        if not path.is_dir() or path.name not in FORBIDDEN_DIRS:
            continue
        if any(part in SKIP_DIRS for part in path.parts):
            continue
        failures.append(f"forbidden release directory: {relative(path)}/")

    for path in iter_files():
        rel = relative(path)
        lower_name = path.name.lower()
        if lower_name in FORBIDDEN_NAMES or path.suffix.lower() in FORBIDDEN_SUFFIXES:
            failures.append(f"forbidden release file: {rel}")

        if path.name.startswith("resource_windows_") and path.suffix.lower() == ".syso":
            failures.append(f"generated Windows resource stored in source: {rel}")

        try:
            text = read_text(path)
        except AssertionError as exc:
            failures.append(str(exc))
            continue
        if text is None:
            continue

        if path.name != "repository_audit.py":
            for label, pattern in SECRET_PATTERNS.items():
                if pattern.search(text):
                    failures.append(f"possible {label}: {rel}")
            for label, pattern in ABSOLUTE_PATH_PATTERNS.items():
                if pattern.search(text):
                    failures.append(f"absolute developer path ({label}): {rel}")

        if path.name != "repository_audit.py":
            marker_lines = [
                i
                for i, line in enumerate(text.splitlines(), 1)
                if AUDIT_MARKERS.search(line)
            ]
            if marker_lines:
                warnings.append(
                    f"manual marker audit: {rel}:{','.join(map(str, marker_lines[:8]))}"
                )

        if path.suffix.lower() == ".json":
            try:
                json.loads(text)
            except json.JSONDecodeError as exc:
                failures.append(
                    f"invalid JSON {rel}:{exc.lineno}:{exc.colno}: {exc.msg}"
                )

    if failures:
        print("Repository audit: FAIL")
        for item in failures:
            print(f"FAIL: {item}")
        return 1

    print("Repository audit: PASS")
    if warnings:
        for item in warnings:
            print(f"WARN: {item}")
    else:
        print("WARN: none")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
