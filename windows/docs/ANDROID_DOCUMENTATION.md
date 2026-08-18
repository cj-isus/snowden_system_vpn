# snowden.system — Android: техническая документация

> Версия: 1.0.0 · sing-box-lx 1.14.0-lx.3 · Flutter 3.x · Kotlin · minSdk 23

Документ описывает, **как** работает Android-клиент и **почему** принят именно такой
архитектурный подход. Каждое решение обосновано конкретной проблемой, с которой
мы столкнулись при отладке на реальном устройстве (Honor PTP-N49 / HarmonyOS 6).

---

## 1. Архитектура одним взглядом

```
┌─────────────────────────────────────────────────────────────┐
│  Flutter UI (lib/main.dart)                                 │
│  ┌───────────────┐   MethodChannel 'snowden.system/vpn'     │
│  │  Кнопка ВКЛ   │ ─────────────────────────────────────┐   │
│  │  Панель логов │ ◄────────── onLog (binaryMessenger)  │   │
│  └───────────────┘                                      │   │
└──────────────────────────────────────────────────────────┼───┘
                          │ startVpn(config)               │
                          ▼                                │
┌──────────────────────────────────────────────────────────┼───┐
│  MainActivity.kt (FlutterActivity)                       │   │
│  · VpnService.prepare() → запрос разрешения              │   │
│  · startForegroundService(SnowdenVpnService, config)     │   │
│  · binaryMessenger (static) — мост к Flutter UI ◄────────┘   │
└──────────────────────────────────────────────────────────────┘
                          │ Intent(ACTION_START, config)
                          ▼
┌────────────────────────────────────────────────────────────────┐
│  SnowdenVpnService.kt (VpnService)                             │
│                                                                │
│  ┌─ Libbox.setup(setupOpts) ── setFixAndroidStack(true)        │
│  ├─ CommandServer(handler, platform).start()                   │
│  ├─ CommandClient(CommandLog) → ядровые логи во Flutter        │
│  ├─ startOrReloadService(configJson, OverrideOptions)          │
│  │                                                              │
│  │  SnowdenPlatformInterface (PlatformInterface):               │
│  │  ├─ openTun() → VpnService.Builder → ParcelFileDescriptor   │
│  │  ├─ autoDetectInterfaceControl(fd) → service.protect(fd)    │
│  │  ├─ startDefaultInterfaceMonitor → updateDefaultInterface   │
│  │  └─ getInterfaces() → java.net.NetworkInterface enumeration │
│  │                                                              │
│  └─ tunFd (поле класса — защита от GC)                         │
└────────────────────────────────────────────────────────────────┘
                          │ fd (TUN)
                          ▼
┌────────────────────────────────────────────────────────────────┐
│  sing-box ядро (libbox.so, native)                             │
│                                                                │
│  TUN inbound (172.19.0.1/30, stack=system)                     │
│       │                                                        │
│       ▼                                                        │
│  Маршрутизатор (route rules):                                  │
│  · sniff → hijack-dns (только DNS)                             │
│  · .ru/.su/.рф + банки/Госуслуги/ВК → direct (мимо VPN)        │
│  · Telegram IP 91.108.0.0/16, 149.154.0.0/16 → auto (VPN)      │
│  · YouTube/Discord/AI/соцсети → auto (VPN)                     │
│  · private IP → direct                                         │
│  · final → auto (urltest: VLESS + Hysteria2)                   │
│       │                                                        │
│       ▼                                                        │
│  VLESS+TLS (YOUR_VPS_IP:443) — основной канал                  │
│  Hysteria2 (YOUR_VPS_IP:8443/UDP) — резерв (UDP режется)       │
└────────────────────────────────────────────────────────────────┘
```

---

## 2. Стек технологий и почему именно он

