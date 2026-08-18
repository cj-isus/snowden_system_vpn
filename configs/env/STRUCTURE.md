# STRUCTURE.md — configs/env/

> Токены и секреты (Telegram-бот и пр.).

## Файлы
| Файл | Роль |
|------|------|
| `.env` | **Секрет** (не пушится) — реальные токены |
| `.env.example` | Безопасный шаблон имён переменных |

## Переменные (читает `windows/app.go::loadEnvFile` и telegram)
- `SNOWDEN_TG_TOKEN` — токен Telegram-бота.
- `SNOWDEN_TG_CHAT_ID` — чат админа (и значение по умолчанию для admin-запроса).
- `SNOWDEN_TG_ADMIN_ID` — Telegram ID админа (если отдельно от CHAT_ID).
- `SNOWDEN_FILE_URL` — базовый URL файлов на VPS (`http://IP:8090`).
- `SNOWDEN_WORKER_URL` — URL Cloudflare Worker (переопределение `cfclient.DefaultWorkerURL`).

## Как используется
- `app.loadEnvFile()` читает `.env` рядом с exe/текущей директории и делает `os.Setenv`.
- `getTgToken()`, `getTgChatID()` → `app.NewApp()` → TelegramLogger.
- `adminUserID` = `SNOWDEN_TG_ADMIN_ID` || `SNOWDEN_TG_CHAT_ID`.

## Безопасность
- `.env` в `.gitignore`; НЕ коммитить. Своим — папка целиком, чужим — только `.example`.