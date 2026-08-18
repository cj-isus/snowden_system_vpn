// Package snowdencore exports Go functions callable from Swift via gomobile.
// This is a full port of the PC backend: Engine, Manager, AdaptiveEngine,
// CircuitBreaker, ErrorClassifier, split-tunneling, VLESS+Hysteria2+urltest.
//
// Build: gomobile bind -target ios -o SnowdenCore.xcframework .
package snowdencore

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	box "github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/json"
	E "github.com/sagernet/sing/common/exceptions"
)

// ============================================================================
// Engine (embedded sing-box, same as PC)
// ============================================================================

type EngineState int32

const (
	StateStopped EngineState = iota
	StateStarting
	StateRunning
	StateStopping
	StateError
)

func (s EngineState) String() string {
	switch s {
	case StateStopped:
		return "stopped"
	case StateStarting:
		return "starting"
	case StateRunning:
		return "running"
	case StateStopping:
		return "stopping"
	case StateError:
		return "error"
	default:
		return "unknown"
	}
}

// LogHandler receives sing-box log lines in real time.
type LogHandler interface {
	OnLog(line string)
}

// Engine embeds sing-box inside the same process (no subprocess).
type Engine struct {
	mu            sync.Mutex
	state         atomic.Int32
	currentCtx    context.Context
	currentCancel context.CancelFunc
	currentBox    *box.Box
	logHandler    LogHandler
	classifier    *ErrorClassifier
	done          chan struct{}
}

func NewEngine() *Engine {
	e := &Engine{}
	e.setState(StateStopped)
	return e
}

func (e *Engine) SetLogHandler(h LogHandler) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.State() != StateStopped {
		return
	}
	e.logHandler = h
}

func (e *Engine) SetClassifier(c *ErrorClassifier) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.classifier = c
}

func (e *Engine) State() EngineState {
	return EngineState(e.state.Load())
}

func (e *Engine) Running() bool {
	return e.State() == StateRunning
}

func (e *Engine) platformWriter() log.PlatformWriter {
	return platformLogWriter{handler: e.logHandler, classifier: e.classifier}
}

type platformLogWriter struct {
	handler    LogHandler
	classifier *ErrorClassifier
}

func (w platformLogWriter) WriteMessage(level log.Level, message string) {
	line := fmt.Sprintf("[%s] %s", log.FormatLevel(level), message)
	if w.handler != nil {
		w.handler.OnLog(line)
	}
	if w.classifier != nil {
		w.classifier.OnLog(line)
	}
}

func (e *Engine) Start(configJSON []byte) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.State() != StateStopped {
		return fmt.Errorf("engine already running")
	}

	e.setState(StateStarting)
	e.done = make(chan struct{})

	registryCtx := boxContext(context.Background())
	options, err := json.UnmarshalExtendedContext[option.Options](registryCtx, configJSON)
	if err != nil {
		e.failLocked(fmt.Errorf("decode config: %w", E.Cause(err)))
		return err
	}

	ctx, cancel := context.WithCancel(registryCtx)
	instance, err := box.New(box.Options{
		Context:           ctx,
		Options:           options,
		PlatformLogWriter: e.platformWriter(),
	})
	if err != nil {
		cancel()
		e.failLocked(fmt.Errorf("create sing-box: %w", E.Cause(err)))
		return err
	}

	if err := instance.Start(); err != nil {
		cancel()
		e.failLocked(fmt.Errorf("start sing-box: %w", E.Cause(err)))
		return err
	}

	e.currentBox = instance
	e.currentCtx = ctx
	e.currentCancel = cancel
	e.setState(StateRunning)
	return nil
}

func (e *Engine) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.State() == StateStopped {
		return nil
	}
	if e.State() == StateStopping {
		return fmt.Errorf("already stopping")
	}

	e.setState(StateStopping)
	boxInstance := e.currentBox
	cancel := e.currentCancel

	if cancel != nil {
		cancel()
	}

	var err error
	if boxInstance != nil {
		err = boxInstance.Close()
	}

	e.currentBox = nil
	e.currentCancel = nil
	e.currentCtx = nil
	e.setState(StateStopped)

	if e.done != nil {
		select {
		case <-e.done:
		default:
			close(e.done)
		}
	}
	return err
}

