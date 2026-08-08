# Iris Online 1.0 — TEST_REPORT

Дата проверки: 2026-08-08.

Отчёт описывает текущее состояние исходного проекта Iris Online 1.0. Публикационная Windows-сборка должна выполняться Go из `.go-version` (`go1.26.5`). Проверочные EXE в этой среде собраны Go 1.23.2 только как diagnostic build.

## Игровые данные

UI/lifecycle/security-правки не меняли игровые assets. Финальные SHA-256:

- `assets/game_data.json.gz`: `7c3698494233696f2f5728ef17f7e13953159191f966d77b90742dbced23875e`
- `assets/set_effects.json.gz`: `b789ee576e4a006d1a5ddfe1addaf9e1fed334703c841f6be36282ef95839673`
- `assets/item_abilities.json.gz`: `7e5113eb7d75879614b7768750fe08d8ae9c11ca5fabe5a3708c8d0851f05351`
- `assets/item_recipes.json.gz`: `3613929c92ca01af620d75a88c5401a2cc8a35702d23169090f2ced612acdee8`
- `assets/monster_details.json.gz`: `3601607f8bd7e4919738d8ccd3bfd99212526560a46771a7b9df880d0bae25d5`

Data presentation/completeness audit: PASS.

- предметов: 13 927;
- монстров: 1 342;
- embedded sets: 467;
- строк эффектов комплектов: 972;
- реальные thresholds: `2, 3, 4, 5`;
- active effects: 160;
- комплектов с порогом 5: 126; active effects на пороге 5: 122;
- recipes: 1 207; material links: 4 129;
- fatal data-loss findings: отсутствуют.

Цвета редкости не менялись: unique `#fff600`, epic `#d800ff`, rare `#00fffc`, normal `#ffffff`, magic `#00ff00`, shop `#ffcd00`. Формат цены продажи остаётся `Цена продажи: 2,460 тер`.

Русские количественные формы в UI используют общий formatter с правилом для `11–14` и последних цифр `1 / 2–4 / 0,5–9`. Regression cases включают `1, 2, 4, 5, 11–14, 21–25, 101, 111–114, 121–125`; подписи предметов, источников, вариантов, записей и дополнительных попыток проверены.

## Drop model

`Drop.cpp` и `DropScript.cpp` использовались только как reference и не входят в распространяемый source tree:

- `Drop.cpp`: `4ce16a7721b61e55ec8a188a45588b98ec8f6f4792543c2d8995c4f349d1c390`
- `DropScript.cpp`: `5a9726db029dfe2ed13c2a66ee13f11d404f41b2d06adf399a16aceedeb37c60`

Deterministic reference tests: PASS. Модель сохраняет server semantics `1..1 000 000`, cumulative group/item weights, дополнительные attempts, penalty, time restrictions/AM-PM weights, duplicate prevention, quest, field/instance world branch и event/fallback attempts. Monte-Carlo не используется как доказательство. UI не показывает недоказанное произведение `group × item` как точный per-kill chance. Выбор теперь подписан по шагам: «Шанс выбрать группу» → «Внутри группы» → «За одну основную попытку». Ненулевые малые значения не округляются до `0,0000%`; regression fixture `Шипастая сова → Поножи со следами битв` подтверждает `0,0042% × 0,0833% = 0,0000034986%` для одной немодифицированной основной попытки (UI: `0,0000035%`, примерно 1 из 28,6 млн).

Read-only raw audit повторно выполнен для Kiss/Original. Обнаруженные cumulative overflow/missing group references остаются свойствами исходных таблиц и не нормализуются догадкой. Поле `field / instance` не трактуется как связь с конкретным dungeon/map.

## Lifecycle adversarial audit

Single-instance дополнительно защищён межпроцессным OS-level lock, который берётся после разрешения app-owned paths, но до maintenance, логов и профиля. Поэтому второй Iris Online с тем же data-root не может запуститься на другом `-addr` и стать конкурирующим writer. Same-port health probe сохраняется для удобного повторного открытия той же сборки. Lock освобождается ОС при закрытии/аварийном завершении; immediate reuse проверен.

