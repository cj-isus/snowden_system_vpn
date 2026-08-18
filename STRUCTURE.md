# Архитектура snowden.system

Документ описывает компоненты, их связи и потоки данных. Предназначен и для
человека, и для ИИ-ассистента как первая точка входа.

## 1. Компоненты верхнего уровня

| Компонент | Путь | Технологии | Роль |
|---|---|---|---|
| Десктоп | `windows/` | Go + Wails v2 + Vue3/TS | Основной клиент: одна кнопка «ВКЛ», адаптивное переключение серверов, дашборд |
| Core-движок | `windows/backend/core/` | Go (package `core`) | Управление sing-box (subprocess), адаптивный failover, метрики, классификатор ошибок, Circuit Breaker |
| Конфиг-билдер | `windows/backend/config/` | Go (package `config`) | Сборка sing-box JSON-конфига, подстановка серверов |
| CF-клиент | `windows/backend/cfclient/` | Go | Клиент к Cloudflare (KV/обновления) |
| Frontend | `windows/frontend/src/` | Vue3 + TS | UI: дашборд, статус, серверы, логи, трафик |
| Telegram-бот | `windows/telegram_bot.go` | Go | Админ-панель + раздача файлов пользователям |
| Мобильное | `android/` | Flutter + Kotlin/VPNService | Клиент для Android/iOS |
| Конфиги | `configs/` | JSON/conf | Точка истины для всех настроек |
| Раздача | `configs/cloudflare/` + `scripts/vps-deploy/` | Cloudflare Workers + статик-сервер | Лендинг и файлы обновлений |

## 2. Поток данных в десктопе (windows/)

```
Vue frontend ──wailsjs bindings──► Go app (app.go)
   ▲                                    │
   │  EventsOn/EventsOff (webview)      │ Calls
   │                                    ▼
   └──── события (logs, traffic, diag) ◄── backend/core (engine, manager, adaptive)
                                            │  управляет
                                            ▼
                                       sing-box.exe (subprocess, Clash API)
                                            │  поднимает
                                            ▼
                                       VPN-туннель + системный прокси
```

- **Frontend** вызывает методы Go через сгенерированные привязки
  `frontend/wailsjs/` (не редактировать — генерятся командой `wails generate module`).
- **`app.go`** — слой приложения: старт/стоп VPN, загрузка конфига
  (`LoadConfigFile` из `assets/configs/`), проброс событий в UI, Telegram.
- **`backend/core`** — сердце системы:
  - `engine.go` — запуск/останов sing-box субпроцесса;
  - `manager.go` — `StartVPN/StopVPN/ReloadVPN` + состояние;
  - `adaptive.go` — цикл мониторинга и переключения;
  - `metrics.go` — EWMA-латентность, потери, success rate;
  - `classifier.go` — классификация ошибок (server_down, DPI и т.п.);
  - `domain_stats.go` — статистика по доменам;
  - `registry.go` — реестр серверов.

## 3. Главные компоненты Vue (компонентная разбивка)

```
frontend/src/
├── components/            # @feature/<компонент>/
│   ├── Dashboard/         # Status, Traffic, Diagnostics, Events, DomainStats, MatrixRain
│   ├── Servers/           # ServersCard, RoutingCard
│   ├── Layout/            # Sidebar, TopBar, TerminalBar (+ ui/MatrixRain)
│   └── Settings/          # SettingsCard
├── hooks/                 # общие composables (используются фичами)
├── data/                  # клиенты Wails API, Pinia-хранилища
└── ui/                    # переиспользуемые базовые UI-компоненты
```

Правило: **каждый главный компонент — папка** со своими подразделами
`hooks/` (логика), `data/` (запросы/хранилище), `ui/` (вложенные мелкие
компоненты), `components/` (подкомпоненты). Вынос логики в `hooks/` делает
код читаемым и переиспользуемым.

## 4. Конфигурация (configs/)

```
configs/
├── env/       # токены Telegram и пр. (секреты)
├── singbox/   # template-vps-reality.json (серверы), template-warp-awg.json (WARP),
│              # server-params.json, warp-keys.json, ru-cidr.lst (split-tunnel)
├── cloudflare/ # worker.js (API), r2-worker.js (раздача), wrangler.toml, schema.sql
├── landing/   # index.html + version.json + отвечающие за скачивание файлы
└── templates/ # сантья-шаблоны для публикации (платформы)
```

> Приложение читает конфиги из `windows/assets/configs/` — это **рабочие копии**.
> Редактируй `configs/singbox/`, затем применяй скриптом `configs/sync-to-windows.sh`.

## 5. Раздача (работает в РФ без домена)

- Файлы и лендинг хостятся на **VPS** (`http://<IP>:8090`), см. `scripts/vps-deploy/`.
- **Telegram-бот** качает файл с VPS (`SNOWDEN_FILE_URL`) и пересылает его в
  Telegram пользователю — файлы доходят через Telegram, а не с собственного IP.
- Cloudflare worker — для API статуса и обновлений (не обязателен для РФ-Direct).

## 6. Известные ограничения

- Clash API на Windows sing-box-lx не работает → счётчик трафика возвращает 0.
- Reality handshake на lx-форке багованный → используется VLESS+TLS / WARP.
- APK-сборка Android пока отсутствует (лимит GitHub 100 МБ; собирать локально).