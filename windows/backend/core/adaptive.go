package core

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// TunnelState is the circuit-breaker state of the VPN tunnel.
type TunnelState int

const (
	StateClosed   TunnelState = iota // HEALTHY — tunnel works
	StateHalfOpen                    // DEGRADED — probing after failure
	StateOpen                        // FAILED — tunnel down, action needed
)

// CircuitBreaker implements a 3-state circuit breaker:
//
//	Closed ──(N fails)──► Open ──(cooldown)──► HalfOpen ──(M successes)──► Closed
//	                                                    └──(1 fail)──► Open
//
// On Open it asks the lifecycle owner to re-apply the selected protected
// channel. If that does not help, the protected route remains failed-closed;
// it must never silently degrade to direct traffic.
type circuitBreaker struct {
	mu               sync.Mutex
	state            TunnelState
	consecutiveFails int
	consecutiveOK    int
	lastStateChange  time.Time

	// Thresholds (tuned for VPN use case)
	failThreshold   int           // consecutive fails in Closed to trip Open (default: 2)
	halfOpenProbes  int           // successes needed in HalfOpen to close (default: 2)
	cooldownStart   time.Duration // initial Open→HalfOpen cooldown (default: 10s)
	cooldownMax     time.Duration // max cooldown after exponential growth (default: 60s)
	currentCooldown time.Duration // current cooldown (grows on repeated Opens)
}

func newCircuitBreaker() *circuitBreaker {
	return &circuitBreaker{
		state:           StateClosed,
		failThreshold:   2,
		halfOpenProbes:  2,
		cooldownStart:   10 * time.Second,
		cooldownMax:     60 * time.Second,
		currentCooldown: 10 * time.Second,
	}
}

// RecordSuccess resets fail counters. In HalfOpen, counts towards recovery.
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
		// Unexpected success in Open — treat as recovery
		cb.transition(StateHalfOpen)
		cb.consecutiveOK = 1
	}
}

// RecordFailure increments fail counter. Trips Open when threshold reached.
// Returns true if the state changed to Open (caller should take action).
func (cb *circuitBreaker) RecordFailure() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.consecutiveOK = 0
	cb.consecutiveFails++

	switch cb.state {
	case StateClosed:
		if cb.consecutiveFails >= cb.failThreshold {
			cb.transition(StateOpen)
			return true // caller should reload
		}
	case StateHalfOpen:
		cb.transition(StateOpen)
		return true
	}
	return false
}

// ShouldProbe checks if the cooldown has expired and we should try HalfOpen.
func (cb *circuitBreaker) ShouldProbe() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	if cb.state != StateOpen {
		return false
	}
	return time.Since(cb.lastStateChange) >= cb.currentCooldown
}

// EnterHalfOpen moves from Open to HalfOpen (called by the recovery probe loop).
func (cb *circuitBreaker) EnterHalfOpen() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	if cb.state == StateOpen {
		cb.transition(StateHalfOpen)
		cb.consecutiveOK = 0
	}
}

// State returns the current state thread-safely.
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

// CurrentCooldown returns the current backoff (for tuning the monitor ticker).
func (cb *circuitBreaker) CurrentCooldown() time.Duration {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.currentCooldown
}

// Counters returns a consistent diagnostic snapshot of breaker counters.
func (cb *circuitBreaker) Counters() (fails, oks, threshold int) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.consecutiveFails, cb.consecutiveOK, cb.failThreshold
}

// transition is the internal state-change helper (caller holds lock).
func (cb *circuitBreaker) transition(newState TunnelState) {
	oldState := cb.state
	cb.state = newState
	cb.lastStateChange = time.Now()
	cb.consecutiveFails = 0
	cb.consecutiveOK = 0

	switch newState {
	case StateOpen:
		if oldState == StateClosed {
			// first Open since recovery: base cooldown (10s)
			cb.currentCooldown = cb.cooldownStart
		} else {
			// repeated Open (back from HalfOpen): exponential backoff
			cb.currentCooldown *= 2
			if cb.currentCooldown > cb.cooldownMax {
				cb.currentCooldown = cb.cooldownMax
			}
		}
	case StateClosed:
		// Reset backoff on recovery to Closed
		cb.currentCooldown = cb.cooldownStart
	}
}

