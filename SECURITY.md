# Политика безопасности

## Поддерживаемая версия

Security fixes выпускаются для текущей major-версии 2.x. Версии 1.x больше не получают исправления desktop lifecycle.

Сборки Windows 7/8/8.1 используют те же актуальные зависимости приложения и проверяемый патч Go, но требуют WebView2 109, который Microsoft больше не обновляет. Поддержка запуска приложения не восстанавливает безопасность самой ОС и Runtime. Подробнее: [Windows 7/8/8.1](docs/WINDOWS_LEGACY.md).

## Сообщение об уязвимости

Не публикуйте секреты, персональные пути, crash dumps или рабочие профили в публичном issue. Используйте GitHub Security Advisory репозитория. В отчёте достаточно версии, воспроизводимых действий и обезличенного результата.

## Модель безопасности

- UI и игровые assets встроены в EXE и обслуживаются Wails внутри процесса без TCP-listener.
- Внутренний handler принимает только origin/host `wails.localhost`, проверяет методы, лимиты JSON и cross-site запросы.
- CSP: только self scripts/styles/network; frames, objects, camera, microphone и geolocation запрещены.
- WebView file drop и release devtools отключены; bindings недоступны remote origins.
- Внешние URL разрешены только по HTTPS и exact-host allowlist, без userinfo, IP-адресов и нестандартных портов.
- Профиль записывается только в выделенный каталог приложения, атомарно и с проверкой path safety.
- У приложения нет токенов VK/GitHub, встроенных credentials, telemetry или analytics.

CI запускает repository audit, Bandit, pip-audit, govulncheck, staticcheck, Gitleaks current/history, CodeQL для Go, Python и JavaScript, а также dependency review. Анализ Go учитывает теги production-сборки; перед публикацией проверяются полный SHA в EXE и подпись манифеста. Сетевой сбой security check считается ошибкой/непроверенным состоянием, а не отсутствием уязвимостей.
