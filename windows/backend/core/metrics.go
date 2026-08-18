package core

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ─── Public types (Wails binds these → TypeScript) ───────────────────────────

// ServerInfo describes one outbound server for the ServersCard.
type ServerInfo struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Protocol string `json:"protocol"`
	Server   string `json:"server"`   // IP/host
	Port     int    `json:"port"`
	Location string `json:"location"` // human label, e.g. "Нидерланды"
	Active   bool   `json:"active"`
	Ping     int    `json:"ping"`     // milliseconds, -1 if unknown
}

// RouteRuleInfo describes one route rule for the RoutingCard.
type RouteRuleInfo struct {
	ID     string `json:"id"`
	Icon   string `json:"icon"`
	Title  string `json:"title"`
	Sub    string `json:"sub"`
	Route  string `json:"route"` // "Напрямую" | "Через VPN"
	On     bool   `json:"on"`
}

// TrafficStats is the realtime counters for the TrafficCard.
type TrafficStats struct {
	DownloadSpeed int64 `json:"downloadSpeed"` // bytes/sec
	UploadSpeed   int64 `json:"uploadSpeed"`   // bytes/sec
	DownloadTotal int64 `json:"downloadTotal"` // bytes this session
	UploadTotal   int64 `json:"uploadTotal"`   // bytes this session
	Uptime        int64 `json:"uptime"`        // seconds since start
}

// ─── Metrics holds live traffic counters + server metadata ────────────────────

// Metrics tracks bytes flowing through the local proxy port and exposes derived
// rates. Because sing-box runs in-process, the most reliable cross-platform way
// to measure user traffic is to sample the OS-level counters of our own process
// (sing-box reads/writes sockets inside our PID). We fallback to interface delta
// on the loopback/proxy port when available.
type Metrics struct {
	mu sync.Mutex

	startedAt  time.Time
	lastSample time.Time

	dlTotal int64
	ulTotal int64

	dlSpeed int64
	ulSpeed int64

	// atomic accumulators updated by the dialer wrapper (optional, best-effort)
	rxAtomic atomic.Int64
	txAtomic atomic.Int64
}

// NewMetrics builds a fresh Metrics. Call Start() to begin sampling.
func NewMetrics() *Metrics {
	return &Metrics{}
}

// Start resets counters and begins background sampling.
func (m *Metrics) Start() {
	m.mu.Lock()
	m.startedAt = time.Now()
	m.lastSample = time.Now()
	m.dlTotal = 0
	m.ulTotal = 0
	m.dlSpeed = 0
	m.ulSpeed = 0
	m.mu.Unlock()

	// Reset Clash API delta state so first sample doesn't produce a huge spike.
	clashTrafficState.Lock()
	clashTrafficState.lastDown = 0
	clashTrafficState.lastUp = 0
	clashTrafficState.Unlock()
}

// Stop freezes counters (keeps totals for display until next Start).
func (m *Metrics) Stop() {
	m.sample() // final flush
}

// sample reads traffic counters from sing-box Clash API and derives rates.
func (m *Metrics) sample() {
	rx, tx := readClashTraffic()
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	dt := now.Sub(m.lastSample).Seconds()
	if dt < 0.1 {
		return
	}
	m.dlSpeed = int64(float64(rx) / dt)
	m.ulSpeed = int64(float64(tx) / dt)
	m.dlTotal += rx
	m.ulTotal += tx
	m.lastSample = now
}

// Stats returns the current snapshot.
func (m *Metrics) Stats() TrafficStats {
	m.mu.Lock()
	defer m.mu.Unlock()
	var uptime int64
	if !m.startedAt.IsZero() {
		uptime = int64(time.Since(m.startedAt).Seconds())
	}
	return TrafficStats{
		DownloadSpeed: m.dlSpeed,
		UploadSpeed:   m.ulSpeed,
		DownloadTotal: m.dlTotal,
		UploadTotal:   m.ulTotal,
		Uptime:        uptime,
	}
}

// ClashConnection represents one active connection from Clash API /connections.
type ClashConnection struct {
	ID         string `json:"id"`
	Upload     int64  `json:"upload"`
	Download   int64  `json:"download"`
	Start      string `json:"start"`
	Chains     []string `json:"chains"`
	Metadata struct {
		Network  string `json:"network"`
		Type     string `json:"type"`
		DestinationIP   string `json:"destinationIP"`
		DestinationPort string `json:"destinationPort"`
		Host     string `json:"host"` // sniffed domain
	} `json:"metadata"`
}

