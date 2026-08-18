"""
Full E2E: run our Go engine binary (which sets system proxy itself),
then probe via curl that uses the system proxy.
"""
import subprocess, time, os, urllib.request, json

# 1. Build the e2e binary
env = dict(os.environ)
env["PATH"] = r"C:\Users\Пользо\go-sdk\go\bin;C:\Users\Пользо\go\bin;" + env.get("PATH","")
env["GOROOT"] = r"C:\Users\Пользо\go-sdk\go"
print("building e2e binary...")
r = subprocess.run(["C:\\Users\\Пользо\\go-sdk\\go\\bin\\go.exe","build","-tags","with_awg,with_wireguard,with_utls",
                    "-o","e2e.exe","./backend/enginetest/"],
                   cwd=r"D:\ОБХОДЫ\unkillable-vpn", capture_output=True, text=True, env=env, timeout=120)
if r.returncode != 0:
    print("BUILD FAILED:", r.stderr); exit(1)
print("built e2e.exe")

# 2. Set system proxy BEFORE launching (mimics what StartVPN does)
import winreg
key = winreg.OpenKey(winreg.HKEY_CURRENT_USER, r"Software\Microsoft\Windows\CurrentVersion\Internet Settings", 0, winreg.KEY_SET_VALUE)
winreg.SetValueEx(key, "ProxyEnable", 0, winreg.REG_DWORD, 1)
winreg.SetValueEx(key, "ProxyServer", 0, winreg.REG_SZ, "127.0.0.1:20808")
winreg.CloseKey(key)
print("system proxy set to 127.0.0.1:20808")

# 3. Launch engine in background (it will listen on 20808)
print("launching engine...")
proc = subprocess.Popen([r"D:\ОБХОДЫ\unkillable-vpn\e2e.exe"],
                       cwd=r"D:\ОБХОДЫ\unkillable-vpn",
                       stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
# give it time to start; poll for output
time.sleep(6)
# drain any stdout/stderr produced so far without blocking
import threading
out_buf, err_buf = [], []
def drain(stream, buf):
    while True:
        line = stream.readline()
        if not line: break
        buf.append(line.rstrip())
t1 = threading.Thread(target=drain, args=(proc.stdout, out_buf), daemon=True); t1.start()
t2 = threading.Thread(target=drain, args=(proc.stderr, err_buf), daemon=True); t2.start()
print("engine stdout:", out_buf[-8:] if out_buf else "(empty)")
print("engine stderr:", err_buf[-8:] if err_buf else "(empty)")

# 4. Probe via curl (which reads system proxy via --proxy-env if set, or we pass explicitly)
print("\n=== curl via system proxy ===")
r = subprocess.run(["curl","-sS","--max-time","15","--proxy","http://127.0.0.1:20808",
                    "https://www.cloudflare.com/cdn-cgi/trace"],
                   capture_output=True, text=True, timeout=20)
print(r.stdout[:500])
if r.returncode != 0: print("CURL ERR:", r.stderr[:300])

# 5. Cleanup
proc.terminate()
try: proc.wait(timeout=3)
except: proc.kill()

# Reset proxy
key = winreg.OpenKey(winreg.HKEY_CURRENT_USER, r"Software\Microsoft\Windows\CurrentVersion\Internet Settings", 0, winreg.KEY_SET_VALUE)
winreg.SetValueEx(key, "ProxyEnable", 0, winreg.REG_DWORD, 0)
winreg.CloseKey(key)
print("\nproxy reset")
