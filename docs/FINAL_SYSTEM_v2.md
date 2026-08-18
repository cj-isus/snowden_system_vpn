Plan твой 
🏗️ ПЛАН: Десктопное VPN-приложение «Snowden_system VPN» (Wails + Go)
Архитектура спроектирована на основе изучения эталонных репозиториев (GUI.for.SingBox, Clash Verge Rev) и адаптивного движка (Circuit Breaker + EWMA + РФ-детекторы). если уместно то дополнительно связать с ваня впн(уже есть купленый) для изобилия стран и стабильности(опционально)

🎯 ЧТО СТРОИМ
Десктопное приложение для Windows (ПК), под капотом — адаптивный алгоритм, который сам анализирует доступность/скорость/блокировки и переключается между серверами/протоколами. Для себя и близких. Кодим вместе (вы программируете сами).

Ключевая фишка: кнопка «Вкл» — и приложение само поддерживает работоспособность при ЛЮБЫХ блокировках (ЧС, БС, DPI, шейпинг), анализируя себя в реальном времени.

📐 АРХИТЕКТУРА

text
┌─────────────────────────────────────────────────────────┐
│              FRONTEND (Vue3 + TypeScript)                │
│   UI: One-button + Pro-режим (дашборд/графики/логи)      │
└────────────────────────┬────────────────────────────────┘
                         │ Wails bindings (авто-генерация)
┌────────────────────────▼────────────────────────────────┐
│              BACKEND (Go, Wails v2)                      │
│  ┌─────────────┐  ┌──────────────┐  ┌────────────────┐  │
│  │ CoreManager │  │ AdaptiveEng  │  │ ConfigBuilder  │  │
│  │ (subprocess │  │ (health,     │  │ (llimonix API  │  │
│  │  sing-box)  │  │  failover)   │  │  + свои VPS)   │  │
│  └──────┬──────┘  └──────┬───────┘  └────────────────┘  │
│         │                │ Clash API (RESTful+WS)        │
└─────────┼────────────────┼───────────────────────────────┘
          ▼                ▼
┌─────────────────────────────────────────────────────────┐
│   sing-box.exe (subprocess) + wintun.dll  [TUN-режим]    │
│   Управление: 127.0.0.1:20123 (Clash API + secret)       │
└─────────────────────────────────────────────────────────┘
          ▲
┌─────────┴────────────────────────────────────────────────┐
│   your-vpn-service.exe (Windows Service под SYSTEM)      │
│   Ставится 1 раз (UAC). Запускает sing-box с правами.    │
└──────────────────────────────────────────────────────────┘
📁 СТРУКТУРА ПРОЕКТА

text
unkillable-vpn/
├── main.go                      # Wails entrypoint
├── go.mod                       # wails, x/sys, gopsutil, systray
├── wails.json
├── backend/
│   ├── app.go                   # App struct, Wails-биндинги, трей
│   ├── core/
│   │   ├── manager.go           # CoreManager: Start/Stop/Reload sing-box
│   │   ├── subprocess.go        # exec.Command("sing-box.exe", "run", "-c", cfg)
│   │   ├── subprocess_windows.go# SetCmdWindowHidden, SendExitSignal (CTRL_BREAK)
│   │   └── clash_api.go         # HTTP-клиент к Clash API (/configs, /proxies)
│   ├── engine/
│   │   ├── adaptive.go          # AdaptiveEngine: главный цикл мониторинга
│   │   ├── metrics.go           # EWMA latency, packet loss, success rate
│   │   ├── state_machine.go     # CONNECTING→HEALTHY→DEGRADED→FAILED→FALLBACK
│   │   ├── circuit_breaker.go   # Circuit Breaker с гистерезисом
│   │   ├── probes.go            # Active probes (generate_204, youtube, discord)
│   │   ├── detectors_ru.go      # РФ-детекторы: DME colo, ТСПУ RST, whitelist
│   │   └── executor.go          # Switch proxy/protocol, rotate I1
│   ├── config/
│   │   ├── builder.go           # Сборка sing-box JSON-конфига
│   │   ├── llimonix.go          # Клиент llimonix JSON-API (WARP+MASQUE)
│   │   ├── templates.go         # Шаблоны: AmneziaWG/Reality/Hysteria2 для своих VPS
│   │   └── rules.go             # РФ split-tunneling правила (itdoginfo)
│   ├── service/
│   │   ├── install.go           # Установка Windows-сервиса (svc/mgr) под SYSTEM
│   │   ├── service_main.go      # +build svc — точка входа сервиса
│   │   └── ipc.go               # Named pipe GUI↔service
│   └── system_proxy.go          # Реестр Windows (опционально System Proxy)
├── frontend/
│   ├── package.json             # pnpm, vue3, vite, recharts, notistack
│   └── src/
│       ├── api/
│       │   ├── kernel.ts        # Clash API клиент (hot-reload, переключение)
│       │   ├── websocket.ts     # WS /traffic /logs /memory /connections
│       │   └── wails.ts         # Wails bindings (вызовы Go)
│       ├── views/
│       │   ├── Home.vue         # One-button режим (● ВКЛ /выкл)
│       │   └── Pro.vue          # Дашборд: серверы, графики, правила, логи
│       ├── stores/
│       │   ├── vpn.ts           # Pinia: состояние, активный сервер
│       │   └── metrics.ts       # Pinia: метрики health-check
│       └── components/
│           ├── BigButton.vue    # Главная кнопка
│           ├── ServerList.vue   # Список серверов с латенси
│           ├── TrafficChart.vue # График трафика
│           └── StatusBadge.vue  # ● HEALTHY/DEGRADED/FAILED
├── build/windows/
│   ├── icon.ico
│   ├── wails.exe.manifest       # requestedExecutionLevel
│   ├── installer.nsi            # NSIS: app + sing-box.exe + wintun.dll
│   └── service-installer.nsi    # Установка сервиса
└── assets/
    └── kernels/                 # sing-box.exe, mihomo.exe (скачиваются/обновляются)
