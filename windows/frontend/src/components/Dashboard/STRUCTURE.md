# STRUCTURE.md — components/Dashboard

> Дашборд: карточки состояния/трафика/диагностики/событий/доменов/логов.
> Каждая карточка сама поллит Go-методы. Все импортируют `../../../wailsjs/go/main/App`.

## Файлы
| Файл | Роль | Wails-вызовы | Props/Emits |
|------|------|--------------|-------------|
| `StatusCard.vue` | Главный статус: активный сервер + латентность; кнопка вкл/выкл | `GetServers`, `GetLatency` | props `{state, configId, diag, busy}`; emit `toggle` |
| `TrafficCard.vue` | Скорость/объёмы/uptime + график (30 точек) | `Status`, `GetTraffic` | — |
| `DiagnosticsCard.vue` | Список проверок (интернет/VPS/туннель/DNS/TLS) из категории | `Diagnostics`, `GetLatency` | — |
| `EventsCard.vue` | Лента адаптивных событий (движок) | — | props `{ events }` |
| `DomainStatsCard.vue` | Per-domain: лучший outbound + score | `GetDomainStats` | — |
| `LogsCard.vue` | Лог-терминал с фильтром/поиском/уровнями/копированием | (props от App) | props `{ logs, forceExpanded }` |

## Поведение
- **Поллинг**: каждый `onMounted` → мгновенный вызов + `setInterval` (3-10 c),
  `onUnmounted` → `clearInterval`. Ошибки Go глотаются (`catch {}`).
- **Цвета/значки**: CSS-переменные `--green-success`, `--red-danger`,
  `--yellow-warn`, `--blue-info` (определены в `src/style.css`).
- **Диагностика**: `checks` computed проецирует `diag.category` на «лампочки»:
  `network_down`, `server_down`, `dns_failure`, `tls_failure` и `state==HEALTHY`.
- **Events**: `displayEvents = events.slice(0,8)`; цвета по `type` (success/error/warn/info).
- **LogsCard**: фильтрация по `[error]/[warn]/[info]`, поиск подстроки, последние 100 строк.

## Ассеты
- `StatusCard`/`TrafficCard`/`LogsCard` используют мемы из `../../assets/memes/` (pepe_*).

## Связи
Данные приходят из `App.vue` (logs/events через `EventsOn`) или напрямую из Go.
`App.vue` импортирует все карточки и раскладывает по страницам `activePage`.