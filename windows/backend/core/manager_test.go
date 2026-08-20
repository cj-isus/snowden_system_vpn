package core

import (
	"testing"
	"time"
)

// newTestManager builds a Manager with a fresh engine (not started — the
// metrics sampler checks engine.Running(), so a stopped engine just skips the
// poll body; the goroutine lifecycle is what we test here).
func newTestManager() *Manager {
	return NewManager(NewEngine())
}

// TestStartMetricsSingleActiveGoroutine verifies A2: repeated startMetrics calls
// (one per Reload) leave exactly ONE live sampling goroutine.
func TestStartMetricsSingleActiveGoroutine(t *testing.T) {
	m := newTestManager()

	for i := 0; i < 50; i++ {
		m.startMetrics()
		// Give the old goroutine a chance to observe its closed stop channel.
		time.Sleep(time.Millisecond)
	}
	if got := m.metricsActive.Load(); got != 1 {
		t.Fatalf("expected exactly 1 active metrics goroutine, got %d", got)
	}

	m.stopMetrics()
	if got := m.metricsActive.Load(); got != 0 {
		t.Fatalf("expected 0 active goroutines after stop, got %d", got)
	}
}

// TestStopMetricsIdempotent verifies stopMetrics with no running loop, and a
// second stop, do not panic or hang.
func TestStopMetricsIdempotent(t *testing.T) {
	m := newTestManager()

	done := make(chan struct{})
	go func() {
		m.stopMetrics() // never started → must be a no-op
		m.stopMetrics() // second call → still a no-op
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("stopMetrics hung on an idempotent call")
	}
}

// TestStartStopMetricsRace sanity-checks start+stop churn under -race.
func TestStartStopMetricsRace(t *testing.T) {
	m := newTestManager()
	for i := 0; i < 20; i++ {
		m.startMetrics()
		m.stopMetrics()
	}
	if got := m.metricsActive.Load(); got != 0 {
		t.Fatalf("expected 0 active goroutines, got %d", got)
	}
}
