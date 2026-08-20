package core

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// ─── Record + GetBest ──────────────────────────────────────────────────────

func TestDomainStatsGetBestEmpty(t *testing.T) {
	r := NewDomainStatsRegistry()
	if got := r.GetBest("example.com"); got != "" {
		t.Errorf("GetBest on empty registry = %q, want empty", got)
	}
}

func TestDomainStatsGetBestReturnsHighestScoredOutbound(t *testing.T) {
	r := NewDomainStatsRegistry()

	// VPS: 5 successes, 0 errors, low latency.
	for i := 0; i < 5; i++ {
		r.Record("youtube.com", "vps-hy2", 50, 10000, true)
	}
	// WARP: 3 successes, 2 errors, high latency.
	for i := 0; i < 3; i++ {
		r.Record("youtube.com", "warp", 300, 5000, true)
	}
	for i := 0; i < 2; i++ {
		r.Record("youtube.com", "warp", 0, 0, false)
	}

	best := r.GetBest("youtube.com")
	if best != "vps-hy2" {
		t.Errorf("GetBest(youtube.com) = %q, want vps-hy2", best)
	}
}

func TestDomainStatsGetBestUnknownDomain(t *testing.T) {
	r := NewDomainStatsRegistry()
	r.Record("known.com", "vps", 50, 1000, true)

	if got := r.GetBest("unknown.com"); got != "" {
		t.Errorf("GetBest for unknown domain = %q, want empty", got)
	}
}

func TestDomainStatsGetBestPrefersLowLatency(t *testing.T) {
	r := NewDomainStatsRegistry()

	// Both outbounds have 100% success, but different latencies.
	for i := 0; i < 10; i++ {
		r.Record("fast.com", "low-lat", 30, 1000, true)
		r.Record("fast.com", "high-lat", 500, 1000, true)
	}

	best := r.GetBest("fast.com")
	if best != "low-lat" {
		t.Errorf("GetBest(fast.com) = %q, want low-lat (lower latency)", best)
	}
}

// ─── EWMA latency tracking ─────────────────────────────────────────────────

func TestDomainStatsEWMALatency(t *testing.T) {
	r := NewDomainStatsRegistry()

	r.Record("ewma.com", "ob", 100, 100, true)
	r.Record("ewma.com", "ob", 200, 100, true)
	r.Record("ewma.com", "ob", 300, 100, true)

	r.mu.RLock()
	m := r.metrics["ewma.com"]["ob"]
	avg := m.AvgLatencyMs
	r.mu.RUnlock()

	// First sample = 100.
	// Second: 100*0.7 + 200*0.3 = 70 + 60 = 130.
	// Third:  130*0.7 + 300*0.3 = 91 + 90 = 181.
	if avg != 181 {
		t.Errorf("EWMA AvgLatencyMs = %d, want 181", avg)
	}
}

func TestDomainStatsZeroLatencyIgnored(t *testing.T) {
	r := NewDomainStatsRegistry()

	r.Record("test.com", "ob", 100, 100, true)
	r.Record("test.com", "ob", 0, 100, true) // latency 0 = unknown, should not update EWMA

	r.mu.RLock()
	m := r.metrics["test.com"]["ob"]
	r.mu.RUnlock()

	if m.AvgLatencyMs != 100 {
		t.Errorf("AvgLatencyMs = %d after zero-latency sample, want 100", m.AvgLatencyMs)
	}
}

// ─── Bytes tracking ────────────────────────────────────────────────────────

func TestDomainStatsAccumulatesBytes(t *testing.T) {
	r := NewDomainStatsRegistry()

	r.Record("data.com", "ob", 50, 1000, true)
	r.Record("data.com", "ob", 50, 2000, true)
	r.Record("data.com", "ob", 50, 3000, true)

	r.mu.RLock()
	m := r.metrics["data.com"]["ob"]
	r.mu.RUnlock()

	if m.TotalBytes != 6000 {
		t.Errorf("TotalBytes = %d, want 6000", m.TotalBytes)
	}
	if m.Requests != 3 {
		t.Errorf("Requests = %d, want 3", m.Requests)
	}
}

// ─── Success / error counting ──────────────────────────────────────────────

func TestDomainStatsSuccessAndErrorCounts(t *testing.T) {
	r := NewDomainStatsRegistry()

	r.Record("mixed.com", "ob", 50, 100, true)
	r.Record("mixed.com", "ob", 0, 0, false)
	r.Record("mixed.com", "ob", 50, 100, true)

	r.mu.RLock()
	m := r.metrics["mixed.com"]["ob"]
	r.mu.RUnlock()

	if m.SuccessCount != 2 {
		t.Errorf("SuccessCount = %d, want 2", m.SuccessCount)
	}
	if m.ErrorCount != 1 {
		t.Errorf("ErrorCount = %d, want 1", m.ErrorCount)
	}
}

// ─── Empty domain/outbound ignored ──────────────────────────────────────────

func TestDomainStatsRecordEmptyDomainIgnored(t *testing.T) {
	r := NewDomainStatsRegistry()
	r.Record("", "ob", 50, 100, true)

	if len(r.metrics) != 0 {
		t.Errorf("empty domain should not be recorded, got %d domains", len(r.metrics))
	}
}

