"""Validate GitHub workflow YAML before a release reaches GitHub."""

from __future__ import annotations

import re
from pathlib import Path

import yaml

ROOT = Path(__file__).resolve().parents[1]
ACTION_PIN = re.compile(r"^\s*uses:\s*[^@\s]+@([0-9a-f]{40})(?:\s|$)", re.MULTILINE)
ACTION_REFERENCE = re.compile(r"^\s*uses:\s*[^@\s]+@([^\s#]+)", re.MULTILINE)


def release_policy_failures() -> int:
    failures = 0
    ci_path = ROOT / ".github" / "workflows" / "ci.yml"
    codeql_path = ROOT / ".github" / "workflows" / "codeql.yml"
    try:
        ci = ci_path.read_text(encoding="utf-8")
        codeql = codeql_path.read_text(encoding="utf-8")
    except OSError:
        return 1

    required_checks = (
        "name: Linux quality and security",
        "name: Go race detector",
        "name: Native Windows Wails release matrix",
    )
    if any(marker not in ci for marker in required_checks):
        failures += 1
    if (
        "name: Analyze (${{ matrix.language }})" not in codeql
        or "language: [go, python]" not in codeql
    ):
        failures += 1

    windows_start = ci.find("  windows-build:")
    if windows_start < 0:
        failures += 1
        return failures
    windows = ci[windows_start:]
    ordered_steps = (
        "- name: Setup Python 3.13",
        "- name: Setup pinned Go",
        "- name: Install pinned Windows release Go tools",
        "- name: Parse and self-test Windows PowerShell 5.1 tooling",
    )
    positions = [windows.find(marker) for marker in ordered_steps]
    if any(position < 0 for position in positions) or positions != sorted(positions):
        failures += 1
    for marker in (
        "wails@v2.14.0",
        "staticcheck@2026.1",
        "govulncheck@v1.6.0",
        'Platform = "windows/amd64"',
        'Platform = "windows/386"',
        'Platform = "windows/arm64"',
        "verify_release_assets.py",
        "verify_executables.py",
        "verify_windows_resources.py",
    ):
        if marker not in windows:
            failures += 1
    return failures


def main() -> int:
    failures = 0
    workflows = sorted((ROOT / ".github" / "workflows").glob("*.yml"))
    if not workflows:
        print("Workflow validation: FAIL [WF001] count=1")
        return 1

    for path in workflows:
        text = path.read_text(encoding="utf-8")
        try:
            document = yaml.safe_load(text)
        except yaml.YAMLError:
            failures += 1
            continue
        if not isinstance(document, dict) or not isinstance(document.get("jobs"), dict):
            failures += 1
            continue
        references = ACTION_REFERENCE.findall(text)
        pins = ACTION_PIN.findall(text)
        if len(references) != len(pins):
            failures += 1

    failures += release_policy_failures()

    if failures:
        print(f"Workflow validation: FAIL [WF001] count={failures}")
        return 1
    print(f"Workflow validation: PASS files={len(workflows)}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
