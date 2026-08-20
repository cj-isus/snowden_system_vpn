package core

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// VPNStatus is the JSON-friendly snapshot the UI binds to. Wails generates the
// TypeScript bindings from this struct, so keep field names stable and exported.
type VPNStatus struct {
	State     string `json:"state"`     // "stopped" | "starting" | "running" | "stopping" | "error"
	ConfigID  string `json:"configId"`  // which config is active (human label)
	Message   string `json:"message"`   // last error / status detail
	Connected bool   `json:"connected"` // convenience: true iff state == running
}

// Manager is the high-level facade the Wails layer talks to. It owns an Engine
// and keeps track of which named config is currently active, so the UI can say
// "EU-1 / AmneziaWG" rather than "config #3".
//
// It is safe for concurrent use: every public method takes the manager mutex,
// and Engine itself serialises Start/Close/Reload internally, so the UI thread
// and the adaptive engine can call into the manager without extra locking.
type Manager struct {
	mu sync.Mutex

	engine *Engine

	// activeConfigID is the label of the config the engine is running (or, when
	// stopped, the last one it ran). Empty until the first Start.
	activeConfigID string

	// activeConfigJSON keeps the last config we started, so the UI can inspect
	// servers / route rules without re-reading the file.
	activeConfigJSON []byte

	// metrics tracks live traffic counters for the TrafficCard.
	metrics *Metrics

	// domainStats remembers which outbound works best per-domain.
	domainStats *DomainStatsRegistry

	// metricsStop / metricsWG let us run exactly ONE sampling goroutine and stop
	// it cleanly, so repeated Reloads do not leak goroutines (each one would
	// otherwise double-sample traffic and double-feed domain stats).
	metricsMu     sync.Mutex
	metricsStop   chan struct{}
	metricsWG     sync.WaitGroup
	metricsActive atomic.Int32 // live sampling goroutines (must stay ≤ 1)

	// lastError carries the most recent failure message for the UI.
	lastError string
}

// NewManager wires an Engine into a fresh Manager.
func NewManager(engine *Engine) *Manager {
	return &Manager{
		engine:      engine,
		metrics:     NewMetrics(),
		domainStats: NewDomainStatsRegistry(),
	}
}

// startMetrics idempotently spawns a single sampling goroutine. Reloads reuse
// the same worker. Callers hold the manager lifecycle mutex while starting or
// stopping it so a Stop cannot race a delayed StartMetrics call.
func (m *Manager) startMetrics() {
	m.metricsMu.Lock()
	defer m.metricsMu.Unlock()

	if m.metricsStop != nil {
		return
	}

	stop := make(chan struct{})
	m.metricsStop = stop
	m.metricsWG.Add(1)
	m.metricsActive.Add(1)
	go func() {
		defer m.metricsWG.Done()
		defer m.metricsActive.Add(-1)

		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				if m.engine.Running() {
					m.metrics.sample()
					m.PollConnections()
				}
			}
		}
	}()
}

// stopMetrics stops any running sampling goroutine and waits for it to exit.
// Safe to call when metrics were never started. The lifecycle mutex prevents a
// concurrent start from adding to the WaitGroup while it is being drained.
func (m *Manager) stopMetrics() {
	m.metricsMu.Lock()
	defer m.metricsMu.Unlock()

	if m.metricsStop == nil {
		return
	}
	close(m.metricsStop)
	m.metricsStop = nil
	m.metricsWG.Wait()
}

// StartVPN starts the engine with a named config payload. configID is a label
// chosen by the caller (e.g. "eu-1-reality"); it is reported back in Status()
// and used by the adaptive engine to pick the fallback chain.
//
// On success the state becomes Running synchronously by the time this returns.
func (m *Manager) StartVPN(configID string, configJSON []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Own a private immutable snapshot. Callers (Wails/Telegram/tests) may reuse
	// or mutate their input slice after this method returns.
	snapshot := append([]byte(nil), configJSON...)
	if err := m.engine.Start(snapshot); err != nil {
		if !errors.Is(err, ErrAlreadyRunning) {
			m.lastError = err.Error()
		}
		return err
	}
	m.activeConfigID = configID
	m.activeConfigJSON = snapshot
	m.lastError = ""
	m.metrics.Start()
	m.startMetrics()
	return nil
}

// StopVPN gracefully stops the engine. Safe to call when already stopped.
func (m *Manager) StopVPN() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Keep the same lock order as StartVPN/ReloadVPN (manager → metrics). This
	// prevents a Stop that starts while StartVPN is between engine.Start and
	// startMetrics from leaving a fresh worker behind after the engine closes.
	m.stopMetrics()
	m.metrics.Stop()
	if err := m.engine.Close(); err != nil && !errors.Is(err, ErrAlreadyStopping) {
		m.lastError = err.Error()
		return err
	}
	m.lastError = ""
	return nil
}

