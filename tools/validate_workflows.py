"""Validate GitHub workflow YAML before a release reaches GitHub."""

from __future__ import annotations

import os
import re
import subprocess  # nosec B404
import tempfile
from pathlib import Path

import yaml

ROOT = Path(__file__).resolve().parents[1]
ACTION_PIN = re.compile(r"^\s*uses:\s*[^@\s]+@([0-9a-f]{40})(?:\s|$)", re.MULTILINE)
ACTION_REFERENCE = re.compile(r"^\s*uses:\s*[^@\s]+@([^\s#]+)", re.MULTILINE)
ACTION_SHA = re.compile(r"[^@\s]+@[0-9a-f]{40}\Z")
LOCAL_ACTION = re.compile(r"\./[^\s]+\Z")
EXPRESSION = re.compile(r"\$\{\{(.*?)\}\}", re.DOTALL)
JOB_ENV_DISALLOWED_CONTEXT = re.compile(r"\b(?:env|job|runner|steps)\.")
POSIX_SHELL = frozenset({"bash", "sh"})
POWERSHELL_PARSER = r"""param([Parameter(Mandatory = $true)][string]$ScriptPath)
$Tokens = $null
$Errors = $null
[System.Management.Automation.Language.Parser]::ParseFile(
    $ScriptPath,
    [ref]$Tokens,
    [ref]$Errors
) | Out-Null
if ($Errors.Count -ne 0) {
    foreach ($ErrorRecord in $Errors) {
        Write-Output $ErrorRecord.Message
    }
    exit 1
}
"""


class UniqueKeySafeLoader(yaml.SafeLoader):
    """Reject duplicate YAML keys instead of silently replacing workflow jobs."""

    def construct_mapping(self, node, deep=False):
        if not isinstance(node, yaml.MappingNode):
            return super().construct_mapping(node, deep=deep)
        seen = set()
        for key_node, _ in node.value:
            if key_node.tag == "tag:yaml.org,2002:merge":
                continue
            key = self.construct_object(key_node, deep=deep)
            try:
                if key in seen:
                    raise yaml.constructor.ConstructorError(
                        None, None, "duplicate workflow YAML key", key_node.start_mark
                    )
                seen.add(key)
            except TypeError as error:
                raise yaml.constructor.ConstructorError(
                    None, None, "unhashable workflow YAML key", key_node.start_mark
                ) from error
        return super().construct_mapping(node, deep=deep)


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


def unpinned_action_failures(document: dict[object, object]) -> int:
    """Inspect parsed YAML: flow-style `uses` entries evade line-based regexes."""
    jobs = document.get("jobs")
    if not isinstance(jobs, dict):
        return 1

    failures = 0
    for job in jobs.values():
        if not isinstance(job, dict):
            continue
        references = []
        if "uses" in job:
            references.append(job["uses"])
        if "steps" in job:
            steps = job["steps"]
            if not isinstance(steps, list):
                failures += 1
            else:
                for step in steps:
                    if not isinstance(step, dict):
                        failures += 1
                    elif "uses" in step:
                        references.append(step["uses"])
        for reference in references:
            if not isinstance(reference, str) or not (
                ACTION_SHA.fullmatch(reference) or LOCAL_ACTION.fullmatch(reference)
            ):
                failures += 1
    return failures


def windows_only_workflow_failures(document: dict[object, object], _text: str) -> int:
    jobs = document.get("jobs")
    if not isinstance(jobs, dict):
        return 1

    def is_posix_shell(value: object) -> bool:
        if not isinstance(value, str):
            return False
        # GitHub Actions accepts custom shell templates, e.g. `bash -e {0}`.
        command = value.strip().split(maxsplit=1)
        return bool(command) and command[0].lower().removesuffix(".exe") in POSIX_SHELL

    failures = 0
    for scope in (document, *jobs.values()):
        if not isinstance(scope, dict):
            continue
        defaults = scope.get("defaults")
        if isinstance(defaults, dict):
            run = defaults.get("run")
            if isinstance(run, dict) and is_posix_shell(run.get("shell")):
                failures += 1

    for job in jobs.values():
        if not isinstance(job, dict):
            failures += 1
            continue
        runs_on = job.get("runs-on")
        if not isinstance(runs_on, str) or not runs_on.startswith("windows-"):
            failures += 1
        steps = job.get("steps")
        if isinstance(steps, list):
            for step in steps:
                if isinstance(step, dict) and is_posix_shell(step.get("shell")):
                    failures += 1

    return failures