func (e *Engine) Reload(configJSON []byte) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.State() == StateRunning || e.State() == StateStarting {
		e.setState(StateStopping)
		if e.currentCancel != nil {
			e.currentCancel()
		}
		if e.currentBox != nil {
			_ = e.currentBox.Close()
		}
		e.currentBox = nil
		e.currentCancel = nil
		e.currentCtx = nil
	}
	e.setState(StateStopped)
	e.done = nil

	// Re-run Start inline
	e.setState(StateStarting)
	e.done = make(chan struct{})

	registryCtx := boxContext(context.Background())
	options, err := json.UnmarshalExtendedContext[option.Options](registryCtx, configJSON)
	if err != nil {
		e.failLocked(fmt.Errorf("decode config: %w", E.Cause(err)))
		return err
	}

	ctx, cancel := context.WithCancel(registryCtx)
	instance, err := box.New(box.Options{
		Context:           ctx,
		Options:           options,
		PlatformLogWriter: e.platformWriter(),
	})
	if err != nil {
		cancel()
		e.failLocked(fmt.Errorf("create sing-box: %w", E.Cause(err)))
		return err
	}

	if err := instance.Start(); err != nil {
		cancel()
		e.failLocked(fmt.Errorf("start sing-box: %w", E.Cause(err)))
		return err
	}

	e.currentBox = instance
	e.currentCtx = ctx
	e.currentCancel = cancel
	e.setState(StateRunning)
	return nil
}

func (e *Engine) failLocked(err error) {
	e.setState(StateError)
	if e.done != nil {
		select {
		case <-e.done:
		default:
			close(e.done)
		}
	}
}

func (e *Engine) setState(s EngineState) {
	e.state.Store(int32(s))
}

// ============================================================================
// Manager (facade, same as PC)
// ============================================================================

type VPNStatus struct {
	State     string `json:"state"`
	ConfigID  string `json:"configId"`
	Message   string `json:"message"`
	Connected bool   `json:"connected"`
}

type Manager struct {
	mu             sync.Mutex
	engine         *Engine
	activeConfigID string
	lastError      string
}

func NewManager(engine *Engine) *Manager {
	return &Manager{engine: engine}
}

func (m *Manager) StartVPN(configID string, configJSON []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.engine.Start(configJSON); err != nil {
		m.lastError = err.Error()
		return err
	}
	m.activeConfigID = configID
	m.lastError = ""
	return nil
}

func (m *Manager) StopVPN() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.engine.Close(); err != nil {
		m.lastError = err.Error()
		return err
	}
	m.lastError = ""
	return nil
}

func (m *Manager) ReloadVPN(configID string, configJSON []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.engine.Reload(configJSON); err != nil {
		m.lastError = err.Error()
		return err
	}
	m.activeConfigID = configID
	m.lastError = ""
	return nil
}

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

func (m *Manager) SetLogHandler(h LogHandler) {
	m.engine.SetLogHandler(h)
}

// ============================================================================
// ErrorClassifier (same as PC)
// ============================================================================

type ErrorCategory int

const (
	CatHealthy ErrorCategory = iota
	CatNetworkDown
	CatServerDown
	CatDNSFailure
	CatTLSFailure
	CatServerBlocked
	CatWhitelistMode
	CatDegraded
	CatUnknown
)

func (c ErrorCategory) String() string {
	switch c {
	case CatHealthy:
		return "healthy"
	case CatNetworkDown:
		return "network_down"
	case CatServerDown:
		return "server_down"
	case CatDNSFailure:
		return "dns_failure"
	case CatTLSFailure:
		return "tls_failure"
	case CatServerBlocked:
		return "server_blocked"
	case CatWhitelistMode:
		return "whitelist_mode"
	case CatDegraded:
		return "degraded"
	default:
		return "unknown"
	}
}

func (c ErrorCategory) HumanExplain() string {
	switch c {
	case CatHealthy:
		return "Всё работает нормально"
	case CatNetworkDown:
		return "Нет интернета. Проверьте подключение."
	case CatServerDown:
		return "Сервер VPN не отвечает. Возможно, упал или заблокирован."
	case CatDNSFailure:
		return "Ошибка DNS. Не удаётся разрешить доменные имена."
	case CatTLSFailure:
		return "TLS-рукопожатие не удалось. Провайдер может блокировать."
	case CatServerBlocked:
		return "ТСПУ/DPI заблокировал соединение."
	case CatWhitelistMode:
		return "Обнаружен режим белых списков (БС). Обход невозможен."
	case CatDegraded:
		return "Туннель работает медленно."
	default:
		return "Неизвестная ошибка."
	}
}

