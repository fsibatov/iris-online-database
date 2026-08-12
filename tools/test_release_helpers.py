import shutil
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

from smoke_common import free_port, require_binary

ROOT = Path(__file__).resolve().parents[1]


class ReleaseHelperTests(unittest.TestCase):
    def test_free_port_is_valid(self):
        self.assertGreater(free_port(), 0)

    def test_require_binary_rejects_missing_path(self):
        with self.assertRaises(SystemExit):
            require_binary(str(Path(tempfile.gettempdir()) / "missing-iris-binary"))

    def test_build_ps1_is_powershell_51_safe_utf8_bom(self):
        path = ROOT / "build.ps1"
        raw = path.read_bytes()
        self.assertTrue(
            raw.startswith(b"\xef\xbb\xbf"),
            "build.ps1 must be UTF-8 with BOM for Windows PowerShell 5.1",
        )
        text = raw.decode("utf-8-sig")
        self.assertNotIn("$Version:", text)
        self.assertIn("${Version}:", text)
        self.assertIn("Собрано Iris Online", text)
        self.assertIn('$Version = "1.1.0"', text)
        self.assertIn("IrisOnlineRelease/$Version", text)
        self.assertIn("IrisOnlineDiagnostic/$Version/$ActualGo", text)
        self.assertIn('$env:CGO_ENABLED = "1"', text)
        self.assertIn('$env:CGO_ENABLED = "0"', text)
        self.assertIn("Для go test -race требуется GCC/CGO", text)
        self.assertIn(
            "IRIS_SKIP_CHECKS=1 разрешён только для диагностической сборки", text
        )
        self.assertIn("$SavedEnvironment", text)
        self.assertIn("finally", text)
        powershell = shutil.which("powershell.exe") or shutil.which("powershell")
        if powershell:
            command = (
                "$e=$null; $t=$null; "
                f"[System.Management.Automation.Language.Parser]::ParseFile('{str(path).replace("'", "''")}',[ref]$t,[ref]$e)|Out-Null; "
                "if($e.Count -gt 0){$e|ForEach-Object{$_.ToString()}; exit 1}"
            )
            subprocess.run(
                [powershell, "-NoProfile", "-NonInteractive", "-Command", command],
                check=True,
            )

    def test_build_scripts_separate_release_and_diagnostic_markers(self):
        ps = (ROOT / "build.ps1").read_text(encoding="utf-8-sig")
        sh = (ROOT / "build.sh").read_text(encoding="utf-8")
        self.assertIn('$Version = "1.1.0"', ps)
        self.assertIn('VERSION="1.1.0"', sh)
        for text in (ps, sh):
            self.assertIn("IrisOnlineRelease/", text)
            self.assertIn(
                "IRIS_SKIP_CHECKS=1 разрешён только для диагностической сборки", text
            )
            self.assertIn("IrisOnlineDiagnostic/", text)
            self.assertIn("diagnostic-", text)
            legacy_versions = tuple(f"1.0.{patch}" for patch in range(7))
            for stale in (*legacy_versions, "IrisOnline" + "Preview/"):
                self.assertNotIn(stale, text)

    def test_windows_resources_are_generated_and_reproducible(self):
        icon = ROOT / "resources" / "icon.ico"
        manifest = ROOT / "resources" / "app.manifest"
        self.assertTrue(icon.is_file())
        self.assertTrue(manifest.is_file())
        header = icon.read_bytes()[:6]
        self.assertEqual(header[:4], b"\x00\x00\x01\x00")
        self.assertGreater(int.from_bytes(header[4:6], "little"), 0)
        generator = ROOT / "tools" / "generate_windows_resources.py"
        with tempfile.TemporaryDirectory(prefix="iris-rsrc-") as temp_dir:
            for arch in ("386", "amd64", "arm64"):
                first = Path(temp_dir) / f"first-{arch}.syso"
                second = Path(temp_dir) / f"second-{arch}.syso"
                command = [
                    "python3",
                    str(generator),
                    "--icon",
                    str(icon),
                    "--manifest",
                    str(manifest),
                    "--arch",
                    arch,
                ]
                subprocess.run([*command, "--output", str(first)], check=True)
                subprocess.run([*command, "--output", str(second)], check=True)
                self.assertEqual(
                    first.read_bytes(),
                    second.read_bytes(),
                    f"non-reproducible {arch} resource",
                )
        self.assertFalse(
            any(ROOT.glob("resource_windows_*.syso")),
            "generated .syso files must not be stored in the source tree",
        )
        self.assertIn(
            "resource_windows_*.syso", (ROOT / ".gitignore").read_text(encoding="utf-8")
        )

    def test_local_release_gate_uses_actual_go_and_declared_smoke_dependencies(self):
        script = (ROOT / "tools" / "run_all_checks.sh").read_text(encoding="utf-8")
        self.assertIn('ACTUAL_GO="$(go version', script)
        self.assertIn("IrisOnlineDiagnostic/$VERSION/$ACTUAL_GO", script)
        self.assertNotIn("IrisOnlineDiagnostic/1.1.0/go1.23.2", script)
        self.assertIn("tools/repository_audit.py", script)
        self.assertIn("go mod verify", script)
        self.assertIn("go mod tidy -diff", script)
        self.assertIn('go build -o "$temp_dir/build-probe" .', script)
        self.assertIn("version(package)", script)
        self.assertIn("actual != expected", script)

        ci = (ROOT / ".github" / "workflows" / "ci.yml").read_text(encoding="utf-8")
        self.assertIn('go build -o "$RUNNER_TEMP/iris-online-build-check" .', ci)
        self.assertIn('PYTHONPYCACHEPREFIX="$RUNNER_TEMP/pycache"', ci)

        requirements = {
            line.strip().split("==", 1)[0]
            for line in (ROOT / "tools" / "requirements-audit.txt")
            .read_text(encoding="utf-8")
            .splitlines()
            if line.strip() and not line.lstrip().startswith("#")
        }
        self.assertIn("playwright", requirements)
        self.assertIn("psutil", requirements)

    def test_repository_audit_rejects_release_artifact_classes(self):
        source = (ROOT / "tools" / "repository_audit.py").read_text(encoding="utf-8")
        for marker in ("$coverage", "__pycache__", ".ruff_cache", '"dist"'):
            self.assertIn(marker, source)

        cache_dir = ROOT / "__pycache__"
        coverage_file = ROOT / "$coverage"
        self.assertFalse(cache_dir.exists())
        # repository_audit is intentionally read-only: the regression test owns cleanup.
        coverage_file.unlink(missing_ok=True)
        try:
            cache_dir.mkdir()
            (cache_dir / "audit-probe.pyc").write_bytes(b"probe")
            coverage_file.write_text("mode: set\n", encoding="utf-8")
            result = subprocess.run(
                [sys.executable, "-B", str(ROOT / "tools" / "repository_audit.py")],
                cwd=ROOT,
                capture_output=True,
                text=True,
                check=False,
            )
            self.assertNotEqual(result.returncode, 0)
            self.assertIn("forbidden release directory: __pycache__/", result.stdout)
            self.assertIn("forbidden release file: $coverage", result.stdout)
        finally:
            coverage_file.unlink(missing_ok=True)
            shutil.rmtree(cache_dir, ignore_errors=True)

    def test_source_tree_has_no_development_package_artifacts(self):
        forbidden_names = {
            "AUDIT.md",
            "CHANGELOG.md",
            "PATCH_APPLY.md",
            "ORIGINAL-PUBLIC-ZIP-CONTENTS.txt",
            "ORIGINAL-SOURCE-ZIP-CONTENTS.txt",
        }
        names = {path.name for path in ROOT.iterdir()}
        self.assertFalse(names & forbidden_names)
        self.assertFalse(
            (ROOT / "iris-online-database").exists(),
            "Linux build artifact must not be packaged in source",
        )
        self.assertFalse(
            any(path.suffix.lower() == ".exe" for path in ROOT.rglob("*.exe")),
            "Windows EXE must not be packaged in source",
        )
        forbidden_dirs = {
            "dist",
            "__pycache__",
            ".pytest_cache",
            ".mypy_cache",
            ".ruff_cache",
        }
        forbidden_files = {"$coverage", "coverage.out", ".coverage"}
        for path in ROOT.rglob("*"):
            self.assertFalse(path.is_dir() and path.name in forbidden_dirs, path)
            if path.is_file():
                name = path.name.lower()
                self.assertNotIn(path.name, forbidden_files, path)
                self.assertFalse(name.endswith((".tmp", ".bak", ".orig", ".pyc")), path)
                self.assertFalse(
                    any(f"{1}.{1}.{patch}" in name for patch in range(7)), path
                )


if __name__ == "__main__":
    unittest.main()
