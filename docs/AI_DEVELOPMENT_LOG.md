# Авторазработка Iris Online Database

## 19 октября 2026 — интеграция исправлений кеширования

Исходная база: `main` / `f17d835e7443eec6d0dd5884ad3ddb53867ad214`, версия `2.0.5`.
Ветка продолжения: `ai-development`. Исходная ветка `main` и GitHub Release не изменялись.

### Принятые изменения

- Интегрированы ранее подготовленные исправления v1–v4: вытеснение просроченных записей перед актуальными, расширение кешируемых игровых каталогов, ограниченная буферизация больших ответов с потоковой передачей, пропуск приватных/условных запросов.
- Реализован обход кеша для входящих `Cache-Control`/`Pragma`, корректное распознавание приватных HTTP-директив, включая `max-age="0"`.
- Исключено кеширование ответов с `Content-Encoding`, `Content-Range`, `Pragma`, `Set-Cookie`, `Vary` и приватными директивами, поскольку полный набор этих заголовков не может быть безопасно воспроизведён.
- Ключ кеша разделяет `Accept`, `Accept-Language`, `Accept-Encoding`, `Accept-Charset`; чрезмерно длинные ключи обходят кеш. Безопасные заголовки `Content-Type`, `Content-Language`, `Cache-Control`, `ETag`, `Last-Modified` сохраняются при попадании в кеш.
- Добавлены регрессионные Go-тесты кеша и интегрирован `00_RELEASE_WINDOWS.bat`, который работает и из корня репозитория, и рядом с его каталогом.

### Фактически выполненные проверки

- Полный ZIP исходного проекта с `.git`: CRC проверки исходного архива OK; объекты Git присутствуют, история читается.
- Изолированный тестовый модуль, содержащий точные реализации кеша и кеширующей части HTTP-обработчика: Go tests (40 повторений), `go vet`, Race Detector (6 повторений) — PASS.
- Python unittest (`PYTHONPATH=tools`): 217 тестов PASS.
- `tools/data_presentation_audit.py`: `fatal=[]`; встроенные игровые ресурсы не изменены.
- `node --check web/app.js`: PASS.
- Полная сборка, Go-тесты всего приложения, Windows Wails/WebView2, Playwright, security gate: NOT RUN в Linux, поскольку необходимый Go toolchain 1.26.6, Windows и закреплённые зависимости отсутствуют. **Не считать этот архив релизом.**

### Следующая итерация

Проверить совместимость и последовательно восстановить ранее подготовленные отдельные патчи v6–v9 (singleflight, конвейер ограничения параллельности и graceful shutdown). Перед интеграцией обязательны полный контекст исходного кода и тесты; не применять механически без проходящих проверок. v5 и v11 частично воссозданы непосредственно в коде выше, исходные патчи отсутствовали.

Сохранять изменения только в отдельной ветке. Не публиковать `main`, теги и GitHub Releases без разрешения владельца. Для реального релиза выполнить канонический Windows Quality Gate.

## 19 октября 2026 — дедупликация параллельных запросов к игровому каталогу

- В `responseCache` добавлена координация одновременных промахов по полному ключу представления (URI + Accept*). Генератор ответа освобождает ожидающих через `defer`; ожидание не удерживает LRU mutex.
- Переставлен HTTP middleware-конвейер: проверки источника и жизненного цикла остаются внешними, кеш находится перед 32-слотовым ограничителем исполнения API. Ожидающие одинаковый ответ больше не расходуют backend-слоты.
- Если ответ не кешируется, ожидающие повторно получают право выполнения последовательно; отмена HTTP-запроса и завершение приложения прерывают ожидание. Ключи разных серверов сохраняют изоляцию.
- Добавлены `cache_singleflight_test.go`, `cache_pipeline_test.go`, `cache_pipeline_v8_test.go`, `cache_shutdown_v9_test.go` (конкуренция, приватные ответы, разные серверы, лимит, отмена и shutdown).

### Проверки текущей итерации

- Дефект воспроизведён на предыдущем HEAD в изолированном Go 1.23-контуре: 48 одинаковых одновременных запросов запускали **48** обработчиков вместо одного (тест FAIL до исправления).
- Изолированный контур содержит актуальные `cache.go` и **точные реализации HTTP middleware/кеша из `server.go`**, но заменяет тип `application` тестовой заглушкой: Go tests **40 повторений PASS**, Race Detector **5 повторений PASS**, `go vet` PASS, `gofmt` PASS. На исправленном коде те же 48 запросов дают **1** вызов обработчика; тесты также проверяют отсутствие HTTP 503 при 96 идентичных запросах и пиковую параллельность 1 для 64 некешируемых запросов.
- Python unittest (`PYTHONPATH=tools`): **217 PASS**; `data_presentation_audit.py`: `fatal=[]`; `node --check web/app.js`: PASS; `repository_audit.py`: PASS; `git fsck`: PASS. Встроенные игровые ресурсы и профиль не изменены.
- Полные Go-тесты приложения, Go/Wails Windows build, `IrisTools.ps1 -Action Test/Build`, Staticcheck, govulncheck, Ruff, Bandit, pip-audit, Gitleaks, Playwright: **NOT RUN**. Среда Linux с Go 1.23.2, проект требует Go 1.26.6 и Windows; сетевой доступ к toolchain/зависимостям недоступен. Это **не** Windows release PASS.

