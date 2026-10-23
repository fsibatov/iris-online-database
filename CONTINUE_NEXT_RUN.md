# Iris Online Database — продолжение после инженерной итерации 96

**UTC:** 2026-10-23 16:22:01 UTC. **Статус:** один узкий патч RED→GREEN; полный Go/Windows gate BLOCKED/NOT RUN. НЕ релиз.

## Входная база и подлинность

Conversation iter88–95, Library iter86–95 и latest/immutable checkpoint; выбран Library `IrisOnlineDB_AI_2026-10-23_iter95.zip`. SHA-256 `84d1c94d1b3bb6a6e2598f1710dd5efe0995ed2f66258c9386f8a7102536e92c`, 5 149 077 байт, 327 ZIP-записей, CRC 327/327 PASS, CRC-таблица SHA-256 `7a9c2ccedadf8d0ff77a754b176562510457dd5deeda45f2322c220ed273f369` (`filename\tCRC32\n`), один корень `iris-online-database`, полная распаковка/побайтовая идентичность 327/327 PASS, conversation/Library ZIP cmp PASS, latest/immutable checkpoint cmp PASS. Git `ai-development` HEAD `38e29204ec71e75a2b236c7d00854839a122981a`, parent `5412f8fc6f84f1b7ff56199d3cc247432d73ced8`, clean/fsck PASS (существующий dangling commit); VERSION `2.0.5`, `.go-version` `1.26.6`, обязательные документы и IrisTools.ps1 проверены. Методический файл OpenAI в доступной Library не найден.

## Один патч и регрессия

`tools/verify_executables.py` ранее принимал некорректные Go metadata с завершающими пробелами и табами: 3 FAIL до исправления, 3 PASS после. Парсер теперь сохраняет завершающие символы и принимает необязательное четвёртое поле зависимости только в формате непустого `h1:` checksum. Обычный формат `go version -m` и ранее добавленные тесты сохранены.

## QA и блокеры

Python unittest 224 PASS, compileall PASS, data audit `fatal=[]`, repository audit PASS, `node --check` PASS, offline Playwright PASS, gofmt PASS, git diff/fsck PASS. Workflow static PASS, PowerShell NOT RUN [WINDOWS_POWERSHELL]. Полный Go test/vet/race BLOCKED (Linux Go 1.23.2 < go.mod >=1.25, pin 1.26.6); изолированный Go NOT RUN, Windows Quality Gate NOT RUN, основной TCP smoke NOT RUN, Staticcheck/govulncheck/Ruff/Bandit/pip-audit/Gitleaks NOT RUN.

## Выход и продолжение

После локального коммита `ai-development` требуется один полный ZIP с `.git`, CRC, полной распаковкой, SHA-256, Git/fsck и побайтовой идентичностью. Затем Library upload/readback/cmp и отдельный immutable checkpoint upload/readback/cmp; latest pointer только после подтверждения. Финальные ZIP SHA и Git HEAD — во внешнем checkpoint. Никаких main/remote/tags/Releases изменений.

**Критическое расхождение:** архивная `.go-version=1.26.6` не сверена с работающей Windows-копией после merge и обновлений Go/security; Windows-копию не заменять, Go/security не понижать, ZIP не релиз.

**Одно следующее действие:** при наличии подлинного снимка работающей Windows-копии сверить HEAD, Go pin и security toolchain без замены файлов.
