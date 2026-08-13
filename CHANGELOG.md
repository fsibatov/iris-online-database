# История изменений

## 2.0.0 — 2026-08-13

### Added

- Нативное Windows-окно на Wails 2.14.0 и Microsoft WebView2 с системной иконкой, Alt+Tab, панелью задач, сворачиванием, разворачиванием и нормальным закрытием.
- Single-instance lifecycle: второй запуск активирует уже открытое окно.
- Строгий allowlist внешних HTTPS-ссылок и системное открытие вне WebView.
- Проверка Windows PE metadata, DPI manifest и hardening-флагов.
- Детерминированные VK fixtures и frontend smoke на нескольких DPI-scale factors.

### Changed

- Внешняя браузерная вкладка и UI-сервер на `127.0.0.1:8765` заменены embedded asset handler и внутренним Go/JavaScript transport Wails без TCP-listener.
- Проект использует один канонический Git/source tree для теста, сборки и публикации.
- Поддерживаемая release-архитектура сфокусирована на реально проверяемой Windows `amd64`.
- Блок «Предметы с наибольшим шансом» показывает компактные строки `название — процент`, не меняя математику выпадения.
- Release-скрипты сгруппированы в `scripts/`, а артефакты всегда создаются вне source tree.

### Fixed

- Исправлена Windows-проверка инструментов: native output больше не обрывается до завершения процесса, корректно распознаётся вывод `staticcheck.exe`, а Python packages проверяются в том же внешнем audit venv, куда были установлены.
- `Check` и `Install` автоматически запрашивают права администратора через UAC и повторно загружают `PATH` после winget без обязательного перезапуска терминала.
- VK updater больше не заменяет last-known-good новость пустым текстом или записью с меньшим ID.
- Восстановлено корректное превью записи №62337 из проверенной истории репозитория вместо пустого состояния №62336.
- Добавлены DOM/OpenGraph fallback, ограниченные retry/timeout и безопасная диагностика `ERR_ABORTED`/timeout.
- Принудительное обновление VK больше не очищает видимое последнее корректное превью при временной сетевой ошибке.
- Repository audit и его regression-тест используют единый redacted contract с категориями и точным количеством нарушений.
- Python shebang/executable mode приведены в соответствие с Ruff EXE001.

### Security

- CSP запрещает remote scripts, frames, objects и произвольные сетевые запросы WebView.
- Remote VK text всегда проходит escaping; внешняя навигация не получает доступ к Wails bindings.
- Audit/CodeQL paths не печатают raw failure payload, токены или пользовательские абсолютные пути.
- GitHub Actions имеют минимальные permissions и полные commit SHA; write permission оставлена только VK job.
- Gitleaks проверяет текущий source и полную Git history из официального бинарника с SHA-256.

### Build/CI

- Добавлены Linux quality/security, Go race и native Windows Wails build jobs.
- RELEASE fingerprint связывает source SHA-256, HEAD, branch, версии инструментов и конфигурацию.
- Все прямые Python audit/smoke-зависимости, включая Ruff, Bandit, pip-audit, Playwright и PyYAML, закреплены только в `tools/requirements-audit.txt`.
- pip-audit использует переиспользуемое окружение вне source и проверяет полное установленное dependency graph.
- Потенциально зависающие security/build/browser checks имеют watchdog и не превращают infrastructure failure в PASS.
- Windows `govulncheck` повторяет канонический запрос и переключается на Google-hosted storage endpoint той же Go Vulnerability Database; недоступность обоих адресов оставляет security status `UNKNOWN` и блокирует RELEASE fingerprint.
- Windows-wrapper считывает фактический exit code `govulncheck` через `System.Diagnostics.Process`; успешный ответ `No vulnerabilities found.` больше не классифицируется как `SECURITY FAIL`; паузы retry сокращены до 2 с.
- Вывод Windows-инструментов захватывается прямым процессом без PowerShell Job/remoting и декодируется как UTF-8; диагностический `RemoteException` и повреждение кириллицы в VK fixture output устранены.
- Windows `SelfTest` проверяет UTF-8 по детерминированной последовательности байтов через audit Python и при сбое сообщает безопасную категорию конкретного probe вместо одного непрозрачного счётчика.
- Repository audit проверяет executable mode Python-файлов по Git index, поэтому EXE001 regression одинаково работает на Windows и Linux; Windows runner сохраняет redacted traceback провалившегося tool вместо одной строки progress output.

Исторические изменения выпусков 1.x доступны в Git history и соответствующих тегах.
