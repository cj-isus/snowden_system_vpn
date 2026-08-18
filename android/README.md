# android/ — мобильный клиент (Flutter + нативный VPNService)

Клиент для Android (и iOS-вариант в `ios/`). Flutter-код (`lib/`) + нативный
слой Kotlin (`android/app/src/main/kotlin/`): `MainActivity`,
`SnowdenPlatformInterface`, `SnowdenVpnService` (VPNService поверх sing-box).

## Структура

```
android/
├── pubspec.yaml            # Flutter-зависимости
├── lib/
│   ├── main.dart           # точка входа + UI
│   └── ...                 # (при необходимости: features/<фича>/ c hooks/data/ui)
├── android/                # нативный Gradle-проект + Kotlin/VPNService
│   └── app/libs/libbox.aar # предсобранная sing-box для Android
├── ios/                    # Flutter-вариант для iOS
├── build_android.bat       # сборка APK
└── test/                   # тесты
```

## Сборка

```bash
# Flutter CLI
flutter pub get
flutter build apk --release

# или через батник
build_android.bat
```

## Примечания

- `android/local.properties` (пути SDK) генерируется локально и не хранится.
- Нативный слой `SnowdenVpnService.kt` поднимает VPNService; `libbox.aar` —
  sing-box для Android.