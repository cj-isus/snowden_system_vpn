# STRUCTURE.md — backend/cfclient

> Модуль `snowden-system/backend/cfclient`. Пакет: `package cfclient`.
> HTTP-клиент к Cloudflare Worker: динамические конфиги, health, версии, телеметрия.

## Назначение
Забирает с Cloudflare Worker (`https://snowden-system-api.pcel628.workers.dev`)
динамические данные, чтобы обновлять серверы/протоколы/эндпоинты БЕЗ пересборки
`.exe` — достаточно обновить KV на Cloudflare.

## Файлы
| Файл | Роль | Ключевые типы / методы |
|------|------|--------------------------|
| `client.go` | Весь клиент в одном файле | `Client`, `New()`, `SetWorkerURL`, `FetchConfig()`, `FetchHealth()`, `FetchVersion()`, `SendTelemetry()` |

## Доменные типы
- `ServerConfig` — одна нода (id, name, address, port, protocol, domain, uuid/password/publicKey, active).
- `DynamicConfig` — `GET /api/config`: servers + routing(ruCidrUrl, splitTunneling) + version.
- `HealthResult` — `GET /api/health`: edge, timestamp, tests{провайдер→состояние}.
- `VersionInfo` — `GET /api/version`: version, downloadUrl.

## Потоки данных
- **Вход (вызовы)**: `app.GetRemoteHealth()` → `FetchHealth` (проверка VPS со стороны Cloudflare edge);
  `app.GetRemoteVersion`/`CheckForUpdate` → `FetchVersion`.
- **Исходящий**: `SendTelemetry(region, event, protocol, latencyMs)` → D1 база на CF (POST /api/telemetry).

## Конфигурация
- URL: `SNOWDEN_WORKER_URL` env → иначе `DefaultWorkerURL`. Таймаут 10 c.
- Клиент не использует системный прокси (не кодирует VPN-путь) — health с CF edge.

## Нюансы
- `byteReader` вручную реализует `io.Reader` (минимализм, без импорта `bytes`).
- Секретов нет; KV-ключи на стороне Cloudflare.