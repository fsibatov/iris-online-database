# Iris Online Database — продолжение после iter108

**UTC:** 2026-10-24 04:19:15 UTC. **Источник:** Library iter107; SHA-256 `ebcfaeb76964674dacfb2f8d0e7ca81ca56ff86ae9b7ed00873eba4262a718e0`; 6 727 267 байт, 435 файлов, CRC 435/435, CRC-table SHA-256 `108fe6ed5905533f9678707da4dac2b63b111fec47d605e783a7644c6670609f`; полная распаковка и побайтовое сравнение 435/435 PASS. Conversation ZIP совпал с Library; latest/immutable checkpoint iter107 совпали, SHA-256 `38df34563d1a80b5da8b964a54a2ca899c8a2871cb2313ef6b8f50ae7493f00a`. Единственный корень `iris-online-database`.

**Вход Git:** `ai-development` HEAD `5211e89ec1b1b5ed9dad709c9edbd53ee4b23507`, parent `ada1d872678a509ffeaf19ab96a847b072afa4ca`, чистое дерево, fsck PASS (старый dangling commit). VERSION `2.0.5`; `.go-version` `1.26.6`; go.mod `1.25.0`. Проверены `docs/ARCHITECTURE.md`, `docs/BUILD.md`, `docs/RELEASE.md`, `IrisTools.ps1`, `docs/AI_DEVELOPMENT_LOG.md`, `CONTINUE_NEXT_RUN.md`. `Engineering_Maintenance_Standard_OpenAI_2026-10-22.md` не обнаружен в доступной Library; явные правила задания соблюдены.

**Единственный патч:** `ci_trigger_failures` отвергает `paths` и `paths-ignore` в обязательных `push`/`pull_request` CI-триггерах: иначе можно пропустить Windows quality/security/race/build при изменениях исходников, сохранив ложный static PASS. Тест: 4 отрицательных случая RED до, 4 GREEN после, неизменённый CI контроль PASS. Риск низкий; без изменения workflow, Go, UI, игровых данных, security pins, профиля и сети.

**QA (Linux):** Python 3.13.5, `PYTHONPATH=tools`, `PYTHONPYCACHEPREFIX` вне source: 237 unittest PASS, compileall PASS; data audit `fatal=[]`, items=13927, monsters=1342, sets=467; repository audit PASS; node --check PASS; gofmt PASS; офлайн Chromium Playwright monster-drop smoke PASS; git fsck/diff --check PASS. Workflow static PASS, Windows PowerShell NOT RUN (exit 2), workflow NOT PASS. Полный Go test/vet/race BLOCKED (Linux Go 1.23.2 < go.mod 1.25, offline), изолированный Go NOT RUN (Python-only). Windows QA/PowerShell/Wails/WebView2 NOT RUN; TCP frontend smoke NOT RUN; Staticcheck/govulncheck/Ruff/Bandit/pip-audit/Gitleaks NOT RUN (недоступны).

**Хранение:** один локальный коммит `ai-development` и чистое дерево, один полный ZIP с `.git`; выходные ZIP SHA/HEAD, CRC, Library readback — во внешнем immutable/latest checkpoint после проверки. `main`, remote, tags, Releases не менять; ZIP не релиз.

**Критическое расхождение:** архивная `.go-version=1.26.6` не сверена с работающей Windows-копией после merge `ai-development`→`main` и Go/security обновлений. Не заменять Windows-копию, не понижать Go/security.

**Одно следующее действие:** получить подлинный read-only снимок рабочей Windows-копии и сравнить HEAD, `.go-version`, security toolchain без замены файлов.
