"""Capture server logs WHILE the probe is running."""
import paramiko, threading, time, subprocess, sys, os

HOST="192.109.206.234"; PORT=22; USER="root"; PASS="ibi32E5vMy56U1cGCX"
logs = []

def tail_logs():
    c=paramiko.SSHClient(); c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    c.connect(HOST,port=PORT,username=USER,password=PASS,timeout=20,allow_agent=False,look_for_keys=False)
    _,o,e=c.exec_command("journalctl -u sing-box -f -n 0 --no-pager -o cat",get_pty=True)
    start=time.time()
    while time.time()-start < 25:
        try:
            line=o.readline()
            if line: logs.append(line.decode('utf-8','replace').rstrip())
        except: break
    c.close()

t=threading.Thread(target=tail_logs,daemon=True); t.start()
print("log tailer started, launching probe...")
time.sleep(2)

env=dict(os.environ); env["PATH"]="/c/Users/Пользо/go-sdk/go/bin:/c/Users/Пользо/go/bin:"+env.get("PATH","")
env["GOROOT"]="/c/Users/Пользо/go-sdk/go"
r=subprocess.run(["bash","-c","cd /d/ОБХОДЫ/unkillable-vpn && go run -tags 'with_awg,with_wireguard,with_utls' ./backend/enginetest/ 2>&1 | tail -8"],
                 capture_output=True,text=True,timeout=40,env=env)
print("=== PROBE OUTPUT ==="); print(r.stdout)

time.sleep(2)
print("\n=== SERVER LOGS (during probe) ===")
for l in logs[-30:]: print(l)
