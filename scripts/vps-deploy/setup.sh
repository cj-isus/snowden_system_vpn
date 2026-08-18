#!/usr/bin/env bash
# snowden.system — раздача лендинга и файлов на VPS (без домена, порт 8090)
#
# Использование (на сервере с root, из папки, где лежит setup.sh и public/):
#   chmod +x setup.sh
#   sudo ./setup.sh
#
# Что делает:
#   - ставит caddy (простой статический веб-сервер)
#   - копирует public/ в /opt/snowden-web
#   - поднимает systemd-сервис snowden-web на порту 8090
#   - проверяет, что http://<IP>:8090/index.html отвечает

set -euo pipefail

DIR="$(cd "$(dirname "$0")" && pwd)"
PUB="$DIR/public"
WEBROOT="/opt/snowden-web"
PORT="${PORT:-8090}"
HOST="${HOST:-0.0.0.0}"

# 1. Caddy (статик-сервер одной командой, с автоперезапуском через systemd)
if ! command -v caddy >/dev/null 2>&1; then
  echo "[1/4] Устанавливаю caddy..."
  # Официальный репозиторий Caddy
  apt-get update -y >/dev/null 2>&1 || true
  apt-get install -y debian-keyring debian-archive-keyring apt-transport-https curl >/dev/null 2>&1 || true
  curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
  curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | tee /etc/apt/sources.list.d/caddy-stable.list >/dev/null
  apt-get update -y >/dev/null 2>&1
  apt-get install -y caddy >/dev/null 2>&1
fi
echo "caddy: $(caddy version 2>/dev/null || echo 'установка не удалась')"

# 2. Кладём файлы в веб-корень
echo "[2/4] Копирую файлы в $WEBROOT ..."
mkdir -p "$WEBROOT"
cp -r "$PUB"/. "$WEBROOT"/
chmod -R a+r "$WEBROOT"
chmod a+rX "$(find "$WEBROOT" -type d)"

# 3. systemd-сервис
echo "[3/4] Создаю systemd-сервис snowden-web (порт $PORT) ..."
cat >/etc/systemd/system/snowden-web.service <<EOF
[Unit]
Description=snowden.system static file server
After=network.target

[Service]
ExecStart=/usr/bin/caddy file-server --root $WEBROOT --listen $HOST:$PORT
Restart=always
RestartSec=3

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable snowden-web >/dev/null 2>&1 || true
systemctl restart snowden-web
sleep 1

# 4. Проверка
IP="$(hostname -I 2>/dev/null | awk '{print $1}')"
echo "[4/4] Проверяю..."
systemctl --no-pager --lines=0 status snowden-web 2>/dev/null | head -3 || true
if curl -sf "http://127.0.0.1:${PORT}/version.json" >/dev/null; then
  echo "OK: лендинг отвечает на http://127.0.0.1:${PORT}"
  echo "Снаружи: http://${IP:-<IP>}:${PORT}"
else
  echo "ВНИМАНИЕ: curl не ответил. Проверь: systemctl status snowden-web; ss -tlnp | grep $PORT"
fi