def windows_powershell_executable() -> Path | None:
    """Locate the native parser; Linux cannot certify Windows PowerShell syntax."""
    system_root = os.environ.get("SYSTEMROOT")
    if not system_root:
        return None
    executable = (
        Path(system_root) / "System32" / "WindowsPowerShell" / "v1.0" / "powershell.exe"
    )
    return executable if executable.is_file() else None


def powershell_step_failures(document: dict[object, object]) -> int:
    jobs = document.get("jobs")
    if not isinstance(jobs, dict):
        return 1

    powershell_executable = windows_powershell_executable()
    if powershell_executable is None:
        return 1

    scripts: list[str] = []
    for job in jobs.values():
        if not isinstance(job, dict):
            continue
        steps = job.get("steps")
        if not isinstance(steps, list):
            continue
        for step in steps:
            if not isinstance(step, dict):
                continue
            shell = str(step.get("shell") or "").strip().lower()
            run = step.get("run")
            if shell not in {"powershell", "pwsh"} or not isinstance(run, str):
                continue
            scripts.append(EXPRESSION.sub("GITHUB_EXPRESSION", run))

    if not scripts:
        return 0

    failures = 0
    with tempfile.TemporaryDirectory(prefix="iris-workflow-powershell-") as temp_dir:
        temp = Path(temp_dir)
        parser_path = temp / "parse-workflow.ps1"
        parser_path.write_text(POWERSHELL_PARSER, encoding="utf-8-sig")

        for index, script in enumerate(scripts, start=1):
            script_path = temp / f"workflow-step-{index}.ps1"
            script_path.write_text(script, encoding="utf-8-sig")
            try:
                # Fixed executable and controlled argument list; shell execution is disabled.
                result = subprocess.run(  # nosec B603
                    [
                        str(powershell_executable),
                        "-NoLogo",
                        "-NoProfile",
                        "-NonInteractive",
                        "-ExecutionPolicy",
                        "Bypass",
                        "-File",
                        str(parser_path),
                        str(script_path),
                    ],
                    cwd=ROOT,
                    stdout=subprocess.PIPE,
                    stderr=subprocess.STDOUT,
                    text=True,
                    encoding="utf-8",
                    errors="replace",
                    timeout=30,
                    check=False,
                )
            except (OSError, subprocess.TimeoutExpired):
                failures += 1
                continue
            if result.returncode != 0:
                failures += 1
                output = result.stdout.strip()
                if output:
                    print(f"PowerShell workflow syntax failure #{index}: {output}")

    return failures



