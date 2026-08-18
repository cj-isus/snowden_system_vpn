# STRUCTURE.md — android/

> Мобильный клиент: Flutter (UI) + нативный Android (VPNService/Kotlin) +
> sing-box (`libbox`) для туннеля. Цель — переиспользовать ту же sing-box-логику.

## Состав
| Папка/Файл | Роль |
|-----------|------|
| `lib/main.dart` | Flutter-UI: экран «ВКЛ/ВЫКЛ», статус, тёмная тема (#3CFF5A) |
| `android/` | Нативный проект Gradle (Kotlin) |
| `android/app/src/main/kotlin/com/snowden/system/snowden_android/` | Kotlin-код |
| `android/app/libs/libbox.aar` | sing-box-библиотека (embedded) |
| `android/app/build.gradle.kts`, `build.gradle.kts`, `settings.gradle.kts` | Сборка |
| `test/`, `pubspec.yaml`, `analysis_options.yaml` | Flutter-метаданные |
| `build_android.bat` | Скрипт сборки APK |
| `ANDROID_SETUP.md`, `README.md` | Документация |

## Ключевой Kotlin (`android/app/src/main/kotlin/.../snowden_android/`)
| Файл | Роль |
|------|------|
| `MainActivity.kt` | Activity + MethodChannel |
| `SnowdenPlatformInterface.kt` | Мост Flutter ↔ нативный |
| `SnowdenVpnService.kt` | `VPNService` — поднятие TUN + проброс в sing-box |

## Поток
```
Flutter (main.dart) ──MethodChannel("com.snowden.system/vpn")──► MainActivity
   ▲                                                            └─► SnowdenVpnService (VPNService + libbox)
   └─► статус/state <──────────────────────────────────────────────┘
```
- Конфиг sing-box формируется в `main.dart` (TUN inbound) и передаётся нативу.

## Сборка
```bash
cd android
flutter pub get
flutter build apk --release   # или build_android.bat
```
(см. `configs/README.md` — нужен рабочий sing-box-конфиг или `SNOWDEN_FILE_URL`).

## Статус/ограничения
- APK-релиз с GitHub ограничен 100 МБ — собирается локально (см. STRUCTURE.md корня).