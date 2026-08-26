# snowden.system — строгий план построения с нуля

> Версия: 2026-08-26.
>
> Документ является master-plan для greenfield-разработки. Он описывает не
> обещание «обхода любых блокировок», а систему с максимально высокой
> практической устойчивостью против заранее измеренных классов отказа.
>
> **Критическое правило:** никакая схема не даёт 100% гарантии. Оператор может
> блокировать IP/ASN, домен, SNI, TLS fingerprint, CDN, протокол, приложение,
> все внешние маршруты или канал доставки конфигурации. Мы гарантируем только
> поведение системы: она выбирает исключительно подтверждённый канал и при его
> отказе не выпускает protected traffic напрямую.

---

## 0. Управление знаниями проекта и журнал разработки

`PLAN.md` — **единственный главный источник правды для намерений, требований,
решений, текущего статуса и истории разработки**. После сжатия контекста,
перезапуска сессии или передачи задачи другой модели первым обязательным шагом
является повторное чтение этого файла; нельзя продолжать работу по памяти,
старым сообщениям или предположениям.

### 0.1. Обязательный дневник

В этом же файле ведётся append-only журнал. В него записывается всё, что
страшно потерять:

- что выполнено и чем проверено;
- какой результат получен без проблем;
- встреченная трудность или дефект;
- блокер, его причина, владелец и влияние;
- принятое решение и отвергнутые альтернативы;
- изученный готовый аналог и решение: адаптировать/скопировать идею,
  улучшить или отказаться;
- незавершённый следующий шаг и точный критерий закрытия.

Каждая запись содержит дату, статус (`done`, `partial`, `blocked`, `decision`),
затронутый компонент, доказательство и следующий шаг. Секреты, credentials,
сырые приватные логи и чувствительные endpoint details в дневник не заносятся.
Нельзя переписывать историю так, чтобы исчезли прежние трудности; исправление
делается новой записью с ссылкой на предыдущую.

### 0.2. Правило работы после восстановления контекста

Перед любой правкой после context compression необходимо:

1. прочитать `PLAN.md` полностью или релевантный раздел и последний дневник;
2. проверить `git status --short --branch`;
3. сверить заявленный статус с кодом, тестами и live-evidence;
4. только затем планировать и выполнять изменения.

Если важный факт не записан в `PLAN.md`, он считается потерянным для следующей
сессии и должен быть добавлен до продолжения.

### 0.3. Обязательное обновление правил и журнала

Любое новое устойчивое правило проекта сначала фиксируется в `PLAN.md`, а затем
при необходимости синхронизируется в `AGENTS.md` и специализированной
документации. Это относится к:

- новому обязательному поведению продукта;
- новой границе безопасности или правилу работы с секретами;
- новому архитектурному контракту между UI, backend, native runtime или config;
- новому формату данных, lifecycle state, API или источнику метрик;
- новому обязательному тесту, live-check или release criterion;
- запрету конкретной реализации, fallback, зависимости или инструмента;
- уточнению, предотвращающему повторение найденного дефекта;
- новому решению по производительности, ownership, cancellation, timeout или
  cleanup.

Для каждого правила в дневнике указываются:

```text
ID
дата
приоритет P0/P1/P2
формулировка «обязательно/запрещено»
владелец
причина
способ проверки
статус
связанные компоненты/документы
```

Правило нельзя считать принятым только потому, что оно обсуждалось в чате.
Пока оно не записано в `PLAN.md`, оно не является обязательным контрактом для
следующей сессии. Если правило изменяет поведение уже существующего кода,
сначала обновляются план и тест-критерий, затем код.

### 0.4. Reuse-first и исследование аналогов

Мы не выдумываем код там, где существует зрелое открытое решение. До новой
реализации обязательно:

1. найти 2–3 релевантных аналога;
2. изучить исходный код, документацию, issue/ограничения и лицензию;
3. определить, существует ли готовый компонент, который можно безопасно
   адаптировать;
4. сравнить его с текущим стеком и threat model;
5. записать в дневник решение: адаптированно скопировать паттерн, использовать
   зависимость, сделать собственную реализацию лучше или отказаться, с причиной;
6. только после этого писать код.

«Похожее название» или tutorial не считается исследованным аналогом. Копируем
только совместимый по лицензии и безопасности код, сохраняем attribution и
фиксируем upstream revision. Не переносим credentials, telemetry, небезопасные
fallbacks или неподтверждённые claims.

### 0.4. Компонентность и производительность

Код проектируется по bounded-компонентам с одной ответственностью:

```text
UI → typed facade → application service → domain policy → adapter → platform/engine
```

Требования: явные ownership/lifecycle, bounded queues, cancellation, timeout,
backpressure, отсутствие лишних копирований, неизменяемые snapshots, lazy
initialization, кэширование только с доказанной пользой, профилирование перед
оптимизацией и отсутствие premature micro-optimizations. Каждая оптимизация
должна иметь benchmark/profile до и после, не ухудшать безопасность и не
ломать fail-closed. Производительность измеряется отдельно для startup,
probe, steady-state throughput, memory, CPU и reconnect.

