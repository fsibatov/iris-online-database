# Iris Online Database — продолжение после iter105

**UTC:** 2026-10-24 01:15 UTC. **Статус:** одна безопасная RED→GREEN итерация, полный Go BLOCKED, Windows QA NOT RUN, workflow NOT PASS; **НЕ релиз**.

## Вход, происхождение, целостность

- Перед началом перечислены свежие conversation ZIP iter95–104 и Library ZIP iter86–104, latest/immutable checkpoints. Выбран самый новый проверенный полный источник — Library `/Iris Online Database/Engineering_Candidates/IrisOnlineDB_AI_2026-10-24_iter104.zip`, совпадающий побайтово с conversation iter104.
- Вход SHA-256 `2aa45d8cbf322a91f4e73f20926b4d188f4c122095169e827159c59704971261`, 6 312 911 байт, 408 ZIP-файлов, один корень `iris-online-database`; CRC 408/408 PASS; SHA-256 CRC-таблицы `d3eda046c1ca20a0686aa42f5a89a182314a93598a4f9d5045b3dffd88570d49`; полная распаковка и побайтовое сравнение 408/408 PASS. Latest и immutable checkpoint iter104 совпадают побайтово (SHA-256 `2674aa1126fb2a491b323dd8c9b44526d1fbf93797ed22353754ddf3f4115873`).
- Вход Git `ai-development` HEAD `cc8f1774cb4fa4735c8e61bc7e0b0ab00a87dbb8`, parent `03b2c632f92fa4e441fe460e59217ee46fd2656e`, clean, fsck PASS (старый dangling commit). `VERSION=2.0.5`, `.go-version=1.26.6`, `go.mod` >=1.25; docs/ARCHITECTURE.md, docs/BUILD.md, docs/RELEASE.md, IrisTools.ps1, docs/AI_DEVELOPMENT_LOG.md и continuation проверены. Методический файл `Engineering_Maintenance_Standard_OpenAI_2026-10-22.md` в доступной Library не обнаружен.

## Одна итерация, сравнение, QA

- `tools/validate_workflows.py::release_policy_failures()` ранее принимал комментарии `# runs-on: windows-2025` вместо реальных `runs-on`. Тест на три workflow `ci.yml`, `codeql.yml`, `dependency-review.yml`: 3 FAIL до исправления, 3 PASS после; исходные workflow PASS. Теперь `jobs[*].runs-on` сравниваются точно с `windows-2025` в разобранном YAML `UniqueKeySafeLoader`. Риск низкий, CI/workflows и production/Go/UI/game/security не менялись.
- Linux Python 3.13.5: unittest **234 PASS**, `PYTHONPATH=tools`, pycache вне source; compileall PASS; data audit `fatal=[]`, items 13927, monsters 1342 PASS; repository audit PASS; node --check PASS; gofmt PASS; offline Chromium Playwright monster-drop PASS; Git fsck PASS. Workflow static PASS, Windows PowerShell NOT RUN (exit 2), overall workflow NOT PASS. Полный Go test/vet/race BLOCKED (Go 1.23.2 < go.mod 1.25, offline). Изолированный Go NOT RUN (Python-only patch). Windows QA/PowerShell/Wails/WebView2 NOT RUN; frontend TCP smoke NOT RUN (запрет отдельного TCP-сервера); Staticcheck/govulncheck/Ruff/Bandit/pip-audit/Gitleaks NOT RUN (недоступны).

## Порядок сохранения и критическое расхождение

- Создать один локальный коммит на `ai-development` и чистое дерево; полный ZIP с `.git` и проверкой SHA/CRC/распаковки/побайтовой идентичности/Git fsck; загрузить в Library, материализовать и побайтово сравнить. Создать отдельный immutable checkpoint с финальным ZIP SHA/HEAD, материализовать и сверить; лишь затем обновить latest checkpoint, материализовать и сверить. При любом сбое указатель не продвигать. Полные выходные SHA/HEAD сохранять во внешнем checkpoint во избежание самоссылки.
- Архивная `.go-version=1.26.6` не сверена с работающей Windows-копией после merge `ai-development`→`main` и обновлений Go/security. Windows-копию не заменять, Go/security не понижать, main/remote/tags/Releases не изменять и не публиковать. ZIP не релиз.

**Одно следующее действие:** получить подлинный read-only снимок работающей Windows-копии и сверить HEAD, `.go-version` и security toolchain без замены файлов.
