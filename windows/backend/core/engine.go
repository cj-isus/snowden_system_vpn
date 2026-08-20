// Package core bridges the sing-box engine (as an embedded Go library, via box.Box)
// with the rest of the application. This is NOT a subprocess: sing-box runs inside
// the same OS process, which is the only way to get graceful Start/Close on Windows
// without a console (see sing-box issue #3806).
package core

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	box "github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/json"
)

// EngineState describes the lifecycle state of the embedded sing-box instance.
type EngineState int32

const (
	StateStopped EngineState = iota
	StateStarting
	StateRunning
	StateStopping
	StateError
)

// String returns a human-readable state name for the UI / logs.
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

// LogHandler receives sing-box log lines in real time. Implemented by the UI layer.
type LogHandler interface {
	OnLog(line string)
}

// Engine embeds a single sing-box *box.Box and serialises Start/Close/Reload so
// the adaptive engine and the UI never race on the same instance.
//
// Design notes:
//   - Hot-reload via Clash API PUT /configs is NOT supported by sing-box
//     (issues #386, #2698, closed not planned). Therefore Reload() rebuilds the
//     whole *box.Box: Close() the old one, then New()+Start() the new one.
//   - Start/Close are synchronous and hold a mutex for the duration of the call,
//     so callers can rely on State() being accurate once the call returns.
//   - The instance is run under a cancellable context; cancelling it is part of
//     the Close() path, mirroring what `sing-box run` does with SIGTERM.
type Engine struct {
	mu sync.Mutex

	// state is read by other goroutines (UI, adaptive engine) without the mutex,
	// so it lives in an atomic. Use setState() to write it.
	state atomic.Int32

	// currentCtx / currentCancel belong to the running instance; replaced on Reload.
	currentCtx    context.Context
	currentCancel context.CancelFunc
	currentBox    *box.Box

	// logHandler is optional; when set, sing-box logs are forwarded to it.
	// Must be set before Start() and not changed concurrently.
	logHandler LogHandler
	// classifier is optional; when set, sing-box logs are classified in
	// real-time for the adaptive engine's circuit-breaker decisions.
	classifier *ErrorClassifier

	// startedCond lets tests / the UI wait until the first Start() has reported a
	// terminal state (Running or Error).
	done chan struct{}
}

// NewEngine builds an Engine in the Stopped state.
func NewEngine() *Engine {
	e := &Engine{}
	e.setState(StateStopped)
	return e
}

// SetLogHandler wires a log sink. Must be called before Start(); calling it on a
// running engine is a data race and is ignored.
func (e *Engine) SetLogHandler(h LogHandler) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.State() != StateStopped {
		return
	}
	e.logHandler = h
}

// SetClassifier wires an error classifier. Each log line is also fed to the
// classifier for real-time error classification.
func (e *Engine) SetClassifier(c *ErrorClassifier) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.classifier = c
}

// State returns the current lifecycle state atomically.
func (e *Engine) State() EngineState {
	return EngineState(e.state.Load())
}

// Running is a convenience check for the adaptive engine's hot path.
func (e *Engine) Running() bool {
	return e.State() == StateRunning
}

// platformWriter returns a log.PlatformWriter that forwards every sing-box
// log line to the configured LogHandler. This is the native streaming path:
// sing-box calls WriteMessage for each line, with no polling or REST API
// required. Passing it via box.Options also flips needObservable=true inside
// box.New, so the traffic/connection trackers are initialised too.
func (e *Engine) platformWriter() log.PlatformWriter {
	return platformLogWriter{handler: e.logHandler, classifier: e.classifier}
}

// platformLogWriter adapts our LogHandler to sing-box's log.PlatformWriter.
type platformLogWriter struct {
	handler    LogHandler
	classifier *ErrorClassifier
}

