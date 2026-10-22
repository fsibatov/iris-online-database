# Iris Online Database — продолжение после итерации 85

UTC: 2026-10-22 12:26 UTC. Подлинный вход: Library `/Iris Online Database/Engineering_Candidates/IrisOnlineDB_AI_2026-10-22_iter84.zip`, SHA-256 `38eb8623b80e1a454f40fad779607dfbc239fde72e6ba68aaea4a641c094f37f`, 8 404 251 байт, 686 файлов, один корень `iris-online-database`, ZIP CRC/распаковка/побайтовая идентичность PASS. Git входа `ai-development` HEAD `862f84af8b8a6f8256f29a9c7c984d149b726e19`, parent `0580063ffa59f6059391bb0740c9fdfe7529fa32`, чистое дерево, fsck PASS. VERSION `2.0.5`, `.go-version` `1.26.6`; обязательные docs/IrisTools присутствуют.

Изменено: отклонение недопустимых для Windows имён с `< > : " | ? *` в компонентах `pending-delete.json`, без отклонения двоеточия тома `C:`. Новый `pending_delete_windows_punctuation_test.go`: **3 FAIL + 1 контрольный PASS до**, **4 PASS после**, новые ×100/race ×5 PASS. Исправный backup восстанавливается и сохраняется побайтово; только `storage.go`, тест и инженерные документы.

Фактический QA: изолированный Go-контур вне source, 176 тестов ×30 PASS, race ×3 PASS, vet/gofmt PASS, Windows x64 cross-compile изолированного модуля PASS (на Windows NOT RUN); Python unittest 217 PASS, compileall PASS, data audit fatal=[] PASS, repository audit PASS, node --check PASS, офлайн Chromium monster-drop PASS, Git diff --check/fsck PASS. Полный Go gate BLOCKED (Go 1.23.2, go.mod≥1.25); основной frontend smoke NOT PASS [BROWSER_RUNTIME], workflow validation NOT PASS [WF001] count=4; Windows Quality Gate и Staticcheck/govulncheck/Ruff/Bandit/pip-audit/Gitleaks NOT RUN.

Предполагаемый полный ZIP: Library `/Iris Online Database/Engineering_Candidates/IrisOnlineDB_AI_2026-10-22_iter85.zip`. Точный финальный SHA-256, CRC, размер, HEAD/parent/branch/status/fsck, распаковка и повторная материализация из Library должны быть записаны во внешний checkpoint ПОСЛЕ проверки, поскольку собственный SHA/HEAD внутри ZIP самоссылочны. Если проверка или сохранение не прошли, iter84 остаётся последней подтверждённой версией.

Критическое расхождение: архивная ветка Go 1.26.6 не сверена с работающей Windows-копией после merge и обновлений Go/security. Не заменять рабочую Windows-копию, не считать ZIP релизом, не изменять main/remote/tags/Releases.

**Первое действие следующего запуска:** проверить свежие ZIP и checkpoint в conversation/Library по SHA-256, CRC, распаковке, Git и версиям.