| Слой | Технология | Почему не альтернатива |
|------|------------|------------------------|
| **UI** | Flutter (Dart) | Один код для Android+iOS; быстрее нативной разработки UI; Material 3 из коробки. Нативный Kotlin UI потребовал бы в 3-5 раз больше кода для того же интерфейса. |
| **Ядро VPN** | sing-box-lx 1.14.0-lx.3 через libbox.aar | Единое ядро с ПК-версией (Wails). lx-форк добавляет AmneziaWG 2.0, MASQUE, XHTTP. Альтернативы (Xray-core, v2ray) не имеют готового AAR и PlatformInterface под Android. |
| **Мост Kotlin↔Go** | gomobile (libbox.aar) | libbox экспортирует Go-классы как Java-интерфейсы через gomobile binding. Это позволяет вызывать sing-box как библиотеку, а не subprocess — корректная очистка ресурсов и логирование. |
| **VPN-интерфейс** | Android VpnService API | Единственный способ поднять TUN без root. VpnService.Builder создаёт ParcelFileDescriptor, который sing-box читает/пишет как TUN. |
| **TUN стек** | `stack: "system"` | gVisor вызывал SIGSEGV (native crash) в libbox.so на HarmonyOS. System-стек использует ядерный TCP/IP напрямую через `setFixAndroidStack(true)`. |

---

## 3. Ключевые архитектурные решения (и почему)

### 3.1. Почему sing-box встроен как библиотека, а не subprocess

**Проблема:** На ПК мы пробовали запускать sing-box как отдельный процесс. На Windows это создавало проблемы с завершением (зомби-процессы, зависшие TUN).

**Решение:** На Android sing-box встроен через `box.New().Start()/Close()` (через libbox `startOrReloadService`). Ядро работает в том же процессе, что и VpnService.

**Почему:** VpnService предоставляет fd TUN только в контексте своего процесса. Если sing-box в subprocess — пришлось бы пробрасывать fd через UNIX-domain сокет, что усложняет код и нестабильно. Плюс CommandClient для логов работает только внутри процесса.

---

### 3.2. Почему `setFixAndroidStack(true)` — критично

**Проблема:** TUN с `stack: "system"` на Android требует особой обработки netlink-сокетов. Без `FixAndroidStack` sing-box не может корректно читать/писать в TUN fd.

```kotlin
val setupOpts = SetupOptions()
setupOpts.setFixAndroidStack(true)  // ← КРИТИЧНО
Libbox.setup(setupOpts)
```

**Почему:** Go-рантайм на Android использует MTE (Memory Tagging) и PAC (Pointer Authentication). `FixAndroidStack` настраивает Go-рантайм на корректную работу с tagged pointers при системных вызовах к TUN. Без этого — silent corruption или SIGSEGV.

---

### 3.3. Почему `getInterfaces()` нельзя оставлять пустым

**Симптом:** `outbound vless unavailable: no available network interface` — все outbounds падали, трафик не шёл, хотя TUN был поднят.

**Причина:** sing-box `NetworkManager` вызывает `PlatformInterface.getInterfaces()` для получения списка физических интерфейсов (wlan0, rmnet_data2). Если метод выбрасывает `UnsupportedOperationException`, NetworkManager получает пустой список → не может найти интерфейс для исходящего соединения к VPS → outbound недоступен.

**Решение:** Реализовали перечисление через `java.net.NetworkInterface.getNetworkInterfaces()`:

```kotlin
override fun getInterfaces(): NetworkInterfaceIterator {
    return AndroidNetworkInterfaceIterator(service)
}
```

Каждый интерфейс возвращает:
- `name` — "rmnet_data2", "wlan2"
- `addresses` — **с prefix length** (`10.65.83.35/29`), иначе Go panic → SIGABRT
- `type` — WIFI / Cellular / Other (через `Libbox.InterfaceTypeWIFI` и т.д.)

**Почему с prefix length:** Go-side парсит адреса через `netip.ParsePrefix()`. Строка `"10.65.83.35"` без `/29` вызывает panic в Go-рантайме → SIGABRT → краш всего процесса. Это заняло 2 часа дебага, пока мы не увидели tombstone.

---

### 3.4. Почему `startDefaultInterfaceMonitor` + `updateDefaultInterface` обязательны

**Симптом:** TUN поднят, трафик попадает в туннель, но `protect()` никогда не вызывается → соединения к VPS уходят в TUN → бесконечная петля → нет интернета.

