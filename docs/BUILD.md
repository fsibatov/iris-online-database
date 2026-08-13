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

`Install` повторно использует Python audit venv и Playwright cache под `%LOCALAPPDATA%\IrisOnlineDatabase\BuildTools`; source не загрязняется. System tools устанавливаются только при явном `Install`.

## Linux CI / audit workstation

```bash
scripts/release-gate.sh
scripts/build-release.sh /absolute/path/outside/source
```

Cross-build Wails Windows amd64 выполняется без CGO. `go test -race` исполняется на Linux, где race detector поддержан и не требует менять Windows release binary.

## Обязательные проверки

Gate включает repository hygiene, gofmt, module verify/tidy-diff/list, build, tests, race (Linux), vet, staticcheck, govulncheck, Python unittest, Ruff, Bandit, full installed-environment pip-audit, YAML/action-pin validation workflows, audit встроенной проекции данных, JS syntax, deterministic Playwright frontend smoke, Gitleaks current/history и повторную Git cleanliness проверку.

`raw_projection_audit.py` и `drop_table_audit.py` остаются отдельными read-only forensic-инструментами для исходных игровых таблиц, которые не распространяются с приложением. Их нельзя выдавать за CI PASS без явных `--resource` либо `--normal`/`--groups`/`--world`; встроенные production assets проверяются `data_presentation_audit.py`, exact SHA-256 и regression-тестами.

Security/network checks ограничены watchdog. Timeout или download failure завершает gate ошибкой; сообщение `PASS` означает реальное успешное выполнение.
