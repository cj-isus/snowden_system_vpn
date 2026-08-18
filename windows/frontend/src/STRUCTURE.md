# STRUCTURE.md — frontend/src (Vue3 приложение)

> Root-слой фронтенда Wails. Vue 3 `<script setup>` + TS, Vite. Главный
> компонент — `App.vue`, держит глобальное состояние и разворачивает страницы.

## Файлы (корень src/)
| Файл | Роль |
|------|------|
| `App.vue` | Корневой компонент: состояние (state/configId/logs/events/diag/toasts/uptime), логика StartVPN/StopVPN, выбор страницы, `provide("showToast")` |
| `main.ts` | Bootstrap: `createApp(App).mount('#app')` |
| `style.css` | Глобальные CSS-переменные и стили (тёмная тема, `--bg-*`, `--text-*`) |
| `vite-env.d.ts` | Типы Vite |
| `assets/` | шрифты (Nunito), иконки, logo, memes (pepe_*) |
| `components/` | Папки фич: `Layout/`, `Dashboard/`, `Servers/`, `Settings/` (см. их STRUCTURE.md) |
| `hooks/` | (пусто, `.gitkeep`) — заготовка под общие composables |
| `data/` | (пусто, `.gitkeep`) — заготовка под клиенты Wails API / Pinia |
| `ui/` | (пусто, `.gitkeep`) — заготовка под переиспользуемые базовые UI |

## App.vue — ключевое
- **Импорты Wails**: `StartVPN, StopVPN, Status, LoadConfigFile, Diagnostics, GetTraffic`
  из `../wailsjs/go/main/App`; `EventsOn/EventsOff` из `../wailsjs/runtime/runtime`.
- **Константы**: `CONFIG_ID="vps-reality"`, `CONFIG_FILE="template-vps-reality.json"`.
- **Страницы** (переключение `activePage`): dashboard, routing, traffic, settings,
  diagnostics, events, logs, about — в зависимости от `Sidebar` эмита `nav`.
- **События из Go**: `EventsOn("engine:log")` → массив `logs`; адаптивные события
  (`engine:diag` и др.) → массив `events`.
- **provide/inject**: `provide("showToast", showToast)` — вложенные карточки вызывают
  `inject("showToast")`.
- **Старт VPN в UI**: кнопка → `LoadConfigFile(CONFIG_FILE)` → `StartVPN(CONFIG_ID, json)`.

## Внешние привязки (важно)
- `wailsjs/` (генерация, НЕ редактировать): `wailsjs/go/main/App` (типы TS для всех
  Go-методов App), `wailsjs/runtime/runtime` (EventsOn/Off, окно, диалоги).

## Направление зависимостей
`App.vue` ← Layout (Sidebar/TopBar/TerminalBar) управляют навигацией/вкл.
`App.vue` ← Dashboard (Status/Traffic/Diag/Events/Logs/DomainStats) показывают состояние.
`App.vue` ← Servers (ServersCard/RoutingCard) выбирают сервер/правила.
`App.vue` ← Settings (SettingsCard) настройки.
`App.vue` → wailsjs → Go App → core.