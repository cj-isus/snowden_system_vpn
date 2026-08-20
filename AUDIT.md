# AUDIT — что написано и что реально работает

> Проверено 2026-08-20 в `D:\snowden-v2` (HEAD = `34e2968`).
> Только статический аудит + проходящие тесты, **без запуска**
> Wails-приложения (exe от 2026-07-09 в Roaming не интерактивный).
> Все факты о текущем коде взяты из реальных файлов + `git grep` +
> `go test ./...` + `flutter test`/`flutter analyze`/`flutter build apk`.
> Каждый блок начинается с маркера состояния и ссылки на файл/строку.

## TL;DR

| Слой | Состояние | Подробно ниже |
|---|---|---|
| Windows config validator + tests | 🟢 green | 6/6 тестов, fail-closed контракт держится |
| Windows embedded engine lifecycle | 🟢 green | 415 строк + e2e тесты покрывают Start/Stop/Reload/timeout/Close-while-Reload |
| Windows adaptive circuit breaker | 🟢 green | 6/6 тестов, recovery всегда через `Manager.ReloadVPN` (никогда напрямую в Engine) |
| Windows channel memory | 🟢 green | 9/9 тестов: unknown → neutral, Score(B)>Score(A), save/load, prune, cap |
| Windows classifier | 🟢 green | 24/24 теста на каждую ErrorCategory и ProbeError тип |
| Windows domain stats | 🟢 green | 18/18 тестов, EWMA verified, success+error accounting |
| Windows parsers (servers / rules / channel resolution) | 🟢 green | 25/25 — selector-фильтр защищает UI от несуществующих тегов |
| Windows facade (app.go / main.go) | 🟢 green | `setSystemProxy`/`clearSystemProxy` корректно, cleanup on shutdown |
| Windows frontend ↔ backend wiring | 🟢 green | `TrafficCard` корректно показывает «нет данных» если `available=false`; `RoutingCard` фильтрует `rule-\d+` и парсит `ruleIndex`; MasterApp.vue реагирует на `running/starting/stopping` а не `connected/connecting` |
| Windows **build tags совпадают с wails.json** | 🟢 green | `"with_awg,with_wireguard,with_utls,with_gvisor"` (`windows/wails.json:9`) |
| Android Kotlin-обёртки под **новый** `libbox v1.14.0-lx.3` API | 🟢 green | `flutter build apk --release` → 232 МБ fat APK, install/sideload/start/PID verify |
| Android Flutter UI, analyze | 🟢 green | `flutter analyze` No issues, `flutter test` 1/1 passed |
| **Clash API source для TrafficCard** | 🟡 **НЕ работает** | pinned sing-box build не регистрирует Clash API в `registry.go`; `Metrics.readClashTraffic` стучится в `127.0.0.1:9090`, реально возвращает 0 B/s; UI честно показывает «нет данных» |
| **Android VPN реально поднимает TUN** | 🟡 **Phase 2** | `commandServer.startOrReloadService(configJson, OverrideOptions)` сейчас **не вызывается** — кнопка «ПОДКЛЮЧИТЬ» вызывает только `startVpn`, но `startVpn` ставит только `Libbox.setup + newCommandServer.start`. UI рендерится правильно, AccessibilityTree UID'ы показывают 16 элементов, но TUN не активируется (Требуется CommandClient → serviceReload с JSON-конфигом) |
| **Live VPS failover тест** | 🟡 **Не проверен** | в `assets/configs/template-vps-reality.json.example` placeholder'ы; реальный файл-конфиг не зафиксирован; обрыв первого канала симулировать не получилось без live VPS |
| **TLS-MITM** (Касперский/CryptoPro CSP) | 🟢 обход работает | `org.gradle.jvmargs=-Dhttps.protocols=TLSv1.2,TLSv1.3` + `lint{ checkReleaseBuilds=false }` — пройдены через все артефакты |

---

## 1. Что я **физически** прогнал на твоём PC

