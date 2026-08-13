# Архитектура v2

## Выбор desktop stack

На 13 августа 2026 года стабильная ветка Wails — v2.14.0; Wails v3 остаётся beta. Поэтому production v2.0.0 использует Wails 2.14.0, Go backend, существующий vanilla HTML/CSS/JavaScript и системный Microsoft WebView2. Electron не используется и отдельный Chromium runtime не поставляется.

## Поток выполнения

`main_windows.go` создаёт одно системное окно и передаёт Wails embedded `web/` assets. Запросы `/api/*` обрабатывает существующий Go `http.Handler` через Wails AssetServer — это внутрипроцессный adapter, а не `net.Listen`.

Внутренние переходы — hash routes (`#item/…`, `#monster/…`, `#recipe/…`). Внешний HTTPS-link перехватывается frontend и передаётся `DesktopBridge.OpenExternalURL`; Go повторно проверяет exact-host allowlist и открывает системный браузер.

`SingleInstanceLock` использует стабильный application UUID. Повторный запуск показывает и разворачивает существующее окно. X закрывает приложение; `OnShutdown` отменяет online operations, сбрасывает cache, записывает профиль и закрывает log writer.

## Storage

Backend profile path и schema v1 сохранены. `%LOCALAPPDATA%\IrisOnlineDatabase\WebView2` — стабильный WebView2 data directory, поэтому pending write переживает немедленное закрытие и повторный запуск. Подробности в `PROFILE_COMPATIBILITY.md`.

## Security boundaries

Production host — только `wails.localhost`. Remote origins не получают bindings. Embedded CSP разрешает `connect-src 'self'`; VK image CDN не разрешён. Updater сохраняет только текст JSON, поэтому приложение не загружает непроверенные VK thumbnails.

Release build использует tags `desktop,wv2runtime.embed,production`, `-trimpath`, пустой Go build ID и external output directory. Wails dev mode может иметь devtools; production build — нет.