func (w platformLogWriter) WriteMessage(level log.Level, message string) {
	line := fmt.Sprintf("[%s] %s", log.FormatLevel(level), message)
	if w.handler != nil {
		w.handler.OnLog(line)
	}
	// Feed every line into the error classifier for real-time classification.
	if w.classifier != nil {
		w.classifier.OnLog(line)
	}
}

// Start parses configJSON and launches a new sing-box instance in this process.
// If an instance is already running, Start returns ErrAlreadyRunning — use
// Reload() to swap configs. Start blocks until Start() on box.Box returns.
func (e *Engine) Start(configJSON []byte) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Allow restart from both Stopped and Error states (Error = previous crash).
	if e.State() != StateStopped && e.State() != StateError {
		return ErrAlreadyRunning
	}

	e.setState(StateStarting)
	e.done = make(chan struct{})
	return e.startLocked(configJSON)
}

// startLocked is the single shared launch path for both Start and Reload.
// The caller MUST hold e.mu. It handles panic recovery, config decode, box.New
// and a 15s start timeout — so a broken config can never leave the engine stuck
// in "Starting" (it always ends in Running or Error).
func (e *Engine) startLocked(configJSON []byte) (errRet error) {
	var instance *box.Box

	// Catch panics from sing-box config decode/New/Start — sing-box panics on
	// malformed configs. Without this, state would be stuck in Starting forever.
	defer func() {
		if r := recover(); r != nil {
			if instance != nil {
				_ = closeBoxWithTimeout(instance, 5*time.Second)
			}
			e.failLocked(fmt.Errorf("sing-box panic: %v", r))
			errRet = fmt.Errorf("sing-box panic: %v", r)
		}
	}()

	// 1. Parse the JSON config into option.Options using sing-box's extended
	//    decoder, which understands config selectors and merge semantics.
	// boxContext registers only the protocols this app uses (tun/vless/reality/
	// hysteria2/shadowsocks/...) — see registry.go for why we don't use the full
	// include.Context.
	registryCtx := boxContext(context.Background())
	options, err := json.UnmarshalExtendedContext[option.Options](registryCtx, configJSON)
	if err != nil {
		e.failLocked(fmt.Errorf("decode config: %w", E.Cause(err)))
		return err
	}

	// 2. Build a cancellable context with timeout to prevent hangs.
	ctx, cancel := context.WithCancel(registryCtx)
	instance, err = box.New(box.Options{
		Context:           ctx,
		Options:           options,
		PlatformLogWriter: e.platformWriter(),
	})
	if err != nil {
		cancel()
		e.failLocked(fmt.Errorf("create sing-box: %w", E.Cause(err)))
		return err
	}

	// 3. Start with a bounded timeout. If Start fails or times out, cancel the
	// instance and give Close a bounded chance to release partially-created
	// resources before publishing StateError.
	startDone := make(chan error, 1)
	go func() {
		startDone <- startBox(instance)
	}()
	startTimer := time.NewTimer(15 * time.Second)
	defer startTimer.Stop()

	select {
	case err := <-startDone:
		if err != nil {
			cancel()
			_ = closeBoxWithTimeout(instance, 5*time.Second)
			wrapped := fmt.Errorf("start sing-box: %w", E.Cause(err))
			e.failLocked(wrapped)
			return wrapped
		}
	case <-startTimer.C:
		cancel()
		cleanupErr := closeBoxWithTimeout(instance, 5*time.Second)
		wrapped := fmt.Errorf("sing-box start timeout (15s)")
		if cleanupErr != nil {
			wrapped = fmt.Errorf("%w; cleanup: %v", wrapped, cleanupErr)
		}
		e.failLocked(wrapped)
		return wrapped
	}

	e.currentBox = instance
	e.currentCtx = ctx
	e.currentCancel = cancel
	e.setState(StateRunning)
	e.closeDoneLocked()
	return nil
}

// startBox converts a panic in the Start goroutine into an ordinary error so
// the parent lifecycle can publish StateError instead of crashing the process.
func startBox(instance *box.Box) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("sing-box start panic: %v", r)
		}
	}()
	return instance.Start()
}

