# STRUCTURE.md — ios/

> iOS-клиент: два варианта — Flutter (`ios/flutter/`) и нативный Swift
> (`ios/native/SnowdenSystem/`). Переиспользует sing-box (GoCore) через XCFramework.

## Состав
| Папка | Роль |
|-------|------|
| `flutter/` | Flutter-вариант для iOS + sing-box (libbox) |
| `native/SnowdenSystem/` | Нативный Swift/Xcode-проект (XcodeGen `project.yml`) + GoCore |

## flutter/ (Системный туннель)
| Файл | Роль |
|------|------|
| `lib/main.dart` | Flutter-UI |
| `ios/PacketTunnel/PacketTunnelProvider.swift`, `BoxPlatformInterface.swift` | NEPacketTunnelProvider + мост к sing-box |
| `ios/Runner/AppDelegate.swift`, `TelegramReporter.swift` | AppDelegate + репортер в Telegram |
| `ios/build_libbox.sh` | Сборка libbox (XCFramework) |

## native/SnowdenSystem/ (Swift)
| Файл | Роль |
|------|------|
| `GoCore/snowdencore.go` | Go-ядро (sing-box) с `.mod` |
| `SnowdenSystem/ContentView.swift`, `VPNManager.swift`, `Info.plist` | UI + NWVPNManager |
| `SnowdenSystemVPN/PacketTunnelProvider.swift` | Туннель |
| `project.yml`, `build.sh` | XcodeGen-конфиг и сборка |

## Поток
```
UI (swift/flutter) → VPNManager (NWVPNManager) → PacketTunnelProvider
   → networking через sing-box (GoCore) → туннель
   → TelegramReporter шлёт статус в Telegram-бот
```

## Сборка
- Flutter: как `android`, но для iOS (нужен macOS/Xcode).
- Native: `cd ios/native/SnowdenSystem && bash build.sh` (XcodeGen + роли/entitlements).

## Ограничения
- iOS требует сертификатов (entitlements `.entitlements` присутствуют).
- Релиз в App Store — вне текущего scope; раздача — через конфиг/TestFlight/адресное.