// ──────────────────────────────────────────────────────────────────────────

// AdaptiveEngine monitors the running VPN tunnel and automatically recovers
// from failures using a circuit-breaker + error classifier + graceful
// degradation chain.
//
// Recovery sequence:
//  1. HEALTHY → consecutive failures → FAILED (Open)
//  2. FAILED → Manager recovery callback re-applies the protected channel
//  3. Still FAILED → keep protected routes blocked; do not use direct fallback
//  4. Cooldown expires → HalfOpen probe
//  5. HalfOpen → required successes → HEALTHY (Closed)
type AdaptiveEngine struct {
	engine     *Engine
	proxyAddr  string
	configID   string
	config     []byte
	classifier *ErrorClassifier
	cb         *circuitBreaker
	memory     *ChannelMemory

	ctx      context.Context
	cancel   context.CancelFunc
	recovery func(string, []byte) error
	wg       sync.WaitGroup
	appCtx   context.Context

	// lifecycleMu serializes Start and Stop. Without it, a concurrent Start can
	// add a new loop after Stop has begun waiting on the old loop, leaving Stop
	// blocked forever and allowing two monitors to coexist.
	lifecycleMu sync.Mutex
	mu          sync.Mutex
	running     bool
}

// NewAdaptiveEngine wires an Engine to monitor.
func NewAdaptiveEngine(engine *Engine) *AdaptiveEngine {
	ae := &AdaptiveEngine{
		engine:     engine,
		proxyAddr:  "127.0.0.1:20808",
		classifier: NewErrorClassifier(200),
		cb:         newCircuitBreaker(),
		memory:     NewChannelMemory(DefaultChannelMemoryPath()),
	}
	_ = ae.memory.Load() // restore persisted channel health, best-effort
	return ae
}

// SetWailsContext lets the adaptive engine emit events to the frontend.
func (ae *AdaptiveEngine) SetWailsContext(ctx context.Context) {
	ae.mu.Lock()
	ae.appCtx = ctx
	ae.mu.Unlock()
}

// Classifier returns the error classifier (for OnLog wiring and UI queries).
func (ae *AdaptiveEngine) Classifier() *ErrorClassifier {
	return ae.classifier
}

// SetRecoveryFunc routes adaptive reloads through the application/Manager
// lifecycle instead of bypassing it with a direct Engine call.
func (ae *AdaptiveEngine) SetRecoveryFunc(fn func(string, []byte) error) {
	ae.mu.Lock()
	ae.recovery = fn
	ae.mu.Unlock()
}

// Start begins background health-checking. configID + config are saved so
// Reload can re-launch the same tunnel.
func (ae *AdaptiveEngine) Start(configID string, config []byte) {
	ae.lifecycleMu.Lock()
	defer ae.lifecycleMu.Unlock()

	snapshot := append([]byte(nil), config...)
	ae.mu.Lock()
	if ae.running {
		// A reload while monitoring is active updates the snapshot but does not
		// reset the breaker or start a second loop.
		ae.configID = configID
		ae.config = append(ae.config[:0], snapshot...)
		ae.mu.Unlock()
		if ks := ChannelKeysFromConfig(snapshot); len(ks) > 0 {
			ae.memory.Prune(ks)
			ae.memory.EnforceCap()
		}
		return
	}

	ae.configID = configID
	ae.config = append(ae.config[:0], snapshot...)
	ae.classifier.Reset()
	ae.cb = newCircuitBreaker() // fresh breaker
	// Drop channel memory for endpoints that no longer exist in this config.
	if ks := ChannelKeysFromConfig(snapshot); len(ks) > 0 {
		ae.memory.Prune(ks)
		ae.memory.EnforceCap()
	}
	ae.running = true
	ae.ctx, ae.cancel = context.WithCancel(context.Background())
	// Add before releasing ae.mu so Stop cannot observe a running engine with
	// a zero WaitGroup counter between Start and the goroutine launch.
	ae.wg.Add(1)
	ae.mu.Unlock()

	go ae.loop()
}