🚀 ЭТАПЫ РАЗРАБОТКИ (с детальными deliverables)
ЭТАП 0: Подготовка окружения (1 день)
Что: разворачиваем toolchain, создаём скелет Wails-проекта.

 Установить Go 1.22+, Node 20+, pnpm
 wails init unkillable-vpn -t vue-ts
 Скачать sing-box.exe (последний релиз) + wintun.dll → assets/kernels/
 Проверить: sing-box.exe run -c test.json запускается вручную
 Создать GitHub-репозиторий Snowden_system для папки D:\ОБХОДЫ\Snowden_system
Deliverable: пустой Wails-проект собирается (wails dev).

ЭТАП 1: CoreManager — запуск/остановка sing-box (3-4 дня)
Цель: GUI умеет запускать sing-box как subprocess, видеть логи.

 backend/core/subprocess.go: Start(configPath) → exec.Command("sing-box.exe", "run", "-c", cfg), скрытие окна (Windows), сохранение PID
 backend/core/subprocess_windows.go: SetCmdWindowHidden (CREATE_NEW_PROCESS_GROUP | HideWindow), SendExitSignal (CTRL_BREAK через AttachConsole)
 backend/core/manager.go: state (STOPPED/RUNNING/ERROR), Start()/Stop()/Restart()
 Тестовый sing-box конфиг (один Shadowsocks/VLESS на свой VPS)
 Wails-биндинги: StartVPN(configPath), StopVPN(), GetStatus()
 Frontend: минимальная кнопка «Старт/Стоп» + статус
Deliverable: кнопка в UI запускает/останавливает VPN-соединение с одним сервером.

ЭТАП 2: Clash API клиент + hot-reload (2-3 дня)
Цель: управление ядром без перезапуска, real-time метрики.

 Включить в sing-box конфиге: experimental.clash_api.external_controller = "127.0.0.1:20123", secret = random
 backend/core/clash_api.go: HTTP-клиент с Bearer auth
GET /version, GET /proxies, PUT /proxies/{group} (переключение)
PATCH /configs {"path": newConfig} (hot-reload)
GET /proxies/{name}/delay?url=... (health-check одной ноды)
 frontend/src/api/kernel.ts (адаптировать из GUI.for.SingBox)
 WebSocket: /traffic, /logs, /connections с авто-переподключением
 Frontend: переключение серверов из списка без перезапуска
Deliverable: можно сменить сервер/профиль на лету, виден реальный трафик и логи.

ЭТАП 3: ConfigBuilder — генерация конфигов (3-4 дня)
Цель: приложение само собирает конфиг из нескольких источников.

 backend/config/llimonix.go: клиент JSON-API llimonix (5 WARP-зеркал + 2 MASQUE). Получает свежие WARP+MASQUE endpoints/I1.
 backend/config/templates.go: шаблоны для своих VPS (AmneziaWG 2.0, VLESS+Reality+XHTTP, Hysteria2+Gecko). Генерация sing-box outbounds из шаблона.
 backend/config/rules.go: РФ split-tunneling (загрузка itdoginfo allow-domains → RULE-SET → DIRECT), геоблок-сервисы через proxy.
 backend/config/builder.go: сборка итогового sing-box JSON (outbounds + inbounds[TUN] + route + rule_set + clash_api).
 Авто-обновление: периодический фоновый рефетч llimonix (раз в сутки).
