# ПК (Windows) — инструкция

## Вариант 1: sing-box CLI (простой)

### Установка
1. Скачать sing-box: https://github.com/SagerNet/sing-box/releases
2. Распаковать в папку
3. Скопировать `config-template.json` → `config.json`
4. Заменить `YOUR_VPS_IP`, `YOUR_UUID`, `YOUR_DOMAIN`
5. Запустить:
```bash
sing-box run -c config.json
```
6. Настроить прокси в Windows: 127.0.0.1:20808

## Вариант 2: Wails приложение (полноценный UI)

### Требования
- Go 1.22+
- Node.js 22+
- Wails v2: `go install github.com/wailsapp/wails/v2/cmd/wails@latest`

### Создать проект
```bash
wails init -n myvpn -t vue
cd myvpn
```

### Сборка
```bash
# Установить sing-box как зависимость
go get github.com/sagernet/sing-box@latest

# Build
wails build -tags "with_utls,with_wireguard,with_gvisor"
```

### Конфиг
Положить `config.json` рядом с exe. Приложение читает его при старте.

## Структура приложения (Go backend)
```
main.go          — Wails entrypoint
app.go           — биндинги (StartVPN, StopVPN, Status)
backend/
  core/
    engine.go    — sing-box lifecycle
    manager.go   — менеджер VPN
  config/
    builder.go   — конвертация CIDR
frontend/
  src/App.vue    — Vue3 UI
assets/configs/
  config.json    — конфиг sing-box
```
