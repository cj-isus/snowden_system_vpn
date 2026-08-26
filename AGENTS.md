# AGENTS.md — правила разработки для ИИ-ассистента

Этот файл — короткая память проекта. Его нужно прочитать перед изменением
кода, конфигурации или документации. Правила ниже важнее старых планов и
исторических описаний, но всегда уступают фактическому коду, тестам и
результатам live-проверки.

## 1. Базовый порядок работы

1. Сначала прочитай `README.md`, `STRUCTURE.md`, `PLAN.md`, `AUDIT.md` и
   `DEVELOPMENT_REQUIREMENTS.md`.
2. Перед работой в подпапке прочитай ближайший `STRUCTURE.md` и README этой
   подсистемы: особенно `windows/backend/core`, `windows/backend/config`,
   `windows/frontend/src`, `android` и `configs`.
3. Найди реальную точку входа и текущий data flow. Не делай вывод о поведении
   по имени файла, старому TODO или историческому документу.
4. Перед правками проверь `git status --short --branch`. Рабочее дерево может
   содержать изменения пользователя или другой сессии.
5. Не удаляй, не откатывай, не перезаписывай и не форматируй чужие изменения.
   Меняй только файлы и строки, необходимые для текущей задачи.
6. После изменения проверь diff, запусти подходящие тесты и обнови документацию,
   если изменился контракт или фактический статус функции.
7. Не делай commit, push, reset, rebase, deploy или публикацию без явной просьбы.

Если документация и код расходятся, код является текущим фактом. Если код и
live-проверка расходятся, явно укажи ограничение и не называй функцию готовой.

## 2. Главные принципы

### 2.1. Доказательства важнее обещаний

Разделяй три состояния:

- **реализовано** — есть код и локальная проверка;
- **подтверждено live** — есть проверка на реальном устройстве, сервере или
  production-like окружении;
- **запланировано** — есть только план, шаблон или интерфейс.

Не превращай placeholder-конфиг, успешную компиляцию, поднятый TUN, открытый
TCP-порт или ответ Cloudflare в доказательство работающего защищённого канала.
Для VPN-пути нужны реальные protected DNS/HTTPS проверки и, для failover,
реальный второй канал.

Не обещай «работает при любых блокировках». Формулируй конкретный класс отказа,
профиль, сеть, устройство и шаг проверки.

### 2.2. Fail-closed — обязательное поведение

Для защищённого трафика действует граф:

```text
явно разрешённые private/RU направления -> direct
защищённые направления                  -> selector "proxy"
selector "proxy"                        -> только validated protected channels
нет рабочего protected channel          -> block или отсутствие защищённого пути
```

`direct` нельзя добавлять в protected selector, fallback или recovery. Успешный
прямой запрос нельзя считать успешным VPN-probe. Если безопасный путь не
подтверждён, показывай ошибку/недоступность и закрывай ресурсы, а не скрывай
проблему прямым соединением.

`urltest` не должен становиться вторым владельцем выбора канала. Политикой
выбора управляет `AdaptiveController`; применение проходит через `Manager`.

### 2.3. Источник истины должен быть один

`PLAN.md` — главный источник правды проекта для требований, решений, текущих
статусов и дневника разработки. После сжатия контекста или новой сессии его
нужно прочитать до продолжения работы. Важные факты, трудности, блокеры,
решения и результаты проверок заносятся в дневник `PLAN.md`; справочные audit- и
context-файлы не могут переопределять его.

До написания нового кода сначала изучаются зрелые аналоги, их исходники,
лицензии и реальные ограничения. Затем в `PLAN.md` фиксируется решение:
адаптировать паттерн, использовать зависимость, сделать улучшение или отказаться.
Не изобретай заново уже доказанное решение без зафиксированной причины.

Новое устойчивое правило сначала записывается в `PLAN.md`, а при необходимости
синхронизируется здесь и в специализированной документации. Это обязательно для:

- нового поведения продукта;
- новой границы безопасности или правила секретов;
- архитектурного контракта UI/backend/native/config;
- формата данных, lifecycle state, API или источника метрик;
- обязательного теста, live-check или release criterion;
- запрета реализации, fallback, зависимости или инструмента;
- уточнения, предотвращающего повторение дефекта;
- решений по performance, ownership, cancellation, timeout и cleanup.

Каждое такое правило получает в `PLAN.md` ID, дату, приоритет, формулировку
«обязательно/запрещено», владельца, причину, способ проверки и статус. Обсуждение
в чате без записи в `PLAN.md` не создаёт обязательного контракта. Если правило
меняет существующее поведение, сначала обновляются план и тест-критерий, затем
код.

