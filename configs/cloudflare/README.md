# snowden.system — Cloudflare Infrastructure

## Быстрый деплой (wrangler v4)

### Шаг 0. Установить и залогиниться

```bash
npm install -g wrangler
wrangler login
```

### Шаг 1. Создать ресурсы (команды v4)

```bash
cd cloudflare/

# KV для конфигов (wrangler v4 синтаксис: kv namespace, без двоеточия)
wrangler kv namespace create SNOWDEN_CONFIG
# → скопировать id

wrangler kv namespace create SNOWDEN_VERSION
# → скопировать id

# D1 для телеметрии
wrangler d1 create snowden-telemetry
# → скопировать database_id
```

### Шаг 2. Вставить ID в wrangler.toml

Раскомментировать блоки `[[kv_namespaces]]` и `[[d1_databases]]`, вставить скопированные ID.

### Шаг 3. Применить D1 схему и задеплоить

```bash
# Схема БД (на remote)
wrangler d1 execute snowden-telemetry --remote --file=schema.sql

# Деплой Worker
wrangler deploy

# Деплой Pages (status page)
wrangler pages deploy pages/ --project-name snowden-system
```

## Endpoints

| Endpoint | Method | Описание |
|---|---|---|
| `/api/config` | GET | Динамический конфиг (серверы, протоколы) |
| `/api/health` | GET | Edge health-check VPS из Cloudflare |
| `/api/version` | GET | Версия приложения + ссылка на скачивание |
| `/api/telemetry` | POST | Анонимная телеметрия (D1) |
| `/` | GET | Status page (HTML) |

## Обновление конфига без пересборки .exe

```bash
# Записать новый конфиг в KV:
wrangler kv:key put --binding=SNOWDEN_CONFIG config '{"servers":[...]}'

# Или через API:
curl -X PUT "https://snowden-system-api.<sub>.workers.dev/api/config" \
  -H "Content-Type: application/json" \
  -d '{"servers":[...]}'
```

Клиентское приложение (Go cfclient) запрашивает `/api/config` при запуске и обновляет список серверов без пересборки.

## Edge Health Check

Worker пингует VPS:443 из ближайшего дата-центра Cloudflare. Это позволяет:

- Отличить «провайдер заблокировал VPS» от «VPS реально упал»
- Показать статус на status page
- Уведомить через Telegram бота

## D1 Telemetry (анонимная)

```sql
-- Просмотр статистики:
wrangler d1 execute snowden-telemetry --command "SELECT * FROM stats_daily LIMIT 10"

-- События:
wrangler d1 execute snowden-telemetry --command "SELECT event, COUNT(*) FROM events GROUP BY event"
```

Никаких UUID или IP — только: region, event, protocol, latency, timestamp.
