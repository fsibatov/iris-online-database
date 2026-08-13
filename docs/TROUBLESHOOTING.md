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