Код должен быть компонентным, с явным ownership и lifecycle, но без дробления
ради дробления. Производительность подтверждается профилированием или
benchmark, а не интуицией; безопасность и fail-closed имеют приоритет над
микрооптимизациями.

- Исходные sing-box-конфиги: `configs/singbox/`.
- Рабочая копия Windows: `windows/assets/configs/`.
- Синхронизация: `bash configs/sync-to-windows.sh`.
- Публичные метаданные каналов: `configs/singbox/channel-manifest.json`.

Редактируй исходник и осознанно выполняй синхронизацию. Не исправляй вручную
stale bundled copy в `windows/assets/configs/` и не зашивай в Go список стран,
серверов или outbound-тегов.

Манифест описывает только существующие публичные metadata. Он не создаёт
outbound. В UI и selector попадают только точные теги, реально присутствующие
в runtime-конфиге, прошедшие validation и включённые в манифесте/политике.

### 2.4. Секреты не являются обычными данными

Нельзя коммитить, печатать в логи, diagnostics, UI, тестовые snapshots, чат или
публичный API:

- UUID, пароли HY2/VLESS/mieru;
- private keys и Reality keys;
- Telegram tokens, Cloudflare tokens/IDs и SSH credentials;
- реальные IP/домены, если они являются частью приватного provisioning;
- локальные `configs/env/.env`, `android/config.local.json` и реальные
  sing-box-конфиги.

`.gitignore` не исправляет уже раскрытый секрет: если credential был опубликован,
его нужно считать скомпрометированным и перевыпустить. Публичный Cloudflare API
может отдавать metadata, но не credentials. Ключи памяти каналов должны быть
секрет-free: идентификатор канала, профиль и хэш endpoint, без самого секрета.

Не добавляй debug-print секретов даже временно. Для диагностики маскируй
значения или сообщай только тип ошибки и безопасные metadata.

## 3. Архитектура, которую нельзя ломать

### 3.1. Windows: один владелец lifecycle

Рабочий путь Windows:

```text
Vue/Wails UI
  -> main.App facade
  -> core.Manager
  -> core.Engine
  -> embedded sing-box-lx box.Box
```

В production не предполагай наличие отдельного `sing-box.exe` subprocess.
`Manager` владеет сериализацией `Start`, `Stop`, `Reload`, `ApplyChannel` и
immutable config snapshots. Правильное направление зависимостей:

```text
UI / Telegram / AdaptiveEngine -> Manager -> Engine -> embedded box.Box
```

`AdaptiveEngine` не вызывает `Engine.Reload()` напрямую в production. Recovery
идёт через `Manager.ReloadVPN`, чтобы одновременно обновлялись engine, active
config, metrics и adaptive snapshot.

Базовый lifecycle:

```text
StartVPN
  -> load config
  -> NormalizeProtectedRoute
  -> Validate(RequireFailClosed: true)
  -> Engine.Start
  -> set system proxy
  -> Adaptive.Start

ReloadVPN
  -> normalize + validate
  -> serialized Engine.Reload
  -> update snapshots

StopVPN
  -> Adaptive.Stop
  -> stop metrics worker
  -> Engine.Close
  -> clear system proxy
```

При ошибке Start или Stop системный proxy не должен оставаться включённым.
Start/Close/Reload должны иметь bounded timeout, panic/error boundary и
предсказуемое состояние; нельзя оставлять бесконтрольные goroutines.
Metrics worker запускается один раз на lifecycle Manager, а не заново на каждый
Reload. Lock order и cancellation должны сохраняться при concurrent Start,
Reload и Stop.

### 3.2. Конфиг: normalize, validate, затем runtime

До создания embedded box нужно проверить:

- корректный JSON без trailing data;
- placeholders (`YOUR_*`, `CHANGE_ME`, `REPLACE_WITH`) в runtime запрещены;
- все outbound/inbound/route/DNS/detour references существуют;
- есть `route.final`;
- protected selector `proxy` содержит только validated protected tags;
- default selector входит в candidates;
- `direct` отсутствует в protected selector;
- build tags и фактические protocol registrations совместимы.

Legacy `urltest`/`route.final: auto` можно нормализовать в рабочий selector
`proxy`, но не нужно делать вид, что old input сохранён без изменений.
После изменения route rules повторяй normalization и validation. Служебные
`sniff` и `hijack-dns` не являются обычными пользовательскими правилами и не
должны переключаться по ошибочному индексу из UI.

### 3.3. AdaptiveController и memory

