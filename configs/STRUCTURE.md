# STRUCTURE.md — configs/ (единый источник конфигурации)

> ВСЕ настройки проекта. Правило: **`.example` публикуется, одноимённый без
> `.example` — секрет (в `.gitignore`)**. `configs/sync-to-windows.sh` копирует
> singbox+env в `windows/assets/configs/` (рабочие копии, их читает приложение).

## Подпапки
| Папка | Назначение | Ключевые файлы |
|-------|-----------|----------------|
| `env/` | Токены Telegram и пр. | `.env` (секрет), `.env.example` |
| `singbox/` | Конфиги sing-box: серверы, WARP, split-tunnel | `template-vps-reality.json`, `template-warp-awg.json`, `server-params.json`, `warp-keys.json`, `ru-cidr.lst`, `template-reality.json`, `test-reality-20810.json`, `mieru-credentials.json` |
| `cloudflare/` | Worker + wrangler + схема D1 | `worker.js`, `r2-worker.js`, `wrangler.toml`, `schema.sql` |
| `landing/` | Лендинг + version.json (раздача) | `index.html`, `version.json`, `snowden-*.json/(.conf/.zip)`, `_redirects` |
| `templates/` | Шаблоны для распространения | `pc/`, `android/`, `ios/`, `landing/`, `bot/`, `vps/`, `amnezia/` |

## Модель работы (из README.md)
- Отдаёшь «своим» — вся папка (это «ключ доступа» к серверам).
- Чужим — только `templates/` + `.example` (собирают свою версию).

## Синк в проект
```bash
bash configs/sync-to-windows.sh   # → windows/assets/configs/
```
⚠️ Редактировать только здесь (источник истины), НЕ `windows/assets/configs/` напрямую.

## Что публиковать нельзя
`env/.env`, `singbox/*(без .example)`, `cloudflare/wrangler.toml`,
`landing/version.json`, `landing/snowden-*.json`.

## Импорт в код
- Windows: `app.LoadConfigFile("template-vps-reality.json")` читает `assets/configs/`.
- Мобильные конфиги: `landing/snowden-android-singbox.json`, `snowden-ios-config.json`,
  `snowden-mieru.json`, `snowden-amnezia.conf` — раздаются с VPS/ботом.