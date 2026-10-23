# Iris Online Database — продолжение после итерации 92

**UTC:** 2026-10-23 12:22. **Статус:** инженерный кандидат, НЕ релиз и НЕ Windows Quality Gate.

## Подтверждённая входная база
- Library `/Iris Online Database/Engineering_Candidates/IrisOnlineDB_AI_2026-10-23_iter91.zip`; SHA-256 `d3a14f4964e02a939f1bf5d500ad66b1921553336903860b892cd6e815c3edbc`, 4 644 413 байт, 290 ZIP-файлов, один корень `iris-online-database`; CRC/полная распаковка/побайтовая идентичность 290/290 PASS, повторная материализация и cmp PASS. Перечислены conversation iter88–91 и Library iter86–91/checkpoint. Git входа `ai-development` HEAD `9853dc1c145ba423ace9d78a98de4aeceffcfda5`, parent `7e43d3c0fb7634e06053da54b277640c6357459f`, clean/fsck PASS; VERSION `2.0.5`, `.go-version` `1.26.6`; обязательные документы и IrisTools.ps1 присутствуют.

## Одна инженерная правка
- Linux `validate_workflows.py` ошибочно классифицировал отсутствие Windows PowerShell как 4 структурных `WF001` FAIL при нулевых статических нарушениях. RED: базовый запуск `FAIL [WF001] count=4`, три новых теста ERROR до патча. GREEN: 3/3 теста PASS; Linux теперь `NOT RUN [WINDOWS_POWERSHELL]`/exit 2, статические проверки PASS; реальное статическое нарушение остаётся FAIL/exit 1; Windows parser gate сохраняется. Файлы: `tools/validate_workflows.py`, `tests/test_workflow_platform_gate.py`.

## Фактический QA и блокеры
- Python unittest 220 PASS; compileall, data audit `fatal=[]`, repository audit, node --check, офлайн Chromium monster-drop smoke, gofmt, git diff --check/fsck PASS. Workflow NOT RUN для Windows PowerShell; полный Go gate BLOCKED (Go 1.23.2 < go.mod 1.25.0); Go test/vet/race NOT RUN в этом Python-only цикле. Windows Wails/WebView2 QA и основной frontend TCP smoke NOT RUN; Staticcheck, govulncheck, Ruff, Bandit, pip-audit, Gitleaks NOT RUN. Методика `Engineering_Maintenance_Standard_OpenAI_2026-10-22.md` не найдена в доступной Iris Library/conversation; использованы явные требования задания.
- Архивная `.go-version=1.26.6` НЕ сверена с работающей Windows-копией после merge ai-development→main и Go/security обновлений. Windows-копию не заменять, Go/security не понижать, main/remote/tags/Releases не менять.

## Одно следующее действие
- При наличии достоверного снимка работающей Windows-копии сверить HEAD, Go pin и security toolchain без замены файлов; при отсутствии продолжать только узкие доказуемые офлайн-исправления.