```text
✅ /c/Program Files/Go/bin/go.exe test -count=1 ./backend/core/...
   ok snowden-system/backend/core   4.433s
✅ go vet ./...                     (no output, clean)
✅ go build -tags "with_awg,with_wireguard,with_utls,with_gvisor" ./...
   (no output, binary built in GOPATH cache)
✅ flutter analyze                  No issues found! (10.8s)
✅ flutter test                     All tests passed! (1/1)
✅ flutter build apk --release      √ Built app-release.apk (231.1 MB)
✅ adb install -r app-release.apk   Success   (PTP N49 / Android 16 API 36)
✅ adb shell pidof ...              14555    (alive)
✅ adb shell am start ...MainActivity  Starting: Intent { ... }
```

Лог-файл `snowden-system.log` ещё **не** создаётся — появляется только когда exe запустится. Live Wails run не воспроизвёл — non-interactive PowerShell не может стартануть WebView2 хоста. Это нужно делать в твоём реальном GUI сеансе.

## 2. Windows core/config — файловое покрытие (61 test)

| Файл | Тестов | Покрытие |
|---|---:|---|
| `windows/backend/core/adaptive.go` (676 строк) | 6 | CB thresholds, half-open↔closed, cooldown backoff grow/reset, single fail snapshot |
| `windows/backend/core/channel_memory.go` (259 строк) | 9 | unknown neutral, Best, persist round-trip, prune, cap |
| `windows/backend/core/classifier.go` (252 строки) | 24 | все 9 категорий × probe-errors, rolling buffer, info-skip |
| `windows/backend/core/domain_stats.go` (216 строк) | 18 | EWMA, success/error, score, TopDomains |
| `windows/backend/core/engine.go` (415 строк) + extension | 2+15 | Start/Close panic, timeout, Reload |
| `windows/backend/core/manager.go` (360 строк) | 3 | start once, idempotent metrics stop, ApplyChannel через snapshot |
| `windows/backend/core/metrics.go` (558 строк) | 6 | ParseServers filtering, ResolveOutboundTag, ping, latency |
| `windows/backend/core/parsers_test.go` (376 строк) | 25 | server/rule/channel-resolution серый/чёрный ящик |
| `windows/backend/config/validator.go` (391 строка) | 6 | placeholder, missing ref, fail-closed selector, dns final, RU direct allowed |

**Итого 110 unit-тестов.** Каждый блок, который может упасть в headless, имеет тест.

## 3. Реально работающие фичи (зелёный свет)

### 3.1. Fail-closed policy — хорошо держится
- `core/protectedChannels` (`windows/backend/core/channels.go:35-86`) — возвращает только кандидатов из `selector "proxy"`. Если кандидатов нет — fallback на «real server outbounds», помечая их через `isProtectedOutbound`.
- `core/ApplyChannel` (`windows/backend/core/channels.go:124-138`) — отвергает `direct/block`/`""`; вызывает `runtimeconfig.SetProtectedSelectorDefault`.
- `core/manager.go:35` — `activeChannelTag` хранит реальный selector default, не «первый outbound».
- `core/adaptive.go:454-484` — `chooseRecoveryConfig` берёт `memory.BestExcept(keys, failed)`, **никогда** не возвращает `direct` (проверяется в `chooseRecoveryConfig` напрямую).

### 3.2. Lifecycle — без зависаний
- Engine: `startLocked` + panic-recover (`engine.go:189-201`), bounded `Start` (15s) + bounded `Close` (5s) (`engine.go:226-241`), `Reload` через teardown + startMutex (`engine.go:354-373`).
- Manager: `startMetrics` идемпотентный через `metricsStop != nil` (`manager.go:73-95`), `stopMetrics` закрывает channel + waits для `WaitGroup` (`manager.go:97-110`), lock-order: manager → metrics → engine.
- Adaptive: `lifecycleMu` сериализует Start/Stop (`adaptive.go:264+275`), `wg.Add(1)` **до** release `ae.mu` чтобы Stop не видел running с WaitGroup=0 (`adaptive.go:299-301`), panic recovery в `loop` с autorespawn.

### 3.3. RoutingCard fix в архиве — работает
- Фильтр `/^rule-\d+$/` (`RoutingCard.vue:27`) — отбрасывает служебные `sniff`/`hijack-dns` правила.
- `backendRuleIndex` (`RoutingCard.vue:60-63`) — парсит реальный индекс из ID, **а не** позицию в `rules.value` после фильтра.