type DiagEvent struct {
	Timestamp time.Time
	Category  ErrorCategory
	RawLine   string
}

type ErrorClassifier struct {
	mu        sync.Mutex
	current   ErrorCategory
	lastError string
	events    []DiagEvent
	maxEvents int
}

func NewErrorClassifier(maxEvents int) *ErrorClassifier {
	return &ErrorClassifier{
		current:   CatHealthy,
		maxEvents: maxEvents,
	}
}

func (ec *ErrorClassifier) OnLog(line string) {
	if !strings.Contains(line, "[error]") && !strings.Contains(line, "[warn]") {
		return
	}
	cat := classify(line)
	if cat == CatHealthy || cat == CatUnknown {
		return
	}

	ec.mu.Lock()
	defer ec.mu.Unlock()

	ec.current = cat
	ec.lastError = line
	ec.events = append(ec.events, DiagEvent{
		Timestamp: time.Now(),
		Category:  cat,
		RawLine:   line,
	})
	if len(ec.events) > ec.maxEvents {
		ec.events = ec.events[len(ec.events)-ec.maxEvents:]
	}
}

func (ec *ErrorClassifier) Current() ErrorCategory {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	return ec.current
}

func (ec *ErrorClassifier) LastError() string {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	return ec.lastError
}

func (ec *ErrorClassifier) Reset() {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	ec.current = CatHealthy
}

func classify(line string) ErrorCategory {
	lower := strings.ToLower(line)

	if strings.Contains(lower, "whitelist") || strings.Contains(lower, "белый список") {
		return CatWhitelistMode
	}
	if strings.Contains(lower, "reality: processed invalid") || strings.Contains(lower, "reality: failed") {
		return CatServerBlocked
	}
	if strings.Contains(lower, "tls handshake") && (strings.Contains(lower, "timeout") || strings.Contains(lower, "failed") || strings.Contains(lower, "eof") || strings.Contains(lower, "context deadline exceeded")) {
		return CatTLSFailure
	}
	if strings.Contains(lower, "lookup failed") || strings.Contains(lower, "no such host") || (strings.Contains(lower, "dns:") && strings.Contains(lower, "error")) {
		return CatDNSFailure
	}
	if strings.Contains(lower, "dial tcp") && (strings.Contains(lower, "i/o timeout") || strings.Contains(lower, "connection refused") || strings.Contains(lower, "no route to host")) {
		return CatServerDown
	}
	if strings.Contains(lower, "connection reset") || strings.Contains(lower, "wsarecv: an existing connection was forcibly closed") {
		if strings.Contains(lower, "outbound") || strings.Contains(lower, "vless") {
			return CatServerBlocked
		}
		return CatUnknown
	}
	if strings.Contains(lower, "context deadline exceeded") && strings.Contains(lower, "outbound") {
		return CatServerDown
	}
	if strings.Contains(lower, "eof") && strings.Contains(lower, "outbound") {
		return CatServerDown
	}
	return CatUnknown
}

func ClassifyProbeError(err error) ErrorCategory {
	if err == nil {
		return CatHealthy
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "connection refused") && strings.Contains(msg, "127.0.0.1") {
		return CatNetworkDown
	}
	if strings.Contains(msg, "timeout") || strings.Contains(msg, "deadline") {
		return CatServerDown
	}
	if strings.Contains(msg, "connection reset") || strings.Contains(msg, "forcibly closed") {
		return CatServerBlocked
	}
	return CatUnknown
}

// ============================================================================
// Circuit Breaker (same as PC)
// ============================================================================

type TunnelState int

const (
	StateClosed TunnelState = iota
	StateHalfOpen
	StateOpen
)

type circuitBreaker struct {
	mu               sync.Mutex
	state            TunnelState
	consecutiveFails int
	consecutiveOK    int
	lastStateChange  time.Time

	failThreshold   int
	halfOpenProbes  int
	cooldownStart   time.Duration
	cooldownMax     time.Duration
	currentCooldown time.Duration
}

