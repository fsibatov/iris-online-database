# Release process

Выпуск выполняется из единственного канонического Git working tree. Не создавайте отдельную GitHub-копию source.

## Порядок

1. На `main` закоммитьте все изменения и убедитесь, что `git status` чист.
2. `IrisTools.ps1 -Action Test` запускает строгий gate. Только после PASS в `.git/iris-release-fingerprint.json` атомарно записываются source SHA-256, HEAD, branch, version, tracked file count, UTC timestamp и toolchain.
3. `IrisTools.ps1 -Action Build -OutputDirectory <внешняя папка>` повторно проверяет fingerprint и создаёт `IrisOnlineDB-2.0.0-Windows-x64.exe`, `IrisOnlineDB-2.0.0-Windows-x86.exe`, `IrisOnlineDB-2.0.0-Windows-arm64.exe` плюс единый `SHA256SUMS.txt` вне source.
4. Повторите сборку в чистой внешней папке и сравните SHA-256. Расхождение — release blocker.
5. `IrisTools.ps1 -Action Publish` ещё раз проверяет fingerprint и отправляет ровно проверенный HEAD в `origin/main`.
6. Дождитесь PASS всех GitHub CI и CodeQL checks для этого HEAD.
7. `IrisTools.ps1 -Action Release` проверяет remote HEAD/check-runs, создаёт подписанный `v2.0.0` tag и GitHub release с ранее проверенными artifacts.

Любое tracked/untracked изменение source, другой HEAD/branch или удалённый fingerprint блокирует build/publish. Build удаляет только известные generated Wails paths и проверяет, что Git state не изменился.

Source ZIP формируется отдельно от EXE и не содержит release artifacts, caches, venv или stale fingerprint. После упаковки ZIP нужно распаковать, сравнить manifest hashes, запустить repository audit и повторить secret scan.
