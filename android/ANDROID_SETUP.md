# Android: локальная сборка и ADB-приёмка

> Актуально после **Phase 1** завершения (`efa0b9a feat(android): port
> Kotlin wrappers to libbox v1.14.0-lx.3 new API`). Build pipeline
> собирает релизный APK (≈ 230 МБ fat), APK устанавливается на
> ADB-устройство, MainActivity стартует, процесс живёт.
>
> «Фаза 2» — живая проверка **через CommandClient** протокол и
> работающий VPN-туннель — это отдельный этап, описан в `STRUCTURE.md`.

## Состояние toolchain (что лежит на этой машине)

| Компонент | Где | Версия |
|---|---|---|
| Flutter SDK | `D:\flutter\bin\flutter.bat` | **3.47.1** stable, Dart 3.13.1 |
| Junction для Gradle | `D:\flutter_link` → `D:\flutter` | на него ссылается `android/android/settings.gradle.kts` |
| JDK | `C:\Program Files\Eclipse Adoptium\jdk-17.0.19.10-hotspot` | Temurin 17.0.19+10 |
| Gradle wrapper | `android/android/gradle/wrapper/gradle-wrapper.properties` | **8.14.1** |
| AGP | `android/android/settings.gradle.kts` (`com.android.application`) | **8.11.1** |
| Kotlin | `android/android/settings.gradle.kts` (`org.jetbrains.kotlin.android`) | **2.2.20** |
| Android SDK | `C:\Users\Пользо\Android\Sdk` | platform 36, build-tools 34.0.0 |
| ADB | `C:\Users\Пользо\Android\Sdk\platform-tools\adb.exe` (рабочий) `C:\SDK\platform-tools\adb_unused.exe` (отключён) | один источник истины |
| libbox AAR | `android/android/app/libs/libbox.aar` | **v1.14.0-lx.3** |

## 1. Подготовить локальный профиль

В репозитории нет IP, UUID, паролей и Telegram-токенов. Локальный
профиль создаётся из безопасного шаблона и **не должен** попадать в
git или в чат.

```powershell
cd D:\snowden-v2\android
Copy-Item .\config.example.json .\config.local.json
notepad .\config.local.json
#  подставь SNOWDEN_VPS_IP, SNOWDEN_HY2_PASSWORD, SNOWDEN_VPN_DOMAIN
```

`lib/main.dart` получает значения только через
`--dart-define-from-file=config.local.json`. Без профиля кнопка
подключения намеренно откажет вместо старта с пустыми полями.

## 2. Проверить Flutter toolchain

```powershell
# один раз после установки SDK; проверить что 3.47.1 видит JDK 17
flutter config --jdk-dir='C:\Program Files\Eclipse Adoptium\jdk-17.0.19.10-hotspot'
flutter doctor -v
flutter pub get
Test-Path .\android\app\libs\libbox.aar   # должен быть True, размер ~98 MB
```

Если `D:\flutter_link` нет — создать junction:

```powershell
New-Item -ItemType Junction -Path 'D:\flutter_link' -Target 'D:\flutter'
```

## 3. Сборка APK

```powershell
cd D:\snowden-v2\android
flutter build apk --release --dart-define-from-file=config.local.json
```

Ожидаемый результат:

```
√ Built build\app\outputs\flutter-apk\app-release.apk (≈ 230 MB)
```

Все промежуточные warning'и про «Gradle 8.14→9.1, AGP 8.11→9.0,
Kotlin 2.2→2.3 will soon be dropped» — это ознакомительные. APK
собирается без понижения версий.

### Что важно знать про toolchain

- **TLS-MITM workaround**: в `android/android/gradle.properties`
  `org.gradle.jvmargs` форсирует TLSv1.2/TLSv1.3 на HTTPS до Maven
  repos. Без него Gradle получает `Remote host terminated the
  handshake` на `plugins.gradle.org`/`repo.maven.apache.org`. Это
  типичный симптом Касперского/CryptoPro CSP/корпоративного
  SSL-фильтра. На отечественных Maven-зеркалах не блокируется.
