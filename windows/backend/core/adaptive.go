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
	StateClosed TunnelState = iota // HEALTHY — tunnel works
	StateHalfOpen                  // DEGRADED — probing after failure
	StateOpen                      // FAILED — tunnel down, action needed
)

// CircuitBreaker implements a 3-state circuit breaker:
//
//	Closed ──(N fails)──► Open ──(cooldown)──► HalfOpen ──(M successes)──► Closed
//	                                                    └──(1 fail)──► Open
//
// On Open it reloads the engine (re-establishes the VLESS connection).
// If reload doesn't help it switches to graceful degradation (WARP → direct).
type circuitBreaker struct {
	mu               sync.Mutex
	state            TunnelState
	consecutiveFails int
	consecutiveOK    int
	lastStateChange  time.Time

	// Thresholds (tuned for VPN use case)
	failThreshold    int           // fails in Closed to trip Open (default: 3)
	halfOpenProbes   int           // successes needed in HalfOpen to close (default: 2)
	cooldownStart    time.Duration // initial Open→HalfOpen cooldown (default: 10s)
	cooldownMax      time.Duration // max cooldown after exponential growth (default: 60s)
	currentCooldown  time.Duration // current cooldown (grows on repeated Opens)
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

// transition is the internal state-change helper (caller holds lock).
func (cb *circuitBreaker) transition(newState TunnelState) {
	cb.state = newState
	cb.lastStateChange = time.Now()
	cb.consecutiveFails = 0
	cb.consecutiveOK = 0

	// Exponential backoff on each Open
	if newState == StateOpen {
		cb.currentCooldown *= 2
		if cb.currentCooldown > cb.cooldownMax {
			cb.currentCooldown = cb.cooldownMax
		}
	}
	// Reset backoff on recovery to Closed
	if newState == StateClosed {
		cb.currentCooldown = cb.cooldownStart
	}
}

// ──────────────────────────────────────────────────────────────────────────

// AdaptiveEngine monitors the running VPN tunnel and automatically recovers
// from failures using a circuit-breaker + error classifier + graceful
// degradation chain.
//
// Recovery sequence:
//  1. HEALTHY → N failures → FAILED (Open)
//  2. FAILED → Engine.Reload (re-establish VLESS)
//  3. Still FAILED → urltest group auto-switches to WARP (sing-box internal)
//  4. Still FAILED → recovery probe loop (every cooldown) tries HalfOpen
//  5. HalfOpen → M successes → HEALTHY (Closed)
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
	appCtx context.Context

	mu      sync.Mutex
	running bool
}

