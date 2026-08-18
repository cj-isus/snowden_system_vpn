# STRUCTURE.md — configs/templates/

> Безопасные шаблоны для распространения (без секретов). По каждой платформе —
> инструкция + готовый конфиг для сборки своей версии.

## Подпапки
| Папка | Назначение | Ключевые файлы |
|-------|-----------|----------------|
| `pc/` | Десктоп Windows | `PC_INSTRUCTIONS.md`, `config-template.json` |
| `android/` | Android (Flutter/sing-box) | `ANDROID_INSTRUCTIONS.md`, `sing-box-config.json` |
| `ios/` | iOS | `IOS_INSTRUCTIONS.md`, `sing-box-config.json` |
| `landing/` | Лендинг | `LANDING_INSTRUCTIONS.md`, `index.html`, `version.json` |
| `telegram-bot/` | Бот-шаблон | `BOT_INSTRUCTIONS.md`, `bot-template.go` |
| `vps/` | Серверная подготовка | `VPS_INSTRUCTIONS.md`, `setup.sh` |
| `amnezia/` | AmneziaWG | `warp-config-template.conf` |

## Назначение
Дать новому человеку (без доступа к секретам) инструкцию: поднять VPS по
`vps/setup.sh`, собрать конфиг под свою платформу, задеплоить бота/лендинг.

## Модель
- Публикуются в репо (безопасно) — это «чистая» версия.
- Реальные параметры — в соседних `singbox/`, `env/`, `cloudflare/`, `landing/`
  (не пушатся).