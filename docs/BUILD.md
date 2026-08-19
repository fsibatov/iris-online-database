# Сборка и проверки

## Поддерживаемая среда

Iris Online Database — Windows desktop-приложение. Канонические локальные проверки, сборка и выпуск выполняются только в Windows 10/11 через PowerShell 5.1+.

Закреплённые зависимости:

- публикуемые цели: Windows x64 (`amd64`), x86 (`386`) и ARM64 (`arm64`);
- Go 1.26.6 — единственный pin находится в `.go-version`;
- Wails CLI 2.14.0;
- Git, Python 3.13 и Node.js 24;
- Staticcheck 2026.1 и govulncheck 1.6.0;
- Python tools — только `tools/requirements-audit.txt`;
- Gitleaks 8.30.1 для Windows x64 с обязательной проверкой SHA-256;
- Microsoft Edge WebView2 Runtime.

`go.mod` задаёт совместимость зависимостей, а фактический воспроизводимый toolchain выбирается `.go-version`. На Windows `Install` не зависит от задержки каталога Winget для Go: exact `windows-amd64` ZIP загружается из `go.dev`, размер и SHA-256 сверяются с официальными release metadata, после чего проверяется фактический `go version`. Проверенный toolchain хранится versioned под `%LOCALAPPDATA%\IrisOnlineDatabase\BuildTools` и ставится первым в process `PATH`; системная установка Go не перезаписывается.

## Команды Windows

Откройте PowerShell в корне репозитория:

```powershell
powershell -ExecutionPolicy Bypass -File .\IrisTools.ps1 -Action Check
powershell -ExecutionPolicy Bypass -File .\IrisTools.ps1 -Action Install
powershell -ExecutionPolicy Bypass -File .\IrisTools.ps1 -Action Test
powershell -ExecutionPolicy Bypass -File .\IrisTools.ps1 -Action Build -OutputDirectory C:\IrisRelease\2.0.1
```

Для отдельного запуска строгого теста в прежнем формате используйте поставляемый отдельно `01_TEST.bat`: храните его рядом с папкой `iris-online-database`, а не внутри source. Это только launcher: он находит корневой `IrisTools.ps1`, вызывает `-Action Test`, возвращает тот же exit code и делает `pause`, но не содержит второй копии release-проверок.

Корневой launcher автоматически запрашивает повышение через Windows UAC для `Check` и `Install`. `Test`, `Build`, `Publish` и `Release` выполняются с обычными правами. `Install` повторно использует Python audit venv и Playwright cache под `%LOCALAPPDATA%\IrisOnlineDatabase\BuildTools`; source не загрязняется.

Windows gate сам задаёт `PYTHONPATH` для `tools` и выносит `PYTHONPYCACHEPREFIX` из source. Старые `tools\__pycache__` и `tests\__pycache__` удаляются перед первой проверкой как известные generated-каталоги; если текущий запуск создаст их снова, финальный repository audit завершится ошибкой. Python и Go checkout-ятся с LF согласно `.gitattributes`, поэтому глобальный `core.autocrlf` не должен создавать ложные ошибки Ruff/gofmt.

## Обязательные проверки

`IrisTools.ps1 -Action Test` выполняет fail-closed gate: repository hygiene, gofmt, module verify/tidy-diff, Windows build probe, Go tests и vet, Staticcheck, govulncheck, Python unittest, Ruff lint/format, Bandit, full installed-environment pip-audit, YAML/action-pin validation, audit встроенной проекции данных, JS syntax, deterministic Playwright smoke, Gitleaks current/history и повторную Git cleanliness проверку. Fingerprint записывается только после полного PASS.

GitHub CI использует Windows Server 2025. Отдельно выполняются тот же Windows quality/security gate, Windows `amd64` race detector и Wails-сборка x64/x86/ARM64. CodeQL и dependency review для release-контуров также выполняются на Windows runners. Linux не является поддерживаемой платформой приложения и не входит в release gate.

Release build последовательно собирает Windows x64 (`windows/amd64`, `GOAMD64=v1`), x86 (`windows/386`, `GO386=sse2`) и ARM64 (`windows/arm64`, `GOARM64=v8.0`). Для каждой цели явно устанавливается `CGO_ENABLED=0`, а исходное process environment восстанавливается после сборки. Каждый EXE проходит проверку Go/Wails metadata, PE architecture, Windows resources и hardening flags.

`raw_projection_audit.py` и `drop_table_audit.py` остаются отдельными read-only forensic-инструментами для исходных игровых таблиц, которые не распространяются с приложением. Их нельзя выдавать за CI PASS без явных входных ресурсов; production assets проверяются `data_presentation_audit.py`, exact SHA-256 и regression-тестами.

Security/network checks ограничены watchdog. `govulncheck` использует официальную Go vulnerability database, повторяет сетевую попытку и применяет канонический fallback. Если база недоступна, статус уязвимостей остаётся `UNKNOWN`, а gate завершается ошибкой без fingerprint. Сообщение `PASS` означает реальное успешное выполнение.
