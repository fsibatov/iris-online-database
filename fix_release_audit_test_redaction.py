from __future__ import annotations

import re
import sys
from pathlib import Path


def find_target(base: Path) -> Path:
    direct = base / "tools" / "test_release_helpers.py"
    if direct.is_file():
        return direct

    candidates = sorted(
        (
            p
            for p in base.glob("iris-online-source-*/tools/test_release_helpers.py")
            if p.is_file()
        ),
        key=lambda p: p.stat().st_mtime,
        reverse=True,
    )
    if not candidates:
        raise SystemExit(
            "Не найден tools/test_release_helpers.py. Запустите FIX_RELEASE_AUDIT_TEST_REDACTION.bat "
            "из корня iris-online-source-* либо из папки, где находится iris-online-source-*."
        )
    return candidates[0]


def replace_assertion(
    text: str,
    *,
    old_message: str,
    new_message: str,
    forbidden_fragment: str,
) -> tuple[str, bool]:
    # Match the exact old unittest assertion while preserving indentation and quote style.
    pattern = re.compile(
        r"^(?P<indent>[ \t]*)self\.assertIn\("
        + r"(?P<quote>['\"])"
        + re.escape(old_message)
        + r"(?P=quote),[ \t]*result\.stdout\)[ \t]*$",
        re.MULTILINE,
    )

    match = pattern.search(text)
    if not match:
        return text, False

    indent = match.group("indent")
    replacement = (
        f'{indent}self.assertIn("{new_message}", result.stdout)\n'
        f'{indent}self.assertNotIn("{forbidden_fragment}", result.stdout)'
    )
    return text[: match.start()] + replacement + text[match.end() :], True


def main() -> int:
    path = find_target(Path.cwd().resolve())
    original = path.read_text(encoding="utf-8-sig")
    text = original

    text, changed_dir = replace_assertion(
        text,
        old_message="forbidden release directory: __pycache__/",
        new_message="forbidden release directory detected",
        forbidden_fragment="__pycache__/",
    )
    text, changed_file = replace_assertion(
        text,
        old_message="forbidden release file: $coverage",
        new_message="forbidden release file detected",
        forbidden_fragment="$coverage",
    )

    required = (
        'self.assertIn("forbidden release directory detected", result.stdout)',
        'self.assertNotIn("__pycache__/", result.stdout)',
        'self.assertIn("forbidden release file detected", result.stdout)',
        'self.assertNotIn("$coverage", result.stdout)',
    )

    if not (changed_dir or changed_file):
        if all(item in text for item in required):
            print(f"[OK] Исправление уже присутствует: {path}")
            return 0
        raise SystemExit(
            "Ожидаемые старые assertions не найдены, а полный новый набор проверок отсутствует. "
            "Файл отличается от ожидаемой версии; автоматическое изменение остановлено."
        )

    missing = [item for item in required if item not in text]
    if missing:
        raise SystemExit(
            "После изменения не удалось подтвердить полный набор безопасных assertions: "
            + "; ".join(missing)
        )

    # Do not leave a .bak file inside source: repository_audit intentionally rejects it.
    path.write_text(text, encoding="utf-8", newline="\n")

    try:
        compile(text, str(path), "exec")
    except SyntaxError as exc:
        # Restore original bytes if our edit somehow made the file invalid.
        path.write_text(original, encoding="utf-8", newline="\n")
        raise SystemExit(f"Синтаксическая проверка не прошла; исходный файл восстановлен: {exc}") from exc

    print(f"[FIXED] {path}")
    print("[OK] Старые подробные expectations заменены на безопасные категории.")
    print("[OK] Добавлены assertNotIn для __pycache__/ и $coverage.")
    print("Теперь запустите 06_RUFF_FORMAT.bat, затем 01_TEST.bat.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
