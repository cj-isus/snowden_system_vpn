# STRUCTURE.md — `configs/singbox/`

This directory is the source of truth for sing-box JSON inputs. The Windows
runtime reads a synchronized copy under `windows/assets/configs/`; the Manager
normalizes and validates it before starting embedded sing-box.

## Components

| File | Status |
|---|---|
| `template-vps-reality.json` | deployment-specific desktop config; the actual protected candidates are whatever the file contains, commonly Hysteria2/VPS entries |
| `template-vps-reality.json.example` | public placeholder example; never a release credential source |
| `template-reality.json` | minimal selector-based Reality example with placeholders |
| `template-warp-awg.json` | standalone WARP/AmneziaWG experiment; `YOUR_*` and `i1` are placeholders until live validation |
| `warp-outbound.json`, `warp2.json` | auxiliary WARP formats; not automatically part of Windows failover |
| `mieru-credentials*.json` | secret material; external client integration only, not native sing-box outbound |
| `server-params*.json`, `warp-keys*.json` | provisioning inputs; keep local/ignored |

## Target graph

```text
route.final → selector "proxy" → actual protected outbounds in this file
                                         ├── Hysteria2/VPS when provisioned
                                         ├── validated WARP/MASQUE candidate later
                                         └── never direct
```

Legacy `urltest`/`route.final: auto` inputs are normalized at runtime. `urltest`
may remain available for diagnostics, but it is not a second owner of live
selection. A clean example must not claim that VLESS, FR, AWG or mieru is live
unless its exact config and binary have been accepted.

## Provisioning rules

- Replace placeholders only in a local ignored config or a controlled release
  renderer.
- Keep UUID/password/private key out of public Worker metadata and docs.
- After changing a source template, sync it deliberately and run the Windows
  validator/build; do not edit a stale bundled copy by hand.
