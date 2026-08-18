# STRUCTURE.md — configs/landing/

> Лендинг + файлы раздачи (version.json отвечает за апдейты). Хостится на
> VPS (`:8090`) и/или Cloudflare Pages (`main.snowden-system.pages.dev`).

## Файлы
| Файл | Роль |
|------|------|
| `index.html` | Лендинг-страница |
| `_redirects` | Правила редиректов Pages |
| `version.json` / `.example` | Версия + `pc_url` + `changelog` (читает `app.GetRemoteVersion`) |
| `version.txt` | Версия текстом |
| `docs_pc.html`, `docs_android.html` | Docs-страницы |
| `snowden-*.json/.conf/.zip/.rar` | Распространяемые артефакты: android singbox, ios config, mieru, amnezia conf, portable zip |

## Роль version.json
- `app.GetRemoteVersion()` сначала пробует `https://main.snowden-system.pages.dev/version.json`
  (всегда актуальный), фолбэк — CF Worker `/api/version`.
- `app.CheckForUpdate()` сравнивает с локальной `LOCAL_VERSION="1.3.5"`.

## Раздача в РФ без домена
- Файлы хостятся на VPS `http://<IP>:8090` (`scripts/vps-deploy/public/`).
- Telegram-бот (`telegram_bot.go`) качает с `SNOWDEN_FILE_URL` и пересылает файл в чат
  (файлы доходят через Telegram, минуя собственный IP).

## Публикация
- `configs/landing/` → `scripts/vps-deploy/` → залить на VPS.
- `version.json` — источник правды апдейтов.
- ⚠️ `version.json` и `snowden-*.json` без `.example` — секреты/IP, не пушить.