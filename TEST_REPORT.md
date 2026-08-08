# Iris Online 1.0.1 — TEST_REPORT

Дата проверки: 2026-08-08.

Отчёт описывает текущее состояние исходного проекта Iris Online 1.0.1. Публикационная Windows-сборка должна выполняться Go из `.go-version` (`go1.26.5`). Проверочные EXE в этой среде собраны Go 1.23.2 только как diagnostic build.

## Игровые данные

UI/lifecycle/security-правки не меняли игровые assets. Финальные SHA-256:

- `assets/game_data.json.gz`: `7c3698494233696f2f5728ef17f7e13953159191f966d77b90742dbced23875e`
- `assets/set_effects.json.gz`: `b789ee576e4a006d1a5ddfe1addaf9e1fed334703c841f6be36282ef95839673`
- `assets/item_abilities.json.gz`: `7e5113eb7d75879614b7768750fe08d8ae9c11ca5fabe5a3708c8d0851f05351`
- `assets/item_recipes.json.gz`: `3613929c92ca01af620d75a88c5401a2cc8a35702d23169090f2ced612acdee8`
- `assets/monster_details.json.gz`: `3601607f8bd7e4919738d8ccd3bfd99212526560a46771a7b9df880d0bae25d5`
- additive `assets/chest_contents.json.gz`: `2b77350006cfb7f992c1707b9d0c703ebb82206dac1810084aa105d0878fe98b`

Data presentation/completeness audit: PASS.

- предметов: 13 927;
- монстров: 1 342;
- embedded sets: 467;
- строк эффектов комплектов: 972;
- реальные thresholds: `2, 3, 4, 5`;
- active effects: 160;
- комплектов с порогом 5: 126; active effects на пороге 5: 122;
- recipes: 1 207; material links: 4 129;
- item-change containers: 399 profiles / 4 944 item rows на каждый сервер;
- 24 output item ID отсутствуют в публичной item-таблице: ID сохраняются, UI показывает нейтральную подпись без битой ссылки;
- профиль `873079` имеет исходные `changerate` выше 1 000 000: содержимое сохраняется, неподтверждённый процент не показывается;
- fatal data-loss findings: отсутствуют.

Цвета редкости не менялись: unique `#fff600`, epic `#d800ff`, rare `#00fffc`, normal `#ffffff`, magic `#00ff00`, shop `#ffcd00`. Формат цены продажи остаётся `Цена продажи: 2,460 тер`.

Русские количественные формы в UI используют общий formatter с правилом для `11–14` и последних цифр `1 / 2–4 / 0,5–9`. Regression cases включают `1, 2, 4, 5, 11–14, 21–25, 101, 111–114, 121–125`; подписи предметов, источников, вариантов, записей и дополнительных попыток проверены.

## Drop model

`Drop.cpp` и `DropScript.cpp` использовались только как reference и не входят в распространяемый source tree:

- `Drop.cpp`: `4ce16a7721b61e55ec8a188a45588b98ec8f6f4792543c2d8995c4f349d1c390`
- `DropScript.cpp`: `5a9726db029dfe2ed13c2a66ee13f11d404f41b2d06adf399a16aceedeb37c60`

Deterministic reference tests: PASS. Модель сохраняет server semantics `1..1 000 000`, cumulative group/item weights, дополнительные attempts, penalty, time restrictions/AM-PM weights, duplicate prevention, quest, field/instance world branch и event/fallback attempts. Monte-Carlo не используется как доказательство. UI не показывает недоказанное произведение `group × item` как точный per-kill chance. Пользовательские подписи упрощены до: «Шанс группы» → «Если группа выбрана» → «За одну основную попытку». Диалог справки объясняет эти три шага простыми словами и не показывает названия исходных игровых файлов, cumulative weights или другие внутренние термины. Ненулевые малые значения не округляются до `0,0000%`; regression fixture `Шипастая сова → Поножи со следами битв` подтверждает `0,0042% × 0,0833% = 0,0000034986%` для одной немодифицированной основной попытки (UI: `0,0000035%`, примерно 1 из 28,6 млн).

Read-only raw audit drop-таблиц из базовой версии сохраняет результат: обнаруженные cumulative overflow/missing group references остаются свойствами исходных таблиц и не нормализуются догадкой. Поле `field / instance` не трактуется как связь с конкретным dungeon/map.

