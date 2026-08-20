# snowden.system — раздача на VPS (без домена)

Самый дешёвый вариант: лендинг и файлы отдаются прямо с сервера `$SNOWDEN_VPS_IP` на порту `8090`.
Работает из РФ **без VPN**, пока конкретный IP не попал в реестр (свежий сервер — нет).
Плюс: весь конфиг уже указывает на `http://$SNOWDEN_VPS_IP:8090`.

## Состав

```
vps-deploy/
├── setup.sh                 # установка caddy + systemd-сервис на порту 8090
├── public/                  # веб-корень (лендинг + файлы скачивания)
│   ├── index.html           # лендинг (кнопки → http://$SNOWDEN_VPS_IP:8090/...)
│   ├── version.json         # версия + URL скачивания
│   ├── snowden-portable.zip # свежий ПК-сборка (14.7 МБ, 10.08)
│   ├── snowden-ios-config.json
│   ├── snowden-android-singbox.json
│   ├── snowden-amnezia.conf
│   └── snowden-mieru.json
```

## Как запустить на сервере (нужен SSH/root к $SNOWDEN_VPS_IP)

```bash
# залить папку на сервер (например в /root/vps-deploy)
scp -r vps-deploy root@$SNOWDEN_VPS_IP:/root/
# на сервере
cd /root/vps-deploy && chmod +x setup.sh && sudo ./setup.sh
```

Скрипт: ставит caddy → копирует `public/` в `/opt/snowden-web` → поднимает
systemd-сервис `snowden-web` → проверяет `http://127.0.0.1:8090/version.json`.

После этого лендинг доступен: `http://$SNOWDEN_VPS_IP:8090`

## Telegram-раздача (единственный бот уже готов)

Бот в приложении (`/pc`, `/apk`, `/ios`) сам качает файл с `SNOWDEN_FILE_URL`
и пересылает его в Telegram (`sendDocument`). В `.env` уже прописано:

```
SNOWDEN_FILE_URL=http://$SNOWDEN_VPS_IP:8090
```

То есть как только файлы лежат на сервере и порт 8090 открыт — команды бота
работают без правок кода. Файлы доходят до пользователя через Telegram, а не с твоего IP.

## Чего НЕТ и нужно дособрать

- `snowden-android.apk` — в проекте не собрано (нет Android build). Пока его нет,
  кнопка/команда `/apk` и ссылка в version.json дадут 404. Нужно собрать APK
  (Flutter `snowden_android/`) и положить его в `public/`.
- Порт 8090 должен быть открыт в файрволе сервера:
  `sudo ufw allow 8090/tcp` (или в панели провайдера).