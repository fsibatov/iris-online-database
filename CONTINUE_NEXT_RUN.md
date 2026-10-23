# Iris Online Database — продолжение после итерации 101

**UTC:** 2026-10-23 21:17:21 UTC. **Статус:** одна узкая итерация RED→GREEN, полный Go/Windows QA BLOCKED/NOT RUN. НЕ релиз.

## Подлинная база

- Library `/Iris Online Database/Engineering_Candidates/IrisOnlineDB_AI_2026-10-23_iter100.zip`, SHA-256 `345b20a49020be89ae4b1f6595c17b46090a12cde96be976c6f0262e78195024`, 5 779 832 байта, 372 ZIP-файла; CRC 372/372 PASS, CRC-таблица SHA-256 `7708237c8703bc1362e8ac98f69fa862764bc521b77bfec4f124ec0bc3f072cd`; один корень, полная распаковка и побайтовое сравнение PASS. Conversation/Library ZIP идентичны; latest/immutable checkpoint iter100 идентичны.
- Git вход `ai-development`, HEAD `4e4c134a2231256fb3c1813200510e4db07c96ff`, parent `fa726d7ac45942731e57cc00d8b8458701b429e3`, clean, fsck PASS (старый dangling commit). VERSION `2.0.5`, `.go-version` `1.26.6`; архитектура/build/release docs, `IrisTools.ps1`, журнал и continuation проверены. Методический `Engineering_Maintenance_Standard_OpenAI_2026-10-22.md` в доступной Library не найден.

## Итерация 101

- `tools/validate_workflows.py` пропускал custom POSIX shell `bash -e {0}`, `sh -e {0}`, `bash.exe -e {0}` в Windows workflow, выдавая ложный static PASS. RED: 4 FAIL и 2 контрольных PASS. Минимальная проверка первого токена команды с необязательным `.exe`: GREEN 8/8 подслучаев, включая workflow/job defaults. Python unittest **230 PASS**. UI, игровые данные, профиль, сеть, Go/security pins не менялись.
- QA: Python unittest 230 PASS (`PYTHONPATH=tools`, pycache вне source), compileall PASS, data audit `fatal=[]` items=13 927 PASS, repository audit PASS, node --check PASS, offline Playwright PASS, gofmt PASS, git diff/fsck PASS. Workflow static PASS, Windows PowerShell NOT RUN (exit 2); полный workflow NOT PASS. Полный Go test/vet/race BLOCKED (Linux Go 1.23.2 < go.mod>=1.25, pin 1.26.6); изолированный Go NOT RUN; Windows Wails/WebView2/Quality Gate NOT RUN; основной TCP frontend smoke NOT RUN; Staticcheck/govulncheck/Ruff/Bandit/pip-audit/Gitleaks NOT RUN.

## Продолжение и запреты

- Только локальный коммит `ai-development`, чистое дерево; полный ZIP с `.git`, SHA/CRC/полной распаковкой/cmp/Git fsck. Library upload/readback/cmp; immutable checkpoint upload/readback/cmp, latest pointer только после полного подтверждения. Выходной SHA ZIP и Git HEAD фиксировать во внешнем checkpoint, без самоссылки.
- Архивная `.go-version=1.26.6` не сверена с работающей Windows-копией после merge `ai-development`→`main` и Go/security обновлений. Windows-копию не заменять, Go/security не понижать, не трогать main/remote/tags/Releases; ZIP не релиз.
- **Одно следующее действие:** при наличии подлинного снимка работающей Windows-копии сверить HEAD, Go pin и security toolchain без замены файлов.
