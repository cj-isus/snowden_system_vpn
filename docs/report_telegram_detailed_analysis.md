# Полный разбор содержимого Telegram-экспорта (ChatExport_2026-07-07)

## 1. Конфигурационные файлы WireGuard/AmneziaWG

### 1.1 `ANTIDOT-X-PRO.conf`
- **Тип**: WireGuard/AmneziaWG конфиг (модификация с obfuscation)
- **Endpoint**: `8.35.211.7:7103`
- **DNS**: 83.220.169.155, 212.109.195.93, 195.133.25.16 (российские/нидерландские)
- **MTU**: 1380
- **AmneziaWG параметры** (обфускация):
  - `Jc = 1`, `Jmin = 100`, `Jmax = 200` — параметры джиттера (скремблирования пакетов)
  - `H1-H4 = 1,2,3,4` — хидеры
  - `S1-S4 = 0` — стартовые значения
- **PublicKey**: `bmXOC+F1FxEMF9dyiK2H5/1SUtzH0JuVo51h2wPfgyo=` — **стандартный ключ Cloudflare WARP!**
- **Address**: `172.16.0.2` — IP из пула WARP
- **Вывод**: Это **WARP+ конфиг через AmneziaWG** (обфусцированный WireGuard). Трафик заворачивается в Cloudflare WARP, но с obfuscation для обхода блокировок протокола.

### 1.2 `SmartTv2.conf` и `SmartTv2 (1).conf`
- **Тип**: Идентичные конфиги WARP+ с AmneziaWG
- **Endpoint**: `8.34.70.7:4233` (Cloudflare WARP relay)
- **DNS**: 45.155.204.190, 37.230.192.51
- **MTU**: 1280
- **AmneziaWG параметры**: `Jc = 4`, `Jmin = 40`, `Jmax = 70`
- **PersistentKeepalive**: 25 сек
- **PublicKey**: Тот же WARP ключ
- **Вывод**: Резервные/альтернативные WARP+ конфиги с другими параметрами obfuscation. Вероятно, для разных провайдеров (разные параметры джиттера).

---

## 2. Текстовые файлы

### 2.1 `mihomo_rules_guide.txt`
- **Тип**: Шпаргалка по правилам Clash.Meta / Mihomo / Clash Verge Rev
- **Объём**: 214 строк, полный справочник
- **Содержание**:
  - **DOMAIN / DOMAIN-SUFFIX / DOMAIN-KEYWORD / DOMAIN-REGEX** — фильтрация по доменам
  - **GEOSITE / GEOIP / IP-ASN** — геолокация и автономные системы
  - **IP-CIDR / IP-CIDR6 / SRC-IP-CIDR** — подсети
  - **DST-PORT / SRC-PORT / IN-PORT** — порты
  - **PROCESS-NAME / PROCESS-PATH** — перехват по Windows-процессам (!)
  - **RULE-SET / SUB-RULE / AND / OR / NOT / MATCH** — логика правил
  - **APPEND / DELETE** — модификация профилей на лету
- **Ключевой insight**: `prepend:` блок в Clash Verge Rev позволяет вставлять правила в начало списка без редактирования основного конфига
- **Для Snowden_system**: Можно использовать как reference для реализации `rules_ru.go` — конвертация Clash-правил в sing-box формат

### 2.2 `mozilla_vpn.cmd`
- **Тип**: Windows batch-скрипт
- **Функция**: Активация встроенного VPN в Firefox (Mozilla VPN / IP Protection)
- **Механика**:
  1. Убивает процесс Firefox
  2. Патчит `user.js` во всех профилях: `browser.ipProtection.enabled = true`
  3. Перезапускает Firefox
- **Примечание**: Требует НЕ прав администратора (явно указано в скрипте)
- **Для Snowden_system**: Firefox IP Protection — это Cloudflare WARP под капотом. Показывает, что обычные пользователи ищут простые решения без админ-прав.

