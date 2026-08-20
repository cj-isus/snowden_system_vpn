# STRUCTURE.md — `configs/cloudflare/`

Cloudflare is a metadata and edge-observation layer. It is not the owner of the
local VPN channel and does not prove that the user's ISP can reach a VPS.

## Files

| File | Role |
|---|---|
| `worker.js` | Worker routes, public config metadata, edge health and optional telemetry |
| `r2-worker.js` | separate R2 download worker |
| `wrangler.toml.example` | public binding/config example |
| `wrangler.toml` | local deployment config; do not commit credentials |
| `schema.sql` | D1 telemetry schema |
| `README.md` | deploy and KV/D1 setup instructions |

## Endpoints in `worker.js`

| Endpoint | Meaning |
|---|---|
| `GET /api/config` | public metadata only; `publicConfig` strips UUID/password/private key/token/secret fields |
| `GET /api/health` | fetches VPS/Internet checks from the current Cloudflare edge; informational, not local-path proof |
| `GET /api/version` | version metadata from KV |
| `POST /api/telemetry` | optional anonymous event storage in D1; no credentials |
| `/`, `/version.json`, download paths | KV-backed landing/version/download responses |

## Safety rules

- VPS address/domain come from Worker bindings (`VPS_IP`, `VPN_DOMAIN`) or a
  deliberately non-working placeholder fallback.
- The Worker must never manufacture or return client UUID/password/private key.
- `node --check worker.js` is the minimum syntax gate. Production health is
  accepted only after a real Worker deployment and edge request.
