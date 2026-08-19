"""Validate GitHub workflow YAML before a release reaches GitHub."""

from __future__ import annotations

import re
from pathlib import Path

import yaml

ROOT = Path(__file__).resolve().parents[1]
ACTION_PIN = re.compile(r"^\s*uses:\s*[^@\s]+@([0-9a-f]{40})(?:\s|$)", re.MULTILINE)
ACTION_REFERENCE = re.compile(r"^\s*uses:\s*[^@\s]+@([^\s#]+)", re.MULTILINE)
EXPRESSION = re.compile(r"\$\{\{(.*?)\}\}", re.DOTALL)
JOB_ENV_DISALLOWED_CONTEXT = re.compile(r"\b(?:env|job|runner|steps)\.")
POSIX_SHELL = re.compile(r"^\s*shell:\s*(?:bash|sh)\s*$", re.IGNORECASE | re.MULTILINE)


def invalid_job_env_contexts(document: dict[object, object]) -> int:
    jobs = document.get("jobs")
    if not isinstance(jobs, dict):
        return 0
    failures = 0
    for job in jobs.values():
        if not isinstance(job, dict):
            continue
        job_env = job.get("env")
        if not isinstance(job_env, dict):
            continue
        for value in job_env.values():
            if not isinstance(value, str):
                continue
            for expression in EXPRESSION.findall(value):
                if JOB_ENV_DISALLOWED_CONTEXT.search(expression):
                    failures += 1
    return failures


def windows_only_workflow_failures(
    document: dict[object, object], text: str
) -> int:
    jobs = document.get("jobs")
    if not isinstance(jobs, dict):
        return 1

    failures = 0
    for job in jobs.values():
        if not isinstance(job, dict):
            failures += 1
            continue
        runs_on = job.get("runs-on")
        if not isinstance(runs_on, str) or not runs_on.startswith("windows-"):
            failures += 1

    failures += len(POSIX_SHELL.findall(text))
    return failures


def release_policy_failures() -> int:
    failures = 0
    ci_path = ROOT / ".github" / "workflows" / "ci.yml"
    codeql_path = ROOT / ".github" / "workflows" / "codeql.yml"
    dependency_path = ROOT / ".github" / "workflows" / "dependency-review.yml"
    try:
        ci = ci_path.read_text(encoding="utf-8")
        codeql = codeql_path.read_text(encoding="utf-8")
        dependency = dependency_path.read_text(encoding="utf-8")
    except OSError:
        return 1

    required_checks = (
        "name: Windows quality and security",
        "name: Windows race detector",
        "name: Native Windows Wails release matrix",
    )
    if any(marker not in ci for marker in required_checks):
        failures += 1
    if "ubuntu-" in ci or "ubuntu-" in codeql or "ubuntu-" in dependency:
        failures += 1
    if ci.count("runs-on: windows-2025") < 3:
        failures += 1
    if (
        "runs-on: windows-2025" not in codeql
        or "runs-on: windows-2025" not in dependency
    ):
        failures += 1
    if (
        "name: Analyze (${{ matrix.language }})" not in codeql
        or "language: [go, python]" not in codeql
        or "Build Windows Go sources" not in codeql
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
    for marker in (
        "Run canonical Windows release gate",
        "IrisTools.ps1 -Action Test",
        "GITLEAKS_WINDOWS_X64_SHA256",
        "playwright install chromium",
        "Windows amd64 race detector",
        "go test -race -count=1 ./...",
        "libsynchronization.a",
    ):
        if marker not in ci:
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
        failures += invalid_job_env_contexts(document)
        failures += windows_only_workflow_failures(document, text)
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