### 2.3 `cidr-string1.lst`
- **Тип**: Список CIDR (IP-подсетей)
- **Объём**: ~470 KB, сплошная строка через запятую
- **Содержание**: Российские IP-диапазоны (начинается с `2.63.0.0/17`, `5.8.0.0/21` и т.д.)
- **Формат**: `IP/mask,IP/mask,IP/mask` (одна строка)
- **Для Snowden_system**: Используется для split-tunneling (itdoginfo-стиль) — российские IP идут напрямую, заблокированные — через VPN. Можно парсить в `rules_ru.go`.

---

## 3. Документы DOCX

### 3.1 `mieru_guide.docx` — Установка mieru/mita на VPS
- **Протокол**: mieru (SOCKS5/HTTP/HTTPS прокси, НЕ TLS)
- **Особенности**:
  - Не использует TLS → не нужны домены и сертификаты
  - Шифрование: XChaCha20-Poly1305 с ключом от логина/пароля/времени
  - **Критично**: требуется синхронизация времени (`timedatectl` → `synchronized: yes`)
  - Поддержка TCP/UDP (BBR для UDP), IPv4/IPv6
- **Установка**: `curl -fSsLO https://raw.githubusercontent.com/enfein/mieru/.../setup.py`
- **Параметры**: порт (например 9000), логин/пароль, протокол TCP
- **Клиенты**: Karing, Shadowrocket (мобильные), официальный CLI
- **Для Snowden_system**: mieru — потенциальный fallback протокол, если VLESS+Reality и Hysteria2 заблокированы. DPI не видит TLS → сложнее детектировать.

### 3.2 `SRTP-сервер-инструкция.docx` — VK Turn Proxy (iOS)
- **Назначение**: Обход блокировки VK звонков через собственный TURN/SRTP сервер
- **Архитектура**:
  - VPS с Ubuntu 22.04/24.04
  - WireGuard (wg0, порт 51820)
  - `vk-turn-proxy` сервер (бинарник `server-linux-amd64` от anton48)
  - SRTP-режим: шифрование медиа-трафика
- **Связка**: `vk-turn-proxy` слушает `0.0.0.0:56000` → заворачивает в WireGuard `127.0.0.1:51820`
- **iOS приложение**: VK Turn Proxy (TestFlight)
- **Для Snowden_system**: Показывает спрос на **специализированные VPN под конкретные сервисы** (VK звонки). Можно учесть как use-case для split-tunneling.

### 3.3 `WDTT-WARP-инструкция.docx` — WDTT сервер + WARP
- **Протокол**: WDTT (WebRTC DataChannel Turn Tunnel)
- **Назначение**: Проксирование VK через WebRTC + WireGuard
- **Установка**:
  - Go 1.23+ (не из apt, а с go.dev)
  - `git clone https://github.com/amurcanov/proxy-turn-vk-android`
  - Сборка: `go build -o /usr/local/bin/wdtt-server server.go`
- **Порты**: DTLS 56000, WG 56001
- **Ссылка подключения**: `wdtt://IP:DTLS_PORT:WG_PORT:9000:PASSWORD:VK_CALL_HASH`
- **Скрытие IP**: Cloudflare WARP (через warp-cli)
- **Для Snowden_system**: WDTT — ещё один нишевый протокол. Показывает, что пользователи комбинируют собственные серверы с WARP для анонимизации IP.

### 3.4 `AntiZapret VPN instrukciya_edited.docx`
- **Проект**: AntiZapret VPN (open-source скрипт для VPS)
- **GitHub**: `GubernievS/AntiZapret-VPN`
- **Функция**: Split-tunneling — только заблокированные сайты через VPN
- **Протоколы**: OpenVPN (UDP/TCP), WireGuard, AmneziaWG
- **Особенности**:
  - Патч для обхода блокировки OpenVPN-протокола
  - OpenVPN DCO (снижение CPU)
  - Cloudflare WARP для исходящего трафика
  - Блокировка рекламы
- **Клиенты**: OpenVPN Connect (iOS/Android), AmneziaWG
- **Для Snowden_system**: Прямой конкурент/аналог. Показывает, что split-tunneling — must-have для РФ. Можно изучить их списки заблокированных доменов.

---

## 4. База данных SQLite