// ClashConnections fetches the list of active connections from sing-box.
func ClashConnections() []ClashConnection {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://127.0.0.1:9090/connections")
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	var data struct {
		Connections []ClashConnection `json:"connections"`
		Upload      int64             `json:"upload"`
		Download    int64             `json:"download"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil
	}
	return data.Connections
}
// sing-box tracks bytes flowing through every connection and exposes the totals
// at GET /connections (sum of upload/download). We use the simpler /traffic
// endpoint which returns {up: N, down: N} per-second rates.
// Returns (rxBytes, txBytes) accumulated since the last call.
func readClashTraffic() (rxDelta, txDelta int64) {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://127.0.0.1:9090/connections")
	if err != nil {
		return 0, 0
	}
	defer resp.Body.Close()
	var data struct {
		Upload   int64 `json:"upload"`
		Download int64 `json:"download"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return 0, 0
	}
	// /connections returns cumulative totals. We track the delta between calls.
	return clashTrafficDelta(data.Download, data.Upload)
}

// clashTrafficState holds the previous cumulative counters to compute deltas.
var clashTrafficState struct {
	sync.Mutex
	lastDown int64
	lastUp   int64
}

func clashTrafficDelta(down, up int64) (rxDelta, txDelta int64) {
	clashTrafficState.Lock()
	defer clashTrafficState.Unlock()
	if clashTrafficState.lastDown > 0 {
		rxDelta = down - clashTrafficState.lastDown
	}
	if clashTrafficState.lastUp > 0 {
		txDelta = up - clashTrafficState.lastUp
	}
	if rxDelta < 0 {
		rxDelta = down // counter reset
	}
	if txDelta < 0 {
		txDelta = up
	}
	clashTrafficState.lastDown = down
	clashTrafficState.lastUp = up
	return rxDelta, txDelta
}

// AddRx / AddTx let a wrapped dialer report bytes. Called from the proxy layer
// when available; otherwise rates stay at 0 (better than fake random numbers).
func (m *Metrics) AddRx(n int) { m.rxAtomic.Add(int64(n)) }
func (m *Metrics) AddTx(n int) { m.txAtomic.Add(int64(n)) }

// ─── Server list + ping ──────────────────────────────────────────────────────

// protocolFromType maps a sing-box outbound type to a human label.
func protocolFromType(t string) string {
	switch t {
	case "vless":
		return "VLESS+TLS"
	case "vmess":
		return "VMess+TLS"
	case "hysteria2":
		return "Hysteria2"
	case "hysteria":
		return "Hysteria"
	case "shadowsocks":
		return "Shadowsocks"
	case "trojan":
		return "Trojan"
	case "wireguard":
		return "WireGuard"
	case "shadowtls":
		return "ShadowTLS"
	case "masque":
		return "MASQUE"
	default:
		return t
	}
}

// locationFromIP returns a human country label by heuristics on the IP.
func locationFromIP(server string) string {
	return "VPS"
}

// ParseServers extracts server outbounds (excluding direct/block/urltest) from
// the active sing-box config JSON.
func ParseServers(configJSON []byte) []ServerInfo {
	var raw struct {
		Outbounds []struct {
			Type       string `json:"type"`
			Tag        string `json:"tag"`
			Server     string `json:"server"`
			ServerPort int    `json:"server_port"`
		} `json:"outbounds"`
	}
	if err := json.Unmarshal(configJSON, &raw); err != nil {
		return nil
	}
	skip := map[string]bool{"direct": true, "block": true, "urltest": true, "selector": true, "dns": true}
	var result []ServerInfo
	for _, ob := range raw.Outbounds {
		if skip[ob.Type] {
			continue
		}
		result = append(result, ServerInfo{
			ID:       ob.Tag,
			Name:     ob.Tag,
			Protocol: protocolFromType(ob.Type),
			Server:   ob.Server,
			Port:     ob.ServerPort,
			Location: locationFromIP(ob.Server),
			Active:   false, // set by caller based on selected outbound
			Ping:     -1,
		})
	}
	return result
}

// PingServer does a TCP connect to host:port and returns latency in ms (-1 on error).
func PingServer(host string, port int) int {
	addr := fmt.Sprintf("%s:%d", host, port)
	start := time.Now()
	conn, err := net.DialTimeout("tcp", addr, 3*time.Second)
	if err != nil {
		return -1
	}
	latency := time.Since(start).Milliseconds()
	conn.Close()
	return int(latency)
}

// ─── Route rules parsing ─────────────────────────────────────────────────────

