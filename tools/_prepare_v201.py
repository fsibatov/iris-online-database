from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]


def replace_exact(path, old, new, *, expected=None, minimum=None):
    target = ROOT / path
    text = target.read_text(encoding="utf-8")
    count = text.count(old)
    if expected is not None and count != expected:
        raise SystemExit(f"{path}: expected {expected} occurrences, found {count}")
    if minimum is not None and count < minimum:
        raise SystemExit(f"{path}: expected at least {minimum} occurrences, found {count}")
    if count == 0:
        raise SystemExit(f"{path}: source text not found")
    target.write_text(text.replace(old, new), encoding="utf-8")


def replace_block(path, old, new):
    target = ROOT / path
    text = target.read_text(encoding="utf-8")
    if old not in text:
        raise SystemExit(f"{path}: source block not found")
    if text.count(old) != 1:
        raise SystemExit(f"{path}: source block is not unique")
    target.write_text(text.replace(old, new, 1), encoding="utf-8")


version_file = ROOT / "VERSION"
if version_file.read_text(encoding="utf-8") != "2.0.0\n":
    raise SystemExit("VERSION is not the expected 2.0.0 baseline")
version_file.write_text("2.0.1\n", encoding="utf-8")

replace_exact("web/app.js", "const APP_VERSION = '2.0.0';", "const APP_VERSION = '2.0.1';", expected=1)
replace_exact("web/index.html", "2.0.0", "2.0.1", expected=2)
replace_exact("README.md", "2.0.0", "2.0.1", minimum=5)
replace_exact("wails.json", '"productVersion": "2.0.0"', '"productVersion": "2.0.1"', expected=1)
replace_exact("build/windows/info.json", "2.0.0", "2.0.1", expected=4)

changelog = ROOT / "CHANGELOG.md"
changelog_text = changelog.read_text(encoding="utf-8")
if "## 2.0.1 — 2026-08-19" in changelog_text:
    raise SystemExit("CHANGELOG already contains 2.0.1")
header = "# История изменений\n\n"
if not changelog_text.startswith(header):
    raise SystemExit("CHANGELOG header changed unexpectedly")
entry = """## 2.0.1 — 2026-08-19

### Fixed

- Переходы между страницами больше не показывают промежуточный экран загрузки поверх уже открытой страницы, поэтому навигация не должна кратковременно мигать.
- Рецепты исключены из каталога «Предметы» и результатов глобального поиска по предметам; старые ссылки `#item/<id>` на рецепты переводятся в раздел «Рецепты».
- Рецепты в «Избранном» сохраняют представление рецепта, а материалы рецепта больше не дублируются в обычной карточке предмета.
- VK updater устойчивее к временной недоступности VK, удалённым и скрытым записям, устаревшим ID и параллельным push; сомнительное понижение ID требует повторного подтверждения.
- Повторная проверка одного и того же VK-поста не создаёт новый commit только из-за косметических различий текста, emoji или разметки.
- Локальные Linux/Windows release-gate снова запускают Python regression-тесты из актуального каталога `tests/`, а не из прежнего `tools/`.

### Changed

- Главная страница стала короче: удалены жёстко заданные счётчики монстров и повторяющиеся пояснения о серверах.
- Убрана принудительная плавная прокрутка; форматтеры чисел и дат переиспользуются вместо повторного создания.
- Python regression-тесты вынесены из `tools/` в отдельный каталог `tests/`; CI запрещает возвращение `tools/test_*.py`.
- Scheduled VK workflow использует системный Chrome/Chromium GitHub Runner вместо повторной тяжёлой установки браузера.
- Для изменений VK updater добавлена live-проверка после push в `main` без рекурсивного запуска от bot-коммитов данных.

### Build/CI

- Усилен контракт согласованности версии: `VERSION`, web UI, README, Wails metadata и Windows PE metadata должны указывать одну версию.
- Release-gate синхронизирован с текущей структурой тестов и Bandit-проверок.
- Сохранены строгие проверки Go 1.26.6, Staticcheck, govulncheck, Ruff, Bandit, pip-audit, frontend smoke, Gitleaks current/history, CodeQL, dependency review и Windows-сборки x64/x86/ARM64.

"""
changelog.write_text(header + entry + changelog_text[len(header):], encoding="utf-8")

replace_block(
    "scripts/release-gate.sh",
    '"$AUDIT_ENV/bin/python" -B -m unittest discover -s tools -p \'test_*.py\'\n"$AUDIT_ENV/bin/ruff" check --no-cache .\n"$AUDIT_ENV/bin/ruff" format --check --no-cache .\n"$AUDIT_ENV/bin/bandit" -q -r tools -x \'tools/test_*.py\'\n',
    '"$AUDIT_ENV/bin/python" -m compileall -q tools tests\nif find tools -maxdepth 1 -type f -name \'test_*.py\' -print -quit | grep -q .; then\n  echo "Python regression tests must live in tests/, not tools/."\n  exit 1\nfi\n"$AUDIT_ENV/bin/python" -B -m unittest discover -s tests -p \'test_*.py\'\n"$AUDIT_ENV/bin/ruff" check --no-cache .\n"$AUDIT_ENV/bin/ruff" format --check --no-cache .\n"$AUDIT_ENV/bin/bandit" -q -r tools\n',
)

