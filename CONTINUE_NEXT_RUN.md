# Iris Online Database — продолжение после итерации 88

**UTC:** 2026-10-22 15:18. Это инженерный кандидат, не релиз.

## Подтверждённый вход
- Library: `/Iris Online Database/Engineering_Candidates/IrisOnlineDB_AI_2026-10-22_iter87.zip`
- SHA-256: `0984b6136d9c95798f1ef9f09f047df95b6635548d95fa6bd648771fda3eaba0`; 8 741 042 байта; 710 ZIP-файлов; единственный корень `iris-online-database`.
- ZIP CRC, полная распаковка, побайтовая идентичность всех 710 файлов: PASS. Повторная материализация Library и сравнение с conversation-файлом (`cmp`/SHA/CRC): PASS.
- Входной Git: branch `ai-development`, HEAD `9d5bd700d2107c81595ad2cd65b20f5e21de6ce6`, parent `e77bde9d2870486f3a2824379f42afede019d5eb`, clean status, `git fsck --full`: PASS. VERSION `2.0.5`; `.go-version=1.26.6`. Все обязательные docs и `IrisTools.ps1` присутствуют.

## Итерация 88
`windowsReservedDeviceComponent` теперь распознаёт зарезервированные Win32 device aliases с ASCII-пробелом перед расширением (`CON .txt`, `COM1 .bin`). До патча 3 новых верхнеуровневых теста FAIL: 5 вариантов принимались, backup recovery не выполнялась, append терял прежний путь/backup. После патча 3 теста PASS, `-count=100` и `-race -count=5` PASS. Изменены только `storage.go`, `pending_delete_windows_reserved_alias_spaces_test.go`, журнал и этот checkpoint.

## QA
- Изолированный Linux Go 1.23.2 pending-delete suite: `go test -count=30` PASS (4.885 s), race `-count=3` PASS (4.131 s), vet/gofmt PASS; новые тесты `-count=100` PASS (0.645 s), race `-count=5` PASS (1.068 s). Изолированный Windows x64 PE cross-compile PASS; Windows-исполнение NOT RUN.
- Python unittest: 217 PASS (4.054 s); compileall PASS; data audit `fatal=[]` PASS (13 927 items, 1 342 monsters); repository audit PASS; `node --check web/*.js` PASS; офлайн Chromium monster-drop smoke PASS; Git diff-check/fsck PASS.
- Полный Go gate BLOCKED: локальный Go 1.23.2, `go.mod` требует 1.25.0, загрузка toolchain недоступна. Основной frontend smoke NOT PASS `[BROWSER_RUNTIME]`; workflow validation NOT PASS `[WF001] count=4`. Windows Wails/WebView2/Quality Gate и Staticcheck/govulncheck/Ruff/Bandit/pip-audit/Gitleaks NOT RUN.

## Блокер и первое действие
Архивная ветка с `.go-version=1.26.6` не сверена с рабочей Windows-копией после merge `ai-development`→`main` и обновления Go/security. Не подменять рабочую копию, не понижать Go/security, не считать архив релизом. **Первое действие:** найти новейший полный ZIP/checkpoint в conversation/Library и подтвердить SHA-256, CRC, распаковку, Git и версии перед новым патчем.

Локальный ZIP iter88 прошёл CRC/распаковку/побайтовую/Git-проверку, но Library upload дважды вернул `library_storage_limit_exceeded`. Пять старых ZIP iter81–iter85 перемещены в trash ради освобождения места; их immutable checkpoints сохранены. Повторная загрузка не удалась, поэтому `IrisOnlineDB_AI_LATEST_CHECKPOINT.md` не продвигался и последней подтверждённой Library-версией остаётся iter87. Финальные локальные ZIP: SHA-256/CRC/размер и HEAD приведены в отдельном отчёте текущего запуска. **Первое действие:** освободить Library storage/очистить trash, затем загрузить iter88, материализовать и сверить SHA/bytes; до этого не менять latest checkpoint.
