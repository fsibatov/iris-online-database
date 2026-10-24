from __future__ import annotations

import hashlib
import json
import os
import re
import shutil
import subprocess
import tempfile
import unittest
from pathlib import Path
from unittest import mock

from prepare_windows_legacy import (
    compatibility_config,
    compatibility_marker,
    prepare_overlay,
    validate_overlay,
    verified_sources,
)
from release_targets import RELEASE_TARGETS
from verify_executables import expected_metadata_markers, missing_metadata_categories

ROOT = Path(__file__).resolve().parents[1]


@unittest.skipUnless(shutil.which("git"), "Git is required")
class WindowsLegacyPreparationTests(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory(prefix="iris-overlay-test-")
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name) / "Проверка с пробелами"
        self.project = self.root / "project"
        self.compat = self.project / "tools" / "compat"
        self.compat.mkdir(parents=True)
        self.goroot = self.root / "official Go"
        self.relative = "src/runtime/probe.go"
        self.original = b'package runtime\n\nvar value = "original"\n'
        self.patched = b'package runtime\n\nvar value = "patched"  \n'
        self.source = self.goroot / self.relative
        self.source.parent.mkdir(parents=True)
        self.source.write_bytes(self.original)
        self.config = {
            "go_version": "1.26.6",
            "patch": "probe.patch",
            "files": {
                self.relative: {
                    "original_sha256": hashlib.sha256(self.original).hexdigest(),
                    "patched_sha256": hashlib.sha256(self.patched).hexdigest(),
                }
            },
        }
        (self.compat / self.config["patch"]).write_bytes(
            b"--- a/src/runtime/probe.go\n"
            b"+++ b/src/runtime/probe.go\n"
            b"@@ -1,3 +1,3 @@\n"
            b" package runtime\n"
            b" \n"
            b'-var value = "original"\n'
            b'+var value = "patched"  \n'
        )
        self.directory = self.root / "output" / "overlay"
        self.git_config = self.root / "user.gitconfig"
        self.git_config.write_bytes(b"")
        self.attributes = self.root / "user.attributes"
        self.attributes.write_bytes(b"*.go text eol=crlf\n")
        self.environment = {
            name: value
            for name, value in os.environ.items()
            if not name.startswith("GIT_")
        }
        self.environment.update(
            GIT_CONFIG_GLOBAL=str(self.git_config), GIT_CONFIG_NOSYSTEM="1"
        )
        self.native_run = subprocess.run
        self.git_calls = []

    def run_tool(self, command, **kwargs):
        if command[0] == "fixture-go":
            self.assertEqual(command[1:], ["env", "-json", "GOVERSION", "GOROOT"])
            return subprocess.CompletedProcess(
                command,
                0,
                stdout=json.dumps(
                    {"GOVERSION": "go1.26.6", "GOROOT": str(self.goroot)}
                ),
            )
        self.git_calls.append(command)
        return self.native_run(command, **kwargs)

    def prepare(self):
        with (
            mock.patch.dict(os.environ, self.environment, clear=True),
            mock.patch("prepare_windows_legacy.ROOT", self.project),
            mock.patch("prepare_windows_legacy.COMPAT", self.compat),
            mock.patch(
                "prepare_windows_legacy.compatibility_config", return_value=self.config
            ),
            mock.patch(
                "prepare_windows_legacy.subprocess.run", side_effect=self.run_tool
            ),
        ):
            return prepare_overlay("fixture-go", self.directory)

    def test_preparation_preserves_bytes_with_user_git_settings(self):
        cases = (
            ("core.autocrlf", "true"),
            ("core.eol", "crlf"),
            ("core.attributesFile", str(self.attributes)),
            ("apply.whitespace", "fix"),
        )
        for index, (name, value) in enumerate(cases):
            with self.subTest(setting=name):
                self.git_config.write_bytes(b"")
                self.native_run(
                    [
                        shutil.which("git"),
                        "config",
                        "--file",
                        str(self.git_config),
                        name,
                        value,
                    ],
                    check=True,
                    capture_output=True,
                    timeout=30,
                    env=self.environment,
                )
                settings = self.git_config.read_bytes()
                self.directory = self.root / "output" / f"case-{index}"
                overlay = self.prepare()
                self.assertEqual(overlay, self.directory / "overlay.json")
                self.assertEqual(
                    (self.directory / self.relative).read_bytes(), self.patched
                )
                self.assertEqual(self.source.read_bytes(), self.original)
                self.assertEqual(self.git_config.read_bytes(), settings)

    def test_cached_overlay_is_reused_and_reverified(self):
        overlay = self.prepare()
        self.assertEqual(len(self.git_calls), 2)
        self.assertEqual(self.prepare(), overlay)
        self.assertEqual(len(self.git_calls), 2)
        (self.directory / self.relative).write_bytes(b"tampered\n")
        with self.assertRaisesRegex(
            ValueError, "Compatibility source checksum mismatch"
        ):
            self.prepare()
        self.assertEqual(self.source.read_bytes(), self.original)

    def test_goroot_directory_link_preserves_compiler_paths_and_output_guard(self):
        link = self.root / "linked Go"
        if os.name == "nt":
            self.native_run(
                ["cmd.exe", "/d", "/c", "mklink", "/J", str(link), str(self.goroot)],
                check=True,
                capture_output=True,
                timeout=30,
            )
        else:
            link.symlink_to(self.goroot, target_is_directory=True)
        self.goroot = link
        overlay = self.prepare()
        replacements = json.loads(overlay.read_text(encoding="utf-8"))["Replace"]
        self.assertEqual(
            replacements,
            {str(link / self.relative): str(self.directory / self.relative)},
        )
        self.assertNotIn(str(self.source.resolve()), replacements)
        self.assertEqual(self.prepare(), overlay)
        self.assertEqual(len(self.git_calls), 2)
        self.directory = link / "unsafe-overlay"
        with self.assertRaisesRegex(ValueError, "outside project and official Go"):
            self.prepare()
        self.assertFalse(self.directory.exists())
        self.assertEqual(self.source.read_bytes(), self.original)

    def test_checksum_failure_does_not_publish_partial_overlay(self):
        self.config["files"][self.relative]["patched_sha256"] = "0" * 64
        with self.assertRaisesRegex(ValueError, "Applied patch checksum mismatch"):
            self.prepare()
        self.assertFalse(self.directory.exists())
        self.assertEqual(list(self.directory.parent.iterdir()), [])
        self.assertEqual(self.source.read_bytes(), self.original)