func newCircuitBreaker() *circuitBreaker {
	return &circuitBreaker{
		state:           StateClosed,
		failThreshold:   3,
		halfOpenProbes:  2,
		cooldownStart:   10 * time.Second,
		cooldownMax:     60 * time.Second,
		currentCooldown: 10 * time.Second,
	}
}

func (cb *circuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.consecutiveFails = 0

	switch cb.state {
	case StateHalfOpen:
		cb.consecutiveOK++
		if cb.consecutiveOK >= cb.halfOpenProbes {
			cb.transition(StateClosed)
		}
	case StateOpen:
		cb.transition(StateHalfOpen)
		cb.consecutiveOK = 1
	}
}

func (cb *circuitBreaker) RecordFailure() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.consecutiveOK = 0
	cb.consecutiveFails++

	switch cb.state {
	case StateClosed:
		if cb.consecutiveFails >= cb.failThreshold {
			cb.transition(StateOpen)
			return true
		}
	case StateHalfOpen:
		cb.transition(StateOpen)
		return true
	}
	return false
}

func (cb *circuitBreaker) ShouldProbe() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	if cb.state != StateOpen {
		return false
	}
	return time.Since(cb.lastStateChange) >= cb.currentCooldown
}

func (cb *circuitBreaker) EnterHalfOpen() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	if cb.state == StateOpen {
		cb.transition(StateHalfOpen)
		cb.consecutiveOK = 0
	}
}

func (cb *circuitBreaker) State() TunnelState {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.state
}

func (cb *circuitBreaker) StateName() string {
	switch cb.State() {
	case StateClosed:
		return "HEALTHY"
	case StateHalfOpen:
		return "RECOVERING"
	default:
		return "FAILED"
	}
}

func (cb *circuitBreaker) transition(newState TunnelState) {
	cb.state = newState
	cb.lastStateChange = time.Now()
	cb.consecutiveFails = 0
	cb.consecutiveOK = 0

	if newState == StateOpen {
		cb.currentCooldown *= 2
		if cb.currentCooldown > cb.cooldownMax {
			cb.currentCooldown = cb.cooldownMax
		}
	}
	if newState == StateClosed {
		cb.currentCooldown = cb.cooldownStart
	}
}

// ============================================================================
// AdaptiveEngine (same as PC)
// ============================================================================

type DiagStatus struct {
	State       string `json:"state"`
	Category    string `json:"category"`
	Explanation string `json:"explanation"`
	LastError   string `json:"lastError"`
	FailCount   int    `json:"failCount"`
	OkCount     int    `json:"okCount"`
}

type AdaptiveEngine struct {
	engine     *Engine
	proxyAddr  string
	configID   string
	config     []byte
	classifier *ErrorClassifier
	cb         *circuitBreaker

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu      sync.Mutex
	running bool

	// iOS callback bridge
	diagCallback func(string)
	logCallback  func(string)
}

func NewAdaptiveEngine(engine *Engine) *AdaptiveEngine {
	return &AdaptiveEngine{
		engine:     engine,
		proxyAddr:  "127.0.0.1:20808",
		classifier: NewErrorClassifier(200),
		cb:         newCircuitBreaker(),
	}
}

func (ae *AdaptiveEngine) SetDiagCallback(cb func(string)) {
	ae.mu.Lock()
	ae.diagCallback = cb
	ae.mu.Unlock()
}

func (ae *AdaptiveEngine) SetLogCallback(cb func(string)) {
	ae.mu.Lock()
	ae.logCallback = cb
	ae.mu.Unlock()
}

func (ae *AdaptiveEngine) Classifier() *ErrorClassifier {
	return ae.classifier
}

func (ae *AdaptiveEngine) Start(configID string, config []byte) {
	ae.mu.Lock()
	ae.configID = configID
	ae.config = config
	ae.classifier.Reset()
	ae.cb = newCircuitBreaker()
	if ae.running {
		ae.mu.Unlock()
		return
	}
	ae.running = true
	ae.ctx, ae.cancel = context.WithCancel(context.Background())
	ae.mu.Unlock()

	ae.wg.Add(1)
	go ae.loop()
}

func (ae *AdaptiveEngine) Stop() {
	ae.mu.Lock()
	ae.running = false
	cancel := ae.cancel
	ae.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	ae.wg.Wait()
}

