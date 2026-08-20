# snowden.system — Полная техническая документация (ПК)

> **Версия документа:** 3.0 (21 июля 2026)
> **Версия приложения:** 1.2.5
> **Ядро:** sing-box-lx v1.14.0-lx.2 (ПК), v1.14.0-lx.3 (Android)

---

## Оглавление

1. [Обзор проекта](#1-обзор-проекта)
2. [Технологический стек и обоснование](#2-технологический-стек-и-обоснование)
3. [Архитектура](#3-архитектура)
4. [Компоненты backend](#4-компоненты-backend)
5. [Telegram-бот — админ-панель](#5-telegram-бот--админ-панель)
6. [Cloudflare Worker](#6-cloudflare-worker)
7. [Сервер (VPS) — конфигурация](#7-сервер-vps--конфигурация)
8. [Split-tunneling](#8-split-tunneling)
9. [Adaptive Engine + Circuit Breaker](#9-adaptive-engine--circuit-breaker)
10. [Per-domain статистика](#10-per-domain-статистика)
11. [Теория обхода блокировок](#11-теория-обхода-блокировок)
12. [Frontend (Vue3) — UI компоненты](#12-frontend-vue3--ui-компоненты)
13. [Что НЕ работает и почему](#13-что-не-работает-и-почему)
14. [Сборка и развёртывание](#14-сборка-и-развёртывание)
15. [Структура проекта](#15-структура-проекта)
16. [Ключевые уроки](#16-ключевые-уроки)

---

## 1. Обзор проекта

**snowden.system** — кроссплатформенная адаптивная VPN-система для обхода блокировок
в России. ПК-версия (Wails+Go+Vue3) + Android (Flutter+libbox).

### Что делает
- «Нажал кнопку — работает» для обычных пользователей
- Адаптивное восстановление при сбоях (Circuit Breaker)
- Раздельное туннелирование (РФ-сайты напрямую, заблокированные через VPN)
- Telegram-бот для удалённого мониторинга и управления
- Cloudflare Worker для динамических конфигов и health-check

### Параметры
| Параметр | Значение |
|---|---|
| Платформа | Windows 10/11 x64 + Android (Flutter) |
| Размер ПК бинарника | ~35 МБ |
| Ядро | sing-box-lx v1.14.0-lx.2 (embedded) |
| Протокол | VLESS+TLS (основной), Hysteria2 (резерв) |
| Сертификат | Let's Encrypt (через nip.io) |
| VPS | YOUR_VPS_IP (Нидерланды), BBR, 600 Мбит/с |
| Скорость через туннель | ~40 Мбит/с стабильно |

---

## 2. Технологический стек и обоснование

### sing-box-lx вместо upstream sing-box

| Вариант | Плюсы | Минусы | Вердикт |
|---|---|---|---|
| **sing-box-lx (Leadaxe)** | AmneziaWG 2.0, MASQUE, XHTTP, CommandClient; thin fork | Reality handshake нестабилен | ✅ выбран |
| upstream sing-box | Стабильный | Нет AWG/MASQUE в stable | ❌ |
| mihomo (Clash.Meta) | Лучший rule-engine | Нет embedded Go-API | ❌ |
| Xray-core | Лучший Reality | Нет Hysteria2 | ❌ |

**Ключевое:** sing-box встроен как Go-библиотека (`box.New().Start()/Close()`), а не
subprocess. Единственный способ получить graceful shutdown на Windows без консоли.

### Wails v2 (Go backend + Vue3 frontend)

Go-backend позволяет `import sing-box` и вызывать `box.New().Start()` в том же
процессе — без subprocess, без IPC, без overhead.

### Build tags

```
with_awg          — AmneziaWG 2.0
with_wireguard    — WireGuard (WARP fallback)
with_utls         — uTLS fingerprinting
with_gvisor       — userspace TCP/IP stack
```

**Важно:** `wails build` игнорирует `tags` из `wails.json` — передавайте через CLI:
```bash
wails build -s -tags "with_awg,with_wireguard,with_utls,with_gvisor"
```

---

## 3. Архитектура

```
┌──────────────────────────────────────────────────────────────┐
│                    FRONTEND (Vue3 + TypeScript)               │
│  Dashboard: Status, Servers, Routing, Traffic, Diagnostics,   │
│  Events, DomainStats, Terminal (live logs), Settings          │
└────────────────────────┬─────────────────────────────────────┘
                         │ Wails bindings (13 методов)
┌────────────────────────▼─────────────────────────────────────┐
│                    BACKEND (Go, Wails v2)                     │
│                                                               │
│  ┌──────────┐  ┌──────────────┐  ┌────────────────────────┐  │
│  │ Manager  │  │ AdaptiveEng  │  │ ErrorClassifier        │  │
│  │ +Metrics │  │ (CB+probe)   │  │ (парсер логов)         │  │
│  │ +Domain  │  └──────┬───────┘  └────────────────────────┘  │
│  │  Stats   │         │                                       │
│  └────┬─────┘    ┌────▼───────────────────────────────────┐  │
│       │          │  TelegramLogger (admin panel + reports) │  │
│  ┌────▼──────────┴────────────────────────────────────────┐ │
│  │              Engine (box.Box lifecycle)                 │ │
│  │  Start / Close / Reload + PlatformLogWriter             │ │
│  └─────────────────────────────────────────────────────────┘ │
│                                                               │
│  ┌─────────────────┐  ┌──────────────┐  ┌─────────────────┐  │
│  │ System Proxy    │  │ Autostart    │  │ CF Client       │  │
│  │ (Win32 registry)│  │ (HKCU\Run)   │  │ (Worker API)    │  │
│  └─────────────────┘  └──────────────┘  └─────────────────┘  │
└───────────────────────────────────────────────────────────────┘
          │
┌─────────▼─────────────────────────────────────────────────────┐
│   sing-box-lx (embedded)                                       │
│   mixed-in(127.0.0.1:20808) → route rules → urltest[auto]     │
│   VLESS+TLS → VPS (YOUR_VPS_IP:443)                       │
│   Hysteria2 → VPS (YOUR_VPS_IP:8443/UDP)                  │
└───────────────────────────────────────────────────────────────┘
```

### Поток трафика

```
Браузер (системный прокси 127.0.0.1:20808)
    ↓
sing-box mixed-in
    ↓ route rules
    ├── .ru, банки, Госуслуги → DIRECT (быстро, без VPN)
    ├── youtube.com, telegram → urltest[auto]
    │       ├── VLESS+TLS → VPS (основной, TCP)
    │       └── Hysteria2 → VPS (UDP, режется оператором)
    └── остальное → urltest[auto] (по умолчанию через VPN)
```

---

## 4. Компоненты backend

### 4.1. Engine (`backend/core/engine.go`)

Обёртка над `box.Box` (sing-box). Живёт в **том же процессе**.

```go
instance, err := box.New(box.Options{
    Context:           ctx,
    Options:           options,
    PlatformLogWriter: platformWriter,
})
instance.Start()  // блокирует до полного старта
instance.Close()  // graceful shutdown
```

**PlatformLogWriter** перехватывает КАЖДУЮ строку лога sing-box:
- → `Wails EventsEmit("engine:log")` → UI (TerminalBar)
- → `ErrorClassifier.OnLog()` → классификация ошибок
- → `TelegramLogger.PushLog()` → буфер для отчётов
- → `extractDomainFromLog()` → DomainStats

### 4.2. Manager (`backend/core/manager.go`)

Фасад между Wails и Engine. Хранит:
- `activeConfigJSON` — последний конфиг (для toggle rules, export, reconnect)
- `metrics *Metrics` — счётчики трафика
- `domainStats *DomainStatsRegistry` — per-domain статистика

**metricsLoop()** — фоновая горутина (каждую секунду):
- `metrics.sample()` — сэмплирует трафик
- `PollConnections()` — читает live-соединения для DomainStats

### 4.3. Error Classifier (`backend/core/classifier.go`)

Парсер логов sing-box в реальном времени. Реагирует только на `[error]` и `[warn]`.

| Категория | Паттерн | Объяснение |
|---|---|---|
| `network_down` | proxy connection refused | Нет интернета |
| `server_down` | `dial tcp ... i/o timeout` | Сервер не отвечает |
| `dns_failure` | `lookup failed` | Ошибка DNS |
| `tls_failure` | `TLS handshake ... timeout` | TLS заблокирован |
| `server_blocked` | `connection reset` on outbound | ТСПУ/DPI блокировка |
| `degraded` | TTFB > 2с | Медленный туннель |

### 4.4. Metrics (`backend/core/metrics.go`)

Счётчики трафика для TrafficCard. **Clash API недоступен** на Windows sing-box-lx
(см. раздел 13), поэтому `readClashTraffic()` безопасно возвращает (0,0) если
endpoint не отвечает.

**PingServer()** — TCP connect latency к VPS (для ServersCard).
**ProbeLatencyThroughProxy()** — HTTP-проба через туннель (для StatusCard).

### 4.5. System Proxy (`proxy_windows.go`)

Управление через Windows registry (HKCU) + `InternetSetOption`:
```go
SetProxyEnable = 1
SetProxyServer = "127.0.0.1:20808"
InternetSetOption(INTERNET_OPTION_SETTINGS_CHANGED)
```

### 4.6. Autostart (`autostart_windows.go`)

`HKCU\Software\Microsoft\Windows\CurrentVersion\Run` → `"SnowdenSystem" = path`.
Не требует прав администратора.

### 4.7. CF Client (`backend/cfclient/client.go`)

Клиент Cloudflare Worker. Методы:
- `FetchHealth()` — VPS health-check через CF edge
- `FetchVersion()` — проверка обновлений
- `FetchConfig()` — динамический конфиг из KV

Подключён в `app.go` через `GetRemoteHealth()` и `GetRemoteVersion()`.

---

## 5. Telegram-бот — админ-панель

### Архитектура (`telegram_bot.go`)

Две горутины:
1. **loop()** — проверяет состояние каждую минуту. Отчёт отправляется ТОЛЬКО при
   ошибках (не чаще раза в час).
2. **commandLoop()** — `getUpdates` polling каждые 3 сек — слушает команды.

### Inline-кнопки (админ-панель)

Под каждым отчётом — 6 кнопок:

| Кнопка | Callback | Действие |
|--------|----------|----------|
| 📊 Статус | `status` | `sendStatus()` — state + circuit + category |
| 🌐 Серверы | `servers` | `sendServers()` — список + live ping |
| ⏹/▶ Стоп/Старт | `toggle` | `manager.StopVPN()/StartVPN()` |
| 🔄 Переподключить | `reconnect` | `manager.ReloadVPN()` |
| 📈 Трафик | `traffic` | Live bytes + uptime |
| 🩺 Диагностика | `diag` | Circuit breaker snapshot |

### Команды
```
/status     — статус VPN
/servers    — список серверов с пингом
/traffic    — трафик сессии
/reconnect  — переподключить
/help       — справка
```

### Маршрутизация через sing-box прокси

`api.telegram.org` заблокирован в РФ. Бот использует `transport.Proxy = http.ProxyURL("127.0.0.1:20808")`
когда VPN запущен. Proxy + DialContext (tcp4) — DialContext подключается к прокси,
прокси делает CONNECT к Telegram.

### Логика отчётов

```
VPN работает нормально → МОЛЧИТ
Ошибка (server_down и т.д.) → присылает отчёт ⚠️
Ошибка продолжается → МОЛЧИТ (не чаще раза в час)
Запуск приложения → "🟢 запущен"
Закрытие приложения → "🔴 остановлен"
Команда/кнопка от пользователя → отвечает мгновенно
```

### Пометка «от кого»

Каждое сообщение содержит `hostname` ПК: `🖥 snowden.system — отчёт с MYPC`.

---

## 6. Cloudflare Worker

### Endpoints (`cloudflare/worker.js`)

| Endpoint | Метод | Что делает |
|----------|-------|------------|
| `/api/config` | GET | Конфиг серверов из KV (SNOWDEN_CONFIG) |
| `/api/health` | GET | Health-check VPS (TCP:443 + интернет) |
| `/api/version` | GET | Версия + URL обновления |
| `/api/telemetry` | POST | Сохранение телеметрии в D1 |

### KV-хранилища
- `SNOWDEN_CONFIG` — конфиг серверов (UUID, пароли)
- `SNOWDEN_VERSION` — версия приложения

### D1 database
- `snowden-telemetry` — телеметрия сессий (время, трафик, ошибки)

### Секреты
Worker использует переменные окружения (`env.VLESS_UUID`, `env.HY2_PASS`) вместо
хардкода. Устанавливаются через `wrangler secret put`.

### Интеграция с ПК
`cfclient.New()` в `app.go` → `GetRemoteHealth()` и `GetRemoteVersion()`.

---

## 7. Сервер (VPS) — конфигурация

### Параметры
| Параметр | Значение |
|---|---|
| IP | YOUR_VPS_IP |
| ОС | Ubuntu 24.04 LTS |
| CPU | 1 ядро Xeon Skylake |
| RAM | 2 GB |
| sing-box | sing-box-lx v1.14.0-lx.2 |
| Congestion control | **BBR** (был cubic) |
| VLESS | 443/tcp + TLS (Let's Encrypt) |
| Hysteria2 | 8443/udp + TLS |
| Shadowsocks | 1080 (tcp+udp) |
| Скорость VPS→CF | ~600 Мбит/с |

### BBR Congestion Control

```bash
sysctl net.ipv4.tcp_congestion_control=bbr
sysctl net.core.default_qdisc=fq
```

BBR лучше cubic для дальних линков (Россия→Нидерланды, ~76ms):
- Не реагирует на случайные потери (cubic воспринимает как перегрузку)
- Стабилизирует latency: было 0.4-2.8с → стало 0.3-0.5с

### Firewall
```
ufw allow 443/tcp    # VLESS
ufw allow 8443/udp   # Hysteria2
ufw allow 22/tcp     # SSH
ufw allow 80/tcp     # certbot
```

---

## 8. Split-tunneling

### Концепция
Российские сайты идут напрямую (мимо VPN), заблокированные — через VPN.

### Реализация
1. `ru-cidr.lst` (470 КБ, ~30K CIDR) → конвертируется в sing-box source rule-set
2. Доменные правила: `.ru`, `.su`, `.рф` + 80 конкретных доменов (банки, Госуслуги)
3. Инжекция через `injectSplitTunnel()` в `app.go`

### Почему
- Банки блокируют зарубежные IP (антифрод)
- Госуслуги не пускают с иностранных IP
- Российские сайты доступны напрямую (нет смысла тратить VPN-трафик)

### Toggle (RoutingCard)
Клик по тогглу → `ToggleRouteRule(index, enabled)`:
- Меняет `action: direct` ↔ `outbound: auto` в JSON
- `ReloadVPN()` (hot-reload без остановки)
- Toast подтверждение, revert при ошибке

---

## 9. Adaptive Engine + Circuit Breaker

### 3-state автомат

```
HEALTHY (Closed)
  │ 3 неудачных probe подряд
  ▼
FAILED (Open) ── Engine.Reload() ── не помог? ── urltest переключит
  │ cooldown (10с → 20с → 40с → 60с, экспоненциальный)
  ▼
RECOVERING (HalfOpen)
  │ 2 успешных probe подряд
  ▼
HEALTHY (Closed)
```

### Health-check (deepProbe)
- HTTP GET `http://www.gstatic.com/generate_204` через прокси
- **Принудительно IPv4** (`DialContext: tcp4`) — VPS не имеет IPv6, IPv6-провал
  приводил к ложному `server_down`
- Timeout: 10с, интервал: 30с

### Действия при FAILED
1. Классификация через ErrorClassifier
2. `network_down` → ничего не делать
3. Иначе → `Engine.Reload(config)`
4. Если не помог → ждём, urltest сам переключит

---

## 10. Per-domain статистика

### DomainStatsRegistry (`backend/core/domain_stats.go`)

Запоминает какой outbound работает лучше для каждого домена.

### Источники данных
1. **extractDomainFromLog()** — парсит `sniffed protocol: tls, domain: X` из логов
2. **PollConnections()** — читает Clash API `/connections` (если доступен):
   - `metadata.host` — домен
   - `chains[0]` — outbound (vless-tls / direct / hysteria2)
   - `upload + download` — реальные байты

### Score-формула
```
score = reliability(40%) + latency(40%) + freshness(20%)
```
- **Reliability** — success rate (0-1)
- **Latency** — EWMA (70% старое + 30% новое), 200ms=100 баллов, 2000ms=0
- **Freshness** — использовался в последние 5 мин=100, затухает к 0 за час

### UI
`DomainStatsCard.vue` — карточка «ИНТЕЛЛЕКТ СИСТЕМЫ»:
- Домен + лучший outbound + score-бар + requests + latency
- Polling каждые 5 сек через `GetDomainStats()`

---

## 11. Теория обхода блокировок

### ТСПУ (DPI)
Роскомнадзор устанавливает ТСПУ у провайдеров — Deep Packet Inspection,
анализирует трафик и блокирует по сигнатурам.

### Почему VLESS+TLS работает
VLESS оборачивает трафик в стандартный TLS handshake с настоящим Let's Encrypt
сертификатом. Для ТСПУ это выглядит как обычный HTTPS:

```
[ПК] → TLS(ClientHello, SNI=snowden-system.nip.io) → [ТСПУ] → [VPS]
                          ↑
                 ТСПУ видит обычный HTTPS.
```

### Почему НЕ Reality
Reality handshake нестабилен в sing-box-lx 1.14 (`processed invalid connection`).
Чистый VLESS+TLS с настоящим сертификатом надёжнее.

### Почему НЕ flow=vision
`xtls-rprx-vision` режется ТСПУ (детектится по паттернам). Чистый VLESS без flow
проходит как обычный TLS.

### Почему Hysteria2 режется
Hysteria2 — UDP/QUIC. Мобильные операторы в РФ режут или ограничивают UDP.
urltest автоматически выбирает VLESS (TCP).

---

## 12. Frontend (Vue3) — UI компоненты

### Dashboard карточки

| Карточка | Данные | Источник |
|----------|--------|----------|
| **StatusCard** | Статус + сервер + latency | `Status()` + `GetServers()` + `GetLatency()` |
| **ServersCard** | Серверы + TCP ping | `GetServers()` (poll 10с) |
| **RoutingCard** | Правила + тогглы | `GetRouteRules()` + `ToggleRouteRule()` |
| **TrafficCard** | Скорость + total + uptime | `GetTraffic()` (poll 1с) |
| **DiagnosticsCard** | Health-check + fail count | `Diagnostics()` + `GetLatency()` |
| **EventsCard** | Лента событий | `engine:diag` events |
| **DomainStatsCard** | Per-domain scores | `GetDomainStats()` (poll 5с) |
| **SettingsCard** | Автозапуск + импорт/экспорт | `SetAutostart()` + `ExportConfig()` |

### TerminalBar (нижняя панель)
- **Реальные логи sing-box** (через `engine:log` events)
- Цветовая подсветка: `[error]` → красный, `[warn]` → жёлтый
- Выделение мышью + копирование (user-select: text)
- Кнопка паузы (автопауза при выделении)
- Кнопка копирования (⧉)

### Sidebar навигация
- Табы скроллят к нужной карточке + подсвечивают зелёным
- «Логи» → скроллит к TerminalBar
- «О системе» → полноэкранная страница с версиями

---

## 13. Что НЕ работает и почему

### Clash API (счётчик трафика)
**Симптом:** `create clash-server: invalid argument` на Windows sing-box-lx.
**Причина:** Clash server требует `trafficManager` из контекста, который не
создаётся корректно в Windows-сборке lx.
**Workaround:** `readClashTraffic()` безопасно возвращает (0,0). TrafficCard
показывает нули. Нужен Windows-специфичный счётчик (`GetIfTable2`).

### Reality handshake
**Симптом:** `TLS handshake: REALITY: processed invalid connection`.
**Причина:** Баг в lx-форке.
**Workaround:** VLESS+TLS с Let's Encrypt.

### Multiplex
**Симптом:** `sp.mux.sing-box.arpa:444` + `global multiplex deprecated`.
**Причина:** sing-box 1.14 депрекейтил глобальный multiplex. Включение в
inbound-полях ломает throughput (head-of-line blocking).
**Workaround:** Без multiplex. Скорость выше без него.

### ALPN h2 на VLESS
**Симптом:** `TLS handshake: EOF` на сервере.
**Причина:** VLESS не HTTP/2, h2 ALPN ломает TLS negotiation.
**Workaround:** Без ALPN.

### AmneziaWG параметры
**Симптом:** `IPC error -22: invalid UAPI device key: jc`.
**Причина:** sing-box-lx не передаёт AWG-параметры корректно.
**Workaround:** Чистый WireGuard.

---

## 14. Сборка и развёртывание

### Окружение
```
Go 1.26+ (zip, не MSI — из-за кириллицы в пути пользователя)
Node.js 22.x + pnpm
Wails v2.13+
```

### GOPROXY для РФ
```bash
go env -w GOPROXY=https://goproxy.cn,https://proxy.golang.org,direct
go env -w GOSUMDB=off
go env -w GOINSECURE=*
```

### Сборка
```bash
# 1. Биндинги (после изменения app.go)
wails generate module

# 2. Фронтенд
cd frontend && pnpm run build

# 3. Wails (ОБЯЗАТЕЛЬНО с -tags!)
wails build -skipbindings -s -tags "with_awg,with_wireguard,with_utls,with_gvisor"
```

### Портативная версия
```bash
# Собрать папку с exe + assets + .env + wintun.dll
cp build/bin/snowden-system.exe portable/
cp build/bin/wintun.dll portable/
cp build/bin/.env portable/
cp -r build/bin/assets portable/
# ZIP
powershell Compress-Archive -Path portable\* -DestinationPath snowden-portable.zip
```

### Конфиг рядом с exe
```
build/bin/
├── snowden-system.exe
├── .env                        # SNOWDEN_TG_TOKEN, SNOWDEN_TG_CHAT_ID
├── wintun.dll
├── assets/configs/
│   ├── template-vps-reality.json  # основной конфиг
│   ├── template-warp-awg.json     # WARP шаблон
│   └── ru-cidr.lst                # 30K CIDR для split-tunnel
└── ru-cidr.json                   # генерируется при первом ВКЛ
```

---

## 15. Структура проекта

```
snowden-system/
├── main.go                      — Wails entrypoint + tray + shutdown
├── app.go                       — 13 Wails-биндингов
├── telegram_bot.go              — Telegram admin panel (polling + inline)
├── proxy_windows.go             — System proxy (Win32)
├── autostart_windows.go         — Autostart (HKCU\Run)
├── tray_windows.go              — System tray
├── wails.json
│
├── backend/
│   ├── core/
│   │   ├── engine.go            — box.Box lifecycle + PlatformLogWriter
│   │   ├── manager.go           — Manager + Metrics + DomainStats
│   │   ├── metrics.go           — Traffic counters + Clash API + ping
│   │   ├── domain_stats.go      — Per-domain registry (EWMA scoring)
│   │   ├── adaptive.go          — Circuit Breaker (3-state) + deepProbe
│   │   ├── classifier.go        — Error Classifier (7 категорий)
│   │   └── registry.go          — Protocol registration
│   ├── config/
│   │   └── builder.go           — Config builder + CIDR converter
│   └── cfclient/
│       └── client.go            — Cloudflare Worker client
│
├── frontend/src/
│   ├── App.vue                  — Dashboard layout + navigation
│   ├── components/
│   │   ├── StatusCard.vue       — Power button + status + server
│   │   ├── ServersCard.vue      — Server list + live ping
│   │   ├── RoutingCard.vue      — Rules + functional toggles
│   │   ├── TrafficCard.vue      — Speed + chart + totals
│   │   ├── DiagnosticsCard.vue  — Health checks
│   │   ├── EventsCard.vue       — Event timeline
│   │   ├── DomainStatsCard.vue  — Per-domain intelligence
│   │   ├── SettingsCard.vue     — Autostart + import/export
│   │   ├── TerminalBar.vue      — Live logs (real, copyable)
│   │   ├── Sidebar.vue          — Navigation
│   │   └── TopBar.vue           — Status indicator + power button
│   └── style.css
│
├── assets/configs/
│   ├── template-vps-reality.json    — основной конфиг
│   ├── template-warp-awg.json       — WARP шаблон
│   ├── ru-cidr.lst                  — 30K CIDR
│   ├── *.example                    — sanitized шаблоны (без секретов)
│   └── server-params.json           — UUID/пароли (в .gitignore)
│
└── cloudflare/
    ├── worker.js                — CF Worker (config/health/version/telemetry)
    ├── wrangler.toml            — KV + D1 bindings
    └── schema.sql               — D1 schema
```

---

## 16. Ключевые уроки (выяснено опытным путём)

1. **`wails build` игнорирует `tags` из `wails.json`** — всегда `-tags` через CLI.
2. **Reality сломан в sing-box-lx 1.14** — VLESS+TLS с Let's Encrypt.
3. **`flow: xtls-rprx-vision` режется ТСПУ** — чистый VLESS без flow.
4. **Multiplex ломает throughput** — `sp.mux.sing-box.arpa:444` + head-of-line blocking.
5. **ALPN h2 ломает VLESS TLS** — `TLS handshake: EOF`.
6. **Clash API не работает на Windows lx** — `create clash-server: invalid argument`.
7. **IPv6 probe → ложный server_down** — принудительно `tcp4` в deepProbe.
8. **Telegram API заблокирован в РФ** — маршрут через sing-box прокси, НЕ напрямую.
9. **BBR стабилизирует latency** — cubic давал 0.4-2.8с, BBR даёт 0.3-0.5с.
10. **DuckDNS требует reCAPTCHA** — nip.io (без регистрации).
11. **DNS-loop** — `server` = IP, `server_name` = домен (для TLS).
12. **sing-box 1.14 формат** — `type: https`, `action: sniff`, `action: hijack-dns`.
13. **Кэшировать конфиг в build/bin** — `wails build -skipbindings` НЕ копирует assets.
14. **Honor шифрует logcat** — используйте `--pid` фильтр или log-файл.

---

*Документация v2.0 — 15 июля 2026. Отражает все компоненты ПК + теорию обхода.*
