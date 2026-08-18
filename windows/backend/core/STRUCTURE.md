# STRUCTURE.md — backend/core (ядро движка)

> Модуль `snowden-system/backend/core`. Сердце системы: встроенная sing-box,
> адаптивный failover, метрики, статистика по доменам, реестр протоколов.
> Для быстрой ориентации LLM/человека. Пакет: `package core`.

## Назначение
Управляет sing-box как **встраиваемой библиотекой** (не субпроцессом, см. `engine.go`)
и предоставляет `Manager` — высокоуровневый фасад, через который Wails-слой
(`app.go`) стартует/останавливает VPN и читает метрики. Плюс `AdaptiveEngine` —
цикл мониторинга с Circuit Breaker.

## Файлы
| Файл | Роль | Ключевые типы / функции |
|------|------|--------------------------|
| `engine.go` | Встраиваемый sing-box `*box.Box`; сериализация Start/Close/Reload | `Engine`, `EngineState` (Stopped/Starting/Running/Stopping/Error), `Start`, `Close`, `Reload`, `Wait`, `SetLogHandler`, `SetClassifier`, `LogHandler` |
| `registry.go` | Реестр протоколов sing-box (решает, какие in/outbound зарегистрированы) | `boxContext()`, `inbounds()`, `outbounds()`, `endpoints()`, `dnsTransports()`, `services()`, `certificateProviders()` |
| `manager.go` | Фасад для UI: жизненный цикл + снимки состояния | `Manager`, `VPNStatus`, `StartVPN`, `StopVPN`, `ReloadVPN`, `Status`, `GetServers`, `GetTraffic`, `GetDomainStats`, `RecordDomainStat`, `PollConnections` |
| `adaptive.go` | Адаптивный failover: Circuit Breaker + классификатор + пробы | `AdaptiveEngine`, `circuitBreaker`, `TunnelState` (Closed/HalfOpen/Open), `DiagStatus`, `loop`, `deepProbe`, `onCircuitOpen` |
| `classifier.go` | Классификация ошибок в категории; история событий | `ErrorClassifier`, `ErrorCategory` (NetworkDown/ServerDown/DNS/TLS/Blocked/Whitelist/Degraded/Unknown), `classify()`, `ClassifyProbeError()` |
| `metrics.go` | Счётчики трафика, парсинг серверов/маршрутов, пинг, экспорт конфига | `Metrics`, `TrafficStats`, `ServerInfo`, `RouteRuleInfo`, `ClashConnection`, `ParseServers`, `ParseRouteRules`, `PingServer`, `ProbeLatencyThroughProxy`, `ExportConfig/ImportConfig` |
| `domain_stats.go` | Per-domain «память»: лучший outbound для каждого домена | `DomainStatsRegistry`, `DomainMetric`, `DomainScore`, `Record()`, `GetBest()`, `TopDomains()`, `scoreMetric()` |

## Потоки данных
```
app.go (methods) ──► Manager ──► Engine (embedded sing-box)
   ▲                 │               │
   │  Get*()         │  Status()     ▼
   └─────────────────┴─  sing-box (lox смеш. in 127.0.0.1:20808, Clash API :9090)
                        ▲
   AdaptiveEngine.loop ─┤ deepProbe через proxy → правки (Reload / cb)
```
- **Вход**: `StartVPN/ReloadVPN(configID, configJSON)` от App.
- **Логи**: `LogHandler.OnLog` → `app.logEmitter` → UI/Telegram; параллельно `ErrorClassifier.OnLog`.
- **Метрики**: `Metrics.sample()` из Clash API `/connections`; `PollConnections()` кормит `domain_stats`.
- **Адаптив**: `AdaptiveEngine.loop` каждые 30 c (`adaptive.go`) → `deepProbe` → при фейлах Circuit Breaker → `onCircuitOpen` (reload).

## Ключевые состояния
- `EngineState`: Stopped → Starting → Running → Stopping/Error. `Error` разрешает рестарт.
- `TunnelState` (Circuit Breaker): Closed(здоров) → Open(фейл ×N) → HalfOpen(проба) → Closed.
  ⚠️ `ShouldProbe`/`EnterHalfOpen` сейчас определены, но не вызываются (см. `PLAN.md` A1).

## Известные ограничения / нюансы
- sing-box не умеет hot-reload (`Reload` = Close+Start) — `engine.go` держит мьютекс на весь своп, состояние не публикуется как Stopped.
- Clash API на Windows sing-box-lx отдаёт трафик, но счётчик может быть 0 при недоступности `:9090`.
- `registry.go` намеренно не регистрирует `protocol/naive` (тянет cronet, не качается через RU-прокси).

## Тесты
- `enginetest/e2e.go` (пакет `main`, вне этой папки) — ручной e2e-прогон движка.
- Юнит-тестов `*_test.go` пока нет (см. PLAN: добавить для circuitBreaker/metrics).