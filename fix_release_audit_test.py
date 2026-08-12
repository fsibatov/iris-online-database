from __future__ import annotations

import shutil
import subprocess
import sys
import tempfile
from pathlib import Path

BASE = Path(__file__).resolve().parent
TARGET = BASE / "tools" / "test_release_helpers.py"

OLD_DIR = '        self.assertIn("forbidden release directory: __pycache__/", result.stdout)'
OLD_FILE = '        self.assertIn("forbidden release file: $coverage", result.stdout)'

# Also accept the earlier intermediate wording if it is present locally.
OLD_DIR_GENERIC = '        self.assertIn("forbidden release directory detected", result.stdout)'
OLD_FILE_GENERIC = '        self.assertIn("forbidden release file detected", result.stdout)'

NEW_BLOCK = '''        failure_lines = [\n            line for line in result.stdout.splitlines() if line.startswith("FAIL: ")\n        ]\n        self.assertGreaterEqual(len(failure_lines), 2, result.stdout)\n        self.assertNotIn("__pycache__/", result.stdout)\n        self.assertNotIn("$coverage", result.stdout)'''


def main() -> int:
    if not TARGET.is_file():
        print(f"[FAIL] File not found: {TARGET}")
        print("Extract this ZIP into the root of iris-online-source-1.1.0 and run again.")
        return 2

    original = TARGET.read_text(encoding="utf-8")

    if (
        "failure_lines = [" in original
        and 'self.assertNotIn("__pycache__/", result.stdout)' in original
        and 'self.assertNotIn("$coverage", result.stdout)' in original
    ):
        print("[OK] test_release_helpers.py is already updated.")
        return run_targeted_test()

    dir_marker = OLD_DIR if OLD_DIR in original else OLD_DIR_GENERIC if OLD_DIR_GENERIC in original else None
    file_marker = OLD_FILE if OLD_FILE in original else OLD_FILE_GENERIC if OLD_FILE_GENERIC in original else None

    if dir_marker is None or file_marker is None:
        print("[FAIL] Expected old assertions were not found; nothing was changed.")
        print("This prevents overwriting an unknown/newer version of the test file.")
        return 3

    lines = original.splitlines()
    try:
        dir_index = lines.index(dir_marker)
        file_index = lines.index(file_marker)
    except ValueError:
        print("[FAIL] Could not locate old assertions safely; nothing was changed.")
        return 3

    if file_index != dir_index + 1:
        print("[FAIL] Old assertions are not adjacent; refusing an unsafe automatic edit.")
        return 3

    replacement = NEW_BLOCK.splitlines()
    updated_lines = lines[:dir_index] + replacement + lines[file_index + 1 :]
    newline = "\r\n" if "\r\n" in original else "\n"
    updated = newline.join(updated_lines)
    if original.endswith(("\n", "\r\n")):
        updated += newline

    backup = Path(tempfile.gettempdir()) / "iris_test_release_helpers_before_redaction_fix.py"
    shutil.copy2(TARGET, backup)

    TARGET.write_text(updated, encoding="utf-8", newline="")
    print(f"[OK] Updated: {TARGET}")
    print(f"[i] Safety backup: {backup}")

    rc = run_targeted_test()
    if rc != 0:
        shutil.copy2(backup, TARGET)
        print("[FAIL] Targeted regression test failed; original file restored.")
        return rc

    print("[OK] Targeted regression test PASS.")
    return 0


def run_targeted_test() -> int:
    command = [
        sys.executable,
        "-B",
        "-m",
        "unittest",
        "discover",
        "-s",
        "tools",
        "-p",
        "test_release_helpers.py",
    ]
    print("[i] Running test_release_helpers.py ...")
    completed = subprocess.run(command, cwd=BASE, check=False)
    return completed.returncode


if __name__ == "__main__":
    raise SystemExit(main())
