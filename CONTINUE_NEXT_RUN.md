# Iris Online Database — продолжение после итерации 91

**UTC:** 2026-10-23 11:17. **Статус:** инженерный кандидат, НЕ релиз и НЕ Windows Quality Gate.

## Подтверждённая входная база
- Library `/Iris Online Database/Engineering_Candidates/IrisOnlineDB_AI_2026-10-23_iter90.zip`; SHA-256 `5b25cbfb35a29769cd52cb22f74a1100f9a69938f60935800b63a6421f4ce6e1`, 4 521 608 байт, 282 ZIP-файла, единственный корень `iris-online-database`; CRC/полная распаковка/побайтовая идентичность 282/282 PASS. Library checkpoint `_ITER90.md` прочитан; более нового полного ZIP в conversation/Library на момент старта не обнаружено.
- Git входа `ai-development`, HEAD `7e43d3c0fb7634e06053da54b277640c6357459f`, parent `8e3b0633698bce23149e9e62b37c7a5ae3734f38`, clean, fsck PASS. VERSION `2.0.5`, `.go-version` `1.26.6`, все обязательные docs/IrisTools.ps1 на месте.

## Одна инженерная правка
- Дефект: `CONIN$`/`CONOUT$` (Win32 console devices) принимались как обычные имена в `pending-delete.json`, скрывали валидный backup и приводили к потере очереди.
- RED до изменения: 3 верхнеуровневых FAIL (5 отрицательных подслучаев, process backup recovery, append backup preservation); контроль похожих допустимых имён PASS.
- GREEN: два имени добавлены к проверке `windowsReservedDeviceComponent` в `storage.go`, 4 теста PASS; новый `pending_delete_windows_console_devices_test.go`. Польза — безопасное восстановление backup; риск ограничен зарезервированными именами.

## Фактический QA
- Изолированный Linux Go 1.23.2-контур с production storage/path/cache и точными JSON/server/version helpers: pending-delete suite `-count=30` PASS, race `-count=3` PASS, новые тесты `-count=100`/race `-count=5` PASS, vet/gofmt PASS; Windows x64 PE cross-compile PASS, исполнение Windows NOT RUN. **Это не полный Go gate**; полный Go gate BLOCKED (Go 1.23.2 < go.mod 1.25.0).
- Python unittest 217 PASS, compileall PASS, data audit `fatal=[]`, repository audit PASS, node --check PASS, офлайн Chromium monster-drop smoke PASS, Git/fsck PASS. Промежуточные Python/repository FAIL вызваны только созданным при preflight `tools/__pycache__` и устранены удалением untracked pycache с повторными PASS.
- Основной frontend smoke NOT RUN (TCP runtime), workflow NOT PASS `[WF001] count=4`, Windows Wails/WebView2 Quality Gate и security (Staticcheck/govulncheck/Ruff/Bandit/pip-audit/Gitleaks) NOT RUN.

## Критическое расхождение и одно следующее действие
- Архивная `.go-version=1.26.6` не сверена с работающей Windows-копией после merge ai-development→main и Go/security обновлений. Не заменять Windows-копию, не понижать Go/security, не менять main/remote/tags/Releases. ZIP не релиз.
- **Следующее действие:** проверить свежие ZIP/checkpoint и при наличии достоверного снимка Windows-копии сверить HEAD, Go pin и security toolchain без замены файлов; при отсутствии снимка продолжать только доказуемые узкие офлайн-исправления.
