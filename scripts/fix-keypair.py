"""Regenerate a matching Reality keypair and provision local client configs.

The script deliberately keeps private material in memory/SFTP only. It never
prints generated keys, remote config contents, or credential values.
"""
from __future__ import annotations

import base64
import json
import os
import shutil
from pathlib import Path

import paramiko

from env import load_project_env, require_env

load_project_env()

HOST = require_env("SNOWDEN_VPS_IP")
PORT = int(os.environ.get("SNOWDEN_VPS_SSH_PORT", "22"))
USER = os.environ.get("SNOWDEN_VPS_SSH_USER", "root")
PASSWORD = require_env("SNOWDEN_VPS_SSH_PASSWORD")
VPS_UUID = require_env("SNOWDEN_VPS_UUID")
REALITY_SHORT_ID = require_env("SNOWDEN_REALITY_SHORT_ID")
CLIENT_CONFIG_PATH = Path(require_env("SNOWDEN_CLIENT_CONFIG_PATH"))
BUILD_CONFIG_PATH = os.environ.get("SNOWDEN_BUILD_CONFIG_PATH")


def remote_output(client: paramiko.SSHClient, command: str) -> tuple[int, str]:
    """Run a remote command without printing its output."""
    _, stdout, stderr = client.exec_command(command, get_pty=True)
    output = stdout.read().decode("utf-8", "replace")
    output += stderr.read().decode("utf-8", "replace")
    return stdout.channel.recv_exit_status(), output.strip()


def parse_keypair(output: str) -> tuple[str, str]:
    private_key = ""
    public_key = ""
    for line in output.splitlines():
        fields = line.split()
        if not fields:
            continue
        if "PrivateKey" in line:
            private_key = fields[-1]
        elif "PublicKey" in line:
            public_key = fields[-1]
    if not private_key or not public_key:
        raise RuntimeError("sing-box did not return a complete Reality keypair")
    return private_key, public_key


def derive_public_key(private_key: str) -> None:
    """Validate an optional local private key without printing key material."""
    try:
        from cryptography.hazmat.primitives import serialization
        from cryptography.hazmat.primitives.asymmetric.x25519 import X25519PrivateKey

        padded = private_key + "=" * (-len(private_key) % 4)
        raw = base64.urlsafe_b64decode(padded)
        if len(raw) != 32:
            print("local Reality key was ignored: expected 32 bytes")
            return
        X25519PrivateKey.from_private_bytes(raw).public_key().public_bytes(
            encoding=serialization.Encoding.Raw,
            format=serialization.PublicFormat.Raw,
        )
        print("local Reality private key decoded successfully")
    except Exception as exc:
        print(f"local Reality key validation skipped: {exc}")


def main() -> None:
    client = paramiko.SSHClient()
    client.set_missing_host_key_policy(paramiko.AutoAddPolicy())
    print(f"Connecting to {USER}@{HOST}:{PORT} ...")
    client.connect(
        HOST,
        port=PORT,
        username=USER,
        password=PASSWORD,
        timeout=20,
        allow_agent=False,
        look_for_keys=False,
    )

    try:
        configured_private_key = os.environ.get("SNOWDEN_REALITY_PRIVATE_KEY", "")
        if configured_private_key:
            derive_public_key(configured_private_key)
        else:
            print("local Reality private key is not provisioned; using server keypair")

        print("Generating a fresh matching Reality keypair on the server ...")
        code, output = remote_output(client, "sing-box generate reality-keypair")
        if code != 0:
            raise RuntimeError("sing-box reality-keypair generation failed")
        private_key, public_key = parse_keypair(output)

        server_config = {
            "log": {"level": "info", "timestamp": True},
            "inbounds": [
                {
                    "type": "vless",
                    "tag": "vless-in",
                    "listen": "::",
                    "listen_port": 443,
                    "users": [{"uuid": VPS_UUID, "flow": "xtls-rprx-vision"}],
                    "tls": {
                        "enabled": True,
                        "server_name": "www.microsoft.com",
                        "reality": {
                            "enabled": True,
                            "handshake": {
                                "server": "www.microsoft.com",
                                "server_port": 443,
                            },
                            "private_key": private_key,
                            "short_id": [REALITY_SHORT_ID],
                        },
                    },
                }
            ],
            "outbounds": [{"type": "direct", "tag": "direct"}],
        }
        sftp = client.open_sftp()
        try:
            with sftp.file("/etc/sing-box/config.json", "w") as remote_file:
                remote_file.write(json.dumps(server_config, indent=2))
        finally:
            sftp.close()

        code, _ = remote_output(
            client,
            "sing-box check -c /etc/sing-box/config.json && "
            "systemctl restart sing-box && sleep 2 && systemctl is-active sing-box",
        )
        if code != 0:
            raise RuntimeError("server rejected the regenerated config")
        print("Server keypair installed and sing-box restarted")
    finally:
        client.close()

    client_config = {
        "log": {"level": "info", "timestamp": True},
        "dns": {
            "servers": [
                {
                    "type": "https",
                    "tag": "cloudflare",
                    "server": "1.1.1.1",
                    "path": "/dns-query",
                    "detour": "proxy",
                },
                {"type": "local", "tag": "local", "detour": "direct"},
            ],
            "rules": [{"outbound": "any", "server": "local"}],
            "strategy": "ipv4_only",
        },
        "inbounds": [
            {
                "type": "mixed",
                "tag": "mixed-in",
                "listen": "127.0.0.1",
                "listen_port": 20808,
            }
        ],
        "outbounds": [
            {
                "type": "selector",
                "tag": "proxy",
                "outbounds": ["vless-reality"],
                "default": "vless-reality",
            },
            {
                "type": "vless",
                "tag": "vless-reality",
                "server": HOST,
                "server_port": 443,
                "uuid": VPS_UUID,
                "flow": "xtls-rprx-vision",
                "tls": {
                    "enabled": True,
                    "server_name": "www.microsoft.com",
                    "utls": {"enabled": True, "fingerprint": "chrome"},
                    "reality": {
                        "enabled": True,
                        "public_key": public_key,
                        "short_id": REALITY_SHORT_ID,
                    },
                },
            },
            {"type": "direct", "tag": "direct"},
            {"type": "block", "tag": "block"},
        ],
        "route": {
            "rules": [
                {"action": "sniff"},
                {"action": "hijack-dns", "inbound": "mixed-in"},
                {"ip_is_private": True, "action": "direct"},
            ],
            "final": "proxy",
            "default_domain_resolver": "local",
            "auto_detect_interface": True,
        },
    }

    CLIENT_CONFIG_PATH.parent.mkdir(parents=True, exist_ok=True)
    CLIENT_CONFIG_PATH.write_text(
        json.dumps(client_config, indent=2, ensure_ascii=False),
        encoding="utf-8",
    )
    if BUILD_CONFIG_PATH:
        build_path = Path(BUILD_CONFIG_PATH)
        build_path.parent.mkdir(parents=True, exist_ok=True)
        shutil.copy2(CLIENT_CONFIG_PATH, build_path)

    print("Local client config written; credential values were not printed")


if __name__ == "__main__":
    main()
