# Сборка и проверки

## Закреплённые зависимости

- Windows 10/11 amd64;
- Go 1.26.5 — единственный pin находится в `.go-version`;
- Wails CLI 2.14.0;
- Git, Python 3.13 и Node.js 24;
- Staticcheck 2026.1, govulncheck 1.6.0;
- Python tools — только `tools/requirements-audit.txt`;
- Gitleaks 8.30.1 для Linux и Windows. Linux CI проверяет закреплённый SHA-256, Windows tooling сверяет ZIP с официальным checksums-файлом перед распаковкой. Историческое расхождение Windows x64 asset 8.30.1 повторно проверено 13.08.2026: текущий официальный ZIP соответствует опубликованному SHA-256.

`go.mod` содержит минимальный language compatibility level зависимостей, но фактический воспроизводимый toolchain выбирается `.go-version`.

## Windows

Откройте PowerShell в корне:

```powershell
powershell -ExecutionPolicy Bypass -File .\IrisTools.ps1 -Action Check
powershell -ExecutionPolicy Bypass -File .\IrisTools.ps1 -Action Install
powershell -ExecutionPolicy Bypass -File .\IrisTools.ps1 -Action Test
powershell -ExecutionPolicy Bypass -File .\IrisTools.ps1 -Action Build -OutputDirectory C:\IrisRelease\2.0.0
```

Для отдельного запуска строгого теста в прежнем формате используйте поставляемый отдельно `01_TEST.bat`: храните его рядом с папкой `iris-online-database`, а не внутри source. Это только совместимый launcher: он находит корневой `IrisTools.ps1`, вызывает `-Action Test`, возвращает тот же exit code и делает `pause`, но не содержит второй копии release-проверок.

Корневой launcher автоматически запрашивает повышение через Windows UAC для `Check` и `Install`, в том числе при выборе этих пунктов из интерактивного меню. Таблица `Check` отдельно подтверждает версию PowerShell и роль `Administrator`. При отмене UAC операция завершается ошибкой и ничего не устанавливает.

`Test`, `Build`, `Publish` и `Release` запускаются с обычными правами: повышенные права для проверки и сборки исходного кода не требуются. `Install` повторно использует Python audit venv и Playwright cache под `%LOCALAPPDATA%\IrisOnlineDatabase\BuildTools`; source не загрязняется. System tools устанавливаются только при явном `Install`.

После установки через winget launcher повторно загружает системный и пользовательский `PATH`, поэтому новый терминал обычно не требуется. Версии Python tools проверяются тем же внешним audit venv, в который они были установлены.

## Linux CI / audit workstation

```bash
scripts/release-gate.sh
scripts/build-release.sh /absolute/path/outside/source
```

Release build Wails Windows amd64 явно устанавливает `CGO_ENABLED=0` и восстанавливает исходное process environment после сборки. Поэтому native Windows и Linux cross-build имеют одинаковый pure-Go WebView2 loader и проверяемые build metadata. `go test -race` исполняется на Linux, где race detector поддержан и не требует менять Windows release binary.

## Обязательные проверки

Gate включает repository hygiene, gofmt, module verify/tidy-diff/list, build, tests, race (Linux), vet, staticcheck, govulncheck, Python unittest, Ruff, Bandit, full installed-environment pip-audit, YAML/action-pin validation workflows, audit встроенной проекции данных, JS syntax, deterministic Playwright frontend smoke, Gitleaks current/history и повторную Git cleanliness проверку.

`raw_projection_audit.py` и `drop_table_audit.py` остаются отдельными read-only forensic-инструментами для исходных игровых таблиц, которые не распространяются с приложением. Их нельзя выдавать за CI PASS без явных `--resource` либо `--normal`/`--groups`/`--world`; встроенные production assets проверяются `data_presentation_audit.py`, exact SHA-256 и regression-тестами.

Security/network checks ограничены watchdog. `govulncheck` сначала использует канонический `https://vuln.go.dev`, повторяет запрос, а затем пробует Google-hosted storage endpoint той же базы `https://storage.googleapis.com/go-vulndb` через официально поддерживаемый параметр `-db`. Произвольные сторонние базы не используются. Если все три попытки завершились сетевой ошибкой, результат маркируется `NETWORK/INFRASTRUCTURE SKIP`, статус уязвимостей остаётся `UNKNOWN`, а gate завершается ошибкой без fingerprint. Сообщение `PASS` означает реальное успешное выполнение. Формат базы и `-db`: [Go Vulnerability Database](https://go.dev/doc/security/vuln/database).
