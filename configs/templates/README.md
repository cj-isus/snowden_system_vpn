# snowden-template — VPN система для развёртывания

Полная копия технологии snowden.system. Без данных — заполняй своими.

## Что входит

```
snowden-template/
├── README.md                ← эта инструкция
├── vps/
│   ├── setup.sh             ← скрипт настройки VPS (sing-box + BBR + nginx)
│   ├── sing-box-server.json ← шаблон серверного конфига
│   └── VPS_INSTRUCTIONS.md  ← пошаговая инструкция
├── pc/
│   ├── config-template.json ← sing-box клиентский конфиг (ПК)
│   └── PC_INSTRUCTIONS.md   ← инструкция сборки ПК приложения
├── android/
│   ├── sing-box-config.json ← конфиг для официального sing-box
│   └── ANDROID_INSTRUCTIONS.md
├── ios/
│   ├── sing-box-config.json ← конфиг для Karing / sing-box iOS
│   └── IOS_INSTRUCTIONS.md
├── landing/
│   ├── index.html           ← шаблон лендинга
│   └── version.json         ← шаблон version.json
├── telegram-bot/
│   ├── bot-template.go      ← бот с админ-панелью и логами
│   └── BOT_INSTRUCTIONS.md
└── amnezia/
    └── warp-config-template.conf ← шаблон WARP (AmneziaWG)
```

## Быстрый старт (5 шагов)

### 1. Купить VPS
- Любой хостинг (Hetzner, AdminVPS, Aeza)
- 1 ядро, 2GB RAM, Ubuntu 24.04
- Стоимость: ~200-400₽/мес

### 2. Настроить VPS
```bash
scp vps/setup.sh root@ТВОЙ_IP:/root/
ssh root@ТВОЙ_IP
bash setup.sh
```
Скрипт сам установит sing-box, BBR, получит сертификат, откроет порты.

### 3. Создать Telegram бота
- Написать @BotFather → /newbot → получить токен
- Узнать свой chat_id: @userinfobot

### 4. Заполнить конфиги
Во всех файлах заменить:
- `YOUR_VPS_IP` → IP твоего сервера
- `YOUR_UUID` → сгенерировать: `uuidgen`
- `YOUR_BOT_TOKEN` → токен от BotFather
- `YOUR_CHAT_ID` → твой Telegram ID
- `YOUR_DOMAIN` → домен (nip.io работает бесплатно), если TLS/SNI реально настроены на VPS

### 5. Развернуть
- **ПК:** собрать через `wails build`; embedded sing-box не является subprocess.
- **Android:** Flutter APK + local `config.local.json`; не вставлять credentials в Dart.
- **iOS:** отдельный Flutter/Network Extension клиент; локальная provisioning.
- **Лендинг:** Cloudflare Pages (бесплатно)

## Технология

| Компонент | Технология |
|-----------|-----------|
| VPN ядро | sing-box / sing-box-lx |
| Основной транспорт | Hysteria2 (QUIC/UDP), если он provisioned в runtime-конфиге |
| Резерв | Только реально проверенные каналы из selector `proxy` |
| Политика | AdaptiveController + selector, fail-closed; direct только для явных split-tunnel правил |
| Анти-ТСПУ | Firefox uTLS fingerprint |
| ПК приложение | Wails (Go + Vue3) |
| Android | Flutter + libbox.aar |
| iOS | sing-box / Karing + JSON конфиг |
| Бот | Telegram Bot API (Go) |
| Лендинг | Cloudflare Pages |
| Файлы | VPS nginx |

## Поддержка
- GitHub: приватный репозиторий
- Cloudflare: бесплатный тариф
- VPS: ~200-400₽/мес
- Итого: ~500₽/мес за полноценную VPN систему
