# План snowden.system v2 — проверенная реализация адаптивной системы

> Версия плана: 2026-08-21.
>
> Этот документ заменяет прежний план и отделяет проверенные факты от проектных
> решений. Цель — не «добавить ещё протоколов», а получить управляемую систему:
> обнаружить проблему → выбрать рабочий канал → применить → проверить → запомнить.
>
> Эталонный контур реализации на первом этапе — **Windows: Go + Wails + embedded
> sing-box-lx**. Android и iOS подключаются после стабилизации контракта канала.

---

## 0. Статус проверки

### 0.1. Что проверено

- Windows использует embedded `box.Box`, а не subprocess sing-box.
- Живой путь конфигурации: `app.LoadConfigFile` → `windows/assets/configs/`.
- `windows/backend/config/builder.go` — legacy и не участвует в рабочем пути.
- В текущей незакоммиченной ветке уже есть попытки исправить lifecycle, circuit
  breaker, metrics loop, outbound tag resolution и добавить `ChannelMemory`.
- В основной рабочей конфигурации фактически нельзя автоматически считать все
  документированные каналы существующими: runtime-конфиг и публичный `.example`
  расходятся. Реальные каналы определяются только фактическим конфигом.
- Основной проверенный транспорт — Hysteria2 VPS. Наличие рабочих VLESS/FR/AWG
  каналов требует отдельного live-теста.
- `DomainStatsRegistry.GetBest()` реализован, но не подключён к выбору маршрута.
- Windows использует mixed inbound + системный HTTP proxy, а не полноценный TUN
  для всего трафика.

### 0.2. Проверенные исправления против старых документов

1. `experimental.cache_file` sing-box хранит fake IP/DNS-кэш, но не является
   памятью выбора `urltest`.
2. `store_selected` относится к старому Clash API и Selector; на текущем
   `urltest` без Clash API рассчитывать на него нельзя.
3. В sing-box/sing-box-lx текущей ветки нет native `mieru` outbound.
4. AWG-поля присутствуют в исходнике lx, но рабочая AWG-сборка проекта не
   доказана: main module использует `sagernet/wireguard-go v0.0.3`, а runtime
   совместимость с obfuscation должна быть проверена отдельно.
5. Clash API на `127.0.0.1:9090` нельзя считать работающим: основной конфиг его
   не включает, а текущие Wails build tags не включают `with_clash_api`.
6. Cloudflare Worker был синтаксически невалиден при `node --check`; его нельзя
   считать готовым production-сенсором.
7. `go 1.25.0` валиден: Go 1.25 выпущен в августе 2025. Понижать версию из-за
   старой документации не требуется.

### 0.3. Состояние рабочей копии на момент создания плана

На 2026-08-21 в ветке `master` остаётся большой незакоммиченный функциональный
набор изменений. Он затрагивает Windows lifecycle/adaptive core, channel
manifest, Android `VpnService`/TUN/probe/status bridge, Flutter profile/runtime,
тесты и сопутствующую документацию. Эти изменения принадлежат текущей рабочей
сессии и не должны смешиваться с будущими коммитами без повторной проверки.

`AUDIT.md` и этот `PLAN.md` фиксируют состояние и очередь отдельно от кода.
При последующем коммите документации нельзя считать весь функциональный diff
принятым: для него отдельно нужны `go test -race`, `go vet`, сборки Flutter/
Kotlin и live-проверки.

### 0.4. Локально подтверждённое состояние окружения

Согласно последнему аудиту:

- `windows/frontend`: `npm ci` и `npm run build` проходят;
- Windows Go core: `go test`, `go test -race`, `go vet` и build с рабочими tags
  проходили при запуске Go по абсолютному пути;
- Android: `flutter analyze`, `flutter test` (20 тестов), Kotlin release compile,
  release APK и установка через ADB проходили;
- на PTP N49 / Android 16 подтверждены TUN, physical underlay, повторный
  start/stop и fail-closed cleanup;
- успешный внешний HTTPS через текущий HY2 не подтверждён;
- Wails CLI и интерактивный Windows GUI live-run ещё не приняты;
- `npm audit fix` автоматически не запускать: advisory требует отдельного
  решения и может изменить lockfile/toolchain.

### 0.5. Актуальный срез и очередь на 2026-08-21

Этот блок — рабочая очередь, а не новая архитектура. Подробные факты и ссылки
на файлы находятся в `AUDIT.md`; при расхождении с более старым текстом ниже
приоритет имеет свежий audit.