Сохранено в локальной ветке `ai-development`; `main`, remote, tags и Releases не изменялись.

## 19 октября 2026 — сохранение HTTP-заголовков и приватности кеша

- В `server.go` добавлена строгая проверка заголовков перед записью ответа в кеш: разрешены только пять однозначно воспроизводимых полей (`Content-Type`, `Content-Language`, `Cache-Control`, `ETag`, `Last-Modified`) с ровно одним непустым значением. Ответы с неизвестными, дублированными или многозначными полями передаются клиенту без изменений, но не кешируются.
- Устранена ошибка проверки только первого `Cache-Control`: последовательность `public, max-age=60` + `no-store` ранее сохранялась в кеше и при повторном запросе теряла директиву `no-store`. Теперь такой ответ не кешируется независимо от порядка полей.
- Добавлен `cache_header_fidelity_test.go`: проверка обоих порядков `Cache-Control`, сохранения `Content-Disposition`, `Expires`, `X-Request-ID` и многозначных `Content-Language`/`ETag`.

### Воспроизведение и QA

- До исправления новые регрессионные тесты **FAIL**: приватный ответ повторно выдавался из кеша; пять видов невоспроизводимых заголовков терялись на повторном запросе. После исправления все эти сценарии **PASS**.
- Изолированный Go 1.23.2-контур с точными `cache.go`, HTTP middleware/кеш-функциями `server.go` и всеми 29 тестами `cache_*_test.go`: `go test -count=50` PASS, `go test -race -count=5` PASS, `go vet` PASS, `gofmt` PASS. Контур заменяет тип `application` заглушкой, поэтому это **не** полный Go gate приложения.
- Python unittest (`PYTHONPATH=tools`): 217 PASS; `tools/data_presentation_audit.py --json`: `fatal=[]`; `node --check` для JavaScript: PASS; `git fsck`: PASS. Игровые ресурсы и профиль не изменены.
- Полный Go test/build, Wails, Windows `IrisTools.ps1 -Action Test/Build`, Staticcheck, govulncheck, Ruff, Bandit, pip-audit, Gitleaks, Playwright: **NOT RUN**. В Linux установлен Go 1.23.2, тогда как `go.mod` требует Go >=1.25 и Windows pin `.go-version` равен 1.26.6. Windows-only workflow validation также недоступна.

Продолжать с этого локального HEAD ветки `ai-development`; `main`, remote и Releases не изменялись.

## 19 октября 2026 — срок жизни HTTP-кеша и Cache-Control max-age

- Исправлено превышение срока свежести: ответ с `Cache-Control: max-age=1` ранее оставался во внутреннем кеше 5 минут (настроенный TTL приложения), теперь время хранения ограничено 1 секундой. Более короткий настроенный TTL по-прежнему имеет приоритет.
- Некорректные, нулевые и конфликтующие `max-age` теперь не допускают сохранение ответа в кеше; арифметика сравнения защищена от переполнения `time.Duration`.
- Добавлены регрессионные тесты `cache_max_age_test.go`: срок хранения, кавычки/регистр, недействительные значения, повторные директивы, ограничение настроенным TTL. Ошибка воспроизведена до исправления в изолированном тестовом контуре и устранена после.

### Проверки

- Изолированный Go 1.23.2-контур с точными `cache.go`, кеширующей частью и middleware из `server.go`, всеми `cache_*_test.go` и заглушкой `application`: `go test -count=50` PASS, `go test -race -count=5` PASS, `go vet` PASS, `gofmt` PASS. **Не является полным Go gate приложения**.
- Python unittest (`PYTHONPATH=tools`): 217 PASS; `data_presentation_audit.py --json`: `fatal=[]`; `repository_audit.py`: PASS; `node --check web/app.js`: PASS; `git fsck`: PASS.
- Полные Go-тесты: BLOCKED (Go 1.23.2, `go.mod` требует >=1.25). Windows `IrisTools.ps1 -Action Test/Build`, Wails/WebView2, Staticcheck, govulncheck, Ruff, Bandit, pip-audit, Gitleaks, Playwright: NOT RUN. Windows release PASS не заявляется.

Изменения затрагивают только HTTP-кеш, тесты и этот журнал. Игровые ресурсы, профиль и ветка `main` не изменялись.

## 19 октября 2026 — возраст повторно выдаваемого HTTP-ответа (итерация 12)

