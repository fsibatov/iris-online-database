from __future__ import annotations

import argparse
import hashlib
import json
import os
import re
import shutil
import subprocess  # nosec B404
import tempfile
from pathlib import Path

from repository_audit import audit

ROOT = Path(__file__).resolve().parents[1]
REPOSITORY_URL = "https://github.com/fsibatov/iris-online-database.git"


def source_snapshot(root: Path) -> dict[str, str]:
    return {
        path.relative_to(root).as_posix(): hashlib.sha256(path.read_bytes()).hexdigest()
        for path in root.rglob("*")
        if path.is_file() and ".git" not in path.relative_to(root).parts
    }


def restore_repository(
    root: Path,
    name: str,
    email: str,
    repository: str = REPOSITORY_URL,
    branch: str = "main",
) -> bool:
    root = root.resolve()
    git = shutil.which("git")
    if not git:
        raise ValueError("Git is missing. Run INSTALL/UPDATE TOOLS first.")
    environment = os.environ.copy()
    for key in (
        "GIT_DIR",
        "GIT_WORK_TREE",
        "GIT_INDEX_FILE",
        "GIT_COMMON_DIR",
        "GIT_OBJECT_DIRECTORY",
        "GIT_ALTERNATE_OBJECT_DIRECTORIES",
    ):
        environment.pop(key, None)

    def run(
        directory: Path, *arguments: str, allowed: tuple[int, ...] = (0,)
    ) -> subprocess.CompletedProcess[str]:
        result = subprocess.run(  # nosec B603
            [git, "-C", str(directory), *arguments],
            env=environment,
            check=False,
            capture_output=True,
            text=True,
            encoding="utf-8",
            errors="replace",
            timeout=300,
        )
        if result.returncode not in allowed:
            raise ValueError(
                f"Git recovery step failed with exit code {result.returncode}: "
                f"{result.stderr.strip()}"
            )
        return result

    run(root, "check-ref-format", "--branch", branch)
    destination = root / ".git"
    if os.path.lexists(destination):
        top = run(root, "rev-parse", "--show-toplevel").stdout.strip()
        if Path(top).resolve() != root:
            raise ValueError("Existing Git metadata points to another source tree.")
        return False

    base_file = root / "tools" / "archive_base.txt"
    base = base_file.read_text(encoding="ascii").strip()
    if not re.fullmatch(r"[0-9a-f]{40}", base):
        raise ValueError("Archive base commit is invalid.")
    if not (root / "VERSION").is_file() or not (root / "wails.json").is_file():
        raise ValueError("The complete project archive is required.")
    findings, _ = audit(root)
    if findings:
        raise ValueError("Source audit failed. Git history was not changed.")
    before = source_snapshot(root)

    with tempfile.TemporaryDirectory(
        prefix="iris-git-recovery-", dir=root.parent
    ) as temporary:
        staging = Path(temporary) / "repository"
        run(
            Path(temporary),
            "clone",
            "--no-checkout",
            "--single-branch",
            "--branch",
            branch,
            "--no-hardlinks",
            "--",
            repository,
            str(staging),
        )
        run(staging, "config", "--local", "core.filemode", "false")
        run(staging, "reset", "--mixed", "HEAD")
        remote_head = run(staging, "rev-parse", "HEAD").stdout.strip()
        work_tree = f"--work-tree={root}"
        status = run(
            staging, work_tree, "status", "--porcelain=v1", "--untracked-files=all"
        ).stdout.strip()
        if remote_head != base and status:
            raise ValueError(
                f"GitHub {branch} changed since this archive was prepared. "
                "Update the archive before restoring Git; local files are preserved."
            )
        if source_snapshot(root) != before:
            raise ValueError("Source files changed during Git recovery. Try again.")
        run(staging, "config", "--local", "user.name", name)
        run(staging, "config", "--local", "user.email", email)
        if status:
            run(staging, work_tree, "add", "--all")
            changed = run(
                staging, work_tree, "diff", "--cached", "--quiet", allowed=(0, 1)
            ).returncode
            if changed:
                version = (root / "VERSION").read_text(encoding="ascii").strip()
                run(
                    staging,
                    work_tree,
                    "-c",
                    "commit.gpgSign=false",
                    "commit",
                    "-m",
                    f"{version}: restore release sources from archive",
                )
        remaining = run(
            staging, work_tree, "status", "--porcelain=v1", "--untracked-files=all"
        ).stdout.strip()
        if remaining or source_snapshot(root) != before:
            raise ValueError("Source files changed during Git recovery. Try again.")
        if os.path.lexists(destination):
            raise ValueError("Git metadata appeared during recovery; it was preserved.")
        (staging / ".git").rename(destination)
    return True


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--name", required=True)
    parser.add_argument("--email", required=True)
    arguments = parser.parse_args()
    try:
        config = json.loads((ROOT / "build/release.json").read_text(encoding="utf-8"))
        branch = config.get("sourceBranch", "main")
        if not isinstance(branch, str) or not branch:
            raise ValueError("Archive source branch is invalid.")
        restored = restore_repository(
            ROOT, arguments.name, arguments.email, branch=branch
        )
    except (OSError, ValueError, subprocess.SubprocessError) as error:
        print(f"Git repository recovery: FAIL: {error}")
        return 1
    if restored:
        print("Git repository recovered; source files preserved and committed locally.")
    else:
        print("Git repository: ready.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
