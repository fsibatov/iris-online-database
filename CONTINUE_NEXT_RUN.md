# Iris Online Database — продолжение после итерации 89

**UTC:** 2026-10-23 09:18. Только инженерный кандидат, не релиз.

## Подтверждённый вход
- Источник: conversation `IrisOnlineDB_AI_2026-10-22_iter88.zip`, затем Library `/Iris Online Database/Engineering_Candidates/IrisOnlineDB_AI_2026-10-22_iter88.zip`.
- SHA-256: `7a7e90cdc8dfe7f70956b008e53e37b90ca0121c931c1cc17b4d1b03b8023c1c`; 4 282 482 байта; 265 файлов; один корень `iris-online-database`; ZIP CRC, полная распаковка, побайтовая идентичность и повторная материализация Library/cmp PASS.
- Git `ai-development`, входной HEAD `6c8b62935d09cd36eabfe3d98212337d92e68a73`, parent `9d5bd700d2107c81595ad2cd65b20f5e21de6ce6`, clean, fsck exit 0 (старый dangling commit); VERSION `2.0.5`, `.go-version` `1.26.6`. docs/ARCHITECTURE.md, docs/BUILD.md, docs/RELEASE.md, IrisTools.ps1 и AI_DEVELOPMENT_LOG.md присутствуют.

## Один патч
- Дефект: некорректный UTF-8 в `pending-delete.json` молча превращался JSON-декодером в U+FFFD, скрывал корректный backup и терял очередь при append.
- До патча: 3 верхнеуровневых теста FAIL (3 malformed UTF-8 подслучая + recovery + append). После: `utf8.Valid(data)` в `storage.go` до декодирования, новые тесты PASS. Нормальные Unicode пути продолжают работать. Изменения только в `storage.go`, `pending_delete_invalid_utf8_test.go`, инженерном журнале и этом checkpoint.

## QA
- Изолированный Linux Go 1.23.2 pending-delete suite `-count=10` PASS, целевые `-count=100` PASS, `-race -count=3` PASS, `go vet`/gofmt PASS. Windows x64 PE cross-compile PASS, нативное исполнение Windows NOT RUN. **Изолированный Go-контур не полный Go gate.**
- Python unittest 217 PASS; compileall с pycache вне source PASS; data audit `fatal=[]` (13 927 items, 1 342 monsters) PASS; repository audit PASS; node --check PASS; офлайн Chromium monster-drop smoke PASS; Git diff-check/fsck PASS.
- Полный Go gate BLOCKED (Go 1.23.2 < go.mod 1.25); основной frontend smoke NOT RUN (требует отдельного TCP-сервера); workflow NOT PASS без подходящей среды; Windows Wails/WebView2/Quality Gate, Staticcheck, govulncheck, Ruff, Bandit, pip-audit, Gitleaks NOT RUN.

## Ограничения и следующее действие
- **Критическое расхождение:** `.go-version=1.26.6` в архивной ветке не сверена с актуальной рабочей Windows-копией после merge `ai-development`→`main` и Go/security обновлений. Нельзя заменять Windows-копию архивом, понижать toolchain/security, менять main/remote/tags/Releases или считать ZIP релизом.
- **Одно следующее действие:** перед новым патчем найти новейший полный ZIP/checkpoint в conversation/Library и проверить SHA-256, CRC, распаковку, Git/fsck и повторную материализацию из Library.