#### Уже сделано и подтверждено

- Windows embedded engine, Manager lifecycle, metrics worker, Adaptive
  circuit breaker и recovery через Manager собраны в единый fail-closed контур.
- ChannelMemory, classifier, DomainStats и parser/config-validator tests уже
  покрывают основные headless-сценарии.
- UI больше не выдаёт fake active server и fake traffic: отсутствующие метрики
  показываются как «нет данных».
- Android переведён на актуальный `libbox` API: lifecycle проходит через
  `startForeground` → `Libbox.setup` → command server → `startOrReloadService`.
- Android TUN реально поднимается на тестовом устройстве; внешний probe идёт
  после старта и при отказе закрывает VPN без direct fallback.
- Channel manifest и metadata-aware resolution уже добавлены в рабочее дерево;
  их ещё нужно подтвердить тестами, синхронизацией runtime-конфига и UI.

#### Открытые блокеры

1. **P0 — серверный транспорт:** DNS `kopilot.com` не указывает на ожидаемый
   VPS, TLS/SNI не принят, а HY2 UDP/QUIC handshake не подтверждён.
2. **P0 — protected HTTPS:** пока не доказан полный путь DNS → HTTPS через TUN
   на реальном профиле.
3. **P0 — failover:** нет второго validated protected channel и live-теста
   переключения без direct leak.
4. **P1 — TrafficCard:** pinned sing-box не регистрирует Clash API, поэтому
   реальные counters пока недоступны.
5. **P1 — release acceptance:** свежая Wails-сборка и интерактивный Windows
   Start/Stop/Reload ещё не проверены.

#### Ближайший порядок работ

```text
1. Повторно проверить текущий функциональный diff на чистых командах.
2. Пока ждём доступ к серверу, закрыть локальные задачи:
   manifest/config tests, validator после route toggles, фильтрацию
   protected samples в DomainStats, build-tag/TrafficSource проверку.
3. После исправления VPS принять реальный protected HTTPS на Android.
4. Добавить второй независимый канал и принять live failover.
5. Включить/проверить Clash API или реализовать честный TrafficSource.
6. Собрать и вручную принять свежий Wails desktop build.
7. Только после Windows MVP переносить стабильный channel contract на iOS.
```

#### Что целесообразно делать без серверной поддержки

- не добавлять новые протоколы «на глаз», а дописать тесты на disabled/unknown
  manifest entries, selector resolution и сохранение fail-closed policy;
- прогнать `ToggleRouteRule` через тот же validator, чтобы UI не мог обойти
  проверку конфигурации;
- ограничить `DomainStats` только samples от validated protected outbounds;
- проверить `with_clash_api` на точной embedded-сборке и оставить UI в состоянии
  «нет данных», если источник не зарегистрирован;
- довести Wails build/release checklist и документацию provisioning, не добавляя
  реальные credentials в репозиторий;
- подготовить детерминированный сценарий live acceptance, который после выдачи
  доступа к VPS сводится к повторяемому набору команд и проверок.

Сознательно отложены до закрытия этих блокеров: mieru-адаптер, новые AWG/I1
профили, zapret/byedpi, полноценный Windows TUN/service mode и iOS-перенос.

---

## 1. Зафиксированные архитектурные решения

### 1.1. Один владелец выбора канала

**AdaptiveController владеет политикой выбора.**

sing-box выполняет выбранный канал через `selector`:

```text
route.final → selector "proxy" → active outbound
                         ↑
                 AdaptiveController
```

`urltest` не используется одновременно как второй владелец выбора в рабочем
защищённом маршруте. Его можно оставить только для отдельной диагностики.

Это устраняет конфликт:

```text
urltest выбрал A
ChannelMemory предпочитает B
AdaptiveEngine reload'ит C
```

### 1.2. Fail-closed по умолчанию

```text
явно разрешённые RU/private направления → direct
защищённые направления → selector "proxy"
нет рабочего канала → block
```

`direct` не входит в защищённый fallback. Если позже понадобится fail-open,
это должна быть отдельная видимая настройка с предупреждением об утечке.

### 1.3. Конфиг — источник истины

Go не должен зашивать `grpc-nl`, `grpc-fr`, `vless-nl` и подобные имена.
Каналы строятся из фактического состава конфигурации и описываются через:

```text
ChannelDescriptor:
  id
  tag
  protocol
  server
  port
  country
  profile
  enabled
  priority
```

Если в текущем конфиге есть только Hysteria2 NL, UI должен показывать только
доступные `Auto/NL`. Нельзя показывать FR и направлять его в несуществующий tag.

### 1.4. Локальная память — собственная

`ChannelMemory` хранит историю каналов и влияет на следующий выбор. На
`cache_file` sing-box как на память выбора полагаться нельзя.

Ключ памяти не содержит private key, password, UUID или Telegram token:

```text
channel-id + profile-id + endpoint-hash
```

### 1.5. Cloudflare — дополнительный сенсор, не источник истины

Local probe и фактический traffic path имеют приоритет. Cloudflare health может
подсказать, что VPS доступен извне, но не доказывает доступность из сети
пользователя и не должен сам по себе запускать reload.

### 1.6. Мобильные клиенты — после Windows

Сначала стабилизируется модель канала, состояния, fail-closed и память на
Windows. После этого Android/iOS получают общий формат конфигурации.

---

## 2. Целевая архитектура

```text
┌──────────────────────────────────────────────────────────────────┐
│ Vue/Wails UI                                                     │
│ Start / Stop / Select / Diagnostics / Logs                       │
└──────────────────────────────┬───────────────────────────────────┘
                               │
┌──────────────────────────────▼───────────────────────────────────┐
│ Manager / LifecycleCoordinator                                   │
│ единая сериализация Start, Stop, Reload, ApplyChannel            │
└──────────────┬────────────────┬────────────────┬─────────────────┘
               │                │                │
               ▼                ▼                ▼
        Embedded Engine   AdaptiveController   Metrics/Memory
        box.Box           probes + CB           достоверные samples
               │                │
               ▼                │
        sing-box selector ◄─────┘
               │
               ├─ Hysteria2 VPS
               ├─ второй проверенный VPS/протокол
               ├─ WARP/AWG или MASQUE после live validation
               ├─ внешний mieru через localhost SOCKS (позже)
               └─ block при отсутствии protected channel
```

---

# Фаза 0. Безопасность и конфигурация — P0

## 0.1. Секреты

- Перевыпустить Telegram token, если он когда-либо был опубликован.
- Проверить UUID, Hysteria2/mieru passwords, WARP private keys и Cloudflare IDs.
- Не отдавать private credentials через публичный `/api/config`.
- Не коммитить реальные configs; для release использовать защищённый provisioning.
- Добавить secret scanning в CI.
- Не считать `.gitignore` защитой уже утёкшего секрета.

## 0.2. Config validator

Добавить проверку перед embedded `box.New` и в CI:

- JSON syntax;
- все `route`, `detour`, `selector`, `urltest` и DNS-ссылки существуют;
- нет `YOUR_*` в release-конфиге;
- есть `route.final`;
- защищённые направления не имеют скрытого direct fallback;
- build tags соответствуют используемым протоколам;
- секреты не попадают в diagnostics/logs;
- конфиг можно проверить на точном sing-box-lx build.

Отдельно исправить `template-warp-awg.json`: DNS использует `detour: "proxy"`,
но outbound с tag `proxy` отсутствует.

## 0.3. Config provisioning

Оставить поток:

```text
configs/singbox/
  → configs/sync-to-windows.sh
  → windows/assets/configs/
```

Но release должен явно проверять наличие реального runtime-конфига. Чистый
checkout не должен считаться рабочим без безопасного provisioning.

## 0.4. Документация

После исправлений обновить:

- корневой `STRUCTURE.md` — embedded вместо subprocess;
- `configs/singbox/STRUCTURE.md` — фактический состав каналов;
- `configs/templates/README.md` — не обещать VLESS, если runtime использует HY2;
- Windows documentation — не обещать Clash API/AWG/Gecko без live-приёмки.

### Приёмка Фазы 0

```text
node --check configs/cloudflare/worker.js
конфиг без placeholder проходит validator
публичный API не выдаёт private credentials
sing-box check/embedded validation проходит на release-конфиге
```

---

# Фаза 1. Lifecycle embedded engine — P0

## 1.1. Engine

Закончить единый `startLocked()`:

- общий путь Start и Reload;
- panic recovery;
- гарантированный `StateError`;
- bounded start timeout;
- корректное закрытие экземпляра после timeout;
- `Wait()` завершается после `Running` и после `Error`;
- Reload не публикует промежуточный `Stopped`;
- не удалять `cache.db` безусловно на каждом старте;
- не оставлять бесконтрольную goroutine с `instance.Start()`.

Тесты:

- битый JSON;
- panic из фабрики/старта;
- timeout;
- успешный Start + Wait;
- Reload после Error;
- Close во время Reload.

## 1.2. Manager metrics worker

Использовать один управляемый worker на весь жизненный цикл Manager:

```text
Manager.Start → start worker один раз
Manager.Reload → обновить snapshot/счётчики
Manager.Stop → cancel + WaitGroup.Wait
```

Не запускать новый worker при каждом Reload. Проверить `WaitGroup` под
параллельными Start/Reload/Stop.

## 1.3. Единый lifecycle API

AdaptiveController не вызывает `Engine.Reload()` напрямую. Он вызывает Manager:

```text
ApplyChannel(channelID)
ReloadConfig(snapshot)
ReportHealth(result)
```

Все вызовы из UI, Telegram и adaptive loop проходят через один сериализованный
контур.

## 1.4. Shutdown и proxy

При shutdown:

```text
Adaptive.Stop()
Manager.StopVPN()
Engine.Close()
clearSystemProxy()
```

При ошибке Start системный proxy не должен оставаться включённым.

### Приёмка Фазы 1

```bash
cd windows
go test -race ./backend/core/...
go vet ./...
go build -tags "with_awg,with_wireguard,with_utls,with_gvisor" ./...
```

Результат: нет зависшего `Starting`, нет утечки metrics goroutines, нет race.

---

# Фаза 2. Controller, selector и безопасная маршрутизация — P0

## 2.1. ChannelDescriptor и capabilities

Добавить получение списка каналов из фактического конфига. Для country mapping
использовать явные metadata/manifest; substring tag допустим только как
временный fallback и только при однозначном результате.

Неизвестный/отсутствующий канал должен давать понятную ошибку или быть скрыт в
UI, а не silently fallback в `auto`.

## 2.2. Рабочий selector

Защищённый маршрут должен использовать:

```text
selector "proxy"
```

В selector входят только validated protected channels. `direct` не входит.
Если selector не имеет рабочего канала, применяется `block`.

`SelectServer("auto")` означает выбор политики AdaptiveController, а не
передачу управления `urltest`.

## 2.3. Исправить frontend/backend contract

Проверить и исправить:

- `running/starting/stopped/error` против `connected/connecting`;
- отображение фактически выбранного outbound;
- динамический список стран/каналов;
- индексы RoutingCard после фильтрации служебных правил;
- применение импортированного конфига;
- hardcoded VLESS/Hysteria labels;
- отсутствие fake active server.

### Приёмка Фазы 2

```text
Auto/NL/доступные каналы работают по фактическому конфигу
несуществующий tag невозможно выбрать
protected traffic не переходит в direct незаметно
в UI отображается реальный active channel
```

---

# Фаза 3. Быстрая детекция и Circuit Breaker — P0

## 3.1. Категории

Использовать проверяемые операционные категории:

```text
NetworkDown
DNSFailure
ServerUnreachable
TLSFailure
ProtocolFailure
ProtectedChannelUnavailable
Degraded
Unknown
```

«ТСПУ», «DPI» и «DME» — гипотезы объяснения, а не доказанный диагноз.

## 3.2. Protected probe

Probe обязан идти через активный selector channel. Успех прямого fallback не
должен считаться успехом VPN.

Использовать:

- timeout 2–3 секунды;
- два последовательных подтверждения отказа;
- сброс Suspect после одного успешного probe;
- при необходимости два независимых HTTP targets;
- passive failure signals из логов только как триггер ускоренной проверки.

## 3.3. Машина состояний

```text
Closed
  └─ первый fail → Suspect
       ├─ success → Closed
       └─ второй fail → Open

Open
  └─ cooldown 10s/20s/40s/60s → HalfOpen

HalfOpen
  ├─ fail → Open + backoff
  └─ 2 success → Closed + reset backoff
```

Для SLA обнаружения менее 10 секунд closed probe нельзя оставлять только раз в
30 секунд. Использовать cadence около 5 секунд либо passive failure trigger.

Recovery должен быть асинхронным, но сериализованным: одновременно выполняется
не более одного reload/recovery operation.

### Приёмка Фазы 3

