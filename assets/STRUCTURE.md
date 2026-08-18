# STRUCTURE.md — assets/

> Общие мультимедиа-ассеты проекта (логотипы).

## Файлы
| Файл | Роль |
|------|------|
| `logo/snowden_system_{16,32,64,128,256,512}.png` | Логотип в разных разрешениях |
| `logo/snowden_system_icon.ico` | Иконка (Windows) |

## Использование
- Логотип используется в UI/документации.
- Иконка `.ico` — приложение Windows (`appicon.ico` рядом с exe, трей `tray_windows.go`).

## Примечание
- Фронтенд ассеты (мемы, шрифты) — в `windows/frontend/src/assets/`.
- Конфиги картинок приложения Windows — `windows/assets/` (appicon, snowden-system.png).