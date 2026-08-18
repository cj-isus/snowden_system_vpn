# STRUCTURE.md — windows/ (Package main: слой приложения + OS-обвязка)

> Точка входа Wails-приложения. `package main` в `windows/` (module `snowden-system`).
> Здесь живут: GUI-слой (App + Wails-методы), Telegram-бот, системный прокси,
> трей, автозапуск, обработка краша. Бизнес-логика — в `backend/core`.

## Файлы
| Файл | Роль | Ключевые символы |
|------|------|-------------------|
| `main.go` | Точка входа: `//go:embed frontend/dist`, Wails `Run`, тред-запуск | `main()`, `NewApp()`, `startup`, `OnShutdown`, `ClearStaleProxyOnStartup`, `installCrashHandler` |
| `app.go` | **Wails-фасад**: методы для Vue | `App` (struct), `StartVPN/StopVPN/ReloadVPN/Status/GetServers/GetTraffic/GetLatency/SelectServer/ToggleRouteRule/EnableRouteRule/ExportConfig/ImportConfig/ListConfigs/CheckForUpdate/GetRemoteHealth/OpenExternalApp/SetAutostart/LoadConfigFile`, `logEmitter` |
| `telegram_bot.go` | TelegramLogger: отчёты + админ-панель (inline-клавиатура) | `TelegramLogger`, `loop`, `commandLoop`, `pollUpdates`, `sendReport`, `makeClient`, `RegisterDevice` |
| `tray_windows.go` | Иконка в трее: показать/скрыть, автозапуск, выход | `trayManager`, `newTrayManager`, `Start/onReady/onExit`, `exeDir()` |
| `proxy_windows.go` | Системный HTTP-прокси через реестр (HKCU Internet Settings) | `setSystemProxy`, `clearSystemProxy`, `regSetDWORD/SZ`, `notifySettingsChanged` |
| `crash_windows.go` | Чистка прокси при Ctrl+C/taskkill + stale-прокси на старте | `installCrashHandler`, `ClearStaleProxyOnStartup`, `regGetDWORD/SZ` |
| `autostart_windows.go` | Автозапуск с Windows (HKCU Run) | `setAutostartRegistry`, `isAutostartEnabled` |
| `wails.json`, `go.mod`, `build/`, `assets/`, `frontend/`, `docs/` | сборка/конфиг/Wails-бандл | — |

## Поток вызова (главный сценарий «ВКЛ»)
```
Vue ──StartVPN("vps-reality", configJSON)──► app.StartVPN
  1) удаляет cache.db                       2) manager.StartVPN (→core)
  3) setSystemProxy("127.0.0.1:20808")     4) adaptive.Start(configID, config)
  5) если !Status().Connected → error
Vue ◄── Status()/GetTraffic()/GetLatency() ◄── manager
Vue ◄── EventsEmit("engine:log") ◄── logEmitter.OnLog (из core)
```

## Ключевые константы / точки
- Прокси-порт: `127.0.0.1:20808` (говорот в `app.go`, `telegram_bot.go`).
- Конфиг по умолчанию: `CONFIG_FILE="template-vps-reality.json"`, `CONFIG_ID="vps-reality"`.
- Версия локального клиента: `LOCAL_VERSION="1.3.5"` в `CheckForUpdate`.
- Секреты читаются из `.env` (`loadEnvFile`) и env: `SNOWDEN_TG_TOKEN`, `SNOWDEN_TG_CHAT_ID`, `SNOWDEN_TG_ADMIN_ID`, `SNOWDEN_FILE_URL`, `SNOWDEN_WORKER_URL`.

## Пресеты-ограничения
- `SelectServer` хардкодит страны `nl`/`fr` → теги `grpc-nl`/`grpc-fr` (см. `PLAN.md` A5).
- `injectSplitTunnel` — мёртвый код (см. `PLAN.md` A4).
- Трей/прокси/краш — только Windows (`//go:build windows`).