**Причина:** sing-box должен знать имя активного сетевого интерфейса (например, `rmnet_data2` для LTE), чтобы:
1. Привязывать outbound-сокеты к этому интерфейсу
2. Вызывать `autoDetectInterfaceControl(fd)` для каждого нового сокета
3. `autoDetectInterfaceControl` → наш `service.protect(fd)` — защищает сокет от попадания в TUN

**Решение:** При старте определяем активный интерфейс через `ConnectivityManager`:

```kotlin
private fun detectDefaultInterface() {
    val cm = getSystemService(Context.CONNECTIVITY_SERVICE) as ConnectivityManager
    val active = cm.activeNetwork
    val lp = cm.getLinkProperties(active)
    defaultInterfaceName = lp?.interfaceName ?: ""  // "rmnet_data2"
}
```

Затем при вызове `startDefaultInterfaceMonitor(listener)` сразу уведомляем sing-box:

```kotlin
listener.updateDefaultInterface(defaultInterfaceName, 0, false, true)
```

**Почему это работает:** После уведомления sing-box начинает вызывать `autoDetectInterfaceControl(fd)` для каждого outbound-сокета → наш `protect()` срабатывает → сокет к VPS (YOUR_VPS_IP:443) уходит через физический интерфейс, а не через TUN → петля разорвана → интернет работает.

---

### 3.5. Почему `addDisallowedApplication` для собственного приложения

```kotlin
builder.addDisallowedApplication("com.snowden.system.snowden_android")
```

**Проблема:** Без этого sing-box пытается отправить свой собственный трафик к VPS через TUN → петля.

**Двойная защита:** `protect()` (вызывается sing-box) + `addDisallowedApplication` (на уровне VpnService.Builder). Даже если `protect()` по какой-то причине не сработает, приложение исключено из VPN на системном уровне.

---

### 3.6. Почему `auto_route: false` + ручной `addRoute("0.0.0.0", 0)`

**Проблема:** `auto_route: true` в sing-box пытается изменить таблицу маршрутизации через `ip route`, что на Android без root невозможно.

**Решение:**
- `auto_route: false` в конфиге TUN
- Вручную `builder.addRoute("0.0.0.0", 0)` в `openTun()` — это делает VpnService API корректно, через Android framework, а не через netlink.

```kotlin
builder.addRoute("0.0.0.0", 0)  // захватываем ВЕСЬ IPv4 трафик
```

**Почему OverrideOptions.autoRedirect = false:** lx-форк добавил autoRedirect, который пытается использовать iptables REDIRECT — требует root. Отключаем:

```kotlin
override.setAutoRedirect(false)
```

---

### 3.7. Почему ParcelFileDescriptor хранится в поле класса

```kotlin
private var tunFd: ParcelFileDescriptor? = null
// ...
service.tunFd = pfd  // в openTun()
```

**Проблема:** Если PFD будет собран GC, Android закроет TUN fd → sing-box получит EBADF → трафик остановится.

**Решение:** Держим сильную ссылку в поле класса `SnowdenVpnService`. Пока сервис жив — PFD не будет собран.

---

### 3.8. Почему `stack: "system"`, а не `"gvisor"`

**Проблема:** gVisor (userspace TCP/IP) вызывал SIGSEGV в libbox.so на Honor PTP-N49 (HarmonyOS 6, ARM64 с MTE).

**Решение:** `stack: "system"` использует ядерный TCP/IP стек через `/dev/tun` + `setFixAndroidStack(true)`.

**Компромисс:** System-стек требует корректной работы `protect()` (gVisor не нуждается в protect, т.к. обрабатывает TCP в userspace). Но зато нет крашей.

---

## 4. Раздельное туннелирование (Split Tunneling)

### Логика маршрутизации

```
Трафик приложения
    │
    ▼
[TUN 172.19.0.1/30] ── весь трафик попадает сюда
    │
    ▼
Правило 1: sniff (определить протокол/домен по SNI)
    │
    ▼
Правило 2: hijack-dns (ТОЛЬКО для DNS-портов — перехват DNS)
    │
    ▼
Правило 3: domain_suffix .ru/.su/.рф → DIRECT (мимо VPN)
    │      └─ Сбер, Тинькофф, ВТБ, Альфа, Госуслуги, ВК, Яндекс...
    ▼
Правило 4: Telegram IP (91.108.0.0/16, 149.154.0.0/16) → VPN
    │      └─ Telegram использует MTProto, подключается по IP напрямую
    ▼
Правило 5: YouTube/Discord/AI/соцсети → VPN
    │
    ▼
Правило 6: private IP (10.x, 192.168.x, 172.16-31.x) → DIRECT
    │
    ▼
Правило 7 (final): всё остальное → auto (urltest: VLESS/Hysteria2)
```

