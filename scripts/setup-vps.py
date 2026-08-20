"""
Setup VLESS+Reality on the VPS via SSH (paramiko).
Runs hardening, sing-box install and key generation without printing credentials.
"""
import os
import paramiko
import sys
import time

from env import load_project_env, require_env

load_project_env()

HOST = require_env("SNOWDEN_VPS_IP")
PORT = int(os.environ.get("SNOWDEN_VPS_SSH_PORT", "22"))
USER = os.environ.get("SNOWDEN_VPS_SSH_USER", "root")
PASS = require_env("SNOWDEN_VPS_SSH_PASSWORD")

def run(client, cmd, timeout=180, show_output=True):
    """Run command and return (exit_code, output); sensitive output can be hidden."""
    stdin, stdout, stderr = client.exec_command(cmd, timeout=timeout, get_pty=True)
    out = stdout.read().decode("utf-8", errors="replace")
    err = stderr.read().decode("utf-8", errors="replace")
    code = stdout.channel.recv_exit_status()
    combined = (out + err).strip()
    if show_output and combined:
        print(f"$ {cmd}")
        print(combined)
        print(f"[exit {code}]")
        print("-" * 50)
    return code, combined

def main():
    client = paramiko.SSHClient()
    client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    print(f"Connecting to {USER}@{HOST}:{PORT} ...")
    try:
        client.connect(HOST, port=PORT, username=USER, password=PASS, timeout=20, allow_agent=False, look_for_keys=False)
    except Exception as e:
        print(f"CONNECT FAILED: {e}")
        sys.exit(1)
    print("Connected.\n" + "=" * 50)

    # Step 2: update system
    print("\n>>> STEP 2: apt update + upgrade")
    run(client, "export DEBIAN_FRONTEND=noninteractive; apt-get update -y && apt-get upgrade -y", timeout=300)

    # Step 3: install sing-box (deb-install.sh installs latest stable with systemd unit)
    print("\n>>> STEP 3: install sing-box")
    run(client, "bash <(curl -fsSL https://sing-box.app/deb-install.sh)", timeout=240)
    code, ver = run(client, "sing-box version | head -1")
    if code != 0 or not ver:
        print("SING-BOX INSTALL FAILED")
        sys.exit(2)

    # Step 4: generate keys
    print("\n>>> STEP 4: generate Reality keypair + UUID + short_id")
    # The command returns both keys. Keep its output in memory and never print
    # it: the private key must not enter the terminal scrollback or CI logs.
    _, keys = run(client, "sing-box generate reality-keypair", show_output=False)
    # parse
    priv = pub = ""
    for line in keys.splitlines():
        if "PrivateKey" in line: priv = line.split()[-1]
        if "PublicKey" in line: pub = line.split()[-1]
    _, uuid = run(client, "sing-box generate uuid")
    uuid = uuid.strip().splitlines()[-1].strip()
    _, shortid = run(client, "openssl rand -hex 8")
    shortid = shortid.strip().splitlines()[-1].strip()

    print("\n=== GENERATED KEYS ===")
    print("Generated a fresh server keypair and client profile; credentials are not printed.")

    # Step 5: write server config
    print("\n>>> STEP 5: write /etc/sing-box/config.json")
    server_conf = f'''{{
  "log": {{ "level": "info", "timestamp": true }},
  "inbounds": [
    {{
      "type": "vless",
      "tag": "vless-in",
      "listen": "::",
      "listen_port": 443,
      "users": [ {{ "uuid": "{uuid}", "flow": "xtls-rprx-vision" }} ],
      "tls": {{
        "enabled": true,
        "server_name": "www.microsoft.com",
        "reality": {{
          "enabled": true,
          "handshake": {{ "server": "www.microsoft.com", "server_port": 443 }},
          "private_key": "{priv}",
          "short_id": ["{shortid}"]
        }}
      }}
    }}
  ],
  "outbounds": [ {{ "type": "direct", "tag": "direct" }} ]
}}
'''
    # write via SFTP to be safe with quoting
    sftp = client.open_sftp()
    with sftp.file("/etc/sing-box/config.json", "w") as f:
        f.write(server_conf)
    sftp.close()
    print("config written.")

    # Validate config before restart
    run(client, "sing-box check -c /etc/sing-box/config.json")

    # Step 6: firewall
    print("\n>>> STEP 6: ufw allow 443 + 22")
    run(client, "ufw --force enable")
    run(client, "ufw allow 443/tcp && ufw allow 22/tcp")

    # Step 7: enable + restart sing-box
    print("\n>>> STEP 7: systemctl enable + restart sing-box")
    run(client, "systemctl enable sing-box")
    run(client, "systemctl restart sing-box")
    time.sleep(2)
    run(client, "systemctl is-active sing-box")

    # Step 8: verify listening
    print("\n>>> STEP 8: verify port 443")
    run(client, "ss -tlnp | grep ':443' || echo 'NOT LISTENING'")

    # Write client config to a file on the server too, and print it
    client_conf = f'''{{
  "outbounds": [
    {{
      "type": "vless",
      "tag": "vless-reality",
      "server": "{HOST}",
      "server_port": 443,
      "uuid": "{uuid}",
      "flow": "xtls-rprx-vision",
      "tls": {{
        "enabled": true,
        "server_name": "www.microsoft.com",
        "utls": {{ "enabled": true, "fingerprint": "chrome" }},
        "reality": {{
          "enabled": true,
          "public_key": "{pub}",
          "short_id": "{shortid}"
        }}
      }}
    }}
  ]
}}
'''
    print("\n=== CLIENT CONFIG ===")
    print("Client profile generated locally; credentials are not printed.")

    # save client config locally too
    client_config_path = require_env("SNOWDEN_CLIENT_CONFIG_PATH")
    with open(client_config_path, "w", encoding="utf-8") as f:
        # wrap into a full sing-box config with mixed inbound + route
        full = f'''{{
  "log": {{ "level": "info", "timestamp": true }},
  "dns": {{
    "servers": [
      {{ "tag": "cloudflare", "address": "https://1.1.1.1/dns-query", "detour": "proxy" }},
      {{ "tag": "local", "address": "local", "detour": "direct" }}
    ],
    "rules": [ {{ "outbound": "any", "server": "local" }} ],
    "strategy": "ipv4_only"
  }},
  "inbounds": [
    {{
      "type": "mixed",
      "tag": "mixed-in",
      "listen": "127.0.0.1",
      "listen_port": 20808,
      "sniff": true,
      "set_system_proxy": true
    }}
  ],
  "outbounds": [
    {{
      "type": "vless",
      "tag": "proxy",
      "server": "{HOST}",
      "server_port": 443,
      "uuid": "{uuid}",
      "flow": "xtls-rprx-vision",
      "tls": {{
        "enabled": true,
        "server_name": "www.microsoft.com",
        "utls": {{ "enabled": true, "fingerprint": "chrome" }},
        "reality": {{
          "enabled": true,
          "public_key": "{pub}",
          "short_id": "{shortid}"
        }}
      }}
    }},
    {{ "type": "direct", "tag": "direct" }},
    {{ "type": "block", "tag": "block" }},
    {{ "type": "dns", "tag": "dns-out" }}
  ],
  "route": {{
    "rules": [
      {{ "protocol": "dns", "outbound": "dns-out" }},
      {{ "ip_is_private": true, "outbound": "direct" }}
    ],
    "final": "proxy",
    "default_domain_resolver": "local",
    "auto_detect_interface": true
  }},
  "experimental": {{
    "clash_api": {{ "external_controller": "127.0.0.1:9090", "secret": "YOUR_CLASH_API_SECRET" }}
  }}
}}
'''
        f.write(full)
    print("client config saved to SNOWDEN_CLIENT_CONFIG_PATH (path value not printed)")

    client.close()
    print("\n=== DONE ===")

if __name__ == "__main__":
    main()