### `wg-tunnel-db-2026-05-27-10_56_54.sqlite3`
- **Приложение**: WG Tunnel (Android WireGuard клиент)
- **Таблицы**:
  - `tunnel_config` — 8 WARP-конфигов (!)
  - `proxy_settings` — SOCKS5/HTTP proxy (отключены)
  - `general_settings` — базовые настройки
  - `auto_tunnel_settings` — автоподключение (отключено)
  - `monitoring_settings` — пинг-мониторинг
  - `dns_settings` — DNS over HTTPS/TLS
- **Конфиги в tunnel_config**:
  - `warpv3_94`, `warpv1_32`, `warpv2_43`, `warpv3_23`, `warpv3_33`, `plwarpv3_36`, `warpw10527`
  - Все с адресом `172.16.0.2` (WARP)
  - Разные приватные ключи
  - AmneziaWG поле (`am_quick`) присутствует у всех
- **Для Snowden_system**: Пользователь активно использует **множество WARP+ конфигов** с AmneziaWG. Подтверждает выбор WARP как основного бесплатного решения.

---

## 5. ZIP-архив

### `ClashMi_nothinn (1).backup.zip`
- **Приложение**: Clash Mi (iOS/macOS Clash клиент)
- **Содержимое**:
  - `service_core_setting.json` — настройки ядра Mihomo
  - `setting.json` — UI настройки
  - `profiles.json` — профиль `205506188.yaml`
  - `profiles/205506188.yaml` — **основной конфиг**
- **Ключевой конфиг** (`205506188.yaml`):
```yaml
# MASQUE прокси через Cloudflare
proxies:
- name: "MASQUE"
  server: 162.159.198.2
  port: 443
  sni: 4pda.to
  type: masque
  private-key: <ECDSA key>
  public-key: <ECDSA pub>
  ip: 172.16.0.2
  ipv6: 2606:4700:110:859b:5a73:ede5:2d3e:f17a
  
- name: "MASQUE h2"
  server: 162.159.198.2
  port: 443
  sni: 4pda.to
  network: h2  # HTTP/2
```
- **Правила**:
  - `DOMAIN-KEYWORD,nextdns,DIRECT`
  - `DOMAIN-SUFFIX,2ip.ru,DIRECT`
  - `DOMAIN-KEYWORD,yandex,DIRECT`
  - `GEOIP,RU,DIRECT`
  - `MATCH,WARP` — всё остальное через WARP
- **TUN**: Включён, stack gvisor, MTU 4064
- **DNS**: fake-ip, Cloudflare DoH
- **Для Snowden_system**:
  - **MASQUE** — современный протокол обфускации (HTTP/3 или HTTP/2 через QUIC/TLS)
  - `sni: 4pda.to` — domain fronting на популярный русский форум
  - Правила: российский трафик напрямую, всё остальное через WARP

---

## 6. HTML-переписка (messages.html — messages8.html)

### Общая статистика:
| Файл | Символов | URL |
|------|---------|-----|
| messages1 | 115,724 | 87 |
| messages2 | 116,107 | 55 |
| messages3 | 88,955 | 14 |
| messages4 | 95,922 | 32 |
| messages5 | 112,030 | 28 |
| messages6 | 79,292 | 27 |
| messages7 | 92,228 | 21 |
| messages8 | 67,963 | 39 |
| **ИТОГО** | **~768K** | **303** |

### Основные темы переписки:
1. **WARP/WARP+/WARP Generator** — генерация конфигов, обсуждение скорости
2. **AmneziaWG** — установка на роутеры (Keenetic, OpenWrt), Windows, Android
3. **Clash/Mihomo** — настройка правил, обход блокировок
4. **MASQUE** — новый протокол для обхода
5. **VK/Turn Proxy/SRTP** — звонки через блокировки
6. **VPS/Хостинги** — Hetzner, DigitalOcean, Vultr, Aeza, VDSina
7. **Роутеры** — Keenetic OS 5.1, OpenWrt, entware

---

## 7. GitHub-репозитории (извлечённые из URL)

