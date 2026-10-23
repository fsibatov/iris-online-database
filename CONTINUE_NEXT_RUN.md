# Iris Online Database — продолжение после инженерной итерации 97

**UTC:** 2026-10-23 17:18:56 UTC. **Статус:** один узкий патч RED→GREEN; полный Go/Windows gate BLOCKED/NOT RUN. НЕ релиз.

## Входная база и подлинность

Перечислены conversation ZIP iter89–96, Library iter86–96 и latest/immutable checkpoint. Выбран Library `IrisOnlineDB_AI_2026-10-23_iter96.zip` (совпадает с conversation). SHA-256 `395da133442940831b7517d224508d20b7af334b020f4a2f0ecce20d6343fca6`, 5 278 524 байта, 336 ZIP-файлов, CRC 336/336 PASS, CRC-таблица SHA-256 `d968a48f60f71439bd356f86dc3c38b311cce24dbd07758e39801dd447e3f19c`, один корень, полная распаковка и побайтовое сравнение 336/336 PASS. Conversation/Library ZIP cmp PASS, latest/immutable checkpoint iter96 cmp PASS. Git `ai-development` HEAD `aacd3fd95dc563f0fd147c4c1ae0c3a9e384f21c`, parent `38e29204ec71e75a2b236c7d00854839a122981a`, clean/fsck PASS; VERSION 2.0.5, `.go-version` 1.26.6, обязательные документы проверены. Методический файл OpenAI в доступной Library не найден.

## Один патч и регрессия

`tools/validate_workflows.py` не проверял YAML flow-style `- {uses: actions/checkout@main}`, выдавая ложный статический PASS. Новый тест: 1 FAIL до изменения, GREEN после; контрольная SHA-40 ссылка PASS. Парсинг `uses` по структуре PyYAML дополняет прежний regex-контроль, без ослабления остальных проверок.

## QA и блокеры

Python unittest 225 PASS, compileall PASS, data audit `fatal=[]`, repository audit PASS, node --check PASS, offline Playwright PASS, gofmt PASS, git diff/fsck PASS. Workflow static PASS, PowerShell NOT RUN [WINDOWS_POWERSHELL]. Полный Go test/vet/race BLOCKED (Linux Go 1.23.2 < go.mod >=1.25, pin 1.26.6); изолированный Go NOT RUN, Windows Quality Gate NOT RUN, основной TCP smoke NOT RUN, security tools NOT RUN (недоступны). Первый QA временно обнаружил untracked `tools/__pycache__`, созданный диагностикой; каталог удалён, повторные тесты PASS.

## Выход и продолжение

После локального коммита `ai-development` нужен один полный ZIP с `.git`, CRC, полной распаковкой, SHA-256, Git/fsck и побайтовой идентичностью. Затем Library upload/readback/cmp и отдельный immutable checkpoint upload/readback/cmp; latest pointer только после подтверждения. Итоговый ZIP SHA/HEAD — во внешнем checkpoint (без самоссылки). Не менять main/remote/tags/Releases.

**Критическое расхождение:** архивная `.go-version=1.26.6` не сверена с работающей Windows-копией после merge и обновлений Go/security; Windows-копию не заменять, Go/security не понижать, ZIP не релиз.

**Одно следующее действие:** при наличии подлинного снимка работающей Windows-копии сверить HEAD, Go pin и security toolchain без замены файлов.