// NewAdaptiveEngine wires an Engine to monitor.
func NewAdaptiveEngine(engine *Engine) *AdaptiveEngine {
	return &AdaptiveEngine{
		engine:     engine,
		proxyAddr:  "127.0.0.1:20808",
		classifier: NewErrorClassifier(200),
		cb:         newCircuitBreaker(),
	}
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

// Start begins background health-checking. configID + config are saved so
// Reload can re-launch the same tunnel.
func (ae *AdaptiveEngine) Start(configID string, config []byte) {
	ae.mu.Lock()
	ae.configID = configID
	ae.config = config
	ae.classifier.Reset()
	ae.cb = newCircuitBreaker() // fresh breaker
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

// Stop halts the background checker.
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

// loop runs the health-check cycle with circuit-breaker logic.
func (ae *AdaptiveEngine) loop() {
	defer ae.wg.Done()

	// Recovery: a panic in Reload/deepProbe must not kill monitoring forever.
	defer func() {
		if r := recover(); r != nil {
			ae.emit("engine:diag", fmt.Sprintf("[diag] ⚠️ adaptive loop panicked: %v — restarting", r))
			time.Sleep(5 * time.Second)
			select {
			case <-ae.ctx.Done():
				return
			default:
				ae.wg.Add(1)
				go ae.loop() // respawn
			}
		}
	}()

	const (
		checkInterval = 30 * time.Second // deep probe interval when healthy
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

			// Deep probe: full HTTP round-trip through the proxy.
			err := ae.deepProbe(probeTimeout)

			if err == nil {
				ae.onProbeSuccess()
				continue
			}

			// Probe failed — classify and record
			cat := ClassifyProbeError(err)
			ae.emit("engine:diag", fmt.Sprintf("[diag] probe failed: %s (%s)", cat.String(), err))

			tripped := ae.cb.RecordFailure()
			if tripped {
				// Circuit opened — take recovery action
				ae.onCircuitOpen(cat)
			} else {
				ae.emit("engine:diag", fmt.Sprintf("[diag] failure %d/%d — watching",
					ae.cb.consecutiveFails, ae.cb.failThreshold))
			}
		}
	}
}

// onProbeSuccess records a successful health-check and may close the circuit.
func (ae *AdaptiveEngine) onProbeSuccess() {
	oldState := ae.cb.State()
	ae.cb.RecordSuccess()
	ae.classifier.Reset()

	newState := ae.cb.State()
	if oldState != StateClosed && newState == StateClosed {
		ae.emit("engine:diag", "[diag] ✅ tunnel recovered — circuit CLOSED")
	}
}

// onCircuitOpen handles the transition to FAILED state.
// It attempts Engine.Reload first; if that fails repeatedly it relies on
// the sing-box urltest group to auto-switch to WARP, then to direct.
func (ae *AdaptiveEngine) onCircuitOpen(cat ErrorCategory) {
	ae.mu.Lock()
	cfgID := ae.configID
	cfg := ae.config
	ae.mu.Unlock()

	ae.emit("engine:diag", fmt.Sprintf("[diag] 🔴 circuit OPEN — %s: %s",
		cat.String(), cat.HumanExplain()))

	// Special case: local internet is down — no point reloading
	if cat == CatNetworkDown {
		ae.emit("engine:diag", "[diag] local internet appears down — waiting for recovery")
		return
	}

	// Attempt 1: reload the engine (re-establish VLESS connection)
	ae.emit("engine:diag", fmt.Sprintf("[diag] 🔄 reloading %s...", cfgID))
	if err := ae.engine.Reload(cfg); err != nil {
		ae.emit("engine:diag", fmt.Sprintf("[diag] reload failed: %v", err))
	} else {
		ae.emit("engine:diag", "[diag] reload OK — probing to verify...")
		// Quick probe after reload (shorter timeout)
		time.Sleep(2 * time.Second)
		if err := ae.deepProbe(8 * time.Second); err == nil {
			ae.cb.RecordSuccess()
			ae.classifier.Reset()
			ae.emit("engine:diag", "[diag] ✅ reload recovered the tunnel")
			return
		}
		ae.emit("engine:diag", "[diag] reload did not fix the tunnel")
	}

	// Attempt 2: the sing-box urltest group (VPS+WARP+direct) should
	// auto-switch. We just log and wait.
	ae.emit("engine:diag", "[diag] ⏳ waiting for urltest auto-failover (VPS→WARP→direct)...")
}

// deepProbe sends an HTTP request through the mixed-in proxy and returns
// nil on success, or the error if the tunnel is not delivering traffic.
// CRITICAL: forces IPv4 — the VPS has no IPv6 connectivity, so an IPv6 probe
// fails instantly and the circuit-breaker wrongly flags the server as down.
func (ae *AdaptiveEngine) deepProbe(timeout time.Duration) error {
	client := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			Proxy: func(*http.Request) (*url.URL, error) {
				return url.Parse("http://" + ae.proxyAddr)
			},
			// Force IPv4 only — VPS tunnel has no IPv6 route.
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return (&net.Dialer{Timeout: timeout}).DialContext(ctx, "tcp4", addr)
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
	State        string `json:"state"`         // HEALTHY / RECOVERING / FAILED
	Category     string `json:"category"`      // error category string
	Explanation  string `json:"explanation"`   // human-readable Russian text
	LastError    string `json:"lastError"`     // raw last error line
	FailCount    int    `json:"failCount"`     // consecutive failures
	OkCount      int    `json:"okCount"`       // consecutive successes (HalfOpen)
}

// Diagnostics returns the current diagnostic state for the UI.
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