### 0.5. Рекомендуемый greenfield-стек

До принятия альтернативы базовый стек такой:

- **Windows:** Go + embedded pinned sing-box/libbox; Wails/Vue/TypeScript только
  как UI boundary; domain policy и lifecycle без UI-зависимостей.
- **Android:** Kotlin `VpnService`/foreground lifecycle + pinned libbox AAR;
  Flutter/Dart для presentation и typed facade, pure domain logic в Dart/Kotlin
  tests.
- **iOS:** не входит в продукт и не разрабатывается; iOS-код, требования и
  acceptance criteria отсутствуют в scope.
- **Control-plane:** Cloudflare Worker/Pages/KV/D1 только для metadata, health и
  opt-in telemetry; credentials — вне публичного plane.
- **Provisioning/ops:** Python для read-only audit и явно разрешённых операций;
  shell — только для локальных проверок; IaC/Ansible/Terraform вводить после
  стабилизации контрактов и с dry-run.
- **Contracts:** versioned JSON Schema/OpenAPI-подобные typed contracts,
  Ed25519-signed metadata, redacted evidence JSON, SBOM и pinned dependencies.
- **Testing:** Go/Dart/Kotlin unit tests, local deterministic harness,
  integration tests с test servers, operator/device acceptance отдельно.

Не добавлять второй VPN core только ради большего списка протоколов. Сначала
доказать, что текущий core не покрывает требование; затем провести interop,
license, binary-size, crash/lifecycle и security review.

## 1. Журнал разработки

### 1.1. Запись 2026-08-26 — очистка greenfield

> До начала первой реализации нужно закрыть операционный preflight: корректно
> сформировать минимальный репозиторий и исключить generated/dependency files из
> staging. Сейчас legacy-файлы помечены на удаление, но физическая очистка
> Windows/Android каталогов ранее столкнулась с блокировками файлов; это
> отдельный технический блокер, а не завершённая очистка.

| Поле | Решение |
|---|---|
| Статус | decision |
| Цель | удалить старую реализацию и начать разработку с нуля |
| References | внешние проекты вынесены в `references/` как ссылки и манифест |
| Почему не клонируем всё | полные клоны создают шум и устаревают; локальная копия нужна только после выбора revision |
| Ограничение | удаление файлов из рабочего дерева необратимо для незакоммиченных изменений; перед выполнением требуется явное подтверждение владельца |
| Следующий шаг | завершить очистку после остановки процессов, проверить staging, затем commit/push только после финальной проверки |

### 1.2. Запись 2026-08-26 — scope платформ

| Поле | Решение |
|---|---|
| Статус | decision |
| Обязательно | разрабатываются только Windows и Android |
| Запрещено | не разрабатывать iOS и не добавлять iOS acceptance criteria |
| Причина | подтверждено владельцем проекта |
| Проверка | отсутствие iOS в roadmap, scope и production requirements |


| Дата | Статус | Компонент | Факт/решение | Доказательство | Следующий шаг |
|---|---|---|---|---|---|
| 2026-08-26 | done | План | Greenfield master-plan расширен строгими требованиями, серверным baseline, Cloudflare-порядком, закупками и evidence-gates | текущий `PLAN.md`, `node --check`, `git diff --check` | вести журнал в этом файле при каждом значимом этапе |
| 2026-08-26 | decision | Источник правды | `PLAN.md` — единственный master-документ; остальные документы являются справочными и не могут переопределять этот план | это правило | синхронизировать устаревшие ссылки/статусы по мере затрагивания |
| 2026-08-26 | decision | Процесс разработки | Перед новой реализацией сначала изучать готовые аналоги, лицензию, issues и benchmark; затем адаптировать или улучшать только по доказанной причине | reuse-first правило выше | применять к каждому новому компоненту |
| 2026-08-26 | decision | Платформы | Разрабатывать только Windows и Android; iOS исключён из scope | запись 1.2 | не добавлять iOS-код и критерии |
| 2026-08-26 | blocked | Greenfield preflight | После очистки и push рабочее дерево всё ещё содержит физически оставшиеся legacy Android/Windows файлы, помеченные к удалению; они не вошли в опубликованный commit из-за занятых/заблокированных файлов | `git status --short --branch` после push | закрыть preflight отдельной очисткой после остановки процессов и нового commit |
| 2026-08-26 | done | Greenfield preflight | Локальные legacy-каталоги (windows/, ios/, configs/, scripts/, docs/, assets/) физически удалены; git index очищен до 6 greenfield-файлов; android/ пуст и заблокирован Java-сервисом (kosmeticheskoe, не отслеживается Git); .freebuff/ корректно игнорируется | физическая структура, `git ls-files`, `git status`, `git check-ignore`, `git grep` | начать Phase 0 |

