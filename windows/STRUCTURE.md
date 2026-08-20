# STRUCTURE.md — windows/

`windows/` — Wails desktop application. `package main` is the OS/application
facade; VPN business rules live in `backend/core`.

## Files

| File | Responsibility |
|---|---|
| `main.go` | Wails bootstrap, embedded frontend, shutdown ordering |
| `app.go` | Wails methods, env loading, config import staging, system proxy, events |
| `backend/core/` | embedded sing-box lifecycle and adaptive controller |
| `backend/config/` | runtime config normalization and validation |
| `backend/cfclient/` | Cloudflare Worker client |
| `telegram_bot.go` | optional remote log/reporting integration; credentials from env |
| `proxy_windows.go` | Windows HTTP proxy registry integration |
| `crash_windows.go` | stale-proxy cleanup and crash/shutdown hooks |
| `tray_windows.go` | tray UI and hide/exit behavior |
| `autostart_windows.go` | HKCU autostart setting |
| `assets/configs/` | runtime config copies bundled beside the executable |

## Start/stop flow

```text
App.StartVPN
  → Manager.StartVPN
    → NormalizeProtectedRoute
    → config.Validate(fail-closed)
    → Engine.Start (embedded box.Box)
  → setSystemProxy
  → AdaptiveEngine.Start(normalized snapshot)

App.StopVPN
  → AdaptiveEngine.Stop
  → clearSystemProxy
  → Manager.StopVPN
  → Engine.Close
```

Reloads use the same Manager boundary and update the adaptive snapshot only after
the new embedded instance has started successfully. A failed start never leaves
the Windows proxy enabled through the normal App path.

## Config import

`ImportConfig` validates and stages a normalized snapshot. The next
`LoadConfigFile` consumes that snapshot, so Settings → Import affects the next
VPN start without writing an imported secret into the repository.

## Boundaries

- The engine is **embedded**, not `sing-box.exe` subprocess.
- No production behavior may depend on Clash API being enabled; metrics and
  connection stats are best-effort and must be shown as unavailable when absent.
- `backend/config/builder.go` is legacy. It is not the active config builder.