func closeBox(instance *box.Box) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("sing-box close panic: %v", r)
		}
	}()
	return instance.Close()
}

// closeBoxWithTimeout prevents a failed or replaced instance from blocking the
// lifecycle forever. The timeout is a final safety boundary for a library call
// that may be stuck in platform/network cleanup.
func closeBoxWithTimeout(instance *box.Box, timeout time.Duration) error {
	if instance == nil {
		return nil
	}
	closeDone := make(chan error, 1)
	go func() {
		closeDone <- closeBox(instance)
	}()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case err := <-closeDone:
		return err
	case <-timer.C:
		return fmt.Errorf("sing-box close timeout (%s)", timeout)
	}
}

// Close gracefully shuts the running instance down. It is safe to call when
// Stopped (no-op). Returns the first error encountered, if any.
func (e *Engine) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.State() == StateStopped {
		return nil
	}
	if e.State() == StateStopping {
		return ErrAlreadyStopping
	}

	e.setState(StateStopping)
	boxInstance := e.currentBox
	cancel := e.currentCancel

	// Cancel the context first: this signals long-lived goroutines inside
	// sing-box that shutdown was requested, mirroring SIGTERM handling.
	if cancel != nil {
		cancel()
	}

	var err error
	if boxInstance != nil {
		err = closeBoxWithTimeout(boxInstance, 5*time.Second)
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

// Reload atomically swaps the running config for a new one. Because sing-box
// has no in-process hot-reload, this is implemented as teardown + Start via the
// shared startLocked path; the mutex guarantees callers never observe an
// in-between (Stopped) state from outside. On failure the engine ends up in
// Error and the error is returned; callers should treat that as "VPN is down"
// and retry with a fallback config.
func (e *Engine) Reload(configJSON []byte) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.State() == StateStopping {
		return ErrAlreadyStopping
	}

	// Tear down whatever is running (if anything). Close is bounded so a broken
	// old instance cannot prevent the new config from reaching a terminal state.
	if e.State() == StateRunning || e.State() == StateStarting {
		e.setState(StateStopping)
		if e.currentCancel != nil {
			e.currentCancel()
		}
		if e.currentBox != nil {
			_ = closeBoxWithTimeout(e.currentBox, 5*time.Second)
		}
		e.currentBox = nil
		e.currentCancel = nil
		e.currentCtx = nil
	}

	// Wake waiters attached to the previous attempt before replacing the channel.
	e.closeDoneLocked()
	e.setState(StateStarting)
	e.done = make(chan struct{})
	return e.startLocked(configJSON)
}

// Wait blocks until the current Start/Reload has reached a terminal state
// (Running or Error). Returns ErrNotStarted if Start was never called.
func (e *Engine) Wait() error {
	e.mu.Lock()
	ch := e.done
	e.mu.Unlock()
	if ch == nil {
		return ErrNotStarted
	}
	<-ch
	return nil
}

// closeDoneLocked wakes callers waiting for the current lifecycle attempt.
// Caller must hold e.mu.
func (e *Engine) closeDoneLocked() {
	if e.done != nil {
		select {
		case <-e.done:
		default:
			close(e.done)
		}
	}
}

// failLocked transitions the engine to StateError and closes the done channel.
// Caller must hold e.mu.
func (e *Engine) failLocked(err error) {
	e.setState(StateError)
	e.closeDoneLocked()
}

// setState atomically stores the lifecycle state.
func (e *Engine) setState(s EngineState) {
	e.state.Store(int32(s))
}

// Sentinel errors. Use errors.Is to match them — do not compare with == in
// package-boundary code.
var (
	ErrAlreadyRunning  = errors.New("engine: already running")
	ErrAlreadyStopping = errors.New("engine: already stopping")
	ErrNotStarted      = errors.New("engine: not started")
)
