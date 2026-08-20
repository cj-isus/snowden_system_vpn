# Android structure

> Источник истины для всего, что относится к Android-клиенту
> `snowden.system`. **Не рассказ** об общей архитектуре (см.
> `STRUCTURE.md` в корне и `PLAN.md`) — а узкая спецификация
> Android-слоя: что лежит в каком файле, какие компоненты отвечают
> за что, какие контракты с `libbox` сейчас живы, какие заглушки
> ещё ждут реализации command protocol.

## Контейнер проекта

```text
D:\snowden-v2\android\
├─ android\                       ← Gradle multi-project root (см. STRUCTURE.md / окно ниже)
│  ├─ app\src\main\kotlin\com\snowden\system\snowden_android\
│  │  ├─ MainActivity.kt         ← UI entry, держит конфиг профиля
│  │  ├─ SnowdenPlatformInterface.kt ← libbox PlatformInterface, новый API
│  │  ├─ SnowdenVpnService.kt    ← VpnService + Libbox.setup + newCommandServer
│  │  └─ SnowdenCommandServerHandler.kt ← CommandServerHandler (фаза 2 будет расти)
│  ├─ app\libs\libbox.aar         ← pinned v1.14.0-lx.3 (игнорируется git-ом)
│  ├─ gradle\wrapper              ← gradle-wrapper.jar + .properties (8.14.1)
│  ├─ build.gradle.kts           ← AGP 8.11.1, Kotlin 2.2.20, JDK 17
│  ├─ settings.gradle.kts         ← AGP/Kotlin versions, includeBuild flutter_tools
│  ├─ gradle.properties           ← JVM args: TLSv1.2 forced, parallel=true
│  └─ .gitignore                  ← /build, /app/build, /app/libs/libbox.aar
├─ lib\main.dart                  ← UI: connect button + status + stats
├─ test\widget_test.dart          ← smoke build SnowdenApp()
├─ config.example.json            ← щаблон VPN-профиля (placeholders)
├─ ANDROID_SETUP.md               ← ручной сценарий сборки и ADB приёмки
├─ STRUCTURE.md                   ← этот файл
└─ pubspec.yaml                   ← Dart-зависимости, минимальные (material+services)
```

### Контракт с libbox v1.14.0-lx.3

Сигнатуры `io.nekohasekai.libbox.*` **полностью переписаны** относительно
старого Android-API. Реальные сигнатуры (получены через `javap -public`
на `app/libs/libbox.aar`):

| Старый API (был) | Новый API (сейчас) | Где живёт |
|---|---|---|
| `openTun(TunOptions): Long` | `openTun(TunOptions): Int` | `SnowdenPlatformInterface.kt` |
| `closeTun(fd: Long)` | (нет, закрытие через libbox) | – |
| `autoDetectInterfaceControl(fd: Long): Long` | `autoDetectInterfaceControl(fd: Int)` (void) | там же |
| `usePlatformAutoDetectInterface(): Boolean` | `usePlatformAutoDetectInterfaceControl(): Boolean` | там же |
| `useWIFIState(): Boolean` | (убран — WIFIState теперь всегда есть) | – |
| `underVPN(): Boolean` | `underNetworkExtension(): Boolean` | там же |
| `getSystemProxyStatus(): SystemProxyStatus` | (убран из PlatformInterface; живёт в `CommandServerHandler.getSystemProxyStatus`) | `SnowdenCommandServerHandler` |
| `setSystemProxyEnabled(Boolean)` | (аналогично — в `CommandServerHandler`) | там же |
| `packageNameByUid / uidByPackageName / getPackageName / getUserID / getGroupID` | (всё убрано) — заменено на `lookupUser(name): PlatformUser` + `openShellSession(...)` | SSH/SFTP scope, для MVP возвращаем `throw UnsupportedOperationException` |
| `findConnectionOwner(...): ConnectionOwner` | сигнатура та же ✅ | `SnowdenPlatformInterface.kt` |
| `getInterfaces(): NetworkInterfaceIterator` | сигнатура та же ✅ | там же |
| `readWIFIState(): WIFIState` | сигнатура та же ✅ (`WIFIState` теперь mutable) | там же |
| `Libbox.start(path)` / `Libbox.stop()` | (**убраны**!) — Lifecycle через `CommandServer`/`CommandClient` | `SnowdenVpnService.kt` + фаза 2 |
| `newStringBox(...)` | (убран) — используется `new StringBox().apply { value = ... }` | (если нужен) |
| – (отсутствовал) | `Libbox.setup(SetupOptions)` | `SnowdenVpnService.kt` — обязателен перед `newCommandServer` |
| – (отсутствовал) | `Libbox.newCommandServer(handler, platform)` | `SnowdenVpnService.kt` — заменяет start() |
| – (отсутствовал) | `Libbox.reloadSetupOptions(...)` | для hot reload setup (фаза 2) |

