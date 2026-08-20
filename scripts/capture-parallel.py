"""Capture server logs WHILE the probe is running."""
import paramiko, threading, time, subprocess, sys, os

from env import load_project_env, require_env

load_project_env()

HOST=require_env("SNOWDEN_VPS_IP")
PORT=int(os.environ.get("SNOWDEN_VPS_SSH_PORT", "22"))
USER=os.environ.get("SNOWDEN_VPS_SSH_USER", "root")
PASS=require_env("SNOWDEN_VPS_SSH_PASSWORD")
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

env=dict(os.environ); env["PATH"] = os.environ.get("SNOWDEN_GO_PATH", env.get("PATH", ""))
if os.environ.get("GOROOT"):
    env["GOROOT"] = os.environ["GOROOT"]
r=subprocess.run(["bash", "-c", "cd \"$SNOWDEN_PROJECT_DIR\" && go run -tags 'with_awg,with_wireguard,with_utls' ./backend/enginetest/ 2>&1 | tail -8"],
                 capture_output=True,text=True,timeout=40,env=env)
print("=== PROBE OUTPUT ==="); print(r.stdout)

time.sleep(2)
print("\n=== SERVER LOGS (during probe) ===")
for l in logs[-30:]: print(l)