Контроллер проверяет именно protected path через активный selector или
локальный proxy. Категория ошибки должна описывать наблюдаемый факт
(`DNSFailure`, `TLSFailure`, `ServerUnreachable`, `ProtocolFailure`,
`ProtectedChannelUnavailable`, `Degraded`, `Unknown`), а не выдавать гипотезу
«это точно DPI» за доказанный диагноз.

Ожидаемая логика circuit breaker:

```text
Closed  -- 2 подтверждённых fail --> Open
Open    -- cooldown 10/20/40/60 s --> HalfOpen
HalfOpen -- fail -------------------> Open
HalfOpen -- 2 success --------------> Closed
```

Recovery сериализуется; одновременно выполняется не больше одной операции
reload/recovery. Один краткий сетевой сбой не должен немедленно ронять канал,
но отсутствие подтверждённого protected пути не должно замалчиваться.

`ChannelMemory` — только preference/history signal, не доказательство текущей
доступности. Он должен:

- сохраняться атомарно через temp + rename;
- иметь версию схемы, debounce, cap/LRU и обработку повреждения;
- удалять несуществующие каналы;
- не содержать секреты;
- учитывать фактически выбранный канал, ошибки, успех, latency и cooldown;
- влиять на следующий выбор только среди validated protected candidates.

### 3.4. Метрики не выдумывать

Если `TrafficSource` или Clash API недоступен, UI показывает `нет данных`/
`unavailable`, а не `0 B/s`, который выглядит как измерение. Build tag сам по
себе не доказывает, что controller зарегистрирован в pinned sing-box build.
Общий сетевой счётчик интерфейса нельзя выдавать за трафик VPN без доказательства
границы измерения.

`DomainStats` принимает только достоверные samples с exact enabled protected
outbound. Sniff домена сам по себе не доказывает успешный запрос, latency или
фактический outbound и не должен автоматически менять routing policy.

### 3.5. Android: native lifecycle важнее UI

Границы Android:

```text
Flutter UI
  -> MethodChannel / typed runtime facade
  -> MainActivity
  -> foreground SnowdenVpnService
  -> libbox CommandServer
  -> startOrReloadService
  -> SnowdenPlatformInterface.openTun
  -> Android VpnService.Builder / TUN
```

`startForeground()` вызывается до долгой native работы. `running` публикуется
только после успешного `Builder.establish()` и подтверждённого TUN. Native
sockets защищаются через `VpnService.protect(fd)` и не должны попадать обратно в
TUN. `ParcelFileDescriptor` хранится до остановки сервиса.

Start/probe/stop должны иметь единый cleanup path: закрыть TUN, command server,
foreground service и другие native resources; не затирать `error` состоянием
`stopped`; повторный start/stop не должен оставлять сервис или native crash.

Protected DNS/HTTPS probe запускается отдельным bounded компонентом после TUN.
Не возвращай в startup путь опасный `newStandaloneCommandClient().urlTestOutbound()`
для pinned AAR: текущая документация фиксирует, что такой вызов внутри startup
transaction приводил к native SIGSEGV. Debug DNS bootstrap с direct допустим
только для изоляции причины и не является production acceptance.

Flutter отвечает за presentation и typed bridge. Не добавляй UI-переключатель,
credentials editor, protocol selector, fake traffic или adaptive feature без
реального native/backend контракта и тестируемого источника данных.

### 3.6. Frontend: UI показывает факты

Активный desktop composition root — `windows/frontend/src/App.vue`. Legacy
components могут оставаться для справки, но не являются автоматически активным
экраном. Wails bindings в `windows/frontend/wailsjs/` генерируются и вручную не
редактируются.

UI должен показывать:

- реальные lifecycle states (`starting`, `running`, `stopping`, `stopped`,
  `error`), а не подменять их старым `connected/connecting`;
- только фактические каналы активного конфига;
- active channel, diagnostics и logs из backend events;
- явное отсутствие данных при отсутствии источника.

Не показывай fake first server, fake country, fake zero traffic или control,
которому не соответствует backend/native реализация.

## 4. Правила изменения по подсистемам

### Go / Windows

- Модуль лежит в `windows/`; сохраняй текущие imports и package boundaries.
- Соблюдай `gofmt`; не переноси файлы между `backend/*` без необходимости.
- Для lifecycle/config/selector изменений добавляй или обновляй unit tests.
- Проверяй обычный build с теми же tags, которые использует проект/Wails:
  `with_awg,with_wireguard,with_utls,with_gvisor`.
- Для race-sensitive кода запускай `go test -race ./...`, если доступен C
  compiler; если окружение не позволяет, сообщи об этом, не называй race-test
  пройденным.

