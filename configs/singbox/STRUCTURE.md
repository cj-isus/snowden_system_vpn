# STRUCTURE.md — configs/singbox/ (конфиги sing-box)

> Источник истины sing-box-конфигов. Синкается в `windows/assets/configs/`.
> Реальный путь чтения приложением: `app.LoadConfigFile` → `assets/configs/<name>`.

## Файлы (рабочие)
| Файл | Назначение | Примечания (паттерн) |
|------|-----------|----------------------|
| `template-vps-reality.json` | **Основной конфиг ПК**: urltest-группа `auto` из 7 VPS-каналов (NL/FR) + split-tunnel | `inbounds`: mixed 127.0.0.1:20808; `route.final=auto` |
| `template-vps-reality.json.example` | Тот же, с плейсхолдерами `YOUR_*` | публикуется |
| `template-warp-awg.json` | AmneziaWG (WARP) standalone | endpoint `warp-awg`, `i1` — placeholder; `jc/jmin/jmax/s1..4/h1..4` |
| `template-reality.json` | Упрощённый Reality-шаблон | legacy/тест |
| `test-reality-20810.json` | Тестовый конфиг для `enginetest/e2e.go` | порт 20810 |
| `server-params.json` / `.example` | Параметры личного VLESS VPS | секрет |
| `server2-params.json` / `warp-keys.json` / `.example` | Доп. параметры / WARP-ключи | секрет |
| `warp-outbound.json` / `warp2.json` / `warp-amnezia-vpn.conf` | WARP-инструменты/выгрузки | разные форматы |
| `ru-cidr.lst` | RU CIDR для split-tunnel (11 401 правило) | сейчас не инжектится (см. PLAN A4) |
| `mieru-credentials.json`, `mieru-fr-credentials.json` | Логин/пароль mieru-сервера | секрет; пока не в движке (см. PLAN B4) |

## Ключевой шаблон: `template-vps-reality.json`
- **DNS**: https `cloudflare` (1.1.1.1, detour auto) + local; `strategy: ipv4_only`.
- **Inbounds**: `mixed` на `127.0.0.1:20808` (это системный прокси-порт).
- **Outbounds `auto` (urltest)**: `grpc-nl`, `grpc-fr`, `httpupgrade-nl/fr`,
  `vless-nl/fr`, `hysteria2-nl`, `direct`, `block`. `url: gstatic/generate_204`,
  `interval:30s`, `tolerance:50`.
- **Route rules**: sniff → hijack-dns → `domain_suffix .ru/.su/.рф + банки` = direct
  (split-tunnel) → youtube/google/tg/discord/ai/social/streaming → `auto` →
  `ip_is_private`=direct → telegram IP-CIDR → `final: auto`.

## Важно
- ⚠️ Правило: source of truth здесь; правки → `bash configs/sync-to-windows.sh`.
- WARP сейчас НЕ в основном конфиге (отдельный шаблон) — см. PLAN.md B1.
- `i1` в `template-warp-awg.json` — заглушка под генерацию (см. PLAN.md B3).