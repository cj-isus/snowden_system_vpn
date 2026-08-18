# STRUCTURE.md — backend/config

> Модуль `snowden-system/backend/config`. Пакет: `package config`.
> ⚠️ КОНФИГ-БИЛДЕР СЕЙЧАС НЕ В ЖИВОМ ПУТИ (legacy): реальный рантайм читает
> шаблоны напрямую через `app.LoadConfigFile` из `assets/configs/`. См. `PLAN.md` A4.

## Назначение (по умолчанию)
Сборка sing-box JSON-конфига из параметров VPS + опциональный split-tunnel
по RU-CIDR (rule-set). Описан в `STRUCTURE.md` (корень) как активный компонент,
но фактически не вызывается живым кодом.

## Файлы
| Файл | Роль | Ключевые типы / функции |
|------|------|--------------------------|
| `builder.go` | Сборка конфига и конвертация CIDR-списка в sing-box source rule-set | `VPSConfig`, `BuildConfig(vps, listenPort, ruCIDRPath)`, `EnsureCIDRFile(rawList, dir)`, `splitCSV`, `trim` |
| — (весь пакет) | — | используется только :  `config.EnsureCIDRFile` из `app.injectSplitTunnel` (которое тоже мёртвое) |

## Реальность
- **Живой путь сборки конфига — в `app.go`**: `LoadConfigFile("template-vps-reality.json")`
  читает файл из `assets/configs/`, возвращает как есть (без подстановки CIDR).
- **`BuildConfig` / `EnsureCIDRFile` / `injectSplitTunnel`** — мёртвый код по `grep`.

## Решение (см. PLAN.md A4, два варианта)
1. Удалить мёртвые функции из `builder.go` (+ поправить `STRUCTURE.md` корневой).
2. Либо вернуть под флагом: один rule-set (не 11 тыс. плоских правил) + `split_tunnel_cidr`.