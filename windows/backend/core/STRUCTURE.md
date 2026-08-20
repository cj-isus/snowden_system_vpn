# STRUCTURE.md — `windows/backend/core`

Package `core` owns the Windows VPN runtime around an embedded sing-box-lx
`box.Box`.

## Components

| File | Contract |
|---|---|
| `engine.go` | serialized `Start`, `Close`, `Reload`, `Wait`; states Stopped/Starting/Running/Stopping/Error; panic and timeout boundaries |
| `manager.go` | one public lifecycle facade; immutable config snapshots; one metrics worker; active channel state |
| `adaptive.go` | protected probe loop, classifier, circuit breaker, recovery callback through Manager |
| `channels.go` | derives protected `ChannelDescriptor` values from selector `proxy`; applies a validated default |
| `channel_memory.go` | secret-free channel keys, scores, persistence, prune/cap and restart memory |
| `classifier.go` | operational error categories and rolling log events |
| `metrics.go` | traffic best-effort sampling, server parsing, route parsing, TCP ping, config I/O |
| `domain_stats.go` | per-domain observations; informational only until wired to policy |
| `registry.go` | exact sing-box protocol registrations for the build |

## Ownership rules

```text
UI / Telegram / AdaptiveEngine
              │
              ▼
          Manager
              │
              ▼
          Engine → embedded box.Box
```

AdaptiveEngine never calls `Engine.Reload` in production. Recovery calls the
Manager callback, which updates engine, active config, metrics and then the
adaptive snapshot consistently.

## Protected channel model

```text
route.final → selector "proxy" → [only validated protected tags]
                                            ├── current default
                                            ├── alternative candidate
                                            └── no direct fallback
```

`ChannelKey` hashes endpoint identity and does not persist IP, UUID, password or
private key in the memory file. `ChannelMemory` is a preference/history signal,
not proof that a channel is currently live; every selected candidate still needs
a protected probe.

## Circuit breaker

```text
Closed --2 consecutive probe failures--> Open
Open --10/20/40/60s cooldown--> HalfOpen
HalfOpen --2 successes--> Closed
HalfOpen --1 failure--> Open
```

Closed probes run on a bounded cadence. Recovery is fail-closed: if no validated
protected candidate works, the route is not changed to `direct`.

## Metrics limitation

The embedded runtime does not automatically expose the Clash API. `Metrics` and
`PollConnections` therefore return real data only when the exact build/config
provides the endpoint; otherwise zero/empty values mean unavailable, not fake
traffic. This must remain visible in UI/docs rather than being presented as a
successful measurement.

## Tests

- `engine_test.go`, `engine_ext_test.go`: lifecycle and waiter contracts.
- `manager_test.go`: single metrics worker and stop safety.
- `adaptive_test.go`: circuit transitions and backoff.
- `channel_memory_test.go`, `channels` coverage: persistence and selection.
- `validator` tests live in `backend/config` because validation is a separate
  package boundary.

Run from `windows/`:

```text
go test ./...
go vet ./...
go test -race ./...   # requires CGO/C compiler on Windows
```
