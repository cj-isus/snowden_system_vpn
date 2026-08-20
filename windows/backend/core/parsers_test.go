package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// ─── ParseServers ──────────────────────────────────────────────────────────

func TestParseServersExtractsRealOutbounds(t *testing.T) {
	cfg := testCfgWith(
		map[string]string{"type": "hysteria2", "tag": "hy2-nl", "server": "1.2.3.4", "server_port": "8443"},
		map[string]string{"type": "vless", "tag": "vless-fr", "server": "5.6.7.8", "server_port": "443"},
		map[string]string{"type": "direct", "tag": "direct"},
		map[string]string{"type": "block", "tag": "block"},
		map[string]string{"type": "urltest", "tag": "auto"},
	)
	servers := ParseServers(cfg)
	if len(servers) != 2 {
		t.Fatalf("ParseServers len = %d, want 2 (hy2 + vless)", len(servers))
	}

	if servers[0].ID != "hy2-nl" {
		t.Errorf("servers[0].ID = %q, want hy2-nl", servers[0].ID)
	}
	if servers[0].Protocol != "Hysteria2" {
		t.Errorf("servers[0].Protocol = %q, want Hysteria2", servers[0].Protocol)
	}
	if servers[0].Server != "1.2.3.4" {
		t.Errorf("servers[0].Server = %q, want 1.2.3.4", servers[0].Server)
	}
	if servers[0].Port != 8443 {
		t.Errorf("servers[0].Port = %d, want 8443", servers[0].Port)
	}

	if servers[1].ID != "vless-fr" {
		t.Errorf("servers[1].ID = %q, want vless-fr", servers[1].ID)
	}
	if servers[1].Protocol != "VLESS+TLS" {
		t.Errorf("servers[1].Protocol = %q, want VLESS+TLS", servers[1].Protocol)
	}
}

func TestParseServersAllSkipped(t *testing.T) {
	cfg := testCfgWith(
		map[string]string{"type": "direct", "tag": "direct"},
		map[string]string{"type": "block", "tag": "block"},
		map[string]string{"type": "urltest", "tag": "auto"},
		map[string]string{"type": "selector", "tag": "proxy"},
		map[string]string{"type": "dns", "tag": "dns"},
	)
	servers := ParseServers(cfg)
	if len(servers) != 0 {
		t.Errorf("ParseServers with only control outbounds len = %d, want 0", len(servers))
	}
}

func TestParseServersInvalidJSON(t *testing.T) {
	servers := ParseServers([]byte("not json"))
	if servers != nil {
		t.Errorf("ParseServers(invalid JSON) = %v, want nil", servers)
	}
}

func TestParseServersEmpty(t *testing.T) {
	servers := ParseServers([]byte(`{"outbounds":[]}`))
	if len(servers) != 0 {
		t.Errorf("ParseServers(empty outbounds) len = %d, want 0", len(servers))
	}
}

func TestParseServersPingInitiallyMinus1(t *testing.T) {
	cfg := testCfgWith(
		map[string]string{"type": "hysteria2", "tag": "hy2", "server": "1.2.3.4", "server_port": "8443"},
	)
	servers := ParseServers(cfg)
	if len(servers) != 1 {
		t.Fatalf("len = %d, want 1", len(servers))
	}
	if servers[0].Ping != -1 {
		t.Errorf("Ping = %d, want -1 (not yet pinged)", servers[0].Ping)
	}
	if servers[0].Active {
		t.Error("Active should be false initially")
	}
}

// ─── protocolFromType ──────────────────────────────────────────────────────