Deliverable: приложение генерирует рабочий мульти-протокольный конфиг (свой VPS + WARP + MASQUE + правила).

ЭТАП 4: AdaptiveEngine — сердце системы (5-7 дней) ⭐
Цель: самоанализ и авто-переключение. Главная фишка продукта.

 backend/engine/metrics.go: хранилище метрик
EWMA latency (α=0.2) по каждой ноде
Passive: TCP retransmissions, connection errors (из WS /connections)
Success rate (скользящее окно)
 backend/engine/probes.go: active probing
generate_204 (HTTPS, 10-30с интервал)
Доступность youtube.com, discord.com (60-120с)
Download speed test (5-10 мин, через speed.cloudflare.com)
 backend/engine/detectors_ru.go: РФ-спецдетекторы ⭐
DME-детектор: GET cdn-cgi/trace → если colo=DME/KJA/LED → DEGRADED
ТСПУ-детектор: handshake failures, RST-подсчёт (TCP reset после ClientHello)
Whitelist-детектор: корреляция падений (все заблокированные упали + рунет работает + свой VPS недоступен = включили БС)
 backend/engine/state_machine.go: state per нода
CONNECTING → HEALTHY (1 успех за <1с)
HEALTHY → DEGRADED (EWMA >2× baseline ИЛИ loss >5%)
DEGRADED → FAILED (нет ответа 5с)
FAILED → CONNECTING (background probe каждые 30с с backoff)
 backend/engine/circuit_breaker.go: hysteresis пороги + dwell time (60с) + confirmation count (3 провала для open, 2 успеха для recovery)
 backend/engine/executor.go: действие FALLBACK
PUT /proxies/{group} → переключить на резервную ноду
PATCH /configs → если нужна смена протокола
Ротация I1 при детекте блокировки (генерация через mini_quic_generator)
 Frontend: статус-бейдж (● HEALTHY/DEGRADED/FAILED), график латенси
Deliverable: приложение само переключается на резерв при падении сервера/блокировке, без действий пользователя.

ЭТАП 5: TUN-режим + Service Mode (4-5 дней)
Цель: весь трафик ПК через VPN, без админ-прав каждый запуск.

 Включить inbounds.type=tun (stack=gVisor, auto_route, strict_route)
 backend/service/service_main.go (+build svc): Go-Windows-сервис под SYSTEM, IPC named pipe, запускает sing-box.exe по команде
 backend/service/install.go: установка/удаление через golang.org/x/sys/windows/svc/mgr (один UAC)
 GUI проверяет IsPrivileged() → если нет сервиса, предлагает установить
 Проверить: TUN-адаптер создаётся, весь трафик идёт через VPN, kill-switch (при падении — интернет отрубается, не течёт мимо)
Deliverable: после одной установки — TUN-режим работает без UAC при каждом запуске.

ЭТАП 6: UI — One-button + Pro-режим (3-4 дня)
Цель: финальный интерфейс для себя и близких.

 One-button режим (дефолт): большая кнопка ● ВКЛ/ВЫКЛ, индикатор (сервер/скорость/статус), иконка в трее
 Pro-режим (по кнопке ⚙️):
Список серверов с латенси/цветом
Графики трафика (Recharts)
Правила split-tunneling (включить/выключить сервисы)
Логи в реальном времени
Настройки (свои VPS, llimonix ключи, свои I1)
 Автозапуск с Windows (реестр Run)
 Сворачивание в трей
Deliverable: готовое приложение для ежедневного использования близкими.

ЭТАП 7: Подготовка VPS + финальная интеграция (2-3 дня)
Что: поднимаем серверы и связываем всё.

 Арендовать VPS-EU (Aeza, Нидерланды) — амнезияWG 2.0 + Reality/XHTTP + Hysteria2
 Арендовать VPS-RU (Yandex Cloud) — для БС-релея
 Внести свои серверы в ConfigBuilder
 Протестировать полный стек: обычный день → DPI → БС
 NSIS-инсталлятор (app + sing-box + wintun + service)
 Раздать близким
Deliverable: готовый дистрибутив .exe + работающая инфраструктура.