## 2. Цели, границы и определения

### 0.1. Цель продукта

```text
Пользователь нажимает Connect.
Система проверяет доступный protected channel.
Если канал подтверждён — трафик идёт через него.
Если канал не подтверждён — система показывает причину и блокирует protected traffic.
При отказе одного канала система выбирает другой validated channel.
```

### 0.2. Что не является целью

- гарантия работы при полном отключении международного трафика;
- обещание обхода любого будущего DPI;
- публикация credentials в Worker/KV/Pages;
- автоматическое включение неподтверждённых протоколов;
- использование прямого соединения как скрытого fallback;
- маскировка ошибки под «подключено»;
- сбор лишних персональных данных;
- добавление чужого кода без license/security review.

### 0.3. Статусы доказательности

| Статус | Значение |
|---|---|
| `planned` | идея или ссылка на внешнее решение |
| `configured` | создана конфигурация, live path не доказан |
| `locally-tested` | проверен код/стенд, но не операторская сеть |
| `live-verified` | handshake + protected DNS + HTTPS подтверждены на tuple |
| `degraded` | ранее работал, текущий результат хуже/неполон |
| `blocked` | отказ воспроизведён в конкретной сети и профиле |
| `retired` | отключён из-за риска, лицензии или плохого результата |

`live-verified` всегда привязан к tuple:

```text
channel_id + core_version + profile_version + device + OS + operator/network +
test_location + timestamp + handshake + protected_dns + protected_https + duration
```

---

## 1. Фактчек и исходный baseline

### 1.1. Что известно о текущей инфраструктуре

- Cloudflare account существует и содержит зоны `snowden.dpdns.org` и
  `snowden-vpn.us.kg`.
- Обе зоны на последней проверке имели статус `pending`.
- `snowden.dpdns.org` публично всё ещё делегирован на DigitalPlat, а не на
  Cloudflare NS.
- `snowden-vpn.us.kg` публично не резолвится.
- Account-level Worker `snowden-system-api`, KV namespaces и D1 database
  существуют; их наличие не доказывает рабочий CDN/VPN path.
- Pages project существует, custom domain не подключён.
- Старый VPS `89.125.1.217` имеет подтверждённый Hysteria2 UDP/8443 и старый
  сертификат для `89-125-1-217.nip.io`.
- Успешный HY2 handshake и protected HTTPS не подтверждены.
- Документы о VLESS/TCP/443 расходятся; этот транспорт нельзя считать
  существующим до новой проверки фактического runtime-конфига.
- `snowden.live` исключён из актуальной схемы и не должен использоваться.

### 1.2. Что подтверждено внешними источниками

Использованы официальные документы и независимые полевые отчёты:

- Cloudflare сообщал о российских resets/timeouts и throttling примерно до
  16 KB для Cloudflare-protected соединений; это затрагивает TCP/TLS и QUIC.
- Net4People/BBS issue #490 описывает freeze после примерно 15–20 KB, а также
  SNI/CIDR allowlists и различия между операторами. Это field report, а не
  универсальная константа.
- 4PDA/NTC/GitHub reports показывают, что REALITY/VLESS работают
  неравномерно: разные операторы и сети дают разные результаты.
- Официальная документация Cloudflare подтверждает WebSockets, но также
  connection/idle limits, необходимость keepalive/reconnect и закрытие
  соединений при edge updates.
- Cloudflare Spectrum — отдельный платный продукт для произвольного TCP/UDP;
  обычный Free Website proxy не является raw UDP proxy.
- Psiphon подтверждает ценность concurrent attempts, signed server entries,
  tactics, out-of-band delivery и memory успешных способов.
- Tor Snowflake/WebTunnel/obfs4 — зрелая pluggable-transport экосистема, но
  это не drop-in replacement для обычного VPN.
- AmneziaWG, Xray/REALITY и другие проекты полезны как open-source reference,
  но их claims о «невидимости» не принимаются без собственной приёмки.

### 1.3. Что исключено как мусор

- проценты обнаружения без воспроизводимой методики;
- «неотличим», «невозможно обнаружить», «оператор не может заблокировать CDN»;
- один успешный комментарий как статистика всей страны;
- generic tutorial без сети, даты, устройства и protected-traffic результата;
- port-open, DNS-only, TLS-only, HTTP 404, `sing-box check` как доказательство
  VPN.

---

## 2. Строгие функциональные требования

### FR-001. Protected routing

1. Все направления делятся на:
   - явно разрешённые direct private/RU направления;
   - protected направления через selector;
   - block/service rules.
2. `direct` не входит в protected selector, fallback или recovery.
3. При отсутствии validated channel protected traffic блокируется.
4. UI не показывает `running` как доказательство внешнего доступа: внешний
   probe имеет отдельное состояние.

