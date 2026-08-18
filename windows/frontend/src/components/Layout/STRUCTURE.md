# STRUCTURE.md — components/Layout

> Каркас окна: верхняя панель, сайдбар, терминал-бар, декоративная матрица.

## Файлы
| Файл | Роль | Props/Emits |
|------|------|-------------|
| `TopBar.vue` | Шапка: индикатор соединения + кнопка ВКЛ/ВЫКЛ | props `{ connected, busy }`; emit `toggle` |
| `Sidebar.vue` | Навигация: меню страниц + статус-бейдж | props `{ active, connected, state, uptime }`; emit `nav(page)` |
| `TerminalBar.vue` | Нижний терминал: последние строки лога (marquee) | props `{ logs, state, connected }` |
| `ui/MatrixRain.vue` | Canvas-анимация «матрица» (кириллица+hex) | — (декоративно) |

## Поведение
- **TopBar**: кнопка показывает `busy && !connected` как «ПОДКЛЮЧЕНИЕ…»; клик →
  `emit('toggle')` → `App.vue` стартует/стопит VPN.
- **Sidebar**: статический массив `menu` (СЕРВЕРЫ/МАРШРУТИЗАЦИЯ/ТРАФИК/НАСТРОЙКИ/
  ДИАГНОСТИКА/СОБЫТИЯ/ЛОГИ/О СИСТЕМЕ); клик → `emit('nav', id)`.
  Статус-бейдж computed: ЗАЩИЩЁН/ПОДКЛЮЧЕНИЕ/ОТКЛЮЧЕНИЕ/ОШИБКА/ОТКЛЮЧЕН.
- **TerminalBar**: `watch(logs,{deep})` рендерит последние 8 строк, автоскролл вниз,
  кнопка паузы; класс строки по уровню (err/warn).
- **MatrixRain**: canvas + `requestAnimationFrame`, символы «アイウ…», дропы.

## Ассеты
- Мемы `pepe_fck_dpi_vpn_tears.png`, `pepe_shhh_silence.png` — как декор в Sidebar.

## Связи
`App.vue` рендерит TopBar/`Sidebar`/`TerminalBar` и передаёт им состояние (`connected`,
`busy`, `state`, `uptime`, `logs`, `active`); слушает их эмиты `toggle`/`nav`.
`TerminalBar` получает `logs` из `App.vue` (событие `engine:log`).