def ci_trigger_failures(ci: str) -> int:
    """Require main push/PR and manual CI triggers, not merely a jobs block."""
    try:
        # BaseLoader preserves the literal `on` key (SafeLoader treats it as True).
        document = yaml.load(ci, Loader=yaml.BaseLoader)
    except yaml.YAMLError:
        return 1
    if not isinstance(document, dict):
        return 1
    triggers = document.get("on")
    if not isinstance(triggers, dict) or "workflow_dispatch" not in triggers:
        return 1
    for event in ("push", "pull_request"):
        settings = triggers.get(event)
        if not isinstance(settings, dict):
            return 1
        branches = settings.get("branches")
        if not isinstance(branches, list) or "main" not in branches:
            return 1
        # Path filters can suppress mandatory Windows CI for otherwise valid
        # pushes and pull requests (including changes to Go/security code).
        if "paths" in settings or "paths-ignore" in settings:
            return 1
    return 0


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

    failures += ci_trigger_failures(ci)

    required_checks = {
        "Windows quality and security",
        "Windows race detector",
        "Native Windows Wails release matrix",
    }
    try:
        ci_document = yaml.load(ci, Loader=UniqueKeySafeLoader)
        codeql_document = yaml.load(codeql, Loader=UniqueKeySafeLoader)
        dependency_document = yaml.load(dependency, Loader=UniqueKeySafeLoader)
    except yaml.YAMLError:
        ci_document = codeql_document = dependency_document = None
    jobs = ci_document.get("jobs") if isinstance(ci_document, dict) else None
    if not isinstance(jobs, dict):
        failures += 1
    else:
        actual_checks = {
            job.get("name")
            for job in jobs.values()
            if isinstance(job, dict) and isinstance(job.get("name"), str)
        }
        if not required_checks.issubset(actual_checks):
            failures += 1
        # Required Windows gates must run unconditionally for every CI trigger.
        # A job-level `if` can silently skip a correctly named security gate.
        if any(
            isinstance(job, dict)
            and job.get("name") in required_checks
            and "if" in job
            for job in jobs.values()
        ):
            failures += 1
    if "ubuntu-" in ci or "ubuntu-" in codeql or "ubuntu-" in dependency:
        failures += 1
    def pinned_windows_runner_count(document: object) -> int:
        jobs = document.get("jobs") if isinstance(document, dict) else None
        if not isinstance(jobs, dict):
            return 0
        return sum(
            isinstance(job, dict) and job.get("runs-on") == "windows-2025"
            for job in jobs.values()
        )

    # Counting pinned runners is insufficient: unrelated jobs can mask a
    # required quality/race/build gate moved to an older Windows image.
    if not isinstance(jobs, dict) or any(
        not isinstance(jobs.get(job_id), dict)
        or jobs[job_id].get("runs-on") != "windows-2025"
        for job_id in ("windows-quality", "windows-race", "windows-build")
    ):
        failures += 1
    if pinned_windows_runner_count(ci_document) < 3:
        failures += 1
    if (
        pinned_windows_runner_count(codeql_document) < 1
        or pinned_windows_runner_count(dependency_document) < 1
    ):
        failures += 1
    if (
        "name: Analyze (${{ matrix.language }})" not in codeql
        or "language: [go, python]" not in codeql
    ):
        failures += 1
    codeql_jobs = codeql_document.get("jobs") if isinstance(codeql_document, dict) else None
    analyze = codeql_jobs.get("analyze") if isinstance(codeql_jobs, dict) else None
    # Text markers can be preserved in comments while a real matrix entry
    # silently drops Go or Python from CodeQL coverage.
    strategy = analyze.get("strategy") if isinstance(analyze, dict) else None
    matrix = strategy.get("matrix") if isinstance(strategy, dict) else None
    languages = matrix.get("language") if isinstance(matrix, dict) else None
    if not (
        isinstance(languages, list)
        and len(languages) == 2
        and all(isinstance(language, str) for language in languages)
        and set(languages) == {"go", "python"}
    ):
        failures += 1
    codeql_steps = analyze.get("steps") if isinstance(analyze, dict) else None
    go_build_steps = (
        [step for step in codeql_steps if isinstance(step, dict)
         and step.get("name") == "Build Windows Go sources"]
        if isinstance(codeql_steps, list) else []
    )
    if len(go_build_steps) != 1 or not (
        go_build_steps[0].get("if") == "matrix.language == 'go'"
        and go_build_steps[0].get("shell") == "powershell"
        and isinstance(go_build_steps[0].get("run"), str)
        and all(marker in go_build_steps[0]["run"] for marker in (
            "iris-codeql-go-temp-", "go test -count=1 ./...", "go build -trimpath"
        ))
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
    windows_job = jobs.get("windows-build") if isinstance(jobs, dict) else None
    steps = windows_job.get("steps") if isinstance(windows_job, dict) else None
    step_names = (
        [step.get("name") if isinstance(step, dict) else None for step in steps]
        if isinstance(steps, list)
        else []
    )
    required_names = [marker.removeprefix("- name: ") for marker in ordered_steps]
    positions = [
        step_names.index(name) if name in step_names else -1
        for name in required_names
    ]
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
        "iris-release-gate-temp-",
        "GITLEAKS_WINDOWS_X64_SHA256",
        "playwright install chromium",
        "Windows amd64 race detector",
        "go test -race -count=1 ./...",
        "iris-go-race-temp-",
        "--print-file-name libsynchronization.a",
        "libsynchronization.a",
    ):
        if marker not in ci:
            failures += 1
    return failures


def main() -> int:
    failures = 0
    native_powershell = windows_powershell_executable() is not None
    workflows = sorted((ROOT / ".github" / "workflows").glob("*.yml"))
    if not workflows:
        print("Workflow validation: FAIL [WF001] count=1")
        return 1

    for path in workflows:
        text = path.read_text(encoding="utf-8")
        try:
            document = yaml.load(text, Loader=UniqueKeySafeLoader)
        except yaml.YAMLError:
            failures += 1
            continue
        if not isinstance(document, dict) or not isinstance(document.get("jobs"), dict):
            failures += 1
            continue
        failures += invalid_job_env_contexts(document)
        failures += unpinned_action_failures(document)
        failures += windows_only_workflow_failures(document, text)
        if native_powershell:
            failures += powershell_step_failures(document)
        references = ACTION_REFERENCE.findall(text)
        pins = ACTION_PIN.findall(text)
        if len(references) != len(pins):
            failures += 1

    failures += release_policy_failures()

    if failures:
        print(f"Workflow validation: FAIL [WF001] count={failures}")
        return 1
    if not native_powershell:
        print("Workflow validation: NOT RUN [WINDOWS_POWERSHELL] (static checks PASS)")
        return 2
    print(f"Workflow validation: PASS files={len(workflows)}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
