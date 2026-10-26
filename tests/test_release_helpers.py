"""Regression tests for repository, build and release invariants."""

from __future__ import annotations

import json
import os
import re
import stat
import struct
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path
from types import SimpleNamespace
from unittest.mock import patch

from frontend_smoke_test import playwright_failure_category
from playwright.sync_api import Error as PlaywrightError
from release_fingerprint import (
    FingerprintError,
    assert_release_tree,
    source_hash,
    verify_fingerprint,
    write_fingerprint,
)
from release_targets import RELEASE_TARGETS
from repository_audit import python_mode_violation
from verify_executables import (
    EXPECTED_METADATA_MARKERS,
    expected_metadata_markers,
    missing_metadata_categories,
    verify_executable,
)
from verify_release_assets import expected_asset_names, verify_release_assets
from verify_python_environment import expected_versions
from verify_windows_resources import directory_entries, parse_pe, resource_payloads, verify_icon_group, verify_target, verify_version_resource, version_tuple

ROOT = Path(__file__).resolve().parents[1]
AUDIT = ROOT / "tools" / "repository_audit.py"
VALID_ICON_GROUP = struct.pack("<HHHBBBBHHIH", 0, 1, 1, 16, 16, 0, 0, 1, 32, 4, 1)


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
        version = (ROOT / "VERSION").read_text(encoding="utf-8").strip()
        self.assertRegex(version, r"^\d+\.\d+\.\d+$")
        server = (ROOT / "server.go").read_text(encoding="utf-8")
        html = (ROOT / "web" / "index.html").read_text(encoding="utf-8")
        script = (ROOT / "web" / "app.js").read_text(encoding="utf-8")
        wails = json.loads((ROOT / "wails.json").read_text(encoding="utf-8"))
        resources = json.loads(
            (ROOT / "build" / "windows" / "info.json").read_text(encoding="utf-8")
        )
        self.assertIn(f'var appVersion = "{version}"', server)
        self.assertIn(f"Версия {version}", html)
        self.assertIn(f"const APP_VERSION = '{version}'", script)
        self.assertEqual(wails["info"]["productVersion"], version)
        self.assertEqual(resources["fixed"]["product_version"], f"{version}.0")
        self.assertEqual(resources["info"]["0419"]["ProductVersion"], version)

    def test_release_uses_only_current_changelog_section_for_notes(self):
        version = (ROOT / "VERSION").read_text(encoding="utf-8").strip()
        changelog = (ROOT / "CHANGELOG.md").read_text(encoding="utf-8")
        script = (ROOT / "scripts" / "windows" / "IrisTools.ps1").read_text(
            encoding="utf-8"
        )
        self.assertIn(f"## {version} —", changelog)
        self.assertIn("$ReleaseSectionPattern", script)
        self.assertIn('"--notes-file", $ReleaseNotesPath', script)
        self.assertNotIn('"--notes-file", "CHANGELOG.md"', script)
        self.assertIn("GetTempFileName", script)
        self.assertIn("WriteAllText", script)
        self.assertIn("[IO.File]::ReadAllText($ChangelogPath", script)
        self.assertIn("New-Object Text.UTF8Encoding($false)", script)
        self.assertNotIn(
            "$ChangelogText = Get-Content -LiteralPath $ChangelogPath -Raw",
            script,
        )
        self.assertTrue(
            script.isascii(),
            "Windows PowerShell 5.1 release engine must remain ASCII-safe because UTF-8 without BOM is code-page dependent",
        )

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

    def test_release_manifest_requires_effective_windows_settings(self):
        """Comments and unrelated text must not satisfy PE manifest hardening."""
        target = RELEASE_TARGETS[0]
        valid = (ROOT / "build" / "windows" / "wails.exe.manifest").read_text(
            encoding="utf-8"
        )
        cases = (
            ("valid", valid, True),
            (
                "comment-only",
                '<assembly xmlns="urn:schemas-microsoft-com:asm.v1">'
                '<!-- asInvoker permonitorv2 longPathAware -->'
                '</assembly>',
                False,
            ),
            (
                "malformed-xml",
                '<assembly><!-- asInvoker permonitorv2 longPathAware -->',
                False,
            ),
            (
                "entity-declaration",
                '<!DOCTYPE assembly [<!ENTITY bypass "asInvoker permonitorv2 longPathAware">]>'
                '<assembly xmlns="urn:schemas-microsoft-com:asm.v1">&bypass;</assembly>',
                False,
            ),
            (
                "wrong-namespace",
                valid.replace('urn:schemas-microsoft-com:asm.v3', 'urn:invalid-namespace'),
                False,
            ),
            (
                "elevated-privileges",
                valid.replace('level="asInvoker"', 'level="requireAdministrator"')
                .replace('<description>', '<!-- asInvoker --><description>'),
                False,
            ),
            (
                "disabled-dpi",
                valid.replace('permonitorv2,permonitor', 'system')
                .replace('<description>', '<!-- permonitorv2 --><description>'),
                False,
            ),
            (
                "disabled-long-paths",
                valid.replace('>true</longPathAware>', '>false</longPathAware>')
                .replace('<description>', '<!-- longPathAware true --><description>'),
                False,
            ),
        )
        flags = 0x0040 | 0x0100 | 0x8000 | 0x0020
        pe = (
            target.pe_machine,
            {".rsrc": (0, 0, 1, 0)},
            b"",
            0,
            target.pe_magic,
            2 | (flags << 16),
        )
        for case, manifest, should_pass in cases:
            with self.subTest(case=case), patch(
                "verify_windows_resources.parse_pe", return_value=pe
            ), patch(
                "verify_windows_resources.resource_type_ids",
                return_value={3, 14, 16, 24},
            ), patch(
                "verify_windows_resources.resource_payloads",
                side_effect=lambda _path, kind: [
                    manifest.encode("utf-8") if kind == 24 else
                    VALID_ICON_GROUP if kind == 14 else b"fixture"
                ],
            ), patch(
                "verify_windows_resources.resource_icon_ids", return_value={1, 2}
            ), patch(
                "verify_windows_resources.resource_icon_sizes",
                return_value={1: {4}, 2: {4}},
            ), patch("verify_windows_resources.verify_version_resource"):
                if should_pass:
                    verify_target(Path("fixture.exe"), "2.0.5", target)
                else:
                    with self.assertRaises(SystemExit):
                        verify_target(Path("fixture.exe"), "2.0.5", target)

    def test_windows_release_requires_nonempty_icon_image_payloads(self):
        """A group icon alone must not hide missing or empty RT_ICON images."""
        target = RELEASE_TARGETS[0]
        manifest = (ROOT / "build" / "windows" / "wails.exe.manifest").read_bytes()
        flags = 0x0040 | 0x0100 | 0x8000 | 0x0020
        pe = (
            target.pe_machine,
            {".rsrc": (0, 0, 1, 0)},
            b"",
            0,
            target.pe_magic,
            2 | (flags << 16),
        )
        cases = (
            ("no-icon-image", [], False),
            ("empty-icon-image", [b""], False),
            ("valid-icon-image", [b"icon-bytes"], True),
        )
        for label, icon_images, valid in cases:
            with self.subTest(case=label), patch(
                "verify_windows_resources.parse_pe", return_value=pe
            ), patch(
                "verify_windows_resources.resource_type_ids", return_value={3, 14, 16, 24}
            ), patch(
                "verify_windows_resources.resource_payloads",
                side_effect=lambda _path, kind: (
                    icon_images if kind == 3 else
                    [manifest] if kind == 24 else
                    [VALID_ICON_GROUP] if kind == 14 else [b"fixture"]
                ),
            ), patch(
                "verify_windows_resources.resource_icon_ids", return_value={1, 2}
            ), patch(
                "verify_windows_resources.resource_icon_sizes",
                return_value={1: {4}, 2: {4}},
            ), patch("verify_windows_resources.verify_version_resource"):
                if valid:
                    verify_target(Path("fixture.exe"), "2.0.5", target)
                else:
                    with self.assertRaises(SystemExit):
                        verify_target(Path("fixture.exe"), "2.0.5", target)

    def test_windows_release_rejects_invalid_icon_group_directory(self):
        """RT_GROUP_ICON must contain a valid, nonempty icon directory."""
        target = RELEASE_TARGETS[0]
        manifest = (ROOT / "build" / "windows" / "wails.exe.manifest").read_bytes()
        flags = 0x0040 | 0x0100 | 0x8000 | 0x0020
        pe = (
            target.pe_machine,
            {".rsrc": (0, 0, 1, 0)},
            b"",
            0,
            target.pe_magic,
            2 | (flags << 16),
        )
        cases = (
            ("valid", VALID_ICON_GROUP, True),
            ("empty", b"", False),
            ("wrong-type", b"\x00\x00\x02\x00" + VALID_ICON_GROUP[4:], False),
            ("zero-count", b"\x00\x00\x01\x00\x00\x00", False),
            ("truncated-entry", VALID_ICON_GROUP[:-1], False),
            ("zero-icon-id", VALID_ICON_GROUP[:-2] + b"\x00\x00", False),
            ("zero-icon-size", VALID_ICON_GROUP[:14] + b"\x00\x00\x00\x00" + VALID_ICON_GROUP[18:], False),
        )
        for label, group, valid in cases:
            with self.subTest(case=label), patch(
                "verify_windows_resources.parse_pe", return_value=pe
            ), patch(
                "verify_windows_resources.resource_type_ids", return_value={3, 14, 16, 24}
            ), patch(
                "verify_windows_resources.resource_payloads",
                side_effect=lambda _path, kind: (
                    [b"icon"] if kind == 3 else
                    [group] if kind == 14 else
                    [manifest] if kind == 24 else [b"fixture"]
                ),
            ), patch(
                "verify_windows_resources.resource_icon_ids", return_value={1, 2}
            ), patch(
                "verify_windows_resources.resource_icon_sizes",
                return_value={1: {4}, 2: {4}},
            ), patch("verify_windows_resources.verify_version_resource"):
                if valid:
                    verify_target(Path("fixture.exe"), "2.0.5", target)
                else:
                    with self.assertRaises(SystemExit):
                        verify_target(Path("fixture.exe"), "2.0.5", target)

    def test_windows_release_rejects_group_icon_without_matching_rt_icon_id(self):
        """A nonempty RT_ICON of another ID cannot satisfy GRPICONDIR.nID."""
        target = RELEASE_TARGETS[0]
        manifest = (ROOT / "build" / "windows" / "wails.exe.manifest").read_bytes()
        flags = 0x0040 | 0x0100 | 0x8000 | 0x0020
        group_entry = VALID_ICON_GROUP[6:]
        two_image_group = (
            struct.pack("<HHH", 0, 1, 2) + group_entry
            + group_entry[:-2] + struct.pack("<H", 2)
        )
        cases = (
            ("matching-id", 1, VALID_ICON_GROUP, True),
            ("wrong-id", 2, VALID_ICON_GROUP, False),
            ("missing-second-id", 1, two_image_group, False),
        )
        for label, available_id, group, valid in cases:
            with self.subTest(case=label):
                resource = bytearray(0x90)
                struct.pack_into("<HH", resource, 12, 0, 1)
                struct.pack_into("<II", resource, 16, 3, 0x80000020)
                struct.pack_into("<HH", resource, 0x20 + 12, 0, 1)
                struct.pack_into("<II", resource, 0x20 + 16, available_id, 0x80000040)
                struct.pack_into("<HH", resource, 0x40 + 12, 0, 1)
                struct.pack_into("<II", resource, 0x40 + 16, 1033, 0x60)
                struct.pack_into("<IIII", resource, 0x60, 0x1080, 4, 0, 0)
                resource[0x80:0x84] = b"icon"
                pe = (
                    target.pe_machine,
                    {".rsrc": (len(resource), 0x1000, len(resource), 0)},
                    bytes(resource), 0, target.pe_magic, 2 | (flags << 16),
                )
                with patch("verify_windows_resources.parse_pe", return_value=pe), patch(
                    "verify_windows_resources.resource_type_ids",
                    return_value={3, 14, 16, 24},
                ), patch(
                    "verify_windows_resources.resource_payloads",
                    side_effect=lambda _path, kind: (
                        [b"icon"] if kind == 3 else
                        [group] if kind == 14 else
                        [manifest] if kind == 24 else [b"fixture"]
                    ),
                ), patch("verify_windows_resources.verify_version_resource"):
                    if valid:
                        verify_target(Path("fixture.exe"), "2.0.5", target)
                    else:
                        with self.assertRaisesRegex(SystemExit, "RT_ICON"):
                            verify_target(Path("fixture.exe"), "2.0.5", target)

    def test_windows_release_rejects_icon_group_declared_size_mismatch(self):
        """GRPICONDIRENTRY.dwBytesInRes must match its RT_ICON payload."""
        target = RELEASE_TARGETS[0]
        manifest = (ROOT / "build" / "windows" / "wails.exe.manifest").read_bytes()
        flags = 0x0040 | 0x0100 | 0x8000 | 0x0020
        resource = bytearray(0x90)
        struct.pack_into("<HH", resource, 12, 0, 1)
        struct.pack_into("<II", resource, 16, 3, 0x80000020)
        struct.pack_into("<HH", resource, 0x20 + 12, 0, 1)
        struct.pack_into("<II", resource, 0x20 + 16, 1, 0x80000040)
        struct.pack_into("<HH", resource, 0x40 + 12, 0, 1)
        struct.pack_into("<II", resource, 0x40 + 16, 1033, 0x60)
        struct.pack_into("<IIII", resource, 0x60, 0x1080, 4, 0, 0)
        resource[0x80:0x84] = b"icon"
        pe = (
            target.pe_machine,
            {".rsrc": (len(resource), 0x1000, len(resource), 0)},
            bytes(resource), 0, target.pe_magic, 2 | (flags << 16),
        )
        for declared_size, valid in ((4, True), (3, False), (5, False)):
            with self.subTest(declared_size=declared_size), patch(
                "verify_windows_resources.parse_pe", return_value=pe
            ), patch(
                "verify_windows_resources.resource_type_ids",
                return_value={3, 14, 16, 24},
            ), patch(
                "verify_windows_resources.resource_payloads",
                side_effect=lambda _path, kind: (
                    [b"icon"] if kind == 3 else
                    [
                        VALID_ICON_GROUP[:14]
                        + struct.pack("<I", declared_size)
                        + VALID_ICON_GROUP[18:]
                    ]
                    if kind == 14 else
                    [manifest] if kind == 24 else [b"fixture"]
                ),
            ), patch("verify_windows_resources.verify_version_resource"):
                if valid:
                    verify_target(Path("fixture.exe"), "2.0.5", target)
                else:
                    with self.assertRaisesRegex(SystemExit, "icon.*size"):
                        verify_target(Path("fixture.exe"), "2.0.5", target)

    def test_group_icon_entry_reserved_byte_must_be_zero(self):
        """GRPICONDIRENTRY.bReserved must be zero, not an arbitrary byte."""
        for reserved, valid in ((0, True), (1, False), (255, False)):
            with self.subTest(reserved=reserved):
                payload = bytearray(VALID_ICON_GROUP)
                payload[9] = reserved  # Fourth byte of the 14-byte group entry.
                if valid:
                    verify_icon_group(bytes(payload), "amd64")
                else:
                    with self.assertRaisesRegex(SystemExit, "icon group entry"):
                        verify_icon_group(bytes(payload), "amd64")

    def test_windows_release_rejects_duplicate_group_icon_ids(self):
        """An icon group must not reuse one RT_ICON ID for multiple images."""
        target = RELEASE_TARGETS[0]
        manifest = (ROOT / "build" / "windows" / "wails.exe.manifest").read_bytes()
        flags = 0x0040 | 0x0100 | 0x8000 | 0x0020
        pe = (
            target.pe_machine,
            {".rsrc": (0, 0, 1, 0)},
            b"",
            0,
            target.pe_magic,
            2 | (flags << 16),
        )
        first = VALID_ICON_GROUP[6:]
        second = first[:-2] + struct.pack("<H", 2)
        cases = (
            ("distinct", struct.pack("<HHH", 0, 1, 2) + first + second, True),
            ("duplicate", struct.pack("<HHH", 0, 1, 2) + first + first, False),
            (
                "duplicate-different-size",
                struct.pack("<HHH", 0, 1, 2) + first
                + first[:8] + struct.pack("<I", 8) + first[12:],
                False,
            ),
        )
        for label, group, valid in cases:
            with self.subTest(case=label), patch(
                "verify_windows_resources.parse_pe", return_value=pe
            ), patch(
                "verify_windows_resources.resource_type_ids", return_value={3, 14, 16, 24}
            ), patch(
                "verify_windows_resources.resource_payloads",
                side_effect=lambda _path, kind: (
                    [b"icon"] if kind == 3 else
                    [group] if kind == 14 else
                    [manifest] if kind == 24 else [b"fixture"]
                ),
            ), patch(
                "verify_windows_resources.resource_icon_ids", return_value={1, 2}
            ), patch(
                "verify_windows_resources.resource_icon_sizes",
                return_value={1: {4}, 2: {4}},
            ), patch("verify_windows_resources.verify_version_resource"):
                if valid:
                    verify_target(Path("fixture.exe"), "2.0.5", target)
                else:
                    with self.assertRaises(SystemExit):
                        verify_target(Path("fixture.exe"), "2.0.5", target)

    def test_pe_optional_header_must_contain_all_read_fields(self):
        """The section table must not supply missing optional-header flags."""
        import struct

        pe_offset = 0x80
        optional = pe_offset + 24
        for declared_size in (0, 70, 71, 72):
            with (
                self.subTest(declared_size=declared_size),
                tempfile.TemporaryDirectory() as temp,
            ):
                data = bytearray(256)
                data[:2] = b"MZ"
                struct.pack_into("<I", data, 0x3C, pe_offset)
                data[pe_offset : pe_offset + 4] = b"PE\0\0"
                struct.pack_into("<HH", data, pe_offset + 4, 0x8664, 0)
                struct.pack_into("<H", data, pe_offset + 20, declared_size)
                struct.pack_into("<H", data, optional, 0x20B)
                struct.pack_into("<H", data, optional + 68, 2)
                struct.pack_into("<H", data, optional + 70, 0x8160)
                path = Path(temp) / "test.exe"
                path.write_bytes(data)
                if declared_size < 72:
                    with self.assertRaises(ValueError):
                        parse_pe(path)
                else:
                    machine, _sections, _data, _timestamp, magic, flags = parse_pe(
                        path
                    )
                    self.assertEqual(
                        (machine, magic, flags & 0xFFFF), (0x8664, 0x20B, 2)
                    )

    def test_pe_duplicate_section_names_are_rejected(self):
        """Ambiguous .rsrc sections must not select a decoy resource table."""
        import struct

        pe_offset = 0x80
        optional = pe_offset + 24
        section_table = optional + 72
        cases = (
            ("duplicate-resource", (b".rsrc", b".rsrc"), False),
            ("duplicate-text", (b".text", b".text"), False),
            ("distinct", (b".rsrc", b".text"), True),
        )
        for label, names, valid in cases:
            with self.subTest(case=label), tempfile.TemporaryDirectory() as temp:
                data = bytearray(512)
                data[:2] = b"MZ"
                struct.pack_into("<I", data, 0x3C, pe_offset)
                data[pe_offset : pe_offset + 4] = b"PE\0\0"
                struct.pack_into("<HH", data, pe_offset + 4, 0x8664, 2)
                struct.pack_into("<H", data, pe_offset + 20, 72)
                struct.pack_into("<H", data, optional, 0x20B)
                struct.pack_into("<H", data, optional + 68, 2)
                struct.pack_into("<H", data, optional + 70, 0x8160)
                for index, name in enumerate(names):
                    start = section_table + index * 40
                    data[start : start + len(name)] = name
                    struct.pack_into("<IIII", data, start + 8, 16, index * 4096, 16, 256 + index * 16)
                path = Path(temp) / "fixture.exe"
                path.write_bytes(data)
                if valid:
                    self.assertEqual(set(parse_pe(path)[1]), {".rsrc", ".text"})
                else:
                    with self.assertRaises(ValueError):
                        parse_pe(path)

    def test_resource_directory_rejects_duplicate_entry_keys(self):
        """A later resource type/ID must not be hidden by a first match."""
        import struct

        cases = (
            ("duplicate-type", ((16, 0x80000020), (16, 0x80000040)), False),
            ("duplicate-name-offset", ((0x80000020, 0x80000040), (0x80000020, 0x80000060)), False),
            ("distinct-types", ((16, 0x80000020), (24, 0x80000040)), True),
        )
        for label, entries, valid in cases:
            with self.subTest(case=label):
                directory = bytearray(16 + len(entries) * 8)
                named_count = len(entries) if label == "duplicate-name-offset" else 0
                struct.pack_into("<HH", directory, 12, named_count, len(entries) - named_count)
                for index, entry in enumerate(entries):
                    struct.pack_into("<II", directory, 16 + index * 8, *entry)
                if valid:
                    self.assertEqual(directory_entries(directory, 0, len(directory), 0), list(entries))
                else:
                    with self.assertRaises(ValueError):
                        directory_entries(directory, 0, len(directory), 0)

    def test_resource_directory_validates_named_and_id_entry_layout(self):
        """PE resource name entries must precede numeric IDs as counted in the header."""
        import struct

        cases = (
            ("named-in-id-slot", 0, (0x80000020,), False),
            ("id-in-named-slot", 1, (16,), False),
            ("wrong-order", 1, (16, 0x80000020), False),
            ("named-only", 1, (0x80000020,), True),
            ("mixed-valid", 1, (0x80000020, 16), True),
            ("numeric-only", 0, (16, 24), True),
        )
        for label, named_count, keys, valid in cases:
            with self.subTest(case=label):
                directory = bytearray(0x50 if named_count else 16 + len(keys) * 8)
                if named_count:
                    struct.pack_into("<H", directory, 0x20, 2)
                    directory[0x22:0x26] = "OK".encode("utf-16le")
                struct.pack_into(
                    "<HH", directory, 12, named_count, len(keys) - named_count
                )
                for index, key in enumerate(keys):
                    struct.pack_into(
                        "<II", directory, 16 + index * 8, key, 0x80000040
                    )
                if valid:
                    self.assertEqual(
                        [
                            key
                            for key, _ in directory_entries(
                                directory, 0, len(directory), 0
                            )
                        ],
                        list(keys),
                    )
                else:
                    with self.assertRaises(ValueError):
                        directory_entries(directory, 0, len(directory), 0)

    def test_resource_directory_rejects_out_of_bounds_named_strings(self):
        """Named PE resource entries must point to complete UTF-16 names in .rsrc."""
        import struct

        # The high bit selects a name offset relative to the start of .rsrc.
        # Valid names are stored beyond the directory table, not inline.
        cases = (
            ("valid-named", 0x80000040, 2, True),
            ("valid-empty", 0x80000040, 0, True),
            ("pointer-outside", 0x80000080, 2, False),
            ("length-outside", 0x80000040, 0x40, False),
            ("truncated-length-word", 0x8000006F, 2, False),
        )
        for label, name_pointer, length, valid in cases:
            with self.subTest(case=label):
                raw_offset = 32
                section_end = raw_offset + 0x70
                data = bytearray(section_end)
                struct.pack_into("<HH", data, raw_offset + 12, 1, 0)
                struct.pack_into("<II", data, raw_offset + 16, name_pointer, 0x80000028)
                struct.pack_into("<H", data, raw_offset + 0x40, length)
                data[raw_offset + 0x42:raw_offset + 0x46] = "OK".encode("utf-16le")
                if valid:
                    self.assertEqual(
                        directory_entries(data, raw_offset, section_end, 0),
                        [(name_pointer, 0x80000028)],
                    )
                else:
                    with self.assertRaises(ValueError):
                        directory_entries(data, raw_offset, section_end, 0)

    def test_resource_directory_rejects_duplicate_named_text(self):
        """Distinct UTF-16 offsets must not disguise duplicate resource names."""
        import struct

        for second_name, valid in (("A", False), ("B", True)):
            with self.subTest(second_name=second_name):
                directory = bytearray(0x70)
                struct.pack_into("<HH", directory, 12, 2, 0)
                struct.pack_into("<II", directory, 16, 0x80000040, 0x80000028)
                struct.pack_into("<II", directory, 24, 0x80000050, 0x80000030)
                for offset, name in ((0x40, "A"), (0x50, second_name)):
                    struct.pack_into("<H", directory, offset, len(name))
                    directory[offset + 2 : offset + 4] = name.encode("utf-16le")
                if valid:
                    self.assertEqual(len(directory_entries(directory, 0, len(directory), 0)), 2)
                else:
                    with self.assertRaises(ValueError):
                        directory_entries(directory, 0, len(directory), 0)

    def test_resource_data_entry_reserved_field_must_be_zero(self):
        """Nonzero IMAGE_RESOURCE_DATA_ENTRY.Reserved must fail PE verification."""
        for reserved, valid in ((0, True), (1, False), (0xFFFFFFFF, False)):
            with self.subTest(reserved=reserved):
                resource = bytearray(0x100)
                # Type 3 -> icon ID 1 -> language 0x409 -> data entry.
                for directory_offset, name_or_id, target in (
                    (0x00, 3, 0x80000020),
                    (0x20, 1, 0x80000040),
                    (0x40, 0x409, 0x60),
                ):
                    struct.pack_into("<HH", resource, directory_offset + 12, 0, 1)
                    struct.pack_into(
                        "<II", resource, directory_offset + 16, name_or_id, target
                    )
                struct.pack_into("<IIII", resource, 0x60, 0x1080, 4, 0, reserved)
                resource[0x80:0x84] = b"ICON"
                pe = (
                    0x8664, {".rsrc": (0x100, 0x1000, 0x100, 0)},
                    bytes(resource), 0, 0x20B, 2,
                )
                with patch("verify_windows_resources.parse_pe", return_value=pe):
                    if valid:
                        self.assertEqual(resource_payloads(Path("fixture.exe"), 3), [b"ICON"])
                    else:
                        with self.assertRaisesRegex(ValueError, "reserved"):
                            resource_payloads(Path("fixture.exe"), 3)

    def test_resource_directory_rejects_reserved_numeric_id_bits(self):
        """Numeric PE resource IDs are 16-bit; upper bits must be zero."""
        import struct

        cases = (
            ("upper-bit-16", 0x00010010, False),
            ("upper-bit-30", 0x40000010, False),
            ("maximum-id", 0x0000FFFF, True),
            ("ordinary-id", 16, True),
        )
        for label, resource_id, valid in cases:
            with self.subTest(case=label):
                directory = bytearray(24)
                struct.pack_into("<HH", directory, 12, 0, 1)
                struct.pack_into("<II", directory, 16, resource_id, 0x80000040)
                if valid:
                    self.assertEqual(
                        directory_entries(directory, 0, len(directory), 0),
                        [(resource_id, 0x80000040)],
                    )
                else:
                    with self.assertRaises(ValueError):
                        directory_entries(directory, 0, len(directory), 0)

    def test_resource_directory_numeric_ids_must_be_sorted(self):
        """Windows resource ID lookup requires ascending numeric directory keys."""
        import struct

        cases = (
            ("numeric-unsorted", False, (24, 16), False),
            ("mixed-unsorted", True, (24, 16), False),
            ("numeric-sorted", False, (16, 24), True),
            ("mixed-sorted", True, (16, 24), True),
        )
        for label, has_name, ids, valid in cases:
            with self.subTest(case=label):
                directory = bytearray(0x60)
                keys = ((0x80000040,) if has_name else ()) + ids
                struct.pack_into("<HH", directory, 12, int(has_name), len(ids))
                for index, key in enumerate(keys):
                    struct.pack_into("<II", directory, 16 + index * 8, key, 0x80000050)
                if has_name:
                    struct.pack_into("<H", directory, 0x40, 1)
                    directory[0x42:0x44] = "A".encode("utf-16le")
                if valid:
                    self.assertEqual(
                        [key for key, _ in directory_entries(directory, 0, len(directory), 0)],
                        list(keys),
                    )
                else:
                    with self.assertRaises(ValueError):
                        directory_entries(directory, 0, len(directory), 0)

    def test_resource_directory_named_entries_must_be_sorted(self):
        """PE named resource keys must be in ascending UTF-16 code-unit order."""
        import struct

        cases = (
            ("named-unsorted", ("Z", "A"), False, False),
            ("mixed-unsorted", ("Z", "A"), True, False),
            ("named-sorted", ("A", "Z"), False, True),
            ("mixed-sorted", ("A", "Z"), True, True),
            ("unicode-sorted", ("\u00ff", "\u0100"), False, True),
            ("unicode-unsorted", ("\u0100", "\u00ff"), False, False),
            ("prefix-sorted", ("A", "AA"), False, True),
            ("prefix-unsorted", ("AA", "A"), False, False),
        )
        for label, names, has_id, valid in cases:
            with self.subTest(case=label):
                directory = bytearray(0xA0)
                keys = (0x80000050, 0x80000070) + ((16,) if has_id else ())
                struct.pack_into("<HH", directory, 12, 2, int(has_id))
                for index, key in enumerate(keys):
                    struct.pack_into("<II", directory, 16 + index * 8, key, 0x80000040)
                for offset, name in zip((0x50, 0x70), names):
                    encoded = name.encode("utf-16le")
                    struct.pack_into("<H", directory, offset, len(encoded) // 2)
                    directory[offset + 2 : offset + 2 + len(encoded)] = encoded
                if valid:
                    self.assertEqual(
                        [key for key, _ in directory_entries(directory, 0, len(directory), 0)],
                        list(keys),
                    )
                else:
                    with self.assertRaises(ValueError):
                        directory_entries(directory, 0, len(directory), 0)

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
        launcher = (ROOT / "IrisTools.ps1").read_text(encoding="utf-8")
        windows = (ROOT / "scripts" / "windows" / "IrisTools.ps1").read_text(
            encoding="utf-8"
        )
        self.assertEqual(
            [(target.goarch, target.asset_suffix) for target in RELEASE_TARGETS],
            [("amd64", "x64"), ("386", "x86"), ("arm64", "arm64")],
        )
        self.assertIn("scripts\\windows\\IrisTools.ps1", launcher)
        for marker in (
            "release_fingerprint.py",
            "windows/amd64",
            "windows/386",
            "windows/arm64",
            "IrisOnlineDB-$Version-Windows-$($Target.Suffix).exe",
            "webview2",
            "IrisOnlineDatabase.exe",
            "SHA256SUMS.txt",
        ):
            self.assertIn(marker, windows)
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
            "function Repair-ReleaseSources {", 1
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

    def test_release_executable_rejects_unpinned_compiler_metadata(self):
        """A valid Windows binary built with older Go must fail the release gate."""
        pinned = (ROOT / ".go-version").read_text(encoding="utf-8").strip()
        markers = "\n".join(
            f"\t{'dep' if category == 'WAILS_VERSION' else 'build'}\t{marker}"
            for category, marker in EXPECTED_METADATA_MARKERS
        )
        with tempfile.TemporaryDirectory(prefix="iris-go-toolchain-gate-") as directory:
            binary = Path(directory) / "release.exe"
            binary.write_bytes(b"IrisOnlineRelease/2.0.5/")
            for case, header, should_pass in (
                ("older-compiler", "release.exe: go1.23.2", False),
                ("compiler-suffix", f"release.exe: go{pinned}0", False),
                ("missing-compiler", "release.exe: unknown", False),
                (
                    "wrong-first-line",
                    "release.exe: go1.23.2\nrelease.exe: go" + pinned,
                    False,
                ),
                ("pinned-compiler", f"release.exe: go{pinned}", True),
            ):
                with self.subTest(case=case):
                    result = SimpleNamespace(
                        returncode=0, stdout=header + "\n" + markers
                    )
                    with patch(
                        "verify_executables.subprocess.run", return_value=result
                    ):
                        if should_pass:
                            verify_executable(binary, "2.0.5", "amd64", "go")
                        else:
                            with self.assertRaises(SystemExit):
                                verify_executable(binary, "2.0.5", "amd64", "go")

    def test_release_metadata_diagnostic_identifies_cgo_without_raw_payload(self):
        valid = "\n".join(
            f"\t{'dep' if category == 'WAILS_VERSION' else 'build'}\t{marker}"
            for category, marker in EXPECTED_METADATA_MARKERS
        )
        self.assertEqual(missing_metadata_categories(valid), [])
        cgo_enabled = valid.replace("CGO_ENABLED=0", "CGO_ENABLED=1")
        categories = missing_metadata_categories(cgo_enabled)
        self.assertEqual(categories, ["CGO_DISABLED"])
        self.assertNotIn("CGO_ENABLED=1", " ".join(categories))

    def test_release_metadata_rejects_substring_spoofing(self):
        # go version -m emits tab-separated build and dependency records.
        markers = dict(expected_metadata_markers("amd64"))
        legitimate = "\n".join(
            f"\t{'dep' if category == 'WAILS_VERSION' else 'build'}\t{marker}"
            for category, marker in markers.items()
        )
        self.assertEqual(missing_metadata_categories(legitimate), [])
        self.assertEqual(
            missing_metadata_categories(
                legitimate.replace(markers["WAILS_VERSION"], markers["WAILS_VERSION"] + "\th1:checksum")
            ),
            [],
        )
        wrong_record = legitimate.replace("build\tGOOS=windows", "dep\tGOOS=windows")
        self.assertEqual(missing_metadata_categories(wrong_record), ["TARGET_OS"])

        forged = "\n".join(
            f"\t{'dep' if category == 'WAILS_VERSION' else 'build'}\t{marker}SPOOF"
            for category, marker in markers.items()
        )
        self.assertEqual(
            missing_metadata_categories(forged), list(markers),
            "substring matches must not satisfy Go build metadata requirements",
        )

    def test_release_metadata_rejects_trailing_whitespace(self):
        markers = dict(expected_metadata_markers("amd64"))
        legitimate = "\n".join(
            f"\t{'dep' if category == 'WAILS_VERSION' else 'build'}\t{marker}"
            for category, marker in markers.items()
        )
        self.assertEqual(missing_metadata_categories(legitimate), [])
        with self.subTest("trailing space in build value"):
            forged = legitimate.replace("GOOS=windows", "GOOS=windows ")
            self.assertEqual(missing_metadata_categories(forged), ["TARGET_OS"])
        with self.subTest("trailing tab in build record"):
            forged = legitimate.replace("GOOS=windows", "GOOS=windows\t")
            self.assertEqual(missing_metadata_categories(forged), ["TARGET_OS"])
        with self.subTest("empty dependency checksum"):
            forged = legitimate.replace(
                markers["WAILS_VERSION"], markers["WAILS_VERSION"] + "\t"
            )
            self.assertEqual(missing_metadata_categories(forged), ["WAILS_VERSION"])

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
                metadata = "\n".join(
                    f"\t{'dep' if category == 'WAILS_VERSION' else 'build'}\t{marker}"
                    for category, marker in markers.items()
                )
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

    def test_release_fingerprint_rejects_changed_toolchain(self):
        """Build cannot reuse a gate fingerprint after Go/security tool drift."""
        with tempfile.TemporaryDirectory(
            prefix="iris-fingerprint-toolchain-"
        ) as temporary:
            root = Path(temporary)
            subprocess.run(
                ["git", "init", "--initial-branch=main", str(root)],
                check=True,
                capture_output=True,
                timeout=30,
            )
            for key, value in (
                ("user.name", "Iris test"),
                ("user.email", "test@example.invalid"),
            ):
                subprocess.run(
                    ["git", "-C", str(root), "config", key, value],
                    check=True,
                    capture_output=True,
                    timeout=30,
                )
            (root / "VERSION").write_text("2.0.5\n", encoding="utf-8")
            subprocess.run(
                ["git", "-C", str(root), "add", "VERSION"],
                check=True,
                capture_output=True,
                timeout=30,
            )
            subprocess.run(
                ["git", "-C", str(root), "commit", "-m", "fixture"],
                check=True,
                capture_output=True,
                timeout=30,
            )
            original = {"go": "go1.26.6", "govulncheck": "v1"}
            with patch("release_fingerprint.toolchain", return_value=original):
                write_fingerprint(root, "main")
                verify_fingerprint(root, "main")
            for changed in (
                {"go": "go1.25.0", "govulncheck": "v1"},
                {"go": "go1.26.6", "govulncheck": "v2"},
            ):
                with self.subTest(changed=changed):
                    with patch("release_fingerprint.toolchain", return_value=changed):
                        with self.assertRaises(FingerprintError):
                            verify_fingerprint(root, "main")

    def test_python_requirements_reject_normalized_duplicate_pins(self):
        """PEP 503-equivalent names must not bypass unique direct pins."""
        duplicate_pairs = (
            ("pip-audit", "pip_audit"),
            ("pip-audit", "pip.audit"),
            ("pip--audit", "PIP__AUDIT"),
            ("PlayWright", "playwright"),
        )
        with tempfile.TemporaryDirectory() as temporary:
            requirements = Path(temporary) / "requirements.txt"
            for first, second in duplicate_pairs:
                with self.subTest(first=first, second=second):
                    requirements.write_text(
                        f"{first}==1.0\n{second}==2.0\n", encoding="utf-8"
                    )
                    with self.assertRaises(ValueError):
                        expected_versions(requirements)
            requirements.write_text(
                "pip-audit==2.10.1\nplaywright==1.62.0\n", encoding="utf-8"
            )
            self.assertEqual(
                expected_versions(requirements),
                {"pip-audit": "2.10.1", "playwright": "1.62.0"},
            )

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
            ROOT / "scripts" / "windows" / "IrisTools.ps1",
        )
        combined = "\n".join(path.read_text(encoding="utf-8") for path in paths)
        self.assertNotIn("--source", combined)
        self.assertIn("data_presentation_audit.py", combined)
        self.assertIn("function Test-GitleaksDetection", combined)
        self.assertIn("Gitleaks functional detection self-test failed.", combined)
        self.assertIn("Invoke-GitleaksHistoryScan", combined)
        self.assertIn("Test-GitleaksHistoryProof", combined)
        self.assertIn("ConvertFrom-AnsiToolOutput", combined)
        self.assertIn("commits scanned", combined)
        self.assertIn("--exit-code", combined)
        self.assertNotIn("python3 -B tools/raw_projection_audit.py\n", combined)
        self.assertNotIn("python3 -B tools/drop_table_audit.py\n", combined)

    def test_release_requires_successful_ci_codeql_and_unchanged_artifact(self):
        script = (ROOT / "scripts" / "windows" / "IrisTools.ps1").read_text(
            encoding="utf-8"
        )
        for check in (
            "Windows quality and security",
            "Windows race detector",
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
        self.assertIn("govulncheck@v1.6.0", workflow)
        self.assertNotIn("ubuntu-", workflow)
        self.assertNotIn("ubuntu-", codeql)

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
        self.assertLess(
            prepare.index("Invoke-GitFetchMain"), prepare.index("Repair-ReleaseSources")
        )
        self.assertLess(
            prepare.index("Repair-ReleaseSources"), prepare.index("Test-Release")
        )
        self.assertLess(prepare.index("Test-Release"), prepare.index("Build-Release"))
        self.assertIn("Test-Release -SkipToolingCheck", prepare)
        gate = script.split("function Test-Release {", 1)[1].split(
            "function Clear-BuildGenerated {", 1
        )[0]
        self.assertLess(
            gate.index("Clear-BuildGenerated"), gate.index("repository_audit.py")
        )
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

    def test_prepare_release_safe_autofix_is_deterministic_and_fail_closed(self):
        script = (ROOT / "scripts" / "windows" / "IrisTools.ps1").read_text(
            encoding="utf-8"
        )
        repair = script.split("function Repair-ReleaseSources {", 1)[1].split(
            "function Test-Release {", 1
        )[0]
        for marker in (
            'Invoke-Checked "gofmt" $GofmtArguments',
            'Invoke-Checked "go" @("mod", "tidy")',
            '@("check", "--fix", "--no-cache", ".")',
            '@("format", "--no-cache", ".")',
            'Invoke-Checked "go" @("mod", "tidy", "-diff")',
            '@("check", "--no-cache", ".")',
            '@("format", "--check", "--no-cache", ".")',
            'git merge-base --is-ancestor "origin/main" HEAD',
            "commit.gpgSign=false",
            '"commit", "--amend", "--no-edit"',
            "Assert-CleanTree",
        ):
            self.assertIn(marker, repair)
        self.assertNotIn("--unsafe-fixes", repair)
        self.assertIn("unexpected untracked files", repair)
        self.assertIn("already-published origin/main", repair)
        self.assertLess(
            repair.index('check", "--fix"'), repair.index('check", "--no-cache"')
        )
        self.assertLess(
            repair.index('format", "--no-cache"'),
            repair.index('format", "--check"'),
        )

    def test_release_tooling_is_windows_only_and_uses_verified_resolvers(self):
        for name in ("release-tools.sh", "release-gate.sh", "build-release.sh"):
            self.assertFalse((ROOT / "scripts" / name).exists(), name)
        windows = (ROOT / "scripts" / "windows" / "IrisTools.ps1").read_text(
            encoding="utf-8"
        )
        self.assertIn("& $GoExecutable.Source version -m $Executable 2>&1", windows)
        self.assertIn("github.com/wailsapp/wails/v2/cmd/wails", windows)
        self.assertIn("Get-PinnedWailsExecutable", windows)
        self.assertIn("Get-PinnedStaticcheckExecutable", windows)
        self.assertIn("Get-PinnedGovulncheckExecutable", windows)
        self.assertIn("Invoke-Govulncheck", windows)
        self.assertIn("Test-GitleaksDetection", windows)
        self.assertIn("Invoke-GitleaksHistoryScan", windows)
        self.assertNotIn('Invoke-Checked "wails" @(', windows)

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

    def test_release_assets_reject_unexpected_directory_entries(self):
        # A release staging directory must contain only the four approved files.
        import hashlib

        with tempfile.TemporaryDirectory(prefix="iris-release-dir-entries-") as temporary:
            directory = Path(temporary)
            manifest = []
            for index, name in enumerate(expected_asset_names("2.0.0")[:-1], 1):
                payload = b"MZ" + bytes([index])
                (directory / name).write_bytes(payload)
                manifest.append(f"{hashlib.sha256(payload).hexdigest()}  {name}")
            (directory / "SHA256SUMS.txt").write_text(
                "\n".join(manifest) + "\n", encoding="ascii"
            )
            verify_release_assets(directory, "2.0.0")
            (directory / "staging").mkdir()
            with self.assertRaises(SystemExit):
                verify_release_assets(directory, "2.0.0")

    def test_release_assets_reject_symlinked_entries(self):
        import hashlib

        with tempfile.TemporaryDirectory(prefix="iris-release-symlink-") as temporary:
            root = Path(temporary)
            directory = root / "staging"
            directory.mkdir()
            names = expected_asset_names("2.0.0")[:-1]
            manifest = []
            for index, name in enumerate(names, start=1):
                payload = b"MZ" + bytes([index])
                (directory / name).write_bytes(payload)
                manifest.append(f"{hashlib.sha256(payload).hexdigest()}  {name}")
            (directory / "SHA256SUMS.txt").write_text(
                "\n".join(manifest) + "\n", encoding="ascii"
            )
            verify_release_assets(directory, "2.0.0")
            for index, name in enumerate((*names, "SHA256SUMS.txt")):
                with self.subTest(name=name):
                    original = directory / name
                    outside = root / f"outside-{index}"
                    original.rename(outside)
                    try:
                        original.symlink_to(outside)
                    except (OSError, NotImplementedError) as error:
                        outside.rename(original)
                        self.skipTest(f"Symlinks unavailable: {error}")
                    try:
                        with self.assertRaises(SystemExit):
                            verify_release_assets(directory, "2.0.0")
                    finally:
                        original.unlink()
                        outside.rename(original)

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
        self.assertEqual(script.count('"refs/tags/${Tag}:refs/tags/${Tag}"'), 2)
        self.assertNotIn('"refs/tags/$Tag:refs/tags/$Tag"', script)
        self.assertIn("GIT_TAG_REFSPEC_INTERPOLATION", script)
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
        self.assertIn("function Test-Release {", script)
        self.assertIn("if (-not $SkipToolingCheck) { Test-WindowsTooling }", script)
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
        self.assertIn('"--refetch"', script)
        self.assertNotIn('"--negotiation-tip=$OriginMain"', script)
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

    def test_repository_audit_scans_its_own_source_for_sensitive_content(self):
        fake_token = "ghp_" + "A" * 36
        sensitive_path = "/" + "home/" + "private-user/work/project"
        for payload, category in (
            (fake_token, "SEC001"),
            (sensitive_path, "SEC002"),
        ):
            with self.subTest(category=category):
                with tempfile.TemporaryDirectory(prefix="iris-self-audit-") as temporary:
                    root = Path(temporary)
                    tools = root / "tools"
                    tools.mkdir()
                    (tools / "repository_audit.py").write_text(
                        "# self-audit fixture\n" + payload + "\n", encoding="utf-8"
                    )
                    result = self.run_audit(root)
                    self.assertNotEqual(result.returncode, 0)
                    self.assertIn(f"[{category}]", result.stdout)
                    self.assertNotIn(payload, result.stdout)

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
                # runnable by a normal non-elevated user. Manifest rejection is
                # still tested even when this optional Windows privilege is absent.
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

    def test_release_tooling_has_no_posix_release_scripts(self):
        self.assertFalse(any((ROOT / "scripts").glob("*.sh")))
        self.assertTrue((ROOT / "IrisTools.ps1").is_file())
        self.assertTrue((ROOT / "scripts" / "windows" / "IrisTools.ps1").is_file())
        self.assertTrue(
            (ROOT / "scripts" / "windows" / "00_RELEASE_WINDOWS.bat").is_file()
        )

    def test_frontend_smoke_redacts_missing_browser_details(self):
        sensitive = "/" + "home/private-user/playwright/chrome"
        error = PlaywrightError(f"Executable doesn't exist at {sensitive}")
        category = playwright_failure_category(error)
        self.assertEqual(category, "BROWSER_MISSING")
        self.assertNotIn(sensitive, category)
        self.assertNotIn("private-user", category)


if __name__ == "__main__":
    unittest.main()
