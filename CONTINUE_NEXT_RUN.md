# Iris Online Database — продолжение после инженерной итерации 93

**UTC:** 2026-10-23 13:23. **Статус:** одна узкая правка прошла RED→GREEN; полный Windows/Go gate не пройден. НЕ релиз.

## Подлинная база

Сначала перечислены conversation ZIP iter88–92 и Library ZIP iter86–92/checkpoint. Вход: Library `/Iris Online Database/Engineering_Candidates/IrisOnlineDB_AI_2026-10-23_iter92.zip`, SHA-256 `99fcaa5671a203df191c4cc97125e83c89cddb9854f75a416cb194cfd39a1633`, 4 759 355 байт, 300 файлов, единственный корень `iris-online-database`; CRC, полная распаковка, побайтовая идентичность 300/300 PASS; conversation/Library ZIP cmp PASS; latest/immutable checkpoint iter92 cmp PASS. Вход Git `ai-development` HEAD `7e9281c093b132062a439944e36d632a3e9ce540`, parent `9853dc1c145ba423ace9d78a98de4aeceffcfda5`, clean, fsck PASS. VERSION `2.0.5`, `.go-version` `1.26.6`; обязательные docs и IrisTools.ps1 присутствуют. Методический файл `Engineering_Maintenance_Standard_OpenAI_2026-10-22.md` поиском в доступной Library не найден; явные требования пользователя соблюдены.

## Одно изменение

`tools/verify_release_assets.py` ошибочно игнорировал дополнительные каталоги при проверке точного состава release staging. Новый `tests/test_release_helpers.py::test_release_assets_reject_unexpected_directory_entries`: RED до патча (ожидаемый SystemExit не возник, ложный PASS), GREEN после минимальной однострочной правки; контрольный набор из четырёх файлов и SHA-256 PASS. Изменены только Python verifier и регрессионный тест, игровые данные/UI/профиль/Go pin/сеть не менялись.

## QA и блокеры

Python unittest **221 PASS**, compileall PASS, data audit `fatal=[]`, repository audit, node --check, офлайн Chromium Playwright, gofmt, git diff --check/fsck PASS. Workflow static PASS, Windows PowerShell **NOT RUN** (exit 2); полный Go test/vet/race **BLOCKED**: Linux Go 1.23.2 при go.mod >=1.25.0; изолированный Go-контур не является полным gate и здесь не запускался. Windows Wails/WebView2 Quality Gate **NOT RUN**, основной frontend TCP smoke **NOT RUN**, Staticcheck/govulncheck/Ruff/Bandit/pip-audit/Gitleaks **NOT RUN**. Не заявлять Windows/Go/workflow PASS.

## Целостность и продолжение

Ожидается один полный ZIP с `.git` и локальным коммитом `ai-development`, чистым деревом, CRC, полной распаковкой, побайтовой идентичностью, Git/fsck и SHA-256; итоговый ZIP SHA сохраняется в отдельном Library checkpoint, не в ZIP. Latest pointer продвигать только после загрузки ZIP+checkpoint, повторной материализации и побайтовой сверки.

**Критическое расхождение:** архивная `.go-version=1.26.6` не сверена с рабочей Windows-копией после merge ai-development→main и Go/security обновлений. Windows-копию не заменять, Go/security не понижать, main/remote/tags/Releases не менять, ZIP не релиз.

**Одно следующее действие:** при наличии достоверного снимка рабочей Windows-копии сверить HEAD, Go pin и security toolchain без замены файлов.
