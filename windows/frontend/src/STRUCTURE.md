# STRUCTURE.md — `windows/frontend/src`

Vue 3 + TypeScript frontend for the Wails facade. Components call generated
Wails bindings; they do not construct VPN protocol configs themselves.

## Layers

```text
App.vue
  ├── Layout: navigation, status and terminal
  ├── Servers: actual protected channels + route rules
  ├── Dashboard: status, diagnostics, traffic, events, domain stats
  └── Settings: autostart, validated import, export
       │
       ▼
  wailsjs/go/main/App → windows/app.go → backend/core
```

## Data ownership

- `App.vue` owns global lifecycle state and Wails events (`engine:log`,
  `engine:diag`).
- `ServersCard` renders the list returned by `GetServers`; it does not assume NL,
  FR or any other country exists.
- `RoutingCard` keeps the backend's original `rule-N` index so filtered service
  rules cannot toggle the wrong route.
- `DiagnosticsCard` shows the actual active server/protocol and labels missing
  Clash/traffic data as unavailable rather than inventing values.
- `SettingsCard` stages a validated import for the next start and exports the
  active normalized snapshot.

## Feature docs

- `components/Servers/STRUCTURE.md`
- `components/Dashboard/STRUCTURE.md`
- `components/Settings/STRUCTURE.md`
- `components/Layout/STRUCTURE.md`

Generated `wailsjs/` bindings are outputs of Wails and should not be hand-edited.
