# windows/ — десктопное приложение (Wails v2 : Go + Vue3)

Основной клиент. Go-модуль лежит **в этой папке** (`go.mod`, module `snowden-system`).

## Структура

```
windows/
├── main.go app.go ...     # package main: точка входа, слой приложения
├── telegram_bot.go        # Telegram-бот (админ-панель + раздача файлов)
├── tray_windows.go        # системный трей
├── proxy_windows.go       # системный прокси Windows
├── autostart_windows.go   # автозапуск
├── crash_windows.go       # обработка аварийного завершения
├── backend/
│   ├── core/              # движок: engine, manager, adaptive, metrics, classifier...
│   ├── config/            # сборка sing-box конфига
│   ├── cfclient/          # клиент Cloudflare
│   └── enginetest/        # диагностика (e2e)
├── frontend/              # Vue3 + TS (components/hooks/data/ui)
├── assets/                # иконки + рабочие копии конфигов (см. ../configs)
├── build/                 # wails build-конфиг (NSIS, манифесты)
└── docs/                  # DOCUMENTATION.md, ANDROID_DOCUMENTATION.md
```

## Сборка

```bash
cd frontend && npm ci && npm run build && cd ..
go build ./...                 # компилирует Go + вшитый фронтенд
# для полноценного exe — Wails:
wails build -skipbindings -s -tags "with_awg,with_wireguard,with_utls,with_gvisor"
```

## Конфигурация

- Серверы/конфиг sing-box читаются из `assets/configs/` (рабочие копии).
- Источник истины — `../configs/`; применяй `bash ../configs/sync-to-windows.sh`.