- Исправлено повторное предоставление полного клиентского `Cache-Control: max-age` при попадании во внутренний кеш: ранее ответ, пролежавший в кеше 45 секунд, выдавался без `Age`, и клиент мог считать его новым. Теперь кеш хранит фактический TTL каждой записи и выставляет рассчитанный `Age` на повторной выдаче. Исходный `Cache-Control` и тело ответа сохраняются; для первого ответа `Age` не добавляется.
- Добавлен `cache_age_test.go`: воспроизводит отсутствие `Age` до исправления, моделирует 45 секунд хранения без задержки и проверяет возраст, метаданные, отсутствие лишнего обращения к обработчику.
- Проверки изолированного контура (точные `cache.go` и функции кеширования/middleware из `server.go`, заглушка `application`): **33 Go-теста ×20 PASS**, Race Detector ×3 PASS, `go vet` PASS; тест до исправления FAIL, после PASS. Это **не** полный Go gate.
- Python unittest (`PYTHONPATH=tools`): **217 PASS**; `data_presentation_audit.py --json`: `fatal=[]`; `repository_audit.py`: PASS; `node --check web/app.js`: PASS; gofmt, `git diff --check`, `git fsck`: PASS. Playwright smoke **BLOCKED** (Chromium browser runtime отсутствует).
- Полные Go-тесты: **BLOCKED** (Go 1.23.2 при `go.mod` >=1.25). Windows `IrisTools.ps1 -Action Test/Build`, Wails/WebView2, Staticcheck, govulncheck, Ruff, Bandit, pip-audit, Gitleaks: **NOT RUN**. Это не релизный Windows PASS. Игровые данные, профиль, ветка `main` и удалённый репозиторий не изменялись.

## 19 октября 2026 — защита кеша от повторного заполнения после shutdown (итерация 13)

- Исправлена гонка жизненного цикла: ранее обработчик, начавшийся до `application.shutdown()`, мог завершиться после очистки кеша и снова записать ответ в память. Регрессионный сценарий воспроизвёл **1 запись / 72 байта** после shutdown (FAIL до исправления).
- Добавлен атомарный терминальный `responseCache.Close()`: очистка и запрет последующих `Get`/`PutWithTTL` под одним mutex. `application.shutdown()` теперь вызывает `Close()`; существующий `Clear()` остаётся переиспользуемым. После исправления тест подтверждает **0 записей / 0 байт** даже при завершении старого обработчика после shutdown.
- Добавлены `cache_shutdown_cleanup_test.go` (управляемая конкурентная регрессия, терминальность Close, идемпотентность и обратная совместимость Clear). Игровые данные, профиль, сетевое поведение и `main` не менялись.

### Фактически выполненные проверки

- Изолированный Go 1.23.2-контур: точные `cache.go`, кеширующие/middleware-функции `server.go`, функции shutdown из `lifecycle.go`, тестовая заглушка `application` и все `cache_*_test.go`: `go test -count=50` PASS, `go test -race -count=5` PASS, `go vet` PASS, gofmt PASS. **Не полный Go gate приложения.**
- Python unittest (`PYTHONPATH=tools`): **217 PASS**; `data_presentation_audit.py --json`: `fatal=[]`; `repository_audit.py`: PASS; `node --check web/app.js`: PASS; `git diff --check`, `git fsck`: PASS.
- Полный Go test: BLOCKED (`go.mod` требует Go >=1.25, Linux Go 1.23.2, загрузка toolchain недоступна). Windows `IrisTools.ps1 -Action Test/Build`, Wails/WebView2, Staticcheck, govulncheck, Ruff, Bandit, pip-audit, Gitleaks и Playwright: NOT RUN. **Это не Windows release PASS.**

## 19 октября 2026 — согласованность Cache-Control при промахах и потоковой передаче (итерация 14)

- Исправлено различие HTTP-заголовков между первым ответом и попаданием во внутренний кеш: внешний middleware задавал `Cache-Control: no-store`, а буферизованный обработчик добавлял своё явное `Cache-Control: public, max-age=60` вторым полем. При попадании в кеш возвращалось только `public, max-age=60`. Теперь явная политика обработчика **заменяет** значение по умолчанию при первой выдаче, поэтому оба ответа содержат одно одинаковое поле.
- Тот же дефект устранён для больших ответов, которые передаются потоком и не сохраняются в кеше: `no-store` + `private, no-store` больше не дублируются. Прочие заголовки по-прежнему дополняют существующие, поэтому защитные CSP и X-Frame-Options сохраняются. Если обработчик не задаёт Cache-Control, внешний `no-store` остаётся при промахе и попадании в кеш.
- Добавлен `cache_pipeline_headers_test.go` с тремя регрессионными сценариями: промах/попадание, потоковая передача и сохранение политики по умолчанию. Первые два теста воспроизвели ошибку до исправления (**FAIL**: два поля Cache-Control вместо одного), после исправления **PASS**.

### Проверки и ограничения

