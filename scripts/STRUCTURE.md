# STRUCTURE.md — scripts/

> Python/PS-инструменты: диагностика, деплой, генерация иконок, вспомогательное.
> `env.py` загружает локальный `configs/env/.env` без печати секретов; process env имеет приоритет.
> Часть из них — разовые (ручные) утилиты, не часть рантайма.

## Основные категории
| Файл | Назначение |
|------|-----------|
| `deploy.py`, `update-server.py`, `setup-vps.py` | Деплой/обновление на VPS |
| `diagnose-*.py`, `verify-ip.py`, `fix-keypair.py`, `full-e2e.py`, `app.py` | Диагностика/подготовка |
| `capture-parallel.py`, `test-click.py` | Тесты/съём |
| `extract_memes*.py`, `make_icon.py`, `IconMaker.cs` | Генерация ассетов/иконок |
| `_diag.ps1`, `_protect.ps1`, `_smart.ps1`, `_survey.ps1`, `_disk_e.ps1` | PowerShell-обёртки (админ/диагностика) |
| `Dockerfile` | Контейнер (деплой сервера) |

## Подпапки
| Папка | Назначение | Ключевые файлы |
|-------|-----------|----------------|
| `server/` | Сервер раздачи `Snowden_system` (+`_backup`) | `app.py` (Flask/FastAPI), `requirements.txt`, `Dockerfile`, `README.md` |
| `vps-deploy/` | Статика для VPS (`:8090`) | `public/` (index.html + конфиги + version.json), `setup.sh`, `README.md` |

## Связь с кодом
- `scripts/server/app.py` — HTTP-сервер файлов раздачи; его адрес (`SNOWDEN_FILE_URL`)
  использует Telegram-бот.
- `scripts/vps-deploy/public/version.json` — источник версии для `app.GetRemoteVersion`.
- Деплой лендинга: `configs/landing/` → `scripts/vps-deploy/` → VPS (см. README корневого).