### Сундуки / item_change

Добавлен детерминированный supplement `chest_contents.json.gz`, который строится из исходных `item_change.txt`. Контейнер определяется не по UI-категории, а по подтверждённым полям item projection: `kindOf=3`, `eventType=3`, `changeIndex=<свой ID>`. Это восстановило 9 ящиков/шкатулок, которые лежат в других каталожных категориях; всего сохраняется 399 профилей и 4 944 item-строки на каждый сервер. Повторная сборка supplement из raw tables побайтно совпадает с committed asset.

Для профилей с `changerate` в шкале `1..1 000 000` одинаковые пороги трактуются как группа наград; при `rate > 1` вероятность предмета считается как получение хотя бы одной его строки при выборе без повторения. Regression fixture `808094 → 101402` (`Сундук с матерчатыми доспехами из лабиринта → Шёлковая шляпа первопроходца`) даёт `15,204%` и совпадает в обе стороны UI/API. Профиль `873079` выходит за подтверждаемую шкалу, поэтому приложение fail-safe показывает его содержимое и количества, но не процент. Отдельного server implementation кода `item_change` в предоставленных reference-файлах нет; эта граница явно документирована.

Источники предмета выводятся блоками: точные монстры → мировая добыча → сундуки → квестовые источники. Точные монстры сортируются chance desc → level asc → name. Мировой источник раскрывается лениво в список подходящих монстров по подтверждённым level/type conditions; UI отдельно предупреждает, что конкретная `field/instance` принадлежность монстра не доказана.

Monster-side world-drop regression: карточка монстра теперь лениво получает мировые правила, подходящие по level/type, вместо показа только `direct`-ветки. Конкретный fixture `Враждебный дух` (ID 85, уровень 50) подтверждает наличие `Четки души` (ID 835221) через мировое правило с вариантами `0,85% ×1`, `0,10% ×3`, `0,05% ×5` и `Сундук с оружием из золотой пустыни` (ID 808100) с `0,36%` за одну основную мировую попытку. На обоих серверах item-side candidate expansion также содержит монстра ID 85. UI не выдаёт это за доказанную принадлежность конкретной карте: условие типа локации остаётся отдельной оговоркой.

Дополнительный generated-data audit для мировой ветки: все ссылки WorldRule → drop-list group разрешаются, и все item ID из этих групп существуют в основной таблице предметов на Kiss и Original. Известные missing-group references в отдельных direct-правилах остаются свойством исходных таблиц и не заполняются догадками.

## Lifecycle adversarial audit

Single-instance дополнительно защищён межпроцессным OS-level lock, который берётся после разрешения app-owned paths, но до maintenance, логов и профиля. Поэтому второй Iris Online с тем же data-root не может запуститься на другом `-addr` и стать конкурирующим writer. Same-port health probe сохраняется для удобного повторного открытия той же сборки. Lock освобождается ОС при закрытии/аварийном завершении; immediate reuse проверен.

Startup health probe не следует HTTP redirects: автоматическая проверка уже запущенной копии остаётся строго loopback и не может быть перенаправлена внешним локальным процессом на удалённый URL. `openBrowser()` также сам валидирует plain `http://` loopback target до вызова системного launcher.

Исправлен сценарий `active → heartbeat expiry → direct close`: heartbeat-expired session переносится в bounded tombstone storage, поэтому поздний explicit close того же подтверждённого ID остаётся авторитетным. Произвольный неизвестный ID shutdown вызвать не может.

В 1.0.1 дополнительно устранена гонка `pagehide ↔ in-flight /api/session/open`. Frontend заранее создаёт валидный session ID, не теряет его после heartbeat failure и отменяет незавершённый open при закрытии. Если `close` с пометкой pending-open приходит раньше самого `open`, backend на 30 секунд запоминает этот ID в отдельном bounded pre-close storage и отклоняет запоздалый `open` кодом 409. Это не даёт уже закрытой вкладке оставить новую orphan-session. Обычный неизвестный close без pending-open остаётся полностью неавторитетным.

Инварианты:

