# STRUCTURE.md — frontend/ (Vite + Vue3 фронтенд Wails)

> Фронтенд встраиваемого Wails-приложения. Собирается в `dist/` и вшивается
> в exe через `//go:embed all:frontend/dist` (см. `windows/main.go`).

## Состав
| Пали | Роль |
|------|------|
| `src/` | Исходники Vue (App.vue, components/, hooks/, data/, ui/, assets/, main.ts, style.css) |
| `index.html` | HTML-обёртка (корень приложения, title) |
| `dist/` | Бандл Vite (собрано; publish-артефакт embed) |
| `wailsjs/` | Сгенерированные bindings (НЕ редактировать) |
| `package.json`, `pnpm-lock.yaml`, `package-lock.json` | Зависимости (vite, vue, typescript) |
| `vite.config.ts`, `tsconfig.json`, `tsconfig.node.json` | Конфиг сборки |
| `README.md` | Заметки фронтенда |

## Сборка
```bash
cd windows/frontend
npm ci && npm run build   # → dist/ (нужен для go build ./... и wails)
```

## Сгенерированные bindings `wailsjs/`
- `go/main/App.*` — типы/обёртки для всех Go-методов `App`.
- `go/models.ts` — TS-типы структур (VPNStatus, TrafficStats, ServerInfo…).
- `runtime/` — события (EventsOn/Off), окно, диалоги.
- ⚠️ Держать в репо, но НЕ править руками — перегенерировать `wails generate module`.

## Расширение (правило)
- Каждый главный компонент — папка `src/components/<Feature>/` со своими
  `hooks/`, `data/`, `ui/`, `components/`. Логика в `hooks/`, запросы в `data/`.
- Импорты Wails: из `src/` — `../wailsjs/...`; из `components/<F>/` — `../../../wailsjs/...`.
- Ассеты внутри фичи — `../../assets/...`.