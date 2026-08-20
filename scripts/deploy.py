#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
snowden.system DEPLOY
=====================
PowerShell:  python .\deploy.py

  1. Автоинкремент версии
  2. Сборка ПК (wails build)
  3. Сборка Android (flutter build apk)
  4. Portable ZIP
  5. Загрузка на VPS с прогресс-баром
  6. Cloudflare Pages
  7. Установка APK на телефон (ADB)
"""
import os, sys, subprocess, json, shutil, re, time

# ═══════════════════════════════════════════════
PC_DIR       = r"D:\ОБХОДЫ\unkillable-vpn"
ANDROID_DIR  = os.environ.get("SNOWDEN_ANDROID_DIR", os.path.join(os.path.dirname(__file__), "..", "android"))
ANDROID_DIR2 = os.environ.get("SNOWDEN_ANDROID_FALLBACK_DIR", ANDROID_DIR)
PORTABLE_DIR = r"D:\ОБХОДЫ\Snowden_system\snowden-portable"
PAGES_DIR    = os.path.join(PC_DIR, "cloudflare", "pages")
VERSION_FILE = r"D:\ОБХОДЫ\Snowden_system\version.txt"

VPS_IP   = os.environ["SNOWDEN_VPS_IP"]
VPS_USER = os.environ.get("SNOWDEN_VPS_SSH_USER", "root")
VPS_PASS = os.environ["SNOWDEN_VPS_SSH_PASSWORD"]

GO_BIN    = r"C:\Users\Пользо\go-sdk\go\bin"
WAILS_BIN = r"C:\Users\Пользо\go\bin"
JAVA_HOME = r"C:\Program Files\Eclipse Adoptium\jdk-17.0.19.10-hotspot"
FLUTTER   = os.environ.get("FLUTTER_BIN", "flutter")
ADB       = os.environ.get("ADB_BIN", "adb")
TAGS      = "with_awg,with_wireguard,with_utls,with_gvisor"

# ═══════════════════════════════════════════════

def get_next_version():
    """Автоинкремент: читает version.txt, увеличивает patch."""
    try:
        with open(VERSION_FILE, "r") as f:
            v = f.read().strip()
        parts = v.split(".")
        parts[-1] = str(int(parts[-1]) + 1)
        new_v = ".".join(parts)
    except:
        new_v = "1.2.2"
    with open(VERSION_FILE, "w") as f:
        f.write(new_v)
    code = int("".join(new_v.split(".")))
    return new_v, code

def step(n, msg):
    print(f"\n{'='*50}")
    print(f"  [{n}/7] {msg}")
    print(f"{'='*50}")

def run(cmd, cwd=None, env=None, timeout=600):
    print(f"  > {cmd[:120]}")
    r = subprocess.run(cmd, shell=True, cwd=cwd, env=env, timeout=timeout, capture_output=True)
    if not r.returncode == 0 and r.stderr:
        print(f"  ! {r.stderr.decode('utf-8','replace')[-300:]}")
    return r.returncode == 0

def progress_bar(done, total, label=""):
    pct = done * 100 // total if total else 100
    bar = "=" * (pct // 3)
    sys.stdout.write(f"\r  [{bar:<33}] {pct:3d}% {label}")
    sys.stdout.flush()
    if pct >= 100:
        print()

def upload_progress(local, remote):
    """Загрузка на VPS с простым прогрессом."""
    import paramiko
    fsize = os.path.getsize(local)
    fname = os.path.basename(local)

    ssh = paramiko.SSHClient()
    ssh.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    ssh.connect(VPS_IP, username=VPS_USER, password=VPS_PASS, timeout=20)
    sftp = ssh.open_sftp()

    def callback(transferred, total):
        pct = transferred * 100 // total if total else 100
        bar = "=" * (pct // 3)
        sys.stdout.write(f"\r  [{bar:<33}] {pct:3d}% {fname}")
        sys.stdout.flush()

    try:
        sftp.put(local, remote, callback=callback)
        print(f"\n  + {fname} OK")
    except Exception as e:
        print(f"\n  ! ERROR: {e}")
        sftp.close()
        ssh.close()
        return False

    sftp.close()
    ssh.close()
    return True

def main():
    # Автоинкремент версии
    VERSION, VERSION_CODE = get_next_version()
    print(f"\n  snowden.system v{VERSION} DEPLOY")
    print(f"  {'='*46}")

    go_env = os.environ.copy()
    go_env["PATH"] = f"{GO_BIN};{WAILS_BIN};{go_env['PATH']}"
    go_env["GOPROXY"]    = "https://goproxy.cn,https://proxy.golang.org,direct"
    go_env["GOSUMDB"]    = "off"
    go_env["GOINSECURE"] = "*"
    go_env["GOFLAGS"]    = "-mod=mod"

    fl_env = os.environ.copy()
    fl_env["JAVA_HOME"] = JAVA_HOME
    fl_env["PATH"] = f"{JAVA_HOME}\\bin;{fl_env['PATH']}"
    fl_env["ANDROID_SDK_ROOT"] = r"C:\Users\Пользо\Android\Sdk"

    # ═══ 1. ПК ═══
    step(1, "PC build (wails)")
    run(f'wails build -skipbindings -s -tags "{TAGS}"', cwd=PC_DIR, env=go_env)
    shutil.copy2(
        os.path.join(PC_DIR, "assets", "configs", "template-vps-reality.json"),
        os.path.join(PC_DIR, "build", "bin", "assets", "configs", "template-vps-reality.json"))
    print("  OK")

    # ═══ 2. Android ═══
    step(2, "Android build (flutter)")
    android_ok = run(f'"{FLUTTER}" build apk --release --dart-define-from-file=config.local.json', cwd=ANDROID_DIR, env=fl_env)
    if not android_ok and os.path.exists(ANDROID_DIR2):
        print("  Primary dir locked, trying snowden_build...")
        # Sync ALL source files to snowden_build
        for src_name in ["lib/main.dart", "pubspec.yaml"]:
            src = os.path.join(ANDROID_DIR, src_name)
            dst = os.path.join(ANDROID_DIR2, src_name)
            if os.path.exists(src):
                shutil.copy2(src, dst)
        kotlin_src = os.path.join(ANDROID_DIR, "android", "app", "src", "main", "kotlin", "com", "snowden", "system", "snowden_android")
        kotlin_dst = os.path.join(ANDROID_DIR2, "android", "app", "src", "main", "kotlin", "com", "snowden", "system", "snowden_android")
        if os.path.exists(kotlin_src):
            os.makedirs(kotlin_dst, exist_ok=True)
            for f in os.listdir(kotlin_src):
                if f.endswith(".kt"):
                    shutil.copy2(os.path.join(kotlin_src, f), os.path.join(kotlin_dst, f))
        # Sync AndroidManifest
        manifest_src = os.path.join(ANDROID_DIR, "android", "app", "src", "main", "AndroidManifest.xml")
        manifest_dst = os.path.join(ANDROID_DIR2, "android", "app", "src", "main", "AndroidManifest.xml")
        if os.path.exists(manifest_src):
            shutil.copy2(manifest_src, manifest_dst)
        # Clean build dir before rebuild
        build_dir = os.path.join(ANDROID_DIR2, "build")
        if os.path.exists(build_dir):
            subprocess.run(f'rd /s /q "{build_dir}"', shell=True, capture_output=True)
        android_ok = run(f'"{FLUTTER}" build apk --release --dart-define-from-file=config.local.json', cwd=ANDROID_DIR2, env=fl_env)
        ANDROID_USED = ANDROID_DIR2
    else:
        ANDROID_USED = ANDROID_DIR

    if not android_ok:
        print("  ! Android build FAILED — continuing without APK")

    # Найти APK
    apk = os.path.join(ANDROID_USED, "build", "app", "outputs", "flutter-apk", "app-release.apk")
    apk_exists = os.path.exists(apk)
    print(f"  APK: {'OK' if apk_exists else 'MISSING'} ({apk})")

    # ═══ 3. Portable ZIP ═══
    step(3, "Portable ZIP")
    shutil.copy2(
        os.path.join(PC_DIR, "build", "bin", "snowden-system.exe"),
        PORTABLE_DIR)
    shutil.copy2(
        os.path.join(PC_DIR, "assets", "configs", "template-vps-reality.json"),
        os.path.join(PORTABLE_DIR, "assets", "configs"))
    for f in ["snowden-system.log", "ru-cidr.json", "cache.db"]:
        p = os.path.join(PORTABLE_DIR, f)
        if os.path.exists(p):
            try: os.remove(p)
            except: pass
    zip_path = r"D:\ОБХОДЫ\Snowden_system\snowden-portable.zip"
    if os.path.exists(zip_path): os.remove(zip_path)
    subprocess.run([
        "powershell.exe", "-Command",
        f"Compress-Archive -Path '{PORTABLE_DIR}\\*' -DestinationPath '{zip_path}' -Force"
    ], capture_output=True)
    print("  OK")

    # ═══ 4. Version ═══
    step(4, f"Version -> {VERSION}")
    vdata = {
        "version": VERSION,
        "versionCode": VERSION_CODE,
        "pc_url":      f"http://{VPS_IP}:8090/snowden-portable.zip",
        "android_url": f"http://{VPS_IP}:8090/snowden-android.apk",
        "ios_config_url":      f"http://{VPS_IP}:8090/snowden-ios-config.json",
        "singbox_config_url":  f"http://{VPS_IP}:8090/snowden-android-singbox.json",
        "amnezia_config_url":  f"http://{VPS_IP}:8090/snowden-amnezia.conf",
        "changelog": f"v{VERSION}"
    }
    vpath = os.path.join(PAGES_DIR, "version.json")
    with open(vpath, "w", encoding="utf-8") as f:
        json.dump(vdata, f, indent=2, ensure_ascii=False)
    ipath = os.path.join(PAGES_DIR, "index.html")
    with open(ipath, "r", encoding="utf-8") as f:
        html = f.read()
    html = re.sub(r'v\d+\.\d+\.\d+', f'v{VERSION}', html)
    with open(ipath, "w", encoding="utf-8") as f:
        f.write(html)
    # Обновить версию в app.go и main.dart
    app_go = os.path.join(PC_DIR, "app.go")
    with open(app_go, "r", encoding="utf-8") as f:
        src = f.read()
    src = re.sub(r'LOCAL_VERSION = "[\d.]+"', f'LOCAL_VERSION = "{VERSION}"', src)
    with open(app_go, "w", encoding="utf-8") as f:
        f.write(src)
    print("  OK")

    # ═══ 5. VPS upload с прогресс-баром ═══
    step(5, "VPS upload")
    files = [
        (zip_path, "/var/www/releases/snowden-portable.zip"),
    ]
    # APK только если существует
    if apk_exists:
        files.append((apk, "/var/www/releases/snowden-android.apk"))
    else:
        print("  ! APK missing — skip upload")
    for fn in ["snowden-ios-config.json", "snowden-android-singbox.json", "snowden-amnezia.conf"]:
        fp = os.path.join(PAGES_DIR, fn)
        if os.path.exists(fp):
            files.append((fp, f"/var/www/releases/{fn}"))

    for local, remote in files:
        upload_progress(local, remote)
    print("  OK")

    # ═══ 6. Cloudflare Pages ═══
    step(6, "Cloudflare Pages deploy")
    cache = os.path.join(PAGES_DIR, ".wrangler")
    if os.path.exists(cache):
        shutil.rmtree(cache)
    run("wrangler pages deploy . --project-name snowden-system", cwd=PAGES_DIR)
    print("  OK")

    # ═══ 7. ADB ═══
    step(7, "ADB install")
    adb_ok = False
    if os.path.exists(ADB):
        r = subprocess.run(f'"{ADB}" devices', shell=True, capture_output=True, text=True)
        for line in r.stdout.split('\n'):
            if 'device' in line and 'List of' not in line and 'unauthorized' not in line and 'offline' not in line:
                adb_ok = True
                break

    if adb_ok and apk_exists:
        print("  Phone detected!")
        subprocess.run(f'"{ADB}" shell am force-stop com.snowden.system.snowden_android', shell=True)

        # Установка с прогрессом
        print(f"  Installing APK ({os.path.getsize(apk)//1048576}MB)...")
        r = subprocess.run(f'"{ADB}" install -r -g "{apk}"', shell=True, capture_output=True, text=True, timeout=120)
        if 'Success' in r.stdout:
            print("  + APK installed")
        else:
            print(f"  ! {r.stdout[-100:]}")

        subprocess.run(f'"{ADB}" logcat -c', shell=True)
        subprocess.run(f'"{ADB}" shell am start -n com.snowden.system.snowden_android/.MainActivity', shell=True)
        print("  + App launched")
    elif adb_ok and not apk_exists:
        print("  Phone detected but APK missing — skip install")
    else:
        print("  Phone not connected (skip)")

    # ═══ DONE ═══
    print(f"\n  {'='*46}")
    print(f"  v{VERSION} PUBLISHED!")
    print(f"  Landing: snowden-system.pages.dev")
    print(f"  Files:   {VPS_IP}:8090")
    print(f"  {'='*46}\n")

if __name__ == "__main__":
    main()
