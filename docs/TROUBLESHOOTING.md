# Устранение проблем

## WebView2 не установлен

Разрешите приложению запустить встроенный Evergreen Bootstrapper или установите Runtime с официальной страницы Microsoft WebView2. После установки перезапустите EXE. Fixed Version runtime не нужен.

## Приложение уже запущено

Второй запуск должен активировать существующее окно. Проверьте панель задач и Alt+Tab. Если после аварийного завершения процесса нет, новый запуск сразу создаёт окно: single-instance lock принадлежит процессу и не использует stale lock file или порт.

## Новости/версия недоступны

Основная база остаётся полностью рабочей offline. Разрешите HTTPS к `api.github.com` и `raw.githubusercontent.com`; приложение не требует доступа к VK. При временной ошибке сохраняется last-known-good preview.

## Профиль не загрузился

Закройте приложение и сохраните копию `%APPDATA%\IrisOnlineDatabase\UserData`. Проверьте `Backups\profile.json.bak`. Не редактируйте profile во время работы приложения.

## Диагностика сборки

Запустите `IrisTools.ps1 -Action Check`, затем `-Action Test`. Gate не считает network timeout успешной security-проверкой. Generated files и audit environments находятся вне source; если Git status изменился, сначала осознанно разберите изменения, не используйте `git reset --hard` или `git clean -fdx`.

`Check` и `Install` автоматически показывают стандартный Windows UAC prompt. Строка `PowerShell` должна содержать `Administrator / OK`. Если UAC отменён, повторите команду и подтвердите запрос; вручную запускать весь release gate от администратора не нужно.

Если `govulncheck` выдаёт `NETWORK/INFRASTRUCTURE SKIP`, разрешите исходящий HTTPS/443 к `vuln.go.dev` и `storage.googleapis.com` в firewall/proxy, затем повторите `-Action Test`. Это не означает, что в коде найдена уязвимость, но и не подтверждает её отсутствие: до успешного запроса RELEASE-gate остаётся закрытым.

Если winget только что установил Git или Node.js, launcher обновляет `PATH` текущего процесса и проверяет executable повторно. Строки Ruff, Bandit, pip, pip-audit, Playwright и PyYAML относятся к изолированному окружению `%LOCALAPPDATA%\IrisOnlineDatabase\BuildTools\python-audit`, а не к глобальному Python.
