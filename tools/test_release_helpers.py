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
from release_targets import RELEASE_TARGETS
from repository_audit import python_mode_violation
from verify_executables import (
    EXPECTED_METADATA_MARKERS,
    expected_metadata_markers,
    missing_metadata_categories,
)
from verify_release_assets import expected_asset_names, verify_release_assets
from verify_windows_resources import verify_version_resource, version_tuple

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
        verifier = (ROOT / "tools" / "verify_windows_resources.py").read_text(
            encoding="utf-8"
        )
        self.assertIn("verify_version_resource(resource_payloads(path, 16)", verifier)
        self.assertIn('"FileVersion".encode("utf-16le")', verifier)
        self.assertIn('"ProductVersion".encode("utf-16le")', verifier)

    def test_windows_fixed_version_resource_is_exact(self):
        import struct

        self.assertEqual(version_tuple("2.0.0"), (2, 0, 0, 0))
        fixed = (
            0xFEEF04BD,
            0x00010000,
            0x00020000,
            0x00000000,
            0x00020000,
            0x00000000,
            0x0000003F,
            0x00000000,
            0x00040004,
            0x00000001,
            0x00000000,
            0x00000000,
            0x00000000,
        )
        payload = b"".join(
            (
                "Iris Online Database".encode("utf-16le"),
                "FileVersion".encode("utf-16le"),
                "ProductVersion".encode("utf-16le"),
                "2.0.0".encode("utf-16le"),
                struct.pack("<13I", *fixed),
            )
        )
        verify_version_resource([payload], "2.0.0", "amd64")
        broken = bytearray(payload)
        offset = broken.find(struct.pack("<I", 0xFEEF04BD))
        struct.pack_into("<I", broken, offset + 8, 0x00030000)
        with self.assertRaises(SystemExit):
            verify_version_resource([bytes(broken)], "2.0.0", "amd64")

    def test_release_build_is_external_and_fingerprint_gated(self):
        build = (ROOT / "scripts" / "build-release.sh").read_text(encoding="utf-8")
        gate = (ROOT / "scripts" / "release-gate.sh").read_text(encoding="utf-8")
        launcher = (ROOT / "IrisTools.ps1").read_text(encoding="utf-8")
        windows = (ROOT / "scripts" / "windows" / "IrisTools.ps1").read_text(
            encoding="utf-8"
        )
        for marker in (
            "release_fingerprint.py --verify",
            "windows/amd64",
            "windows/386",
            "windows/arm64",
            "IrisOnlineDB-$VERSION-Windows-x64.exe",
            "IrisOnlineDB-$VERSION-Windows-x86.exe",
            "IrisOnlineDB-$VERSION-Windows-arm64.exe",
            "-webview2 embed",
            "IrisOnlineDatabase.exe",
            "SHA256SUMS.txt",
        ):
            self.assertIn(marker, build)
        self.assertEqual(
            [(target.goarch, target.asset_suffix) for target in RELEASE_TARGETS],
            [("amd64", "x64"), ("386", "x86"), ("arm64", "arm64")],
        )
        self.assertIn("release_fingerprint.py --write", gate)
        self.assertIn("git status --porcelain", gate)
        self.assertIn("scripts\\windows\\IrisTools.ps1", launcher)
        self.assertIn("CGO_ENABLED=0", build)
        self.assertIn('$env:CGO_ENABLED = "0"', windows)
        self.assertIn(
            '$EnvironmentNames = @("CGO_ENABLED", "GOAMD64", "GO386", "GOARM64")',
            windows,
        )
        self.assertIn('Remove-Item -Path "Env:$Name"', windows)
        for platform, suffix in (
            ("windows/amd64", "x64"),
            ("windows/386", "x86"),
            ("windows/arm64", "arm64"),
        ):
            self.assertIn(platform, windows)
            self.assertIn(f'Windows-{suffix}.exe"', windows)

    def test_optional_test_launcher_is_kept_outside_source(self):
        self.assertFalse((ROOT / "01_TEST.bat").exists())

    def test_windows_release_gate_normalizes_ntfs_file_mode_tracking(self):
        windows = (ROOT / "scripts" / "windows" / "IrisTools.ps1").read_text(
            encoding="utf-8"
        )
        assert_clean_tree = windows.split("function Assert-CleanTree {", 1)[1].split(
            "function Test-Release {", 1
        )[0]
        self.assertIn('$env:OS -eq "Windows_NT"', assert_clean_tree)
        self.assertIn("git config --local core.filemode false", assert_clean_tree)
        self.assertLess(
            assert_clean_tree.index("git config --local core.filemode false"),
            assert_clean_tree.index('git status "--porcelain=v1"'),
        )
        self.assertIn(
            'if ($LASTEXITCODE -ne 0) { throw "Git status failed." }',
            assert_clean_tree,
        )

    def test_release_metadata_diagnostic_identifies_cgo_without_raw_payload(self):
        valid = "\n".join(marker for _category, marker in EXPECTED_METADATA_MARKERS)
        self.assertEqual(missing_metadata_categories(valid), [])
        cgo_enabled = valid.replace("CGO_ENABLED=0", "CGO_ENABLED=1")
        categories = missing_metadata_categories(cgo_enabled)
        self.assertEqual(categories, ["CGO_DISABLED"])
        self.assertNotIn("CGO_ENABLED=1", " ".join(categories))

    def test_release_metadata_matrix_has_exact_architecture_markers(self):
        expected_levels = {
            "amd64": "GOAMD64=v1",
            "386": "GO386=sse2",
            "arm64": "GOARM64=v8.0",
        }
        for target in RELEASE_TARGETS:
            with self.subTest(goarch=target.goarch):
                markers = dict(expected_metadata_markers(target.goarch))
                self.assertEqual(markers["TARGET_ARCH"], f"GOARCH={target.goarch}")
                self.assertEqual(
                    markers["TARGET_LEVEL"], expected_levels[target.goarch]
                )
                metadata = "\n".join(markers.values())
                self.assertEqual(
                    missing_metadata_categories(metadata, target.goarch), []
                )

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

    def test_workflow_validator_rejects_runner_context_in_job_env(self):
        from validate_workflows import invalid_job_env_contexts

        invalid = {
            "jobs": {
                "windows": {
                    "env": {"GOBIN": "${{ runner.temp }}\\go-tools"},
                    "steps": [],
                }
            }
        }
        valid = {
            "jobs": {
                "windows": {
                    "env": {"GOBIN": "C:\\tools"},
                    "steps": [],
                }
            }
        }
        self.assertEqual(invalid_job_env_contexts(invalid), 1)
        self.assertEqual(invalid_job_env_contexts(valid), 0)

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
        self.assertIn("function Test-GitleaksDetection", combined)
        self.assertIn("Gitleaks functional detection self-test failed.", combined)
        self.assertIn("iris_resolve_gitleaks", combined)
        self.assertIn("iris_test_gitleaks_detection", combined)
        self.assertIn("Invoke-GitleaksHistoryScan", combined)
        self.assertIn("Test-GitleaksHistoryProof", combined)
        self.assertIn("ConvertFrom-AnsiToolOutput", combined)
        self.assertIn("GITLEAKS_HISTORY_ANSI_PROOF", combined)
        self.assertIn("GITLEAKS_HISTORY_ZERO_PROOF", combined)
        self.assertIn("iris_gitleaks_history_scan", combined)
        release_tools = (ROOT / "scripts" / "release-tools.sh").read_text(
            encoding="utf-8"
        )
        self.assertIn("iris_gitleaks_history_proof", release_tools)
        self.assertIn("commits scanned", combined)
        self.assertIn("--exit-code 37", combined)
        self.assertNotIn("python3 -B tools/raw_projection_audit.py\n", combined)
        self.assertNotIn("python3 -B tools/drop_table_audit.py\n", combined)
        workflow = (ROOT / ".github" / "workflows" / "ci.yml").read_text(
            encoding="utf-8"
        )
        self.assertIn(
            'iris_gitleaks_history_scan "$(command -v gitleaks)" "."', workflow
        )

    def test_release_requires_successful_ci_codeql_and_unchanged_artifact(self):
        script = (ROOT / "scripts" / "windows" / "IrisTools.ps1").read_text(
            encoding="utf-8"
        )
        for check in (
            "Linux quality and security",
            "Go race detector",
            "Native Windows Wails release matrix",
            "Analyze (go)",
            "Analyze (python)",
        ):
            self.assertIn(check, script)
        self.assertIn('status -ne "completed"', script)
        self.assertIn('conclusion -ne "success"', script)
        self.assertIn("Required GitHub check is still running", script)
        self.assertIn("Required GitHub check failed", script)
        self.assertIn("check-runs?filter=latest&per_page=100", script)
        self.assertIn(
            "GitHub check-runs response exceeded the verified page size.", script
        )
        self.assertIn("Sort-Object -Property id -Descending", script)
        self.assertNotIn('"neutral", "skipped"', script)
        self.assertIn("tools/verify_release_assets.py", script)
        self.assertIn("SHA256SUMS.txt", script)

    def test_windows_audit_environment_rebuild_is_side_by_side(self):
        script = (ROOT / "scripts" / "windows" / "IrisTools.ps1").read_text(
            encoding="utf-8"
        )
        self.assertNotIn('"-m", "venv", "--clear"', script)
        self.assertIn('"python-audit-" + $Hash.Substring(0, 16)', script)
        self.assertIn(
            '$AuditEnvPointer = Join-Path $ToolRoot "python-audit-active.txt"', script
        )
        self.assertIn("function Test-AuditEnvironmentContent", script)
        self.assertIn("function Get-Python313Version", script)
        self.assertIn("function Find-Python313Executable", script)
        self.assertIn('Get-Command "python.exe" -CommandType Application', script)
        self.assertIn('Get-Command "py.exe" -CommandType Application', script)
        self.assertIn("Programs\\Python\\Python313\\python.exe", script)
        self.assertIn(
            'Get-ChildItem -LiteralPath $ToolRoot -Directory -Filter "python-audit*"',
            script,
        )
        self.assertIn("reused validated environment", script)
        self.assertIn('Tool = "Audit Python"', script)
        self.assertIn("$AuditEnvironmentReady = $false", script)
        self.assertIn("if (-not $AuditEnvironmentReady -and", script)
        self.assertIn("Only now require a bootstrap Python 3.13", script)
        self.assertLess(
            script.index("reused validated environment"),
            script.index("Find-Python313Executable -AuditEnvironmentCandidates"),
        )
        self.assertIn("function Invoke-PythonVenv", script)
        self.assertIn(
            "Invoke-PythonVenv -Python $BasePython -Destination $NewAuditEnv", script
        )
        self.assertIn(
            "Test-AuditEnvironment -EnvironmentPath $NewAuditEnv "
            "-ExpectedHash $Hash -ExpectedPythonVersion $PythonVersion",
            script,
        )

    def test_windows_go_pin_uses_private_verified_official_toolchain(self):
        go_pin = (ROOT / ".go-version").read_text(encoding="utf-8").strip()
        match = re.fullmatch(r"(\d+)\.(\d+)\.(\d+)", go_pin)
        self.assertIsNotNone(match)
        self.assertGreaterEqual(tuple(map(int, match.groups())), (1, 26, 6))

        script = (ROOT / "scripts" / "windows" / "IrisTools.ps1").read_text(
            encoding="utf-8"
        )
        self.assertIn("$PinnedGoDirectory = Join-Path $ToolRoot", script)
        self.assertIn(
            '$PinnedGoBinDirectory = Join-Path $PinnedGoDirectory "go\\bin"', script
        )
        self.assertIn('$env:GOTOOLCHAIN = "local"', script)
        self.assertIn("function Test-ExactGoExecutable", script)
        self.assertIn("function Install-PinnedGo", script)
        self.assertIn("https://go.dev/dl/?mode=json&include=all", script)
        self.assertIn('$ArchiveName = "go$GoPin.windows-amd64.zip"', script)
        self.assertIn("Get-FileHash -LiteralPath $Archive -Algorithm SHA256", script)
        self.assertIn("$ActualSize -ne $ExpectedSize", script)
        self.assertIn("$ActualHash -ne $ExpectedHash", script)
        self.assertIn("Test-ExactGoExecutable -Executable $StagedGo", script)
        self.assertIn("Install-PinnedGo", script)
        self.assertNotIn('"GoLang.Go", "--version", $GoPin', script)

        workflow = (ROOT / ".github" / "workflows" / "ci.yml").read_text(
            encoding="utf-8"
        )
        codeql = (ROOT / ".github" / "workflows" / "codeql.yml").read_text(
            encoding="utf-8"
        )
        self.assertIn("go-version-file: .go-version", workflow)
        self.assertIn("go-version-file: .go-version", codeql)
        self.assertIn('iris_run_govulncheck "$(command -v govulncheck)" 15m', workflow)
        release_tools = (ROOT / "scripts" / "release-tools.sh").read_text(
            encoding="utf-8"
        )
        self.assertIn("export GOTOOLCHAIN=local", release_tools)

    def test_windows_wails_pin_uses_go_module_metadata_and_exact_binary(self):
        script = (ROOT / "scripts" / "windows" / "IrisTools.ps1").read_text(
            encoding="utf-8"
        )
        self.assertIn("function ConvertFrom-WailsModuleMetadata", script)
        self.assertIn("function Get-GoBinDirectory", script)
        self.assertIn("function Get-WailsModuleVersion", script)
        self.assertIn("function Get-WailsInfo", script)
        self.assertIn("function Get-PinnedWailsExecutable", script)
        self.assertIn("& $GoExecutable.Source version -m $Executable 2>&1", script)
        self.assertIn("github.com/wailsapp/wails/v2/cmd/wails", script)
        self.assertIn("^path\\s+", script)
        self.assertIn("^mod\\s+", script)
        self.assertIn("WAILS_METADATA_PARSE", script)
        self.assertIn("$WailsExecutable = Get-PinnedWailsExecutable", script)
        self.assertIn("Invoke-Checked $WailsExecutable @(", script)
        self.assertNotIn('Get-VersionLine "wails" @("version")', script)
        self.assertNotIn('Invoke-Checked "wails" @(', script)

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
        failure_display = launcher.index('Write-Host ("FAILED: " + $FailureMessage)')
        elevated_pause = launcher.index(
            'Read-Host "Press Enter to close the administrator window"'
        )
        failure_throw = launcher.index("throw $FailureMessage")
        self.assertLess(failure_display, elevated_pause)
        self.assertLess(elevated_pause, failure_throw)
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
        self.assertIn("function Resolve-NativeExecutablePath", script)
        self.assertIn("Get-Command $File -CommandType Application", script)
        self.assertIn(
            "$ExecutablePath = Resolve-NativeExecutablePath -File $File", script
        )
        self.assertIn("-File $ExecutablePath", script)
        self.assertIn("function Invoke-Staticcheck", script)
        self.assertIn("function Get-GoToolCandidates", script)
        self.assertIn("function Get-StaticcheckInfo", script)
        self.assertIn("function Get-PinnedStaticcheckExecutable", script)
        self.assertIn(
            "$StaticcheckExecutable = Get-PinnedStaticcheckExecutable", script
        )
        self.assertIn("$Output = & $StaticcheckExecutable @Arguments 2>&1", script)
        self.assertIn('Invoke-Staticcheck @("-version")', script)
        self.assertIn('$FailedProbes.Add("STATICCHECK_DIRECT")', script)
        self.assertNotIn('$FailedProbes.Add("STATICCHECK_CAPTURE")', script)
        self.assertNotIn('Invoke-Checked "staticcheck" @("./...")', script)
        self.assertIn('Invoke-Staticcheck @("./...")', script)
        self.assertNotRegex(script, r"(?m)^\s*& winget\s+install\b")
        self.assertIn('Invoke-Checked "winget" @("install"', script)
        self.assertIn(
            '$GitleaksWindowsX64Sha256 = "d29144deff3a68aa93ced33dddf84b7fdc26070add4aa0f4513094c8332afc4e"',
            script,
        )
        self.assertIn("$Expected -ne $GitleaksWindowsX64Sha256", script)
        self.assertNotIn("function Read-NativeCaptureFile", script)
        self.assertIn("-Action SelfTest", workflow)
        self.assertIn("System.Management.Automation.Language.Parser", workflow)
        self.assertIn(
            "$driveRoot = [IO.Path]::GetPathRoot($env:GITHUB_WORKSPACE)", workflow
        )
        self.assertIn("$env:TEMP = $testTemp", workflow)
        self.assertIn("$env:TMP = $testTemp", workflow)
        self.assertIn("Remove-Item -LiteralPath $testTemp", workflow)
        for platform, suffix in (
            ("windows/amd64", "x64"),
            ("windows/386", "x86"),
            ("windows/arm64", "arm64"),
        ):
            self.assertIn(platform, workflow)
            self.assertIn(f'Suffix = "{suffix}"', workflow)
        self.assertIn("IrisOnlineDB-$version-Windows-$($target.Suffix).exe", workflow)
        self.assertIn("verify_release_assets.py --directory $artifactDir", workflow)
        self.assertIn("verify_executables.py --directory $artifactDir", workflow)
        self.assertIn("verify_windows_resources.py --directory $artifactDir", workflow)

    def test_windows_ci_installs_pinned_tools_before_real_selftest(self):
        workflow = (ROOT / ".github" / "workflows" / "ci.yml").read_text(
            encoding="utf-8"
        )
        self.assertIn("name: Native Windows Wails release matrix", workflow)
        python_setup = workflow.index(
            "- name: Setup Python 3.13", workflow.index("windows-build:")
        )
        go_setup = workflow.index("- name: Setup pinned Go", python_setup)
        install = workflow.index(
            "- name: Install pinned Windows release Go tools", go_setup
        )
        selftest = workflow.index(
            "- name: Parse and self-test Windows PowerShell 5.1 tooling", install
        )
        self.assertLess(python_setup, selftest)
        self.assertLess(go_setup, selftest)
        self.assertLess(install, selftest)
        install_block = workflow[install:selftest]
        self.assertIn("wails@v2.14.0", install_block)
        self.assertIn("staticcheck@2026.1", install_block)
        self.assertIn("govulncheck@v1.6.0", install_block)
        windows_job = workflow[workflow.index("windows-build:") :]
        self.assertNotIn("GOBIN: ${{ runner.temp }}\\go-tools", windows_job)
        self.assertIn('$env:GOBIN = Join-Path $env:RUNNER_TEMP "go-tools"', windows_job)
        self.assertIn('$env:GITHUB_ENV, "GOBIN=$env:GOBIN"', windows_job)
        self.assertNotIn("shell: pwsh", windows_job)

    def test_windows_bat_has_cmd_safe_encoding_menu_and_fail_closed_actions(self):
        path = ROOT / "scripts" / "windows" / "00_RELEASE_WINDOWS.bat"
        raw = path.read_bytes()
        self.assertTrue(raw.startswith(b"@echo off\r\n"))
        self.assertFalse(raw.startswith(b"\xef\xbb\xbf"))
        self.assertEqual(raw.replace(b"\r\n", b"").count(b"\n"), 0)
        raw.decode("ascii")
        text = raw.decode("ascii")
        for line in (
            "1 - PREPARE RELEASE",
            "2 - PUSH COMMIT",
            "3 - GITHUB RELEASE",
            "4 - CHECK TOOLS",
            "5 - INSTALL/UPDATE TOOLS",
            "6 - OPEN RELEASE FOLDER",
            "0 - EXIT",
        ):
            self.assertIn(line, text)
        self.assertIn("setlocal EnableExtensions DisableDelayedExpansion", text)
        self.assertIn('pushd "%REPO%"', text)
        self.assertIn("if errorlevel 1 (", text)
        self.assertIn("ERROR: Could not enter repository directory.", text)
        self.assertIn("-NoLogo -NoProfile -ExecutionPolicy Bypass", text)
        self.assertIn('if not "%RC%"=="0" goto failed', text)
        self.assertIn('if "%CHOICE%"=="1" goto action_prepare', text)
        self.assertIn(':action_prepare\r\nset "ACTION=Prepare"', text)
        self.assertNotRegex(text, r"(?im)^if .*&\s*goto ")
        failed_block = text.split(":failed\r\n", 1)[1].split(":success_exit\r\n", 1)[0]
        self.assertIn("pause\r\ngoto menu", failed_block)
        self.assertNotIn("exit /b", failed_block)
        self.assertIn('set "LAST_ACTION_RC=%RC%"', failed_block)
        self.assertIn("exit /b %LAST_ACTION_RC%", text)
        self.assertNotIn("git push", text.lower())
        self.assertNotIn("gh release", text.lower())

    def test_prepare_release_runs_complete_gate_before_artifact_build(self):
        script = (ROOT / "scripts" / "windows" / "IrisTools.ps1").read_text(
            encoding="utf-8"
        )
        prepare = script.split("function Prepare-Release {", 1)[1].split(
            "function Invoke-GitFetchMain {", 1
        )[0]
        self.assertLess(prepare.index("Test-Release"), prepare.index("Build-Release"))
        gate = script.split("function Test-Release {", 1)[1].split(
            "function Clear-BuildGenerated {", 1
        )[0]
        for marker in (
            "repository_audit.py",
            'Invoke-Checked "go" @("mod", "verify")',
            'Invoke-Checked "go" @("mod", "tidy", "-diff")',
            'Invoke-Checked "go" @("build"',
            'Invoke-Checked "go" @("test"',
            'Invoke-Checked "go" @("vet"',
            'Invoke-Staticcheck @("./...")',
            "Invoke-Govulncheck",
            "unittest",
            'ruff.exe") @("check"',
            'ruff.exe") @("format"',
            'bandit.exe")',
            'pip-audit.exe")',
            "validate_workflows.py",
            'Invoke-Checked "node" @("--check", "web/app.js")',
            "data_presentation_audit.py",
            "frontend_smoke_test.py",
            'gitleaks.exe"',
            'release_fingerprint.py", "--write',
        ):
            self.assertIn(marker, gate)

    def test_shell_release_tools_do_not_trust_wails_path_or_version_text(self):
        helper = (ROOT / "scripts" / "release-tools.sh").read_text(encoding="utf-8")
        build = (ROOT / "scripts" / "build-release.sh").read_text(encoding="utf-8")
        gate = (ROOT / "scripts" / "release-gate.sh").read_text(encoding="utf-8")
        self.assertIn("go version -m", helper)
        self.assertIn("github.com/wailsapp/wails/v2/cmd/wails", helper)
        self.assertIn("iris_resolve_staticcheck", helper)
        self.assertIn("iris_resolve_govulncheck", helper)
        self.assertIn("iris_run_govulncheck", helper)
        self.assertIn("iris_govuln_network_failure", helper)
        self.assertLess(
            helper.index('"https://storage.googleapis.com/go-vulndb"'),
            helper.index('"https://vuln.go.dev"'),
        )
        self.assertIn('iris_run_govulncheck "$GOVULNCHECK_BIN" 15m', gate)
        self.assertNotIn('timeout 15m "$GOVULNCHECK_BIN" ./...', gate)
        self.assertIn("iris_resolve_gitleaks", helper)
        self.assertIn("iris_test_gitleaks_detection", helper)
        self.assertIn('GITLEAKS_BIN="$(iris_resolve_gitleaks 8.30.1', gate)
        self.assertIn('WAILS_BIN="$(iris_resolve_wails v2.14.0', build)
        self.assertIn('"$WAILS_BIN" build', build)
        self.assertIn('STATICCHECK_BIN="$(iris_resolve_staticcheck 2026.1', gate)
        self.assertNotIn("wails version", build + gate)
        self.assertNotRegex(build + gate, r"(?m)^\s*wails build")

    def test_release_assets_are_exact_and_checksum_verified(self):
        self.assertEqual(
            expected_asset_names("2.0.0"),
            (
                "IrisOnlineDB-2.0.0-Windows-x64.exe",
                "IrisOnlineDB-2.0.0-Windows-x86.exe",
                "IrisOnlineDB-2.0.0-Windows-arm64.exe",
                "SHA256SUMS.txt",
            ),
        )
        with tempfile.TemporaryDirectory(prefix="iris-release-assets-") as temporary:
            directory = Path(temporary)
            names = expected_asset_names("2.0.0")[:-1]
            import hashlib

            lines = []
            for index, name in enumerate(names, start=1):
                payload = b"MZ" + bytes([index])
                (directory / name).write_bytes(payload)
                lines.append(f"{hashlib.sha256(payload).hexdigest()}  {name}")
            (directory / "SHA256SUMS.txt").write_text(
                "\n".join(lines) + "\n", encoding="ascii"
            )
            verify_release_assets(directory, "2.0.0")
            (directory / "debug.pdb").write_bytes(b"debug")
            with self.assertRaises(SystemExit):
                verify_release_assets(directory, "2.0.0")

    def test_release_uses_explicit_gpg_identity_and_signed_tag_verification(self):
        script = (ROOT / "scripts" / "windows" / "IrisTools.ps1").read_text(
            encoding="utf-8"
        )
        self.assertIn('$ReleaseGitName = "fsibatov"', script)
        self.assertIn('$ReleaseGitEmail = "farushik01@gmail.com"', script)
        self.assertIn(
            '$ReleaseGpgExecutable = "C:\\Program Files\\Git\\usr\\bin\\gpg.exe"',
            script,
        )
        self.assertIn(
            '$ReleaseGpgFingerprint = "B0A5D341B2EE901172F485DE9BC0EBCFE2795291"',
            script,
        )
        self.assertIn("function Assert-ReleaseSigningIdentity", script)
        self.assertIn('"--list-secret-keys", $ReleaseGpgFingerprint', script)
        self.assertIn('"tag", "-s", "-u", $ReleaseGpgFingerprint', script)
        self.assertIn('"verify-tag", "--raw", $Tag', script)
        self.assertIn("[GNUPG:\\] VALIDSIG", script)
        self.assertIn("function Ensure-ReleaseTag", script)
        self.assertIn("Local and origin release tag objects differ.", script)
        self.assertIn('"--verify-tag"', script)
        self.assertNotIn('"tag", "-a"', script)

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
        self.assertEqual(
            script.count('URL = "https://storage.googleapis.com/go-vulndb"'), 2
        )
        self.assertEqual(script.count('URL = "https://vuln.go.dev"'), 1)
        storage_primary = script.index(
            'Name = "Google storage"; URL = "https://storage.googleapis.com/go-vulndb"'
        )
        storage_retry = script.index(
            'Name = "Google storage retry"; URL = "https://storage.googleapis.com/go-vulndb"'
        )
        canonical_fallback = script.index(
            'Name = "canonical fallback"; URL = "https://vuln.go.dev"'
        )
        self.assertLess(storage_primary, storage_retry)
        self.assertLess(storage_retry, canonical_fallback)
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
            '$WindowsPowerShell = Join-Path $env:SystemRoot "System32\\WindowsPowerShell\\v1.0\\powershell.exe"',
            script,
        )
        self.assertIn(
            '"[Console]::OpenStandardOutput().Write([byte[]](0xD1,0x8F,0x0A),0,3)"',
            script,
        )
        self.assertNotIn("sys.stdout.buffer.write(bytes.fromhex", script)
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

    def test_repository_audit_diagnostics_are_structured_not_opaque(self):
        source = AUDIT.read_text(encoding="utf-8")
        self.assertNotIn('print("FAIL: [redacted]")', source)
        self.assertIn("CATEGORY_MESSAGES", source)
        self.assertIn("finding.category", source)
        self.assertIn("count=", source)

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
            symlink_created = False
            try:
                (root / "linked.txt").symlink_to(target)
                symlink_created = True
            except OSError as exc:
                # Creating symlinks on Windows can require Developer Mode or the
                # SeCreateSymbolicLinkPrivilege. The release gate must remain
                # runnable by a normal non-elevated user; Linux CI still exercises
                # the symlink-specific branch when Windows cannot create one.
                if os.name != "nt" or getattr(exc, "winerror", None) != 1314:
                    raise
            result = self.run_audit(root)
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("[HYG002]", result.stdout)
            expected_count = 2 if symlink_created else 1
            self.assertIn(f"(count={expected_count})", result.stdout)
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