func (ae *AdaptiveEngine) loop() {
	defer ae.wg.Done()

	const (
		checkInterval = 30 * time.Second
		probeTimeout  = 10 * time.Second
	)

	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ae.ctx.Done():
			return
		case <-ticker.C:
			if !ae.engine.Running() {
				continue
			}
			err := ae.deepProbe(probeTimeout)
			if err == nil {
				ae.onProbeSuccess()
				continue
			}
			cat := ClassifyProbeError(err)
			ae.emit("engine:diag", fmt.Sprintf("[diag] probe failed: %s (%s)", cat.String(), err))
			tripped := ae.cb.RecordFailure()
			if tripped {
				ae.onCircuitOpen(cat)
			} else {
				ae.emit("engine:diag", fmt.Sprintf("[diag] failure %d/%d — watching", ae.cb.consecutiveFails, ae.cb.failThreshold))
			}
		}
	}
}

func (ae *AdaptiveEngine) onProbeSuccess() {
	oldState := ae.cb.State()
	ae.cb.RecordSuccess()
	ae.classifier.Reset()
	newState := ae.cb.State()
	if oldState != StateClosed && newState == StateClosed {
		ae.emit("engine:diag", "[diag] ✅ tunnel recovered — circuit CLOSED")
	}
}

func (ae *AdaptiveEngine) onCircuitOpen(cat ErrorCategory) {
	ae.mu.Lock()
	cfgID := ae.configID
	cfg := ae.config
	ae.mu.Unlock()

	ae.emit("engine:diag", fmt.Sprintf("[diag] 🔴 circuit OPEN — %s: %s", cat.String(), cat.HumanExplain()))

	if cat == CatNetworkDown {
		ae.emit("engine:diag", "[diag] local internet appears down — waiting for recovery")
		return
	}

	ae.emit("engine:diag", fmt.Sprintf("[diag] 🔄 reloading %s...", cfgID))
	if err := ae.engine.Reload(cfg); err != nil {
		ae.emit("engine:diag", fmt.Sprintf("[diag] reload failed: %v", err))
	} else {
		ae.emit("engine:diag", "[diag] reload OK — probing to verify...")
		time.Sleep(2 * time.Second)
		if err := ae.deepProbe(8 * time.Second); err == nil {
			ae.cb.RecordSuccess()
			ae.classifier.Reset()
			ae.emit("engine:diag", "[diag] ✅ reload recovered the tunnel")
			return
		}
		ae.emit("engine:diag", "[diag] reload did not fix the tunnel")
	}

	ae.emit("engine:diag", "[diag] ⏳ waiting for urltest auto-failover (VPS→WARP→direct)...")
}

func (ae *AdaptiveEngine) deepProbe(timeout time.Duration) error {
	client := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			Proxy: func(*http.Request) (*url.URL, error) {
				return url.Parse("http://" + ae.proxyAddr)
			},
		},
	}
	resp, err := client.Get("http://www.gstatic.com/generate_204")
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode != 204 && resp.StatusCode != 200 {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	return nil
}

func (ae *AdaptiveEngine) emit(name string, data string) {
	ae.mu.Lock()
	cb := ae.diagCallback
	lc := ae.logCallback
	ae.mu.Unlock()
	if cb != nil {
		cb(data)
	}
	if lc != nil && name == "engine:log" {
		lc(data)
	}
}

func (ae *AdaptiveEngine) Diagnostics() DiagStatus {
	cat := ae.classifier.Current()
	return DiagStatus{
		State:       ae.cb.StateName(),
		Category:    cat.String(),
		Explanation: cat.HumanExplain(),
		LastError:   ae.classifier.LastError(),
		FailCount:   ae.cb.consecutiveFails,
		OkCount:     ae.cb.consecutiveOK,
	}
}

// ============================================================================
// Config Builder (same as PC, but synced with working template)
// ============================================================================

type VPSConfig struct {
	Server     string `json:"server"`
	ServerPort uint16 `json:"server_port"`
	UUID       string `json:"uuid"`
	ServerName string `json:"server_name"`
}

