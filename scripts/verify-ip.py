"""
Verify what 192.109.206.234 actually is on the server side.
"""
import paramiko

HOST="192.109.206.234"; PORT=22; USER="root"; PASS="ibi32E5vMy56U1cGCX"

c=paramiko.SSHClient(); c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
c.connect(HOST,port=PORT,username=USER,password=PASS,timeout=20,allow_agent=False,look_for_keys=False)

def run(cmd):
    _,o,e=c.exec_command(cmd,timeout=30,get_pty=True)
    print(f"$ {cmd}")
    print((o.read().decode('utf-8','replace')+e.read().decode('utf-8','replace')).strip())
    print("-"*40)

print("=== Реальные IP-адреса сервера ===")
run("ip addr show | grep 'inet ' | grep -v 127.0.0.1")
run("curl -sS --max-time 5 ifconfig.me")
run("curl -sS --max-time 5 https://www.cloudflare.com/cdn-cgi/trace | grep -E '^ip='")

print("=== Слушает ли sing-box на правильном IP? ===")
run("ss -tlnp | grep sing-box")
run("ss -tlnp | grep ':443'")

print("=== Может ли сервер достучаться до себя на 443? ===")
run("curl -sS --max-time 5 -k https://127.0.0.1:443/ -o /dev/null -w 'localhost 443: %{http_code}\\n' || echo 'localhost 443 FAIL'")
run("curl -sS --max-time 5 -k https://192.109.206.234:443/ -o /dev/null -w 'public 443: %{http_code}\\n' || echo 'public 443 FAIL'")

print("=== hostname / whois IP ===")
run("hostname")
run("hostname -I")

c.close()
