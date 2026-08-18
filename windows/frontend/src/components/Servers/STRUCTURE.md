# STRUCTURE.md — components/Servers

> Управление серверами и маршрутизацией.

## Файлы
| Файл | Роль | Wails-вызовы |
|------|------|--------------|
| `ServersCard.vue` | Список серверов + выбор (`auto`/`nl`/`fr`) + пинг | `GetServers`, `SelectServer` |
| `RoutingCard.vue` | Маршрутизация: список правил, вкл/выкл правило | `GetRouteRules`, `ToggleRouteRule` |

## Поведение
### ServersCard
- `refresh()` поллит `GetServers()` каждые 10 c (показует `active`, `ping`).
- Пользователь выбирает сервер → `SelectServer(name)` → Go переписывает
  `route.final` и перезагружает sing-box.
- `selectedServer` по умолчанию `"auto"`; `switching` блокирует UI на время смены.
- Использует `inject("showToast")` для уведомлений.

### RoutingCard
- Поллит `GetRouteRules()`; мапит в локальные `Rule` c `ruleIndex` (порядок в `route.rules`).
- Переключение `ToggleRouteRule(ruleIndex, enabled)` → Go меняет action
  `direct`↔`outbound:auto` и хот-релоадит.
- `toggling` — состояние ожидания на конкретном правиле (строке).

## Связи
`App.vue` передаёт в `ServersCard` props `{ configId, connected }`; обе карточки
самостоятельно ходят в Go через `../../../wailsjs/go/main/App`.
Тост-уведомления — через `inject("showToast")` (defined в `App.vue`).