"""
Diagnose why the VPS Reality endpoint may not accept client connections.
Checks: server sing-box status, port 443, recent logs, and a direct TCP probe.
"""
import paramiko
import socket
import time

HOST = "192.109.206.234"
PORT = 22
USER = "root"
PASS = "ibi32E5vMy56U1cGCX"

def run(c, cmd, t=30):
    _, stdout, stderr = c.exec_command(cmd, timeout=t, get_pty=True)
    out = (stdout.read().decode("utf-8","replace") + stderr.read().decode("utf-8","replace")).strip()
    code = stdout.channel.recv_exit_status()
    print(f"$ {cmd}")
    if out: print(out)
    print(f"[exit {code}]\n" + "-"*40)
    return out

c = paramiko.SSHClient()
c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
c.connect(HOST, port=PORT, username=USER, password=PASS, timeout=20,
          allow_agent=False, look_for_keys=False)

print("=== 1. systemctl status (last 20 lines) ===")
run(c, "systemctl status sing-box --no-pager -l | tail -20")

print("=== 2. journalctl recent (last 30) ===")
run(c, "journalctl -u sing-box -n 30 --no-pager")

print("=== 3. config check ===")
run(c, "sing-box check -c /etc/sing-box/config.json && echo 'CONFIG OK'")

print("=== 4. listening ports ===")
run(c, "ss -tlnp | grep -E ':443|sing-box'")

print("=== 5. config contents (mask key) ===")
run(c, "sed 's/private_key\": \"[^\"]*/private_key\": \"<HIDDEN>/' /etc/sing-box/config.json")

print("=== 6. sing-box version ===")
run(c, "sing-box version")

c.close()

print("=== 7. TCP probe to 443 from local machine ===")
try:
    s = socket.create_connection(("192.109.206.234", 443), timeout=10)
    # send a TLS ClientHello-ish byte to see if we get a TLS ServerHello (Reality answers as the dest site)
    s.sendall(bytes.fromhex("16030100") + b"\x00"*100)
    data = s.recv(32)
    print(f"  got {len(data)} bytes: {data.hex()}")
    s.close()
    print("  TCP 443 REACHABLE + server responds")
except Exception as e:
    print(f"  PROBE FAILED: {e}")
