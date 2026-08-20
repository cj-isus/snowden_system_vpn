# STRUCTURE.md — `components/Dashboard`

| Component | Responsibility |
|---|---|
| `StatusCard.vue` | lifecycle state, actual active channel, protected latency, power action |
| `TrafficCard.vue` | speed/session counters; zero values are not proof of traffic health |
| `DiagnosticsCard.vue` | probe/circuit category, actual server/protocol and unavailable signals |
| `EventsCard.vue` | adaptive event feed from App |
| `DomainStatsCard.vue` | informational per-domain observations from `DomainStatsRegistry` |
| `LogsCard.vue` | filtered log view when used by layout |

Cards poll their own Wails methods and stop timers on unmount. No dashboard card
selects a protocol by name or changes route policy directly. `DiagnosticsCard`
uses `GetServers()` to avoid hardcoded `VLESS+TLS`/`VPS Netherlands` claims.