### Почему именно так

| Категория | Маршрут | Причина |
|-----------|---------|---------|
| **Российские банки** | direct | Блокируют зарубежные IP (антифрод). Через VPN = отказ входа. |
| **Госуслуги** | direct | Не пускают с иностранных IP; напрямую работают быстрее. |
| **ВК, Яндекс, Mail.ru** | direct | Доступны в РФ напрямую, нет смысла тратить VPN-трафик. |
| **Telegram** | VPN | Заблокирован ТСПУ. IP зашиты в приложении (MTProto). |
| **YouTube** | VPN | Заблокирован (замедление ТСПУ с 2024). |
| **Discord, Instagram, X** | VPN | Заблокированы Роскомнадзором. |
| **Локальная сеть** | direct | Принтеры, NAS, умный дом — не должны идти через VPS. |

### Почему `protocol: "dns"` в hijack-dns критично

**Ошибкa (было):** `{"action": "hijack-dns"}` без матчера → **весь трафик** перехватывался как DNS → соединения зависали.

**Правильно:** `{"protocol": "dns", "action": "hijack-dns"}` → перехватываются только реальные DNS-запросы (UDP/TCP порт 53), остальное идёт дальше по правилам.

---

## 5. Конфигурация sing-box (полный конфиг)

```json
{
  "log": {"level": "debug"},
  "dns": {
    "servers": [
      {"type": "https", "tag": "remote", "server": "1.1.1.1", "detour": "auto"},
      {"type": "local", "tag": "local", "detour": "direct"}
    ],
    "rules": [{"outbound": "any", "server": "local"}],
    "final": "remote",
    "strategy": "ipv4_only"
  },
  "inbounds": [
    {
      "type": "tun", "tag": "tun-in",
      "address": ["172.19.0.1/30"],
      "mtu": 1500,
      "auto_route": false,
      "strict_route": false,
      "stack": "system"
    }
  ],
  "outbounds": [
    {
      "type": "urltest", "tag": "auto",
      "outbounds": ["vless", "hysteria2"],
      "url": "https://www.gstatic.com/generate_204",
      "interval": "30s", "tolerance": 50
    },
    {
      "type": "vless", "tag": "vless",
      "server": "YOUR_VPS_IP", "server_port": 443,
      "uuid": "***",
      "tls": {"enabled": true, "server_name": "YOUR_DOMAIN.nip.io"}
    },
    {
      "type": "hysteria2", "tag": "hysteria2",
      "server": "YOUR_VPS_IP", "server_port": 8443,
      "password": "***",
      "tls": {"enabled": true, "server_name": "YOUR_DOMAIN.nip.io"}
    },
    {"type": "direct", "tag": "direct"},
    {"type": "block", "tag": "block"}
  ],
  "route": {
    "rules": [
      {"action": "sniff"},
      {"protocol": "dns", "action": "hijack-dns"},
      {"domain_suffix": [".ru", ".su", ".рф"], "action": "direct"},
      {"domain": ["...банки, Госуслуги, ВК..."], "action": "direct"},
      {"domain_suffix": ["...YouTube, Discord..."], "action": "auto"},
      {"domain": ["t.me", "telegram.org"], "action": "auto"},
      {"ip_cidr": ["91.108.0.0/16", "149.154.0.0/16"], "action": "auto"},
      {"ip_is_private": true, "action": "direct"}
    ],
    "final": "auto",
    "default_domain_resolver": "local",
    "auto_detect_interface": true
  }
}
```

### Почему VLESS+TLS, а не Reality

