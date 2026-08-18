# Snowden System — iOS

> Full port of PC algorithms: VLESS+Hysteria2+urltest, split-tunneling, adaptive engine.
> UI: single logo button. Under the hood: everything smart and complex.

## Architecture

```
┌─────────────────────────────────────────┐
│  SnowdenSystem (SwiftUI App)            │
│  ┌─────────────────────────────────┐    │
│  │  ContentView — Logo Button      │    │
│  │  ├─ StatusBar (top)             │    │
│  │  ├─ LogoButton (center)         │    │
│  │  │   ├─ Animated glow ring     │    │
│  │  │   ├─ Pepe logo image          │    │
│  │  │   ├─ Status dot               │    │
│  │  │   └─ Press animation          │    │
│  │  ├─ Status text                 │    │
│  │  └─ Bottom toolbar              │    │
│  │      ├─ Diagnostics (sheet)     │    │
│  │      ├─ Logs (sheet)             │    │
│  │      └─ Refresh                  │    │
│  └─────────────────────────────────┘    │
│           │                             │
│  ┌────────▼────────┐                   │
│  │  VPNManager     │  ← Swift facade   │
│  │  ├─ GoCore      │  ← gomobile bind  │
│  │  ├─ NE Manager  │  ← iOS VPN tunnel │
│  │  └─ Status pol  │  ← 2s timer       │
│  └─────────────────┘                   │
└─────────────────────────────────────────┘
                    │
┌───────────────────▼─────────────────────┐
│  SnowdenSystemVPN (Network Extension)     │
│  ┌─────────────────────────────────┐     │
│  │  PacketTunnelProvider           │     │
│  │  ├─ NEPacketTunnelProvider      │     │
│  │  ├─ GoCore (same framework)     │     │
│  │  ├─ Tunnel settings (127.0.0.1)│     │
│  │  └─ Handle app messages         │     │
│  └─────────────────────────────────┘     │
└─────────────────────────────────────────┘
                    │
┌───────────────────▼─────────────────────┐
│  SnowdenCore.xcframework (Go)           │
│  ┌─────────────────────────────────┐   │
│  │  Engine (box.Box)               │   │
│  │  ├─ Start/Close/Reload          │   │
│  │  ├─ PlatformLogWriter           │   │
│  │  └─ State machine (atomic)      │   │
│  ├─ Manager                         │   │
│  │  ├─ StartVPN/StopVPN/ReloadVPN   │   │
│  │  └─ Status JSON                   │   │
│  ├─ AdaptiveEngine                    │   │
│  │  ├─ CircuitBreaker (3-state)     │   │
│  │  ├─ ErrorClassifier              │   │
│  │  ├─ deepProbe (30s interval)     │   │
│  │  └─ auto-reload on failure       │   │
│  ├─ ErrorClassifier                   │   │
│  │  ├─ 8 categories                  │   │
│  │  ├─ Pattern matching              │   │
│  │  └─ Rolling buffer (200 events) │   │
│  ├─ ConfigBuilder                     │   │
│  │  ├─ VLESS + urltest + WARP       │   │
│  │  ├─ Split-tunneling (ru-cidr)    │   │
│  │  └─ DNS (detour: auto)            │   │
│  └─ Registry (selective protocols)   │   │
│      ├─ Inbounds: tun, mixed, socks │   │
│      ├─ Outbounds: vless, hysteria2 │   │
│      ├─ Groups: selector, urltest   │   │
│      └─ Endpoints: wireguard (AWG)   │   │
└─────────────────────────────────────────┘
```

## File Structure

```
SnowdenSystem/
├── SnowdenSystem/
│   ├── ContentView.swift          # Main UI (logo button)
│   ├── VPNManager.swift            # Swift bridge to GoCore
│   ├── Info.plist
│   └── SnowdenSystem.entitlements
├── SnowdenSystemVPN/
│   ├── PacketTunnelProvider.swift  # iOS Network Extension
│   ├── Info.plist
│   └── SnowdenSystemVPN.entitlements
├── GoCore/
│   ├── snowdencore.go              # Full Go port (Engine, Adaptive, etc.)
│   └── go.mod                      # sing-box-lx dependency
├── build/
│   └── SnowdenCore.xcframework     # gomobile output
├── project.yml                      # xcodegen config
└── build.sh                         # Build script
```

