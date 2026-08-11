import shutil
import subprocess
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
        self.assertTrue(raw.startswith(b"\xef\xbb\xbf"), "build.ps1 must be UTF-8 with BOM for Windows PowerShell 5.1")
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
        self.assertIn("IRIS_SKIP_CHECKS=1 разрешён только для диагностической сборки", text)
        self.assertIn("$SavedEnvironment", text)
        self.assertIn("finally", text)
        powershell = shutil.which("powershell.exe") or shutil.which("powershell")
        if powershell:
            command = (
                "$e=$null; $t=$null; "
                f"[System.Management.Automation.Language.Parser]::ParseFile('{str(path).replace("'", "''")}',[ref]$t,[ref]$e)|Out-Null; "
                "if($e.Count -gt 0){$e|ForEach-Object{$_.ToString()}; exit 1}"
            )
            subprocess.run([powershell, "-NoProfile", "-NonInteractive", "-Command", command], check=True)

    def test_build_scripts_separate_release_and_diagnostic_markers(self):
        ps = (ROOT / "build.ps1").read_text(encoding="utf-8-sig")
        sh = (ROOT / "build.sh").read_text(encoding="utf-8")
        self.assertIn('$Version = "1.1.0"', ps)
        self.assertIn('VERSION="1.1.0"', sh)
        for text in (ps, sh):
            self.assertIn("IrisOnlineRelease/", text)
            self.assertIn("IRIS_SKIP_CHECKS=1 разрешён только для диагностической сборки", text)
            self.assertIn("IrisOnlineDiagnostic/", text)
            self.assertIn("diagnostic-", text)
            legacy_versions = tuple(f"1.0.{patch}" for patch in range(7))
            for stale in (*legacy_versions, "IrisOnline" + "Preview/"):
                self.assertNotIn(stale, text)


    def test_windows_resources_are_present_and_reproducible(self):
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
                expected = ROOT / f"resource_windows_{arch}.syso"
                self.assertTrue(expected.is_file(), expected)
                generated = Path(temp_dir) / expected.name
                subprocess.run([
                    "python3", str(generator), "--icon", str(icon), "--manifest", str(manifest),
                    "--arch", arch, "--output", str(generated),
                ], check=True)
                self.assertEqual(generated.read_bytes(), expected.read_bytes(), f"non-reproducible {arch} resource")

    def test_source_tree_has_no_development_package_artifacts(self):
        forbidden_names = {
            "AUDIT.md", "CHANGELOG.md", "PATCH_APPLY.md",
            "ORIGINAL-PUBLIC-ZIP-CONTENTS.txt", "ORIGINAL-SOURCE-ZIP-CONTENTS.txt",
        }
        names = {path.name for path in ROOT.iterdir()}
        self.assertFalse(names & forbidden_names)
        self.assertFalse((ROOT / "iris-online-database").exists(), "Linux build artifact must not be packaged in source")
        self.assertFalse(any(path.suffix.lower() == ".exe" for path in ROOT.rglob("*.exe")), "Windows EXE must not be packaged in source")
        for path in ROOT.rglob("*"):
            if "__pycache__" in path.parts:
                continue
            if path.is_file():
                name = path.name.lower()
                self.assertFalse(name.endswith((".tmp", ".bak", ".orig")), path)
                self.assertFalse(any(f"{1}.{1}.{patch}" in name for patch in range(7)), path)


if __name__ == "__main__":
    unittest.main()