| Протокол | Статус | Причина |
|----------|--------|---------|
| **VLESS+TLS** | ✅ Основной | Reality handshake нестабилен в lx 1.14; чистый VLESS+TLS с Let's Encrypt надёжнее. Flow=vision блокируется ТСПУ — убран. |
| **Hysteria2** | ⚠️ Резерв | Работает по UDP/QUIC. Мобильные операторы РФ режут UDP → `context deadline exceeded`. urltest автоматически выбирает VLESS. |
| **WireGuard/WARP** | 📋 План | Конфиг готов, не интегрирован в Android. |
| **AmneziaWG** | 📋 План | lx-форк поддерживает, требует сборки с `with_awg`. |

### urltest — автоматический выбор

```
urltest[auto]:
  ├─ vless     → available: 165ms ✓ ВЫБРАН
  └─ hysteria2 → unavailable: context deadline exceeded (UDP режется)
```

Каждые 30 секунд sing-box тестирует оба outbound через `https://www.gstatic.com/generate_204`. Выбирает самый быстрый (с допуском 50ms).

---

## 6. Логирование — как логи доходят до UI

```
sing-box ядро (Go)
    │
    ▼ CommandServer (UNIX domain socket внутри процесса)
    │
    ▼ CommandClient(CommandLog)
    │
    ▼ writeLogs(LogIterator) → LogEntry.getMessage()
    │
    ▼ SnowdenVpnService.sendLog()
    │
    ├─→ Log.i("SnowdenVpn", msg)          → logcat
    ├─→ File(filesDir, "vpn.log")          → файл
    └─→ MethodChannel('snowden.system/vpn').invokeMethod("onLog", msg)
                │
                ▼
        Flutter: setState(() => _logs.add(line))
                │
                ▼
        Панель логов в UI (с кнопкой копирования)
```

**Почему через binaryMessenger (static):** VpnService и MainActivity — разные компоненты с разными lifecycle. Service не может держать ссылку на MethodChannel из Activity (Activity может быть уничтожена, а Service работать). Поэтому `binaryMessenger` хранится статически в `MainActivity.Companion`.

---

## 7. Жизненный цикл VPN

```
[Пользователь нажимает ПОДКЛЮЧИТЬ]
    │
    ▼
Flutter → MethodChannel 'startVpn' → MainActivity
    │
    ▼
VpnService.prepare(this) → системный диалог "Разрешить VPN?"
    │
    ├─ Отказ → ничего не происходит
    │
    ▼ Разрешение получено
startForegroundService(SnowdenVpnService, ACTION_START, config)
    │
    ▼
SnowdenVpnService.onStartCommand:
    ├─ createNotificationChannel + startForeground (обязательно для Android 8+)
    ├─ Libbox.setup(setFixAndroidStack=true)
    ├─ detectDefaultInterface() → "rmnet_data2"
    ├─ CommandServer(handler, platform).start()
    ├─ CommandClient(CommandLog).connect() → поток логов
    └─ Thread { startOrReloadService(config, override) }
         │
         ▼ sing-box ядро стартует
         │
         ├─ PlatformInterface.startDefaultInterfaceMonitor(listener)
         │   └─ registerInterfaceListener → updateDefaultInterface("rmnet_data2")
         │
         ├─ PlatformInterface.getInterfaces()
         │   └─ [wlan2, rmnet_data1, rmnet_data2] (с IPv4/prefix)
         │
         ├─ PlatformInterface.openTun(options)
         │   ├─ VpnService.Builder()
         │   ├─ addAddress("172.19.0.1", 30)
         │   ├─ addRoute("0.0.0.0", 0)       ← весь трафик
         │   ├─ addDnsServer("8.8.8.8", "1.1.1.1")
         │   ├─ addDisallowedApplication(own app)  ← анти-петля
         │   ├─ establish() → ParcelFileDescriptor
         │   └─ return fd → sing-box начинает читать TUN
         │
         ├─ [Трафик приложения попадает в TUN]
         │
         ├─ sing-box создаёт outbound-сокет к VPS (YOUR_VPS_IP:443)
         │   └─ PlatformInterface.autoDetectInterfaceControl(fd)
         │       └─ service.protect(fd)      ← сокет не уйдёт в TUN
         │
         └─ [Данные текут: приложение ↔ TUN ↔ sing-box ↔ VLESS ↔ VPS ↔ интернет]
```