func TestDomainStatsRecordEmptyOutboundIgnored(t *testing.T) {
	r := NewDomainStatsRegistry()
	r.Record("example.com", "", 50, 100, true)

	if len(r.metrics) != 0 {
		t.Errorf("empty outbound should not be recorded, got %d domains", len(r.metrics))
	}
}

// ─── TopDomains ────────────────────────────────────────────────────────────

func TestDomainStatsTopDomainsSortsByRequests(t *testing.T) {
	r := NewDomainStatsRegistry()

	for i := 0; i < 100; i++ {
		r.Record("high.com", "ob", 50, 100, true)
	}
	for i := 0; i < 10; i++ {
		r.Record("medium.com", "ob", 50, 100, true)
	}
	for i := 0; i < 1; i++ {
		r.Record("low.com", "ob", 50, 100, true)
	}

	top := r.TopDomains(10)
	if len(top) != 3 {
		t.Fatalf("TopDomains len = %d, want 3", len(top))
	}
	if top[0].Domain != "high.com" {
		t.Errorf("top[0].Domain = %q, want high.com", top[0].Domain)
	}
	if top[1].Domain != "medium.com" {
		t.Errorf("top[1].Domain = %q, want medium.com", top[1].Domain)
	}
	if top[2].Domain != "low.com" {
		t.Errorf("top[2].Domain = %q, want low.com", top[2].Domain)
	}
}

func TestDomainStatsTopDomainsLimitN(t *testing.T) {
	r := NewDomainStatsRegistry()
	for i := 0; i < 10; i++ {
		r.Record(fmt.Sprintf("d%d.com", i), "ob", 50, 100, true)
	}

	top := r.TopDomains(3)
	if len(top) != 3 {
		t.Errorf("TopDomains(3) len = %d, want 3", len(top))
	}
}

func TestDomainStatsTopDomainsEmpty(t *testing.T) {
	r := NewDomainStatsRegistry()
	top := r.TopDomains(10)
	if len(top) != 0 {
		t.Errorf("TopDomains on empty registry len = %d, want 0", len(top))
	}
}

func TestDomainStatsTopDomainsSuccessRate(t *testing.T) {
	r := NewDomainStatsRegistry()

	// 7 success, 3 fail → 70%
	for i := 0; i < 7; i++ {
		r.Record("partial.com", "ob", 50, 100, true)
	}
	for i := 0; i < 3; i++ {
		r.Record("partial.com", "ob", 0, 0, false)
	}

	top := r.TopDomains(10)
	if len(top) != 1 {
		t.Fatalf("TopDomains len = %d, want 1", len(top))
	}
	if top[0].SuccessRate != 70 {
		t.Errorf("SuccessRate = %d, want 70", top[0].SuccessRate)
	}
}

// ─── Summary ───────────────────────────────────────────────────────────────

func TestDomainStatsSummary(t *testing.T) {
	r := NewDomainStatsRegistry()
	r.Record("a.com", "ob1", 50, 100, true)
	r.Record("a.com", "ob1", 50, 100, true)
	r.Record("b.com", "ob2", 50, 100, true)

	s := r.Summary()
	if s.TotalDomains != 2 {
		t.Errorf("TotalDomains = %d, want 2", s.TotalDomains)
	}
	if s.TotalRequests != 3 {
		t.Errorf("TotalRequests = %d, want 3", s.TotalRequests)
	}
}

func TestDomainStatsString(t *testing.T) {
	r := NewDomainStatsRegistry()
	r.Record("x.com", "ob", 50, 100, true)
	s := r.String()
	if s != "DomainStats{domains=1}" {
		t.Errorf("String() = %q, want DomainStats{domains=1}", s)
	}
}

// ─── Concurrent safety ─────────────────────────────────────────────────────

func TestDomainStatsConcurrentRecordAndGetBest(t *testing.T) {
	r := NewDomainStatsRegistry()
	var wg sync.WaitGroup

	// Concurrent writers.
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			domain := fmt.Sprintf("d%d.com", n%5)
			r.Record(domain, "ob1", 50, 100, true)
			r.Record(domain, "ob2", 100, 100, true)
		}(i)
	}

	// Concurrent readers.
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			_ = r.GetBest(fmt.Sprintf("d%d.com", n%5))
			_ = r.TopDomains(10)
			_ = r.Summary()
		}(i)
	}

	wg.Wait()
}

// ─── Freshness decay ───────────────────────────────────────────────────────

func TestDomainStatsFreshnessDecaysScore(t *testing.T) {
	r := NewDomainStatsRegistry()

	// Record a perfect metric and manually set LastUsed to 30 minutes ago.
	r.Record("old.com", "ob", 50, 100, true)
	r.mu.Lock()
	r.metrics["old.com"]["ob"].LastUsed = time.Now().Add(-30 * time.Minute)
	r.mu.Unlock()

	// Record the same perfect metric but fresh.
	r.Record("new.com", "ob", 50, 100, true)

	// TopDomains should rank the fresh one higher (more requests → top).
	top := r.TopDomains(10)
	if len(top) < 2 {
		t.Fatalf("need at least 2 domains, got %d", len(top))
	}
}
