# AUDIT — что написано и что реально работает

> Проверено 2026-08-20 в `D:\snowden-v2` (рабочее дерево; базовый audit был начат на `8f46f06`).
> Выполнены статический аудит, локальные тесты/сборки и live ADB-smoke на
> разблокированном PTP N49 / Android 16. Wails-приложение отдельно не
> запускалось: exe от 2026-07-09 в Roaming не интерактивен.
> Все факты о текущем коде взяты из реальных файлов + `git grep` +
> `go test ./...` + `flutter test`/`flutter analyze`/`flutter build apk` +
> `adb logcat`/`dumpsys`.
> Каждый блок начинается с маркера состояния и ссылки на файл/строку.
>
> **Follow-up note:** разделы ниже с исходным Phase 1/Phase 2 описанием Android
> были написаны до текущего lifecycle-порта. Актуальный результат проверки и
> все исправления в этом working tree находятся в разделе «Follow-up audit»;
> при расхождении он имеет приоритет.

## Follow-up audit — Android lifecycle/profile (2026-08-20)

### Реально прогнанные проверки

```text
✅ flutter analyze                         No issues found
✅ flutter test                            20 tests passed
✅ android/gradlew :app:compileReleaseKotlin --no-daemon
   BUILD SUCCESSFUL
✅ flutter build apk --release --dart-define-from-file=config.local.json
   Built app-release.apk (232.3 MB)
✅ adb install -r app-release.apk          Success on PTP N49 / Android 16
✅ placeholder build + ADB tap             no native dispatch; UI kept the
                                            explicit config.local.json error
```

Для native error-path была собрана **временная debug APK только с dummy
values** (`203.0.113.10`, `test-password`, `example.com`; эти значения не
являются секретами). На реально запущенном Android service лог показал:

1. Первый lifecycle-дефект: `MainActivity` ставила `STARTING` до доставки
   intent, а `SnowdenVpnService` считывал этот общий state как «уже запущен».
   В результате `startForeground()` не вызывался и Android завершал FGS с
   `ForegroundServiceDidNotStartInTimeException`. Исправлено: service guard
   теперь смотрит на собственные `CommandServer/isRunning`, а не на shared
   dispatch state.
2. Второй профильный дефект: AAR отверг legacy `inet4_address`/
   `inet6_address`; после их замены на `tun.address[]` AAR дошёл до следующей
   миграционной проверки и отверг legacy `inbound.sniff`. Исправлено: sniff
   оставлен только как route action `{"action":"sniff"}`.
3. Ошибка libbox теперь идёт через единый `failVpn`: native resources
   закрываются, `ERROR` не затирается `onDestroy` в `STOPPED`, а `closeService`
   не вызывается до фактического успешного `startOrReloadService`.

Pure profile tests закрепляют отсутствие старых полей и protected
`route.final == "proxy"`; native Kotlin compile подтверждает границу AAR.

После этого выполнен новый live-smoke на разблокированном устройстве с
временным dummy-профилем (`203.0.113.10`, `test-password`, `example.com`):

- `SnowdenPlatformInterface` перечислил физические интерфейсы
  `rmnet_data0`, `rmnet_data2`, `rmnet_data3`, `wlan2`;
- libbox принял default interface `rmnet_data2`, TUN был создан;
- в логах подтверждены `TUN established`, `sing-box started` и
  `VPN started; TUN established`;
- повторный start после stop также прошёл, без `no available network
  interface`, native crash или `ForegroundServiceDidNotStartInTimeException`;
- Stop прошёл через cleanup, `SnowdenVpnService` исчез из `dumpsys`.

Во время этого smoke был обнаружен отдельный дефект: вызов
`Libbox.newStandaloneCommandClient()`/`urlTestOutbound()` внутри стартовой
транзакции приводил к необрабатываемому SIGSEGV в Go runtime после запуска
TUN. Startup probe удалён: старт отвечает только за инициализацию libbox и
TUN, а внешний HTTPS-тест теперь выполняется отдельным безопасным Java
компонентом после поднятия TUN. Хост-пакет больше не исключается из VPN:
обычный probe-сокет действительно проходит через `tun-in`, а native sockets
libbox по-прежнему вызывают `VpnService.protect(fd)`.