// UpdateConfig replaces the snapshot used by recovery without resetting the
// circuit breaker or spawning another monitor. Manager calls this after every
// successful UI/Telegram reload.
func (ae *AdaptiveEngine) UpdateConfig(configID string, config []byte) {
	snapshot := append([]byte(nil), config...)
	ae.mu.Lock()
	ae.configID = configID
	ae.config = append(ae.config[:0], snapshot...)
	ae.mu.Unlock()

	if ks := ChannelKeysFromConfig(snapshot); len(ks) > 0 {
		ae.memory.Prune(ks)
		ae.memory.EnforceCap()
	}
}

// Stop halts the background checker and persists channel memory.
func (ae *AdaptiveEngine) Stop() {
	ae.lifecycleMu.Lock()
	defer ae.lifecycleMu.Unlock()

	ae.mu.Lock()
	if !ae.running {
		ae.mu.Unlock()
		_ = ae.memory.Save()
		return
	}
	ae.running = false
	cancel := ae.cancel
	ae.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	ae.wg.Wait()

	ae.mu.Lock()
	ae.cancel = nil
	ae.ctx = nil
	ae.mu.Unlock()
	_ = ae.memory.Save()
}

// loop runs the health-check cycle with circuit-breaker logic.
func (ae *AdaptiveEngine) loop() {
	defer ae.wg.Done()

	// Capture the run context once. Start/Stop are serialized and Stop waits for
	// this goroutine before the context is replaced, so the local value remains
	// valid for the entire loop lifetime.
	ae.mu.Lock()
	loopCtx := ae.ctx
	ae.mu.Unlock()
	if loopCtx == nil {
		return
	}

	// Recovery: a panic in Reload/deepProbe must not kill monitoring forever.
	defer func() {
		if r := recover(); r != nil {
			ae.emit("engine:diag", fmt.Sprintf("[diag] ⚠️ adaptive loop panicked: %v — restarting", r))
			if !waitForContext(loopCtx, 5*time.Second) {
				return
			}
			select {
			case <-loopCtx.Done():
				return
			default:
				ae.wg.Add(1)
				go ae.loop() // respawn
			}
		}
	}()

	const (
		healthyInterval = 5 * time.Second // Closed: bounded detection latency
		probeInterval   = 3 * time.Second // confirming / HalfOpen: aggressive probe
		probeTimeout    = 3 * time.Second
	)

	ticker := time.NewTicker(healthyInterval)
	defer ticker.Stop()

	for {
		select {
		case <-loopCtx.Done():
			return

		case <-ticker.C:
			if !ae.engine.Running() {
				continue
			}

			// Ticker interval is driven by the circuit-breaker state (A1):
			//  - Open: wait out the cooldown, then enter HalfOpen and probe fast.
			//  - HalfOpen: probe every probeInterval to confirm recovery.
			//  - Closed: probe rarely (healthyInterval).
			switch ae.cb.State() {
			case StateOpen:
				if !ae.cb.ShouldProbe() {
					continue // cooldown not elapsed — hold off on a dead channel
				}
				ae.cb.EnterHalfOpen()
				ae.emit("engine:diag", "[diag] ⏱ cooldown elapsed — HalfOpen, probing")
				ticker.Reset(probeInterval)
			case StateHalfOpen:
				ticker.Reset(probeInterval)
			default:
				ticker.Reset(healthyInterval)
			}

			// Deep probe: full HTTP round-trip through the proxy.
			err := ae.deepProbe(probeTimeout)

			if err == nil {
				ae.onProbeSuccess()
				continue
			}

			// Probe failed — classify and record. A single loss never trips;
			// the breaker needs the configured consecutive-failure threshold.
			cat := ClassifyProbeError(err)
			ae.emit("engine:diag", fmt.Sprintf("[diag] probe failed: %s (%s)", cat.String(), err))

			tripped := ae.cb.RecordFailure()
			if tripped {
				// Circuit opened — take recovery action, then hold off.
				ae.onCircuitOpen(cat)
				ticker.Reset(ae.cb.CurrentCooldown())
			} else {
				fails, _, threshold := ae.cb.Counters()
				ae.emit("engine:diag", fmt.Sprintf("[diag] failure %d/%d — re-checking in %s",
					fails, threshold, probeInterval))
				ticker.Reset(probeInterval) // fast re-confirm
			}
		}
	}
}

