package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"snowden-system/backend/cfclient"
	"snowden-system/backend/core"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App is the struct Wails binds to the frontend.
type App struct {
	ctx      context.Context
	manager  *core.Manager
	adaptive *core.AdaptiveEngine
	tray     *trayManager
	tgLogger *TelegramLogger
	cfClient *cfclient.Client
}

// loadEnvFile reads .env from the exe directory or working directory and
// injects key=value pairs into the process environment.
func loadEnvFile() {
	candidates := []string{}
	if exePath, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exePath), ".env"))
	}
	if cwd, err := os.Getwd(); err == nil {
		candidates = append(candidates, filepath.Join(cwd, ".env"))
	}
	for _, p := range candidates {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				val := strings.TrimSpace(parts[1])
				// Remove surrounding quotes if present
				if (strings.HasPrefix(val, "\"") && strings.HasSuffix(val, "\"")) ||
					(strings.HasPrefix(val, "'") && strings.HasSuffix(val, "'")) {
					val = val[1 : len(val)-1]
				}
				os.Setenv(key, val)
				log.Printf("[env] loaded %s from %s", key, p)
			}
		}
		break // loaded first found
	}
}

// Telegram bot credentials — loaded from environment or config file.
// DO NOT hardcode tokens in source code.
// Set SNOWDEN_TG_TOKEN and SNOWDEN_TG_CHAT_ID env vars, or create .env file.
func getTgToken() string {
	if t := os.Getenv("SNOWDEN_TG_TOKEN"); t != "" {
		return t
	}
	return ""
}

func getTgChatID() string {
	if c := os.Getenv("SNOWDEN_TG_CHAT_ID"); c != "" {
		return c
	}
	return ""
}

// NewApp builds the App with a fresh Engine + Manager + AdaptiveEngine + Tray.
func NewApp() *App {
	loadEnvFile() // load .env before anything else

	engine := core.NewEngine()
	adaptive := core.NewAdaptiveEngine(engine)
	engine.SetClassifier(adaptive.Classifier())
	manager := core.NewManager(engine)
	// Adaptive recovery must go through Manager so engine reloads also update
	// metrics and the active-config snapshot. It must not bypass lifecycle with
	// a direct Engine.Reload call.
	adaptive.SetRecoveryFunc(manager.ReloadVPN)
	app := &App{manager: manager, adaptive: adaptive, cfClient: cfclient.New()}
	app.tray = newTrayManager(app)
	if token := getTgToken(); token != "" {
		app.tgLogger = NewTelegramLogger(token, getTgChatID(), engine, adaptive)
		app.tgLogger.SetManager(manager)
	}
	return app
}

// startup is called on the main thread when Wails is ready.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// Open a log file next to the exe for diagnosing GUI runtime errors.
	if exePath, err := os.Executable(); err == nil {
		if logFile, err := os.OpenFile(
			filepath.Join(filepath.Dir(exePath), "snowden-system.log"),
			os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644); err == nil {
			log.SetOutput(logFile)
			log.Printf("[startup] log file opened")
		}
	}

	// Bridge sing-box logs → frontend "engine:log" event + Telegram logger + domain stats.
	a.manager.SetLogHandler(logEmitter{ctx: ctx, tgLogger: a.tgLogger, manager: a.manager})

	// Wire the adaptive engine events to the frontend.
	a.adaptive.SetWailsContext(ctx)

	// Start Telegram logger (reports every 15 min + on critical errors).
	if a.tgLogger != nil {
		a.tgLogger.Start(ctx)
	}

	// Start system tray.
	if a.tray != nil {
		a.tray.Start(ctx)
	}
}

// logEmitter forwards sing-box log lines to the frontend via a Wails event,
// to the Telegram logger for remote monitoring, and extracts domain info for
// the per-domain stats registry.
type logEmitter struct {
	ctx      context.Context
	tgLogger *TelegramLogger
	manager  *core.Manager
}

func (l logEmitter) OnLog(line string) {
	// Write to Go log file for debugging
	log.Printf("[sing-box] %s", line)
	// Emit to frontend TerminalBar
	if l.ctx != nil {
		runtime.EventsEmit(l.ctx, "engine:log", line)
	}
	if l.tgLogger != nil {
		l.tgLogger.PushLog(line)
	}
	// Extract domain from sniffed protocol logs for per-domain stats.
	// sing-box logs: "sniffed protocol: tls, domain: youtube.com"
	if l.manager != nil {
		if domain, ok := extractDomainFromLog(line); ok {
			// Determine which outbound is active (auto → currently vless or hysteria2)
			l.manager.RecordDomainStat(domain, "auto", 0, 0, !containsStr(line, "[error]"))
		}
	}
}