func BuildConfig(vps VPSConfig, listenPort uint16, ruCIDRPath string) ([]byte, error) {
	cfg := map[string]any{
		"log": map[string]any{"level": "info", "timestamp": true},
		"dns": map[string]any{
			"servers": []map[string]any{
				{"type": "https", "tag": "cloudflare", "server": "1.1.1.1", "path": "/dns-query", "detour": "auto"},
				{"type": "local", "tag": "local", "detour": "direct"},
			},
			"rules":    []map[string]any{{"outbound": "any", "server": "local"}},
			"strategy": "ipv4_only",
		},
		"inbounds": []map[string]any{
			{"type": "mixed", "tag": "mixed-in", "listen": "127.0.0.1", "listen_port": listenPort},
		},
		"endpoints": []map[string]any{
			{
				"type": "wireguard", "tag": "warp-fallback",
				"address": []string{"172.16.0.2/32"},
				"private_key": "OO/FIEWNyFKneQ4fF5l8dLMQ2OJcbVVHtqwNk4A+FVU=",
				"peers": []map[string]any{
					{
						"address": "162.159.192.1", "port": 4500,
						"public_key": "bmXOC+F1FxEMF9dyiK2H5/1SUtzH0JuVo51h2wPfgyo=",
						"allowed_ips": []string{"0.0.0.0/0", "::/0"},
						"persistent_keepalive_interval": 25,
					},
				},
			},
		},
		"outbounds": []map[string]any{
			{
				"type": "urltest", "tag": "auto",
				"outbounds": []string{"proxy", "warp-fallback", "direct"},
				"url": "https://www.gstatic.com/generate_204",
				"interval": "1m", "tolerance": 100,
			},
			{
				"type": "vless", "tag": "proxy",
				"server": vps.Server, "server_port": vps.ServerPort,
				"uuid": vps.UUID,
				"tls": map[string]any{
					"enabled":     true,
					"server_name": vps.ServerName,
				},
			},
			{"type": "direct", "tag": "direct"},
			{"type": "block", "tag": "block"},
		},
	}

	rules := []map[string]any{
		{"action": "sniff"},
		{"action": "hijack-dns", "inbound": "mixed-in", "protocol": "dns"},
	}

	var ruleSets []map[string]any
	if ruCIDRPath != "" {
		if _, err := os.Stat(ruCIDRPath); err == nil {
			ruleSets = append(ruleSets, map[string]any{
				"type":   "local", "tag": "ru-cidr", "format": "source", "path": ruCIDRPath,
			})
			rules = append(rules, map[string]any{
				"rule_set": []string{"ru-cidr"}, "action": "direct",
			})
		}
	}

	rules = append(rules, map[string]any{"ip_is_private": true, "action": "direct"})

	route := map[string]any{
		"rules": rules, "final": "auto",
		"default_domain_resolver": "local",
		"auto_detect_interface":     true,
	}
	if len(ruleSets) > 0 {
		route["rule_set"] = ruleSets
	}
	cfg["route"] = route

	return json.MarshalIndent(cfg, "", "  ")
}

func EnsureCIDRFile(rawList string, dir string) (string, error) {
	if rawList == "" {
		return "", nil
	}
	path := filepath.Join(dir, "ru-cidr.json")
	rs := map[string]any{
		"version": 3,
		"rules":   []map[string]any{{"ip_cidr": splitCSV(rawList)}},
	}
	data, err := json.MarshalIndent(rs, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return "", fmt.Errorf("write CIDR file: %w", err)
	}
	return path, nil
}

func splitCSV(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == ',' {
			if start < i {
				out = append(out, trim(s[start:i]))
			}
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, trim(s[start:]))
	}
	return out
}

func trim(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\n' || s[0] == '\r' || s[0] == '\t') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\n' || s[len(s)-1] == '\r' || s[len(s)-1] == '\t') {
		s = s[:len(s)-1]
	}
	return s
}

// ============================================================================
// iOS Bridge — exported functions for gomobile
// ============================================================================

// SnowdenCore is the main facade exported to Swift.
type SnowdenCore struct {
	engine   *Engine
	manager  *Manager
	adaptive *AdaptiveEngine
}

// NewSnowdenCore creates the full stack (Engine + Manager + AdaptiveEngine).
func NewSnowdenCore() *SnowdenCore {
	engine := NewEngine()
	adaptive := NewAdaptiveEngine(engine)
	engine.SetClassifier(adaptive.Classifier())
	manager := NewManager(engine)
	return &SnowdenCore{
		engine:   engine,
		manager:  manager,
		adaptive: adaptive,
	}
}

// StartVPN launches the tunnel with a JSON config string.
func (sc *SnowdenCore) StartVPN(configID string, configJSON string) error {
	return sc.manager.StartVPN(configID, []byte(configJSON))
}

