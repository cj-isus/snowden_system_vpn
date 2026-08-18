"""
Diagnose why VPS accepts Reality handshake but doesn't deliver traffic.
Check: server can reach internet, sing-box logs for connection attempts.
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

print("=== 1. может ли сервер сам выйти в интернет? ===")
run(c,"curl -sS --max-time 8 https://www.cloudflare.com/cdn-cgi/trace | head -5 || echo 'INTERNET FAIL'")
run(c,"curl -sS --max-time 8 http://www.gstatic.com/generate_204 -o /dev/null -w 'gstatic: %{http_code}\\n' || echo 'GSTATIC FAIL'")

print("=== 2. sing-box логи (последние 40 строк) ===")
run(c,"journalctl -u sing-box -n 40 --no-pager")

print("=== 3. текущий полный конфиг (без маскировки для диагностики) ===")
run(c,"cat /etc/sing-box/config.json")

print("=== 4. отключён ли firewall? (или режет ли исходящий) ===")
run(c,"ufw status verbose")

print("=== 5. есть ли IP forwarding? (не нужен для этого режима, но проверим) ===")
run(c,"sysctl net.ipv4.ip_forward")

c.close()
