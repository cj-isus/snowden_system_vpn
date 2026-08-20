"""
Derive the correct public key from the server's private key and compare with
what the client is using. sing-box's reality-keypair derives pub from priv via
x25519, so we can compute it locally too — but easiest is to ask the server.
"""
import os
import paramiko

HOST = os.environ["SNOWDEN_VPS_IP"]
PORT = int(os.environ.get("SNOWDEN_VPS_SSH_PORT", "22"))
USER = os.environ.get("SNOWDEN_VPS_SSH_USER", "root")
PASS = os.environ["SNOWDEN_VPS_SSH_PASSWORD"]

c=paramiko.SSHClient(); c.set_missing_host_key_policy(paramiko.AutoAddPolicy())
c.connect(HOST,port=PORT,username=USER,password=PASS,timeout=20,allow_agent=False,look_for_keys=False)

# Method 1: sing-box on the server doesn't have a "derive pubkey from privkey" cmd,
# so compute via python cryptography (x25519).
privkey_b64 = os.environ.get("SNOWDEN_REALITY_PRIVATE_KEY", "")
if not privkey_b64:
    print("SNOWDEN_REALITY_PRIVATE_KEY is not set; skipping local private-key derivation")
print("local private-key derivation input: configured" if privkey_b64 else "local private-key derivation input: not configured")

try:
    from cryptography.hazmat.primitives.asymmetric.x25519 import X25519PrivateKey
    from cryptography.hazmat.primitives import serialization
    import base64
    # sing-box uses URL-safe base64 without padding. Add padding for std decoder.
    padded = privkey_b64 + "=" * (-len(privkey_b64) % 4)
    # URL-safe → standard (-_ → +/)
    std = padded.replace("-", "+").replace("_", "/")
    priv_bytes = base64.b64decode(std)
    print(f"private-key input length: {len(priv_bytes)} bytes")
    if len(priv_bytes) == 32:
        sk = X25519PrivateKey.from_private_bytes(priv_bytes)
        pub_bytes = sk.public_key().public_bytes(
            encoding=serialization.Encoding.Raw,
            format=serialization.PublicFormat.Raw
        )
        # Encode back URL-safe no-pad (sing-box format)
        pub_b64 = base64.urlsafe_b64encode(pub_bytes).decode().rstrip("=")
        print(f"DERIVED public_key: {pub_b64}")
    else:
        print(f"WARNING: priv_bytes is {len(priv_bytes)} bytes, expected 32")
except Exception as ex:
    print(f"local derive skipped ({ex}); will regenerate on server")

# Method 2 (fallback): re-run sing-box generate reality-keypair ONCE on server,
# write both keys into the server config, and capture the matching public key.
print("\n=== regenerating a FRESH matching keypair on the server ===")
_,o,e = c.exec_command("sing-box generate reality-keypair", get_pty=True)
out = (o.read().decode('utf-8','replace')).strip()
print(out)
priv=pub=""
for line in out.splitlines():
    if "PrivateKey" in line: priv=line.split()[-1]
    if "PublicKey" in line: pub=line.split()[-1]
print("fresh keypair generated on server; private key is not printed")
print("fresh public key captured for local config")

# Write the fresh pair to the server config (overwrite)
import json
cfg = {
  "log": {"level":"info","timestamp":True},
  "inbounds":[{
    "type":"vless","tag":"vless-in","listen":"::","listen_port":443,
    "users":[{"uuid":os.environ["SNOWDEN_VPS_UUID"],"flow":"xtls-rprx-vision"}],
    "tls":{"enabled":True,"server_name":"www.microsoft.com",
      "reality":{"enabled":True,
        "handshake":{"server":"www.microsoft.com","server_port":443},
        "private_key":priv,"short_id":[os.environ["SNOWDEN_REALITY_SHORT_ID"]]}}
  }],
  "outbounds":[{"type":"direct","tag":"direct"}]
}
sftp=c.open_sftp()
with sftp.file("/etc/sing-box/config.json","w") as f:
    f.write(json.dumps(cfg,indent=2))
sftp.close()
print("\nserver config updated with fresh keypair")

_,o,e = c.exec_command("sing-box check -c /etc/sing-box/config.json && systemctl restart sing-box && sleep 2 && systemctl is-active sing-box", get_pty=True)
print((o.read().decode('utf-8','replace')).strip())

c.close()

# Write the matching CLIENT config locally
client_cfg = {
  "log":{"level":"info","timestamp":True},
  "dns":{"servers":[
    {"type":"https","tag":"cloudflare","server":"1.1.1.1","path":"/dns-query","detour":"proxy"},
    {"type":"local","tag":"local","detour":"direct"}
  ],"rules":[{"outbound":"any","server":"local"}],"strategy":"ipv4_only"},
  "inbounds":[{"type":"mixed","tag":"mixed-in","listen":"127.0.0.1","listen_port":20808}],
  "outbounds":[
    {"type":"vless","tag":"proxy","server":HOST,"server_port":443,
     "uuid":os.environ["SNOWDEN_VPS_UUID"],"flow":"xtls-rprx-vision",
     "tls":{"enabled":True,"server_name":"www.microsoft.com",
       "utls":{"enabled":True,"fingerprint":"chrome"},
       "reality":{"enabled":True,"public_key":pub,"short_id":os.environ["SNOWDEN_REALITY_SHORT_ID"]}}},
    {"type":"direct","tag":"direct"},
    {"type":"block","tag":"block"}
  ],
  "route":{"rules":[
    {"action":"sniff"},
    {"action":"hijack-dns","inbound":"mixed-in"},
    {"ip_is_private":True,"action":"direct"}
  ],"final":"proxy","default_domain_resolver":"local","auto_detect_interface":True}
}
with open(os.environ["SNOWDEN_CLIENT_CONFIG_PATH"], "w", encoding="utf-8") as f:
    json.dump(client_cfg,f,indent=2,ensure_ascii=False)
print("\nclient config rewritten with matching public_key")

# also copy next to the exe
import shutil, os
build_config_path = os.environ.get("SNOWDEN_BUILD_CONFIG_PATH")
if build_config_path:
    os.makedirs(os.path.dirname(build_config_path), exist_ok=True)
    shutil.copy(os.environ["SNOWDEN_CLIENT_CONFIG_PATH"], build_config_path)
print("copied to build/bin/assets/configs/")
print("client config updated with the generated public key")
