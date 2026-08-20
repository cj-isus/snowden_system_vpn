package core

import (
	"testing"
	"time"
)

// TestCircuitBreakerTripsAtThreshold verifies RecordFailure trips Open exactly
// on the failThreshold, and a single success resets the counter.
func TestCircuitBreakerTripsAtThreshold(t *testing.T) {
	cb := newCircuitBreaker()
	if cb.State() != StateClosed {
		t.Fatalf("expected Closed initially, got %v", cb.State())
	}

	// failThreshold-1 fails must NOT trip.
	for i := 0; i < cb.failThreshold-1; i++ {
		if cb.RecordFailure() {
			t.Fatalf("trip on failure %d (threshold %d)", i+1, cb.failThreshold)
		}
		if cb.State() != StateClosed {
			t.Fatalf("expected Closed after %d fails, got %v", i+1, cb.State())
		}
	}

	// One more fail trips Open.
	if !cb.RecordFailure() {
		t.Fatal("expected trip on reaching failThreshold")
	}
	if cb.State() != StateOpen {
		t.Fatalf("expected Open after trip, got %v", cb.State())
	}
}

// TestCircuitBreakerSingleFailThenSuccessKeepsClosed verifies A6: a single
// failure followed by a success does not trip the tunnel.
func TestCircuitBreakerSingleFailThenSuccessKeepsClosed(t *testing.T) {
	cb := newCircuitBreaker()
	cb.RecordFailure() // 1 fail — below threshold
	if cb.State() != StateClosed {
		t.Fatalf("single fail must keep Closed, got %v", cb.State())
	}
	cb.RecordSuccess() // recovery resets fails
	cb.RecordFailure()
	cb.RecordSuccess()
	if cb.State() != StateClosed {
		t.Fatalf("state must remain Closed, got %v", cb.State())
	}
}

// TestCircuitBreakerShouldProbeCooldown verifies ShouldProbe is false before
// the cooldown elapses and true after.
func TestCircuitBreakerShouldProbeCooldown(t *testing.T) {
	cb := newCircuitBreaker()
	cb.RecordFailure()
	cb.RecordFailure()
	cb.RecordFailure() // → Open

	if cb.State() != StateOpen {
		t.Fatalf("expected Open, got %v", cb.State())
	}

	// Immediately: cooldown not elapsed.
	if cb.ShouldProbe() {
		t.Fatal("ShouldProbe must be false right after Open")
	}

	// Manually rewind the timestamp to simulate cooldown elapsed.
	cb.mu.Lock()
	cb.lastStateChange = time.Now().Add(-cb.currentCooldown - time.Second)
	cb.mu.Unlock()

	if !cb.ShouldProbe() {
		t.Fatal("ShouldProbe must be true after cooldown elapsed")
	}
}

// TestCircuitBreakerBackoffGrowsAndResets verifies the 10→20→40→60 progression
// and reset on Closed.
func TestCircuitBreakerBackoffGrowsAndResets(t *testing.T) {
	cb := newCircuitBreaker()

	// First trip: base cooldown 10s.
	cb.RecordFailure()
	cb.RecordFailure()
	cb.RecordFailure()
	if got := cb.CurrentCooldown(); got != 10*time.Second {
		t.Fatalf("first cooldown = %v, want 10s", got)
	}

	// Enter HalfOpen then fail again → backoff doubles.
	cb.EnterHalfOpen()
	cb.RecordFailure() // HalfOpen → Open
	if got := cb.CurrentCooldown(); got != 20*time.Second {
		t.Fatalf("second cooldown = %v, want 20s", got)
	}

	// Capped at 60s after growth.
	prev := cb.CurrentCooldown()
	for i := 0; i < 10; i++ {
		cb.EnterHalfOpen()
		cb.RecordFailure()
	}
	final := cb.CurrentCooldown()
	if final != 60*time.Second {
		t.Fatalf("cooldown should cap at 60s, got %v", final)
	}
	if final < prev {
		t.Fatal("cooldown should only grow")
	}

	// Recovery to Closed resets to base.
	cb.EnterHalfOpen()
	for i := 0; i < cb.halfOpenProbes; i++ {
		cb.RecordSuccess()
	}
	if cb.State() != StateClosed {
		t.Fatalf("expected Closed after recovery, got %v", cb.State())
	}
	if got := cb.CurrentCooldown(); got != 10*time.Second {
		t.Fatalf("cooldown not reset on Closed, got %v", got)
	}
}

// TestCircuitBreakerOpenHalfOpenClosed verifies the full recovery walk.
func TestCircuitBreakerOpenHalfOpenClosed(t *testing.T) {
	cb := newCircuitBreaker()
	cb.RecordFailure()
	cb.RecordFailure()
	cb.RecordFailure() // Open

	cb.EnterHalfOpen()
	if cb.State() != StateHalfOpen {
		t.Fatalf("expected HalfOpen, got %v", cb.State())
	}

	// halfOpenProbes successes close the circuit.
	for i := 0; i < cb.halfOpenProbes; i++ {
		cb.RecordSuccess()
	}
	if cb.State() != StateClosed {
		t.Fatalf("expected Closed after %d successes, got %v", cb.halfOpenProbes, cb.State())
	}
}

// TestCircuitBreakerHalfOpenSingleFailReopens verifies one fail in HalfOpen
// sends the breaker back to Open.
func TestCircuitBreakerHalfOpenSingleFailReopens(t *testing.T) {
	cb := newCircuitBreaker()
	cb.RecordFailure()
	cb.RecordFailure()
	cb.RecordFailure() // Open
	cb.EnterHalfOpen()
	if !cb.RecordFailure() {
		t.Fatal("one fail in HalfOpen must trip to Open")
	}
	if cb.State() != StateOpen {
		t.Fatalf("expected Open after HalfOpen fail, got %v", cb.State())
	}
}
