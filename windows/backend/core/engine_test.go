package core

import (
	"testing"
	"time"
)

func waitForEngine(t *testing.T, engine *Engine) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		_ = engine.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Engine.Wait did not return")
	}
}

func TestEngineWaitReturnsAfterRunning(t *testing.T) {
	engine := NewEngine()
	engine.mu.Lock()
	engine.done = make(chan struct{})
	engine.setState(StateRunning)
	engine.closeDoneLocked()
	engine.mu.Unlock()

	waitForEngine(t, engine)
}

func TestEngineWaitReturnsAfterError(t *testing.T) {
	engine := NewEngine()
	engine.mu.Lock()
	engine.done = make(chan struct{})
	engine.setState(StateStarting)
	engine.failLocked(testEngineError{})
	engine.mu.Unlock()

	if engine.State() != StateError {
		t.Fatalf("state = %s, want error", engine.State())
	}
	waitForEngine(t, engine)
}

type testEngineError struct{}

func (testEngineError) Error() string { return "test engine error" }