- Изолированный Go 1.23.2-контур с **точными** `cache.go`, middleware/кеш-функциями из `server.go`, shutdown-функциями из `lifecycle.go`, всеми `cache_*_test.go` и заглушкой зависимостей `application`: `go test -count=30` PASS, `go test -race -count=5` PASS, `go vet` PASS, gofmt PASS. **Это не полный Go gate приложения**.
- Python unittest (`PYTHONPATH=tools`): **217 PASS**; `data_presentation_audit.py --json`: `fatal=[]`; `repository_audit.py`: PASS; `node --check web/app.js`: PASS; `git diff --check`, `git fsck`: PASS.
- Полный Go test: **BLOCKED** (`go.mod` требует Go >=1.25, установлен Linux Go 1.23.2). Playwright smoke: **BLOCKED** (Chromium запускается, но локальный HTTP заблокирован политикой среды: `net::ERR_BLOCKED_BY_ADMINISTRATOR`). `validate_workflows.py`: **NOT PASS** в Linux — Windows PowerShell parser недоступен (`WF001`, count=4). Windows `IrisTools.ps1 -Action Test/Build`, Wails/WebView2, Staticcheck, govulncheck, Ruff, Bandit, pip-audit, Gitleaks: **NOT RUN**. Windows release PASS не заявляется.
- Игровые данные The Original/Iris Kiss Kiss, профиль, `main`, remote и GitHub Releases не изменены.

## 19 октября 2026 — освобождение ожидающих запросов при закрытии HTTP-кеша (итерация 15)

- Исправлен жизненный цикл `responseCache`: `Close()` теперь атомарно удаляет все ключи `inflight` и пробуждает ожидающие запросы, даже если исходный обработчик ещё работает. Новые лидеры после закрытия не создаются; middleware возвращает HTTP 503 вместо запуска backend. Поздний и повторный вызов `release()` безопасен, повторное закрытие канала исключено.
- Добавлен `cache_close_inflight_test.go` с тремя регрессионными тестами. На исходном коде iter14 все три сценария **FAIL**: ожидающий не пробуждался, закрытый кеш принимал новые `inflight`, запрос после `Close()` выполнял backend (HTTP 200). После исправления **PASS**: ожидающий пробуждается, `inflight` после закрытия **1 → 0**, backend вызовов после закрытия **1 → 0**, HTTP 503.
- Изолированный Go 1.23.2-контур использует точные `cache.go`, кеширующие и middleware-функции `server.go`, shutdown-функции `lifecycle.go`, все `cache_*_test.go` и только заглушку типа `application`: `go test -count=30` PASS, `go test -race -count=5` PASS, `go vet` PASS, gofmt PASS. **Это не полный Go gate приложения.**
- Python unittest (`PYTHONPATH=tools`): **217 PASS**; `tools/data_presentation_audit.py --json`: `fatal=[]`; `tools/repository_audit.py`: PASS; `node --check web/app.js`: PASS; `git fsck`, `git diff --check`: PASS. Playwright smoke: **BLOCKED** (`FAIL [BROWSER_RUNTIME]`, браузерный runtime недоступен). Полные Go-тесты **BLOCKED** (Go 1.23.2, `go.mod` требует >=1.25). Windows `IrisTools.ps1 -Action Test/Build`, Wails/WebView2, Staticcheck, govulncheck, Ruff, Bandit, pip-audit, Gitleaks — **NOT RUN**. Игровые данные, профиль, `main`, remote и Releases не изменены.

## 19 октября 2026 — защитные HTTP-заголовки на отклонённых запросах (итерация 16)

- Исправлена неполная защита ответов HTTP 403: `withSecurityHeaders` прежде проверял `Host`/`Origin`/`Sec-Fetch-Site` **до** установки `Cache-Control: no-store`, CSP, `X-Content-Type-Options`, `X-Frame-Options` и прочих защитных заголовков. Теперь эти заголовки выставляются до проверки; статус 403 и отказ в исполнении обработчика сохранены.
- Добавлен `security_rejection_headers_test.go` с тремя сценариями: посторонний Host, `Sec-Fetch-Site: cross-site` и посторонний Origin. На исходном iter15 тест **FAIL** во всех трёх сценариях (отсутствуют no-store/CSP/прочие заголовки); после исправления **PASS**. Число отклонённых ответов без `Cache-Control: no-store` в тесте: **3 → 0**.
- Изолированный Go 1.23.2-контур содержит точные `cache.go`, middleware/кеш-функции `server.go`, shutdown-функции `lifecycle.go`, все `cache_*_test.go` и новый тест; заглушены только зависимости типа `application`. **42 верхнеуровневых Go-теста**, `go test -count=30` PASS, `go test -race -count=5` PASS, `go vet` PASS, gofmt PASS. Это **не** полный Go gate приложения.
- Python unittest (`PYTHONPATH=tools`, без pycache в исходниках): **217 PASS**; `tools/data_presentation_audit.py --json`: `fatal=[]` (13 927 предметов, 1 342 монстра); `tools/repository_audit.py`: PASS; `node --check web/app.js`, `git diff --check`, `git fsck`: PASS.
- Полные Go-тесты **BLOCKED** (`go.mod` требует >=1.25, установлен Go 1.23.2); Playwright smoke **BLOCKED** (`FAIL [BROWSER_RUNTIME]`); `tools/validate_workflows.py` **NOT PASS** (`FAIL [WF001] count=4`, Windows PowerShell parser отсутствует). Windows `IrisTools.ps1 -Action Test/Build`, Wails/WebView2, Staticcheck, govulncheck, Ruff, Bandit, pip-audit, Gitleaks: **NOT RUN**. Не считать Windows-релизом.
- Игровые ресурсы The Original/Iris Kiss Kiss, пользовательский профиль, `main`, remote и Releases не изменялись.

