# Release process

Выпуск Iris Online Database выполняется только из Windows и только из единственного канонического Git working tree. Не создавайте отдельную GitHub-копию source.

## Порядок

1. На `main` закоммитьте все изменения и убедитесь, что `git status` чист. Пункт `PREPARE RELEASE` сначала обновляет сведения об `origin/main`, применяет только безопасные детерминированные автоисправления (`gofmt`, `go mod tidy`, Ruff `--fix` без `--unsafe-fixes`, Ruff formatter) и при необходимости автоматически amend'ит локальный release-коммит. После этого весь строгий gate запускается заново уже для нового чистого HEAD.
2. `IrisTools.ps1 -Action Test` остаётся строгой read-only проверкой без автоисправлений. Только после PASS в `.git/iris-release-fingerprint.json` атомарно записываются source SHA-256, HEAD, branch, version, tracked file count, UTC timestamp и toolchain.
3. `IrisTools.ps1 -Action Build -OutputDirectory <внешняя папка>` повторно проверяет fingerprint и создаёт `IrisOnlineDB-X.Y.Z-Windows-x64.exe`, `IrisOnlineDB-X.Y.Z-Windows-x86.exe`, `IrisOnlineDB-X.Y.Z-Windows-arm64.exe` плюс единый `SHA256SUMS.txt` вне source.
4. Повторите сборку в чистой внешней папке и сравните SHA-256. Расхождение — release blocker.
5. `IrisTools.ps1 -Action Publish` ещё раз проверяет fingerprint и отправляет ровно проверенный HEAD в `origin/main`.
6. Дождитесь PASS обязательных Windows CI, Windows race, Windows Wails matrix и CodeQL checks для этого HEAD.
7. `IrisTools.ps1 -Action Release` проверяет remote HEAD/check-runs, локальные artifacts, создаёт подписанный `vX.Y.Z` tag и GitHub release с ранее проверенными файлами.

После стадии автоисправления любое новое tracked/untracked изменение source, другой HEAD/branch или отсутствующий/устаревший fingerprint блокирует build/publish/release. Автоисправление не применяется к результатам тестов, Staticcheck, govulncheck, Bandit, pip-audit, data audit или security checks: такие ошибки требуют анализа и оставляют gate в состоянии FAIL. Build удаляет только известные generated Wails paths и проверяет, что Git state не изменился.
