from __future__ import annotations

import argparse
import io
import json
import os
import shutil
import subprocess  # nosec B404
import tempfile
import zipfile
from pathlib import Path

from release_fingerprint import FingerprintError, assert_release_tree

ROOT = Path(__file__).resolve().parents[1]
PREFIX = "iris-online-database/"


def build_source_archive(root: Path, output: Path, branch: str) -> str:
    root = root.resolve()
    output = output.resolve()
    if output == root or root in output.parents or output.suffix.lower() != ".zip":
        raise ValueError("The source ZIP must be outside the repository.")
    head, _ = assert_release_tree(root, branch)
    executable = shutil.which("git")
    if not executable:
        raise ValueError("Git executable is unavailable.")
    archive = subprocess.run(  # nosec B603
        [
            executable,
            "-C",
            str(root),
            "archive",
            "--format=zip",
            f"--prefix={PREFIX}",
            head,
        ],
        check=True,
        capture_output=True,
        timeout=120,
    ).stdout
    output.parent.mkdir(parents=True, exist_ok=True)
    with tempfile.NamedTemporaryFile(dir=output.parent, delete=False) as temporary:
        temporary_path = Path(temporary.name)
    try:
        with (
            zipfile.ZipFile(io.BytesIO(archive)) as source,
            zipfile.ZipFile(
                temporary_path, "w", compression=zipfile.ZIP_DEFLATED
            ) as target,
        ):
            metadata = PREFIX + "tools/archive_base.txt"
            if metadata not in source.namelist():
                raise ValueError("Source archive commit metadata is missing.")
            for entry in source.infolist():
                content = source.read(entry)
                if entry.filename == metadata:
                    content = (head + "\n").encode("ascii")
                elif entry.filename.endswith((".ps1", ".bat")):
                    content = content.replace(b"\r\n", b"\n").replace(b"\n", b"\r\n")
                target.writestr(entry, content)
        if assert_release_tree(root, branch)[0] != head:
            raise ValueError("Source HEAD changed during archive creation.")
        os.replace(temporary_path, output)
    finally:
        temporary_path.unlink(missing_ok=True)
    return head


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--output", required=True, type=Path)
    arguments = parser.parse_args()
    try:
        config = json.loads((ROOT / "build/release.json").read_text(encoding="utf-8"))
        branch = config.get("sourceBranch", "main")
        if not isinstance(branch, str) or not branch:
            raise ValueError("Archive source branch is invalid.")
        head = build_source_archive(ROOT, arguments.output, branch)
    except (
        OSError,
        ValueError,
        FingerprintError,
        subprocess.SubprocessError,
        zipfile.BadZipFile,
    ) as error:
        print(f"Source archive: FAIL: {error}")
        return 1
    print(f"Source archive: PASS commit={head}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