## 19 октября 2026 — отменённые запросы не занимают API-слоты (итерация 17)

- Исправлена гонка допуска в `withAPIConcurrencyLimit`: когда контекст API-запроса уже отменён, свободный слот и ветка `Context.Done()` одновременно готовы к `select`, поэтому прежняя реализация могла случайно запустить backend. Добавлена проверка отмены **до** выбора слота и повторная проверка **после** его получения; существующий лимит 32 обработчиков и поведение активных запросов сохранены.
- Добавлен `api_concurrency_cancel_test.go` с двумя сценариями. На исходном коде iter16 тест отменённого запроса **FAIL**: первый уже отменённый запрос получил HTTP 204 и вызвал backend вместо HTTP 408. После исправления 64/64 отменённых запросов получили HTTP 408, вызовов backend — **0**; активный запрос по-прежнему получил HTTP 204.
- Изолированный Go 1.23.2-контур с точными функциями кеша, HTTP middleware и shutdown из исходников, тестами `cache_*_test.go`, `security_rejection_headers_test.go`, `api_concurrency_cancel_test.go` и минимальной заглушкой `application`: **44 верхнеуровневых Go-теста**, `go test -count=30` PASS, `go test -race -count=5` PASS, `go vet` PASS, gofmt PASS. Это **не** полный Go gate.
- Python unittest (`PYTHONPATH=tools`, pycache вне проекта): **217 PASS**; `tools/data_presentation_audit.py --json`: `fatal=[]`; `tools/repository_audit.py`: PASS; `node --check web/app.js`, `git diff --check`, `git fsck`: PASS.
- Полные Go-тесты **BLOCKED** (`go.mod` требует Go >=1.25, установлен Linux Go 1.23.2); Playwright **BLOCKED** (`FAIL [BROWSER_RUNTIME]`); workflow validation **NOT PASS** (`FAIL [WF001] count=4`, нет Windows PowerShell parser). Windows `IrisTools.ps1 -Action Test/Build`, Wails/WebView2, Staticcheck, govulncheck, Ruff, Bandit, pip-audit, Gitleaks: **NOT RUN**. Windows release PASS не заявляется.
- Игровые данные The Original/Iris Kiss Kiss, профиль, `main`, remote и Releases не изменялись.

## 19 октября 2026 — отменённый запрос не получает ответ из HTTP-кеша (итерация 18)

- В `withLifecycleGuard` добавлена проверка отмены контекста API-запроса **перед** обращением к кешу. До исправления кеш-хит обходил backend-лимитер (где отмена уже проверялась) и возвращал HTTP 200 с сохранённым телом уже отменённому запросу. Теперь 64/64 заранее отменённых запросов к прогретому кешу возвращают HTTP 408, не раскрывают кешированное тело и не запускают backend. Для действующего запроса кеш-хит и отсутствие повторного backend-вызова сохранены.
- Сохранён приоритет HTTP 503 при завершении приложения, а также `Cache-Control: no-store` и CSP на отклонённых запросах. Добавлен `api_cancel_cached_test.go` с двумя тестами: отменённый кеш-хит (FAIL на iter17, PASS после правки) и приоритет shutdown (PASS).
- Изолированный контур Go 1.23.2 использовал неизменённые `cache.go`, **точные исходные функции HTTP-конвейера из `server.go`** и `requestShutdown` из `lifecycle.go`, минимальную заглушку `application` и 44 верхнеуровневых теста (`cache_*_test.go`, `security_rejection_headers_test.go`, `api_concurrency_cancel_test.go`, новые тесты; `cache_shutdown_cleanup_test.go` исключён из контура, так как требует полной модели приложения). `go test -count=30` PASS, `go test -race -count=5` PASS, `go vet` PASS, `gofmt -l *.go` пусто. **Не является полным Go gate**.
- Python unittest (`PYTHONPATH=tools`, pycache вне source): 217 PASS; `data_presentation_audit.py --json`: `fatal=[]`, `warnings=[]`; `repository_audit.py`: PASS; `node --check web/app.js`: PASS; `git fsck --full --no-reflogs` и `git diff --check`: PASS.
- Полные Go-тесты **BLOCKED** (Go 1.23.2 при `go.mod` >=1.25); Playwright smoke **FAIL [BROWSER_RUNTIME]**; workflow validation **NOT PASS [WF001] count=4** (Windows PowerShell отсутствует). Windows `IrisTools.ps1 -Action Test/Build`, Wails/WebView2, Staticcheck, govulncheck, Ruff, Bandit, pip-audit, Gitleaks: **NOT RUN**. Никакого Windows release PASS.
- Игровые данные обоих серверов, профиль, ветка `main`, remote, теги и Releases не изменялись.