- active session → close → shutdown: PASS;
- active → heartbeat expire → backend остаётся жив: PASS;
- active → heartbeat expire → close того же ID → shutdown: PASS;
- expire → reopen same ID → close → shutdown: PASS;
- random/unknown session close → shutdown не происходит: PASS;
- две sessions, A expires, B active, A closes → B удерживает backend: PASS;
- две sessions, A expires, B closes → A tombstone удерживает backend до A close/TTL: PASS;
- tombstone TTL cleanup: PASS;
- tombstone hard limit: PASS (`256` IDs);
- pending-open close → late open rejected: PASS;
- обычный unknown close → pre-close state не создаётся: PASS;
- pre-close TTL/hard limit: PASS (`30 секунд`, `256` IDs);
- concurrent open/heartbeat/expire/close under race detector: PASS;
- 10 последовательных start/stop cycles, port release и immediate restart: PASS.
- одинаковый data-root + разные loopback ports: PASS, второй процесс отклонён до profile/log maintenance.
- health-probe redirect: PASS, redirect target не запрашивается.

Tombstone TTL: 6 часов. Это жёстко ограничивает память: забытая session после TTL удаляется; при полном отсутствии иных sessions backend затем может штатно завершиться.

## Filesystem adversarial audit

Все maintenance-delete операции переведены на общий fail-closed validator. Перед `Remove/RemoveAll` проверяются реальный application-owned root, каждый существующий parent component, final object, executable protection и resolved path. Symlink/reparse ancestor считается небезопасным; удаление пропускается.

Проверено:

- normal file inside allowed root: PASS;
- normal directory inside root: PASS;
- absolute outside path: PASS (rejected);
- `../` traversal: PASS (rejected);
- пустой path: PASS (rejected);
- root path itself: PASS (rejected);
- current executable: PASS (protected);
- directory containing current executable: PASS (protected);
- profile outside maintenance roots: PASS (protected);
- final symlink: PASS (rejected, target not followed);
- symlink parent component: PASS;
- nested symlink ancestors: PASS;
- broken symlink: PASS (rejected);
- symlinked allowed root, даже указывающий на собственный sibling/root: PASS (консервативно rejected);
- malformed pending-delete entry: PASS;
- exact exploit `allowed/link -> outside`, delete `allowed/link/victim`: PASS; `outside/victim/keep.txt` сохраняется;
- concurrent maintenance/pending-delete against symlink escape: PASS;
- pre-planted symlink at profile backup path: PASS; outside target не изменяется.

Windows-specific hardening проверяет generic `FILE_ATTRIBUTE_REPARSE_POINT`, а не только `ModeSymlink`. Windows-only regression создаёт настоящий NTFS junction через `mklink /J`, но **нативно на Windows в этой Linux-среде он не запускался**. Windows test binary с этим тестом успешно cross-compiled для amd64, 386 и arm64.

Аудит всех runtime `Remove`, `RemoveAll`, `Rename`, `CreateTemp`, `OpenFile`, `MkdirAll`, `Abs/Rel/EvalSymlinks` выполнен. Предсказуемый backup `*.tmp` заменён на `CreateTemp` в проверенном app-owned directory. Maintenance не принимает пользовательские profile/history строки как filesystem paths.
Pending-delete control file также fail-closed: symlink/reparse вместо `pending-delete.json` игнорируется и не читается как внешний файл. Rotating log writer повторно валидирует app-owned log path перед каждым reopen после ротации; symlink substitution test сохраняет внешний target неизменным.

## HTTP/security regression

PASS:

- listener только loopback, `0.0.0.0` запрещён;
- Host, Origin, `Sec-Fetch-Site` validation;
- API method restrictions;
- Content-Type/body limits;
- `DisallowUnknownFields`, trailing JSON и duplicate-key rejection;
- API concurrency limit;
- HTTP read/write/idle timeouts;
- bounded response cache, active sessions, expired-session tombstones, pre-close race guard, profile, favorites/history;
- embedded static FS и path traversal;
- malformed ID/query/page/pageSize/enum handling;
- HTML escaping/XSS regressions для game/profile strings;
- CSP, `X-Frame-Options`, `X-Content-Type-Options: nosniff`;
- external links используют безопасный new-tab rel;
- remote scripts/images/fonts отсутствуют;
- telemetry, updater, auto-download, downloaded-code execution и persistence отсутствуют.

Source scan runtime behavior:

- `os/exec` используется только для системного открытия URL приложения; URL формируется из заранее проверенного loopback address;
- единственный runtime `http.Client` проверяет уже запущенную копию по локальному `/api/health` с коротким timeout;
- `user32.dll` используется только Windows GUI startup MessageBox;
- registry writes, scheduled tasks, startup persistence, downloader/updater и сторонние payload отсутствуют.

`govulncheck` в текущей среде не установлен; проверка не запускалась. Runtime использует только стандартную библиотеку Go, сторонних Go-модулей нет.

## Resources / bounded stress / performance

RSS smoke: 1 200 mixed API requests после прогрева.

- RSS before: `79 806 464` bytes;
- RSS after: `90 370 048` bytes;
- delta: `+10 563 584` bytes.

Короткий stress-тест не выявил runaway memory growth. Это не доказательство отсутствия всех возможных утечек.

UI smoke сохраняет bounded переходы между item/set/monster pages, server switches и lazy drop open/close, проверяет stale AbortController behavior и cleanup DOM. Новых polling/`setInterval` механизмов не добавлено. Переключение `Предметы ↔ Монстры` сохраняет текущий каталог до готовности следующего ответа и выполняет один DOM commit; regression test с задержанным API запрещает промежуточный full-page loading state/исчезновение `.catalog-page`. Выпадающий поиск на главной проверяется отдельно: `.home-primary` больше не обрезает `#searchSuggestions`, а нижняя часть списка остаётся кликабельной за границей основной карточки.

UI regression 1.0.1: сохранённый `q` из profile/localStorage намеренно не восстанавливается при новом запуске; глобальный и каталожные поисковые поля начинают пустыми. На главной при наличии истории просмотра доступна кнопка «Очистить», которая очищает `recentlyViewed` одновременно в состоянии, localStorage и профиле. Квестовый источник выводит название квеста один раз; контекст остаётся отдельной строкой. Диалоги «О приложении»/«Как работает выпадение» не раскрывают названия внутренних игровых файлов.
UI smoke также проверяет двусторонний chest flow, контейнер вне каталожной категории «Сундук», отсутствие неподтверждённого процента у `873079`, отсутствие ссылки у неизвестного output ID и lazy world-source expansion. Playwright-managed Chromium в текущей среде отсутствует, поэтому smoke запускался тем же test code через доступный системный `/usr/bin/chromium`; source test file не модифицировался под окружение.

Warmed local endpoint benchmark после hardening: `/health` median 0.242 ms / p95 0.357 ms; `/search` 0.218/0.322 ms; `/items` 0.191/0.315 ms; item detail 0.227/0.316 ms; `/monsters` 0.341/0.403 ms; monster detail 0.345/0.411 ms. Hardening не добавляет преобразование игровой базы на каждый route.

## Windows build / icon / reproducibility

`.go-version`: `1.26.5`.

`build.ps1`:

- UTF-8 with BOM, совместимая синтаксическая форма `${Version}:`;
- сохраняет исходные process env `CGO_ENABLED/GOOS/GOARCH/GOAMD64/GO386` и восстанавливает их в `finally`;
- host checks не наследуют случайный cross-target;
- normal tests выполняются с `CGO_ENABLED=0`;
- перед `go test -race` проверяется C compiler (`CC` или `gcc`), затем временно ставится `CGO_ENABLED=1`;
- при отсутствии компилятора выдаётся понятное сообщение `Для go test -race требуется GCC/CGO.`;
- финальные Windows EXE всегда собираются с `CGO_ENABLED=0`;
- `IRIS_SKIP_CHECKS=1` разрешён только для diagnostic build; публикационная release-сборка fail-closed и обязана пройти проверки;
- release marker: `IrisOnlineRelease/1.0.1`;
- unsupported diagnostic marker: `IrisOnlineDiagnostic/1.0.1/<go-version>`;
- x64: `GOAMD64=v1`; x86: `GO386=softfloat`; ARM64: native arm64 target.

Windows PowerShell 5.1 parser недоступен в текущей Linux-среде; BOM/static regression и Windows cross-build проходят.

Application icon + manifest входят в source tree. Сеть и `go-winres` для обычной сборки не нужны. Архитектурные `.syso` регенерируются детерминированным stdlib Python script и в regression test совпадают побайтно с committed resources.

Diagnostic Go 1.23.2 был независимо собран дважды из этого source tree с одинаковыми flags. Byte-for-byte reproducibility: PASS.