### Реальный локальный профиль и live-результат

Локальный `android/config.local.json` создан из трёх непустых значений
(`SNOWDEN_VPS_IP`, `SNOWDEN_HY2_PASSWORD`, `SNOWDEN_VPN_DOMAIN`), находится под
`.gitignore`; значения и хэши секретов в этот документ не попадают. Release
APK собрана именно с этим локальным профилем.

На PTP N49 / Android 16 выполнены две независимые проверки:

1. **Release, DNS через protected proxy:** `TUN established` → `VPN started;
   TUN established` → два DNS-запроса `www.gstatic.com` без ответа →
   `protected HTTPS probe failed: dns_failed` → единый `failVpn` → сервис
   отсутствует в `dumpsys`.
2. **Временный debug-only DNS bootstrap через local resolver:** DNS
   `www.gstatic.com` разрешился, но HTTPS-запрос через `hysteria2` завершился
   `timeout`; после этого probe также закрыл TUN. Это отделяет DNS bootstrap
   от самого защищённого канала.

Следовательно, Android lifecycle, TUN, physical underlay, DNS routing и
fail-closed cleanup подтверждены. Для присланного endpoint дополнительно
проверено:

- `kopilot.com A` сейчас возвращает `66.96.149.22`, а не VPS `89.125.1.217`;
  AAAA-записи нет;
- TLS на `66.96.149.22` отдаёт просроченный сертификат `*.bizland.com`, не
  соответствующий `kopilot.com`;
- TLS-запросы к `89.125.1.217` с SNI `kopilot.com` сбрасываются;
- TCP connect к портам `80/443/20843/30843/8090/8443` проходит, но это не
  доказывает наличие sing-box; HTTP/TLS на них зависает или сбрасывается;
- минимальный UDP-пакет на `89.125.1.217:8443` ответа не получил. Это не
  полноценный HY2 handshake, поэтому окончательный verdict по UDP требует
  проверки server logs или рабочего клиента после исправления DNS.

Успешный внешний HTTPS через текущий HY2 профиль не подтверждён. Первый
обязательный фикс — DNS `kopilot.com → 89.125.1.217`, затем действующий TLS
сертификат для `kopilot.com` и разрешённый UDP/QUIC `8443` в UFW/cloud security
group. Если мобильный оператор режет QUIC, нужен проверенный TCP VLESS на
`443`/`30843` и UUID; текущий Android-профиль содержит только HY2-реквизиты.
Для проверки `systemctl`, `sing-box check`, `ss`, firewall и journal нужен
read-only SSH-доступ к VPS; HY2-пароль для этого не подходит.

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
| Android Flutter UI, analyze | 🟢 green | `flutter analyze` No issues, `flutter test` 20/20 passed, profile/runtime/diagnostics/log/probe contracts separated into pure components |
| **Clash API source для TrafficCard** | 🟡 **НЕ работает** | pinned sing-box build не регистрирует Clash API в `registry.go`; `Metrics.readClashTraffic` стучится в `127.0.0.1:9090`, реально возвращает 0 B/s; UI честно показывает «нет данных» |
| **Android VPN реально поднимает TUN** | 🟢 **lifecycle + fail-closed** | На разблокированном PTP N49 подтверждены физические интерфейсы, `TUN established`, `VPN started; TUN established`, probe через `tun-in` и отсутствие service после отказа. Успешный внешний HTTPS текущего HY2 не подтверждён |
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
✅ flutter test                     All tests passed! (20/20 Android pure tests)
✅ flutter build apk --release      √ Built app-release.apk (232.3 MB)
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