## 19 октября 2026 — проверка жизненного цикла при повторной выдаче кеша (итерация 19)

- Исправлена гонка между первоначальной проверкой жизненного цикла и выдачей готовой записи из HTTP-кеша. Если запрос отменялся либо начиналось завершение приложения, пока `responseCache.Get` ожидал mutex, прежний `withResponseCache` возвращал кешированное тело с HTTP 200. Теперь непосредственно перед каждой выдачей записи повторно проверяются завершение приложения (HTTP 503 с приоритетом) и отмена запроса (HTTP 408). Действующие запросы и кеширование каталогов не изменены.
- Добавлен `cache_replay_lifecycle_test.go` с двумя детерминированными регрессионными сценариями: отмена и shutdown после первой проверки, но до завершения `Get`. На исходном iter18 оба теста **FAIL** (HTTP 200 и раскрытое кешированное тело); после исправления **PASS** (соответственно HTTP 408/503, без тела каталога и дополнительных backend-вызовов). На отказах сохранены `Cache-Control: no-store` и CSP.
- Изолированный контур Go 1.23.2 с точными исходными функциями кеша/HTTP middleware и `requestShutdown` и минимальной заглушкой `application`: **46 верхнеуровневых Go-тестов**, `go test -count=30 ./...` PASS, `go test -race -count=5 ./...` PASS, `go vet ./...` PASS, `gofmt` PASS. **Не является полным Go gate приложения.**
- Python unittest (`PYTHONPATH=tools`, без pycache): **217 PASS**; `tools/data_presentation_audit.py --json`: `fatal=[]` (13 927 предметов, 1 342 монстра); `tools/repository_audit.py`: PASS; `node --check web/app.js`, `git diff --check`, `git fsck`: PASS.
- Полные Go-тесты **BLOCKED** (Go 1.23.2, `go.mod` требует >=1.25); Playwright smoke **NOT PASS [BROWSER_RUNTIME]**; workflow validation **NOT PASS [WF001] count=4** (нет Windows PowerShell parser). Windows `IrisTools.ps1 -Action Test/Build`, Wails/WebView2, Staticcheck, govulncheck, Ruff, Bandit, pip-audit, Gitleaks: **NOT RUN**. Не считать Windows-релизом.
- Игровые данные обоих серверов, профиль, ветка `main`, remote, теги и Releases не изменялись.

## 19 октября 2026 — фиксация HTTP-заголовков при WriteHeader (итерация 20)

- Исправлено несоответствие `captureWriter` контракту `http.ResponseWriter`: заголовки, изменённые обработчиком **после** явного `WriteHeader` или первого `Write`, прежде ошибочно попадали в отправленный ответ и влияли на решение о кешировании. Теперь `WriteHeader` сохраняет копию заголовков, а потоковая передача, буферизованный ответ и проверка допустимости кеширования используют только зафиксированную копию. Обработчики без `WriteHeader` и без тела по-прежнему передают текущие заголовки.
- Добавлен `cache_header_commit_test.go` с тремя регрессионными тестами: явный `WriteHeader` (буферизация и повторная выдача), потоковая передача и неявный `WriteHeader` при первом `Write`. На неизменённом HEAD итерации 19 **все три FAIL**: поздние `Content-Type`, `Cache-Control` и `Set-Cookie` достигали клиента, повторные запросы вызывали backend дважды вместо одного. После исправления **все три PASS**: поздние заголовки не передаются, кеш безопасно переиспользует зафиксированное представление (2 → 1 вызов backend).
- Изолированный Go 1.23.2-контур с неизменённым `cache.go`, точными исходными функциями HTTP middleware/кеша из `server.go`, точным `requestShutdown` из `lifecycle.go`, минимальной заглушкой `application` и 49 верхнеуровневыми тестами: `go test -count=30 ./...` PASS, `go test -race -count=5 ./...` PASS, `go vet ./...` PASS, три новых теста ×100 PASS, `gofmt` PASS. **Это не полный Go gate приложения.**
- Python unittest (`PYTHONPATH=tools`, pycache вне проекта): **217 PASS**; `tools/data_presentation_audit.py --json`: `fatal=[]` (13 927 предметов, 1 342 монстра); `node --check web/app.js`: PASS. Полные Go-тесты **BLOCKED** (`go.mod` требует Go >=1.25, Linux Go 1.23.2). Playwright smoke **NOT PASS [BROWSER_RUNTIME]**; workflow validation **NOT PASS [WF001] count=4** (нет Windows PowerShell). Windows `IrisTools.ps1 -Action Test/Build`, Wails/WebView2, Staticcheck, govulncheck, Ruff, Bandit, pip-audit, Gitleaks: **NOT RUN**. Не считать Windows-релизом.
- Игровые данные The Original/Iris Kiss Kiss, пользовательский профиль, `main`, теги, remote и GitHub Releases не изменялись.

## 20 октября 2026 — отмена буферизованного ответа после завершения backend (итерация 21)

