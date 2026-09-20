# Release process

Выпуск Iris Online Database выполняется только из Windows и только из единственного канонического Git working tree. Не создавайте отдельную GitHub-копию source.

## Порядок

Если архив распакован без `.git`, `PREPARE RELEASE` сначала получает историю канонического репозитория без checkout исходников. `tools/archive_base.txt` задаёт проверенную основу архива. Содержимое файлов сверяется до и после восстановления; изменения архива фиксируются отдельным локальным коммитом поверх ветки `sourceBranch` из `build/release.json` (по умолчанию `main`). Для подготовки 2.1 это `ui-audit-2.1`. При изменившейся исходной ветке, сетевой ошибке или изменении файлов во время операции восстановление останавливается. Существующая `.git` сохраняется.

1. На ветке подготовки из `build/release.json` или на `main` закоммитьте все изменения и убедитесь, что `git status` чист. Пункт `PREPARE RELEASE` сначала обновляет сведения об `origin/main`, применяет только безопасные детерминированные автоисправления (`gofmt`, `go mod tidy`, Ruff `--fix` без `--unsafe-fixes`, Ruff formatter) и при необходимости автоматически amend'ит неопубликованный локальный release-коммит. Уже опубликованные коммиты автоматически не изменяются. После этого весь строгий gate запускается заново уже для нового чистого HEAD.
2. `IrisTools.ps1 -Action Test` остаётся строгой read-only проверкой без автоисправлений. Только после PASS в `.git/iris-release-fingerprint.json` атомарно записываются source SHA-256, HEAD, branch, version, tracked file count, UTC timestamp и toolchain.
3. `IrisTools.ps1 -Action Build -OutputDirectory <внешняя папка>` повторно проверяет fingerprint и создаёт пять EXE: `IrisOnlineDB-<версия>-Windows-x64.exe`, `IrisOnlineDB-<версия>-Windows-x86.exe`, `IrisOnlineDB-<версия>-Windows-arm64.exe`, `IrisOnlineDB-<версия>-Windows-7-8.1-x64.exe`, `IrisOnlineDB-<версия>-Windows-7-8.1-x86.exe`. Единый `SHA256SUMS.txt` содержит все пять файлов; результаты хранятся вне source.
4. Повторите сборку в чистой внешней папке и сравните SHA-256. Расхождение — release blocker.
5. После отдельного одобрения слияния перенесите подготовленные изменения в `main` и повторите проверки и сборку для нового HEAD. `IrisTools.ps1 -Action Publish` ещё раз проверяет fingerprint и отправляет ровно проверенный HEAD в `origin/main`.
6. Дождитесь PASS обязательных Windows CI, Windows race, Windows Wails matrix и CodeQL checks для этого HEAD.
7. `IrisTools.ps1 -Action Release` проверяет remote HEAD/check-runs, локальные artifacts, создаёт подписанный `v<версия>` (для этого релиза — `v2.1`) tag и GitHub release с ранее проверенными файлами.

После стадии автоисправления любое новое tracked/untracked изменение source, другой HEAD/branch или отсутствующий/устаревший fingerprint блокирует build/publish/release. Автоисправление не применяется к результатам тестов, Staticcheck, govulncheck, Bandit, pip-audit, data audit или security checks: такие ошибки требуют анализа и оставляют gate в состоянии FAIL. Build удаляет только известные generated Wails paths и проверяет, что Git state не изменился.

Перед первым выпуском поддержки Windows 7/8/8.1 выполните проверку интерфейса и WebView2 на этих ОС по [WINDOWS_LEGACY.md](WINDOWS_LEGACY.md). CI на современной Windows и успешная компиляция не заменяют эту проверку.

## Подтверждение файлов

`verify_executables.py` требует `--expected-commit` с полным SHA проверенного коммита. Один номер версии недостаточен. При публикации `Sign-ReleaseChecksums` подписывает `SHA256SUMS.txt` настроенным ключом релизов и проверяет подпись и fingerprint перед загрузкой. Подпись GPG относится к манифесту; встроенная Windows Authenticode в этой сборке не добавляется.

После независимой проверки отпечатка публичного ключа подпись проверяется командой `gpg --verify SHA256SUMS.txt.asc SHA256SUMS.txt`; затем необходимо сравнить SHA-256 самого EXE с записью манифеста.

Для защиты `main` нужны обязательные результаты `Windows quality and security`, `Windows race detector`, `Native Windows Wails release matrix`, `Analyze (go)`, `Analyze (python)` и `Analyze (javascript-typescript)`. Применение правил требует прав администратора GitHub. Существующая автоматизация новостей делает прямой push: при включении обязательных проверок её способ записи должен быть согласован с правилами ветки. Общий обход защиты для всех участников не требуется.

Для релизной подписи используется публичный GitHub-адрес автора `137914856+fsibatov@users.noreply.github.com`. Личный адрес в исходниках не хранится. Настройка локальной Git identity и доступ к ключу подписи проверяются до публикации.
