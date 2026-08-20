# Android: локальная сборка и ADB-приёмка

## Что уже проверено

- ADB устройство `AXUG024C16005964` было авторизовано и перешло в статус
  `device`.
- Корень Android-проекта: `D:\snowden-v2\android`.
- JDK 17 установлен на машине разработчика; Flutter SDK и AAR ещё требуют
  локальной проверки.

## 1. Подготовить локальный профиль

Репозиторий не содержит IP, UUID или паролей Android-клиента. Создай локальный
файл из безопасного примера:

```powershell
cd D:\snowden-v2\android
Copy-Item .\config.example.json .\config.local.json
# заполни config.local.json локально; не отправляй его в чат и не коммить
```

`lib/main.dart` получает значения только через `--dart-define-from-file`; без
профиля кнопка подключения намеренно откажет вместо запуска неполного конфига.

## 2. Проверить Flutter toolchain

```powershell
flutter doctor -v
flutter pub get
Test-Path .\android\app\libs\libbox.aar
```

Ожидаемый AAR — `sing-box-lx v1.14.0-lx.3`. Windows использует другой pinned
runtime (`v1.14.0-lx.2`), поэтому Android и Windows не считаются одним бинарным
runtime до live-приёмки.

## 3. Сборка APK

```powershell
cd D:\snowden-v2\android
flutter build apk --release --dart-define-from-file=config.local.json
```

Либо:

```powershell
.\build_android.bat
```

Скрипт использует `%~dp0`, а не устаревший абсолютный путь. Если AAR отсутствует,
он предложит скачать зафиксированный файл `libbox-1.14.0-lx.3.aar` в
`android/app/libs/`.

## 4. Установка и проверка через ADB

```powershell
adb devices
adb install -r .\build\app\outputs\flutter-apk\app-release.apk
adb shell pm clear com.snowden.system.snowden_android
adb logcat -c
adb shell am start -n com.snowden.system.snowden_android/.MainActivity
```

Проверка:

1. Нажать «ПОДКЛЮЧИТЬ».
2. Разрешить Android VPN.
3. Убедиться, что появляется foreground notification.
4. Выполнить `adb logcat | Select-String -Pattern "Snowden|libbox|VpnService"`.
5. Проверить внешний HTTPS-запрос и отсутствие перехода protected traffic в
   `direct`.
6. Нажать «ВЫКЛЮЧИТЬ» и убедиться, что notification/TUN исчезли.

## Ограничения

- `config.local.json`, APK и AAR не отправлять в git или чат.
- Debug signing в Gradle — только для локальной приёмки; production требует
  отдельного keystore.
- Android-слой пока не имеет полноценного adaptive controller и ChannelMemory,
  поэтому Windows MVP является эталоном поведения.
