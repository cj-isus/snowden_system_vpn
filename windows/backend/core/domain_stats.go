package core

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

// DomainStats tracks per-domain performance metrics so the adaptive router can
// "remember" which outbound works best for each destination. Think of it as a
// smart cache: the first time youtube.com is requested, both VLESS and
// Hysteria2 are tested; whichever is faster is remembered for that domain.
//
// Data persists in-memory for the session. The router queries GetBest(domain)
// to pick an outbound, and Record() updates scores after each request.

// DomainMetric holds cumulative stats for one domain × one outbound.
type DomainMetric struct {
	Domain   string `json:"domain"`
	Outbound string `json:"outbound"`
	// Performance
	Requests   int   `json:"requests"`
	TotalBytes int64 `json:"totalBytes"`
	// Latency tracking (EWMA — exponentially weighted moving average)
	AvgLatencyMs int `json:"avgLatencyMs"`
	// Reliability
	SuccessCount int `json:"successCount"`
	ErrorCount   int `json:"errorCount"`
	// Last sample
	LastLatencyMs int       `json:"lastLatencyMs"`
	LastUsed      time.Time `json:"lastUsed"`
}

// DomainStatsRegistry is the central store, safe for concurrent access.
type DomainStatsRegistry struct {
	mu      sync.RWMutex
	metrics map[string]map[string]*DomainMetric // domain → outbound → metric
}

// NewDomainStatsRegistry creates an empty registry.
func NewDomainStatsRegistry() *DomainStatsRegistry {
	return &DomainStatsRegistry{
		metrics: make(map[string]map[string]*DomainMetric),
	}
}

// Record updates the stats for a domain+outbound pair after a request completes.
// latencyMs is the round-trip time (0 if unknown). bytes is how much was transferred.
// success indicates whether the request succeeded.
func (r *DomainStatsRegistry) Record(domain, outbound string, latencyMs int, bytes int64, success bool) {
	if domain == "" || outbound == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.metrics[domain] == nil {
		r.metrics[domain] = make(map[string]*DomainMetric)
	}
	m, exists := r.metrics[domain][outbound]
	if !exists {
		m = &DomainMetric{Domain: domain, Outbound: outbound}
		r.metrics[domain][outbound] = m
	}

	m.Requests++
	m.TotalBytes += bytes
	m.LastUsed = time.Now()
	if success {
		m.SuccessCount++
	} else {
		m.ErrorCount++
	}
	if latencyMs > 0 {
		m.LastLatencyMs = latencyMs
		// EWMA: weigh new sample at 30%, old average at 70% — smooths jitter
		if m.AvgLatencyMs == 0 {
			m.AvgLatencyMs = latencyMs
		} else {
			m.AvgLatencyMs = int(float64(m.AvgLatencyMs)*0.7 + float64(latencyMs)*0.3)
		}
	}
}

// GetBest returns the outbound with the best score for a domain, or "" if no data.
// Score = reliability × speed. Penalises high latency and error rate.
func (r *DomainStatsRegistry) GetBest(domain string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	outbounds, exists := r.metrics[domain]
	if !exists || len(outbounds) == 0 {
		return ""
	}

	bestScore := -1.0
	bestOutbound := ""
	for ob, m := range outbounds {
		score := r.scoreMetric(m)
		if score > bestScore {
			bestScore = score
			bestOutbound = ob
		}
	}
	return bestOutbound
}

// scoreMetric computes a 0-100 score: higher is better.
// Factors: success rate (weight 40%), latency (weight 40%), freshness (weight 20%).
func (r *DomainStatsRegistry) scoreMetric(m *DomainMetric) float64 {
	if m.Requests == 0 {
		return 0
	}
	// Reliability: success rate (0-1)
	reliability := float64(m.SuccessCount) / float64(m.Requests)

	// Latency score: 200ms=100, 2000ms=0, linear
	latencyScore := 100.0
	if m.AvgLatencyMs > 0 {
		latencyScore = 100.0 * (1.0 - float64(m.AvgLatencyMs)/2000.0)
		if latencyScore < 0 {
			latencyScore = 0
		}
	}

	// Freshness: used in last 5 min = 100, decays to 0 over 1 hour
	ageMin := time.Since(m.LastUsed).Minutes()
	freshness := 100.0 * (1.0 - ageMin/60.0)
	if freshness < 0 {
		freshness = 0
	}

	return reliability*40 + latencyScore*40 + freshness*20
}

// TopDomains returns the N most-used domains with their best outbound + score.
// Used by the UI to show "what works best where".
type DomainScore struct {
	Domain       string `json:"domain"`
	BestOutbound string `json:"bestOutbound"`
	Score        int    `json:"score"`
	Requests     int    `json:"requests"`
	AvgLatencyMs int    `json:"avgLatencyMs"`
	SuccessRate  int    `json:"successRate"`
}

func (r *DomainStatsRegistry) TopDomains(n int) []DomainScore {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var results []DomainScore
	for domain, outbounds := range r.metrics {
		bestScore := -1.0
		bestOb := ""
		var totalReqs int
		var avgLat int
		var totalSuccess int
		for ob, m := range outbounds {
			s := r.scoreMetric(m)
			if s > bestScore {
				bestScore = s
				bestOb = ob
				avgLat = m.AvgLatencyMs
			}
			totalReqs += m.Requests
			totalSuccess += m.SuccessCount
		}
		successRate := 100
		if totalReqs > 0 {
			successRate = totalSuccess * 100 / totalReqs
		}
		results = append(results, DomainScore{
			Domain:       domain,
			BestOutbound: bestOb,
			Score:        int(bestScore),
			Requests:     totalReqs,
			AvgLatencyMs: avgLat,
			SuccessRate:  successRate,
		})
	}
	// Sort by requests (most used first)
	sort.Slice(results, func(i, j int) bool {
		return results[i].Requests > results[j].Requests
	})
	if len(results) > n {
		results = results[:n]
	}
	return results
}

// Summary returns aggregate stats for the Telegram bot / UI header.
type DomainStatsSummary struct {
	TotalDomains  int `json:"totalDomains"`
	TotalRequests int `json:"totalRequests"`
}

func (r *DomainStatsRegistry) Summary() DomainStatsSummary {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s := DomainStatsSummary{}
	s.TotalDomains = len(r.metrics)
	for _, obs := range r.metrics {
		for _, m := range obs {
			s.TotalRequests += m.Requests
		}
	}
	return s
}

// String for debug logging
func (r *DomainStatsRegistry) String() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return fmt.Sprintf("DomainStats{domains=%d}", len(r.metrics))
}
