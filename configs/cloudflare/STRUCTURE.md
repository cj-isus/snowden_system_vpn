# STRUCTURE.md — configs/cloudflare/

> Cloudflare-инфраструктура: Worker (API статуса/обновлений), r2-раздача, D1-телеметрия.

## Файлы
| Файл | Роль |
|------|------|
| `worker.js` | Основной Worker: `/api/config`, `/api/health`, `/api/version`, `/api/telemetry`, `/` |
| `r2-worker.js` | Worker раздачи обновлений (R2) |
| `wrangler.toml` / `.example` | Конфиг deploy (KV `SNOWDEN_CONFIG`, `SNOWDEN_VERSION`, D1 `DB`) — секрет |
| `r2-wrangler.toml` | Конфиг R2-раздачи |
| `schema.sql` | D1-схема телеметрии (таблица events) |
| `README.md` | Инструкция деплоя (wrangler v4) |

## Endpoints (worker.js)
| Endpoint | Метод | Что |
|----------|-------|-----|
| `/api/config` | GET | Динамический конфиг (серверы, протоколы) — клиент обновляется без пересборки |
| `/api/health` | GET | Edge health-check VPS (с дата-центров CF) — отличает «заблокирован» от «упал» |
| `/api/version` | GET | Версия + ссылка скачивания |
| `/api/telemetry` | POST | Анонимная телеметрия → D1 (region/event/protocol/latency) |
| `/` | GET | Status page (JSON) |

## KV/D1
- `SNOWDEN_CONFIG` — динамический конфиг JSON (перезаписывается через `wrangler kv:key put`).
- `SNOWDEN_VERSION` — строка версии + download URL.
- `DB` (D1 `snowden-telemetry`) — события; без UUID/IP.

## Связь с кодом
- Go `backend/cfclient` (Windows) дергает `/api/config`, `/api/health`, `/api/version`, `/api/telemetry`.
- `app.GetRemoteHealth`, `app.CheckForUpdate` — через cfclient.
- Клиентский файл конфига приложения: update через `landing/version.json` или `/api/version`.