# Iris Online Database — продолжение после итерации 90

**UTC:** 2026-10-23 10:18. **Статус:** инженерный кандидат, НЕ релиз и НЕ Windows Quality Gate.

## Подтверждённая входная база
- Library `/Iris Online Database/Engineering_Candidates/IrisOnlineDB_AI_2026-10-23_iter89.zip`; SHA-256 `c3984b7b10a1f1068da6fcda7ad0ea8f8d1921e1a3d7294b7ed250a102c415b9`, 4 400 869 байт, 274 ZIP-файла, один корень `iris-online-database`. CRC, полная распаковка, побайтовая идентичность 274/274 и совпадение с conversation ZIP PASS.
- Git `ai-development`, входной HEAD `8e3b0633698bce23149e9e62b37c7a5ae3734f38`, parent `6c8b62935d09cd36eabfe3d98212337d92e68a73`, clean, fsck PASS (старый dangling commit). VERSION `2.0.5`, `.go-version` `1.26.6`; обязательные docs/IrisTools.ps1 на месте.

## Один дефект / RED→GREEN
- Дисковый `profile.json` с некорректным UTF-8 принимался JSON-декодером и скрывал исправный `UserData/Backups/profile.json.bak`; при сохранении испорченный primary мог перезаписать исправный backup.
- До: 2 верхнеуровневых FAIL (3 подслучая invalid UTF-8 + восстановление backup), 1 контрольный PASS. После: минимальная проверка `utf8.Valid(data)` в `loadProfileFile`; все три PASS, сохранность backup после `Replace` проверена. Изменены `storage.go`, новый `profile_invalid_utf8_disk_test.go`, журнал и этот checkpoint. Корректный Unicode сохраняется.

## QA и ограничения
- Изолированный Linux Go 1.23.2-контур 175 тестов `-count=30` PASS, race `-count=3` PASS, целевые `-count=100` PASS, vet/gofmt PASS; Windows x64 PE cross-compile PASS, native Windows NOT RUN. Это НЕ полный Go gate.
- Python unittest 217 PASS, compileall вне source PASS, data audit `fatal=[]` (13 927 items / 1 342 monsters) PASS, repository audit PASS, node --check PASS, офлайн Chromium monster-drop smoke PASS, Git/fsck PASS.
- Полный Go gate BLOCKED (Go 1.23.2 < go.mod 1.25.0); основной frontend smoke NOT RUN (TCP runtime), workflow NOT PASS `[WF001] count=4`; Windows Wails/WebView2/Quality Gate, Staticcheck/govulncheck/Ruff/Bandit/pip-audit/Gitleaks NOT RUN.
- Архивная `.go-version=1.26.6` НЕ сверена с рабочей Windows-копией после merge ai-development→main и Go/security обновлений. Не заменять Windows-копию, не понижать Go/security, не публиковать, не считать ZIP релизом.

## Первое действие следующего запуска
- Сначала найти и проверить новейший полный ZIP/checkpoint в conversation/Library. Затем при наличии достоверного снимка Windows-копии сверить её HEAD, `.go-version` и security toolchain с архивной веткой **без замены файлов**; без такого снимка продолжать только доказуемые узкие офлайн-исправления.
