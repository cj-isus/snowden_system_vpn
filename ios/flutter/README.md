# snowden.system iOS

> privacy is a human right

Кроссплатформенный VPN-клиент для обхода блокировок. iOS-версия — точная копия
Android с адаптацией под Network Extension.

## Возможности
- ✅ VLESS+TLS через VPS (Нидерланды)
- ✅ Hysteria2 (UDP, резерв)
- ✅ Split-tunneling (РФ → direct, заблокированные → VPN)
- ✅ urltest (автовыбор протокола)
- ✅ Telegram-репортер (статус на iPhone в Telegram)
- ✅ Логи sing-box в реальном времени
- ✅ Один интерфейс — одна кнопка

## Необходимое для сборки
- macOS с Xcode 15+
- Apple Developer Account ($99/год)
- Go + gomobile (для libbox)
- Flutter SDK

## Быстрый старт
1. `cd ios && ./build_libbox.sh` — собрать libbox.xcframework
2. `flutter create --platforms=ios --org com.snowden.system .`
3. Открыть в Xcode, настроить signing
4. Run на iPhone

Подробности: [README_IOS.md](README_IOS.md)

## Сервер
```
VPS: 192.109.206.234 (Нидерланды)
VLESS: 443/TCP + TLS (Let's Encrypt)
Hysteria2: 8443/UDP
Домен: snowden-system.192-109-206-234.nip.io
```

## Лицензия
Для личного использования.
