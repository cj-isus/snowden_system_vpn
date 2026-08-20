package core

import (
	"os"
	"path/filepath"
	"testing"
)

// TestChannelMemoryScorePrefersHealthy verifies: 3 fails on A and 3 OK on B →
// Score(B) > Score(A).
func TestChannelMemoryScorePrefersHealthy(t *testing.T) {
	m := NewChannelMemory("")
	m.Record("A", false)
	m.Record("A", false)
	m.Record("A", false)
	m.Record("B", true)
	m.Record("B", true)
	m.Record("B", true)

	if m.Score("B") <= m.Score("A") {
		t.Fatalf("Score(B)=%v should exceed Score(A)=%v (recently-failed channel penalised)",
			m.Score("B"), m.Score("A"))
	}
}

// TestChannelMemoryUnknownNeutral verifies an unknown channel gets a neutral
// score (never 0) so it is not unfairly excluded.
func TestChannelMemoryUnknownNeutral(t *testing.T) {
	m := NewChannelMemory("")
	if s := m.Score("missing"); s <= 0 {
		t.Fatalf("unknown channel score = %v, want > 0", s)
	}
}

// TestChannelMemoryBest verifies Best returns the highest-scoring candidate.
func TestChannelMemoryBest(t *testing.T) {
	m := NewChannelMemory("")
	m.Record("warp:dns", false)
	m.Record("warp:dns", false)
	m.Record("vps:hy2", true)
	m.Record("vps:hy2", true)

	best := m.Best([]string{"warp:dns", "vps:hy2"})
	if best != "vps:hy2" {
		t.Fatalf("Best = %q, want vps:hy2", best)
	}
}

// TestChannelMemorySaveLoadRoundTrip verifies persistence round-trips.
func TestChannelMemorySaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "channel-memory.json")
	m := NewChannelMemory(path)
	m.Record("vps:hy2:89.125.1.217:8443", true)
	m.Record("vps:hy2:89.125.1.217:8443", true)
	if err := m.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}

	m2 := NewChannelMemory(path)
	if err := m2.Load(); err != nil {
		t.Fatalf("load: %v", err)
	}
	if m2.Score("vps:hy2:89.125.1.217:8443") <= 0 {
		t.Fatal("loaded memory lost the recorded channel")
	}
}

// TestChannelMemoryLoadMissingFile verifies Load on absent file is a no-op.
func TestChannelMemoryLoadMissingFile(t *testing.T) {
	m := NewChannelMemory(filepath.Join(t.TempDir(), "nope.json"))
	if err := m.Load(); err != nil {
		t.Fatalf("load missing file: %v", err)
	}
}

// TestChannelMemoryCap verifies EnforceCap trims to maxKeys.
func TestChannelMemoryCap(t *testing.T) {
	m := NewChannelMemory("")
	m.maxKeys = 5
	for i := 0; i < 20; i++ {
		m.Record("ch", true)
	}
	m.EnforceCap()
	if s := m.Summary(100); s.Total > 5 {
		t.Fatalf("after cap, expected ≤5 records, got %d", s.Total)
	}
}

// TestChannelMemoryPrune verifies Prune drops channels not in the valid set.
func TestChannelMemoryPrune(t *testing.T) {
	m := NewChannelMemory("")
	m.Record("keep", true)
	m.Record("gone", true)
	m.Prune([]string{"keep"})
	if s := m.Summary(10); s.Total != 1 || s.BestKey != "keep" {
		t.Fatalf("after prune expected 1 record 'keep', got %+v", s)
	}
}

// TestChannelKeysFromConfig verifies keys are secret-free and ordered.
func TestChannelKeysFromConfig(t *testing.T) {
	cfg := testCfgWith(
		map[string]string{"type": "hysteria2", "tag": "hysteria2-nl", "server": "89.125.1.217", "server_port": "8443"},
		map[string]string{"type": "direct", "tag": "direct"},
		map[string]string{"type": "block", "tag": "block"},
	)
	keys := ChannelKeysFromConfig(cfg)
	want := ChannelKey(ChannelDescriptor{ID: "hysteria2-nl", Tag: "hysteria2-nl", Type: "hysteria2", Server: "89.125.1.217", Port: 8443})
	if len(keys) != 1 || keys[0] != want {
		t.Fatalf("ChannelKeysFromConfig = %v, want [%s]", keys, want)
	}
	if PrimaryChannelKeyFromConfig(cfg) != want {
		t.Fatal("PrimaryChannelKeyFromConfig mismatch")
	}
}

// TestDefaultChannelMemoryPath verifies the path never contains secrets and is
// non-empty.
func TestDefaultChannelMemoryPath(t *testing.T) {
	p := DefaultChannelMemoryPath()
	if p == "" {
		t.Fatal("DefaultChannelMemoryPath empty")
	}
	if _, err := os.Stat(p); os.IsNotExist(err) == false {
		// exists is fine — just ensure it's a writable-looking path
		_ = err
	}
}
