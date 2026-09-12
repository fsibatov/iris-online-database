from __future__ import annotations

import contextlib
import io
import os
import subprocess
import tempfile
import unittest
from pathlib import Path
from unittest import mock

import validate_workflows
import yaml

ROOT = Path(__file__).resolve().parents[1]
PIP_AUDIT_PROBE = r"""param([string]$SourcePath)
$ErrorActionPreference = "Stop"
$Tokens = $null
$Errors = $null
$Ast = [System.Management.Automation.Language.Parser]::ParseFile(
    $SourcePath, [ref]$Tokens, [ref]$Errors
)
if ($Errors.Count) { throw "Source parsing failed" }
$Function = $Ast.Find({
    param($Node)
    $Node -is [System.Management.Automation.Language.FunctionDefinitionAst] -and
    $Node.Name -eq "Invoke-PipAudit"
}, $true)
if ($null -eq $Function) { throw "Audit function missing" }
. ([ScriptBlock]::Create($Function.Extent.Text))
$AuditEnv = Join-Path ([IO.Path]::GetTempPath()) "iris-probe-environment"
$script:Caches = New-Object System.Collections.Generic.List[string]
function Invoke-Checked {
    param([string]$File, [string[]]$Arguments, [int]$TimeoutSeconds)
    if ($File -ne (Join-Path $AuditEnv "Scripts\pip-audit.exe")) {
        throw "Incorrect scanner"
    }
    if ($TimeoutSeconds -ne 600 -or $Arguments.Count -ne 3 -or
        $Arguments[0] -ne "--local" -or $Arguments[1] -ne "--cache-dir") {
        throw "Audit arguments changed"
    }
    $Cache = $Arguments[2]
    if (-not (Test-Path -LiteralPath $Cache -PathType Container)) {
        throw "Cache must exist before the scanner runs"
    }
    if ([IO.Path]::GetFullPath((Split-Path -Parent $Cache)).TrimEnd('\', '/') -ne
        [IO.Path]::GetFullPath([IO.Path]::GetTempPath()).TrimEnd('\', '/')) {
        throw "Cache must use the current process temporary directory"
    }
    if ($script:Caches.Contains($Cache)) { throw "Cache reused between runs" }
    $script:Caches.Add($Cache)
    [IO.File]::WriteAllText((Join-Path $Cache "probe"), "cache entry")
    if ($script:FailAudit) { throw "scanner failure" }
}
foreach ($Fail in @($false, $true, $false)) {
    $script:FailAudit = $Fail
    $Caught = $null
    try { Invoke-PipAudit } catch { $Caught = $_.Exception.Message }
    if ($Fail -and $Caught -ne "scanner failure") { throw "Scanner failure lost" }
    if (-not $Fail -and $null -ne $Caught) { throw $Caught }
    foreach ($Cache in $script:Caches) {
        if (Test-Path -LiteralPath $Cache) { throw "Cache was not removed" }
    }
}
if ($script:Caches.Count -ne 3) { throw "Scanner did not run three times" }
Write-Output "Temporary audit cache: PASS"
"""


def workflow(script: str) -> dict:
    return {
        "jobs": {
            "analyze": {
                "steps": [
                    {"uses": "fixture/action@" + "a" * 40},
                    {
                        "name": "Build Windows Go sources",
                        "shell": "pwsh",
                        "run": script,
                    },
                ]
            }
        }
    }


