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
