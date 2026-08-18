# Android — инструкция

## Вариант 1: sing-box из Play Store (быстро)

1. Установить **sing-box** из Google Play
2. Скачать `sing-box-config.json`
3. Заменить `YOUR_VPS_IP`, `YOUR_UUID`, `YOUR_DOMAIN`
4. Открыть файл в sing-box → Connect

## Вариант 2: Flutter приложение (полноценное)

### Требования
- Flutter SDK
- JDK 17
- Android SDK
- libbox.aar (sing-box Android library)

### Получить libbox.aar
Скачать из релизов sing-box-lx или собрать:
```bash
go install golang.org/x/mobile/cmd/gomobile@latest
gomobile init
gomobile bind -target android -o Libbox.aar \
  -tags "with_utls,with_wireguard,with_gvisor" \
  github.com/sagernet/sing-box/experimental/libbox
```

### Создать проект
```bash
flutter create --platforms android --org com.example.myvpn myvpn
```

### Структура
```
lib/main.dart           — Flutter UI (кнопка ВКЛ + логи)
android/app/libs/
  libbox.aar            — sing-box библиотека
android/app/src/main/kotlin/.../
  MainActivity.kt       — MethodChannel bridge
  VpnService.kt         — VpnService + PlatformInterface
  TileService.kt        — Quick Settings Tile
```

### Ключевые моменты Android
1. **PlatformInterface.openTun()** — создаёт TUN через VpnService.Builder
2. **autoDetectInterfaceControl(fd)** — вызывает `service.protect(fd)` для предотвращения петель
3. **getInterfaces()** — перечисление через `java.net.NetworkInterface` с prefix length
4. **addDisallowedApplication** — исключение банковских приложений
5. **setFixAndroidStack(true)** — обязательно для libbox на Android
6. **CommandClient(CommandLog)** — стриминг логов sing-box в UI

### Банковские исключения
Список пакетов для `addDisallowedApplication`:
```
ru.sberbankmobile, com.idamob.tinkoff.android, ru.vtbmobile.app,
ru.alfabank.mobile.android, ru.ozon.app.android, com.wildberries.ru,
com.vk.im, com.yandex.searchapp, ru.yandex.taxi, ru.mts.app, ...
```
Проверять реальные имена: `adb shell pm list packages`

### Сборка
```bash
flutter build apk --release
```

## Вариант 3: Amnezia VPN (WARP)

Скачать Amnezia VPN → импорт `warp-config-template.conf` → Connect.
WARP даёт российский IP — только для обхода РКН, не для зарубежных сервисов.