- **Lint workaround**: в `android/android/app/build.gradle.kts`
  `lint { checkReleaseBuilds = false }` отключает `lintVitalRelease`,
  иначе Gradle пытается скачать `com.android.tools.lint:lint-checks`
  с `dl.google.com` и снова упирается в TLS-MITM (Google Maven не
  затрагивается). Линт не запускается в release-build,фиксируется
  в Phase 2.

## 4. Установка через ADB

```powershell
adb devices
# Если предыдущая версия с versionCode > 1, сначала uninstall:
adb uninstall com.snowden.system.snowden_android
adb install -r .\build\app\outputs\flutter-apk\app-release.apk
adb shell pm clear com.snowden.system.snowden_android   # стереть state
adb logcat -c
adb shell am start -n com.snowden.system.snowden_android/.MainActivity
```

### Что должно наблюдаться в Phase 1

1. `adb install -r ...` → `Success`.
2. `pm list packages com.snowden.system.snowden_android` →
   `package:com.snowden.system.snowden_android`.
3. `am start ...` → `Starting: Intent { cmp=com.snowden.system.snowden_android/.MainActivity }`.
4. На телефоне появляется **пустой** чёрный экран с заголовком
   «snowden.system», статус с конфигом (placeholder).
5. `adb shell pidof com.snowden.system.snowden_android` →
   непустой PID; `logcat -s AndroidRuntime:E` пуст.

### Phase 1 — что ещё НЕ работает (намеренно)

- Кнопка «ПОДКЛЮЧИТЬ» вызовет `VpnService.ACTION_START` и SnowdenVpnService:
  1. `Libbox.setup(SetupOptions)` — глобальный setup, paths, OOM, log limits.
  2. `Libbox.newCommandServer(handler, platform)` — host command protocol listener.
  3. `commandServer.start()` — стартует TCP-listener на случайном порту.
  4. `commandServer.startOrReloadService(configJson, override)` —
     стартует VPN-сервис с переданным профилем.
- На стороне libbox появится foreground notification при успешном
  старте туннеля, в `logcat -s SnowdenVpn:V` пойдут строки от
  `writeDebugMessage` и `serviceReload`/`serviceStop` callbacks.
- Если serviceStart бросает исключение (например, неверный JSON-
  конфиг или libbox-rejected DNS-сервер), в `logcat -s SnowdenVpn:E`
  появится `startVpn failed ...`. UI остаётся с placeholder-конфигом.

### Phase 2 — что нужно доделать

- **CommandClient и command protocol.** Сейчас `commandServer.start()`
  слушает TCP-порт, но никто к нему не подключается. После Phase 2
  нативный или Flutter-side клиент откроет TCP-сокет, передаст
  secret и выдаст `serviceReload`/`selectOutbound` команды, чтобы
  UI мог менять outbound в живом режиме. Это требует реализации
  `CommandClientHandler` (callbacks для logs/clash-mode/groups и
  т.д.) и проброса состояния в Flutter UI через `MethodChannel`.
- **Adaptive controller и ChannelMemory** — на Android-слое
  отсутствуют, Windows MVP остаётся эталоном.
- **Remote real-VPS failover** — это фаза 3 плана, не зависит
  от Android-сборки и может двигаться параллельно.
- **Live ADB acceptance** — после Phase 2 (когда CommandClient
  работает) проверить что трафик реально проходит через TUN
  и нет утечки DNS.

## Ограничения

- `config.local.json`, релизный APK и локальный AAR **не**
  отправлять в git или чат.
- Debug signing в Gradle — только для локальной приёмки;
  production требует отдельного keystore (нужно добавить
  `key.properties` + `keystore.jks`, игнорируемые через
  `android/android/.gitignore`).
- Android и Windows используют **разные** бинари libbox:
  `1.14.0-lx.3` и `1.14.0-lx.2` соответственно. Live-приёмка
  Android ≠ Windows MVP, поведение конфигурации может отличаться.
