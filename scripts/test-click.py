"""
Launch the built exe, wait for the window, then use Windows UI Automation
to find and click the 'ВКЛ/ВЫКЛ' button, then capture the error text.
"""
import subprocess, time, sys

exe = r"D:\ОБХОДЫ\unkillable-vpn\build\bin\unkillable-vpn.exe"
print("Launching exe, capturing stderr...")
p = subprocess.Popen([exe], stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
print(f"PID {p.pid}, waiting 4s for window...")
time.sleep(4)

# Try UI Automation
try:
    import uiautomation as ua
except ImportError:
    print("uiautomation not installed; skipping auto-click")
    p.terminate()
    sys.exit(0)

root = ua.GetRootControl()
# Find the Wails window
win = None
for ctrl in root.GetChildren():
    name = ctrl.Name
    if "Unkillable" in name or "unkillable" in name:
        win = ctrl
        break
if not win:
    print("Window not found. Children names:")
    for c in root.GetChildren()[:10]:
        print(" -", repr(c.Name))
    p.terminate(); sys.exit(0)

print(f"Found window: {win.Name}")
win.SetTopmost(True)
time.sleep(0.5)

# Find the button by its label "ВКЛ" or "ВЫКЛ"
btn = None
for depth in (1,2,3,4):
    for b in win.GetChildren():
        try:
            for child in b.GetChildren():
                nm = child.Name
                if nm in ("ВКЛ","ВЫКЛ","ON","OFF"):
                    btn = child; break
        except: pass
    if btn: break

if btn:
    print(f"Found button: {btn.Name}, clicking...")
    btn.Click()
    time.sleep(3)
    print("Waiting for result...")
else:
    print("Button ВКЛ/ВЫКЛ not found via UIA")

# Read whatever stderr produced so far
p.terminate()
out, err = p.communicate(timeout=5)
print("\n=== STDERR ===")
print(err[:3000])
print("\n=== STDOUT ===")
print(out[:1500])