class WindowsLegacyTests(unittest.TestCase):
    def test_legacy_targets_are_separate_and_keep_cpu_baselines(self):
        targets = [target for target in RELEASE_TARGETS if target.legacy]
        self.assertEqual(
            [
                (target.asset_suffix, target.goarch, target.build_level_marker)
                for target in targets
            ],
            [("7-8.1-x64", "amd64", "GOAMD64=v1"), ("7-8.1-x86", "386", "GO386=sse2")],
        )
        self.assertEqual(
            len({target.filename("2.0.6") for target in RELEASE_TARGETS}), 5
        )

    def test_legacy_metadata_cannot_pass_as_standard_build(self):
        for arch in ("amd64", "386"):
            metadata = "\n".join(
                value for _, value in expected_metadata_markers(arch, True)
            )
            self.assertEqual(missing_metadata_categories(metadata, arch, True), [])
            self.assertEqual(
                missing_metadata_categories(metadata, arch), ["PRODUCTION_TAGS"]
            )
            reordered = metadata.replace(
                "desktop,wv2runtime.embed,production,windows_legacy",
                "windows_legacy,desktop,wv2runtime.embed,production",
            )
            self.assertEqual(missing_metadata_categories(reordered, arch, True), [])
            self.assertIn(
                "PRODUCTION_TAGS",
                missing_metadata_categories(
                    metadata.replace(",windows_legacy", ""), arch, True
                ),
            )

    def test_compatibility_patch_is_pinned_and_preserves_root_security(self):
        config = compatibility_config()
        self.assertEqual(len(config["files"]), 5)
        self.assertTrue(all(not name.startswith("src/os/") for name in config["files"]))
        patch = (ROOT / "tools" / "compat" / config["patch"]).read_text()
        self.assertIn("+\t\terr = NTStatus(r1)", patch)
        self.assertNotIn("+\t\terr = errnoErr(e1)", patch)
        self.assertEqual(
            compatibility_marker(),
            f"IrisWindowsLegacy/go{config['go_version']}/{config['patch_sha256']}",
        )

    def test_modified_base_source_is_rejected(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            source = root / "src" / "runtime" / "probe.go"
            source.parent.mkdir(parents=True)
            source.write_bytes(b"original\n")
            config = {
                "files": {
                    "src/runtime/probe.go": {
                        "original_sha256": hashlib.sha256(
                            source.read_bytes()
                        ).hexdigest()
                    }
                }
            }
            self.assertEqual(
                verified_sources(root, config), {"src/runtime/probe.go": source}
            )
            source.write_bytes(b"unexpected version\n")
            with self.assertRaisesRegex(
                ValueError, "Official Go source checksum mismatch"
            ):
                verified_sources(root, config)

    def test_overlay_tampering_and_extra_replacements_are_rejected(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            directory = root / "overlay"
            relative = "src/runtime/probe.go"
            patched = directory / relative
            patched.parent.mkdir(parents=True)
            patched.write_bytes(b"patched\n")
            sources = {relative: root / "official" / relative}
            config = {
                "files": {
                    relative: {
                        "patched_sha256": hashlib.sha256(
                            patched.read_bytes()
                        ).hexdigest()
                    }
                }
            }
            document = {
                "Replace": {str(sources[relative].resolve()): str(patched.resolve())}
            }
            overlay = directory / "overlay.json"
            overlay.write_text(json.dumps(document))
            self.assertEqual(validate_overlay(directory, sources, config), overlay)
            patched.write_bytes(b"tampered\n")
            with self.assertRaisesRegex(
                ValueError, "Compatibility source checksum mismatch"
            ):
                validate_overlay(directory, sources, config)
            document["Replace"][str(root / "unrelated.go")] = str(patched)
            overlay.write_text(json.dumps(document))
            with self.assertRaisesRegex(ValueError, "unexpected replacements"):
                validate_overlay(directory, sources, config)

    def test_colour_mix_declarations_have_compatible_fallbacks(self):
        css = (ROOT / "web" / "styles.css").read_text()
        blocks = re.findall(r"\{([^{}]*)\}", css)
        found = 0
        for block in blocks:
            declarations = [part.strip() for part in block.split(";") if ":" in part]
            for index, declaration in enumerate(declarations):
                if "color-mix(" not in declaration:
                    continue
                found += 1
                name = declaration.split(":", 1)[0]
                self.assertGreater(index, 0, name)
                previous = declarations[index - 1]
                self.assertEqual(previous.split(":", 1)[0], name)
                self.assertNotIn("color-mix(", previous)
        self.assertGreater(found, 20)