- Исправлена гонка жизненного цикла в `withResponseCache`: после начала обработки API-запрос мог быть отменён либо приложение могло начать shutdown, пока backend формировал **неотправленный буферизованный** ответ. Ранее ответ всё равно выдавался с HTTP 200, включая его тело, и при отмене запроса мог попадать в кеш. Теперь непосредственно после возврата backend, **до** передачи заголовков/тела и помещения в кеш, повторно проверяются shutdown (HTTP 503, приоритетный) и отмена контекста запроса (HTTP 408). Уже начатый потоковый ответ не отзывается: эта правка относится только к ещё не зафиксированным буферизованным ответам.
- Добавлен `cache_buffered_lifecycle_test.go`: детерминированные сценарии отмены и shutdown во время блокированного backend. На исходном HEAD iter20 оба теста **FAIL** (HTTP 200 и раскрытое тело); после исправления оба **PASS** (HTTP 408/503, без исходного тела). На отказах сохранены `Cache-Control: no-store` и CSP, отменённый ответ не записывается в кеш.
- Изолированный Go 1.23.2-контур содержит неизменённый `cache.go`, **точные исходные функции HTTP-конвейера/кеша из `server.go`**, точный `requestShutdown` из `lifecycle.go`, минимальную заглушку `application`, тесты `cache_*_test.go` (кроме требующего полного приложения `cache_shutdown_cleanup_test.go`), `security_rejection_headers_test.go`, `api_cancel_cached_test.go`, `api_concurrency_cancel_test.go`: **51 верхнеуровневый тест**, `go test -count=30` PASS, `go test -race -count=5` PASS, `go vet` PASS, gofmt PASS. Новые два теста ×100 PASS; **не полный Go gate приложения**.
- Python unittest (`PYTHONPATH=tools`, pycache вне исходников): **217 PASS**; `tools/data_presentation_audit.py --json`: `fatal=[]`; `tools/repository_audit.py`: PASS; `node --check web/app.js`: PASS. Полный `go test ./...` **BLOCKED** (Linux Go 1.23.2 при `go.mod` >=1.25). Playwright smoke **NOT PASS [BROWSER_RUNTIME]**, workflow validation **NOT PASS [WF001] count=4** (Windows PowerShell отсутствует). Windows `IrisTools.ps1 -Action Test/Build`, Wails/WebView2, Staticcheck, govulncheck, Ruff, Bandit, pip-audit, Gitleaks: **NOT RUN**. Это не Windows release PASS.
- Игровые данные The Original/Iris Kiss Kiss, профиль, `main`, теги, remote и Releases не изменялись.

## 20 октября 2026 — приоритет shutdown для ожидающего кеш-запроса (итерация 22)

- Исправлена гонка в `withResponseCache`: ожидающий `singleflight`-запрос, чей контекст отменён после начала завершения приложения, прежде возвращал HTTP **408**, хотя для shutdown принят приоритет HTTP **503**. Причина: ветка `select` по `r.Context().Done()` немедленно записывала 408 без повторной проверки `a.closing`/`a.ctx.Err()`. Теперь ветка повторно проверяет оба признака shutdown и возвращает 503 при их наличии, сохраняя 408 для обычной отмены.
- Добавлен `cache_follower_shutdown_priority_test.go`: три синхронизированных сценария (shutdown-флаг + отмена; отменённый контекст приложения + отмена запроса; только отмена запроса). Тестовый контекст сигнализирует достижение `select` без `Sleep`. **До исправления** первый сценарий детерминированно **FAIL** (408 вместо 503), обычная отмена PASS. **После исправления** все три сценария PASS, новый тест ×100 PASS и Race Detector ×20 PASS; защитные CSP/`no-store` сохранены, backend вызывается один раз.
- Изолированный Go 1.23.2-контур использует точные исходные функции HTTP middleware/кеша из `server.go`, исходный `cache.go`, `requestShutdown` из `lifecycle.go`, минимальную заглушку `application` и **52 верхнеуровневых теста**: `go test -count=30` PASS, `go test -race -count=5` PASS, `go vet` PASS, gofmt PASS. **Не является полным Go gate**.
- Python unittest (`PYTHONPATH=tools`, pycache вне исходников): **217 PASS**; `tools/data_presentation_audit.py --json`: `fatal=[]`; `tools/repository_audit.py`: PASS; `node --check web/app.js`, `git diff --check`, `git fsck`: PASS. Полные Go-тесты **BLOCKED** (`go.mod` требует >=1.25, доступен Linux Go 1.23.2). Playwright smoke **NOT PASS [BROWSER_RUNTIME]**; workflow validation **NOT PASS [WF001] count=4** (Windows PowerShell недоступен). Windows `IrisTools.ps1 -Action Test/Build`, Wails/WebView2, Staticcheck, govulncheck, Ruff, Bandit, pip-audit, Gitleaks: **NOT RUN**.
- Игровые данные The Original/Iris Kiss Kiss, профиль, `main`, теги, remote и Releases не изменялись. Полный Windows release gate не выполнялся.

## 20 октября 2026 — соблюдение s-maxage при внутреннем HTTP-кешировании (итерация 23)

