# Политика безопасности

## Поддерживаемая версия

Security fixes выпускаются для текущей major-версии 2.x. Версии 1.x больше не получают исправления desktop lifecycle.

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

CI запускает repository audit, Bandit, pip-audit, govulncheck, staticcheck, Gitleaks current/history, CodeQL и dependency review. Сетевой сбой security check считается ошибкой/непроверенным состоянием, а не отсутствием уязвимостей.
