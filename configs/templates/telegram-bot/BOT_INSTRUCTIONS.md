# Telegram бот — инструкция

## 1. Создать бота
1. Открыть @BotFather в Telegram
2. `/newbot` → выбрать имя → получить **токен**
3. Узнать свой chat_id: написать @userinfobot

## 2. Заполнить шаблон
В `bot-template.go` заменить:
```go
botToken  = "ТВОЙ_ТОКЕН"
chatID    = "ТВОЙ_CHAT_ID"
adminID   = int64(ТВОЙ_CHAT_ID)
fileURL   = "http://ТВОЙ_VPS_IP:8090"
```

## 3. Запуск
```bash
go run bot-template.go
```
Или скомпилировать:
```bash
go build -o vpnbot bot-template.go
./vpnbot
```

## 4. Функции
| Команда | Для кого | Что делает |
|---------|----------|-----------|
| /start | Все | Приветствие + кнопки |
| /pc | Все | Скачать ПК версию |
| /apk | Все | Скачать Android APK |
| /ios | Все | Скачать iOS конфиг |
| /status | Админ | Статус VPN |
| /logs | Админ | Последние логи |
| /reconnect | Админ | Переподключить |

## 5. Интеграция с ПК приложением
В Go backend (app.go):
```go
telegramLogger = NewTelegramLogger(token, chatID, engine, adaptive)
telegramLogger.Start(ctx)
```
Бот будет:
- Отправлять отчёты при ошибках (не чаще часа)
- Принимать команды из Telegram
- Показывать inline-кнопки

## 6. Раздача файлов
VPS nginx на порту 8090 раздаёт файлы:
```
http://ТВОЙ_IP:8090/snowden-portable.zip
http://ТВОЙ_IP:8090/snowden-android.apk
http://ТВОЙ_IP:8090/snowden-ios-config.json
```
Бот скачивает с VPS и отправляет в чат.

## 7. Запуск как сервис (systemd)
```bash
cat > /etc/systemd/system/vpnbot.service << EOF
[Unit]
Description=VPN Telegram Bot
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/vpnbot
Restart=on-failure

[Install]
WantedBy=multi-user.target
EOF

systemctl enable vpnbot
systemctl start vpnbot
```