Startup health probe не следует HTTP redirects: автоматическая проверка уже запущенной копии остаётся строго loopback и не может быть перенаправлена внешним локальным процессом на удалённый URL. `openBrowser()` также сам валидирует plain `http://` loopback target до вызова системного launcher.

Исправлен сценарий `active → heartbeat expiry → direct close`: heartbeat-expired session переносится в bounded tombstone storage, поэтому поздний explicit close того же подтверждённого ID остаётся авторитетным. Произвольный неизвестный ID shutdown вызвать не может.

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
- bounded response cache, active sessions, expired-session tombstones, profile, favorites/history;
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

- RSS before: `77 570 048` bytes;
- RSS after: `78 077 952` bytes;
- delta: `+507 904` bytes.

Короткий stress-тест не выявил runaway memory growth. Это не доказательство отсутствия всех возможных утечек.

UI smoke сохраняет bounded переходы между item/set/monster pages, server switches и lazy drop open/close, проверяет stale AbortController behavior и cleanup DOM. Новых polling/`setInterval` механизмов не добавлено. Переключение `Предметы ↔ Монстры` сохраняет текущий каталог до готовности следующего ответа и выполняет один DOM commit; regression test с задержанным API запрещает промежуточный full-page loading state/исчезновение `.catalog-page`. Выпадающий поиск на главной проверяется отдельно: `.home-primary` больше не обрезает `#searchSuggestions`, а нижняя часть списка остаётся кликабельной за границей основной карточки.

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
- release marker: `IrisOnlineRelease/1.0`;
- unsupported diagnostic marker: `IrisOnlineDiagnostic/1.0/<go-version>`;
- x64: `GOAMD64=v1`; x86: `GO386=softfloat`; ARM64: native arm64 target.

Windows PowerShell 5.1 parser недоступен в текущей Linux-среде; BOM/static regression и Windows cross-build проходят.

Application icon + manifest входят в source tree. Сеть и `go-winres` для обычной сборки не нужны. Архитектурные `.syso` регенерируются детерминированным stdlib Python script и в regression test совпадают побайтно с committed resources.

Diagnostic Go 1.23.2 был независимо собран дважды из этого source tree с одинаковыми flags. Byte-for-byte reproducibility: PASS.

- x64 SHA-256: `aabdb93ac34661c12b63d3ad43118b5cd2fe276c60fa4adf4805f856c5e20fcf`
- x86 SHA-256: `dcff03c81ad280236857d72769310a4174a67e11fa6da75b0785dec8450b1296`
- ARM64 SHA-256: `2a02e48d0cc6484d951967145e51c675a0d5c7f6130a2c3fc4b9c8b05f8aca42`

`go version -m`/metadata verification: PASS для всех трёх, marker `IrisOnlineDiagnostic/1.0/go1.23.2`.

PE resource verification:

- x64 icon/resource section: PASS;
- x86 icon/resource section: PASS;
- ARM64 icon/resource section: PASS;
- `.rsrc`, group icon, 7 icon image sizes и manifest присутствуют во всех трёх;
- icon payload одинаков между x64/x86/ARM64.

## Финальные автоматические проверки

- `go test -count=1 ./...`: PASS — 117 test case pass events после добавления regression-тестов ротации логов.
- `go test -race -count=1 ./...`: PASS — 11.316 s package time в финальном полном run.
- `go vet ./...`: PASS.
- ротация логов `backups=0`, `backups=1`, `backups>1`: PASS; проверены текущий файл и цепочка `.1/.2/.3`.
- `node --check web/app.js`: PASS.
- `python3 -m unittest discover -s tools -p "test_*.py"`: PASS — 60 tests.
- Windows Node stdout UTF-8 regression: PASS — Node JSON output is decoded explicitly as UTF-8, independent of the Windows ANSI code page.
- API smoke: PASS.
- UI smoke: PASS.
- lifecycle smoke: PASS.
- RSS smoke: PASS.
- security/path adversarial Go tests: PASS.
- data presentation/completeness audit: PASS.
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
5. Bounded expired-session tombstone TTL равен 6 часам. После этого полностью забытая frozen session удаляется; это предотвращает неограниченное удержание процесса/памяти.
6. Точный итоговый per-kill drop chance не показывается для runtime-зависимых серверных веток. Доказанной `monster/NPC → specific dungeon/map` связи в доступных данных нет.