func TestProtocolFromTypeKnown(t *testing.T) {
	tests := map[string]string{
		"vless":       "VLESS+TLS",
		"vmess":       "VMess+TLS",
		"hysteria2":   "Hysteria2",
		"hysteria":    "Hysteria",
		"shadowsocks": "Shadowsocks",
		"trojan":      "Trojan",
		"wireguard":   "WireGuard",
		"shadowtls":   "ShadowTLS",
		"masque":      "MASQUE",
	}
	for input, want := range tests {
		if got := protocolFromType(input); got != want {
			t.Errorf("protocolFromType(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestProtocolFromTypeUnknown(t *testing.T) {
	if got := protocolFromType("mieru"); got != "mieru" {
		t.Errorf("protocolFromType(\"mieru\") = %q, want \"mieru\" (passthrough)", got)
	}
}

// ─── ParseRouteRules ───────────────────────────────────────────────────────

func TestParseRouteRulesExtractsDomainRules(t *testing.T) {
	cfg := []byte(`{
		"outbounds": [{"type":"direct","tag":"direct"}],
		"route": {
			"rules": [
				{"domain_suffix": [".ru", ".su"], "action": "direct"},
				{"domain": ["youtube.com", "googlevideo.com"], "outbound": "auto"},
				{"action": "sniff"},
				{"action": "hijack-dns"}
			]
		}
	}`)
	rules := ParseRouteRules(cfg)
	if len(rules) != 2 {
		t.Fatalf("ParseRouteRules len = %d, want 2", len(rules))
	}

	if rules[0].Route != "Напрямую" {
		t.Errorf("rules[0].Route = %q, want Напрямую", rules[0].Route)
	}
	if rules[0].Icon != "🇷🇺" {
		t.Errorf("rules[0].Icon = %q, want 🇷🇺", rules[0].Icon)
	}

	if rules[1].Route != "Через VPN" {
		t.Errorf("rules[1].Route = %q, want Через VPN", rules[1].Route)
	}
}

func TestParseRouteRulesInvalidJSONReturnsDefaults(t *testing.T) {
	rules := ParseRouteRules([]byte("not json"))
	defaults := defaultRouteRules()
	if len(rules) != len(defaults) {
		t.Errorf("ParseRouteRules(invalid) len = %d, want default len %d", len(rules), len(defaults))
	}
}

func TestParseRouteRulesEmptyReturnsDefaults(t *testing.T) {
	rules := ParseRouteRules([]byte(`{"outbounds":[],"route":{}}`))
	defaults := defaultRouteRules()
	if len(rules) != len(defaults) {
		t.Errorf("ParseRouteRules(empty route) len = %d, want default len %d", len(rules), len(defaults))
	}
}

// ─── routeLabel ────────────────────────────────────────────────────────────

func TestRouteLabel(t *testing.T) {
	tests := []struct {
		action, outbound, want string
	}{
		{"direct", "", "Напрямую"},
		{"", "direct", "Напрямую"},
		{"", "auto", "Через VPN"},
		{"", "vless-fr", "Через VPN"},
	}
	for _, tt := range tests {
		if got := routeLabel(tt.action, tt.outbound); got != tt.want {
			t.Errorf("routeLabel(%q,%q) = %q, want %q", tt.action, tt.outbound, got, tt.want)
		}
	}
}

// ─── joinDomains ───────────────────────────────────────────────────────────

func TestJoinDomainsFew(t *testing.T) {
	domains := []any{".ru", ".su", ".рф"}
	got := joinDomains(domains)
	if got != ".ru, .su, .рф" {
		t.Errorf("joinDomains = %q, want '.ru, .su, .рф'", got)
	}
}

func TestJoinDomainsMany(t *testing.T) {
	domains := []any{".com", ".net", ".org", ".io", ".dev"}
	got := joinDomains(domains)
	// Summaries are sorted for deterministic UI output.
	if got != ".com, .dev, .io…" {
		t.Errorf("joinDomains = %q, want '.com, .dev, .io…'", got)
	}
}

func TestJoinDomainsSingle(t *testing.T) {
	got := joinDomains([]any{"youtube.com"})
	if got != "youtube.com" {
		t.Errorf("joinDomains(single) = %q, want 'youtube.com'", got)
	}
}

// ─── iconForDomains ────────────────────────────────────────────────────────

func TestIconForDomains(t *testing.T) {
	tests := []struct {
		suffixes []any
		want     string
	}{
		{[]any{"youtube.com", "googlevideo.com"}, "🎬"},
		{[]any{"discord.gg"}, "🎮"},
		{[]any{"openai.com"}, "🤖"},
		{[]any{"claude.ai"}, "🤖"},
		{[]any{"anthropic.com"}, "🤖"},
		{[]any{"twitter.com"}, "🐦"},
		{[]any{"x.com"}, "🐦"},
		{[]any{"t.me"}, "📱"},
		{[]any{"telegram.org"}, "📱"},
		{[]any{"example.ru"}, "🇷🇺"},
		{[]any{"netflix.com"}, "🎵"},
		{[]any{"spotify.com"}, "🎵"},
		{[]any{"random.org"}, "🌐"},
	}
	for _, tt := range tests {
		if got := iconForDomains(tt.suffixes); got != tt.want {
			t.Errorf("iconForDomains(%v) = %q, want %q", tt.suffixes, got, tt.want)
		}
	}
}

// ─── defaultRouteRules ─────────────────────────────────────────────────────

func TestDefaultRouteRulesCount(t *testing.T) {
	defaults := defaultRouteRules()
	if len(defaults) < 4 {
		t.Errorf("defaultRouteRules len = %d, want >= 4", len(defaults))
	}
	ids := make(map[string]bool)
	for _, r := range defaults {
		ids[r.ID] = true
	}
	for _, want := range []string{"ru", "tg", "yt", "ai"} {
		if !ids[want] {
			t.Errorf("defaultRouteRules missing ID %q", want)
		}
	}
}

// ─── clashTrafficDelta ─────────────────────────────────────────────────────

func TestClashTrafficDeltaFirstCall(t *testing.T) {
	clashTrafficState.Lock()
	clashTrafficState.lastDown = 0
	clashTrafficState.lastUp = 0
	clashTrafficState.Unlock()

	rx, tx := clashTrafficDelta(1000, 500)
	if rx != 0 || tx != 0 {
		t.Errorf("first call: rx=%d tx=%d, want 0,0 (no previous sample)", rx, tx)
	}
}

func TestClashTrafficDeltaNormal(t *testing.T) {
	clashTrafficState.Lock()
	clashTrafficState.lastDown = 1000
	clashTrafficState.lastUp = 500
	clashTrafficState.Unlock()

	rx, tx := clashTrafficDelta(1500, 800)
	if rx != 500 {
		t.Errorf("rx = %d, want 500", rx)
	}
	if tx != 300 {
		t.Errorf("tx = %d, want 300", tx)
	}
}

func TestClashTrafficDeltaCounterReset(t *testing.T) {
	clashTrafficState.Lock()
	clashTrafficState.lastDown = 5000
	clashTrafficState.lastUp = 3000
	clashTrafficState.Unlock()

	rx, tx := clashTrafficDelta(100, 50)
	if rx != 100 {
		t.Errorf("rx = %d after counter reset, want 100", rx)
	}
	if tx != 50 {
		t.Errorf("tx = %d after counter reset, want 50", tx)
	}
}

// ─── Metrics lifecycle ─────────────────────────────────────────────────────

func TestMetricsStartResetCounters(t *testing.T) {
	m := NewMetrics()
	m.Start()
	stats := m.Stats()
	if stats.DownloadTotal != 0 || stats.UploadTotal != 0 {
		t.Errorf("after Start: dl=%d ul=%d, want 0,0", stats.DownloadTotal, stats.UploadTotal)
	}
	if stats.Uptime < 0 {
		t.Errorf("Uptime = %d, want >= 0", stats.Uptime)
	}
}

func TestMetricsStatsUptimeGrows(t *testing.T) {
	m := NewMetrics()
	m.Start()
	// Sleep long enough that int64 truncation yields at least 1 second.
	time.Sleep(1100 * time.Millisecond)
	stats := m.Stats()
	if stats.Uptime < 1 {
		t.Errorf("Uptime = %d after 1.1s, want >= 1", stats.Uptime)
	}
}

func TestMetricsAddRxTx(t *testing.T) {
	m := NewMetrics()
	m.AddRx(100)
	m.AddTx(200)
	// Verify they don't panic.
}

func TestMetricsStopIdempotent(t *testing.T) {
	m := NewMetrics()
	m.Stop()
	m.Stop() // second call should not panic
}

// ─── ExportConfig / ImportConfig ───────────────────────────────────────────

func TestExportImportConfigRoundTrip(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "test-config.json")
	destPath := filepath.Join(dir, "exported-config.json")

	cfg := []byte(`{"outbounds":[{"type":"direct","tag":"direct"}],"route":{"final":"direct"}}`)
	if err := os.WriteFile(srcPath, cfg, 0644); err != nil {
		t.Fatal(err)
	}

	data, err := ImportConfig(srcPath)
	if err != nil {
		t.Fatalf("ImportConfig: %v", err)
	}
	if string(data) != string(cfg) {
		t.Errorf("ImportConfig returned %s, want %s", data, cfg)
	}

	if err := ExportConfig(data, destPath); err != nil {
		t.Fatalf("ExportConfig: %v", err)
	}

	written, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(written, &parsed); err != nil {
		t.Fatalf("exported JSON invalid: %v", err)
	}
}

func TestImportConfigMissingFile(t *testing.T) {
	_, err := ImportConfig(filepath.Join(t.TempDir(), "nonexistent.json"))
	if err == nil {
		t.Error("ImportConfig(missing) should return error")
	}
}
