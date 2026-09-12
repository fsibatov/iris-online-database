from __future__ import annotations

import os
import shutil
import subprocess
import tempfile
import unittest
from pathlib import Path
from unittest import mock

from restore_repository import restore_repository, source_snapshot


@unittest.skipUnless(shutil.which("git"), "Git is required")
class RepositoryRestoreTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(prefix="iris-git-test-")
        self.addCleanup(self.temporary.cleanup)
        self.directory = Path(self.temporary.name)
        self.remote = self.directory / "remote"
        self.remote.mkdir()
        self.git(self.remote, "init", "-b", "main")
        self.git(self.remote, "config", "user.name", "Fixture")
        self.git(self.remote, "config", "user.email", "fixture@example.invalid")
        self.git(self.remote, "config", "commit.gpgSign", "false")
        (self.remote / "VERSION").write_text("2.0.5\n")
        (self.remote / "wails.json").write_text("{}\n")
        (self.remote / "removed.txt").write_text("previous file\n")
        self.git(self.remote, "add", ".")
        self.git(self.remote, "commit", "-m", "fixture")
        self.base = self.git(self.remote, "rev-parse", "HEAD").strip()
        self.root = self.directory / "Архив с пробелами"
        self.root.mkdir()
        (self.root / "tools").mkdir()
        (self.root / "tools/archive_base.txt").write_text(self.base + "\n")
        (self.root / "VERSION").write_text("2.0.6\n")
        (self.root / "wails.json").write_text("{}\n")
        (self.root / "new.txt").write_bytes("Новые данные\r\n".encode())

    def git(self, root, *arguments):
        environment = os.environ.copy()
        for key in ("GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE"):
            environment.pop(key, None)
        return subprocess.check_output(
            ["git", "-C", str(root), *arguments],
            env=environment,
            text=True,
            encoding="utf-8",
            stderr=subprocess.PIPE,
            timeout=30,
        )

    def restore(self):
        return restore_repository(
            self.root, "Fixture", "fixture@example.invalid", str(self.remote)
        )

    def test_fresh_archive_keeps_bytes_and_history_and_becomes_clean(self):
        before = source_snapshot(self.root)
        self.assertTrue(self.restore())
        self.assertEqual(before, source_snapshot(self.root))
        self.assertEqual(self.git(self.root, "status", "--porcelain").strip(), "")
        self.assertEqual(
            self.git(self.root, "branch", "--show-current").strip(), "main"
        )
        self.assertEqual(self.git(self.root, "rev-parse", "HEAD^").strip(), self.base)
        self.assertEqual(
            self.git(self.root, "rev-parse", "origin/main").strip(), self.base
        )
        self.assertFalse((self.root / "removed.txt").exists())
        self.assertEqual(list(self.directory.glob("iris-git-recovery-*")), [])

    def test_existing_repository_and_local_edits_are_untouched(self):
        self.restore()
        head = self.git(self.root, "rev-parse", "HEAD")
        (self.root / "new.txt").write_bytes(b"local edit\n")
        before = source_snapshot(self.root)
        self.assertFalse(self.restore())
        self.assertEqual(before, source_snapshot(self.root))
        self.assertEqual(self.git(self.root, "rev-parse", "HEAD"), head)

    def test_newer_main_rejects_stale_archive_without_partial_git(self):
        (self.remote / "newer.txt").write_text("new remote work\n")
        self.git(self.remote, "add", ".")
        self.git(self.remote, "commit", "-m", "remote advanced")
        before = source_snapshot(self.root)
        with self.assertRaisesRegex(ValueError, "GitHub main changed"):
            self.restore()
        self.assertFalse((self.root / ".git").exists())
        self.assertEqual(before, source_snapshot(self.root))

    def test_network_failure_leaves_source_and_metadata_untouched(self):
        before = source_snapshot(self.root)
        with self.assertRaisesRegex(ValueError, "Git recovery step failed"):
            restore_repository(
                self.root,
                "Fixture",
                "fixture@example.invalid",
                str(self.directory / "missing"),
            )
        self.assertFalse((self.root / ".git").exists())
        self.assertEqual(before, source_snapshot(self.root))

    def test_existing_invalid_git_is_not_replaced(self):
        (self.root / ".git").mkdir()
        marker = self.root / ".git/keep.txt"
        marker.write_bytes(b"preserve\n")
        with self.assertRaises(ValueError):
            self.restore()
        self.assertEqual(marker.read_bytes(), b"preserve\n")

    def test_parent_repository_and_inherited_git_environment_are_not_used(self):
        self.git(self.directory, "init", "-b", "parent")
        with mock.patch.dict(
            os.environ,
            {
                "GIT_DIR": str(self.directory / ".git"),
                "GIT_WORK_TREE": str(self.directory),
            },
        ):
            self.assertTrue(self.restore())
        self.assertEqual(
            self.git(self.root, "branch", "--show-current").strip(), "main"
        )
        self.assertEqual(
            self.git(self.directory, "branch", "--show-current").strip(), "parent"
        )
