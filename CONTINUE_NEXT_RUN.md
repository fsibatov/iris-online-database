# Iris Online Database — продолжение после инженерной итерации 98

**UTC:** 2026-10-23 18:21:43 UTC. **Вход:** Library `IrisOnlineDB_AI_2026-10-23_iter97.zip`, SHA-256 `73696e741988dfa4a1b02b78fe59b71da8e4a04554f817b3e82adbb44a08bf48`, 5 402 465 байт, 345 ZIP-записей, CRC 345/345 PASS, CRC-таблица SHA-256 `071391ea9b775bd346300744a32db9eb66d358d224c29e57b0c891e525ae5452` (`filename\tCRC32\n`, сортировка по имени), единственный корень `iris-online-database`; полная распаковка и побайтовая идентичность 345/345 PASS. Latest/immutable checkpoint iter97: SHA-256 `bb9bd4bcd602d149bdcd1bfc34ef0e98ecb0945d4078783dfc6bc4e3f07800dd`, cmp PASS. Git вход `ai-development`, HEAD `a356233995ff727a77cfd1761686269d091f4e38`, parent `aacd3fd95dc563f0fd147c4c1ae0c3a9e384f21c`, clean, fsck PASS (старый dangling commit). VERSION `2.0.5`, `.go-version` `1.26.6`; `docs/ARCHITECTURE.md`, `docs/BUILD.md`, `docs/RELEASE.md`, `IrisTools.ps1`, журнал и continuation проверены. Методический файл `Engineering_Maintenance_Standard_OpenAI_2026-10-22.md` не обнаружен в доступной Library; применены явные требования задания.

## Один патч и регрессия

`tools/validate_workflows.py` молча принимал YAML с повторяющимся `jobs:`, поскольку PyYAML `safe_load` перезаписывал первую секцию. Добавленный тест: RED (ожидался WF001, получен статический PASS с PowerShell NOT RUN). `UniqueKeySafeLoader` теперь отвергает дубликаты явных ключей; GREEN. Только Python validator и regression test; игровые данные, UI, профиль, сеть и Go/security pin не менялись.

## QA и ограничения

Python unittest 226 PASS, compileall PASS, data audit `fatal=[]`, repository audit PASS, node --check PASS, offline Playwright PASS, gofmt PASS, git diff/fsck PASS. Workflow static PASS, Windows PowerShell NOT RUN [WINDOWS_POWERSHELL]. Полный Go test/vet/race BLOCKED (Linux Go 1.23.2 < go.mod >=1.25, pin 1.26.6); изолированный Go NOT RUN, Windows Quality Gate NOT RUN, основной TCP smoke NOT RUN, security tools NOT RUN (недоступны). Методический файл OpenAI в доступной Library не найден.

## Сохранение и продолжение

После локального коммита `ai-development` — полный ZIP с `.git`, CRC, полной распаковкой, SHA-256, побайтовой идентичностью, Git/fsck; затем Library upload/readback/cmp и immutable checkpoint upload/readback/cmp. Latest pointer обновлять только после полного подтверждения. Выходной ZIP SHA/HEAD записать во внешнем checkpoint без самоссылки. Не менять main/remote/tags/Releases.

**Критическое расхождение:** архивная `.go-version=1.26.6` не сверена с работающей Windows-копией после merge и обновлений Go/security; Windows-копию не заменять, Go/security не понижать, ZIP не релиз.

**Одно следующее действие:** при наличии подлинного снимка работающей Windows-копии сверить HEAD, Go pin и security toolchain без замены файлов.