replace_block(
    "scripts/windows/IrisTools.ps1",
    '        Invoke-Checked $AuditPython @("-B", "-m", "unittest", "discover", "-s", "tools", "-p", "test_*.py") 600\n        Invoke-Checked (Join-Path $AuditEnv "Scripts\\ruff.exe") @("check", "--no-cache", ".") 300\n        Invoke-Checked (Join-Path $AuditEnv "Scripts\\ruff.exe") @("format", "--check", "--no-cache", ".") 300\n        Invoke-Checked (Join-Path $AuditEnv "Scripts\\bandit.exe") @("-q", "-r", "tools", "-x", "tools/test_*.py") 300\n',
    '        Invoke-Checked $AuditPython @("-m", "compileall", "-q", "tools", "tests") 120\n        $LegacyTests = @(Get-ChildItem -LiteralPath (Join-Path $Root "tools") -Filter "test_*.py" -File -ErrorAction SilentlyContinue)\n        if ($LegacyTests.Count -ne 0) { throw "Python regression tests must live in tests/, not tools/." }\n        Invoke-Checked $AuditPython @("-B", "-m", "unittest", "discover", "-s", "tests", "-p", "test_*.py") 600\n        Invoke-Checked (Join-Path $AuditEnv "Scripts\\ruff.exe") @("check", "--no-cache", ".") 300\n        Invoke-Checked (Join-Path $AuditEnv "Scripts\\ruff.exe") @("format", "--check", "--no-cache", ".") 300\n        Invoke-Checked (Join-Path $AuditEnv "Scripts\\bandit.exe") @("-q", "-r", "tools") 300\n',
)

quality = ROOT / "tests/test_quality_contract.py"
quality_text = quality.read_text(encoding="utf-8")
replace_set_up = '''        cls.readme = (ROOT / "README.md").read_text(encoding="utf-8")
        cls.contributing = (ROOT / "CONTRIBUTING.md").read_text(encoding="utf-8")
'''
replace_set_up_new = '''        cls.readme = (ROOT / "README.md").read_text(encoding="utf-8")
        cls.wails = (ROOT / "wails.json").read_text(encoding="utf-8")
        cls.windows_info = (ROOT / "build/windows/info.json").read_text(encoding="utf-8")
        cls.release_gate = (ROOT / "scripts/release-gate.sh").read_text(encoding="utf-8")
        cls.windows_release_tools = (ROOT / "scripts/windows/IrisTools.ps1").read_text(encoding="utf-8")
        cls.contributing = (ROOT / "CONTRIBUTING.md").read_text(encoding="utf-8")
'''
if quality_text.count(replace_set_up) != 1:
    raise SystemExit("quality contract setup block changed unexpectedly")
quality_text = quality_text.replace(replace_set_up, replace_set_up_new, 1)

old_version_assertions = '''        self.assertEqual(version, app_match.group(1))
        self.assertIn(f"Версия {version}", self.html)
        self.assertIn(f"v{version}", self.readme)
'''
new_version_assertions = '''        self.assertEqual(version, app_match.group(1))
        self.assertIn(f"Версия {version}", self.html)
        self.assertIn(f"v{version}", self.readme)
        self.assertIn(f'"productVersion": "{version}"', self.wails)
        self.assertIn(f'"file_version": "{version}.0"', self.windows_info)
        self.assertIn(f'"product_version": "{version}.0"', self.windows_info)
        self.assertIn(f'"FileVersion": "{version}"', self.windows_info)
        self.assertIn(f'"ProductVersion": "{version}"', self.windows_info)
'''
if quality_text.count(old_version_assertions) != 1:
    raise SystemExit("quality contract version block changed unexpectedly")
quality_text = quality_text.replace(old_version_assertions, new_version_assertions, 1)

marker = '''    def test_item_page_does_not_duplicate_recipe_materials(self):
'''
release_test = '''    def test_release_gates_run_current_python_regression_suite(self):
        self.assertIn("unittest discover -s tests -p 'test_*.py'", self.release_gate)
        self.assertNotIn("unittest discover -s tools", self.release_gate)
        self.assertIn('"discover", "-s", "tests", "-p", "test_*.py"', self.windows_release_tools)
        self.assertNotIn('"discover", "-s", "tools", "-p", "test_*.py"', self.windows_release_tools)
        self.assertIn('bandit" -q -r tools', self.release_gate)
        self.assertIn('@("-q", "-r", "tools")', self.windows_release_tools)

'''
if quality_text.count(marker) != 1:
    raise SystemExit("quality contract insertion marker changed unexpectedly")
quality_text = quality_text.replace(marker, release_test + marker, 1)
quality.write_text(quality_text, encoding="utf-8")

Path(__file__).unlink()