### FR-002. Channel registry

Каждый канал обязан иметь descriptor:

```text
id
transport/core
server reference
port
hostname/SNI reference
profile version
capabilities
failure domain
credential reference (never credential value)
enabled
status
last verified tuple
```

В UI попадают только enabled channels, реально присутствующие в runtime и
прошедшие validation. Никаких hardcoded стран, серверов и протоколов.

### FR-003. Transport adapter

Каждый внешний core реализует:

```text
ValidateProfile
Capabilities
Start
Stop
Reload
ProbeProtected
Diagnostics
RedactError
```

В production допускается только adapter, прошедший license review,
reproducible build, lifecycle tests и live acceptance.

### FR-004. Probe

Probe обязан подтверждать:

```text
channel selected
protected DNS resolved
HTTPS target returned expected status
traffic did not use direct fallback
```

Обычный host HTTPS без доказательства маршрута не принимается. Для каждого
канала нужны минимум два HTTPS target и DNS target, чтобы не спутать captive
portal или частичный доступ с VPN.

### FR-005. Failover

- одновременно выполняется не более одного recovery/reload;
- канал A считается failed после двух подтверждённых protected failures;
- HalfOpen требует двух успешных protected probes;
- если все каналы failed — `BLOCKED`, не direct;
- событие failover записывает только безопасный channel id, category и latency.

### FR-006. Credential security

- credentials вводятся через локальный provisioning;
- публичные metadata endpoints credentials не возвращают;
- credentials не попадают в logs, diagnostics, crash reports, tests и snapshots;
- leaked credentials немедленно считаются скомпрометированными и перевыпускаются;
- server entries подписываются или проверяются pinning/public key;
- обновление конфигурации имеет version, expiry, rollback и anti-downgrade.

### FR-007. Offline/blocked control-plane

Клиент обязан иметь:

```text
last-known-good signed metadata
несколько metadata delivery endpoints
ручной QR/file import
проверку срока и подписи
rollback
```

Нельзя обещать автоматическое восстановление, если одновременно заблокированы
все tunnel и все каналы доставки профиля.

### FR-008. Lifecycle

Windows:

```text
Start → validate → Engine.Start → protected probe → running/blocked
Reload → validate → serialized Engine.Reload → probe
Stop → stop adaptive/metrics → Engine.Close → clear proxy
```

Android:

```text
permission → foreground → libbox setup → command server → TUN → protected probe
→ running или fail-closed cleanup
```

Все операции имеют bounded timeout, panic/error boundary и единый cleanup path.

---

## 3. Целевая архитектура каналов

### 3.1. Минимально необходимая diversity

```text
Channel A — VLESS + WebSocket + TLS через CDN → VPS-1
Channel B — VLESS + REALITY/XHTTP → VPS-2, другой provider/ASN
Channel C — HY2/TUIC или другой UDP transport → VPS-3/другая сеть
Channel D — отдельный emergency PT/backend после отдельного review
```

Два порта на одном VPS не считаются независимыми. Два домена на одном IP не
считаются полным failover.

### 3.2. Приоритет реализации

1. **A: VLESS+WS через CDN** — против прямой блокировки origin IP. Нужны
   активная зона, корректная DNS delegation, origin и live test.
2. **B: REALITY/XHTTP reference** — против зависимости от CDN/домена. Нужен
   другой failure domain; не считать невидимым.
3. **C: HY2/TUIC** — как быстрый канал там, где UDP доступен. Не решает UDP/IP
   block и не проходит через обычный Cloudflare proxy.
4. **D: Psiphon/Tor PT** — отдельная экспериментальная ветка. Использовать
   идеи Psiphon для discovery/tactics/memory, но не смешивать разные trust и
   privacy модели без отдельного дизайна.
5. **AmneziaWG** — только после проверки точного server/client runtime.
6. **Samizdat** — research-only до независимого security/performance review.

### 3.3. Core strategy

Основной runtime остаётся `sing-box/libbox`, потому что проект уже строится
вокруг embedded TUN и lifecycle. Xray не встраивается вторым core без
необходимости: сначала interop adapter или supervised external process.

Любая новая зависимость фиксируется:

```text
repository
commit/tag
license
SBOM
build recipe
security advisories
supported platforms
known limitations
```

---

## 4. Что купить/получить до разработки

### 4.1. Обязательное

#### Инфраструктура

- **VPS-1** — текущий сервер можно использовать только после свежего baseline.
- **VPS-2** — другой provider и ASN, отдельная география или сеть для REALITY.
- **VPS-3** — желательно другой provider/ASN для UDP/HY2/TUIC или третьего
  TCP transport.
- минимум IPv4 на каждом VPS;
- IPv6 — желательно, но не включать в production profile до проверки;
- root/SSH access с host-key verification;
- provider firewall/security-group management;
- snapshots/backups и возможность быстро заменить IP;
- SLA и billing, позволяющие заменить VPS при блокировке IP.