class WorkflowValidationTests(unittest.TestCase):
    def test_missing_parser_fails_without_running_another_shell(self):
        output = io.StringIO()
        with (
            mock.patch.dict(os.environ, {}, clear=True),
            mock.patch.object(validate_workflows.subprocess, "run") as run,
            contextlib.redirect_stdout(output),
        ):
            failures = validate_workflows.powershell_step_failures(
                workflow("go build .")
            )
        self.assertEqual(failures, 1)
        run.assert_not_called()
        self.assertIn("Windows PowerShell 5.1 parser is unavailable", output.getvalue())

    def test_diagnostic_identifies_source_and_preserves_utf8(self):
        diagnostic = "line=2 column=27 [MissingArgument] Отсутствует аргумент."

        def run(command, **kwargs):
            script = Path(command[-1]).read_bytes()
            self.assertTrue(script.startswith(b"\xef\xbb\xbf"))
            self.assertIn(b"GITHUB_EXPRESSION", script)
            self.assertNotIn(b"${{", script)
            self.assertEqual(kwargs["encoding"], "utf-8")
            return subprocess.CompletedProcess(command, 1, diagnostic)

        output = io.StringIO()
        with (
            mock.patch.dict(os.environ, {"SYSTEMROOT": "fixture-windows"}),
            mock.patch.object(Path, "is_file", return_value=True),
            mock.patch.object(validate_workflows.subprocess, "run", side_effect=run),
            contextlib.redirect_stdout(output),
        ):
            failures = validate_workflows.powershell_step_failures(
                workflow('Write-Output "${{ github.sha }}"\ngo build -tags=a,b .'),
                ".github/workflows/codeql.yml",
            )
        self.assertEqual(failures, 1)
        self.assertIn(
            ".github/workflows/codeql.yml: job=analyze step=2 (Build Windows Go sources)",
            output.getvalue(),
        )
        self.assertIn(diagnostic, output.getvalue())

    def test_parser_errors_never_become_success(self):
        for result in (
            subprocess.CompletedProcess([], 1, ""),
            OSError("Cannot start parser"),
            subprocess.TimeoutExpired("powershell.exe", 30),
        ):
            with (
                self.subTest(result=type(result).__name__),
                mock.patch.dict(os.environ, {"SYSTEMROOT": "fixture-windows"}),
                mock.patch.object(Path, "is_file", return_value=True),
                mock.patch.object(validate_workflows.subprocess, "run") as run,
                contextlib.redirect_stdout(io.StringIO()) as output,
            ):
                if isinstance(result, Exception):
                    run.side_effect = result
                else:
                    run.return_value = result
                failures = validate_workflows.powershell_step_failures(
                    workflow("go build ."), "ci.yml"
                )
                self.assertEqual(failures, 1)
                self.assertIn("ci.yml: job=analyze step=2", output.getvalue())


@unittest.skipUnless(os.name == "nt", "Requires native Windows PowerShell 5.1")
class NativeWindowsWorkflowTests(unittest.TestCase):
    def test_codeql_build_and_unquoted_regression(self):
        path = ROOT / ".github" / "workflows" / "codeql.yml"
        document = yaml.safe_load(path.read_text(encoding="utf-8"))
        self.assertEqual(validate_workflows.powershell_step_failures(document), 0)
        output = io.StringIO()
        with contextlib.redirect_stdout(output):
            failures = validate_workflows.powershell_step_failures(
                workflow("go build -tags=desktop,wv2runtime.embed,production .")
            )
        self.assertEqual(failures, 1)
        self.assertIn("[MissingArgument]", output.getvalue())

    def test_pip_audit_cache_cleanup_and_scanner_failure(self):
        executable = (
            Path(os.environ["SYSTEMROOT"])
            / "System32/WindowsPowerShell/v1.0/powershell.exe"
        )
        with tempfile.TemporaryDirectory(prefix="iris-audit-probe-") as directory:
            probe = Path(directory) / "probe.ps1"
            probe.write_text(PIP_AUDIT_PROBE, encoding="utf-8-sig")
            result = subprocess.run(
                [
                    str(executable),
                    "-NoLogo",
                    "-NoProfile",
                    "-NonInteractive",
                    "-ExecutionPolicy",
                    "Bypass",
                    "-File",
                    str(probe),
                    str(ROOT / "scripts/windows/IrisTools.ps1"),
                ],
                capture_output=True,
                text=True,
                encoding="utf-8",
                errors="replace",
                timeout=30,
                check=False,
            )
        self.assertEqual(result.returncode, 0, (result.stdout + result.stderr)[-2000:])
        self.assertIn("Temporary audit cache: PASS", result.stdout)
