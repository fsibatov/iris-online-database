# Iris Online Database — продолжение после инженерной итерации 94

**UTC:** 2026-10-23 14:18 UTC. **Статус:** одна узкая правка RED→GREEN; полный Go/Windows gate BLOCKED/NOT RUN. НЕ релиз.

## Подлинная база

Перечислены conversation ZIP iter88–93 и Library ZIP iter86–93/checkpoint; выбран самый новый подтверждённый Library ZIP `/Iris Online Database/Engineering_Candidates/IrisOnlineDB_AI_2026-10-23_iter93.zip`. SHA-256 `47ce322db2c36760cd3879f9503e8b8216478da29a0638fa9077f373d2e81825`, 4 889 090 байт, 309 ZIP-файлов, один корень; CRC 309/309 PASS, CRC-таблица SHA-256 `fa47909e1bc9b362b99bc9f7bc7f054bd2a8c8f19621ede0db68ef06930ce970`, полная распаковка/побайтовая идентичность 309/309 PASS; conversation/Library ZIP и latest/immutable checkpoint iter93 cmp PASS. Вход Git `ai-development` HEAD `228b8bfc7a614a1c3db5208ce9003d2230c0c049`, parent `7e9281c093b132062a439944e36d632a3e9ce540`, clean/fsck PASS; VERSION `2.0.5`, `.go-version` `1.26.6`; обязательные docs и IrisTools.ps1 проверены. Методический файл из Library не обнаружен; явные требования соблюдены.

## Одно изменение

`tools/verify_release_assets.py` пропускал symlink на внешние файлы под всеми четырьмя разрешёнными именами. `tests/test_release_helpers.py::test_release_assets_reject_symlinked_entries`: RED до патча — 4 FAIL, ложный PASS валидатора; GREEN после — 4 PASS, обычный набор файлов контрольный PASS. Минимальный запрет symlink и не-regular file до чтения. Изменены только verifier и тест; игровых данных/UI/профиля/Go/security pin/сети не касались.

## Фактический QA

Python unittest **222 PASS** (`PYTHONPATH=tools`, pycache вне source), compileall PASS, data audit `fatal=[]`, repository audit PASS, `node --check web/app.js` PASS, офлайн Chromium/Playwright PASS, gofmt PASS, git diff --check/fsck PASS. Первоначальный unittest FAIL из-за `tools/__pycache__`, созданного диагностикой; удалён, полный повтор 222 PASS. Workflow static PASS, Windows PowerShell **NOT RUN** (exit 2); полный Go test/vet/race **BLOCKED** (Linux Go 1.23.2 < go.mod 1.25), изолированный Go не запускался; Windows Quality Gate **NOT RUN**, основной TCP frontend smoke **NOT RUN**, security Staticcheck/govulncheck/Ruff/Bandit/pip-audit/Gitleaks **NOT RUN**.

## Целостность и продолжение

После локального коммита в `ai-development` ожидается один полный ZIP с `.git`, CRC, распаковкой, побайтовой идентичностью, Git/fsck и SHA-256; загрузить в Library, повторно материализовать/cmp, рядом immutable checkpoint, также материализовать/cmp. Финальный ZIP SHA и HEAD фиксировать во внешнем checkpoint, не внутри ZIP. Latest pointer менять только после полного readback. Main/remote/tags/Releases не менять, ZIP не релиз.

**Критическое расхождение:** архивная `.go-version=1.26.6` не сверена с работающей Windows-копией после merge ai-development→main и Go/security обновлений. Не заменять Windows-копию архивом, не понижать Go/security.

**Одно следующее действие:** при наличии подлинного снимка работающей Windows-копии сверить HEAD, Go pin и security toolchain без замены файлов.