// ReloadVPN swaps the active config for a new one without exposing a transient
// Stopped state to observers. The configID updates atomically with the swap.
func (m *Manager) ReloadVPN(configID string, configJSON []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	snapshot := append([]byte(nil), configJSON...)
	if err := m.engine.Reload(snapshot); err != nil {
		m.lastError = err.Error()
		return err
	}
	m.activeConfigID = configID
	m.activeConfigJSON = snapshot
	m.lastError = ""
	m.metrics.Start() // reset traffic timers
	m.startMetrics()
	return nil
}

// Status returns the current snapshot for the UI.
func (m *Manager) Status() VPNStatus {
	m.mu.Lock()
	defer m.mu.Unlock()

	state := m.engine.State()
	return VPNStatus{
		State:     state.String(),
		ConfigID:  m.activeConfigID,
		Message:   m.lastError,
		Connected: state == StateRunning,
	}
}

// SetLogHandler forwards to the engine. Must be called before StartVPN.
func (m *Manager) SetLogHandler(h LogHandler) {
	m.engine.SetLogHandler(h)
}

// GetServers returns the server list parsed from the active config, with live
// TCP ping to each server.
func (m *Manager) GetServers() []ServerInfo {
	m.mu.Lock()
	cfg := append([]byte(nil), m.activeConfigJSON...)
	activeID := m.activeConfigID
	m.mu.Unlock()

	servers := ParseServers(cfg)
	if len(servers) == 0 {
		return servers
	}
	// Determine which outbound is selected (urltest picks first available).
	// We mark the first server as active for display purposes.
	hasActive := false
	for i := range servers {
		if servers[i].ID == activeID || (!hasActive && strings.Contains(strings.ToLower(servers[i].Name), "vless")) {
			servers[i].Active = true
			hasActive = true
		}
	}
	if !hasActive && len(servers) > 0 {
		servers[0].Active = true
	}
	// Ping each server (TCP connect latency).
	for i := range servers {
		servers[i].Ping = PingServer(servers[i].Server, servers[i].Port)
	}
	return servers
}

// GetRouteRules returns the route rules parsed from the active config.
func (m *Manager) GetRouteRules() []RouteRuleInfo {
	m.mu.Lock()
	cfg := append([]byte(nil), m.activeConfigJSON...)
	m.mu.Unlock()
	return ParseRouteRules(cfg)
}

// GetTraffic returns live traffic counters.
func (m *Manager) GetTraffic() TrafficStats {
	return m.metrics.Stats()
}

// ActiveConfigJSON returns the last config we started (for export / inspection).
func (m *Manager) ActiveConfigJSON() []byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]byte(nil), m.activeConfigJSON...)
}

// ProbeLatency measures HTTP latency through the local proxy (if running).
func (m *Manager) ProbeLatency(proxyPort int) int {
	return ProbeLatencyThroughProxy(proxyPort)
}

// GetDomainStats returns per-domain performance for the UI card.
func (m *Manager) GetDomainStats(limit int) []DomainScore {
	if m.domainStats == nil {
		return nil
	}
	return m.domainStats.TopDomains(limit)
}

// RecordDomainStat records a per-domain request result (called from log parser).
func (m *Manager) RecordDomainStat(domain, outbound string, latencyMs int, bytes int64, success bool) {
	if m.domainStats != nil {
		m.domainStats.Record(domain, outbound, latencyMs, bytes, success)
	}
}

// PollConnections reads live connections from Clash API and feeds per-domain
// stats with REAL byte counts and REAL outbound chains. Called periodically.
func (m *Manager) PollConnections() {
	conns := ClashConnections()
	if conns == nil {
		return
	}
	for _, c := range conns {
		domain := c.Metadata.Host
		if domain == "" {
			continue // skip IP-only connections
		}
		// Determine outbound from chains (e.g. ["vless-tls"] or ["direct"])
		outbound := "auto"
		if len(c.Chains) > 0 {
			outbound = c.Chains[0]
		}
		bytes := c.Download + c.Upload
		m.domainStats.Record(domain, outbound, 0, bytes, true)
	}
}

// String is a debug helper.
func (m *Manager) String() string {
	s := m.Status()
	return fmt.Sprintf("Manager{state=%s config=%s connected=%v}", s.State, s.ConfigID, s.Connected)
}