---

## 8. Известные ограничения и будущие доработки

| Компонент | Статус | Что нужно |
|-----------|--------|-----------|
| **Hysteria2 по мобильной сети** | ❌ UDP режется | Настроить UDP-порт на VPS через другой диапазон; или использовать только Wi-Fi. |
| **Адаптивный движок (Circuit Breaker)** | 📋 План | Перенести с ПК (Go) → Kotlin. Автопереключение серверов при сбое. |
| **Error Classifier** | 📋 План | Парсинг логов sing-box на Android для классификации ошибок. |
| **Telegram-бот мониторинг** | 📋 План | Отчёты каждые 15 мин (как на ПК). |
| **Cloudflare Worker** | 📋 План | Динамические конфиги, edge health-check. |
| **IPv6** | ❌ Отключён | `strategy: "ipv4_only"`. Некоторые операторы дают IPv6 — можно включить. |
| **Per-app VPN** | 📋 Возможность | VpnService.Builder поддерживает `addAllowedApplication` — можно пускать через VPN только выбранные приложения. |

---

## 9. Сборка из исходников

### Требования
- Flutter SDK
- JDK 17 (Eclipse Adoptium)
- Android SDK (minSdk 23, targetSdk 34)
- libbox.aar (sing-box-lx 1.14.0-lx.3, arm64-v8a) в `android/app/libs/`

### Команды
```bash
# Сборка release APK
flutter build apk --release

# Результат
build/app/outputs/flutter-apk/app-release.apk
```

### Установка на устройство
```bash
adb install -r app-release.apk
```

---

## 10. Дебаг

### Чтение логов
```bash
# Логи SnowdenVpn (наши)
adb logcat -s SnowdenVpn:V

# Логи по PID процесса
adb logcat --pid=$(adb shell pidof com.snowden.system.snowden_android)

# Native краши (tombstone)
adb logcat -b crash

# Трафик TUN
adb shell cat /proc/net/dev | grep tun0
```

### Типичные проблемы

| Симптом | Причина | Решение |
|---------|---------|---------|
| `no available network interface` | `getInterfaces()` выбрасывает исключение | Реализовать перечисление через `java.net.NetworkInterface` |
| SIGABRT в libbox.so | Адреса без prefix length (`10.0.0.1` вместо `10.0.0.1/24`) | Использовать `InterfaceAddress.getNetworkPrefixLength()` |
| `protected fd=` не появляется | `startDefaultInterfaceMonitor` пустой | Реализовать + вызвать `updateDefaultInterface` |
| Трафик петлит в TUN | `protect()` не вызывается | Проверить `usePlatformAutoDetectInterfaceControl() == true` |
| Банки не работают | Весь трафик через VPN | Добавить split-tunneling (.ru → direct) |
| Telegram не работает | `hijack-dns` без матчера перехватывает всё | `{"protocol": "dns", "action": "hijack-dns"}` |
| gVisor SIGSEGV | Несовместимость с MTE на новых ARM64 | `stack: "system"` + `setFixAndroidStack(true)` |

---

## 11. Структура проекта

```
snowden_android_short/
├── lib/
│   └── main.dart                    # Flutter UI + генерация конфига sing-box
├── android/app/src/main/kotlin/com/snowden/system/snowden_android/
│   ├── MainActivity.kt              # MethodChannel bridge, VPN permission, restart-before-start
│   ├── SnowdenVpnService.kt         # VpnService + libbox + Telegram reporting
│   ├── SnowdenTileService.kt        # Quick Settings Tile (кнопка в шторке)
│   └── TelegramReporter.kt          # Отправка статуса VPN в Telegram
├── android/app/libs/
│   └── libbox.aar                   # sing-box-lx 1.14.0-lx.3 (98MB, arm64-v8a)
└── android/app/src/main/
    └── AndroidManifest.xml          # VpnService + foreground service + Tile declaration
```

---

## 12. Исключение банков и ВК (Anti-Detection)

### Проблема
Российские банки и Госуслуги определяют, что включён VPN, и блокируют вход
(антифрод: "вход из-за границы" / "обнаружен прокси"). Это происходит потому,
что трафик попадает в TUN → приложение видит `tun0` интерфейс + чужой IP.

