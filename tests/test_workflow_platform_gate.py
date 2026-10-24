"""Workflow QA must not report Windows PowerShell absence as a YAML failure."""

from contextlib import redirect_stdout
from io import StringIO
from pathlib import Path
from shutil import copyfile
from tempfile import TemporaryDirectory
from unittest import TestCase
from unittest.mock import patch

import validate_workflows as workflow


class WorkflowPlatformGateTests(TestCase):

    def test_dependency_review_gate_cannot_be_removed_or_silenced(self):
        """Dependency review must execute and reject high-severity changes."""
        with TemporaryDirectory(prefix="iris-dependency-review-gate-") as directory:
            root = Path(directory)
            workflows = root / ".github" / "workflows"
            workflows.mkdir(parents=True)
            for filename in ("ci.yml", "codeql.yml", "dependency-review.yml"):
                copyfile(
                    workflow.ROOT / ".github" / "workflows" / filename,
                    workflows / filename,
                )
            path = workflows / "dependency-review.yml"
            original = path.read_text(encoding="utf-8")
            action = "      - name: Dependency review\n"
            job = "  dependency-review:\n"
            self.assertIn(action, original)
            self.assertIn(job, original)
            with patch.object(workflow, "ROOT", root):
                self.assertEqual(workflow.release_policy_failures(), 0)
                cases = (
                    (
                        "removed-action",
                        original.replace(
                            "        uses: actions/dependency-review-action@",
                            "        run: Write-Host 'skipped' # actions/dependency-review-action@",
                            1,
                        ),
                    ),
                    (
                        "step-condition",
                        original.replace(action, action + "        if: false\n", 1),
                    ),
                    (
                        "step-ignore-failure",
                        original.replace(
                            action, action + "        continue-on-error: true\n", 1,
                        ),
                    ),
                    (
                        "severity-downgrade",
                        original.replace(
                            "          fail-on-severity: high",
                            "          fail-on-severity: low",
                            1,
                        ),
                    ),
                    (
                        "job-condition",
                        original.replace(job, job + "    if: false\n", 1),
                    ),
                    (
                        "job-ignore-failure",
                        original.replace(
                            job, job + "    continue-on-error: true\n", 1,
                        ),
                    ),
                    (
                        "job-runner-downgrade",
                        original.replace("    runs-on: windows-2025", "    runs-on: windows-2022", 1),
                    ),
                )
                for case, modified in cases:
                    with self.subTest(case=case):
                        self.assertNotEqual(modified, original)
                        path.write_text(modified, encoding="utf-8")
                        self.assertGreater(workflow.release_policy_failures(), 0)
                path.write_text(original, encoding="utf-8")
                self.assertEqual(workflow.release_policy_failures(), 0)

    def test_dependency_review_requires_unfiltered_pull_request_trigger(self):
        """A review gate that never runs on relevant PRs is not a gate."""
        with TemporaryDirectory(prefix="iris-dependency-trigger-") as directory:
            root = Path(directory)
            workflows = root / ".github" / "workflows"
            workflows.mkdir(parents=True)
            for filename in ("ci.yml", "codeql.yml", "dependency-review.yml"):
                copyfile(workflow.ROOT / ".github" / "workflows" / filename, workflows / filename)
            path = workflows / "dependency-review.yml"
            original = path.read_text(encoding="utf-8")
            trigger = "on:\n  pull_request:\n"
            self.assertIn(trigger, original)
            cases = {
                "missing-pr": "on:\n  workflow_dispatch:\n",
                "target-only": "on:\n  pull_request_target:\n",
                "filtered-paths": "on:\n  pull_request:\n    paths: [README.md]\n",
                "filtered-paths-ignore": "on:\n  pull_request:\n    paths-ignore: [tools/**]\n",
                "filtered-branches": "on:\n  pull_request:\n    branches: [develop]\n",
                "filtered-types": "on:\n  pull_request:\n    types: [opened]\n",
            }
            with patch.object(workflow, "ROOT", root):
                self.assertEqual(workflow.release_policy_failures(), 0)
                for case, replacement in cases.items():
                    with self.subTest(case=case):
                        path.write_text(original.replace(trigger, replacement, 1), encoding="utf-8")
                        self.assertGreater(workflow.release_policy_failures(), 0)
                path.write_text(original, encoding="utf-8")
                self.assertEqual(workflow.release_policy_failures(), 0)

    def test_ci_trigger_removal_cannot_bypass_release_policy(self):
        """Disabled CI triggers must fail the static release policy."""
        with TemporaryDirectory(prefix="iris-ci-trigger-") as directory:
            root = Path(directory)
            workflows = root / ".github" / "workflows"
            workflows.mkdir(parents=True)
            for filename in ("ci.yml", "codeql.yml", "dependency-review.yml"):
                copyfile(workflow.ROOT / ".github" / "workflows" / filename, workflows / filename)
            ci_path = workflows / "ci.yml"
            original = ci_path.read_text(encoding="utf-8")
            trigger_block = (
                "on:\n"
                "  push:\n    branches: [main]\n"
                "  pull_request:\n    branches: [main]\n"
                "  workflow_dispatch:\n"
            )
            self.assertIn(trigger_block, original)
            with patch.object(workflow, "ROOT", root):
                self.assertEqual(workflow.release_policy_failures(), 0)
                for disabled in (
                    "on: []\n",
                    "on:\n  workflow_dispatch:\n",
                    "on:\n  push:\n    branches: [develop]\n"
                    "  pull_request:\n    branches: [main]\n"
                    "  workflow_dispatch:\n",
                ):
                    with self.subTest(disabled=disabled):
                        ci_path.write_text(
                            original.replace(trigger_block, disabled, 1),
                            encoding="utf-8",
                        )
                        self.assertGreater(workflow.release_policy_failures(), 0)

    def test_ci_path_filters_cannot_silence_required_windows_gates(self):
        """Path-filtered events can skip all mandatory CI for changed Go files."""
        with TemporaryDirectory(prefix="iris-ci-path-filter-") as directory:
            root = Path(directory)
            workflows = root / ".github" / "workflows"
            workflows.mkdir(parents=True)
            for filename in ("ci.yml", "codeql.yml", "dependency-review.yml"):
                copyfile(workflow.ROOT / ".github" / "workflows" / filename, workflows / filename)
            ci_path = workflows / "ci.yml"
            original = ci_path.read_text(encoding="utf-8")
            with patch.object(workflow, "ROOT", root):
                self.assertEqual(workflow.release_policy_failures(), 0)
                for event in ("push", "pull_request"):
                    for filter_name, pattern in (
                        ("paths", "['docs/**']"),
                        ("paths-ignore", "['**/*.go']"),
                    ):
                        with self.subTest(event=event, filter=filter_name):
                            event_block = f"  {event}:\n    branches: [main]\n"
                            self.assertIn(event_block, original)
                            modified = original.replace(
                                event_block,
                                event_block + f"    {filter_name}: {pattern}\n",
                                1,
                            )
                            ci_path.write_text(modified, encoding="utf-8")
                            self.assertGreater(workflow.release_policy_failures(), 0)
                ci_path.write_text(original, encoding="utf-8")
                self.assertEqual(workflow.release_policy_failures(), 0)

    def test_negative_branch_patterns_cannot_suppress_required_checks(self):
        """A later negated branch pattern overrides a preceding `main` match."""
        with TemporaryDirectory(prefix="iris-ci-negative-branch-") as directory:
            root = Path(directory)
            workflows = root / ".github" / "workflows"
            workflows.mkdir(parents=True)
            for filename in ("ci.yml", "codeql.yml", "dependency-review.yml"):
                copyfile(workflow.ROOT / ".github" / "workflows" / filename, workflows / filename)
            with patch.object(workflow, "ROOT", root):
                self.assertEqual(workflow.release_policy_failures(), 0)
                for filename in ("ci.yml", "codeql.yml"):
                    path = workflows / filename
                    original = path.read_text(encoding="utf-8")
                    for event in ("push", "pull_request"):
                        marker = f"  {event}:\n    branches: [main]\n"
                        self.assertIn(marker, original)
                        for negative in ("'!main'", "'!m*'"):
                            with self.subTest(workflow=filename, event=event, negative=negative):
                                path.write_text(
                                    original.replace(
                                        marker,
                                        f"  {event}:\n    branches: [main, {negative}]\n",
                                        1,
                                    ),
                                    encoding="utf-8",
                                )
                                self.assertGreater(workflow.release_policy_failures(), 0)
                        path.write_text(original, encoding="utf-8")
                self.assertEqual(workflow.release_policy_failures(), 0)

    def test_pr_event_type_filters_cannot_skip_required_checks(self):
        """PR event filters must not suppress CI or CodeQL on new commits."""
        with TemporaryDirectory(prefix="iris-pr-event-types-") as directory:
            root = Path(directory)
            workflows = root / ".github" / "workflows"
            workflows.mkdir(parents=True)
            for filename in ("ci.yml", "codeql.yml", "dependency-review.yml"):
                copyfile(workflow.ROOT / ".github" / "workflows" / filename, workflows / filename)
            with patch.object(workflow, "ROOT", root):
                self.assertEqual(workflow.release_policy_failures(), 0)
                for filename in ("ci.yml", "codeql.yml"):
                    path = workflows / filename
                    original = path.read_text(encoding="utf-8")
                    marker = "  pull_request:\n    branches: [main]\n"
                    self.assertIn(marker, original)
                    for types in ("[closed]", "[labeled]", "[opened]"):
                        with self.subTest(workflow=filename, types=types):
                            path.write_text(
                                original.replace(marker, marker + f"    types: {types}\n", 1),
                                encoding="utf-8",
                            )
                            self.assertGreater(workflow.release_policy_failures(), 0)
                    path.write_text(original, encoding="utf-8")
                self.assertEqual(workflow.release_policy_failures(), 0)

    def test_codeql_analyzer_job_cannot_ignore_failures(self):
        """Job-level continue-on-error must not mask failed Go/Python scans."""
        with TemporaryDirectory(prefix="iris-codeql-job-fail-open-") as directory:
            root = Path(directory)
            workflows = root / ".github" / "workflows"
            workflows.mkdir(parents=True)
            for filename in ("ci.yml", "codeql.yml", "dependency-review.yml"):
                copyfile(workflow.ROOT / ".github" / "workflows" / filename, workflows / filename)
            codeql_path = workflows / "codeql.yml"
            original = codeql_path.read_text(encoding="utf-8")
            marker = "  analyze:\n"
            self.assertIn(marker, original)
            with patch.object(workflow, "ROOT", root):
                self.assertEqual(workflow.release_policy_failures(), 0)
                for setting in ("true", "${{ true }}", "false"):
                    with self.subTest(setting=setting):
                        codeql_path.write_text(
                            original.replace(
                                marker,
                                marker + f"    continue-on-error: {setting}\n",
                                1,
                            ),
                            encoding="utf-8",
                        )
                        self.assertGreater(workflow.release_policy_failures(), 0)
                codeql_path.write_text(original, encoding="utf-8")
                self.assertEqual(workflow.release_policy_failures(), 0)

    def test_codeql_main_triggers_cannot_be_disabled_or_path_filtered(self):
        """CodeQL must scan every main push and pull request, not only cron."""
        with TemporaryDirectory(prefix="iris-codeql-trigger-policy-") as directory:
            root = Path(directory)
            workflows = root / ".github" / "workflows"
            workflows.mkdir(parents=True)
            for filename in ("ci.yml", "codeql.yml", "dependency-review.yml"):
                copyfile(workflow.ROOT / ".github" / "workflows" / filename, workflows / filename)
            codeql_path = workflows / "codeql.yml"
            original = codeql_path.read_text(encoding="utf-8")
            push = "  push:\n    branches: [main]\n"
            pull = "  pull_request:\n    branches: [main]\n"
            self.assertIn(push, original)
            self.assertIn(pull, original)
            cases = (
                ("remove-push", push, ""),
                ("remove-pr", pull, ""),
                ("wrong-push-branch", push, "  push:\n    branches: [develop]\n"),
                ("wrong-pr-branch", pull, "  pull_request:\n    branches: [develop]\n"),
                ("filter-push", push, push + "    paths: ['docs/**']\n"),
                ("filter-pr", pull, pull + "    paths-ignore: ['**/*.go']\n"),
            )
            with patch.object(workflow, "ROOT", root):
                self.assertEqual(workflow.release_policy_failures(), 0)
                for name, before, after in cases:
                    with self.subTest(name=name):
                        codeql_path.write_text(
                            original.replace(before, after, 1), encoding="utf-8"
                        )
                        self.assertGreater(workflow.release_policy_failures(), 0)
                codeql_path.write_text(original, encoding="utf-8")
                self.assertEqual(workflow.release_policy_failures(), 0)

    def test_required_windows_jobs_cannot_be_skipped_by_job_if(self):
        """A named required check is ineffective when its entire job is skipped."""
        with TemporaryDirectory(prefix="iris-required-ci-job-if-") as directory:
            root = Path(directory)
            workflows = root / ".github" / "workflows"
            workflows.mkdir(parents=True)
            for filename in ("ci.yml", "codeql.yml", "dependency-review.yml"):
                copyfile(workflow.ROOT / ".github" / "workflows" / filename, workflows / filename)
            ci_path = workflows / "ci.yml"
            original = ci_path.read_text(encoding="utf-8")
            with patch.object(workflow, "ROOT", root):
                self.assertEqual(workflow.release_policy_failures(), 0)
                for job in ("windows-quality", "windows-race", "windows-build"):
                    for condition in ("false", "'false'", "${{ false }}"):
                        with self.subTest(job=job, condition=condition):
                            header = f"  {job}:\n"
                            self.assertIn(header, original)
                            ci_path.write_text(
                                original.replace(header, header + f"    if: {condition}\n", 1),
                                encoding="utf-8",
                            )
                            self.assertGreater(workflow.release_policy_failures(), 0)

    def test_required_windows_jobs_cannot_ignore_errors(self):
        """Mandatory quality/race/build failures must propagate to CI status."""
        with TemporaryDirectory(prefix="iris-required-ci-fail-open-") as directory:
            root = Path(directory)
            workflows = root / ".github" / "workflows"
            workflows.mkdir(parents=True)
            for filename in ("ci.yml", "codeql.yml", "dependency-review.yml"):
                copyfile(workflow.ROOT / ".github" / "workflows" / filename, workflows / filename)
            ci_path = workflows / "ci.yml"
            original = ci_path.read_text(encoding="utf-8")
            with patch.object(workflow, "ROOT", root):
                self.assertEqual(workflow.release_policy_failures(), 0)
                for job in ("windows-quality", "windows-race", "windows-build"):
                    for setting in ("true", "${{ true }}"):
                        with self.subTest(job=job, setting=setting):
                            marker = f"  {job}:\n"
                            self.assertIn(marker, original)
                            ci_path.write_text(
                                original.replace(
                                    marker,
                                    marker + f"    continue-on-error: {setting}\n",
                                    1,
                                ),
                                encoding="utf-8",
                            )
                            self.assertGreater(workflow.release_policy_failures(), 0)
                ci_path.write_text(original, encoding="utf-8")
                self.assertEqual(workflow.release_policy_failures(), 0)

    def test_required_windows_job_steps_cannot_ignore_errors(self):
        """A failing required Windows step must not be marked successful."""
        with TemporaryDirectory(prefix="iris-required-ci-step-fail-open-") as directory:
            root = Path(directory)
            workflows = root / ".github" / "workflows"
            workflows.mkdir(parents=True)
            for filename in ("ci.yml", "codeql.yml", "dependency-review.yml"):
                copyfile(workflow.ROOT / ".github" / "workflows" / filename, workflows / filename)
            ci_path = workflows / "ci.yml"
            original = ci_path.read_text(encoding="utf-8")
            cases = (
                ("windows-quality", "Run canonical Windows release gate"),
                ("windows-race", "Windows amd64 race detector"),
                ("windows-build", "Windows Go tests"),
            )
            with patch.object(workflow, "ROOT", root):
                self.assertEqual(workflow.release_policy_failures(), 0)
                for job_id, step_name in cases:
                    for setting in ("true", "${{ true }}"):
                        with self.subTest(job=job_id, setting=setting):
                            header = f"  {job_id}:\n"
                            step = f"      - name: {step_name}\n"
                            self.assertIn(header, original)
                            job_start = original.index(header)
                            self.assertIn(step, original[job_start:])
                            step_start = original.index(step, job_start)
                            modified = (
                                original[:step_start]
                                + step
                                + f"        continue-on-error: {setting}\n"
                                + original[step_start + len(step):]
                            )
                            ci_path.write_text(modified, encoding="utf-8")
                            self.assertGreater(workflow.release_policy_failures(), 0)
                ci_path.write_text(original, encoding="utf-8")
                self.assertEqual(workflow.release_policy_failures(), 0)

    def test_required_windows_gate_steps_cannot_be_skipped(self):
        """A step-level `if` must not bypass a mandatory Windows gate."""
        with TemporaryDirectory(prefix="iris-ci-gate-step-condition-") as directory:
            root = Path(directory)
            workflows = root / ".github" / "workflows"
            workflows.mkdir(parents=True)
            for filename in ("ci.yml", "codeql.yml", "dependency-review.yml"):
                copyfile(workflow.ROOT / ".github" / "workflows" / filename, workflows / filename)
            ci_path = workflows / "ci.yml"
            original = ci_path.read_text(encoding="utf-8")
            cases = (
                ("windows-quality", "Run canonical Windows release gate"),
                ("windows-race", "Windows amd64 race detector"),
                ("windows-build", "Windows Go tests"),
            )
            with patch.object(workflow, "ROOT", root):
                self.assertEqual(workflow.release_policy_failures(), 0)
                for job_id, step_name in cases:
                    for condition in ("false", "${{ false }}"):
                        with self.subTest(job=job_id, condition=condition):
                            job_header = f"  {job_id}:\n"
                            step_header = f"      - name: {step_name}\n"
                            self.assertIn(job_header, original)
                            job_start = original.index(job_header)
                            self.assertIn(step_header, original[job_start:])
                            step_start = original.index(step_header, job_start)
                            modified = (
                                original[:step_start]
                                + step_header
                                + f"        if: {condition}\n"
                                + original[step_start + len(step_header):]
                            )
                            ci_path.write_text(modified, encoding="utf-8")
                            self.assertGreater(workflow.release_policy_failures(), 0)
                ci_path.write_text(original, encoding="utf-8")
                self.assertEqual(workflow.release_policy_failures(), 0)

    def test_required_ci_job_names_cannot_be_spoofed_by_comments(self):
        """Comments must not satisfy required Windows job identity checks."""
        with TemporaryDirectory(prefix="iris-ci-job-names-") as directory:
            root = Path(directory)
            workflows = root / ".github" / "workflows"
            workflows.mkdir(parents=True)
            for filename in ("ci.yml", "codeql.yml", "dependency-review.yml"):
                copyfile(workflow.ROOT / ".github" / "workflows" / filename, workflows / filename)
            ci_path = workflows / "ci.yml"
            original = ci_path.read_text(encoding="utf-8")
            with patch.object(workflow, "ROOT", root):
                self.assertEqual(workflow.release_policy_failures(), 0)
                for job_name in (
                    "Windows quality and security",
                    "Windows race detector",
                    "Native Windows Wails release matrix",
                ):
                    with self.subTest(job_name=job_name):
                        marker = "name: " + job_name
                        self.assertIn(marker, original)
                        ci_path.write_text(
                            original.replace(marker, "name: Disabled placeholder", 1)
                            + "\n# " + marker + "\n",
                            encoding="utf-8",
                        )
                        self.assertGreater(workflow.release_policy_failures(), 0)

    def test_required_windows_2025_runners_cannot_be_spoofed_by_comments(self):
        """The release gate must inspect actual runner values, not YAML comments."""
        with TemporaryDirectory(prefix="iris-windows-runner-policy-") as directory:
            root = Path(directory)
            workflows = root / ".github" / "workflows"
            workflows.mkdir(parents=True)
            filenames = ("ci.yml", "codeql.yml", "dependency-review.yml")
            for filename in filenames:
                copyfile(workflow.ROOT / ".github" / "workflows" / filename, workflows / filename)
            with patch.object(workflow, "ROOT", root):
                self.assertEqual(workflow.release_policy_failures(), 0)
                for filename in filenames:
                    with self.subTest(filename=filename):
                        path = workflows / filename
                        original = path.read_text(encoding="utf-8")
                        self.assertIn("runs-on: windows-2025", original)
                        path.write_text(
                            original.replace("runs-on: windows-2025", "runs-on: windows-2022")
                            + "\n# runs-on: windows-2025\n" * original.count("runs-on: windows-2025"),
                            encoding="utf-8",
                        )
                        self.assertGreater(workflow.release_policy_failures(), 0)
                        path.write_text(original, encoding="utf-8")

    def test_required_windows_jobs_cannot_borrow_runner_pins_from_decoys(self):
        """Each required gate itself must use windows-2025, not an unrelated job."""
        with TemporaryDirectory(prefix="iris-ci-runner-binding-") as directory:
            root = Path(directory)
            workflows = root / ".github" / "workflows"
            workflows.mkdir(parents=True)
            for filename in ("ci.yml", "codeql.yml", "dependency-review.yml"):
                copyfile(workflow.ROOT / ".github" / "workflows" / filename, workflows / filename)
            ci_path = workflows / "ci.yml"
            original = ci_path.read_text(encoding="utf-8")
            with patch.object(workflow, "ROOT", root):
                self.assertEqual(workflow.release_policy_failures(), 0)
                for job in ("windows-quality", "windows-race", "windows-build"):
                    with self.subTest(job=job):
                        header = f"  {job}:\n"
                        self.assertIn(header, original)
                        runner = original.index("    runs-on: windows-2025\n", original.index(header))
                        modified = (
                            original[:runner]
                            + "    runs-on: windows-2022\n"
                            + original[runner + len("    runs-on: windows-2025\n"):]
                            + "\n  decoy-runner:\n    runs-on: windows-2025\n"
                            + "    steps:\n      - run: Write-Output 'decoy'\n"
                        )
                        ci_path.write_text(modified, encoding="utf-8")
                        self.assertGreater(workflow.release_policy_failures(), 0)
                ci_path.write_text(original, encoding="utf-8")
                self.assertEqual(workflow.release_policy_failures(), 0)

    def test_codeql_analyze_runner_cannot_borrow_pin_from_decoy(self):
        """The actual Go/Python analyzer must run on pinned Windows 2025."""
        with TemporaryDirectory(prefix="iris-codeql-runner-binding-") as directory:
            root = Path(directory)
            workflows = root / ".github" / "workflows"
            workflows.mkdir(parents=True)
            for filename in ("ci.yml", "codeql.yml", "dependency-review.yml"):
                copyfile(workflow.ROOT / ".github" / "workflows" / filename, workflows / filename)
            codeql_path = workflows / "codeql.yml"
            original = codeql_path.read_text(encoding="utf-8")
            self.assertIn("  analyze:\n", original)
            self.assertIn("    runs-on: windows-2025\n", original)
            with patch.object(workflow, "ROOT", root):
                self.assertEqual(workflow.release_policy_failures(), 0)
                modified = (
                    original.replace("    runs-on: windows-2025\n", "    runs-on: windows-2022\n", 1)
                    + "\n  decoy-runner:\n    runs-on: windows-2025\n"
                    + "    steps:\n      - run: Write-Output 'decoy'\n"
                )
                codeql_path.write_text(modified, encoding="utf-8")
                self.assertGreater(workflow.release_policy_failures(), 0)
                codeql_path.write_text(original, encoding="utf-8")
                self.assertEqual(workflow.release_policy_failures(), 0)

    def test_codeql_go_build_gate_cannot_be_spoofed_or_disabled(self):
        """A commented-out or disabled Go build is not CodeQL coverage."""
        with TemporaryDirectory(prefix="iris-codeql-build-gate-") as directory:
            root = Path(directory)
            workflows = root / ".github" / "workflows"
            workflows.mkdir(parents=True)
            for filename in ("ci.yml", "codeql.yml", "dependency-review.yml"):
                copyfile(workflow.ROOT / ".github" / "workflows" / filename, workflows / filename)
            codeql_path = workflows / "codeql.yml"
            original = codeql_path.read_text(encoding="utf-8")
            start = original.index("      - name: Build Windows Go sources\n")
            end = original.index("      - name: Analyze\n", start)
            build_step = original[start:end]
            with patch.object(workflow, "ROOT", root):
                self.assertEqual(workflow.release_policy_failures(), 0)
                for case, replacement in (
                    (
                        "commented-out-build",
                        "      # Build Windows Go sources\n"
                        "      # iris-codeql-go-temp-\n",
                    ),
                    (
                        "disabled-build",
                        build_step.replace(
                            "        if: matrix.language == 'go'\n",
                            "        if: false\n",
                            1,
                        ),
                    ),
                    (
                        "no-build-commands",
                        "      - name: Build Windows Go sources\n"
                        "        if: matrix.language == 'go'\n"
                        "        shell: powershell\n"
                        "        run: Write-Output 'skipped'\n"
                        "      # iris-codeql-go-temp-\n",
                    ),
                ):
                    with self.subTest(case=case):
                        codeql_path.write_text(
                            original[:start] + replacement + original[end:],
                            encoding="utf-8",
                        )
                        self.assertGreater(workflow.release_policy_failures(), 0)
                        codeql_path.write_text(original, encoding="utf-8")

    def test_codeql_matrix_languages_cannot_be_spoofed_by_comments(self):
        """Both Go and Python must be enabled in the parsed CodeQL matrix."""
        with TemporaryDirectory(prefix="iris-codeql-matrix-") as directory:
            root = Path(directory)
            workflows = root / ".github" / "workflows"
            workflows.mkdir(parents=True)
            for filename in ("ci.yml", "codeql.yml", "dependency-review.yml"):
                copyfile(workflow.ROOT / ".github" / "workflows" / filename, workflows / filename)
            codeql_path = workflows / "codeql.yml"
            original = codeql_path.read_text(encoding="utf-8")
            marker = "language: [go, python]"
            self.assertIn(marker, original)
            with patch.object(workflow, "ROOT", root):
                self.assertEqual(workflow.release_policy_failures(), 0)
                for remaining in ("[python]", "[go]"):
                    with self.subTest(remaining=remaining):
                        codeql_path.write_text(
                            original.replace(marker, f"language: {remaining} # {marker}", 1),
                            encoding="utf-8",
                        )
                        self.assertGreater(workflow.release_policy_failures(), 0)
                codeql_path.write_text(original, encoding="utf-8")
                self.assertEqual(workflow.release_policy_failures(), 0)

    def test_codeql_analyze_action_cannot_be_removed_or_disabled(self):
        """CodeQL coverage requires an executable, unconditional analyze action."""
        with TemporaryDirectory(prefix="iris-codeql-analyze-action-") as directory:
            root = Path(directory)
            workflows = root / ".github" / "workflows"
            workflows.mkdir(parents=True)
            for filename in ("ci.yml", "codeql.yml", "dependency-review.yml"):
                copyfile(workflow.ROOT / ".github" / "workflows" / filename, workflows / filename)
            codeql_path = workflows / "codeql.yml"
            original = codeql_path.read_text(encoding="utf-8")
            action_step = (
                "      - name: Analyze\n"
                "        uses: github/codeql-action/analyze@"
                "db488ddef3bf6cb639b32c2e9a7c0a7ea8271d28 # v4.37.8\n"
            )
            self.assertIn(action_step, original)
            with patch.object(workflow, "ROOT", root):
                self.assertEqual(workflow.release_policy_failures(), 0)
                for case, replacement in (
                    ("removed", ""),
                    ("commented", "      # - name: Analyze\n"),
                    (
                        "replaced-by-checkout",
                        "      - name: Analyze\n"
                        "        uses: actions/checkout@"
                        "3d3c42e5aac5ba805825da76410c181273ba90b1\n",
                    ),
                    ("disabled", action_step + "        if: false\n"),
                ):
                    with self.subTest(case=case):
                        codeql_path.write_text(
                            original.replace(action_step, replacement, 1),
                            encoding="utf-8",
                        )
                        self.assertGreater(workflow.release_policy_failures(), 0)
                codeql_path.write_text(original, encoding="utf-8")
                self.assertEqual(workflow.release_policy_failures(), 0)

    def test_codeql_critical_steps_cannot_ignore_errors(self):
        """CodeQL init/build/analyze failures must fail the security job."""
        with TemporaryDirectory(prefix="iris-codeql-fail-open-") as directory:
            root = Path(directory)
            workflows = root / ".github" / "workflows"
            workflows.mkdir(parents=True)
            for filename in ("ci.yml", "codeql.yml", "dependency-review.yml"):
                copyfile(workflow.ROOT / ".github" / "workflows" / filename, workflows / filename)
            codeql_path = workflows / "codeql.yml"
            original = codeql_path.read_text(encoding="utf-8")
            with patch.object(workflow, "ROOT", root):
                self.assertEqual(workflow.release_policy_failures(), 0)
                for step_name in ("Initialize CodeQL", "Build Windows Go sources", "Analyze"):
                    for setting in ("true", "${{ true }}"):
                        with self.subTest(step=step_name, setting=setting):
                            marker = f"      - name: {step_name}\n"
                            self.assertIn(marker, original)
                            codeql_path.write_text(
                                original.replace(
                                    marker,
                                    marker + f"        continue-on-error: {setting}\n",
                                    1,
                                ),
                                encoding="utf-8",
                            )
                            self.assertGreater(workflow.release_policy_failures(), 0)
                codeql_path.write_text(original, encoding="utf-8")
                self.assertEqual(workflow.release_policy_failures(), 0)

    def test_codeql_init_action_cannot_be_removed_disabled_or_misconfigured(self):
        """CodeQL analysis requires an initialized database for each matrix language."""
        with TemporaryDirectory(prefix="iris-codeql-init-action-") as directory:
            root = Path(directory)
            workflows = root / ".github" / "workflows"
            workflows.mkdir(parents=True)
            for filename in ("ci.yml", "codeql.yml", "dependency-review.yml"):
                copyfile(workflow.ROOT / ".github" / "workflows" / filename, workflows / filename)
            codeql_path = workflows / "codeql.yml"
            original = codeql_path.read_text(encoding="utf-8")
            action_step = (
                "      - name: Initialize CodeQL\n"
                "        uses: github/codeql-action/init@"
                "db488ddef3bf6cb639b32c2e9a7c0a7ea8271d28 # v4.37.8\n"
                "        with:\n"
                "          languages: ${{ matrix.language }}\n"
            )
            self.assertIn(action_step, original)
            with patch.object(workflow, "ROOT", root):
                self.assertEqual(workflow.release_policy_failures(), 0)
                for case, replacement in (
                    ("removed", ""),
                    ("commented", "      # - name: Initialize CodeQL\n"),
                    (
                        "replaced-by-checkout",
                        "      - name: Initialize CodeQL\n"
                        "        uses: actions/checkout@"
                        "3d3c42e5aac5ba805825da76410c181273ba90b1\n",
                    ),
                    ("disabled", action_step + "        if: false\n"),
                    (
                        "wrong-language",
                        action_step.replace(
                            "languages: ${{ matrix.language }}",
                            "languages: python",
                        ),
                    ),
                ):
                    with self.subTest(case=case):
                        codeql_path.write_text(
                            original.replace(action_step, replacement, 1),
                            encoding="utf-8",
                        )
                        self.assertGreater(workflow.release_policy_failures(), 0)
                codeql_path.write_text(original, encoding="utf-8")
                self.assertEqual(workflow.release_policy_failures(), 0)

    def test_windows_build_setup_order_cannot_be_spoofed_by_comments(self):
        """Only real Windows build steps may satisfy the required setup sequence."""
        with TemporaryDirectory(prefix="iris-windows-build-order-") as directory:
            root = Path(directory)
            workflows = root / ".github" / "workflows"
            workflows.mkdir(parents=True)
            for filename in ("ci.yml", "codeql.yml", "dependency-review.yml"):
                copyfile(
                    workflow.ROOT / ".github" / "workflows" / filename,
                    workflows / filename,
                )
            ci_path = workflows / "ci.yml"
            original = ci_path.read_text(encoding="utf-8")
            windows_start = original.index("  windows-build:")
            prefix, windows = original[:windows_start], original[windows_start:]
            with patch.object(workflow, "ROOT", root):
                self.assertEqual(workflow.release_policy_failures(), 0)
                for step_name in (
                    "Setup Python 3.13",
                    "Setup pinned Go",
                    "Install pinned Windows release Go tools",
                    "Parse and self-test Windows PowerShell 5.1 tooling",
                ):
                    with self.subTest(step_name=step_name):
                        marker = "      - name: " + step_name
                        self.assertIn(marker, windows)
                        altered = windows.replace(
                            marker,
                            "      - name: Disabled setup placeholder\n"
                            "      # - name: " + step_name,
                            1,
                        )
                        ci_path.write_text(prefix + altered, encoding="utf-8")
                        self.assertGreater(workflow.release_policy_failures(), 0)

    def test_non_windows_reports_not_run_after_static_checks(self):
        output = StringIO()
        with (
            patch.object(
                workflow, "windows_powershell_executable", return_value=None
            ),
            patch.object(workflow, "powershell_step_failures") as parser,
            redirect_stdout(output),
        ):
            result = workflow.main()
        self.assertEqual(result, 2)
        self.assertIn("NOT RUN [WINDOWS_POWERSHELL]", output.getvalue())
        self.assertIn("static checks PASS", output.getvalue())
        parser.assert_not_called()

    def test_static_violation_still_fails_without_powershell(self):
        output = StringIO()
        with (
            patch.object(
                workflow, "windows_powershell_executable", return_value=None
            ),
            patch.object(workflow, "release_policy_failures", return_value=1),
            redirect_stdout(output),
        ):
            result = workflow.main()
        self.assertEqual(result, 1)
        self.assertIn("FAIL [WF001]", output.getvalue())

    def test_native_powershell_path_still_runs_full_validation(self):
        output = StringIO()
        with (
            patch.object(
                workflow,
                "windows_powershell_executable",
                return_value=Path("C:/Windows/powershell.exe"),
            ),
            patch.object(
                workflow, "powershell_step_failures", return_value=0
            ) as parser,
            redirect_stdout(output),
        ):
            result = workflow.main()
        self.assertEqual(result, 0)
        self.assertIn("Workflow validation: PASS", output.getvalue())
        self.assertEqual(parser.call_count, 4)

    def test_duplicate_yaml_keys_cannot_hide_workflow_jobs(self):
        with TemporaryDirectory(prefix="iris-duplicate-workflow-") as directory:
            root = Path(directory)
            workflows = root / ".github" / "workflows"
            workflows.mkdir(parents=True)
            (workflows / "probe.yml").write_text(
                "name: Duplicate jobs fixture\n"
                "jobs:\n"
                "  first:\n"
                "    runs-on: windows-2025\n"
                "    steps: [{uses: actions/checkout@" + "a" * 40 + "}]\n"
                "jobs:\n"
                "  second:\n"
                "    runs-on: windows-2025\n"
                "    steps: [{uses: actions/checkout@" + "b" * 40 + "}]\n",
                encoding="utf-8",
            )
            output = StringIO()
            with (
                patch.object(workflow, "ROOT", root),
                patch.object(workflow, "release_policy_failures", return_value=0),
                patch.object(
                    workflow, "windows_powershell_executable", return_value=None
                ),
                redirect_stdout(output),
            ):
                result = workflow.main()
            self.assertEqual(result, 1, output.getvalue())
            self.assertIn("FAIL [WF001]", output.getvalue())

    def test_flow_style_action_cannot_bypass_commit_pin_gate(self):
        with TemporaryDirectory(prefix="iris-action-pin-") as directory:
            root = Path(directory)
            workflows = root / ".github" / "workflows"
            workflows.mkdir(parents=True)
            fixture = workflows / "probe.yml"
            for action, expected in (
                ("actions/checkout@main", 1),
                ("actions/checkout@1234567890abcdef1234567890abcdef12345678", 2),
            ):
                with self.subTest(action=action):
                    fixture.write_text(
                        "name: Pin fixture\n"
                        "jobs:\n"
                        "  build:\n"
                        "    runs-on: windows-2025\n"
                        "    steps:\n"
                        f"      - {{uses: {action}}}\n",
                        encoding="utf-8",
                    )
                    output = StringIO()
                    with (
                        patch.object(workflow, "ROOT", root),
                        patch.object(
                            workflow, "release_policy_failures", return_value=0
                        ),
                        patch.object(
                            workflow, "windows_powershell_executable", return_value=None
                        ),
                        redirect_stdout(output),
                    ):
                        result = workflow.main()
                    self.assertEqual(result, expected, output.getvalue())

    def test_malformed_steps_cannot_bypass_static_action_checks(self):
        with TemporaryDirectory(prefix="iris-malformed-workflow-steps-") as directory:
            root = Path(directory)
            workflows = root / ".github" / "workflows"
            workflows.mkdir(parents=True)
            fixture = workflows / "probe.yml"
            for steps, expected in (
                ("    steps: {uses: actions/checkout@main}\n", 1),
                ("    steps: [not-a-step-mapping]\n", 1),
                (
                    "    steps: [{uses: actions/checkout@" + "a" * 40 + "}]\n",
                    2,
                ),
            ):
                with self.subTest(steps=steps):
                    fixture.write_text(
                        "name: Steps shape fixture\n"
                        "jobs:\n"
                        "  build:\n"
                        "    runs-on: windows-2025\n"
                        + steps,
                        encoding="utf-8",
                    )
                    output = StringIO()
                    with (
                        patch.object(workflow, "ROOT", root),
                        patch.object(workflow, "release_policy_failures", return_value=0),
                        patch.object(
                            workflow, "windows_powershell_executable", return_value=None
                        ),
                        redirect_stdout(output),
                    ):
                        result = workflow.main()
                    self.assertEqual(result, expected, output.getvalue())

    def test_flow_style_posix_shell_cannot_bypass_windows_gate(self):
        with TemporaryDirectory(prefix="iris-flow-shell-") as directory:
            root = Path(directory)
            workflows = root / ".github" / "workflows"
            workflows.mkdir(parents=True)
            fixture = workflows / "probe.yml"
            for extra, expected in (
                ("    steps: [{shell: bash, run: echo hello}]\n", 1),
                ("    steps: [{shell: sh, run: echo hello}]\n", 1),
                (
                    "    defaults: {run: {shell: bash}}\n"
                    "    steps: [{run: echo hello}]\n",
                    1,
                ),
                ("    steps:\n      - shell: bash\n        run: echo hello\n", 1),
                ("    steps: [{shell: pwsh, run: echo hello}]\n", 2),
            ):
                with self.subTest(extra=extra):
                    fixture.write_text(
                        "name: Shell fixture\n"
                        "jobs:\n"
                        "  build:\n"
                        "    runs-on: windows-2025\n"
                        + extra,
                        encoding="utf-8",
                    )
                    output = StringIO()
                    with (
                        patch.object(workflow, "ROOT", root),
                        patch.object(workflow, "release_policy_failures", return_value=0),
                        patch.object(
                            workflow, "windows_powershell_executable", return_value=None
                        ),
                        redirect_stdout(output),
                    ):
                        result = workflow.main()
                    self.assertEqual(result, expected, output.getvalue())

    def test_custom_posix_shell_command_cannot_bypass_windows_gate(self):
        """GitHub Actions accepts shell command templates, not just shell names."""
        with TemporaryDirectory(prefix="iris-custom-shell-") as directory:
            root = Path(directory)
            workflows = root / ".github" / "workflows"
            workflows.mkdir(parents=True)
            fixture = workflows / "probe.yml"
            for shell, expected in (
                ("bash -e {0}", 1),
                ("sh -e {0}", 1),
                ("bash --noprofile --norc -eo pipefail {0}", 1),
                ("bash.exe -e {0}", 1),
                ("pwsh -File {0}", 2),
                ("powershell -File {0}", 2),
            ):
                with self.subTest(shell=shell):
                    fixture.write_text(
                        "name: Custom shell fixture\n"
                        "jobs:\n"
                        "  build:\n"
                        "    runs-on: windows-2025\n"
                        f"    steps: [{{shell: '{shell}', run: 'echo hello'}}]\n",
                        encoding="utf-8",
                    )
                    output = StringIO()
                    with (
                        patch.object(workflow, "ROOT", root),
                        patch.object(workflow, "release_policy_failures", return_value=0),
                        patch.object(
                            workflow, "windows_powershell_executable", return_value=None
                        ),
                        redirect_stdout(output),
                    ):
                        result = workflow.main()
                    self.assertEqual(result, expected, output.getvalue())

    def test_custom_posix_shell_defaults_cannot_bypass_windows_gate(self):
        with TemporaryDirectory(prefix="iris-custom-shell-defaults-") as directory:
            root = Path(directory)
            workflows = root / ".github" / "workflows"
            workflows.mkdir(parents=True)
            fixture = workflows / "probe.yml"
            for defaults in (
                "defaults: {run: {shell: 'bash -e {0}'}}\n",
                "jobs:\n  build:\n    defaults: {run: {shell: 'sh -e {0}'}}\n",
            ):
                with self.subTest(defaults=defaults):
                    if defaults.startswith("jobs:"):
                        content = (
                            "name: Defaults fixture\n" + defaults
                            + "    runs-on: windows-2025\n"
                            + "    steps: [{run: 'echo hello'}]\n"
                        )
                    else:
                        content = (
                            "name: Defaults fixture\n" + defaults
                            + "jobs:\n  build:\n    runs-on: windows-2025\n"
                            + "    steps: [{run: 'echo hello'}]\n"
                        )
                    fixture.write_text(content, encoding="utf-8")
                    output = StringIO()
                    with (
                        patch.object(workflow, "ROOT", root),
                        patch.object(workflow, "release_policy_failures", return_value=0),
                        patch.object(workflow, "windows_powershell_executable", return_value=None),
                        redirect_stdout(output),
                    ):
                        result = workflow.main()
                    self.assertEqual(result, 1, output.getvalue())

    def test_codeql_job_level_condition_cannot_disable_security_gate(self):
        """A conditional CodeQL job can silently skip both language scans."""
        with TemporaryDirectory(prefix="iris-codeql-job-condition-") as directory:
            root = Path(directory)
            workflows = root / ".github" / "workflows"
            workflows.mkdir(parents=True)
            for filename in ("ci.yml", "codeql.yml", "dependency-review.yml"):
                copyfile(workflow.ROOT / ".github" / "workflows" / filename, workflows / filename)
            codeql_path = workflows / "codeql.yml"
            original = codeql_path.read_text(encoding="utf-8")
            self.assertIn("  analyze:\n", original)
            with patch.object(workflow, "ROOT", root):
                self.assertEqual(workflow.release_policy_failures(), 0)
                for condition in (
                    "false",
                    "${{ false }}",
                    "${{ github.event_name == 'never' }}",
                ):
                    with self.subTest(condition=condition):
                        codeql_path.write_text(
                            original.replace(
                                "  analyze:\n", f"  analyze:\n    if: {condition}\n", 1
                            ),
                            encoding="utf-8",
                        )
                        self.assertGreater(workflow.release_policy_failures(), 0)
                codeql_path.write_text(original, encoding="utf-8")
                self.assertEqual(workflow.release_policy_failures(), 0)

    def test_required_windows_gate_steps_must_execute_real_commands(self):
        """A named no-op step cannot inherit proof from commented-out commands."""
        with TemporaryDirectory(prefix="iris-ci-gate-noop-") as directory:
            root = Path(directory)
            workflows = root / ".github" / "workflows"
            workflows.mkdir(parents=True)
            for filename in ("ci.yml", "codeql.yml", "dependency-review.yml"):
                copyfile(workflow.ROOT / ".github" / "workflows" / filename, workflows / filename)
            ci_path = workflows / "ci.yml"
            original = ci_path.read_text(encoding="utf-8")
            with patch.object(workflow, "ROOT", root):
                self.assertEqual(workflow.release_policy_failures(), 0)
                for name in (
                    "Run canonical Windows release gate",
                    "Windows amd64 race detector",
                    "Windows Go tests",
                ):
                    with self.subTest(step=name):
                        header = f"      - name: {name}\n"
                        start = original.index(header)
                        run_start = original.index("        run: |\n", start)
                        script_start = run_start + len("        run: |\n")
                        ends = [
                            pos for pos in (
                                original.find("\n      - name:", script_start),
                                original.find("\n  windows-", script_start),
                            ) if pos >= 0
                        ]
                        end = min(ends) if ends else len(original)
                        old_script = original[script_start:end]
                        # Keep all original text markers as YAML comments; only
                        # the actual executable step is replaced by a no-op.
                        modified = (
                            original[:run_start]
                            + "        run: Write-Host 'skipped'\n"
                            + original[end:]
                            + "\n# " + old_script.replace("\n", "\n# ") + "\n"
                        )
                        ci_path.write_text(modified, encoding="utf-8")
                        self.assertGreater(workflow.release_policy_failures(), 0)
                ci_path.write_text(original, encoding="utf-8")
                self.assertEqual(workflow.release_policy_failures(), 0)

    def test_codeql_matrix_exclude_cannot_silence_language_scans(self):
        """An exclusion can remove Go/Python jobs despite intact language labels."""
        with TemporaryDirectory(prefix="iris-codeql-matrix-exclude-") as directory:
            root = Path(directory)
            workflows = root / ".github" / "workflows"
            workflows.mkdir(parents=True)
            for filename in ("ci.yml", "codeql.yml", "dependency-review.yml"):
                copyfile(workflow.ROOT / ".github" / "workflows" / filename, workflows / filename)
            codeql_path = workflows / "codeql.yml"
            original = codeql_path.read_text(encoding="utf-8")
            marker = "        language: [go, python]\n"
            self.assertIn(marker, original)
            with patch.object(workflow, "ROOT", root):
                self.assertEqual(workflow.release_policy_failures(), 0)
                for case, languages in (
                    ("exclude-go", ("go",)),
                    ("exclude-python", ("python",)),
                    ("exclude-both", ("go", "python")),
                ):
                    with self.subTest(case=case):
                        exclusion = "        exclude:\n" + "".join(
                            f"          - language: {language}\n" for language in languages
                        )
                        codeql_path.write_text(
                            original.replace(marker, marker + exclusion, 1),
                            encoding="utf-8",
                        )
                        self.assertGreater(workflow.release_policy_failures(), 0)
                codeql_path.write_text(original, encoding="utf-8")
                self.assertEqual(workflow.release_policy_failures(), 0)