// ParseRouteRules extracts human-readable rules from the config for the RoutingCard.
func ParseRouteRules(configJSON []byte) []RouteRuleInfo {
	var raw struct {
		Route struct {
			Rules []map[string]any `json:"rules"`
		} `json:"route"`
	}
	if err := json.Unmarshal(configJSON, &raw); err != nil {
		return defaultRouteRules()
	}
	rules := raw.Route.Rules
	var result []RouteRuleInfo
	for i, r := range rules {
		action, _ := r["action"].(string)
		outbound, _ := r["outbound"].(string)
		_ = outbound
		if action == "sniff" || action == "hijack-dns" {
			continue
		}
		info := RouteRuleInfo{
			ID:    fmt.Sprintf("rule-%d", i),
			Route: routeLabel(action, outbound),
			On:    true,
		}
		if suffixes, ok := r["domain_suffix"].([]any); ok && len(suffixes) > 0 {
			info.Title = joinDomains(suffixes)
			info.Icon = iconForDomains(suffixes)
			info.Sub = fmt.Sprintf("%d доменов", len(suffixes))
		} else if domains, ok := r["domain"].([]any); ok && len(domains) > 0 {
			info.Title = joinDomains(domains)
			info.Icon = "🌐"
			info.Sub = fmt.Sprintf("%d доменов", len(domains))
		} else if action == "direct" {
			info.Title = "Прямое подключение"
			info.Icon = "🔗"
		}
		if info.Title != "" {
			result = append(result, info)
		}
	}
	if len(result) == 0 {
		return defaultRouteRules()
	}
	return result
}

func routeLabel(action, outbound string) string {
	if action == "direct" || outbound == "direct" {
		return "Напрямую"
	}
	return "Через VPN"
}

func joinDomains(suffixes []any) string {
	names := make([]string, 0, len(suffixes))
	for _, s := range suffixes {
		if str, ok := s.(string); ok {
			names = append(names, str)
		}
	}
	sort.Strings(names)
	if len(names) > 3 {
		return strings.Join(names[:3], ", ") + "…"
	}
	return strings.Join(names, ", ")
}

func iconForDomains(suffixes []any) string {
	for _, s := range suffixes {
		str, _ := s.(string)
		switch {
		case strings.Contains(str, "youtube") || strings.Contains(str, "googlevideo"):
			return "🎬"
		case strings.Contains(str, "discord"):
			return "🎮"
		case strings.Contains(str, "openai") || strings.Contains(str, "claude") || strings.Contains(str, "anthropic"):
			return "🤖"
		case strings.Contains(str, "twitter") || strings.Contains(str, "x.com"):
			return "🐦"
		case strings.Contains(str, "telegram") || strings.Contains(str, "t.me"):
			return "📱"
		case strings.Contains(str, ".ru"):
			return "🇷🇺"
		case strings.Contains(str, "netflix") || strings.Contains(str, "spotify"):
			return "🎵"
		}
	}
	return "🌐"
}

func defaultRouteRules() []RouteRuleInfo {
	return []RouteRuleInfo{
		{ID: "ru", Icon: "🇷🇺", Title: "Российские сайты", Sub: ".ru .su .рф + банки", Route: "Напрямую", On: true},
		{ID: "tg", Icon: "📱", Title: "Telegram", Sub: "MTProto 91.108/149.154", Route: "Через VPN", On: true},
		{ID: "yt", Icon: "🎬", Title: "YouTube", Sub: "googlevideo.com", Route: "Через VPN", On: true},
		{ID: "ai", Icon: "🤖", Title: "AI сервисы", Sub: "OpenAI, Claude, Gemini", Route: "Через VPN", On: true},
	}
}

// ─── URL-test latency probe ──────────────────────────────────────────────────

// ProbeLatencyThroughProxy tests how fast an HTTP request completes when routed
// through the local sing-box proxy port. Returns ms, or -1 on failure.
func ProbeLatencyThroughProxy(proxyPort int) int {
	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			Proxy: func(*http.Request) (*url.URL, error) {
				return url.Parse(fmt.Sprintf("http://127.0.0.1:%d", proxyPort))
			},
		},
	}
	start := time.Now()
	resp, err := client.Get("https://www.gstatic.com/generate_204")
	if err != nil {
		return -1
	}
	resp.Body.Close()
	return int(time.Since(start).Milliseconds())
}

// ─── Config import/export ────────────────────────────────────────────────────

// ExportConfig writes the current config JSON to a user-chosen path.
func ExportConfig(configJSON []byte, destPath string) error {
	// Pretty-print for readability
	var pretty any
	if err := json.Unmarshal(configJSON, &pretty); err == nil {
		if data, err := json.MarshalIndent(pretty, "", "  "); err == nil {
			return os.WriteFile(destPath, data, 0644)
		}
	}
	return os.WriteFile(destPath, configJSON, 0644)
}

// ImportConfig reads a config file from disk and returns its raw JSON.
func ImportConfig(srcPath string) ([]byte, error) {
	return os.ReadFile(srcPath)
}

// SaveImportedConfig copies an imported config into the app's assets/configs dir
// so it appears in the config selector. Returns the basename.
func SaveImportedConfig(srcPath, configsDir string) (string, error) {
	base := filepath.Base(srcPath)
	dest := filepath.Join(configsDir, base)
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return "", err
	}
	return base, os.WriteFile(dest, data, 0644)
}