### Решение: `addDisallowedApplication`
В `openTun()` вызываем `builder.addDisallowedApplication(pkg)` для каждого банка.
Это исключает пакет **полностью** из VPN — его сокеты идут через физический
интерфейс напрямую, TUN их не видит.

```kotlin
val bankPackages = listOf(
    "ru.sberbankmobile",      // СберБанк
    "com.idamob.tinkoff.android",  // Т-Банк
    "ru.vtb24.mobilebanking.android",  // ВТБ
    // ... 28 пакетов
)
for (pkg in bankPackages) {
    builder.addDisallowedApplication(pkg)
}
```

### Почему это работает
`addDisallowedApplication` — это уровень Android VpnService API. Система просто
не маршрутизирует сокеты этого UID через TUN. Приложение:
- Не видит `tun0` интерфейс
- Не имеет VPN network capability
- Видит обычный IP оператора

Это **надёжнее** чем доменные правила в sing-box, т.к. работает даже если банк
использует IP-адреса напрямую (без DNS).

### Список исключённых приложений (28 пакетов)
- **Банки**: Сбер, Т-Банк, ВТБ, Альфа, РСХБ, Газпромбанк, МКБ, ПСБ, Открытие,
  Хоум Кредит, Россельхозбанк, ОТП, Райффайзен
- **Госуслуги**: Госуслуги, Налоговая
- **Соцсети/Маркетплейсы**: ВКонтакте, Одноклассники, ВК Мессенджер, Mail.ru,
  Авито, Озон, Wildberries, Яндекс

---

## 13. Quick Settings Tile (кнопка в шторке)

### Архитектура
`SnowdenTileService` наследует `TileService` — стандартный Android API для
добавления кастомных плиток в шторку быстрых настроек.

### Поведение
| Состояние | Действие при тапе |
|-----------|-------------------|
| VPN выключен | Запускает VPN (из сохранённого конфига) |
| VPN включён | Останавливает VPN |
| Нет конфига | Открывает приложение для первичной настройки |

### Хранение конфига
`SnowdenTileConfig` сохраняет последний конфиг в SharedPreferences при каждом
запуске из приложения. Tile читает его и запускает VPN без UI.

### Как добавить плитку
Tile не появляется автоматически — Android требует ручного добавления:
1. Открыть шторку (смахнуть вниз дважды)
2. Нажать ✏️ (карандаш / «Изменить»)
3. Найти «snowden.system» в доступных плитках
4. Перетащить в активную зону

---

## 14. Telegram Reporter (Android → Telegram)

### Зачем
Android-устройство должно сообщать свой статус в тот же Telegram-чат, что и ПК.
Это позволяет удалённо знать: включён ли VPN на телефоне, работает ли он.

### Архитектура
`TelegramReporter` — singleton-объект (Kotlin `object`), который отправляет
HTTPS POST на `api.telegram.org` при событиях VPN.

| Событие | Сообщение |
|---------|-----------|
| VPN подключён | 🟢 «VPN подключён» + модель телефона |
| VPN отключён | 🔴 «VPN отключён» |
| Ошибка запуска | ❌ «Ошибка запуска VPN» + текст ошибки |

### Как это работает
1. `SnowdenVpnService.startVpn()` → успех → `TelegramReporter.report("🟢 VPN подключён")`
2. `TelegramReporter` запускает фоновый поток
3. HTTPS POST к `api.telegram.org/bot.../sendMessage` (через VPN-туннель,
   т.к. Telegram API заблокирован в РФ)
4. Сообщение появляется в чате

### Почему через VPN-туннель
`api.telegram.org` заблокирован в России. Прямой запрос из Android упадёт с
timeout. Но к моменту вызова `TelegramReporter.report()` VPN уже поднят, и
трафик Android-приложения идёт через TUN → VPS → Telegram API. Поэтому запрос
проходит. При `stopVpn` сообщение об отключении может не дойти (VPN уже закрыт).

### Throttling
`reportOkIfDue()` — не чаще раза в 30 минут, чтобы не спамить.

---

## 15. Restart-before-start фикс

