# Iris Online Database — продолжение после инженерной итерации 95

**UTC:** 2026-10-23 15:19:36 UTC. **Статус:** один узкий патч RED→GREEN, полный Go/Windows gate BLOCKED/NOT RUN. НЕ релиз.

## Входная подлинная база

Conversation ZIP iter88–94; Library ZIP iter86–94, latest/immutable checkpoint. Самая новая подтверждённая полная база: `/Iris Online Database/Engineering_Candidates/IrisOnlineDB_AI_2026-10-23_iter94.zip`, SHA-256 `a3971d773c967168602463721a1fb07afb65eed8a347a126d2980be67afbd106`, 5 015 530 байт, 318 файлов; CRC 318/318 PASS, CRC-таблица SHA-256 `450bebdd3e74888fa08e73c178d88e2acceef1e8edc3673d4ce57f8137220c6a` (`filename\tCRC32\n`), полная распаковка и побайтовая идентичность 318/318 PASS; conversation/Library ZIP cmp PASS; latest/immutable checkpoint cmp PASS. Вход Git `ai-development` HEAD `5412f8fc6f84f1b7ff56199d3cc247432d73ced8`, parent `228b8bfc7a614a1c3db5208ce9003d2230c0c049`, clean/fsck PASS. VERSION `2.0.5`, `.go-version` `1.26.6`, обязательные документы/IrisTools.ps1 проверены. Методический файл OpenAI в доступной Library не найден.

## Исправление и тесты

`tools/verify_executables.py` принимал ложные подстроки Go metadata (`GOOS=windowsSPOOF` и аналогичные). `tests/test_release_helpers.py::test_release_metadata_rejects_substring_spoofing`: RED до патча (7 категорий пропущено), GREEN после; проверены настоящий tab-separated `go version -m`, optional h1 и подмена типа записи. Маркеры теперь требуют точного поля build или точной пары dependency module/version. Игровые данные, UI, сеть, профиль и Go/security pin не менялись.

## Фактический QA

Python unittest **223 PASS**, compileall PASS, data audit `fatal=[]`, repository audit PASS, `node --check` PASS, offline Playwright PASS, gofmt PASS, git diff/fsck PASS. Workflow static PASS, PowerShell NOT RUN [WINDOWS_POWERSHELL] exit 2. Полный Go test/vet/race BLOCKED (Linux Go 1.23.2 < go.mod >=1.25, pin 1.26.6); изолированный Go NOT RUN, Windows Quality Gate NOT RUN, основной TCP frontend smoke NOT RUN, Staticcheck/govulncheck/Ruff/Bandit/pip-audit/Gitleaks NOT RUN.

## Выход и продолжение

После коммита в `ai-development` нужен полный ZIP с `.git`, SHA-256, CRC, распаковкой и побайтовой сверкой, Git clean/fsck. Затем upload в Library и повторная материализация/cmp, immutable checkpoint upload/readback/cmp, только после — обновление latest checkpoint. Финальные SHA/HEAD хранить во внешнем checkpoint, чтобы не создавать самоссылку. При любом сбое — PARTIAL/BLOCKED, latest не продвигать.

**Критическое расхождение:** архивная `.go-version=1.26.6` не сверена с работающей Windows-копией после merge `ai-development`→`main` и Go/security обновлений. Архивом Windows-копию не заменять; Go/security не понижать; ZIP не релиз; GitHub только для сравнения.

**Одно следующее действие:** при наличии подлинного снимка работающей Windows-копии сверить HEAD, Go pin и security toolchain без замены файлов.