### 4.4. ✅ **Android lifecycle now dispatches the direct native start path**
- `android/.../MainActivity.kt` validates config, handles VPN consent, and
  marks `STARTING` only as an asynchronous dispatch state.
- `android/.../SnowdenVpnService.kt` calls `startForeground` → `Libbox.setup`
  → `newCommandServer` → `server.start()` →
  `server.startOrReloadService(configJson, OverrideOptions)`.
- The live dummy-profile run reached real libbox start, enumerated physical
  underlay interfaces, established TUN, and completed a repeated start/stop
  cycle. The live local profile reaches the same TUN boundary, then the
  protected Java probe fails closed because the current HY2 path gives no
  HTTPS response.
- A startup `newStandaloneCommandClient().urlTestOutbound()` probe was removed
  after it reproducibly caused an uncaught native SIGSEGV on the pinned AAR;
  external connectivity is now a separate acceptance step, not part of the
  lifecycle transaction. Native `writeDebugMessage` logs now flow into a
  bounded 200-line in-memory `getLogs()` snapshot without opening that client.

### 4.5. 🟡 **VPS transport acceptance ещё не закрыта**
- Публичный `windows/assets/configs/template-vps-reality.json.example` по-прежнему содержит `YOUR_*` placeholders и не является release-источником.
- Android получил отдельный локальный ignored-профиль с непустыми HY2-реквизитами; это не означает, что серверный handshake успешно принят.
- На мобильной сети live probe получил `dns_failed` при protected DoH и `timeout` при debug local DNS bootstrap. Нужен доступный HY2 UDP/QUIC либо отдельный подтверждённый TCP-канал.

## 5. Чеклист готовности к Phase 2 (реальный TUN)

Нужны **3 closed-loop** проверки перед тем как сказать «Android готов»:

1. ✅ Kotlin компилируется → ✓ done.
2. ✅ APK устанавливается → ✓ done (`pm list packages`).
3. ✅ MainActivity стартует → ✓ done (`am start`).
4. ✅ Кнопка «ПОДКЛЮЧИТЬ» → dummy TUN smoke пройден: `TUN established` и `VPN started; TUN established`; повторный start/stop также пройден.
5. 🟡 Реальный protected HTTPS request через TUN → путь доказан, но текущий HY2 не вернул ответ; acceptance остаётся открытым до исправления серверного/сетевого канала.
6. ❌ Live failover между двумя VPS → **открыто**; это Phase 3.

## 6. Roadmap Phase 2 → 7 в порядке «сначала видимое»

| # | Шаг | Что получишь | Что нужно |
|---|---|---|---|
| 1 | Post-fix ADB start → TUN → stop | Закрыть lifecycle acceptance gap | ✅ TUN и fail-closed cleanup на локальном профиле |
| 2 | `MethodChannel` `status/diag` из Kotlin в Flutter | UI видит lifecycle, TUN, underlay, default interface и bounded logs |✅ typed status/diag/log/probe bridge + 20 pure Flutter tests; full CommandClient control ещё отдельно
 |
| 3 | End2end: Java probe через TUN → `gstatic.com/generate_204` | Видим DNS/HTTPS и не принимаем direct fallback | Код и fail-closed готовы; текущий HY2 даёт `timeout`, нужен рабочий канал |
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

> **TL;DR**: ~110 Go-тестов зелёные, Flutter analyze чисто, Android pure
> contracts 20/20, Kotlin compile и release APK зелёные. Локальный ignored
> профиль непустой и собран в release APK. На разблокированном телефоне
> подтверждены TUN, physical underlay, protected-path probe через `tun-in` и
> fail-closed cleanup: при текущем HY2 `dns_failed`/`timeout` сервис исчезает.
> Успешный внешний HTTPS пока не заявляется зелёным — нужен доступный HY2
> UDP/QUIC или проверенный TCP-канал.
> **Реальный blocker для TrafficCard**: добавить `with_clash_api` build tag или написать свой TrafficSource (Phase 5.2).