- x64 SHA-256: `51f8cf6a30801bd2c9a4e51610e377fff598729de96650f87c4ac931ec606e66`
- x86 SHA-256: `158ede17cdaf0cebdac6d953c97bf78a626e831685bade5fbb6cb4b2803ceead`
- ARM64 SHA-256: `8a6118e41f8c389e9214b8eaf11a64b8ff098685645e018137918b783b6c5fe5`

`go version -m`/metadata verification: PASS для всех трёх, marker `IrisOnlineDiagnostic/1.0.1/go1.23.2`.

PE resource verification:

- x64 icon/resource section: PASS;
- x86 icon/resource section: PASS;
- ARM64 icon/resource section: PASS;
- `.rsrc`, group icon, 7 icon image sizes и manifest присутствуют во всех трёх;
- icon payload одинаков между x64/x86/ARM64.

## Финальные автоматические проверки

- `go test -count=1 ./...`: PASS — 147 pass events (139 top-level + 8 subtests).
- `go test -race -count=1 ./...`: PASS.
- `go vet ./...`: PASS.
- ротация логов `backups=0`, `backups=1`, `backups>1`: PASS; проверены текущий файл и цепочка `.1/.2/.3`.
- `node --check web/app.js`: PASS.
- `python3 -m unittest discover -s tools -p "test_*.py"`: PASS — 70 tests.
- Windows Node stdout UTF-8 regression: PASS — Node JSON output is decoded explicitly as UTF-8, independent of the Windows ANSI code page.
- API smoke: PASS.
- UI smoke: PASS.
- lifecycle smoke: фазы 1–15 полного сценария прошли в текущем Linux harness; отдельные 10 последовательных start/stop cycles также PASS. Совмещённый скрипт с 10 холодными перезапусками превысил лимит одного инструментального запуска, поэтому это не помечается как новый полный end-to-end PASS одним процессом.
- RSS smoke: PASS.
- security/path adversarial Go tests: PASS.
- data presentation/completeness audit: PASS, включая 399/4 944 chest projection, unknown output IDs и unsupported-chance profile.
- chest supplement deterministic rebuild from raw `item_change.txt`: PASS.
- fixture `808094 → 101402`: PASS — `15,204%` в source и contents views.
- non-category container `873063`: PASS.
- unsupported chance profile `873079`: PASS — contents visible, percentage omitted.
- deterministic drop reference tests: PASS.
- raw drop read-only audit: PASS (source anomalies сохраняются, не скрываются).
- Windows diagnostic metadata verification: PASS x64/x86/ARM64.
- Windows resource/icon verification: PASS x64/x86/ARM64.
- diagnostic rebuild reproducibility: PASS x64/x86/ARM64.

## Известные ограничения

Браузерный `localStorage` содержит низкочувствительные игровые настройки/избранное/историю/recently-viewed и временную pending-копию профиля. После успешной синхронизации pending-копия удаляется, но legacy/fallback keys остаются в browser profile и не удаляются простым удалением EXE. В проекте нет installer/uninstaller/logout flow, поэтому автоматическое удаление browser-origin storage при uninstall не проверяется.

1. Нативный Windows NTFS junction regression не запускался в Linux; Windows-only test входит в source и успешно cross-compiles для x64/x86/ARM64. Generic reparse-point detection и PE resources проверены кросс-сборкой.
2. Windows PowerShell 5.1 parser недоступен в этой среде; `build.ps1` прошёл статические regression checks, BOM-проверку и фактический Windows cross-build из Linux.
3. `govulncheck` недоступен в окружении.
4. Diagnostic EXE собраны Go 1.23.2 и не являются официальным релизом; публикационная сборка должна использовать Go 1.26.5 из `.go-version`.
5. Bounded expired-session tombstone TTL равен 6 часам для действительно замороженной вкладки. В 1.0.1 закрытие во время повторного `session/open` защищено отдельным 30-секундным bounded pre-close guard, поэтому этот race больше не создаёт новую orphan-session.
6. Точный итоговый per-kill drop chance не показывается для runtime-зависимых серверных веток. Доказанной `monster/NPC → specific dungeon/map` связи в доступных данных нет.
7. Для `item_change` отсутствует предоставленный server implementation. Вероятности показываются только для профилей, согласующихся с подтверждаемой cumulative-моделью `1..1 000 000`; аномальный `873079` намеренно выводится без процента.
