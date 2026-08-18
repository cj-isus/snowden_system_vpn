#!/usr/bin/env python3
"""Replace old VPS servers with the new Netherlands Hysteria2 server in
every copy of template-vps-reality.json."""
import json
import sys

NEW = {
    "type": "hysteria2",
    "tag": "hysteria2-nl",
    "server": "89.125.1.217",
    "server_port": 8443,
    "password": "u2u65B4QHw8YkXpb5Y",
    "tls": {
        "enabled": True,
        "server_name": "snowden-system.89-125-1-217.nip.io",
    },
}

DROPPED = {
    "grpc-fr", "httpupgrade-fr", "vless-fr",
    "grpc-nl", "httpupgrade-nl", "vless-nl", "hysteria2-nl",
}

paths = [
    r"D:\ОБХОДЫ\unkillable-vpn\assets\configs\template-vps-reality.json",
    r"D:\ОБХОДЫ\unkillable-vpn\build\bin\assets\configs\template-vps-reality.json",
    r"D:\ОБХОДЫ\Snowden_system\snowden-portable\assets\configs\template-vps-reality.json",
]

for p in paths:
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
        if srv in ("78.17.160.83", "192.109.206.234"):
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
    assert all(o.get("server") not in ("78.17.160.83", "192.109.206.234") for o in objs if "server" in o)
    print(f"OK {p}  outbounds={[o.get('tag') for o in objs]}")

print("DONE")