### Проблема
Если VPN запущен из Tile (шторки), а потом пользователь нажимает кнопку в
приложении — sing-box падает с `engine: already running`.

### Решение
В `MainActivity.startVpn` перед запуском проверяем `SnowdenVpnService.isRunning`.
Если True — сначала отправляем `ACTION_STOP`, ждём 800мс, потом запускаем заново:

```kotlin
if (SnowdenVpnService.isRunning) {
    val stopIntent = Intent(this, SnowdenVpnService::class.java)
    stopIntent.action = ACTION_STOP
    startService(stopIntent)
    Thread.sleep(800)
}
```

---

## 16. Теория: как работает обход блокировок

### ТСПУ (Технические средства противодействия угрозам)
Роскомнадзор устанавливает ТСПУ у провайдеров — это DPI (Deep Packet Inspection),
который анализирует трафик и блокирует по сигнатурам.

### Что блокирует ТСПУ
| Протокол | Метод блокировки |
|----------|-----------------|
| OpenVPN | Сигнатура TLS handshake |
| WireGuard | UDP-фингерпринт |
| Shadowsocks | Энтропийный анализ |
| Tor | Список relay-узлов |
| Telegram (MTProto) | SNI + IP-блокировка |

### Почему VLESS+TLS работает
VLESS оборачивает трафик в стандартный TLS handshake с настоящим сертификатом
(Let's Encrypt). Для ТСПУ это выглядит как обычный HTTPS к сайту:

```
[Телефон] → TLS(ClientHello, SNI=snowden-system.nip.io) → [ТСПУ] → [VPS]
                            ↑
                   ТСПУ видит обычный HTTPS.
                   Не может отличить от легитимного трафика.
```

### Почему НЕ Reality
Reality — это продвинутый TLS-маскировка (подмена сертификата реального сайта).
Но в sing-box-lx 1.14 Reality handshake нестабилен, а чистый VLESS+TLS с
настоящим Let's Encrypt сертификатом надёжнее и проще.

### Почему НЕ flow=vision
`xtls-rprx-vision` — ускорение VLESS через direct-path (без расшифровки
payload). Но ТСПУ научился детектить vision-трафик по паттернам. Чистый VLESS
без flow проходит как обычный TLS.

### BBR Congestion Control
На VPS включён BBR вместо стандартного cubic. BBR лучше для дальних линков
(Россия → Нидерланды, ~76ms):
- Не реагирует на случайные потери пакетов (cubic воспринимает как перегрузку)
- Агрессивнее захватывает полосу
- Стабилизирует latency (было 0.4-2.8с → стало 0.3-0.5с)

### Split-tunneling (раздельное туннелирование)
Российские сайты идут напрямую (мимо VPN), заблокированные — через VPN.
Это нужно потому что:
- Банки блокируют зарубежные IP (антифрод)
- Госуслуги не пускают с иностранных IP
- Российские сайты доступны напрямую (нет смысла тратить VPN-трафик)

### urltest (автоматический выбор протокола)
sing-box каждые 30 секунд тестирует оба outbound (VLESS + Hysteria2) через
HTTP-запрос к `gstatic.com/generate_204`. Выбирает самый быстрый.

На мобильной сети Hysteria2 (UDP) обычно недоступен — операторы режут QUIC.
urltest автоматически выбирает VLESS (TCP).

---

## 17. Учётные данные сервера

```
VPS: YOUR_VPS_IP (Нидерланды)
VLESS:    порт 443/TCP,  TLS (Let's Encrypt), BBR
Hysteria2: порт 8443/UDP, TLS (режется мобильными операторами)
Домен:    YOUR_DOMAIN.nip.io (wildcard DNS nip.io)
CPU:      1 ядро Xeon Skylake, 2GB RAM
Скорость: ~600 Мбит/с (VPS→Cloudflare), ~40 Мбит/с (через туннель)
```

---

*Документация обновлена 2026-07-15. Версия: 1.1.0.*
*Включает: Tile, Telegram Reporter, банковские исключения, теория обхода.*
*Тестирование: Honor PTP-N49, HarmonyOS 6, через LTE (МегаФон) + Wi-Fi.*
