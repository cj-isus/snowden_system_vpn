# Reference projects

Эта папка содержит только карту внешних open-source проектов, которые изучаются
перед реализацией. Репозитории не копируются автоматически в production-код.

## Правила

1. Перед адаптацией читать исходники, документацию, issues и лицензию.
2. Фиксировать upstream URL и конкретный tag/commit перед использованием идеи.
3. Сохранять attribution и выполнять license/security review.
4. Не переносить credentials, telemetry, unsafe fallbacks или неподтверждённые claims.
5. После изучения записывать решение и вывод в `PLAN.md`.

## Почему ссылки, а не полные клоны

Полные репозитории создают большой шум, устаревают и затрудняют проверку
лицензий. GitHub-исходники хорошо читаются через web/API; локальная копия
добавляется только для конкретного компонента после выбора revision и отдельного
разрешения.

## Каталог

| Project | Назначение | Upstream | License | Использование |
|---|---|---|---|---|
| sing-box | embedded runtime, TUN, routing, transports | https://github.com/SagerNet/sing-box | GPL-3.0 | основной runtime reference; pinned revision обязателен |
| sing-box for Android | Android/libbox integration | https://github.com/SagerNet/sing-box-for-android | GPL-3.0 | lifecycle и platform reference |
| Xray-core | VLESS, REALITY и transport reference | https://github.com/XTLS/Xray-core | MPL-2.0 | interop/reference, не второй core по умолчанию |
| Psiphon Tunnel Core | discovery, tactics, server memory, obfuscation | https://github.com/Psiphon-Labs/psiphon-tunnel-core | GPL-3.0 | архитектурный reference; отдельный license review |
| Tor / Pluggable Transports | Snowflake, WebTunnel, obfs4 | https://gitlab.torproject.org/tpo/anti-censorship | mixed; per-component | emergency PT research, не drop-in VPN |
| AmneziaWG | obfuscated WireGuard implementation | https://github.com/amnezia-vpn/amneziawg-go | GPL-2.0 | candidate transport after compatibility review |
| Lantern Samizdat | multiplexed/obfuscated proxy research | https://github.com/getlantern/samizdat | GPL-3.0 | research-only until security review |
| Cloudflare Workers SDK/docs | metadata/control-plane patterns | https://developers.cloudflare.com/workers/ | documentation terms | Worker/Pages/KV/D1 integration reference |

## Review record

Подробные решения, ограничения и статус каждого аналога находятся только в
главном журнале `../PLAN.md`. Этот файл не является вторым источником правды.
