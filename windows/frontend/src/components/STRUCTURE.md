# STRUCTURE.md — components/ (фичи фронтенда)

> Палитра Vue-карточек по фичам. Соглашение: каждая фича — папка со своими
> компонентами; логика выносится в `hooks/`, запросы — в `data/` (сейчас пусто).

## Папки
| Папка | Назначение | Компоненты |
|-------|-----------|-----------|
| `Layout/` | Каркас окна: шапка, сайдбар, терминал-бар, матрица-фон | `TopBar.vue`, `Sidebar.vue`, `TerminalBar.vue`, `ui/MatrixRain.vue` |
| `Dashboard/` | Статус, трафик, диагностика, события, домены, логи | `StatusCard.vue`, `TrafficCard.vue`, `DiagnosticsCard.vue`, `EventsCard.vue`, `DomainStatsCard.vue`, `LogsCard.vue` |
| `Servers/` | Серверы + маршрутизация | `ServersCard.vue`, `RoutingCard.vue` |
| `Settings/` | Настройки: автозапуск, экспорт/импорт | `SettingsCard.vue` |

## Общие паттерны
- **Импорт Wails**: из компонентов (глубина +2 от `src/`) — `../../../wailsjs/go/main/App`;
  из `App.vue` (глубина +1) — `../wailsjs/...`.
- **Инъекция тостов**: `const showToast = inject("showToast")` — предоставлен `App.vue`.
- **Поллинг**: карточки независимо `setInterval`-ом дёргают Go-методы и `onUnmounted`
  очищают таймер.
- **Ассеты**: мемы/иконки внутри фичи — `../../assets/...` от файла компонента
  (уровень глубины — из `components/<Feature>/<Card>.vue`).

## Что НЕ трогать
- `wailsjs/` — генерация (`wails generate module`), bindings для Go-методов.
- Пустые заготовки `hooks/`, `data/`, `ui/` в `src/` — точки расширения.