"""Workflow QA must not report Windows PowerShell absence as a YAML failure."""

from contextlib import redirect_stdout
from io import StringIO
from pathlib import Path
from tempfile import TemporaryDirectory
from unittest import TestCase
from unittest.mock import patch

import validate_workflows as workflow


class WorkflowPlatformGateTests(TestCase):
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