```text
один сетевой blip не роняет туннель
2 подтверждённых fail переводят канал в Open
cooldown реально соблюдается
HalfOpen → 2 success → Closed
проверка не уходит в direct
```

---

# Фаза 4. Реальный транспортный пул — P1

## 4.1. Базовая линия

Сначала принять только реально работающий Hysteria2 VPS. Документированные
placeholder VLESS/FR endpoints не считаются каналами до live-теста серверной
части.

## 4.2. Второй независимый канал

Добавить минимум один реально проверенный независимый канал:

- отдельный VPS/IP;
- или отдельный серверный протокол;
- с отдельным health result и channel ID.

Приёмка: отключение первого канала приводит к выбору второго без direct leak.

## 4.3. WARP/AWG

Порядок работ:

1. Provision WARP credentials безопасно и вне публичного KV.
2. Проверить plain WireGuard endpoint как базовую связность.
3. Проверить AWG на фактической Windows-сборке, включая совместимый runtime.
4. Добавить один рабочий AWG profile.
5. После этого добавить I1-A/I1-B/I1-C.
6. Каждый профиль проверить handshake и реальным HTTP/HTTPS трафиком.
7. В памяти хранить ID профиля, не private key.

Нельзя считать AWG готовым только потому, что поля `jc/i1` есть в JSON.

WARP является независимой точкой выхода, но не гарантирует страну/colo и не
заменяет отдельные geo endpoints.

## 4.4. I1-ротация

Сначала должен работать один профиль. Затем:

```text
I1-A → I1-B → I1-C
```

Junk-параметры менять последними. Не использовать неподтверждённые blobs,
которые не приняты конкретным AWG runtime/server.

## 4.5. Mieru — поздний внешний адаптер

Native outbound в sing-box не добавлять. Если mieru потребуется:

```text
supervised mieru/mita process
  → localhost SOCKS
  → sing-box socks outbound
  → selector
```

Запуск Karing оставить пользовательской альтернативой, но не считать его
частью автоматического failover.

### Приёмка Фазы 4

```text
минимум два validated protected channels
каждый имеет отдельный descriptor и health state
selector выбирает только validated channels
direct не входит в protected pool
```

---

# Фаза 5. ChannelMemory, метрики и domain intelligence — P1

## 5.1. ChannelMemory

Record должен получать фактически выбранный канал, а не просто первый outbound.

Хранить:

```text
success/failure
consecutive fail/ok
EWMA latency
last outcome
cooldown
profile/context id
```

Требования:

- atomic save через temp + rename;
- schema version;
- debounce записи;
- cap/LRU;
- обработка повреждённого файла;
- prune несуществующих каналов;
- отсутствие секретов в key;
- score влияет на следующий выбор;
- unknown channel получает ограниченное exploration preference.

Текущий `ChannelMemory.Best()` без интеграции в controller не считается
реализованной адаптивностью.

## 5.2. Метрики

Не полагаться на необъявленный `127.0.0.1:9090`.

Сделать `TrafficSource` интерфейс и отдельно проверить на pinned build:

- включение `experimental.clash_api`;
- build tag `with_clash_api`;
- authentication;
- `/connections`/traffic semantics;
- работу на Windows embedded box.

Если источник недоступен, UI показывает `нет данных`, а не ложные `0 B/s`.

## 5.3. DomainStats

Sniff log доказывает только обнаружение домена. Он не доказывает успех,
latency или фактический outbound.

Сначала подключить реальные detour/connection samples. До этого `DomainStats`
остаётся диагностическим журналом и не меняет маршрутизацию.

`GetBest(domain)` подключать только после появления достоверных samples и
механизма применения выбора без reload на каждый запрос.

### Приёмка Фазы 5

```text
падший канал не выбирается первым после рестарта
memory file переживает restart
Diagnostics показывает active/best channel и memory summary
TrafficCard не врёт нулевыми значениями
DomainStats не называет sniff событие успешным запросом
```

---

# Фаза 6. Cloudflare, динамические конфиги и telemetry — P1/P2

## 6.1. Worker

Исправить:

- синтаксис `worker.js`;
- чтение `env.VPS_IP`/секретов через bindings;
- передачу `request` в health-check;
- отсутствие ложного обещания multi-edge checks;
- фактический PUT/auth flow только если он нужен;
- rate limit и schema validation;
- разделение публичных metadata и приватных credentials;
- ограничение CORS;
- telemetry abuse protection.

