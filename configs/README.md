# configs/ — единый источник конфигурации (git-безопасно)

В этой папке живут **все** настройки проекта. Она устроена так, что:

- **В репозиторий можно пушить безопасно** — реальные ключи закрыты `.gitignore`
  (см. `configs/.gitignore` и корневой `.gitignore`).
- **Своим близким ты отправляешь папку целиком** — она содержит твои реальные
  конфиги и служит «ключом доступа» к твоим серверам.
- **Чужие люди** могут взять только шаблоны (`.example`) + `templates/` и
  собрать свою версию под свои серверы.

## Модель работы

```
configs/
├── env/.env                  # РЕАЛЬНЫЙ (не пушится) — токены Telegram
├── env/.env.example          # безопасный шаблон
├── singbox/*.json            # РЕАЛЬНЫЕ (не пушатся) — серверы/ключи
├── singbox/*.example         # безопасные шаблоны
├── cloudflare/wrangler.toml  # реальный (не пушится) — ID/токены CF
├── cloudflare/*.example      # безопасный шаблон
├── landing/version.json      # реальный (не пушится) — IP раздачи
├── landing/*.example         # безопасный шаблон
└── templates/                # безопасные шаблоны для распространения
```

Правило простое: **`.example` — публикуется, одноимённый без `.example` — секрет.**

## Как подготовить свою версию (для чужих)

1. Скопируй этот репозиторий.
2. `cp configs/env/.env.example configs/env/.env` → впиши свои токены.
3. `cp configs/singbox/server-params.json.example configs/singbox/server-params.json`
   → впиши свои серверы. Аналогично для `warp-keys.json` и `template-vps-reality.json`.
4. `cp configs/landing/version.json.example configs/landing/version.json` → свои URL.
5. Примени в проект: `bash configs/sync-to-windows.sh`.

## Как собрать Android из этих конфигов

Подготовка сервера — по шаблонам `configs/templates/vps/` (`setup.sh` + инструкция).
Конфиг для приложений — `configs/templates/android/sing-box-config.json` (или
`ios/sing-box-config.json`). Дальше:
```bash
cd android
flutter pub get
flutter build apk --release     # или build_android.bat
```
APK из коробки собирается, если указан рабочий sing-box-конфиг в `lib/` или через
`SNOWDEN_FILE_URL` (бот тянет файлы с твоего VPS).

## Применение изменений

```bash
bash configs/sync-to-windows.sh   # копирует singbox+env в windows/assets/configs
```

## Что к публикации точно нельзя

- `env/.env`, `singbox/*(без .example)`, `cloudflare/wrangler.toml`,
  `landing/version.json` и `landing/snowden-*.json` — реальные ключи/IP.
- Если сомневаешься — `git status` покажет только *.example и templates/
  (остальное в `.gitignore`).