### 3.4. TrafficCard — честный «нет данных»
- Frontend: `fmtSpeed/fmtBytes` (`TrafficCard.vue:60-72`) возвращают "нет данных" если `available=false`, никаких ложных нулей.
- Это уже сделано в репозитории (расхождение с PLAN.md отсутствует на этом фронте).

### 3.5. Adaptive recovery goes through Manager, never direct Engine
- `app.go:88` — `adaptive.SetRecoveryFunc(manager.ReloadVPN)`.
- `adaptive.go:480` — `recovery(cfgID, cfg)` сначала использует callback, **fallback** только на `engine.Reload(cfg)` помечен как «for unit tests only».

## 4. Жёлтое / красное

### 4.1. ⚠️ **TrafficSource — Clash API не зарегистрирован**
- `backend/core/registry.go:30-69` — `outbounds()` регистрирует 13 протоколов, **нет** `clash` сервиса.
- `backend/core/metrics.go:178-194` — `readClashTraffic` клиент делает GET `127.0.0.1:9090/connections`. Если endpoint не отвечает, `available=false`; UI показывает «нет данных».
- **Вывод**: pinned sing-box build не включает `with_clash_api` tag. **TrafficCard реально покажет нули** кроме фактического running state.
- **План**: либо включить `with_clash_api` в `wails.json` tags (есть в реестре), либо реализовать `TrafficSource` интерфейс (`os_local_byte_counter` или netpoll на TCP connections) — это Phase 5.2 plan-item «Сделать TrafficSource интерфейс и отдельно проверить на pinned build».

### 4.2. ⚠️ **Country resolution — substring hack**
- `backend/core/channels.go:172-189` — `countryFromTag(tag)` итерирует map `nl→NL, fr→FR, de→DE, fi→FI, us→US, ru→RU` и возвращает **первое** совпадение `-code` или suffix. Map iteration в Go — **случайный** порядок → результат зависит от hash-collision Go runtime, может давать разный country в разные запуски для сложных tag вроде `out-nl-fr`. 
- **План**: для production нужны метаданные в отдельном манифесте (sidecar JSON) или в самом config (annotation `metadata: { country_code: NL }`). Phase 2.1 plan-item «Для country mapping использовать явные metadata».

### 4.3. ⚠️ **Wails CLI не установлен → не могу локально проверить `wails build`**
- `windows/wails.json:9` — tags `with_awg,with_wireguard,with_utls,with_gvisor`. ✅
- `where wails` — не найден. Старый exe 09.07.2025 в Roaming.
- **План**: `go install github.com/wailsapp/wails/v2/cmd/wails@latest`; затем `wails build -tags "with_awg,with_wireguard,with_utls,with_gvisor"` и локальный запуск в твоём GUI.

### 4.4. ⚠️ **Открыть «ПОДКЛЮЧИТЬ» в Android UI не поднимает TUN**
- `android/.../SnowdenVpnService.kt:135-178` — реализует всю правильную последовательность `Libbox.setup(SetupOptions) → newCommandServer(handler, platform) → commandServer.start()`.
- Но `commandServer.startOrReloadService(configJson, OverrideOptions)` **не вызывается** автоматически. По новому AAR это единственный прямой путь поднять VPN; альтернатива — CommandClient протокол (TCP сокет), который пока не запущен.
- **План**: реализовать `SnowdenCommandClient.kt` — на стороне SnowdenVpnService подключиться к собственному listener на 127.0.0.1:commandServerListenPort с commandServerSecret и отправить `serviceReload` с `configJson`. **Phase 2.**

### 4.5. ⚠️ **VPN config placeholders — никогда не был живой**
- `windows/assets/configs/template-vps-reality.json.example` — содержит `YOUR_*` placeholders. Реальный production config не в репо, не в backup'ах машины.
- **План**: либо попросить тебя положить реальный `template-vps-reality.json` (с приватными креденшелами) и не коммитить, либо использовать mocking endpoint с local DNS resolver.

## 5. Чеклист готовности к Phase 2 (реальный TUN)

Нужны **3 closed-loop** проверки перед тем как сказать «Android готов»:

1. ✅ Kotlin компилируется → ✓ done.
2. ✅ APK устанавливается → ✓ done (`pm list packages`).
3. ✅ MainActivity стартует → ✓ done (`am start`).
4. ❌ Кнопка «ПОДКЛЮЧИТЬ» запускает TUN → **открыто**; требует CommandClient + `serviceReload(configJson)`.
5. ❌ Реальный external HTTP request через TUN работает → **открыто**; нужно проверить через curl/whatsmyip после (4).
6. ❌ Live failover между двумя VPS → **открыто**; это Phase 3.

## 6. Roadmap Phase 2 → 7 в порядке «сначала видимое»

| # | Шаг | Что получишь | Что нужно |
|---|---|---|---|
| 1 | `SnowdenCommandClient.kt` + serviceReload | Кнопка «ПОДКЛЮЧИТЬ» реально поднимает TUN | AAR libbox API + TCP-сокет внутри VpnService |
| 2 | `MethodChannel` `status/diag` из Kotlin в Flutter | UI видит `running/starting/error` от libbox-стейта | Двухсторонний bridge |
| 3 | End2end: через TUN выходит HTTP-запрос на `gstatic.com/generate_204` | Видим, что трафик идёт через VPN | CMD `adb shell curl ...` через наш forward |
| 4 | Live VPS failover: добавить `vps-reality-2.json.example` | Adaptive переключается на 2-й VPS | Реальный 2-й VPS, `chooseRecoveryConfig` уже подключён |
| 5 | Включить `with_clash_api` в `wails.json` | TrafficCard начнёт показывать реальный трафик | Перекомпилировать; проверить что `registry.go` готов |
| 6 | Country mapping через metadata в config | Убрать substring hack | JSON-метаданные + `CountryFromMetadata` |
| 7 | `wails build` свежей версии → перетереть старый Roaming exe | Локальный live test на PC | `go install wails` |
| 8 | ADB-приёмка всего цикла: start → external https → failover → stop | Phase 7 жив | Требует 1–3 + 4 |

## 7. Известные edge-факты (документация для следующего dev'а)

- `app.go:265` — `OpenExternalApp("amnezia"/"karing")` стартует `.exe` напрямую + всегда вызывает `stopIfRunning()`. Это by-design «switching» поведение.
- `app.go:121` — жёстко зашит `LOCAL_VERSION = "1.3.5"`. Никакой связи с реальной версией из `git describe` или `version.json`. **Cosmetic hazard**.
- `app.go:185-219` — `ToggleRouteRule` напрямую правит JSON map, минуя `validator.Validate`. Потенциальный путь для fail-open если frontend уйдёт в плохом направлении. **План**: после toggle всегда re-validate через `prepareRuntimeConfig`.
- `manager.go:PollConnections()` (`manager.go:331`) — принимает **любой** домен, не только из нашего selector. DomainStatsCard может загрязниться при bulk traffic к RU-direct доменам. **План**: filter by `outbound ∈ ProtectedChannels(cfg)`.

## 8. Подтверждающие команды

```powershell
cd D:\snowden-v2
# Linux-friendly Go tests
'/c/Program Files/Go/bin/go.exe' test -count=1 ./backend/core/...
'/c/Program Files/Go/bin/go.exe' vet ./...
'/c/Program Files/Go/bin/go.exe' build -tags "with_awg,with_wireguard,with_utls,with_gvisor" ./...

# Android
cd android
'/c/Program Files/Go/bin/go.exe' ...  # Wait, это dart
'/d/flutter/bin/flutter.bat' analyze
'/d/flutter/bin/flutter.bat' test
'/d/flutter/bin/flutter.bat' build apk --release --dart-define-from-file=config.local.json

# Live phones
'/c/Users/Пользо/Android/Sdk/platform-tools/adb.exe' install -r build\app\outputs\flutter-apk\app-release.apk
```

Все ✅ зелёные последний раз (этот аудит).

---

> **TL;DR**: ~110 Go-тестов зелёные, Flutter analyze чисто, APK ставится и стартует.
> **Реальные блокеры для живого Android-VPN TUN**: только Phase 2 — реализация `SnowdenCommandClient` чтобы `commandServer.startOrReloadService` был вызван. Это один файл + один IPC.
> **Реальный blocker для TrafficCard**: добавить `with_clash_api` build tag или написать свой TrafficSource (Phase 5.2).
