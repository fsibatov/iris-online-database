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



def ci_trigger_failures(ci: str, *, require_dispatch: bool = True) -> int:
    """Require unfiltered main push/PR; CI also requires manual dispatch."""
    try:
        # BaseLoader preserves the literal `on` key (SafeLoader treats it as True).
        document = yaml.load(ci, Loader=yaml.BaseLoader)
    except yaml.YAMLError:
        return 1
    if not isinstance(document, dict):
        return 1
    triggers = document.get("on")
    if not isinstance(triggers, dict) or (
        require_dispatch and "workflow_dispatch" not in triggers
    ):
        return 1
    for event in ("push", "pull_request"):
        settings = triggers.get(event)
        if not isinstance(settings, dict):
            return 1
        branches = settings.get("branches")
        if not isinstance(branches, list) or "main" not in branches:
            return 1
        # GitHub applies branch patterns in order: a later `!main` (or `!m*`)
        # excludes main despite the earlier literal match. Mandatory checks
        # must not rely on negated patterns whose order changes coverage.
        if any(not isinstance(branch, str) or branch.startswith("!") for branch in branches):
            return 1
        # Path filters can suppress mandatory Windows CI for otherwise valid
        # pushes and pull requests (including changes to Go/security code).
        if "paths" in settings or "paths-ignore" in settings:
            return 1
        # A PR `types` filter can omit synchronize/reopened and skip checks
        # on updated commits even when the main branch filter is intact.
        if event == "pull_request" and "types" in settings:
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
    # CodeQL must scan all main pushes/PRs, not only a scheduled subset.
    failures += ci_trigger_failures(codeql, require_dispatch=False)
    # Dependency review must run on every pull request. A present action/job
    # is not a security gate if its workflow is manual-only or PR-filtered.
    try:
        dependency_events = yaml.load(dependency, Loader=yaml.BaseLoader)
    except yaml.YAMLError:
        dependency_events = None
    triggers = dependency_events.get("on") if isinstance(dependency_events, dict) else None
    if (
        not isinstance(triggers, dict)
        or "pull_request" not in triggers
        or triggers["pull_request"] not in ("", None, {})
    ):
        failures += 1

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
    # Job-level continue-on-error can report a failed mandatory Windows gate
    # as successful, even when its runner and steps are otherwise correct.
    if isinstance(jobs, dict) and any(
        isinstance(jobs.get(job_id), dict)
        and "continue-on-error" in jobs[job_id]
        for job_id in ("windows-quality", "windows-race", "windows-build")
    ):
        failures += 1
    # A required job also fails open when an individual gate step has
    # `continue-on-error`, even if the job itself propagates other failures.
    if isinstance(jobs, dict):
        for job_id in ("windows-quality", "windows-race", "windows-build"):
            job = jobs.get(job_id)
            steps = job.get("steps") if isinstance(job, dict) else None
            if not isinstance(steps, list) or not steps or any(
                not isinstance(step, dict) or "continue-on-error" in step
                for step in steps
            ):
                failures += 1
    # A step-level `if` can skip the actual quality, race, or Go test gate
    # while the enclosing mandatory Windows job still reports success.
    required_gate_steps = {
        "windows-quality": (
            "Run canonical Windows release gate",
            (
                "powershell -NoLogo -NoProfile -ExecutionPolicy Bypass -File .\\IrisTools.ps1 -Action Test",
                "if ($gateExitCode -ne 0) { exit $gateExitCode }",
            ),
        ),
        "windows-race": (
            "Windows amd64 race detector",
            (
                "go test -race -count=1 ./...",
                "if ($raceExitCode -ne 0) { exit $raceExitCode }",
            ),
        ),
        "windows-build": (
            "Windows Go tests",
            (
                "go test -count=1 ./...",
                "if ($goTestExitCode -ne 0) { exit $goTestExitCode }",
            ),
        ),
    }
    for job_id, (gate_name, commands) in required_gate_steps.items():
        job = jobs.get(job_id) if isinstance(jobs, dict) else None
        steps = job.get("steps") if isinstance(job, dict) else None
        gate_steps = (
            [step for step in steps if isinstance(step, dict)
             and step.get("name") == gate_name]
            if isinstance(steps, list) else []
        )
        if len(gate_steps) != 1:
            failures += 1
            continue
        gate = gate_steps[0]
        script = gate.get("run")
        if (
            "if" in gate or "uses" in gate
            or gate.get("shell") != "powershell"
            or not isinstance(script, str)
        ):
            failures += 1
            continue
        # Text markers in comments or unrelated steps do not execute the gate.
        active_lines = [
            line.strip() for line in script.splitlines()
            if line.strip() and not line.lstrip().startswith("#")
        ]
        if any(command not in active_lines for command in commands):
            failures += 1
    if pinned_windows_runner_count(ci_document) < 3:
        failures += 1
    if (
        pinned_windows_runner_count(codeql_document) < 1
        or pinned_windows_runner_count(dependency_document) < 1
    ):
        failures += 1
    # A pinned Windows runner alone does not prove dependency review ran.
    dependency_jobs = (
        dependency_document.get("jobs")
        if isinstance(dependency_document, dict) else None
    )
    dependency_job = (
        dependency_jobs.get("dependency-review")
        if isinstance(dependency_jobs, dict) else None
    )
    dependency_steps = (
        dependency_job.get("steps") if isinstance(dependency_job, dict) else None
    )
    review_actions = (
        [
            step
            for step in dependency_steps
            if isinstance(step, dict)
            and isinstance(step.get("uses"), str)
            and re.fullmatch(
                r"actions/dependency-review-action@[0-9a-f]{40}", step["uses"]
            )
        ]
        if isinstance(dependency_steps, list)
        else []
    )
    if (
        not isinstance(dependency_job, dict)
        or dependency_job.get("runs-on") != "windows-2025"
        or "if" in dependency_job
        or "continue-on-error" in dependency_job
        or len(review_actions) != 1
        or "if" in review_actions[0]
        or "continue-on-error" in review_actions[0]
        or not isinstance(review_actions[0].get("with"), dict)
        or review_actions[0]["with"].get("fail-on-severity") != "high"
        # `warn-only` turns high-severity findings into non-blocking warnings.
        # Reject even expression-valued overrides: their result is runtime-dependent.
        or "warn-only" in review_actions[0]["with"]
    ):
        failures += 1
    if (
        "name: Analyze (${{ matrix.language }})" not in codeql
        or "language: [go, python]" not in codeql
    ):
        failures += 1
    codeql_jobs = codeql_document.get("jobs") if isinstance(codeql_document, dict) else None
    analyze = codeql_jobs.get("analyze") if isinstance(codeql_jobs, dict) else None
    # SARIF publication requires security-events: write in the effective
    # permissions. A job-level permissions map replaces the workflow map.
    effective_permissions = (
        analyze.get("permissions")
        if isinstance(analyze, dict) and "permissions" in analyze
        else codeql_document.get("permissions")
        if isinstance(codeql_document, dict)
        else None
    )
    if (
        not isinstance(effective_permissions, dict)
        or effective_permissions.get("security-events") != "write"
    ):
        failures += 1
    # Bind the pinned runner to the actual analyzer, not an unrelated job.
    if not isinstance(analyze, dict) or analyze.get("runs-on") != "windows-2025":
        failures += 1
    # Job-level conditions can skip both language scans despite intact steps.
    if isinstance(analyze, dict) and "if" in analyze:
        failures += 1
    # A job-level override masks failed scans even when each CodeQL step fails.
    if isinstance(analyze, dict) and "continue-on-error" in analyze:
        failures += 1
    # Text markers can be preserved in comments while a real matrix entry
    # silently drops Go or Python from CodeQL coverage.
    strategy = analyze.get("strategy") if isinstance(analyze, dict) else None
    matrix = strategy.get("matrix") if isinstance(strategy, dict) else None
    languages = matrix.get("language") if isinstance(matrix, dict) else None
    # Matrix exclusions can silently suppress either mandatory language scan
    # while leaving the visible language list unchanged.
    if not (
        isinstance(matrix, dict)
        and "exclude" not in matrix
        and isinstance(languages, list)
        and len(languages) == 2
        and all(isinstance(language, str) for language in languages)
        and set(languages) == {"go", "python"}
    ):
        failures += 1
    # The Go scan must use the declared toolchain, not an arbitrary runner Go.
    # A missing, conditional or fail-open setup-go step breaks that guarantee.
    codeql_steps = analyze.get("steps") if isinstance(analyze, dict) else None
    go_setup_steps = (
        [step for step in codeql_steps if isinstance(step, dict)
         and isinstance(step.get("uses"), str)
         and re.fullmatch(r"actions/setup-go@[0-9a-f]{40}", step["uses"])]
        if isinstance(codeql_steps, list) else []
    )
    if len(go_setup_steps) != 1 or not (
        go_setup_steps[0].get("if") == "matrix.language == 'go'"
        and "continue-on-error" not in go_setup_steps[0]
        and isinstance(go_setup_steps[0].get("with"), dict)
        and go_setup_steps[0]["with"].get("go-version-file") == ".go-version"
    ):
        failures += 1
    go_build_steps = (
        [step for step in codeql_steps if isinstance(step, dict)
         and step.get("name") == "Build Windows Go sources"]
        if isinstance(codeql_steps, list) else []
    )
    # A substring in a comment or Write-Output decoy is not an executable
    # Go test/build. Require the expected commands on active PowerShell lines.
    go_build_script = go_build_steps[0].get("run") if len(go_build_steps) == 1 else None
    active_go_lines = (
        [line.strip() for line in go_build_script.splitlines()
         if line.strip() and not line.lstrip().startswith("#")]
        if isinstance(go_build_script, str) else []
    )
    if len(go_build_steps) != 1 or not (
        go_build_steps[0].get("if") == "matrix.language == 'go'"
        and go_build_steps[0].get("shell") == "powershell"
        and any("iris-codeql-go-temp-" in line for line in active_go_lines)
        and "go test -count=1 ./..." in active_go_lines
        and any(line.startswith("go build -trimpath -o ") and line.endswith(" .")
                for line in active_go_lines)
    ):
        failures += 1
    # A named CodeQL job without its actual analyze action never publishes
    # security findings. Comments and unrelated pinned actions do not count.
    analyze_actions = (
        [step for step in codeql_steps if isinstance(step, dict)
         and isinstance(step.get("uses"), str)
         and re.fullmatch(r"github/codeql-action/analyze@[0-9a-f]{40}", step["uses"])]
        if isinstance(codeql_steps, list) else []
    )
    if len(analyze_actions) != 1 or "if" in analyze_actions[0]:
        failures += 1
    elif isinstance(analyze_actions[0].get("with"), dict) and (
        "upload" in analyze_actions[0]["with"]
    ):
        # CodeQL must publish SARIF. Any upload override can disable it,
        # including an expression that evaluates to false at runtime.
        failures += 1
    # Without an unconditional init action for each matrix language, CodeQL
    # cannot establish the database required by the analyze step.
    init_actions = (
        [
            step
            for step in codeql_steps
            if isinstance(step, dict)
            and isinstance(step.get("uses"), str)
            and re.fullmatch(r"github/codeql-action/init@[0-9a-f]{40}", step["uses"])
        ]
        if isinstance(codeql_steps, list)
        else []
    )
    if (
        len(init_actions) != 1
        or "if" in init_actions[0]
        or not isinstance(init_actions[0].get("with"), dict)
        or init_actions[0]["with"].get("languages") != "${{ matrix.language }}"
    ):
        failures += 1

    # These three mandatory CodeQL stages must propagate failures to the job.
    # Even an expression-valued continue-on-error can silently mask a scan.
    if any(
        "continue-on-error" in step
        for step in (*go_build_steps, *init_actions, *analyze_actions)
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