// onProbeSuccess records a successful health-check and may close the circuit.
func (ae *AdaptiveEngine) onProbeSuccess() {
	oldState := ae.cb.State()
	ae.cb.RecordSuccess()
	ae.classifier.Reset()
	if k := ae.primaryChannelKey(); k != "" {
		ae.memory.Record(k, true)
	}

	newState := ae.cb.State()
	if oldState != StateClosed && newState == StateClosed {
		ae.emit("engine:diag", "[diag] ✅ tunnel recovered — circuit CLOSED")
	}
}

// primaryChannelKey returns the memory key of the current config's primary
// outbound (the VPS entry), or "" if none.
func (ae *AdaptiveEngine) primaryChannelKey() string {
	ae.mu.Lock()
	cfg := append([]byte(nil), ae.config...)
	ae.mu.Unlock()
	return PrimaryChannelKeyFromConfig(cfg)
}

// onCircuitOpen handles the transition to FAILED state. It asks the Manager /
// Application lifecycle to re-apply the protected channel. There is deliberately
// no direct fallback here: protected traffic must fail closed.
func (ae *AdaptiveEngine) onCircuitOpen(cat ErrorCategory) {
	ae.mu.Lock()
	cfgID := ae.configID
	cfg := append([]byte(nil), ae.config...)
	recovery := ae.recovery
	ae.mu.Unlock()

	// Remember the primary channel as failed so it is not preferred first.
	if k := PrimaryChannelKeyFromConfig(cfg); k != "" {
		ae.memory.Record(k, false)
	}

	ae.emit("engine:diag", fmt.Sprintf("[diag] 🔴 circuit OPEN — %s: %s",
		cat.String(), cat.HumanExplain()))

	// Special case: local internet is down — no point reloading
	if cat == CatNetworkDown {
		ae.emit("engine:diag", "[diag] local internet appears down — waiting for recovery")
		return
	}

	if ae.contextDone() {
		return
	}

	// Re-apply through the Manager/Application lifecycle so metrics, active config
	// and adaptive snapshots remain consistent.
	ae.emit("engine:diag", fmt.Sprintf("[diag] 🔄 reloading %s...", cfgID))
	var reloadErr error
	if recovery != nil {
		reloadErr = recovery(cfgID, cfg)
	} else {
		// Fallback keeps the core usable in isolated unit tests; production wires
		// SetRecoveryFunc from App after constructing Manager.
		reloadErr = ae.engine.Reload(cfg)
	}
	if reloadErr != nil {
		err := reloadErr
		ae.emit("engine:diag", fmt.Sprintf("[diag] reload failed: %v", err))
	} else {
		ae.emit("engine:diag", "[diag] reload OK — probing to verify...")
		// Quick probe after reload (shorter timeout), interruptible by Stop.
		if !waitForContext(ae.runContext(), 2*time.Second) {
			return
		}
		if err := ae.deepProbe(8 * time.Second); err == nil {
			ae.cb.RecordSuccess()
			ae.classifier.Reset()
			ae.emit("engine:diag", "[diag] ✅ reload recovered the tunnel")
			return
		}
		ae.emit("engine:diag", "[diag] reload did not fix the tunnel")
	}

	// No direct/urltest fallback is claimed here. The selector/controller must
	// choose another validated protected channel, or the route remains blocked.
	ae.emit("engine:diag", "[diag] ⛔ protected channel unavailable — keeping fail-closed policy")
}

