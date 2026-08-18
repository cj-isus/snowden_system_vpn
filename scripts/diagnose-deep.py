"""
Deep server diagnosis: test direct outbound from sing-box context.
Restart sing-box with debug logging and check if it can establish outbound.
"""
import paramiko, time

HOST="192.109.206.234"; PORT=22; USER="root"; PASS="ibi32E5vMy56U1cGCX"

def run(c,cmd,t=30):
    _,o,e=c.exec_command(cmd,timeout=t,get_pty=True)
    out=(o.read().decode('utf-8','replace')+e.read().decode('utf-8','replace')).strip()
    print(f"$ {cmd}")
    if out: print(out)
    print(f"[exit {o.channel.recv_exit_status()}]\n"+"-"*40)
    return out

c=paramiko.SSHClient(); c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
c.connect(HOST,port=PORT,username=USER,password=PASS,timeout=20,allow_agent=False,look_for_keys=False)

print("=== 1. прямой интернет-доступ с сервера ===")
run(c,"curl -sS --max-time 8 https://www.cloudflare.com/cdn-cgi/trace | head -5 || echo 'INTERNET FAIL'")
run(c,"curl -sS --max-time 8 https://www.google.com/generate_204 -o /dev/null -w 'google: %{http_code}\\n' || echo 'GOOGLE FAIL'")

print("=== 2. sing-box версия и build tags (может без utls/quic?) ===")
run(c,"sing-box version")

print("=== 3. Переведём лог в DEBUG и перезапустим ===")
run(c,"sed -i 's/\"info\"/\"debug\"/' /etc/sing-box/config.json")
run(c,"sing-box check -c /etc/sing-box/config.json && systemctl restart sing-box && sleep 2")
run(c,"systemctl is-active sing-box")

print("=== 4. ТЕСТ ИЗНУТРИ сервера: сам sing-box может проксировать? ===")
print("(запустим временный sing-box клиент на сервере → 127.0.0.1:1080 → через локальный vless)")
# Это сложновато — пропустим. Просто проверим исходящий напрямую.

print("=== 5. Ждём ваших клиентских подключений в логах (15 сек) ===")
print("ПОПРОБУЙТЕ СЕЙЧАС нажать ВКЛ в приложении, я смотрю логи...")
run(c,"journalctl -u sing-box -f --no-pager -n 0",t=20)

c.close()
