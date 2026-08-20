# Архитектура snowden.system

> Карта проекта для человека и ИИ. Фактическая реализация сверяется с кодом,
> а не с историческими описаниями протоколов.

## 1. Компоненты

| Компонент | Путь | Роль |
|---|---|---|
| Windows facade | `windows/app.go`, `windows/main.go` | Wails API, env, proxy, shutdown, UI events |
| Embedded core | `windows/backend/core/` | `box.Box`, lifecycle, selector channel model, probes, circuit breaker |
| Runtime config | `windows/backend/config/` | normalize + fail-closed validation before `box.New` |
| Frontend | `windows/frontend/src/` | dashboard, dynamic channel list, diagnostics, settings |
| sing-box source configs | `configs/singbox/` | templates and deployment inputs; runtime copy is under `windows/assets/configs/` |
| Cloudflare | `configs/cloudflare/` | public metadata, edge health, version and optional telemetry |
| Mobile | `android/`, `ios/` | separate libbox clients; credentials injected at local build time |
| Scripts | `scripts/` | explicit operator tools; not part of the VPN runtime |

## 2. Windows data flow

```text
Vue/Wails UI
    │ Start / Stop / Select / Import
    ▼
App facade
    │ one serialized public boundary
    ▼
Manager
    ├── NormalizeProtectedRoute + Validate
    ├── Engine: embedded sing-box box.Box
    ├── Metrics: one cancellable sampler
    └── AdaptiveEngine: probes + circuit breaker + ChannelMemory
            │ Apply protected snapshot through Manager.ReloadVPN
            ▼
route.final → selector "proxy" → validated protected outbound
                                            ├── Hysteria2 VPS (if configured)
                                            ├── other validated channels
                                            └── block when no protected route is usable
```

`direct` is allowed only in explicit route rules such as private/RU traffic. It
is never a candidate in the protected selector. `urltest` may remain in a
legacy snapshot for diagnostics, but Manager normalizes the live route so the
AdaptiveEngine is the only policy owner.

## 3. Lifecycle contract

```text
StartVPN → normalize/validate → Engine.Start → running → Adaptive.Start
ReloadVPN → normalize/validate → Engine.Reload → update snapshots
StopVPN → Adaptive.Stop → Manager.StopVPN → Engine.Close → clear proxy
```

`Engine.Reload` rebuilds the embedded `box.Box`; there is no sing-box subprocess
and no assumption that Clash API hot reload is available.

## 4. Config and secrets

- Edit source templates under `configs/singbox/`, then sync to
  `windows/assets/configs/` when appropriate.
- Runtime Start/Reload validates JSON references, placeholders, selector
  candidates and `route.final`.
- Public Cloudflare metadata strips credentials. Android/iOS use local
  `--dart-define-from-file`/equivalent provisioning.
- `.env`, `config.local.json`, private keys, UUIDs and passwords stay outside
  commits. `.gitignore` is not a remedy for a secret that was already exposed.

## 5. Verification entry points

```powershell
cd D:\snowden-v2\windows
& 'C:\Program Files\Go\bin\go.exe' test ./...
& 'C:\Program Files\Go\bin\go.exe' vet ./...
& 'C:\Program Files\Go\bin\go.exe' build -tags "with_awg,with_wireguard,with_utls,with_gvisor" ./...

cd D:\snowden-v2\windows\frontend
npm run build -- --emptyOutDir=false

node --check D:\snowden-v2\configs\cloudflare\worker.js
```

`go test -race` additionally requires a working C compiler on Windows. Live
server/protocol checks and Android/iOS acceptance are separate from unit tests.
