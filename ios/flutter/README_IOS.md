# snowden.system iOS — инструкция по сборке

> **Версия:** 1.0.0
> **Требует:** macOS + Xcode 15+ + Apple Developer ($99/год)

## Быстрый старт (5 шагов)

### Шаг 1: Установить инструменты на Mac

```bash
# Xcode (из App Store)
# Go:
brew install go
# gomobile:
go install golang.org/x/mobile/cmd/gomobile@latest
gomobile init
# Flutter:
brew install --cask flutter
```

### Шаг 2: Собрать libbox.xcframework

```bash
cd snowden_ios/ios
chmod +x build_libbox.sh
./build_libbox.sh
# займёт 5-10 минут → создаст Frameworks/Libbox.xcframework
```

### Шаг 3: Создать Flutter-проект

```bash
cd snowden_ios
# Создать iOS-платформу для существующего Flutter-проекта
flutter create --platforms=ios --org com.snowden.system .
```

Это создаст стандартную iOS-структуру. Затем скопируйте наши файлы:

```bash
# Скопировать AppDelegate (замена сгенерированного)
cp ios/Runner/AppDelegate.swift ios/Runner/
cp ios/Runner/TelegramReporter.swift ios/Runner/

# Скопировать Network Extension target
mkdir -p ios/PacketTunnel
cp ios/PacketTunnel/PacketTunnelProvider.swift ios/PacketTunnel/
cp ios/PacketTunnel/BoxPlatformInterface.swift ios/PacketTunnel/
cp ios/PacketTunnel/Info.plist ios/PacketTunnel/
cp ios/PacketTunnel/PacketTunnel.entitlements ios/PacketTunnel/

# Скопировать entitlements
cp ios/Runner/Runner.entitlements ios/Runner/
```

### Шаг 4: Настроить Xcode-проект

1. Открыть `ios/Runner.xcworkspace` в Xcode
2. **Добавить Network Extension target:**
   - File → New → Target → Network Extension
   - Type: Packet Tunnel
   - Name: `PacketTunnel`
   - Bundle ID: `com.snowden.system.PacketTunnel`
3. **Добавить Libbox.xcframework:**
   - Перетащить `Frameworks/Libbox.xcframework` в проект
   - Для ОБА таргета (Runner + PacketTunnel):
     - General → Frameworks, Libraries → Embed & Sign
4. **Настроить entitlements (оба таргета):**
   - Runner → Signing & Capabilities → + Capability:
     - App Groups: `group.com.snowden.system`
     - Network Extensions
   - PacketTunnel → то же самое
5. **Выбрать Signing Team** (Apple Developer account)
6. **Bundle Identifiers:**
   - Runner: `com.snowden.system.snowden`
   - PacketTunnel: `com.snowden.system.PacketTunnel`

### Шаг 5: Запуск на iPhone

```bash
flutter run --release
```

Или через Xcode: выбрать устройство → Run (⌘R).

При первом запуске iPhone покажет диалог "Разрешить VPN конфигурацию" → Разрешить.

---

## Архитектура iOS vs Android

| Компонент | Android | iOS |
|-----------|---------|-----|
| **UI** | Flutter (main.dart) | Flutter (тот же main.dart) |
| **VPN API** | VpnService + Builder | NEPacketTunnelProvider |
| **Bridge** | MethodChannel (Kotlin) | MethodChannel (Swift) |
| **Защита от петель** | protect(fd) | Автоматически (defaultPath) |
| **TUN** | VpnService.Builder → PFD | NEPacketTunnelNetworkSettings + packetFlow |
| **Per-app исключения** | addDisallowedApplication | НЕВОЗМОЖНО без MDM → route rules |
| **Кнопка в шторке** | Quick Settings Tile | НЕТ (виджет/Shortcut опционально) |
| **Ядро** | libbox.aar (98MB) | libbox.xcframework |
| **Telegram-бот** | TelegramReporter.kt | TelegramReporter.swift |

## Ключевые отличия (тонкости)

### 1. Нет protect() — но это нормально
На Android `VpnService.protect(fd)` предотвращает петлю: outbound-сокет к VPS
не уходит в TUN. На iOS `NEPacketTunnelProvider` **автоматически** биндит outbounds
к physical interface через default route. Дополнительно ничего делать не нужно.

### 2. Банковские исключения — через route rules
Android исключает 28 банковских приложений через `addDisallowedApplication`.
На iOS per-app split-tunnel невозможен без MDM. Вместо этого банковские домены
уже в route rules sing-box (`sberbank.ru`, `tinkoff.ru` → direct).

### 3. App Group — shared container
Конфиг и логи хранятся в `group.com.snowden.system` — доступно и основному
приложению, и Network Extension.

### 4. setFixAndroidStack НЕ вызывается
Этот флаг — Android-only. На iOS TUN создаётся через utun Network Extension.

### 5. libbox.xcframework
Собирается через `gomobile bind -target ios`. Включает arm64 (устройство) +
arm64/x86_64 (симулятор). Размер ~80-100 MB.

## Траблшутинг

### "No available network interface"
Реализуйте `getInterfaces()` через `getifaddrs()` — см. `BoxPlatformInterface.swift`.
Адреса должны быть в формате `"IP/prefix"` (без prefix → Go panic).

### "libbox not found"
Убедитесь что Libbox.xcframework добавлен в **оба** таргета (Runner + PacketTunnel)
с опцией "Embed & Sign".

### "VPN Configuration Failed"
- Проверьте Bundle ID: `com.snowden.system.PacketTunnel` точно
- Проверьте entitlements: App Group + Network Extensions
- Проверьте Apple Developer Team в Signing

### Telegram-репорты не приходят
api.telegram.org заблокирован в РФ. URLSession из Network Extension пойдёт через
VPN-туннель (если он поднят). Отчёт об отключении может не дойти (туннель уже закрыт).

## Структура файлов

```
snowden_ios/
├── lib/
│   └── main.dart                       Flutter UI + JSON-конфиг (идентичен Android)
├── pubspec.yaml                        Flutter зависимости
├── ios/
│   ├── build_libbox.sh                 Скрипт сборки xcframework (Mac)
│   ├── Frameworks/
│   │   └── Libbox.xcframework/         (создаётся build_libbox.sh)
│   ├── Runner/
│   │   ├── AppDelegate.swift           MethodChannel: startVpn/stopVpn/getStatus
│   │   ├── TelegramReporter.swift      Отчёты в Telegram (URLSession)
│   │   ├── Info.plist                  Конфигурация основного app
│   │   └── Runner.entitlements         App Group + Network Extensions
│   └── PacketTunnel/
│       ├── PacketTunnelProvider.swift  NEPacketTunnelProvider + libbox sing-box
│       ├── BoxPlatformInterface.swift  PlatformInterface для iOS
│       ├── Info.plist                  NEPacketTunnelProvider регистрация
│       └── PacketTunnel.entitlements   Packet Tunnel + App Group
└── README_IOS.md                       Этот файл
```

---

*Совместимость: iOS 15.0+ · sing-box-lx 1.14.0-lx.3 · libbox.xcframework*