## Algorithms Ported from PC

### 1. Engine (embedded sing-box)
- `box.Box` lifecycle: Start → Running → Close
- `context.WithCancel` for graceful shutdown
- `PlatformLogWriter` for real-time log streaming
- Selective protocol registry (no naive/cronet)

### 2. Manager (facade)
- `StartVPN(configID, configJSON)`
- `StopVPN()`
- `ReloadVPN(configID, configJSON)` — atomic swap
- `Status()` → JSON for Swift UI

### 3. AdaptiveEngine (health monitoring)
- **CircuitBreaker**: 3-state (Closed → Open → HalfOpen)
- **deepProbe**: HTTP 204 through proxy every 30s
- **ErrorClassifier**: 8 categories (network_down, server_down, etc.)
- **Auto-reload**: On 3 consecutive failures
- **Callbacks**: Go → Swift for logs and diagnostics

### 4. ErrorClassifier (log parsing)
- Only `[error]` and `[warn]` levels classified
- Pattern matching: TLS, DNS, dial, connection reset
- Russian explanations for UI
- Rolling buffer of 200 events

### 5. ConfigBuilder (sing-box JSON)
- VLESS + TLS (no flow, no utls, no insecure)
- urltest group: VLESS → WARP → direct
- DNS: detour "auto" (not "proxy")
- Split-tunneling: ru-cidr.json (30K CIDRs)
- WireGuard endpoint for WARP fallback

### 6. Split-tunneling
- `ru-cidr.lst` → `ru-cidr.json` (source rule-set)
- Russian IPs → direct
- Everything else → urltest → best available

## UI Design

### Single Logo Button
- **Size**: 160×160pt circle
- **States**:
  - Disconnected: dark fill, gray border, no glow
  - Connecting: spinner overlay, pulsing opacity
  - Connected: green glow ring (animated pulse), green border
- **Press**: scale to 0.92, spring animation
- **Status dot**: bottom-right, color matches state

### Background
- Pure black (`#0a0a0f`)
- Subtle matrix particles (30 green lines, 15% opacity)
- Terminal aesthetic

### Color Palette
```
--bg-primary:    #0a0a0f
--bg-card:       #12121a
--accent-green:  #34d399   (connected, healthy)
--accent-yellow: #fbbf24   (connecting, degraded)
--accent-red:    #f87171   (error, failed)
--accent-blue:   #60a5fa   (info, upload)
--text-primary:  #e5e7eb
--text-secondary:#6b7280
--border:        #2a2a35
```

### Sheets
- **Logs**: monospaced, color-coded by level, copy button
- **Diagnostics**: status, category, counters, last error

## Build Instructions

### Prerequisites
- macOS with Xcode 15+
- Go 1.23+
- gomobile: `go install golang.org/x/mobile/cmd/gomobile@latest`
- xcodegen (optional): `brew install xcodegen`

### Build
```bash
cd ios/SnowdenSystem
./build.sh
```

### Manual Xcode Setup
1. Open `SnowdenSystem.xcodeproj`
2. Add `SnowdenCore.xcframework` to both targets (Embed & Sign)
3. Add App Group capability: `group.com.snowdensystem`
4. Add Network Extensions capability
5. Set Bundle IDs:
   - App: `com.snowdensystem.app`
   - VPN: `com.snowdensystem.app.vpn`
6. Build and run

## Capabilities Required

| Capability | Target | Purpose |
|---|---|---|
| App Groups | Both | Share data between app and extension |
| Network Extensions | Both | VPN packet tunnel |
| Personal VPN | App | VPN configuration UI |

## Known Limitations

1. **gomobile + sing-box-lx**: May require patching for iOS-specific code (gVisor, wireguard-go)
2. **TUN mode**: iOS Network Extension uses `NEPacketTunnelProvider`, not raw TUN
3. **WARP**: WireGuard endpoint works, but AWG parameters may fail on lx.2 (same as PC)
4. **Background**: iOS may suspend app; VPN extension keeps running
5. **Memory**: sing-box + gVisor may exceed 50MB memory limit for extensions

## Next Steps

1. Test gomobile build with sing-box-lx
2. Optimize memory usage for Network Extension
3. Add widget for Control Center
4. Siri Shortcuts integration
5. Push notifications for status changes

## License

Same as PC version — proprietary, snowden.system