### Текущее состояние (фаза 1)

- `SnowdenPlatformInterface.kt` — реализует **все** методы нового
  `PlatformInterface` с безопасными Android-реализациями: `openTun`
  создаёт VPN через стандартный `VpnService.Builder`; `findConnectionOwner`
  читает `/proc/net/{tcp,tcp6}` (legacy fallback) или возвращает базовый owner;
  `WIFIState` строится из `ConnectivityManager`; SSH/SFTP методы возвращают
  `throw UnsupportedOperationException` (use-case вне MVP).
- `SnowdenVpnService.kt` — больше не вызывает `Libbox.start(path)`. Вместо
  этого строит `SetupOptions` (paths, commandServer listen port/secret,
  logMaxLines, debug=false) → `Libbox.setup(setupOptions)` → создаёт
  handler + platform interface → `Libbox.newCommandServer(handler, platform)`.
  Сервер сам НЕ запускает VPN — это делается через command protocol, и
  до прихода `CommandClient` клиент остаётся в состоянии `Idle`. Это
  вынужденный stub: чтобы реально поднять туннель нужно либо прислать
  через command-протокол команду `serviceReload { ...config json... }`,
  либо дождаться подключения внешнего CommandClient (например, нашего
  Flutter-приложения через IPC, фаза 2).
- `SnowdenCommandServerHandler.kt` — реализует `CommandServerHandler`:
  `serviceReload` (в фазе 2 — `rebuild config + push`), `serviceStop`
  (сбросить state), `getSystemProxyStatus`/`setSystemProxyEnabled`
  get/set через `WIFI/ConnectivityManager`-based state, остальное
  no-op или throw.

### Что НЕ реализовано (фаза 2)

- **CommandClient** в Flutter-части: после старта command server ему надо
  подключиться TCP-сокетом на `commandServerListenPort` с
  `commandServerSecret`, отправить `serviceReload` с содержимым профиля.
  Только после этого libbox впервые вызовет `openTun` и появится
  foreground notification.
- **Adaptive controller и ChannelMemory** для Android-слоя: сейчас их нет,
  флаг из PLAN.md «эталон поведения — Windows MVP» остаётся в силе до
  завершения фазы 2.
- **Bridges, SSH/SFTP/Tailscale scope**: возвращаемые заглушки. Не приоритет
  для обхода DPI; используется только в случае расширения клиента в
  remote-access-tool сценарий.

### Сборка

```powershell
# 1) Один раз выполнить:
flutter config --jdk-dir='C:\Program Files\Eclipse Adoptium\jdk-17.0.19.10-hotspot'
Copy-Item .\android\config.example.json .\android\config.local.json
(отредактировать под свои VPS/UUID)
New-Item -ItemType Junction -Path 'D:\flutter_link' -Target 'D:\flutter'  # если ещё нет

# 2) Сборка:
flutter build apk --release --dart-define-from-file=config.local.json

# 3) Установка:
adb install -r .\build\app\outputs\flutter-apk\app-release.apk
```

Известные блокеры (Phase 2):

- libbox lifecycle **через command protocol** — стартует VPN только после
  того как `CommandClient` подключится к `CommandServer` и отправит
  `serviceReload`. Прямого API `"start config"` в новом AAR нет. Это
  противоречит исходному коду `SnowdenVpnService.Libbox.start(path)`.
- run-time `libbox` 1.14.0-lx.2 (Windows) vs 1.14.0-lx.3 (Android) — бинарь
  различается между слоями, поэтому live-приёмка Android ≠ Windows MVP.
