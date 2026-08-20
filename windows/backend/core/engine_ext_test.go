package core

import (
	"errors"
	"sync"
	"testing"
	"time"
)

// ─── State transitions ─────────────────────────────────────────────────────

func TestEngineInitialStateIsStopped(t *testing.T) {
	e := NewEngine()
	if e.State() != StateStopped {
		t.Errorf("initial State() = %v, want StateStopped", e.State())
	}
	if e.Running() {
		t.Error("Running() = true on fresh engine, want false")
	}
}

func TestEngineStateString(t *testing.T) {
	tests := []struct {
		state EngineState
		want  string
	}{
		{StateStopped, "stopped"},
		{StateStarting, "starting"},
		{StateRunning, "running"},
		{StateStopping, "stopping"},
		{StateError, "error"},
		{EngineState(99), "unknown"},
	}
	for _, tt := range tests {
		if got := tt.state.String(); got != tt.want {
			t.Errorf("EngineState(%d).String() = %q, want %q", tt.state, got, tt.want)
		}
	}
}

// ─── Start rejects concurrent start ────────────────────────────────────────

func TestEngineStartRejectsWhenRunning(t *testing.T) {
	e := NewEngine()
	e.mu.Lock()
	e.setState(StateRunning)
	e.mu.Unlock()

	err := e.Start([]byte(`{}`))
	if !errors.Is(err, ErrAlreadyRunning) {
		t.Errorf("Start while running: err = %v, want ErrAlreadyRunning", err)
	}
}

func TestEngineStartRejectsWhenStarting(t *testing.T) {
	e := NewEngine()
	e.mu.Lock()
	e.setState(StateStarting)
	e.mu.Unlock()

	err := e.Start([]byte(`{}`))
	if !errors.Is(err, ErrAlreadyRunning) {
		t.Errorf("Start while starting: err = %v, want ErrAlreadyRunning", err)
	}
}

// ─── Close lifecycle ───────────────────────────────────────────────────────

func TestEngineCloseFromStoppedIsNoop(t *testing.T) {
	e := NewEngine()
	if err := e.Close(); err != nil {
		t.Errorf("Close from Stopped: err = %v, want nil", err)
	}
	if e.State() != StateStopped {
		t.Errorf("State after noop Close = %v, want StateStopped", e.State())
	}
}

func TestEngineCloseFromRunningTransitionsToStopped(t *testing.T) {
	e := NewEngine()
	e.mu.Lock()
	e.setState(StateRunning)
	e.done = make(chan struct{})
	e.mu.Unlock()

	if err := e.Close(); err != nil {
		t.Errorf("Close: err = %v", err)
	}
	if e.State() != StateStopped {
		t.Errorf("State after Close = %v, want StateStopped", e.State())
	}
}

func TestEngineCloseRejectsWhenStopping(t *testing.T) {
	e := NewEngine()
	e.mu.Lock()
	e.setState(StateStopping)
	e.mu.Unlock()

	err := e.Close()
	if !errors.Is(err, ErrAlreadyStopping) {
		t.Errorf("Close while stopping: err = %v, want ErrAlreadyStopping", err)
	}
}

// ─── Wait ──────────────────────────────────────────────────────────────────

func TestEngineWaitReturnsErrNotStarted(t *testing.T) {
	e := NewEngine()
	err := e.Wait()
	if !errors.Is(err, ErrNotStarted) {
		t.Errorf("Wait before Start: err = %v, want ErrNotStarted", err)
	}
}

func TestEngineWaitReturnsImmediatelyAfterDone(t *testing.T) {
	e := NewEngine()
	e.mu.Lock()
	e.done = make(chan struct{})
	e.setState(StateRunning)
	e.closeDoneLocked()
	e.mu.Unlock()

	done := make(chan error, 1)
	go func() { done <- e.Wait() }()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Wait: err = %v, want nil", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Wait did not return after done was closed")
	}
}

func TestEngineWaitUnblocksOnError(t *testing.T) {
	e := NewEngine()
	e.mu.Lock()
	e.done = make(chan struct{})
	e.setState(StateStarting)
	e.mu.Unlock()

	done := make(chan error, 1)
	go func() { done <- e.Wait() }()

	// Simulate start failure.
	e.mu.Lock()
	e.failLocked(errors.New("test error"))
	e.mu.Unlock()

	select {
	case <-done:
		// OK — Wait returned
	case <-time.After(time.Second):
		t.Fatal("Wait did not unblock after failLocked")
	}
}

// ─── Concurrent Wait calls ─────────────────────────────────────────────────

func TestEngineConcurrentWaits(t *testing.T) {
	e := NewEngine()
	e.mu.Lock()
	e.done = make(chan struct{})
	e.setState(StateRunning)
	e.mu.Unlock()

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = e.Wait()
		}()
	}

	// Close the channel — all waits should unblock.
	e.mu.Lock()
	e.closeDoneLocked()
	e.mu.Unlock()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("concurrent Wait calls did not all return")
	}
}

// ─── Reload from Error state ──────────────────────────────────────────────

func TestEngineReloadFromStoppedSkipsTeardown(t *testing.T) {
	e := NewEngine()
	e.mu.Lock()
	e.done = make(chan struct{})
	e.setState(StateStopped)
	e.mu.Unlock()

	// Reload on a stopped engine with invalid config → should fail with
	// a config error (not panic, not hang).
	err := e.Reload([]byte(`{invalid json`))
	if err == nil {
		t.Error("Reload with bad JSON: err = nil, want error")
	}
	// After failed reload, state should be Error.
	if e.State() != StateError {
		t.Errorf("State after bad Reload = %v, want StateError", e.State())
	}
}

// ─── Sentinel errors are distinct ──────────────────────────────────────────

func TestSentinelErrorsDistinct(t *testing.T) {
	sentinels := []error{ErrAlreadyRunning, ErrAlreadyStopping, ErrNotStarted}
	for i := 0; i < len(sentinels); i++ {
		for j := i + 1; j < len(sentinels); j++ {
			if errors.Is(sentinels[i], sentinels[j]) {
				t.Errorf("sentinel %v and %v should not match", sentinels[i], sentinels[j])
			}
		}
	}
}

// ─── SetLogHandler / SetClassifier ─────────────────────────────────────────

func TestEngineSetLogHandlerWhileRunningIgnored(t *testing.T) {
	e := NewEngine()
	e.mu.Lock()
	e.setState(StateRunning)
	e.mu.Unlock()

	// Should not panic — just ignored.
	e.SetLogHandler(nil)
}

func TestEngineSetClassifierWhileRunningIgnored(t *testing.T) {
	e := NewEngine()
	e.mu.Lock()
	e.setState(StateRunning)
	e.mu.Unlock()

	// Should not panic — just ignored.
	e.SetClassifier(nil)
}