// extractDomainFromLog parses "domain: xxx" from sing-box sniff logs.
func extractDomainFromLog(line string) (string, bool) {
	idx := indexOfStr(line, "domain: ")
	if idx < 0 {
		return "", false
	}
	start := idx + len("domain: ")
	rest := line[start:]
	// domain ends at space, quote, or end of line
	end := len(rest)
	for i, c := range rest {
		if c == ' ' || c == '"' || c == ']' || c == ',' {
			end = i
			break
		}
	}
	domain := rest[:end]
	if len(domain) > 3 && domain != "domain:" {
		return domain, true
	}
	return "", false
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && indexOfStr(s, sub) >= 0
}

func indexOfStr(s, sub string) int {
	if len(sub) == 0 {
		return 0
	}
	for i := 0; i <= len(s)-len(sub); i++ {
		match := true
		for j := 0; j < len(sub); j++ {
			if s[i+j] != sub[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

// ─── Wails-exposed methods (the frontend API) ───

// StartVPN launches the engine with the named config.
func (a *App) StartVPN(configID string, configJSON string) error {
	log.Printf("[StartVPN] configID=%s bytes=%d", configID, len(configJSON))

	err := a.manager.StartVPN(configID, []byte(configJSON))
	if err != nil {
		log.Printf("[StartVPN] FAILED: %v", err)
		return fmt.Errorf("%w", err)
	}
	st := a.manager.Status()
	log.Printf("[StartVPN] OK state=%s connected=%v", st.State, st.Connected)

	if !st.Connected {
		log.Printf("[StartVPN] engine state=%s, not Running — aborting", st.State)
		return fmt.Errorf("engine start failed: state=%s", st.State)
	}

	if perr := setSystemProxy("127.0.0.1:20808"); perr != nil {
		log.Printf("[StartVPN] system proxy WARN: %v", perr)
	} else {
		log.Printf("[StartVPN] system proxy → 127.0.0.1:20808")
	}
	a.adaptive.Start(configID, []byte(configJSON))
	return nil
}

// StopVPN gracefully stops the engine and clears the system proxy.
func (a *App) StopVPN() error {
	a.adaptive.Stop()
	if perr := clearSystemProxy(); perr != nil {
		log.Printf("[StopVPN] clear proxy WARN: %v", perr)
	} else {
		log.Printf("[StopVPN] system proxy cleared")
	}
	return a.manager.StopVPN()
}

// ReloadVPN swaps the active config without a transient stop.
func (a *App) ReloadVPN(configID string, configJSON string) error {
	config := []byte(configJSON)
	if err := a.manager.ReloadVPN(configID, config); err != nil {
		return err
	}
	// Keep adaptive recovery on the same snapshot after a UI/Telegram reload.
	a.adaptive.UpdateConfig(configID, config)
	return nil
}

// Status returns the current VPN state snapshot.
func (a *App) Status() core.VPNStatus {
	return a.manager.Status()
}

// Diagnostics returns the current diagnostic state.
func (a *App) Diagnostics() core.DiagStatus {
	return a.adaptive.Diagnostics()
}

// ─── Real data for dashboard cards ───

// GetServers returns the server list with live TCP pings.
func (a *App) GetServers() []core.ServerInfo {
	return a.manager.GetServers()
}

// GetRouteRules returns the route rules from the active config.
func (a *App) GetRouteRules() []core.RouteRuleInfo {
	return a.manager.GetRouteRules()
}

// GetTraffic returns live traffic counters (bytes/sec + totals + uptime).
func (a *App) GetTraffic() core.TrafficStats {
	return a.manager.GetTraffic()
}

// GetLatency measures end-to-end HTTP latency through the VPN tunnel.
func (a *App) GetLatency() int {
	return a.manager.ProbeLatency(20808)
}

// GetDomainStats returns per-domain performance data (best outbound per domain).
func (a *App) GetDomainStats() []core.DomainScore {
	return a.manager.GetDomainStats(20)
}

// GetRemoteHealth checks VPS health via Cloudflare Worker edge (bypasses local
// network — tests from CF's perspective, not yours).
func (a *App) GetRemoteHealth() (map[string]any, error) {
	h, err := a.cfClient.FetchHealth()
	if err != nil {
		return nil, err
	}
	result := map[string]any{
		"edge":      h.Edge,
		"timestamp": h.Timestamp,
		"tests":     h.Tests,
	}
	return result, nil
}

// GetRemoteVersion checks for app updates via the landing page version.json.
// This is the auto-update endpoint that the SettingsCard polls.
func (a *App) GetRemoteVersion() (map[string]string, error) {
	// Try the landing page first (always up-to-date)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get("https://main.snowden-system.pages.dev/version.json")
	if err == nil {
		defer resp.Body.Close()
		var v struct {
			Version   string `json:"version"`
			PcURL     string `json:"pc_url"`
			Changelog string `json:"changelog"`
		}
		if json.NewDecoder(resp.Body).Decode(&v) == nil && v.Version != "" {
			return map[string]string{
				"version":     v.Version,
				"downloadUrl": v.PcURL,
				"changelog":   v.Changelog,
			}, nil
		}
	}
	// Fallback to CF Worker
	v, err := a.cfClient.FetchVersion()
	if err != nil {
		return nil, err
	}
	return map[string]string{
		"version":     v.Version,
		"downloadUrl": v.DownloadURL,
	}, nil
}

// CheckForUpdate compares local version with remote and returns update info.
// Called by the frontend periodically.
func (a *App) CheckForUpdate() (map[string]any, error) {
	const LOCAL_VERSION = "1.3.5"
	remote, err := a.GetRemoteVersion()
	if err != nil {
		return nil, err
	}
	remoteVer := remote["version"]
	hasUpdate := remoteVer != "" && remoteVer != LOCAL_VERSION
	return map[string]any{
		"hasUpdate":    hasUpdate,
		"localVersion":  LOCAL_VERSION,
		"remoteVersion": remoteVer,
		"downloadUrl":   remote["downloadUrl"],
		"changelog":     remote["changelog"],
	}, nil
}

// SelectServer changes the active server. "auto" = urltest (default),
// "nl" = Нидерланды only, "fr" = Франция only.
// Rebuilds the route.final and reloads sing-box.
func (a *App) SelectServer(server string) error {
	cfg := a.manager.ActiveConfigJSON()
	if len(cfg) == 0 {
		return fmt.Errorf("no active config")
	}

	var config map[string]any
	if err := json.Unmarshal(cfg, &config); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}

	route, ok := config["route"].(map[string]any)
	if !ok {
		return fmt.Errorf("no route section")
	}

	// Map friendly name to outbound tag by resolving against the ACTUAL
	// outbounds in config (the config is the source of truth, not hardcoded
	// names). "auto" → urltest group; "nl"/"fr" → tag containing the code.
	finalOutbound, err := core.ResolveOutboundTag(server, cfg)
	if err != nil {
		return fmt.Errorf("resolve server %q: %w", server, err)
	}

	route["final"] = finalOutbound
	config["route"] = route

	newCfg, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	log.Printf("[SelectServer] → %s (final=%s), reloading", server, finalOutbound)
	return a.ReloadVPN(a.manager.Status().ConfigID, string(newCfg))
}

// ToggleRouteRule enables/disables a route rule by index and reloads the config.
// The frontend passes the rule index (0-based, matching the order in route.rules)
// and the new enabled state. We rebuild the config JSON, swapping the rule's
// action between "direct" and "auto" (VPN), then hot-reload sing-box.
func (a *App) ToggleRouteRule(ruleIndex int, enabled bool) error {
	cfg := a.manager.ActiveConfigJSON()
	if len(cfg) == 0 {
		return fmt.Errorf("no active config")
	}

	var config map[string]any
	if err := json.Unmarshal(cfg, &config); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}

	route, ok := config["route"].(map[string]any)
	if !ok {
		return fmt.Errorf("no route section")
	}
	rules, ok := route["rules"].([]any)
	if !ok || ruleIndex < 0 || ruleIndex >= len(rules) {
		return fmt.Errorf("invalid rule index %d", ruleIndex)
	}

	rule, ok := rules[ruleIndex].(map[string]any)
	if !ok {
		return fmt.Errorf("rule %d is not an object", ruleIndex)
	}

	// Toggle: if enabling → route through VPN (auto), if disabling → direct
	if enabled {
		// Remove "action":"direct" and set "outbound":"auto"
		delete(rule, "action")
		rule["outbound"] = "auto"
	} else {
		// Remove "outbound" and set "action":"direct"
		delete(rule, "outbound")
		rule["action"] = "direct"
	}

	rules[ruleIndex] = rule
	route["rules"] = rules
	config["route"] = route

	newCfg, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	log.Printf("[ToggleRouteRule] rule %d → enabled=%v, reloading", ruleIndex, enabled)
	return a.ReloadVPN(a.manager.Status().ConfigID, string(newCfg))
}

// ExportConfig saves the active config to a file path (Wails frontend dialog
// picks the path). Returns the written path on success.
func (a *App) ExportConfig(destPath string) (string, error) {
	cfg := a.manager.ActiveConfigJSON()
	if len(cfg) == 0 {
		// Fallback: load the default template
		c, err := a.LoadConfigFile("template-vps-reality.json")
		if err != nil {
			return "", err
		}
		cfg = []byte(c)
	}
	if err := core.ExportConfig(cfg, destPath); err != nil {
		return "", err
	}
	log.Printf("[ExportConfig] written to %s", destPath)
	return destPath, nil
}

// ImportConfig reads a config file from disk and validates it. Returns raw JSON.
func (a *App) ImportConfig(srcPath string) (string, error) {
	data, err := core.ImportConfig(srcPath)
	if err != nil {
		return "", err
	}
	log.Printf("[ImportConfig] loaded %d bytes from %s", len(data), srcPath)
	return string(data), nil
}

// ListConfigs returns the names of all config templates in assets/configs/.
func (a *App) ListConfigs() []string {
	var configs []string
	dir := a.configsDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return configs
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			configs = append(configs, e.Name())
		}
	}
	return configs
}