// deepProbe sends an HTTP request through the mixed-in proxy and returns
// nil on success, or the error if the tunnel is not delivering traffic.
// CRITICAL: forces IPv4 — the VPS has no IPv6 connectivity, so an IPv6 probe
// fails instantly and the circuit-breaker wrongly flags the server as down.
func (ae *AdaptiveEngine) deepProbe(timeout time.Duration) error {
	parent := ae.runContext()
	probeCtx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()

	ae.mu.Lock()
	proxyAddr := ae.proxyAddr
	ae.mu.Unlock()
	client := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			Proxy: func(*http.Request) (*url.URL, error) {
				return url.Parse("http://" + proxyAddr)
			},
			// Force IPv4 only — VPS tunnel has no IPv6 route.
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return (&net.Dialer{Timeout: timeout}).DialContext(ctx, "tcp4", addr)
			},
		},
	}
	req, err := http.NewRequestWithContext(probeCtx, http.MethodGet, "http://www.gstatic.com/generate_204", nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 204 && resp.StatusCode != 200 {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	return nil
}

// runContext returns the current monitor context. A background context keeps
// isolated unit tests safe when they call deepProbe before Start.
func (ae *AdaptiveEngine) runContext() context.Context {
	ae.mu.Lock()
	ctx := ae.ctx
	ae.mu.Unlock()
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

// contextDone reports whether the current monitor was stopped.
func (ae *AdaptiveEngine) contextDone() bool {
	ae.mu.Lock()
	running := ae.running
	ctx := ae.ctx
	ae.mu.Unlock()
	if !running || ctx == nil {
		return true
	}
	select {
	case <-ctx.Done():
		return true
	default:
		return false
	}
}

func waitForContext(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

// emit sends an event to the frontend if a Wails context is set.
func (ae *AdaptiveEngine) emit(name string, data string) {
	ae.mu.Lock()
	ctx := ae.appCtx
	ae.mu.Unlock()
	if ctx != nil {
		runtime.EventsEmit(ctx, name, data)
	}
}

// Status returns a diagnostic snapshot for the UI.
type DiagStatus struct {
	State       string `json:"state"`       // HEALTHY / RECOVERING / FAILED
	Category    string `json:"category"`    // error category string
	Explanation string `json:"explanation"` // human-readable Russian text
	LastError   string `json:"lastError"`   // raw last error line
	FailCount   int    `json:"failCount"`   // consecutive failures
	OkCount     int    `json:"okCount"`     // consecutive successes (HalfOpen)
	// Channel memory (the "remember" link of the adaptive loop).
	BestChannel string               `json:"bestChannel"` // highest-scoring known channel
	Memory      ChannelMemorySummary `json:"memory"`      // tracked channels summary
}

// Diagnostics returns the current diagnostic state for the UI.
func (ae *AdaptiveEngine) Diagnostics() DiagStatus {
	cat := ae.classifier.Current()
	mem := ae.memory.Summary(3)
	ae.mu.Lock()
	cb := ae.cb
	ae.mu.Unlock()
	fails, oks, _ := cb.Counters()
	return DiagStatus{
		State:       ae.cb.StateName(),
		Category:    cat.String(),
		Explanation: cat.HumanExplain(),
		LastError:   ae.classifier.LastError(),
		FailCount:   fails,
		OkCount:     oks,
		BestChannel: mem.BestKey,
		Memory:      mem,
	}
}