#### Домены и DNS

- одна основная доменная зона под CDN;
- минимум 2–4 резервные доменные зоны у разных регистраторов или DNS-провайдеров;
- доступ к registrar NS delegation;
- DNSSEC — включать только после проверки registrar/Cloudflare compatibility;
- не использовать домен, где DNS нельзя оперативно изменить;
- не публиковать origin IP в публичных metadata и клиентских профилях.

#### Устройства и сети

- текущий Android Honor/Android 16;
- второй Android OEM;
- Windows 10/11 машина;
- SIM MegaFon;
- SIM минимум другого мобильного оператора;
- независимый домашний Wi‑Fi;
- возможность измерять IPv4-only и IPv6-capable сети;
- безопасный тестовый endpoint/HTTP 204 target.

### 4.2. Доступы и credentials

Нужно получить и хранить локально, не в Git:

```text
Cloudflare account access
Zone DNS Read
Zone Settings Read
отдельный короткоживущий DNS write token
registrar access
SSH key/password для VPS
provider console access
server protocol credentials
certificate/private keys
```

Origin private key, UUID, HY2/VLESS passwords и SSH credentials никогда не
записываются в PLAN.md, чат, Worker KV или публичный API. Если ключ уже
публиковался — сначала перевыпуск, потом настройка.

### 4.3. Не покупать преждевременно

До успешной приёмки A/B не нужны:

- Cloudflare Spectrum;
- десятки VPS;
- десятки доменов;
- платные панели «one click VPN»;
- непроверенные anti-DPI подписки;
- новый core ради списка протоколов;
- expensive telemetry/analytics.

Сначала доказать два независимых канала, затем масштабировать.

---

## 5. Информация и требования к текущему серверу

### 5.1. Известный baseline

```text
IP                    89.125.1.217
OS                    Linux (точная версия — повторно снять)
sing-box              active/enabled по прошлому audit
listener              Hysteria2 UDP 0.0.0.0:8443
TLS                   старый SAN 89-125-1-217.nip.io
firewall              UDP 8443 ранее разрешён
TCP 443               документы противоречат друг другу
nginx                 не подтверждён в последнем baseline
VLESS+WS              не подтверждён
VLESS+REALITY         не подтверждён
client handshake      не принят
protected HTTPS       не принят
```

### 5.2. Обязательный read-only audit перед изменениями

```bash
hostnamectl
cat /etc/os-release
sing-box version
sing-box check -c /etc/sing-box/config.json
systemctl is-active sing-box
systemctl is-enabled sing-box
ss -ltnup
ufw status verbose
nft list ruleset
journalctl -u sing-box --since "30 minutes ago" --no-pager
openssl x509 -in <cert> -noout -subject -issuer -dates -ext subjectAltName
```

Сохранить только redacted metadata. Не копировать runtime config, private key,
UUID или passwords в репозиторий.

### 5.3. Безопасная миграция VPS-1

Нельзя вслепую перезаписывать текущий runtime. Порядок:

1. backup с проверкой восстановления;
2. снять baseline и сохранить hash только metadata-safe файлов;
3. выбрать владельца TCP/443: nginx или sing-box, не оба;
4. настроить отдельный localhost VLESS+WS backend;
5. установить корректный сертификат выбранного hostname;
6. проверить `sing-box check` и `nginx -t`;
7. открыть только необходимые порты;
8. выполнить controlled reload, не restart без необходимости;
9. проверить server logs во время реального клиента;
10. rollback при ухудшении.

Изменения сервера выполняются только после отдельного явного разрешения. Этот
план сам по себе не является разрешением на SSH, deploy, restart или DNS write.

---

## 6. Cloudflare: идеальная настройка по шагам

### 6.1. Выбор зоны

Выбрать одну зону как primary. Текущий рекомендуемый кандидат —
`snowden.dpdns.org`, но только если DigitalPlat позволяет изменить NS.
`snowden-vpn.us.kg` оставить резервом до появления публичной делегации.

### 6.2. Порядок DNS delegation

1. В Cloudflare убедиться, что zone создана в нужном account.
2. На registrar заменить authoritative NS на выданные Cloudflare NS.
3. Дождаться обновления parent delegation.
4. Проверить NS минимум через несколько независимых резолверов.
5. Дождаться `active` в Cloudflare.
6. Только после `active` создавать/проверять production DNS records.

### 6.3. DNS records

Для CDN-канала:

```text
A @    → origin IPv4, Proxied
A www  → origin IPv4, Proxied (только если нужен web alias)
AAAA   → не создавать, пока IPv6 не настроен и не протестирован
```

Не публиковать `nip.io` как SNI для нового домена. Не делать DNS-only запись
для клиентского CDN endpoint. Origin IP всё равно нужно считать скомпрометированным,
если он уже был опубликован: CDN не возвращает прошлую конфиденциальность.

