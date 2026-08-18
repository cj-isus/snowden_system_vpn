# snowden.system v2

Адаптивная VPN-система для обхода блокировок в РФ: десктоп-приложение
(Windows), мобильный клиент (Android/iOS) и инфраструктура раздачи
(Cloudflare, телеграм-бот, VPS).

> Цель: «нажал кнопку — и работает при любых блокировках» (ЧС, БС, DPI, шейпинг).

## Структура (карта для быстрого ориентира)

```
snowden-v2/
├── configs/          # ЕДИНЫЙ источник всех конфигов (env, sing-box, cloudflare, лендинг, шаблоны)
│   ├── env/          #   .env с токенами
│   ├── singbox/      #   конфиги sing-box (серверы, WARP, split-tunnel)
│   ├── cloudflare/   #   worker.js, wrangler.toml, schema
│   ├── landing/      #   лендинг + version.json (раздача)
│   └── templates/    #   шаблоны для распространения (pc/android/ios/landing/bot/vps)
├── windows/          # Десктоп: Wails v2 (Go-бэкенд) + Vue3 (frontend)
│   ├── *.go          #   точка входа и приложение (package main)
│   ├── backend/      #   Go-пакеты: core (движок), config, cfclient, enginetest
│   ├── frontend/     #   Vue3: components/ со своими hooks/data/ui
│   ├── assets/       #   иконки + рабочие копии конфигов (читаются приложением)
│   ├── build/        #   wails build-конфиг (NSIS-установщик, манифесты)
│   └── docs/         #   документация приложения
├── android/          # Мобильное: Flutter (sing-box embedded) + нативный Android (Kotlin/VPNService)
│   ├── lib/          #   Dart-код
│   └── ios/          #   Flutter-вариант для iOS
├── docs/             # Общая документация, отчёты, дизайн-макеты
└── scripts/          # Python-инструменты (диагностика, deploy) + vps-deploy/
```

## Сборка

### Windows-десктоп
```bash
cd windows
# фронтенд
cd frontend && npm ci && npm run build && cd ..
# бэкенд (одним махом компилирует и Go, и встроенный Vue-бандл)
go build ./...
# либо полная сборка Wails (с нужными build-tags)
wails build -skipbindings -s -tags "with_awg,with_wireguard,with_utls,with_gvisor"
```

### Android
```bash
cd android && build_android.bat   # или flutter build apk
```

## Как связаны части

- **Windows-приложение** грузит конфиг серверов из `windows/assets/configs/`
  (источник истины — `configs/singbox/`, см. `configs/sync-to-windows.sh`).
- **Телеграм-бот** (в `windows/telegram_bot.go`) раздаёт файлы пользователям:
  качает их с VPS по `SNOWDEN_FILE_URL=http://IP:8090` и пересылает в Telegram.
- **Лендинг + файлы** раздаются с VPS (`configs/landing/` → `scripts/vps-deploy/`).
- **Cloudflare worker** (`configs/cloudflare/`) — API статуса и раздача обновлений.

Подробная архитектура и потоки данных — в [STRUCTURE.md](STRUCTURE.md).
Рекомендации для ИИ-ассистента — в [AGENTS.md](AGENTS.md).