🔑 КЛЮЧЕВЫЕ ТЕХНИЧЕСКИЕ РЕШЕНИЯ
Решение	Выбор	Почему
Фреймворк	Wails v2 + Vue3/TS	Go создан для network; Vue — быстрый UI
Ядро	sing-box (subprocess)	Изоляция, обновление, hot-reload. НЕ embedded
TUN	wintun.dll + gVisor stack	Надёжнее system-stack на Windows
Привилегии	Go Windows Service под SYSTEM	Один UAC, не каждый запуск
Управление ядром	Clash API (RESTful + WS)	Эталон GUI.for.SingBox, hot-reload без рестарта
Генератор WARP	llimonix JSON-API	Публичный, CORS *, 7 зеркал
Адаптивный движок	Circuit Breaker + EWMA + РФ-детекторы	Само-анализ без ложных срабатываний
📊 ИТОГО
Время: ~4-5 недель вдвоём (при ~3-5 часов в день)
Бюджет: ~800-1500₽/мес (2-3 VPS) + одноразово домен (опц.)
Результат: десктопное .exe-приложение, которое «включил и забыл» — само держит связь при любых блокировках
🎬 С ЧЕГО НАЧАЁМ (после утверждения плана)
Этап 0 + Этап 1 — поднимаем скелет Wails-проекта и CoreManager. Я пишу backend на Go, вы параллельно ставите окружение и тестируете. Первая видимая цель — кнопка в UI, которая запускает VPN.

критика плана:
 проверь прав ли я - Table Утверждение	Что нашёл	Вердикт *llimonix JSON-API (GET, CORS , 5 зеркал + 2 MASQUE)	API реально существует (POST /api/generate), CORS * реальный. Но: не GET, а POST; защищено капчей (Cloudflare Turnstile/WAF — curl возвращает {"success":false,"message":"Капча не пройдена."}); sing-box формат API не поддерживает (validFormats: wireguard, throne, clash, nekoray, husi, karing, wiresock); "5 зеркал" — это приватные endpoints через Telegram (findllimonix), не публичные WARP-зеркала	⚠️ Частично верно, но требует корректировки GUI.for.SingBox (Wails+Go, subprocess, Clash API)	Подтверждено полностью. Репозиторий 7K+ stars, использует Wails + Vue3 + subprocess sing-box + Clash API	✅ Верно Clash Verge Rev Service Mode	Подтверждено. Service Mode ставится один раз (UAC), затем TUN работает без admin-прав каждый запуск. Работает через background helper	✅ Верно Adaptive Engine (CB + EWMA + РФ-детекторы)	Паттерны реальны, но DME/ТСПУ/whitelist требуют тонкой настройки порогов. Не "вкл и забыл", а "вкл и настраивай"	✅ Реально, но сложнее TUN требует admin → Service Mode обязателен	Подтверждено. sing-box без прав: configure tun interface: Access is denied	✅ Верно Критические нюансы, которые ломают план, если не учесть 1. llimonix API — POST с капчей, без sing-box В app/api/generate/route.ts явно: TypeScript Copy const validFormats: ConfigFormat[] = [   'wireguard', 'throne', 'clash', 'nekoray', 'husi', 'karing', 'wiresock' ]; sing-box нет в списке. Это значит, что вы не получите готовый sing-box конфиг из llimonix. Нужно либо: Брать wireguard и конвертировать в sing-box endpoint самостоятельно Или генерировать WARP ключи напрямую через Cloudflare API (как делает llimonix внутри: registerClient → enableWarp) Капча — не просто "CORS * GET". В production-деплое (warp3.llimonix.pw) стоит Cloudflare защита. Для автоматического вызова из приложения нужно либо использовать зеркала без капчи (если есть), либо встроить генерацию WARP-ключей прямо в приложение (тот же алгоритм, что в lib/warp-service.ts). 2. "5 WARP-зеркал" — это приватные endpoints В config/endpoints.ts 5 "зеркал" (DE, NL, FI, PL, LV) имеют externalUrl: tg://resolve?domain=findllimonix. Это не бесплатные публичные endpoints, а доступ через Telegram-канал. Для приложения нужно либо свои endpoints, либо публичные engage.cloudflareclient.com / 162.159.195.1:500. 3. Windows Service Mode — не "просто включить" Clash Verge Rev использует отдельный service binary (helper), который ставится рядом с GUI. В Wails-проекте это означает: два бинарника (GUI + Service) + IPC (Named Pipe/HTTP). Это не "фича sing-box", а отдельная инфраструктурная задача. Оценка: +2-3 недели. 4. Hysteria2 + Gecko — НЕ в sing-box Вы упоминаете Hysteria2+Gecko в шаблонах. Но sing-box (даже sing-box-lx) не поддерживает Gecko. Gecko есть только в apernet/hysteria и mihomo. Если вам критичен Gecko — нужен либо mihomo вместо sing-box, либо Salamander (который есть в sing-box), либо ждать upstream. если я прав то подумай насчёт этого 