### 6.4. SSL/TLS

- client → Cloudflare: Full (strict) после проверки origin certificate;
- origin certificate SAN должен содержать exact chosen hostname;
- TLS 1.2/1.3 с безопасными defaults;
- не использовать `insecure` в production;
- проверить certificate renewal и rollback;
- origin cert private key не помещать в KV/Worker/репозиторий.

### 6.5. WebSocket

- включить WebSockets;
- backend path согласовать в одном manifest;
- добавить heartbeat/ping и reconnect;
- не считать HTTP 404 без Upgrade ошибкой VLESS;
- не считать Upgrade 101 без VLESS auth и protected HTTPS успехом;
- учитывать Cloudflare connection/idle limits;
- тестировать длительность и edge reconnect.

### 6.6. Cloudflare API/Worker

Требования к токенам:

```text
read token: Zone DNS Read + Zone Settings Read
write token: отдельный, короткоживущий, только DNS edit
worker deploy token: отдельный и не используется приложением
```

Worker:

- health — только дополнительный signal;
- `/api/config` — metadata-only, schema/version/signature;
- credentials — только authenticated/local provisioning;
- telemetry — opt-in, rate-limited, без IP/UUID/password;
- KV/D1 bindings проверяются contract tests;
- endpoint A/B/C имеют разные failure domains;
- Worker не выбирает канал и не запускает `ReloadVPN` сам.

Публичный Worker health обязан различать:

```text
edge → origin TCP/HTTP
edge → origin response
client → protocol handshake
client → protected DNS/HTTPS
```

Только последний уровень является VPN acceptance.

### 6.7. Что Cloudflare не делает

Обычный Free Website proxy не является:

- HY2 UDP proxy;
- raw TCP proxy для любого протокола;
- гарантией обхода DPI;
- защитой от блокировки домена;
- защитой от блокировки Cloudflare traffic;
- способом скрыть уже раскрытый origin IP.

Raw TCP/UDP через Spectrum — отдельный платный продукт и не входит в текущую
бесплатную схему.

---

## 7. Серверные профили

### Profile A — VLESS+WS+TLS через Cloudflare

```text
client → Cloudflare edge → nginx:443 → localhost VLESS+WS → internet
```

Требования:

- primary zone active;
- proxied A record;
- exact certificate/SNI;
- nginx owns 443;
- sing-box owns localhost backend;
- exact WS path;
- VLESS UUID provisioned securely;
- no direct origin fallback;
- client protected DNS/HTTPS passes.

### Profile B — VLESS+REALITY/XHTTP

```text
client → VPS-2:443 → REALITY/XHTTP → internet
```

Требования:

- VPS-2 different provider/ASN;
- fresh keys/UUID/short-id;
- target reachable from test network;
- pinned Xray/sing-box revision;
- active probe behaviour checked;
- protected DNS/HTTPS passes;
- no claim of invisibility.

### Profile C — HY2/TUIC

```text
client → independent UDP endpoint → internet
```

Требования:

- UDP handshake observed;
- firewall/provider UDP verified;
- certificate/SNI/password match;
- protected DNS/HTTPS passes;
- known operator-specific UDP result recorded.

### Profile D — emergency backend

Psiphon/Tor PT/Snowflake/WebTunnel/obfs4/AmneziaWG рассматриваются только
как отдельные adapters. Они не попадают в основной selector до всех gates:

```text
license
security
reproducible build
lifecycle
protected traffic
privacy model
performance
operator matrix
```

---

## 8. Канал D: аварийные транспорты

Канал D **включён в план**, но не включён в первый production MVP. Это не
означает, что перечисленные решения хуже. Они решают другие задачи и имеют
другую цену интеграции:

| Решение | Сильная сторона | Причина отдельного этапа |
|---|---|---|
| Psiphon | зрелые discovery, tactics, obfuscated transports и memory | отдельный runtime, серверная экосистема и модель обновлений |
| Tor Snowflake | динамические volunteer-relays, полезные при блокировке VPS | Tor не равен обычному VPN, выше latency и сложнее TUN-интеграция |
| WebTunnel | Tor-трафик выглядит как HTTPS через веб-сервер | нужны совместимые bridge/relay и отдельная Tor-интеграция |
| obfs4 | хорошая защита от простого распознавания bridge-трафика | нужны distribution bridges и самостоятельная эксплуатация |
| AmneziaWG | обфускация WireGuard-трафика | нужна точная совместимая пара client/server; нет универсальной гарантии |
| Другие PT | могут помочь против конкретного DPI/оператора | нужно подтвердить лицензию, безопасность, lifecycle и эффективность |

Это не «плохие запасные протоколы», а отдельный аварийный слой. До прохождения
самостоятельного evidence gate решение получает статус `planned` или
`experimental` и не попадает в protected selector:

```text
license/security review
reproducible build
server/client interoperability
Android/Windows lifecycle
protected DNS + two HTTPS targets
no direct fallback
privacy review
CPU/memory/latency benchmark
operator/network matrix
safe update and revocation
```

После прохождения gate оно становится обычным `validated channel` и участвует в
failover на тех же правилах, что A/B/C. Сначала реализуем reusable discovery,
tactics и memory по мотивам Psiphon, затем выбираем один PT по измеренной
пользе; остальные не добавляем ради количества.

## 9. Control-plane, discovery и ротация

### 8.1. Signed metadata

Metadata envelope:

```text
schema_version
metadata_version
issued_at
expires_at
key_id
signature
channels[]
capabilities
revocations
```

Внутри channel metadata нет:

```text
password
UUID
private key
SSH credential
Cloudflare token
```

Клиент проверяет подпись, срок, monotonic version и capabilities до применения.

### 8.2. Delivery hierarchy

```text
A: primary Worker
B: Pages/static metadata
C: second Worker/domain
D: embedded last-known-good
E: manual QR/file import
```

Каждый endpoint проверяется отдельно и не считается рабочим до signature +
profile validation.

### 8.3. Rotation policy

При отказе:

1. записать безопасную категорию;
2. прекратить retries на cooldown;
3. попробовать только другой validated failure domain;
4. не менять credentials автоматически без authenticated update;
5. при domain block использовать другой metadata endpoint;
6. если delivery заблокирован — показать manual import, не direct fallback.

---

## 9. Локальная разработка и тестовый стенд

### 9.1. Разрешено до рабочего сервера

- transport adapter interfaces;
- fake protected channels;
- local TLS/HTTP/WS integration server;
- deterministic DNS/TLS/probe tests;
- config signing/verification;
- circuit breaker and memory;
- Worker mock KV/D1 tests;
- Android/Windows lifecycle;
- synthetic throttling/freeze/reset tests;
- no-secret logging tests;
- SBOM/license checker.

### 9.2. Обязательный local harness

Harness должен уметь имитировать:

```text
success
DNS failure
TLS mismatch
auth failure
TCP timeout
UDP unavailable
server accepts then freezes after threshold
Cloudflare-style edge reset
origin unavailable
stale metadata
invalid signature
expired metadata
all channels unavailable
```

Он проверяет:

```text
no direct leak
correct classification
serialized failover
cleanup
UI unavailable/error state
rollback
```

### 9.3. Build/test gates

Windows:

```bash
go test ./...
go vet ./...
go test -race ./...
go build -tags "with_awg,with_wireguard,with_utls,with_gvisor" ./...
```

Frontend:

```bash
npm ci
npm run build
```

Android:

```bash
flutter analyze
flutter test
./gradlew :app:compileReleaseKotlin --no-daemon
flutter build apk --release --dart-define-from-file=config.local.json
```

Cloudflare:

```bash
node --check configs/cloudflare/worker.js
Worker contract tests with mocked KV/D1
```

Generated bindings не редактировать вручную.

---

## 10. Live acceptance matrix

### Networks

- домашний Wi‑Fi;
- MegaFon LTE;
- второй мобильный оператор;
- IPv4-only;
- IPv6-capable;
- нестабильная сеть/captive portal.

### Devices

- PTP N49 / Android 16;
- второй Android OEM;
- Windows 10;
- Windows 11;
- sleep/resume;
- Wi‑Fi ↔ LTE handover.

### Per-channel checks

```text
[ ] DNS/profile metadata valid
[ ] listener observed server-side
[ ] protocol handshake observed
[ ] protected DNS success
[ ] HTTPS target 1 success
[ ] HTTPS target 2 success
[ ] egress is expected
[ ] no direct fallback
[ ] 30-minute stability
[ ] edge/server reconnect
[ ] stop cleanup
[ ] repeated start/stop
```

### Failover checks

```text
[ ] A and B independently live-verified
[ ] A intentionally disabled
[ ] B selected
[ ] B protected DNS/HTTPS succeeds
[ ] direct never selected
[ ] A cooldown respected
[ ] A recovery requires protected success
[ ] all channels failure becomes blocked
```

### Evidence artifact

На каждый тест сохранять redacted JSON:

```text
channel_id
profile_version
core_version
network/operator label
OS/device class
timestamp
handshake result
DNS result
HTTP statuses
latency buckets
duration
failure category
```

Не сохранять IP пользователя, credentials, UUID, private keys или сырые логи с
секретами.

---

## 11. Порядок реализации greenfield

### Phase 0 — freeze requirements

- утвердить threat model;
- выбрать platforms;
- выбрать primary domain;
- получить server/registrar/Cloudflare access;
- rotate exposed credentials;
- зафиксировать license policy;
- создать metadata schema.

**Gate:** никаких неизвестных владельцев данных и credentials.

