#!/bin/bash
# ═══════════════════════════════════════════════════════════
# VPS SETUP SCRIPT — snowden-template
# Запуск: bash setup.sh
# ═══════════════════════════════════════════════════════════
set -e

# --- ЗАПОЛНИ ---
VPS_IP="YOUR_VPS_IP"
DOMAIN="YOUR_DOMAIN.nip.io"
EMAIL="admin@YOUR_DOMAIN"
# ---------------

echo "╔══════════════════════════════════════╗"
echo "║  snowden VPS setup                    ║"
echo "╚══════════════════════════════════════╝"

# 1. BBR
echo "=== BBR ==="
echo 'net.core.default_qdisc=fq' >> /etc/sysctl.conf
echo 'net.ipv4.tcp_congestion_control=bbr' >> /etc/sysctl.conf
sysctl -p
echo "BBR: $(sysctl net.ipv4.tcp_congestion_control)"

# 2. Firewall
echo "=== Firewall ==="
ufw --force enable
ufw allow 22/tcp
ufw allow 443/tcp
ufw allow 80/tcp
ufw allow 20843/tcp   # HTTPUpgrade
ufw allow 30843/tcp   # gRPC
ufw allow 8443/udp    # Hysteria2
ufw allow 8090/tcp    # nginx file server

# 3. Сертификат
echo "=== Let's Encrypt ==="
apt-get update -qq
apt-get install -y -qq certbot
certbot certonly --standalone -d "$DOMAIN" --non-interactive --agree-tos -m "$EMAIL"
echo "✅ Сертификат получен"

# 4. sing-box
echo "=== sing-box ==="
curl -Lo /tmp/sb.tar.gz "https://github.com/SagerNet/sing-box/releases/download/v1.13.14/sing-box-1.13.14-linux-amd64.tar.gz"
cd /tmp && tar xzf sb.tar.gz
cp sing-box-*/sing-box /usr/local/bin/sing-box
chmod +x /usr/local/bin/sing-box
sing-box version

# 5. UUID
UUID=$(cat /proc/sys/kernel/random/uuid)
HY2_PASS=$(openssl rand -base64 16)
echo ""
echo "╔══════════════════════════════════════╗"
echo "║  СОХРАНИ ЭТИ ДАННЫЕ!                  ║"
echo "╠══════════════════════════════════════╣"
echo "║  UUID:     $UUID    ║"
echo "║  HY2 PASS: $HY2_PASS        ║"
echo "║  DOMAIN:   $DOMAIN     ║"
echo "╚══════════════════════════════════════╝"

# 6. Конфиг sing-box
mkdir -p /etc/sing-box
cat > /etc/sing-box/config.json << EOFCFG
{
  "log": {"level": "info", "timestamp": true},
  "inbounds": [
    {
      "type": "vless", "tag": "vless-in", "listen": "::", "listen_port": 443,
      "users": [{"name": "user", "uuid": "$UUID"}],
      "tls": {
        "enabled": true, "server_name": "$DOMAIN",
        "certificate_path": "/etc/letsencrypt/live/$DOMAIN/fullchain.pem",
        "key_path": "/etc/letsencrypt/live/$DOMAIN/privkey.pem"
      }
    },
    {
      "type": "vless", "tag": "vless-grpc", "listen": "::", "listen_port": 30843,
      "users": [{"name": "user", "uuid": "$UUID"}],
      "tls": {
        "enabled": true, "server_name": "$DOMAIN",
        "certificate_path": "/etc/letsencrypt/live/$DOMAIN/fullchain.pem",
        "key_path": "/etc/letsencrypt/live/$DOMAIN/privkey.pem"
      },
      "transport": {"type": "grpc", "service_name": "snowden"}
    },
    {
      "type": "vless", "tag": "vless-httpupgrade", "listen": "::", "listen_port": 20843,
      "users": [{"name": "user", "uuid": "$UUID"}],
      "tls": {
        "enabled": true, "server_name": "$DOMAIN",
        "certificate_path": "/etc/letsencrypt/live/$DOMAIN/fullchain.pem",
        "key_path": "/etc/letsencrypt/live/$DOMAIN/privkey.pem"
      },
      "transport": {"type": "httpupgrade", "path": "/snowden"}
    },
    {
      "type": "hysteria2", "tag": "hy2-in", "listen": "::", "listen_port": 8443,
      "users": [{"name": "user", "password": "$HY2_PASS"}],
      "tls": {
        "enabled": true, "server_name": "$DOMAIN",
        "certificate_path": "/etc/letsencrypt/live/$DOMAIN/fullchain.pem",
        "key_path": "/etc/letsencrypt/live/$DOMAIN/privkey.pem"
      }
    }
  ],
  "outbounds": [{"type": "direct", "tag": "direct"}]
}
EOFCFG

# 7. systemd
cat > /etc/systemd/system/sing-box.service << EOFSYS
[Unit]
Description=sing-box
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/sing-box run -c /etc/sing-box/config.json
Restart=on-failure
RestartSec=3
LimitNOFILE=infinity

[Install]
WantedBy=multi-user.target
EOFSYS

systemctl daemon-reload
systemctl enable sing-box
systemctl start sing-box
sleep 2
echo "sing-box: $(systemctl is-active sing-box)"

# 8. nginx для файлов
apt-get install -y -qq nginx
mkdir -p /var/www/releases
cat > /etc/nginx/sites-available/releases << EOFNGINX
server {
    listen 8090;
    server_name _;
    root /var/www/releases;
    location / {
        add_header Access-Control-Allow-Origin *;
        types {
            application/zip zip;
            application/vnd.android.package-archive apk;
            application/json json;
        }
        default_type application/octet-stream;
    }
}
EOFNGINX
ln -sf /etc/nginx/sites-available/releases /etc/nginx/sites-enabled/releases
rm -f /etc/nginx/sites-enabled/default
nginx -t && systemctl restart nginx

# 9. Проверка
echo ""
echo "=== Проверка ==="
echo "VLESS 443:    $(ss -tlnp | grep ':443 ' | head -1 | awk '{print $1}')"
echo "gRPC 30843:   $(ss -tlnp | grep ':30843 ' | head -1 | awk '{print $1}')"
echo "HTTPUp 20843: $(ss -tlnp | grep ':20843 ' | head -1 | awk '{print $1}')"
echo "Hy2 8443:     $(ss -ulnp | grep ':8443 ' | head -1 | awk '{print $1}')"
echo "nginx 8090:   $(ss -tlnp | grep ':8090 ' | head -1 | awk '{print $1}')"

echo ""
echo "╔══════════════════════════════════════╗"
echo "║  ✅ VPS ГОТОВ!                        ║"
echo "║  UUID: $UUID          ║"
echo "║  HY2:  $HY2_PASS          ║"
echo "║  IP:   $VPS_IP            ║"
echo "╚══════════════════════════════════════╝"
echo ""
echo "Далее: заполни конфиги в pc/, android/, ios/ этими данными"
