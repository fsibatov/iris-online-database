# Сторонние компоненты

© 2026 Iris Online Database

## Wails v2.14.0

Wails связывает Go backend с системным WebView и распространяется под MIT License. Source/release: `github.com/wailsapp/wails`.

## Microsoft Edge WebView2 Runtime

WebView2 Runtime — системный компонент Microsoft. EXE включает только Evergreen Bootstrapper через официальный Wails build mode `-webview2 embed`; сам Runtime не включён в source или release artifact.

## Go dependencies

Точный список и cryptographic module sums находятся в `go.mod`/`go.sum`. Основные runtime dependencies Wails и его transitives используют permissive MIT/BSD/Apache-style licenses; перед каждым major release список повторно проверяется.

## Go runtime для Windows 7/8/8.1

Используются пять адаптированных файлов официального Go 1.26.6. Лицензия Go BSD находится в `tools/compat/GO-LICENSE.txt`. Дифф основан на восстановлении совместимости в [XTLS/go-win7](https://github.com/XTLS/go-win7/tree/ccc83255f9ad36e06b119066d0b704944afeeb9a); точный источник и контрольные суммы записаны в `tools/compat/windows7.json`. Отличия от исходного диффа описаны в `WINDOWS_LEGACY.md`. Готовый сторонний компилятор не распространяется.

## Python/Chromium tools

Ruff, Bandit, pip-audit, PyYAML и Playwright/Chromium используются только для разработки, CI и VK updater. Они не входят в Windows release EXE. Exact pins находятся в `tools/requirements-audit.txt`.

Iris Online Database — неофициальное фанатское приложение для Iris Online. Проект не связан с разработчиками, издателями или правообладателями игры. Все игровые материалы, названия, логотипы и товарные знаки принадлежат их соответствующим правообладателям.

Официальный сайт игры: https://irisonline.ru/

Игровые данные и знаки Iris Online не перелицензируются этим документом и принадлежат соответствующим правообладателям.
