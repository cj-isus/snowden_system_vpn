# src/ — Vue3 + TypeScript

Компонентно-ориентированная структура: каждый «главный компонент» — это
папка со своими подразделами.

## Структура

```
src/
├── main.ts App.vue style.css vite-env.d.ts
├── assets/                 # иконки, шрифты, мемы (pepe)
├── components/             # папка на каждый главный компонент
│   ├── Dashboard/          # StatusCard, TrafficCard, DiagnosticsCard,
│   │                       # EventsCard, DomainStatsCard, MatrixRain
│   ├── Servers/            # ServersCard, RoutingCard
│   ├── Layout/             # Sidebar, TopBar, TerminalBar (+ ui/MatrixRain)
│   └── Settings/           # SettingsCard
├── hooks/                  # общие composables (useVpn, useMetrics и т.п.)
├── data/                   # клиенты Wails API, Pinia-хранилища
└── ui/                     # переиспользуемые базовые UI-компоненты
```

## Правила для фич-папок

Внутри `components/<Feature>/` можно добавлять:
- `hooks/` — логика (composable), отделённая от шаблона;
- `data/` — вызовы бэкенда / хранилище, нужные этой фиче;
- `ui/` — мелкие вложенные компоненты;
- `components/` — под-компоненты покрупнее.

## Привязки Wails

`../../wailsjs/` (из `src/`) и `../../../wailsjs/go/main/App` (из
`src/components/<Feature>/`) — **сгенерированные** файлы, не редактировать.
Регенерация: `wails generate module`.