// StopVPN stops the tunnel.
func (sc *SnowdenCore) StopVPN() error {
	sc.adaptive.Stop()
	return sc.manager.StopVPN()
}

// ReloadVPN swaps config without stopping.
func (sc *SnowdenCore) ReloadVPN(configID string, configJSON string) error {
	return sc.manager.ReloadVPN(configID, []byte(configJSON))
}

// Status returns current VPN status as JSON.
func (sc *SnowdenCore) Status() string {
	st := sc.manager.Status()
	b, _ := json.Marshal(st)
	return string(b)
}

// Diagnostics returns diagnostic state as JSON.
func (sc *SnowdenCore) Diagnostics() string {
	d := sc.adaptive.Diagnostics()
	b, _ := json.Marshal(d)
	return string(b)
}

// SetLogCallback sets a callback for log lines (called from Go to Swift).
func (sc *SnowdenCore) SetLogCallback(cb func(string)) {
	sc.adaptive.SetLogCallback(cb)
	sc.engine.SetLogHandler(logHandlerBridge{cb: cb})
}

// SetDiagCallback sets a callback for diagnostic events.
func (sc *SnowdenCore) SetDiagCallback(cb func(string)) {
	sc.adaptive.SetDiagCallback(cb)
}

// BuildConfigJSON generates a sing-box config from VPS params.
func (sc *SnowdenCore) BuildConfigJSON(server string, serverPort int, uuid string, serverName string, listenPort int, ruCIDRPath string) (string, error) {
	cfg, err := BuildConfig(VPSConfig{
		Server:     server,
		ServerPort: uint16(serverPort),
		UUID:       uuid,
		ServerName: serverName,
	}, uint16(listenPort), ruCIDRPath)
	if err != nil {
		return "", err
	}
	return string(cfg), nil
}

// EnsureCIDR generates ru-cidr.json from a comma-separated list.
func (sc *SnowdenCore) EnsureCIDR(rawList string, dir string) (string, error) {
	return EnsureCIDRFile(rawList, dir)
}

type logHandlerBridge struct {
	cb func(string)
}

func (h logHandlerBridge) OnLog(line string) {
	if h.cb != nil {
		h.cb(line)
	}
}

// ============================================================================
// Registry (selective protocol registration, same as PC)
// ============================================================================

func boxContext(parent context.Context) context.Context {
	return box.Context(
		parent,
		inbounds(),
		outbounds(),
		endpoints(),
		dnsTransports(),
		services(),
		certificateProviders(),
	)
}

func inbounds() *inbound.Registry {
	r := inbound.NewRegistry()
	tun.RegisterInbound(r)
	direct.RegisterInbound(r)
	mixed.RegisterInbound(r)
	socks.RegisterInbound(r)
	http.RegisterInbound(r)
	shadowsocks.RegisterInbound(r)
	return r
}

func outbounds() *outbound.Registry {
	r := outbound.NewRegistry()
	direct.RegisterOutbound(r)
	block.RegisterOutbound(r)
	group.RegisterSelector(r)
	group.RegisterURLTest(r)
	socks.RegisterOutbound(r)
	http.RegisterOutbound(r)
	shadowsocks.RegisterOutbound(r)
	vmess.RegisterOutbound(r)
	trojan.RegisterOutbound(r)
	shadowtls.RegisterOutbound(r)
	vless.RegisterOutbound(r)
	masque.RegisterOutbound(r)
	hysteria.RegisterOutbound(r)
	hysteria2.RegisterOutbound(r)
	return r
}

func endpoints() *endpoint.Registry {
	r := endpoint.NewRegistry()
	wireguard.RegisterEndpoint(r)
	return r
}

func dnsTransports() *dns.TransportRegistry {
	r := dns.NewTransportRegistry()
	transport.RegisterTCP(r)
	transport.RegisterUDP(r)
	transport.RegisterTLS(r)
	transport.RegisterHTTPS(r)
	hosts.RegisterTransport(r)
	local.RegisterTransport(r)
	fakeip.RegisterTransport(r)
	resolved.RegisterTransport(r)
	return r
}

func services() *service.Registry {
	r := service.NewRegistry()
	resolved.RegisterService(r)
	return r
}

func certificateProviders() *certificate.Registry {
	return certificate.NewRegistry()
}