// configsDir returns the path to assets/configs next to the executable.
func (a *App) configsDir() string {
	if exePath, err := os.Executable(); err == nil {
		return filepath.Join(filepath.Dir(exePath), "assets", "configs")
	}
	return "assets/configs"
}

// ─── External apps (Amnezia / Karing) ───

// OpenExternalApp launches an installed VPN client by name.
// "amnezia" → Amnezia VPN, "karing" → Karing (Mieru).
// On Windows it tries Start Menu shortcuts first, then common install dirs.
// Returns the resolved executable path, or an error if not installed.
func (a *App) OpenExternalApp(name string) (string, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	candidates := [][]string{}

	switch name {
	case "amnezia":
		// Amnezia VPN for Windows installs to %LOCALAPPDATA%\Programs\AmneziaVPN
		candidates = [][]string{
			{os.Getenv("LOCALAPPDATA"), "Programs", "AmneziaVPN", "AmneziaVPN.exe"},
			{os.Getenv("PROGRAMFILES"), "AmneziaVPN", "AmneziaVPN.exe"},
			{os.Getenv("ProgramFiles(x86)"), "AmneziaVPN", "AmneziaVPN.exe"},
		}
	case "karing":
		// Karing for Windows — portable or installer builds
		candidates = [][]string{
			{os.Getenv("LOCALAPPDATA"), "Programs", "karing", "karing.exe"},
			{os.Getenv("LOCALAPPDATA"), "Programs", "Karing", "karing.exe"},
			{os.Getenv("PROGRAMFILES"), "karing", "karing.exe"},
			{os.Getenv("ProgramFiles(x86)"), "karing", "karing.exe"},
		}
	default:
		return "", fmt.Errorf("неизвестное приложение: %s", name)
	}

	for _, parts := range candidates {
		exePath := filepath.Join(parts...)
		if _, err := os.Stat(exePath); err == nil {
			cmd := exec.Command(exePath)
			cmd.Dir = filepath.Dir(exePath)
			if err := cmd.Start(); err != nil {
				return exePath, fmt.Errorf("запуск %s: %w", name, err)
			}
			log.Printf("[external] launched %s: %s (pid=%d)", name, exePath, cmd.Process.Pid)
			return exePath, nil
		}
	}
	return "", fmt.Errorf("%s не найден — установите приложение с официального сайта", name)
}

