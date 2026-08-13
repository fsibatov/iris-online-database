"""Regression tests for repository, build and release invariants."""

from __future__ import annotations

import json
import os
import re
import stat
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

from frontend_smoke_test import playwright_failure_category
from playwright.sync_api import Error as PlaywrightError
from release_fingerprint import FingerprintError, assert_release_tree, source_hash
from repository_audit import python_mode_violation

ROOT = Path(__file__).resolve().parents[1]
AUDIT = ROOT / "tools" / "repository_audit.py"


class ReleaseHelperTests(unittest.TestCase):
    def run_audit(self, root: Path) -> subprocess.CompletedProcess[str]:
        return subprocess.run(
            [sys.executable, "-B", str(AUDIT), "--root", str(root)],
            check=False,
            capture_output=True,
            text=True,
            encoding="utf-8",
            errors="strict",
            timeout=30,
        )

    def test_version_is_coherent_across_runtime_and_release_metadata(self):
        self.assertEqual(
            (ROOT / "VERSION").read_text(encoding="utf-8").strip(), "2.0.0"
        )
        server = (ROOT / "server.go").read_text(encoding="utf-8")
        html = (ROOT / "web" / "index.html").read_text(encoding="utf-8")
        script = (ROOT / "web" / "app.js").read_text(encoding="utf-8")
        wails = json.loads((ROOT / "wails.json").read_text(encoding="utf-8"))
        resources = json.loads(
            (ROOT / "build" / "windows" / "info.json").read_text(encoding="utf-8")
        )
        self.assertIn('var appVersion = "2.0.0"', server)
        self.assertIn("Версия 2.0.0", html)
        self.assertIn("const APP_VERSION = '2.0.0'", script)
        self.assertEqual(wails["info"]["productVersion"], "2.0.0")
        self.assertEqual(resources["fixed"]["product_version"], "2.0.0.0")
        self.assertEqual(resources["info"]["0419"]["ProductVersion"], "2.0.0")

    def test_desktop_build_contract_has_no_production_listener(self):
        windows_main = (ROOT / "main_windows.go").read_text(encoding="utf-8")
        server = (ROOT / "server.go").read_text(encoding="utf-8")
        combined = windows_main + server
        self.assertIn("wails.Run", windows_main)
        self.assertIn("SingleInstanceLock", windows_main)
        self.assertIn("WebviewUserDataPath", windows_main)
        self.assertNotIn("net.Listen(", combined)
        self.assertNotIn("127.0.0.1:8765", combined)
        self.assertNotIn("-no-browser", combined)
        self.assertNotIn("-addr", combined)

    def test_windows_resources_and_manifest_are_v2(self):
        icon = ROOT / "build" / "windows" / "icon.ico"
        manifest = (ROOT / "build" / "windows" / "wails.exe.manifest").read_text(
            encoding="utf-8"
        )
        header = icon.read_bytes()[:6]
        self.assertEqual(header[:4], b"\x00\x00\x01\x00")
        self.assertGreater(int.from_bytes(header[4:6], "little"), 0)
        self.assertIn("permonitorv2", manifest.lower())
        self.assertIn("longPathAware", manifest)
        self.assertIn('level="asInvoker"', manifest)

    def test_release_build_is_external_and_fingerprint_gated(self):
        build = (ROOT / "scripts" / "build-release.sh").read_text(encoding="utf-8")
        gate = (ROOT / "scripts" / "release-gate.sh").read_text(encoding="utf-8")
        launcher = (ROOT / "IrisTools.ps1").read_text(encoding="utf-8")
        for marker in (
            "release_fingerprint.py --verify",
            "windows/amd64",
            "-webview2 embed",
            "IrisOnlineDatabase.exe",
            "SHA256SUMS.txt",
        ):
            self.assertIn(marker, build)
        self.assertIn("release_fingerprint.py --write", gate)
        self.assertIn("git status --porcelain", gate)
        self.assertIn("scripts\\windows\\IrisTools.ps1", launcher)

    def test_legacy_test_launcher_delegates_to_the_canonical_release_gate(self):
        launcher = (ROOT / "01_TEST.bat").read_text(encoding="utf-8")
        self.assertIn(
            'powershell.exe -NoLogo -NoProfile -ExecutionPolicy Bypass -File "%~dp0IrisTools.ps1" -Action Test',
            launcher,
        )
        self.assertIn('set "code=%errorlevel%"', launcher)
        self.assertIn("pause", launcher)
        self.assertIn("exit /b %code%", launcher)
        self.assertNotIn("-Action Release", launcher)
        for duplicated_command in ("go test", "staticcheck", "govulncheck", "ruff"):
            self.assertNotIn(duplicated_command, launcher.lower())

    def test_fingerprint_hashes_git_mode_and_rejects_dirty_source(self):
        with tempfile.TemporaryDirectory(
            prefix="iris-fingerprint-fixture-"
        ) as temporary:
            root = Path(temporary)
            subprocess.run(
                ["git", "init", "--initial-branch=main", str(root)],
                check=True,
                capture_output=True,
                timeout=30,
            )
            subprocess.run(
                [
                    "git",
                    "-C",
                    str(root),
                    "config",
                    "user.email",
                    "test@example.invalid",
                ],
                check=True,
                timeout=30,
            )
            subprocess.run(
                ["git", "-C", str(root), "config", "user.name", "Iris test"],
                check=True,
                timeout=30,
            )
            tracked = root / "script.sh"
            tracked.write_text("exit 0\n", encoding="utf-8")
            subprocess.run(
                ["git", "-C", str(root), "add", "script.sh"],
                check=True,
                timeout=30,
            )
            subprocess.run(
                ["git", "-C", str(root), "commit", "-m", "fixture"],
                check=True,
                capture_output=True,
                timeout=30,
            )
            regular_hash, count = source_hash(root)
            self.assertEqual(count, 1)
            subprocess.run(
                ["git", "-C", str(root), "update-index", "--chmod=+x", "script.sh"],
                check=True,
                timeout=30,
            )
            executable_hash, _ = source_hash(root)
            self.assertNotEqual(regular_hash, executable_hash)
            with self.assertRaises(FingerprintError):
                assert_release_tree(root, "main")

    def test_python_requirements_are_single_pinned_source(self):
        requirements = (ROOT / "tools" / "requirements-audit.txt").read_text(
            encoding="utf-8"
        )
        packages = {}
        for line in requirements.splitlines():
            line = line.strip()
            if not line or line.startswith("#"):
                continue
            self.assertEqual(line.count("=="), 1, line)
            package, version = line.split("==", 1)
            packages[package] = version
        self.assertEqual(
            set(packages),
            {"bandit", "pip", "pip-audit", "playwright", "pyyaml", "ruff"},
        )
        for workflow in (ROOT / ".github" / "workflows").glob("*.yml"):
            text = workflow.read_text(encoding="utf-8")
            self.assertNotRegex(text, r"playwright==[0-9]")
            self.assertNotRegex(text, r"ruff==[0-9]")
            self.assertNotRegex(text, r"bandit==[0-9]")
            self.assertNotRegex(text, r"pip-audit==[0-9]")

    def test_workflow_actions_are_pinned_to_full_commits(self):
        uses_pattern = re.compile(r"^\s*uses:\s*[^@\s]+@([^\s#]+)", re.MULTILINE)
        for workflow in (ROOT / ".github" / "workflows").glob("*.yml"):
            text = workflow.read_text(encoding="utf-8")
            refs = uses_pattern.findall(text)
            self.assertTrue(refs, workflow.name)
            for ref in refs:
                self.assertRegex(ref, r"^[0-9a-f]{40}$", f"{workflow.name}: {ref}")

    def test_release_gate_uses_current_gitleaks_cli_and_embedded_data_audit(self):
        paths = (
            ROOT / ".github" / "workflows" / "ci.yml",
            ROOT / "scripts" / "release-gate.sh",
            ROOT / "scripts" / "windows" / "IrisTools.ps1",
        )
        combined = "\n".join(path.read_text(encoding="utf-8") for path in paths)
        self.assertNotIn("--source", combined)
        self.assertIn("data_presentation_audit.py", combined)
        self.assertNotIn("python3 -B tools/raw_projection_audit.py\n", combined)
        self.assertNotIn("python3 -B tools/drop_table_audit.py\n", combined)

    def test_release_requires_successful_ci_codeql_and_unchanged_artifact(self):
        script = (ROOT / "scripts" / "windows" / "IrisTools.ps1").read_text(
            encoding="utf-8"
        )
        for check in (
            "Linux quality and security",
            "Go race detector",
            "Native Windows amd64 Wails build",
            "Analyze (go)",
            "Analyze (python)",
        ):
            self.assertIn(check, script)
        self.assertIn('conclusion -ne "success"', script)
        self.assertNotIn('"neutral", "skipped"', script)
        self.assertIn("Release artifact changed after verification", script)

    def test_windows_tool_check_elevates_and_reads_native_output_safely(self):
        launcher = (ROOT / "IrisTools.ps1").read_text(encoding="utf-8")
        script = (ROOT / "scripts" / "windows" / "IrisTools.ps1").read_text(
            encoding="utf-8"
        )
        workflow = (ROOT / ".github" / "workflows" / "ci.yml").read_text(
            encoding="utf-8"
        )
        self.assertIn('@("Check", "Install")', launcher)
        self.assertIn("Restart-AsAdministrator", launcher)
        self.assertIn("-Verb RunAs", launcher)
        self.assertIn("-EncodedCommand", launcher)
        self.assertIn('Tool = "PowerShell"', script)
        self.assertIn(">=5.1 / Administrator", script)
        self.assertIn("$ExitCode = $LASTEXITCODE", script)
        self.assertNotIn("& $Command @Arguments 2>&1 | Select-Object -First 1", script)
        self.assertNotIn(
            'sys.argv[1]))" $Package 2>$null | Select-Object -First 1', script
        )
        self.assertIn(r"^staticcheck(?:\.exe)?\s+", script)
        self.assertIn(
            'Invoke-Checked $AuditPython @("-B", "tools/repository_audit.py")',
            script,
        )
        self.assertNotIn('Invoke-Checked "python" @("-B"', script)
        self.assertIn("Windows tooling self-test: PASS", script)
        self.assertIn("-Action SelfTest", workflow)
        self.assertIn("System.Management.Automation.Language.Parser", workflow)

    def test_windows_govulncheck_retries_and_fails_closed_on_network(self):
        script = (ROOT / "scripts" / "windows" / "IrisTools.ps1").read_text(
            encoding="utf-8"
        )
        self.assertIn("function Invoke-Govulncheck", script)
        self.assertIn("function Test-GovulncheckNetworkFailure", script)
        self.assertIn("function ConvertTo-SafeToolOutput", script)
        self.assertIn("function ConvertTo-NativeArgument", script)
        self.assertIn("function ConvertTo-NativeArgumentString", script)
        self.assertIn("function Invoke-CapturedNativeProcess", script)
        self.assertIn("[NETWORK/INFRASTRUCTURE RETRY]", script)
        self.assertIn("[NETWORK/INFRASTRUCTURE FALLBACK]", script)
        self.assertIn("[NETWORK/INFRASTRUCTURE SKIP]", script)
        self.assertIn("Vulnerability status is UNKNOWN", script)
        self.assertIn("RELEASE gate remains FAILED", script)
        self.assertIn("Invoke-Govulncheck", script)
        self.assertNotIn('Invoke-Checked "govulncheck" @("./...")', script)
        self.assertIn("WaitForExit($TimeoutSeconds * 1000)", script)
        self.assertIn("ReadToEndAsync()", script)
        self.assertIn("$Process.ExitCode", script)
        self.assertIn("without a successful result (exit code $ExitCode)", script)
        self.assertEqual(script.count('URL = "https://vuln.go.dev"'), 2)
        self.assertIn('URL = "https://storage.googleapis.com/go-vulndb"', script)
        self.assertIn('-Arguments @("-db", $Database.URL, "./...")', script)
        self.assertIn("$DelaySeconds = 2", script)
        self.assertNotIn("Start-Process -FilePath $Executable.Source", script)
        self.assertIn("echo No vulnerabilities found. & exit /b 0", script)
        self.assertIn('-Arguments @("/d", "/s", "/c", "exit /b 7")', script)
        self.assertRegex(
            script, r"function Test-Release \{\s+Assert-CleanTree\s+Test-WindowsTooling"
        )
        self.assertIn("[redacted-path]", script)
        self.assertIn("[redacted-token]", script)
        self.assertNotIn("Start-Job -ScriptBlock", script)
        self.assertIn("$StartInfo.StandardOutputEncoding = $Utf8", script)
        self.assertIn("$StartInfo.StandardErrorEncoding = $Utf8", script)
        self.assertIn(
            '$StartInfo.EnvironmentVariables["PYTHONIOENCODING"] = "utf-8"',
            script,
        )
        self.assertIn('$StartInfo.EnvironmentVariables["PYTHONUTF8"] = "1"', script)
        self.assertIn("Required tool failed with exit code $ExitCode", script)
        self.assertIn("ConvertTo-SafeToolOutput $Text", script)
        self.assertIn(
            'Invoke-Checked $CmdExecutable @("/d", "/c", "exit", "/b", "0") 30',
            script,
        )
        self.assertIn(
            'Invoke-Checked $CmdExecutable @("/d", "/c", "exit", "/b", "7") 30',
            script,
        )
        self.assertIn(
            '-Arguments @("-c", "import sys;sys.stdout.buffer.write(bytes.fromhex(\'d18f0a\'))")',
            script,
        )
        self.assertIn("[string][char]0x044F", script)
        self.assertIn('$FailedProbes.Add("UTF8_CAPTURE")', script)
        self.assertIn("categories=$($FailedProbes -join ',')", script)
        self.assertNotIn("Write-Host $StdoutText.TrimEnd()", script)
        self.assertNotIn("Write-Host $StderrText.TrimEnd()", script)
        self.assertIn("fetching vulnerabilities: read tcp: wsarecv", script)
        self.assertIn(
            "Vulnerability #1: GO-TEST-0001; see "
            "https://vuln.go.dev/ID/GO-TEST-0001.json",
            script,
        )

    def test_repository_audit_reports_exact_categories_without_payloads(self):
        sensitive_path = "/" + "home/" + "private-user/work/project"
        fake_token = "ghp_" + "A" * 36
        with tempfile.TemporaryDirectory(prefix="iris-audit-fixture-") as temporary:
            root = Path(temporary)
            (root / "__pycache__").mkdir()
            (root / "__pycache__" / "probe.pyc").write_bytes(b"cache")
            (root / "$coverage").write_text("mode: set\n", encoding="utf-8")
            (root / "artifact.exe").write_bytes(b"MZ")
            (root / "developer-path.txt").write_text(sensitive_path, encoding="utf-8")
            (root / "credential.txt").write_text(fake_token, encoding="utf-8")

            result = self.run_audit(root)
            output = result.stdout + result.stderr
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("[HYG001]", output)
            self.assertIn("[HYG002]", output)
            self.assertIn("(count=2)", output)
            self.assertIn("[SEC001]", output)
            self.assertIn("[SEC002]", output)
            self.assertNotIn(sensitive_path, output)
            self.assertNotIn(fake_token, output)
            self.assertNotIn("private-user", output)
            self.assertNotIn("artifact.exe", output)

    def test_repository_audit_enforces_python_shebang_mode_coherence(self):
        self.assertTrue(python_mode_violation("#!/usr/bin/env python3\n", 0))
        self.assertTrue(python_mode_violation("print('x')\n", stat.S_IXUSR))
        self.assertFalse(python_mode_violation("print('x')\n", 0))
        with tempfile.TemporaryDirectory(prefix="iris-mode-fixture-") as temporary:
            root = Path(temporary)
            tools = root / "tools"
            tools.mkdir()
            shebang = tools / "shebang.py"
            shebang.write_text("#!/usr/bin/env python3\nprint('x')\n", encoding="utf-8")
            executable = tools / "executable.py"
            executable.write_text("print('x')\n", encoding="utf-8")
            subprocess.run(
                ["git", "init", "--initial-branch=main", str(root)],
                check=True,
                capture_output=True,
                timeout=30,
            )
            subprocess.run(
                ["git", "-C", str(root), "add", "tools"],
                check=True,
                capture_output=True,
                timeout=30,
            )
            subprocess.run(
                [
                    "git",
                    "-C",
                    str(root),
                    "update-index",
                    "--chmod=+x",
                    "tools/executable.py",
                ],
                check=True,
                capture_output=True,
                timeout=30,
            )
            result = self.run_audit(root)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("[HYG003]", result.stdout)
            self.assertIn("(count=2)", result.stdout)
            self.assertNotIn("shebang.py", result.stdout)
            self.assertNotIn("executable.py", result.stdout)

    def test_repository_audit_rejects_release_manifest_and_symlink(self):
        with tempfile.TemporaryDirectory(prefix="iris-artifact-fixture-") as temporary:
            root = Path(temporary)
            (root / "SHA256SUMS.txt").write_text(
                "not a release tree\n", encoding="utf-8"
            )
            target = root / "target.txt"
            target.write_text("fixture\n", encoding="utf-8")
            (root / "linked.txt").symlink_to(target)
            result = self.run_audit(root)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("[HYG002]", result.stdout)
            self.assertIn("(count=2)", result.stdout)
            self.assertNotIn("SHA256SUMS.txt", result.stdout)
            self.assertNotIn("linked.txt", result.stdout)

    def test_python_tools_have_no_shebang_or_executable_mode(self):
        for path in (ROOT / "tools").glob("*.py"):
            self.assertFalse(path.read_bytes().startswith(b"#!"), path.name)
            executable = path.stat().st_mode & (
                stat.S_IXUSR | stat.S_IXGRP | stat.S_IXOTH
            )
            self.assertFalse(executable, path.name)

    def test_source_tree_has_no_generated_or_release_artifacts(self):
        forbidden_dirs = {
            "dist",
            "__pycache__",
            ".pytest_cache",
            ".mypy_cache",
            ".ruff_cache",
            "wailsjs",
        }
        forbidden_names = {
            "$coverage",
            "coverage.out",
            ".coverage",
            "appicon.png",
        }
        forbidden_suffixes = {".exe", ".dll", ".pdb", ".pyc", ".syso"}
        for path in ROOT.rglob("*"):
            if ".git" in path.relative_to(ROOT).parts:
                continue
            self.assertFalse(path.is_dir() and path.name in forbidden_dirs, path)
            if path.is_file():
                self.assertNotIn(path.name, forbidden_names, path)
                self.assertNotIn(path.suffix.lower(), forbidden_suffixes, path)

    def test_release_shell_scripts_are_real_executables(self):
        for name in ("release-gate.sh", "build-release.sh"):
            path = ROOT / "scripts" / name
            self.assertTrue(path.read_bytes().startswith(b"#!/usr/bin/env bash\n"))
            if os.name != "nt":
                self.assertTrue(path.stat().st_mode & stat.S_IXUSR, name)

    def test_frontend_smoke_redacts_missing_browser_details(self):
        sensitive = "/" + "home/private-user/playwright/chrome"
        error = PlaywrightError(f"Executable doesn't exist at {sensitive}")
        category = playwright_failure_category(error)
        self.assertEqual(category, "BROWSER_MISSING")
        self.assertNotIn(sensitive, category)
        self.assertNotIn("private-user", category)


if __name__ == "__main__":
    unittest.main()
