# AGENTS.md — инструкция для ИИ-ассистента

Этот файл помогает ИИ-ассистенту быстро и корректно ориентироваться в проекте
и не сломать рабочую сборку.

## 1. Первое чтение

1. Прочитай `README.md` — обзор и структура.
2. Прочитай `STRUCTURE.md` — архитектура, компоненты, потоки данных.
3. Двигайся от точки входа: `windows/main.go` → `windows/app.go` →
   `windows/backend/core/`.

## 2. Критические правила при изменении кода

### Go (`windows/`)
- Модуль лежит в `windows/` (`go.mod`, module `snowden-system`). Импорты вида
  `snowden-system/backend/core` зависят от **относительного пути** от корня
  модуля: не переноси `.go`-файлы между папками `backend/*`, не меняя импорты.
- После любых правок выполни проверку:
  ```bash
  cd windows && go build ./...
  ```
- `main.go` использует `//go:embed all:frontend/dist` — для `go build ./...`
  нужен собранный фронтенд (`frontend/dist`). Сначала: `cd frontend && npm run build`.

### Vue (`windows/frontend/src/`)
- Главные компоненты лежат в `components/<Feature>/`. **Ассеты** внутри фичи
  подключаются как `../../assets/...` (уровень глубины считай от файла).
- Привязки Wails (`frontend/wailsjs/`) **не редактируются** — это генерация
  (`wails generate module`). Импорты из компонентов указывают на
  `../../../wailsjs/go/main/App` (на уровень глубже, чем импорты из `src/`).
- Проверка: `cd windows/frontend && npm run build`.

### Конфиги
- Редактируй **`configs/singbox/`** (источник истины), а не `windows/assets/configs/`
  (рабочие копии). После изменений запусти `configs/sync-to-windows.sh`.
- Не коммить реальные секреты (`.env`, `server-params.json`, `warp-keys.json`).

## 3. Что нельзя терять

- Реальные параметры серверов живут в `configs/singbox/` и
  `windows/assets/configs/`. Не удаляй их — без них приложение не поднимется.
- Документация (`windows/docs/`, `docs/`) — часть проекта, не мусор.
- Скрипты в `scripts/` нужны для диагностики и деплоя.

## 4. Сборка и деплой (шпаргалка)

```bash
# фронтенд → бандл
cd windows/frontend && npm run build
# серверная часть (все пакеты + вшитый UI)
cd windows && go build ./...
# полная сборка Wails
wails build -skipbindings -s -tags "with_awg,with_wireguard,with_utls,with_gvisor"
# синк конфигов из источника в рабочую папку
bash configs/sync-to-windows.sh
```

## 5. Типичные задачи

- «Добавить сервер» → правишь `configs/singbox/template-vps-reality.json`
  (outbounds) → сверка `sync-to-windows` → перезапуск приложения.
- «Раздать обновление» → обновить `configs/landing/version.json` + файлы →
  залить на VPS (`scripts/vps-deploy`) → обновить бот, если нужно.
- «Изменить UI» → работа в `windows/frontend/src/components/<Feature>/`,
  логику выносить в `hooks/`, запросы — в `data/`.