// ─── Autostart ───

func (a *App) SetAutostart(enabled bool) error {
	return setAutostartRegistry(enabled)
}

func (a *App) IsAutostartEnabled() bool {
	return isAutostartEnabled()
}

// ─── Config loading + split-tunnel injection ───

// LoadConfigFile reads a template config, injects split-tunneling (RU CIDR),
// and returns the enriched JSON string.
func (a *App) LoadConfigFile(name string) (string, error) {
	candidates := make([]string, 0, 3)
	if exePath, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exePath), "assets", "configs", name))
	}
	if cwd, err := os.Getwd(); err == nil {
		candidates = append(candidates, filepath.Join(cwd, "assets", "configs", name))
	}
	log.Printf("[LoadConfigFile] looking for %s in %d paths", name, len(candidates))

	var raw []byte
	for _, p := range candidates {
		if data, err := os.ReadFile(p); err == nil {
			log.Printf("[LoadConfigFile] loaded %d bytes from %s", len(data), p)
			raw = data
			break
		} else {
			log.Printf("[LoadConfigFile] miss: %s → %v", p, err)
		}
	}
	if raw == nil {
		return "", fmt.Errorf("config not found in any of: %v", candidates)
	}

	// Split-tunnel: domain rules (.ru/.su/.рф + банки) достаточно.
	// RU CIDR (11401 правил) замедляет route matching → убрано.
	log.Printf("[LoadConfigFile] config loaded (%d bytes, domain split-tunnel only)", len(raw))
	return string(raw), nil
}