### VPN-ядра и клиенты:
| Репозиторий | Описание |
|------------|----------|
| `github.com/MetaCubeX/ClashMetaForAndroid` | Clash.Meta для Android |
| `github.com/MetaCubeX/mihomo` | Ядро Mihomo (Clash.Meta) |
| `github.com/clash-verge-rev/clash-verge-rev` | Clash Verge Rev GUI |
| `github.com/amnezia-vpn/amnezia-client` | Amnezia VPN клиент |
| `github.com/amnezia-vpn/amneziawg-android` | AmneziaWG для Android |
| `github.com/amnezia-vpn/amneziawg-windows-client` | AmneziaWG для Windows |
| `github.com/amnezia-vpn/amneziawg-linux-kernel-module` | Kernel модуль AWG |
| `github.com/RomikB/amneziawg-windows-client` | Форк AWG Windows |
| `github.com/vayulqq/amneziawg-windows-client` | Ещё форк |
| `github.com/spvkgn/amneziawg-android` | Форк AWG Android |
| `github.com/hoaxisr/awg-manager` | Менеджер AWG конфигов |
| `github.com/smkuzmin/mikrotik-bypassing-blocks-using-amneziawg` | MikroTik + AWG |
| `github.com/smkuzmin/mikrotik-installing-openwrt-youtubeunblock` | MikroTik + OpenWrt |
| `github.com/smkuzmin/routerich-amneziawg-youtubeunblock` | Routerich + AWG |
| `github.com/Slava-Shchipunov/awg-openwrt` | AWG для OpenWrt |
| `github.com/Leadaxe/LxBox` | **LxBox — форк sing-box с AmneziaWG!** |
| `github.com/shtorm-7/sing-box-extended` | Расширенный sing-box |
| `github.com/hiddify/hiddify-app` | Hiddify (GUI для Xray/sing-box) |
| `github.com/throneproj/Throne` | Throne (форк NekoBox) |
| `github.com/qr243vbi/nekobox` | Ещё форк NekoBox |
| `github.com/steve228uk/TunnelDeck` | WireGuard для Steam Deck |
| `github.com/wgtunnel/wgtunnel` | WG Tunnel Android |
| `github.com/wgtunnel/desktop` | WG Tunnel Desktop |
| `github.com/Vadim-Khristenko/AmneziaWG-Architect` | Конструктор AWG |
| `github.com/pluralplay/FlClashX` | FlClash для macOS |
| `github.com/MrWaip/vpn-deck` | VPN Deck |

### WARP/WARP+ генераторы:
| Репозиторий | Описание |
|------------|----------|
| `github.com/codelabhq/clash-warp-config` | Clash + WARP конфиги |
| `github.com/nellimonix/warp-config-generator-vercel` | Генератор WARP (llimonix) |
| `github.com/ildarmaga/wdtt` | WDTT протокол |
| `github.com/luminescq/PWDTT` | Python WDTT |
| `github.com/openwarpkit/warp-relay` | WARP Relay |
| `github.com/zieng2/wl` | WARP Link? |

### Обход блокировок (специфические):
| Репозиторий | Описание |
|------------|----------|
| `github.com/flowseal/zapret-discord-youtube` | Zapret (GoodbyeDPI аналог) |
| `github.com/romanvht/ByeByeDPI` | ByeByeDPI для Android |
| `github.com/Internet-Helper/mixomo-openwrt` | Mixomo для OpenWrt |
| `github.com/StressOzz/Mixomo-Manager` | Mixomo Manager |
| `github.com/masterking32/MasterDnsVPN` | DNS VPN |
| `github.com/igareck/vpn-configs-for-russia` | Конфиги для РФ |
| `github.com/anton48/vk-turn-proxy-ios` | VK Turn Proxy iOS |
| `github.com/cacggghp/vk-turn-proxy` | VK Turn Proxy |
| `github.com/amurcanov/tg-ws-proxy-android` | Telegram WS Proxy Android |
| `github.com/Flowseal/tg-ws-proxy` | Telegram WS Proxy |

