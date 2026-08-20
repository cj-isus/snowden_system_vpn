#!/usr/bin/env python3
"""Replace old VPS servers with the new Netherlands Hysteria2 server in
every copy of template-vps-reality.json."""
import json
import os
import sys

NEW = {
    "type": "hysteria2",
    "tag": "hysteria2-nl",
    "server": os.environ["SNOWDEN_VPS_IP"],
    "server_port": int(os.environ.get("SNOWDEN_HY2_PORT", "8443")),
    "password": os.environ["SNOWDEN_HY2_PASSWORD"],
    "tls": {
        "enabled": True,
        "server_name": os.environ["SNOWDEN_VPN_DOMAIN"],
    },
}

DROPPED = {
    "grpc-fr", "httpupgrade-fr", "vless-fr",
    "grpc-nl", "httpupgrade-nl", "vless-nl", "hysteria2-nl",
}

paths = [
    os.environ["SNOWDEN_RUNTIME_CONFIG_PATH"],
    os.environ.get("SNOWDEN_BUILD_CONFIG_PATH"),
    os.environ.get("SNOWDEN_PORTABLE_CONFIG_PATH"),
]

for p in [path for path in paths if path]:
    with open(p, "r", encoding="utf-8") as f:
        cfg = json.load(f)

    outbounds = cfg["outbounds"]

    # Drop old servers (by tag or by old server IP).
    kept = []
    for ob in outbounds:
        tag = ob.get("tag")
        srv = ob.get("server")
        if tag in DROPPED:
            continue
        old_ip = os.environ.get("SNOWDEN_OLD_VPS_IP")
        if old_ip and srv == old_ip:
            continue
        kept.append(ob)
    cfg["outbounds"] = kept

    # Update the urltest "auto" group to reference only the new server.
    for ob in kept:
        if ob.get("type") == "urltest" and ob.get("tag") == "auto":
            ob["outbounds"] = [NEW["tag"]]
            break

    # Insert the new outbound before the first non-direct/block entry marker:
    # simply append before any "direct" / "block" to keep them last.
    insert_at = len(cfg["outbounds"])
    for i, ob in enumerate(cfg["outbounds"]):
        if ob.get("type") in ("direct", "block"):
            insert_at = i
            break
    cfg["outbounds"].insert(insert_at, NEW)

    with open(p, "w", encoding="utf-8") as f:
        json.dump(cfg, f, indent=2, ensure_ascii=False)

    # sanity: verify the result parses and references are consistent
    json.load(open(p, "r", encoding="utf-8"))
    auto = next(o for o in json.load(open(p, "r", encoding="utf-8"))["outbounds"]
                if o.get("tag") == "auto")
    tags = {o["tag"] for o in json.load(open(p, "r", encoding="utf-8"))["outbounds"]}
    assert set(auto["outbounds"]) <= tags, f"auto refs missing in {p}"
    old = json.load(open(p, "r", encoding="utf-8"))
    objs = old["outbounds"]
    assert all(o.get("server") != os.environ.get("SNOWDEN_OLD_VPS_IP", "") for o in objs if "server" in o)
    print(f"OK {p}  outbounds={[o.get('tag') for o in objs]}")

print("DONE")