## 6.2. Windows integration

`FetchConfig()` должен реально вызываться в startup/update flow, но только после
валидации и безопасной схемы. Remote config не должен silently заменять
последний рабочий конфиг.

## 6.3. Health и telemetry

Remote health — дополнительный signal:

```text
local probe fail + remote VPS alive → вероятен локальный barrier
local probe fail + remote VPS dead → вероятен server failure
```

Это не абсолютное доказательство и не единственная причина reload.

Telemetry — opt-in, без credentials/UUID/IP, с ограничением частоты.

## 6.4. Version/release

Сделать один источник версии и rollback last-known-good config. Сейчас версии
landing/Worker/R2 расходятся.

---

# Фаза 7. Android и iOS — P2

После стабилизации Windows-контракта:

1. Зафиксировать общий `ChannelDescriptor`/config schema.
2. Подтвердить Android-смартфонный тестовый контур через ADB.
3. Выбрать одну iOS-реализацию; вторую заморозить или удалить.
4. Проверить актуальность `libbox.aar` и PlatformInterface.
5. Перенести selector/fail-closed/diagnostics, не копируя Windows-specific code.
6. Добавить mobile-specific lifecycle: foreground service, Network Extension,
   permissions, sleep/resume, battery.

Android/iOS не входят в критерий Windows MVP.

---

## 3. Что сознательно не входит в MVP

- zapret/byedpi/WinDivert;
- WDTT/VK media relay;
- собственный WARP relay;
- белые списки операторов;
- llimonix как runtime dependency;
- гарантированный geo через WARP;
- Hysteria2 Gecko в sing-box-lx без отдельной проверки;
- полноценный Windows TUN/service mode;
- автоматический Karing/mieru failover.

`nip.io + Let's Encrypt` остаётся полезным способом получения настоящего TLS
certificate для серверной инфраструктуры, но не является отдельным adaptive
controller и не должен блокировать lifecycle MVP.

---

# 4. Проверки и команды

## Локальная Windows-проверка

```bash
node --check configs/cloudflare/worker.js

cd windows/frontend
npm ci
npm run build

cd ..
go test -race ./backend/core/...
go vet ./...
go build -tags "with_awg,with_wireguard,with_utls,with_gvisor" ./...

# Полная Wails-сборка
wails build -skipbindings -s -tags "with_awg,with_wireguard,with_utls,with_gvisor"
```

## Конфигурация

```bash
bash configs/sync-to-windows.sh
# затем validator и sing-box check на точной pinned-сборке
```

## Live acceptance

1. Start с рабочим config.
2. Проверить RU/private direct policy.
3. Проверить protected HTTP/HTTPS через active channel.
4. Зафиксировать active channel в diagnostics.
5. Имитировать отказ первого канала.
6. Убедиться в переключении на второй validated channel.
7. Убедиться, что direct не используется незаметно.
8. Выполнить несколько Reload подряд.
9. Проверить отсутствие роста goroutines.
10. Перезапустить приложение и проверить memory-based ordering.
11. Проверить ADB Android только после Windows acceptance.

---

# 5. Финальный критерий Windows MVP

Система считается готовой, когда:

1. release-конфиг валиден и не содержит placeholder;
2. Start/Reload/Stop не оставляют зависших box/goroutines;
3. `Wait()` корректен после success и error;
4. `go test -race` и `go vet` проходят;
5. AdaptiveController — единственный владелец выбора;
6. protected traffic использует selector и fail-closed;
7. обрыв обнаруживается в согласованный SLA;
8. Circuit Breaker реально проходит cooldown/HalfOpen/Closed;
9. есть минимум два реально проверенных protected channels;
10. active channel точно виден в UI/Diagnostics;
11. ChannelMemory меняет следующий выбор и переживает restart;
12. TrafficCard не показывает выдуманные нулевые метрики;
13. Cloudflare Worker синтаксически и функционально проверен;
14. публичные endpoints не выдают private credentials;
15. frontend/Go/Wails release build зелёные.

> «Работает при любых блокировках» не является технически гарантируемым
> утверждением. IP/ASN block требует нового endpoint, whitelist-режим требует
> отдельной техники, geo-block требует отдельного выхода, а конкретный DPI
> требует live-профилей. Этот план делает эти классы отказов наблюдаемыми и
> управляемыми, а не обещает невозможную универсальность.