- Исправлено нарушение политики свежести `Cache-Control` для кеша, разделяемого между HTTP-запросами: `responseCacheLifetime` ранее учитывал только `max-age` и игнорировал `s-maxage`. Ответ `public, max-age=60, s-maxage=0` ошибочно переиспользовался (второй ответ содержал тело первого), а `s-maxage=1` сохранялся на 60 секунд. Теперь нулевой `s-maxage` запрещает повторное использование, положительный ограничивает TTL вместе с `max-age` и внутренним лимитом, а некорректный или повторяющийся `s-maxage` запрещает кеширование. Для безопасности применяется минимальный разрешённый срок, без продления `max-age`.
- Добавлен `cache_shared_max_age_test.go`: 3 верхнеуровневых теста (7 проверяемых регрессионных случаев с подслучаями). **До исправления** все 3 верхнеуровневых теста FAIL: повторный запрос при `s-maxage=0` получал старое тело; TTL при `s-maxage=1` составлял 1 минуту вместо 1 секунды; пять некорректных директив кешировались. **После исправления** все PASS: два backend-вызова при `s-maxage=0`, TTL 1 секунда, некорректные директивы не кешируются.
- Изолированный Go 1.23.2-контур: точные функции HTTP middleware/кеша из `server.go`, неизменённый `cache.go`, точный `requestShutdown` из `lifecycle.go`, минимальная заглушка `application`, 55 верхнеуровневых тестов: `go test -count=30 ./...` PASS, `go test -race -count=5 ./...` PASS, `go vet ./...` PASS, `gofmt` PASS. **Это не полный Go gate приложения.**
- Python unittest (`PYTHONPATH=tools`, pycache вне проекта): **217 PASS**; `tools/data_presentation_audit.py --json`: `fatal=[]`; `tools/repository_audit.py`: PASS; `node --check web/app.js`: PASS. Полный Go gate **BLOCKED** (`go.mod` требует >=1.25, установлен Go 1.23.2). Playwright smoke **NOT PASS [BROWSER_RUNTIME]**; workflow validation **NOT PASS [WF001] count=4** (Windows PowerShell недоступен). Windows `IrisTools.ps1 -Action Test/Build`, Wails/WebView2, Staticcheck, govulncheck, Ruff, Bandit, pip-audit, Gitleaks: **NOT RUN**. Windows release PASS не заявляется.
- Игровые данные The Original/Iris Kiss Kiss, профиль, `main`, теги, remote и Releases не изменялись. Новых сетевых вызовов и телеметрии исправление не добавляет.

## 20 октября 2026 — отклонение JSON null при изменении профиля (итерация 24)

- Исправлен дефект `decodeJSONRequest` в `server.go`: `encoding/json` принимает JSON-литерал `null` при декодировании в Go-структуру без ошибки. Для `PUT /api/user-data` это позволяло передать нулевой `userProfile` в `profileStore.Replace`, создавая риск перезаписи сохранённых настроек. Теперь корневой `null` (в том числе с JSON-пробелами) отклоняется с HTTP 400 **до** декодирования. Обычные JSON-объекты и `null` внутри допустимых полей продолжают обрабатываться.
- Добавлен `json_null_request_test.go`: регрессии для профиля (обычный `null` и с пробелами) и POST избранного; позитивный тест для объекта с nullable-полем. На исходном коде iter23 **два верхнеуровневых теста FAIL** (три случая принимались вместо HTTP 400), позитивный PASS. После исправления **все три верхнеуровневых теста PASS**, HTTP 400 в трёх ранее ошибочных случаях.
- Изолированный Go 1.23.2-контур с **точными функциями HTTP middleware, кеша и JSON-декодера из `server.go`**, неизменённым `cache.go`, точным `requestShutdown` из `lifecycle.go`, минимальной заглушкой `application` и **58 верхнеуровневыми тестами**: `go test -count=30 ./...` PASS, `go test -race -count=5 ./...` PASS, `go vet ./...` PASS, gofmt PASS. Отдельные новые тесты JSON-декодера: `go test -count=100` и race ×10 PASS. **Это не полный Go gate приложения**.
- Python unittest (`PYTHONPATH=tools`, pycache вне проекта): **217 PASS**; `tools/data_presentation_audit.py --json`: `fatal=[]`; `tools/repository_audit.py`: PASS; `node --check` всех JS в `web/`: PASS; `git fsck`: PASS. Полный Go gate **BLOCKED** (Linux Go 1.23.2 при `go.mod` >=1.25). Playwright smoke **NOT PASS [BROWSER_RUNTIME]**; workflow validation **NOT PASS [WF001] count=4** (нет Windows PowerShell). Windows `IrisTools.ps1 -Action Test/Build`, Wails/WebView2, Staticcheck, govulncheck, Ruff, Bandit, pip-audit, Gitleaks: **NOT RUN**. Не считать Windows-релизом.
- Игровые ресурсы The Original/Iris Kiss Kiss, профиль на диске, `main`, теги, remote и GitHub Releases не изменялись; новых сетевых вызовов и телеметрии нет.