### Vue / TypeScript

- Активный код — `windows/frontend/src/`; следуй существующим composable и
  binding-паттернам.
- Сначала добавь/измени backend contract, затем UI; не маскируй отсутствие API
  локальным mock в production path.
- После изменений запускай `npm run build` в `windows/frontend/`; этот скрипт
  включает `vue-tsc --noEmit` и Vite build.
- Не редактируй `frontend/wailsjs/` вручную.

### Android / Flutter / Kotlin

- Сохраняй разделение: Dart profile/runtime facade, Activity permission/bridge,
  Service lifecycle, PlatformInterface TUN, Probe connectivity.
- Изменение `libbox` API сначала сверяй с pinned AAR и существующим SDK
  контрактом; не переносы старых legacy-полей вроде `inet4_address`,
  `inet6_address` или legacy `inbound.sniff` без проверки.
- Для profile builder используй pure tests; credentials передавай только через
  ignored local provisioning (`--dart-define-from-file=config.local.json`).
- После Dart-изменений запускай `flutter analyze` и `flutter test`; после
  native/build.gradle изменений дополнительно компилируй Kotlin/Android.
- TUN smoke и protected HTTPS acceptance на реальном ADB-устройстве считаются
  отдельными от unit tests.

### Конфигурации и Cloudflare

- Публичные `.example` и `templates/` должны оставаться безопасными.
- Перед release не допускай placeholders в фактическом runtime-конфиге.
- Cloudflare Worker — дополнительный edge sensor/metadata layer, не владелец
  локального выбора канала и не доказательство доступа из сети пользователя.
- Для Worker минимум: `node --check configs/cloudflare/worker.js`; отдельная
  live-проверка нужна для bindings, KV/D1, CORS/auth и health semantics.
- Не запускай deploy, `wrangler deploy`, SSH или команды на VPS без отдельного
  явного разрешения и понимания последствий.

## 5. Проверки по риску

Для узкой документационной правки достаточно проверить diff и ссылки. Для
кода используй минимальный релевантный набор, расширяя его по blast radius:

```bash
# Windows backend
cd windows
go test ./...
go vet ./...
go build -tags "with_awg,with_wireguard,with_utls,with_gvisor" ./...

# Windows frontend
cd frontend
npm ci
npm run build

# Cloudflare
node --check ../../configs/cloudflare/worker.js

# Android
cd ../../android
flutter analyze
flutter test
flutter build apk --release --dart-define-from-file=config.local.json
```

Не выполняй команды, для которых нет локальных prerequisites, «на удачу» с
реальными credentials. Не запускай production scripts и deploy scripts без
явного разрешения пользователя. Не принимай `npm audit fix` или массовое
обновление dependencies как безрисковую проверку: оно меняет lockfile и
инструментарий.

## 6. Acceptance checklist перед заявлением «готово»

```text
[ ] git diff не содержит чужих или случайно сгенерированных изменений
[ ] runtime config валиден и без placeholders
[ ] protected selector не содержит direct и несуществующих тегов
[ ] Start/Reload/Stop cleanup проверен
[ ] relevant unit tests, typecheck/build и vet пройдены
[ ] unavailable data явно показана как unavailable
[ ] secrets отсутствуют в diff, логах и diagnostics
[ ] platform-specific live blockers отдельно указаны
[ ] документация синхронизирована с фактическим поведением
```

Для Windows MVP дополнительно нужны два реально проверенных protected
channels, protected HTTP/HTTPS, adaptive failover и ручной Wails Start/Stop.
Для Android поднятый TUN и живой foreground service недостаточны без
protected DNS/HTTPS probe; текущий серверный путь должен быть принят отдельно.

## 7. Карта источников

- Общий обзор: `README.md`
- Архитектура: `STRUCTURE.md`
- План и границы MVP: `PLAN.md`
- Проверенные факты и блокеры: `AUDIT.md`
- Windows: `windows/docs/DOCUMENTATION.md`
- Android: `android/ANDROID_SETUP.md` и `windows/docs/ANDROID_DOCUMENTATION.md`
- Конфигурации: `configs/README.md` и `configs/*/STRUCTURE.md`
- Точки входа Windows: `windows/main.go`, `windows/app.go`,
  `windows/backend/core/`
- Требования к разработке: `DEVELOPMENT_REQUIREMENTS.md`

Если задача требует решения, которого нет в этих контрактах, сначала сформулируй
новый backend/native/config contract и критерий проверки. Не добавляй красивую
заглушку в UI и не объявляй незапринятый путь production-ready.