### Утилиты:
| Репозиторий | Описание |
|------------|----------|
| `github.com/bloodwolfik/Sites` | Список сайтов |
| `github.com/bloodwolfik/config-constructor` | Конструктор конфигов |
| `github.com/noVibe/DnsConf` | DNS конфигуратор |
| `github.com/sageptr/mini_quic_generator` | QUIC генератор |
| `github.com/nikkinikki-org/OpenWrt-nikki` | OpenWrt + Mihomo |
| `github.com/stunndard/golangwin7patch` | Патч Go для Windows 7 |

### Сервисы:
| URL | Описание |
|-----|----------|
| `warp-generator.github.io` | Веб-генератор WARP |
| `warp-mirrors.vercel.app` | Зеркала WARP |
| `warp3.llimonix.pw` | llimonix WARP API |
| `awgwarp.notion.site` | База знаний AWG/WARP |
| `vpnstatus.site` | Статус VPN |
| `ssclash.notion.site` | Документация SSClash |
| `help-guide.notion.site` | Proton/WARP гайды |
| `dns-conf-ui.vercel.app` | DNS конфиг UI |
| `voidwaifu.github.io/Special-Junk-Packet-List/` | Списки пакетов |

---

## 8. Telegram-каналы и боты

### Каналы и группы:
- `@warp_1_1_1_1_chat` — Обсуждение WARP
- `@itdogchat` — ITDog сообщество
- `@join_codelab` — Code Lab форум
- `@bypassblock` — Обход блокировок
- `@blokirovki_runeta` — Блокировки Рунета
- `@happ_proxy` — Happ Proxy
- `@wiresock` — WireSock
- `@hosterobzorgroup` — Хостинги
- `@findllimonix_chat` — llimonix support

### Боты:
- `@warp_generator_bot` — Генератор WARP конфигов
- `@wg2awg_bot` — Конвертер WG → AWG
- `@TriBukvyRoBot` — Три буквы (игра?)
- `@ExpressHost_Bot` — Хостинг
- `@nodehost_bot` — Node хостинг
- `@sidylinkbot` — SidyLink

---

## 9. Приложения (App Store)

| Приложение | ID | Назначение |
|-----------|-----|-----------|
| AmneziaWG | `id6478942365` | Обфусцированный WireGuard |
| Happ Proxy Utility | `id6783623643` | Прокси утилита |
| AmneziaVPN | `id1600529900` | Amnezia VPN |
| Clash Mi | `id6744321968` | Clash клиент |
| DefaultVPN | `id6744725017` | VPN клиент |
| Shadowrocket | `id932747118` | Универсальный прокси-клиент |

---

## 10. Ключевые выводы для Snowden_system

### Что пользователи реально используют:
1. **WARP+ с AmneziaWG** — основное бесплатное решение (8 конфигов в БД!)
2. **Clash/Mihomo с правилами** — split-tunneling через GEOIP/DOMAN-KEYWORD
3. **MASQUE** — новейший протокол (HTTP/3 через Cloudflare)
4. **mieru** — без-TLS решение для максимальной скрытности
5. **Собственные VPS** — для VK Turn Proxy, WDTT, AntiZapret

### Архитектурные паттерны:
- **Split-tunneling обязателен**: `GEOIP,RU,DIRECT` + `MATCH,PROXY`
- **Обфускция WG**: AmneziaWG с параметрами Jc/Jmin/Jmax/H1-H4
- **Domain fronting**: `sni: 4pda.to` для MASQUE
- **WARP как базовый слой**: IP 172.16.0.2, ключ `bmXOC+F1FxEMF9dyiK2H5/1SUtzH0JuVo51h2wPfgyo=`
- **Множество конфигов**: пользователи хранят 5-10 резервных конфигов

### Готовые интеграции:
- **LxBox** (`Leadaxe/LxBox`) — sing-box с AmneziaWG → можем форкнуть
- **Clash Verge Rev Service Mode** — запуск под SYSTEM
- **WG Tunnel** — Android клиент с AmneziaWG
- **Clash Mi** — iOS с MASQUE

### Риски:
- llimonix перегружен (упоминание в чате: "разработчики портала выставили релеи ллимоникса за свои, туда вышло куча трафика и лимону была пизда")
- WARP тел-серверы лагают
- Требуется постоянное переподключение для скорости
