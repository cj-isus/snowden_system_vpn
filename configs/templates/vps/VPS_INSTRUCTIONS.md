# VPS — инструкция развёртывания

## 1. Купить VPS
- Hetzner / AdminVPS / Aeza / любой
- Ubuntu 24.04 LTS
- 1 ядро, 2GB RAM, 15GB диск
- Цена: ~200-400₽/мес

## 2. Настроить домен (бесплатно)
Используй nip.io — не нужна регистрация:
```
ТВОЙ_IP = 192.168.1.1
Домен = myvpn.192-168-1-1.nip.io
```
Проверка: `ping myvpn.192-168-1-1.nip.io` → должен резолвиться в IP

## 3. Запустить setup.sh
```bash
# Загрузить на сервер
scp setup.sh root@ТВОЙ_IP:/root/

# Подключиться
ssh root@ТВОЙ_IP

# Запустить (сначала отредактируй переменные в начале файла!)
nano setup.sh  # ← заменить YOUR_VPS_IP, YOUR_DOMAIN, EMAIL
bash setup.sh
```

## 4. Что скрипт делает
1. Включает BBR (congestion control)
2. Настраивает firewall (ufw)
3. Получает сертификат Let's Encrypt
4. Устанавливает sing-box
5. Генерирует UUID и Hysteria2 пароль
6. Создаёт systemd сервис
7. Запускает sing-box (4 inbound: VLESS 443, gRPC 30843, HTTPUpgrade 20843, Hysteria2 8443)
8. Настраивает nginx для раздачи файлов на порту 8090

## 5. Сохранить данные
Скрипт выведет:
```
UUID:     xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
HY2 PASS: xxxxxxxxxxxxxxxx
DOMAIN:   myvpn.ТВОЙ_IP.nip.io
```
Запиши — они нужны для клиентских конфигов.

## 6. Проверка
```bash
systemctl status sing-box    # active
ss -tlnp | grep sing-box     # 4 порта
curl localhost:8090          # nginx
```

## 7. Обновление сертификата
Certbot настроен на автообновление. Проверка:
```bash
certbot renew --dry-run
```
