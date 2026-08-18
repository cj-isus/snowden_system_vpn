# STRUCTURE.md — scripts/vps-deploy/

> Публичный набор файлов для заливки на VPS (`:8090`) — лендинг + версия + конфиги.

## Файлы
| Файл | Роль |
|------|------|
| `public/index.html` | Лендинг-страница |
| `public/snowden-amnezia.conf` | Конфиг AmneziaWireGuard |
| `public/snowden-android-singbox.json` | Конфиг Android |
| `public/snowden-ios-config.json` | Конфиг iOS |
| `public/snowden-mieru.json` | Конфиг mieru |
| `public/snowden-portable.zip` | Портативный Windows-билд |
| `public/version.json` | Версия + `pc_url` + `changelog` (для апдейтов) |
| `setup.sh` | Скрипт поднятия/обновления VPS-раздачи |
| `README.md` | Инструкция |

## Поток
```
configs/landing/  →  (копия)  →  scripts/vps-deploy/public/  →  VPS :8090
```
`version.json` здесь = источник версии, который читает приложение (через
`main.snowden-system.pages.dev` или напрямую).

## Важно
- Это резерв/основа раздачи «в РФ без домена».
- `snowden-portable.zip` — собранный бинарь Windows (обновляется при релизе).