### Phase 1 — core safety

- config validator;
- signed metadata;
- channel descriptors;
- fail-closed selector;
- lifecycle manager;
- local probe;
- no-secret diagnostics;
- local harness.

**Gate:** все unit/integration tests pass; no direct protected fallback.

### Phase 2 — VPS-1 baseline and HY2

- fresh read-only audit;
- backup/restore rehearsal;
- HY2 profile validation;
- Android/Windows protected probe;
- classify current operator result.

**Gate:** HY2 `live-verified` хотя бы в одной сети или честно `blocked`.

### Phase 3 — Cloudflare CDN channel A

- registrar NS delegation;
- zone active;
- DNS records/proxy/settings;
- origin certificate;
- nginx + localhost VLESS+WS;
- exact client profile;
- protected DNS/HTTPS tests.

**Gate:** A live-verified на Wi‑Fi и минимум одном мобильном операторе.

### Phase 4 — independent channel B

- provision VPS-2 другой failure domain;
- REALITY/XHTTP reference/adapter;
- server/client interop;
- protected acceptance;
- failure simulation.

**Gate:** B live-verified независимо от A.

### Phase 5 — controller/failover

- concurrent candidate attempts;
- quality test;
- memory successful tuple;
- circuit breaker;
- signed rotation;
- serialized reload;
- all-failed block.

**Gate:** intentional A failure → B protected HTTPS без direct leak.

### Phase 6 — third transport and emergency path

- C HY2/TUIC on different failure domain;
- optional AmneziaWG;
- research adapters Psiphon/Tor PT/Samizdat only after review;
- no automatic production inclusion before acceptance.

**Gate:** each channel has independent evidence tuple.

### Phase 7 — product hardening

- Windows GUI acceptance;
- Android OEM matrix;
- certificate renewal;
- credential rotation;
- recovery from metadata endpoint loss;
- observability;
- support/runbooks;
- release signing and SBOM.

---

## 12. Критерии готовности

### Channel ready

```text
[ ] exact runtime config validated
[ ] server listener observed
[ ] protocol handshake observed
[ ] protected DNS observed
[ ] two protected HTTPS targets observed
[ ] no direct fallback
[ ] lifecycle cleanup verified
[ ] device/network tuple recorded
```

### Production resilience ready

```text
[ ] at least two live-verified channels
[ ] different failure domains
[ ] tested in Wi‑Fi and mobile networks
[ ] intentional channel failure switches correctly
[ ] signed metadata rotation works
[ ] at least two delivery paths work
[ ] last-known-good rollback works
[ ] all-failed state blocks traffic
[ ] no credentials leaked
[ ] metrics are honest/unavailable when source absent
```

### Security ready

```text
[ ] exposed credentials rotated
[ ] least-privilege Cloudflare tokens
[ ] SSH hardened
[ ] origin IP exposure documented
[ ] cert renewal tested
[ ] dependency licenses/SBOM recorded
[ ] Worker telemetry protected
[ ] public config metadata signed and credential-free
```

---

## 13. Финальный verdict

Лучшее практически достижимое решение — не «самый скрытый протокол», а
система diversity + evidence + delivery resilience:

```text
разные VPS/ASN
+ CDN channel
+ domainless channel
+ optional UDP channel
+ signed discovery
+ multiple delivery paths
+ concurrent attempts
+ success memory
+ protected probes
+ serialized failover
+ fail-closed
```

Даже такая система не гарантирует обход полного shutdown, блокировки всех
Cloudflare IP/доменов, активного распознавания всех transports, блокировки
приложения или одновременной блокировки tunnel и control-plane.

Корректное публичное обещание:

> «Система повышает устойчивость за счёт нескольких независимо проверенных
> защищённых каналов, выбирает доступный канал по фактам и не допускает
> незаметного прямого трафика. Поддержка подтверждается для конкретных сетей,
> устройств, версий core и профилей».

---

## 14. Что делает агент, а что требует разрешения владельца

### Агент может без отдельного инфраструктурного разрешения

- читать репозиторий и публичную документацию;
- выполнять локальные тесты и сборки;
- делать read-only публичные DNS/HTTPS проверки;
- готовить конфиги-шаблоны без credentials;
- писать тесты, adapters и документацию;
- готовить Cloudflare API dry-run и отчёт.

### Нужно отдельное явное разрешение

- изменение Cloudflare DNS/NS/settings;
- создание/удаление Worker, KV, D1, Pages resources;
- deploy/rollback Worker;
- SSH на VPS с изменением файлов;
- `systemctl restart/reload`;
- firewall/security-group changes;
- выпуск/отзыв сертификатов;
- покупка VPS/доменов/услуг;
- публикация APK/EXE или рассылка credentials.

До получения разрешения любые операции Cloudflare выполняются только в
read-only/dry-run режиме. Возможность технически использовать локальный OAuth
не является разрешением на write.
